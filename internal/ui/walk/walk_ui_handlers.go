//go:build windows

package walk

import (
	"fmt"
	"strings"
	"time"

	"cid_fyne/internal/core"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

func (a *walkApp) openSelectedHistory() {
	if a.objTable == nil {
		return
	}
	row := a.objTable.CurrentIndex()
	d, ok := a.deviceModel.Row(row)
	if !ok {
		walk.MsgBox(a.mw, "Історія", "Виберіть об'єкт.", walk.MsgBoxIconInformation)
		return
	}
	a.openHistoryDialog(d)
}

func (a *walkApp) openHistoryDialog(device core.DeviceDTO) {
	state := &historyDialog{
		app:       a,
		model:     &eventTableModel{},
		device:    device,
		limit:     a.initialDeviceHistoryLimit(),
		eventType: "all",
	}

	err := Dialog{
		AssignTo: &state.dlg,
		Title:    fmt.Sprintf("Журнал об'єкта %03d", device.ID),
		MinSize:  Size{Width: 980, Height: 640},
		Background: SolidColorBrush{
			Color: colorWindow,
		},
		Layout: VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 8},
		Children: []Widget{
			Composite{
				Background: SolidColorBrush{Color: colorSurface},
				Layout:     HBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}, Spacing: 8},
				Children: []Widget{
					Label{Text: fmt.Sprintf("Об'єкт: %03d", device.ID), Font: Font{Bold: true}},
					Label{Text: firstNonEmpty(device.ClientAddr, "-"), TextColor: colorSoft},
					HSpacer{},
					Label{AssignTo: &state.status, Text: "Завантаження..."},
				},
			},
			Composite{
				Background: SolidColorBrush{Color: colorSurface},
				Layout:     HBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}, Spacing: 8},
				Children: []Widget{
					ComboBox{
						AssignTo: &state.filter,
						Model:    eventFilters,
						OnCurrentIndexChanged: func() {
							if state.filter.CurrentIndex() >= 0 {
								state.eventType = eventFilters[state.filter.CurrentIndex()]
								state.limit = a.initialDeviceHistoryLimit()
								state.allShown.Store(false)
								a.reloadHistory(state)
							}
						},
					},
					CheckBox{
						AssignTo: &state.hide,
						Text:     "Приховати тестові",
						OnCheckedChanged: func() {
							state.hideTests = state.hide.Checked()
							state.limit = a.initialDeviceHistoryLimit()
							state.allShown.Store(false)
							a.reloadHistory(state)
						},
					},
					LineEdit{
						AssignTo:      &state.search,
						StretchFactor: 1,
						CueBanner:     "Пошук по історії",
						OnTextChanged: func() {
							state.query = state.search.Text()
							state.limit = a.initialDeviceHistoryLimit()
							state.allShown.Store(false)
							a.reloadHistory(state)
						},
					},
					PushButton{Text: "Більше", OnClicked: func() {
						state.limit = addLimitSafe(state.limit, historyLoadChunk)
						state.allShown.Store(false)
						a.reloadHistory(state)
					}},
					PushButton{Text: "Оновити", OnClicked: func() { a.reloadHistory(state) }},
				},
			},
			TableView{
				Background:          SolidColorBrush{Color: colorSurface},
				AssignTo:            &state.table,
				AlternatingRowBG:    true,
				ColumnsOrderable:    true,
				LastColumnStretched: true,
				CustomHeaderHeight:  34,
				CustomRowHeight:     32,
				Model:               state.model,
				StyleCell: func(style *walk.CellStyle) {
					e, ok := state.model.Row(style.Row())
					if !ok {
						return
					}
					style.BackgroundColor, style.TextColor = priorityColors(e.Category, style.Row())
				},
				Columns: []TableViewColumn{
					{Title: "Час", Width: 140},
					{Title: "ППК", Width: 75},
					{Title: "Код", Width: 60},
					{Title: "Тип", Width: 120},
					{Title: "Опис", Width: 340},
					{Title: "Зона/Група", Width: 160},
					{Title: "Категорія", Width: 90},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{Text: "Закрити", OnClicked: func() { state.dlg.Accept() }},
				},
			},
		},
	}.Create(a.mw)
	if err != nil {
		walk.MsgBox(a.mw, "Історія", err.Error(), walk.MsgBoxIconError)
		return
	}

	if state.table != nil {
		state.model.SetTableView(state.table)
		state.table.SetGridlines(false)
		a.applyHistoryTableScale(state)
	}
	state.filter.SetCurrentIndex(0)
	state.dlg.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		state.closed.Store(true)
	})
	a.startHistoryAutoLoad(state)
	a.reloadHistory(state)
	state.dlg.Run()
	state.closed.Store(true)
}

func (a *walkApp) reloadHistory(state *historyDialog) {
	if state == nil || state.closed.Load() {
		return
	}
	if !state.loading.CompareAndSwap(false, true) {
		return
	}
	reqID := state.reqID.Add(1)
	if state.status != nil {
		state.status.SetText("Завантаження...")
	}
	go func() {
		rows, err := a.rt.FilterDeviceHistory(
			a.ctx,
			state.device.ID,
			state.limit,
			time.Time{},
			time.Time{},
			state.eventType,
			state.hideTests,
			false, // hideBlocked
			strings.TrimSpace(state.query),
		)
		if state.closed.Load() || state.dlg == nil {
			state.loading.Store(false)
			return
		}
		state.dlg.Synchronize(func() {
			defer state.loading.Store(false)
			if state.closed.Load() || reqID != state.reqID.Load() {
				return
			}
			if err != nil {
				state.status.SetText("Помилка: " + err.Error())
				return
			}
			state.model.SetRows(rows)
			state.allShown.Store(len(rows) < state.limit)
			state.status.SetText(fmt.Sprintf("Записів: %d", len(rows)))
		})
	}()
}

func (a *walkApp) startHistoryAutoLoad(state *historyDialog) {
	if state == nil || state.table == nil || state.dlg == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(uiScrollTick)
		defer ticker.Stop()
		for {
			if state.closed.Load() {
				return
			}
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				if state.closed.Load() || state.loading.Load() || state.allShown.Load() {
					continue
				}
				state.dlg.Synchronize(func() {
					if state.closed.Load() || state.loading.Load() || state.allShown.Load() || state.table == nil {
						return
					}
					if a.shouldAutoLoadByScroll(state.table, state.model.RowCount()) {
						state.limit = addLimitSafe(state.limit, historyLoadChunk)
						a.reloadHistory(state)
					}
				})
			}
		}
	}()
}

func (a *walkApp) deleteSelectedDevice() {
	if a.objTable == nil {
		return
	}
	row := a.objTable.CurrentIndex()
	d, ok := a.deviceModel.Row(row)
	if !ok {
		walk.MsgBox(a.mw, "Видалення", "Виберіть об'єкт.", walk.MsgBoxIconInformation)
		return
	}
	result := walk.MsgBox(a.mw, "Підтвердження", fmt.Sprintf("Видалити об'єкт %03d разом з історією?", d.ID), walk.MsgBoxYesNo|walk.MsgBoxIconWarning)
	if result != walk.DlgCmdYes {
		return
	}
	go func(device core.DeviceDTO) {
		err := a.rt.DeleteDeviceWithHistory(a.ctx, device.ID)
		if a.mw == nil {
			return
		}
		a.mw.Synchronize(func() {
			if err != nil {
				a.statusErr = "Delete failed: " + err.Error()
				a.updateStatusBar()
				walk.MsgBox(a.mw, "Видалення", err.Error(), walk.MsgBoxIconError)
				return
			}
			// Local cleanup will be handled by pendingDeleted channel in flushPending
			a.status = fmt.Sprintf("Об'єкт %03d видалено", device.ID)
			a.statusErr = ""
			a.updateStatusBar()
		})
	}(d)
}

func (a *walkApp) reloadAll() {
	reqID := a.bootReqID.Add(1)
	go func() {
		boot, err := a.rt.Bootstrap(a.ctx, 1)
		if err != nil {
			if a.mw == nil {
				return
			}
			a.mw.Synchronize(func() {
				if reqID != a.bootReqID.Load() {
					return
				}
				a.statusErr = "Reload failed: " + err.Error()
				a.updateStatusBar()
			})
			return
		}
		rows, err := a.rt.FilterEvents(a.ctx, a.eventsLimit, "all", false, false, "")
		if a.mw == nil {
			return
		}
		a.mw.Synchronize(func() {
			if reqID != a.bootReqID.Load() {
				return
			}
			if err != nil {
				a.statusErr = "Reload failed: " + err.Error()
				a.updateStatusBar()
				return
			}
			a.mu.Lock()
			a.devices = make(map[int]core.DeviceDTO, len(boot.Devices))
			for _, d := range boot.Devices {
				a.devices[d.ID] = d
			}
			a.events = append(a.events[:0], rows...)
			a.eventsAllShown.Store(len(rows) < a.eventsLimit)
			a.status = "Дані оновлено"
			a.statusErr = ""
			a.applyFiltersLocked()
			a.mu.Unlock()
			a.syncUIState(true, true)
		})
	}()
}

func (a *walkApp) loadMoreEvents() {
	if !a.eventsLoading.CompareAndSwap(false, true) {
		return
	}
	next := addLimitSafe(a.eventsLimit, eventsLoadChunk)
	go func(limit int) {
		rows, err := a.rt.FilterEvents(a.ctx, limit, "all", false, false, "")
		if a.mw == nil {
			a.eventsLoading.Store(false)
			return
		}
		a.mw.Synchronize(func() {
			defer a.eventsLoading.Store(false)
			if err != nil {
				a.statusErr = "Events reload failed: " + err.Error()
				a.updateStatusBar()
				return
			}
			a.mu.Lock()
			a.eventsLimit = limit
			a.events = append(a.events[:0], rows...)
			a.eventsAllShown.Store(len(rows) < limit)
			a.statusErr = ""
			a.status = fmt.Sprintf("Завантажено %d подій", len(rows))
			a.applyFiltersLocked()
			a.mu.Unlock()
			a.syncUIState(false, true)
		})
	}(next)
}

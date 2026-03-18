//go:build windows

package windigo

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cid_fyne/internal/config"
	"cid_fyne/internal/core"
	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
)

const (
	maxPendingDevices = 20000
	maxPendingEvents  = 120000
	uiTick            = 900 * time.Millisecond
	eventBatchStep    = 250
)

type appWindow struct {
	wnd *ui.Main
	rt  core.Backend

	lblTitle   *ui.Static
	lblStatus  *ui.Static
	lblStats   *ui.Static
	lblObjects *ui.Static
	lblEvents  *ui.Static

	btnSync       *ui.Button
	btnMore       *ui.Button
	btnObjHistory *ui.Button
	btnObjDelete  *ui.Button
	btnEvtDetails *ui.Button
	btnSettings   *ui.Button

	objSearch   *ui.Edit
	evtSearch   *ui.Edit
	evtFilter   *ui.ComboBox
	hideTestsCB *ui.CheckBox

	objList *ui.ListView
	evtList *ui.ListView

	mu      sync.RWMutex
	devices map[int]core.DeviceDTO
	events  []core.EventDTO
	stats   core.StatsDTO

	pendingDevices chan core.DeviceDTO
	pendingEvents  chan core.EventDTO
	pendingDeleted chan int

	eventLimit int
	eventType  string
	hideTests  bool
	query      string
	objQuery   string

	activityTO time.Duration
}

// Run starts the desktop UI with the native Windigo framework.
func Run(ctx context.Context, rt core.Backend) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	w := &appWindow{
		rt:             rt,
		devices:        make(map[int]core.DeviceDTO),
		pendingDevices: make(chan core.DeviceDTO, maxPendingDevices),
		pendingEvents:  make(chan core.EventDTO, maxPendingEvents),
		pendingDeleted: make(chan int, 100),
		eventLimit:     500,
		eventType:      "all",
	}

	if err := w.startBackend(ctx); err != nil {
		return err
	}
	defer w.stopBackend()

	w.build()
	w.bindEvents(ctx, cancel)
	w.refreshUI()

	go w.uiSyncLoop(ctx)

	w.wnd.RunAsMain()
	return nil
}

func (w *appWindow) startBackend(ctx context.Context) error {
	if err := w.rt.Start(ctx); err != nil {
		return err
	}

	cfg := w.rt.GetConfig()
	w.activityTO = core.ParseDuration(cfg.Monitoring.PpkTimeout, 15*time.Minute)

	boot, err := w.rt.Bootstrap(ctx, w.eventLimit)
	if err != nil {
		return err
	}
	for _, d := range boot.Devices {
		w.devices[d.ID] = d
	}
	w.events = append(w.events, boot.Events...)
	w.stats = w.rt.GetStats()

	w.rt.SubscribeDevice(func(d core.DeviceDTO) {
		select {
		case w.pendingDevices <- d:
		default:
		}
	})
	w.rt.SubscribeEvent(func(e core.EventDTO) {
		select {
		case w.pendingEvents <- e:
		default:
		}
	})
	w.rt.SubscribeDeviceDeleted(func(id int) {
		select {
		case w.pendingDeleted <- id:
		default:
		}
	})
	return nil
}

func (w *appWindow) stopBackend() {
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()
	_ = w.rt.Stop(stopCtx)
}

func (w *appWindow) build() {
	w.wnd = ui.NewMain(
		ui.OptsMain().
			Title("CID Ретранслятор - Windigo UI").
			Size(ui.Dpi(1360, 860)),
	)

	w.lblTitle = ui.NewStatic(
		w.wnd,
		ui.OptsStatic().
			Text("CID Windigo").
			Position(ui.Dpi(20, 20)).
			Size(ui.Dpi(280, 28)),
	)
	w.lblStatus = ui.NewStatic(
		w.wnd,
		ui.OptsStatic().
			Text("Стан: -").
			Position(ui.Dpi(20, 56)).
			Size(ui.Dpi(540, 24)),
	)
	w.lblStats = ui.NewStatic(
		w.wnd,
		ui.OptsStatic().
			Text("Метрики: -").
			Position(ui.Dpi(20, 86)).
			Size(ui.Dpi(1100, 24)),
	)

	w.btnSync = ui.NewButton(
		w.wnd,
		ui.OptsButton().
			Text("&Синхронізувати").
			Position(ui.Dpi(20, 120)).
			Width(ui.DpiX(140)),
	)
	w.btnMore = ui.NewButton(
		w.wnd,
		ui.OptsButton().
			Text("Завантажити &ще").
			Position(ui.Dpi(170, 120)).
			Width(ui.DpiX(165)),
	)
	w.btnObjHistory = ui.NewButton(
		w.wnd,
		ui.OptsButton().
			Text("&Журнал об'єкта").
			Position(ui.Dpi(345, 120)).
			Width(ui.DpiX(150)),
	)
	w.btnObjDelete = ui.NewButton(
		w.wnd,
		ui.OptsButton().
			Text("&Видалити об'єкт").
			Position(ui.Dpi(505, 120)).
			Width(ui.DpiX(160)),
	)
	w.btnEvtDetails = ui.NewButton(
		w.wnd,
		ui.OptsButton().
			Text("Деталі &події").
			Position(ui.Dpi(675, 120)).
			Width(ui.DpiX(130)),
	)
	w.btnSettings = ui.NewButton(
		w.wnd,
		ui.OptsButton().
			Text("&Налаштування").
			Position(ui.Dpi(815, 120)).
			Width(ui.DpiX(140)),
	)

	w.objSearch = ui.NewEdit(
		w.wnd,
		ui.OptsEdit().
			Text("").
			Position(ui.Dpi(20, 158)).
			Width(ui.DpiX(620)),
	)
	w.objSearch.Hwnd().SetWindowText(" ")
	w.objSearch.SetText("")

	w.lblObjects = ui.NewStatic(
		w.wnd,
		ui.OptsStatic().
			Text("Об'єкти").
			Position(ui.Dpi(20, 184)).
			Size(ui.Dpi(320, 24)),
	)
	w.objList = ui.NewListView(
		w.wnd,
		ui.OptsListView().
			Position(ui.Dpi(20, 212)).
			Size(ui.Dpi(620, 602)).
			Column("Стан", ui.DpiX(95)).
			Column("PPK", ui.DpiX(72)).
			Column("Клієнт", ui.DpiX(180)).
			Column("Остання подія", ui.DpiX(170)).
			Column("Date/Time", ui.DpiX(140)),
	)
	w.objList.SetView(co.LV_VIEW_DETAILS)
	w.objList.SetExtendedStyle(true, co.LVS_EX_FULLROWSELECT|co.LVS_EX_GRIDLINES|co.LVS_EX_DOUBLEBUFFER)

	w.evtFilter = ui.NewComboBox(
		w.wnd,
		ui.OptsComboBox().
			Position(ui.Dpi(660, 120)).
			Width(ui.DpiX(110)).
			Texts("all", "alarm", "test", "fault", "guard", "disguard", "other").
			Select(0),
	)
	w.hideTestsCB = ui.NewCheckBox(
		w.wnd,
		ui.OptsCheckBox().
			Text("Приховати test").
			Position(ui.Dpi(782, 120)).
			Size(ui.Dpi(130, 24)),
	)
	w.evtSearch = ui.NewEdit(
		w.wnd,
		ui.OptsEdit().
			Text("").
			Position(ui.Dpi(922, 120)).
			Width(ui.DpiX(418)),
	)

	w.lblEvents = ui.NewStatic(
		w.wnd,
		ui.OptsStatic().
			Text("Події").
			Position(ui.Dpi(660, 166)).
			Size(ui.Dpi(320, 24)),
	)
	w.evtList = ui.NewListView(
		w.wnd,
		ui.OptsListView().
			Position(ui.Dpi(660, 194)).
			Size(ui.Dpi(680, 620)).
			Column("Time", ui.DpiX(136)).
			Column("PPK", ui.DpiX(62)).
			Column("Code", ui.DpiX(60)).
			Column("Тип", ui.DpiX(120)).
			Column("Опис", ui.DpiX(280)).
			Column("Зона", ui.DpiX(110)),
	)
	w.evtList.SetView(co.LV_VIEW_DETAILS)
	w.evtList.SetExtendedStyle(true, co.LVS_EX_FULLROWSELECT|co.LVS_EX_GRIDLINES|co.LVS_EX_DOUBLEBUFFER)

	if rc, err := w.wnd.Hwnd().GetClientRect(); err == nil {
		w.applyMainLayout(win.SIZE{Cx: rc.Right - rc.Left, Cy: rc.Bottom - rc.Top})
	}
}

func (w *appWindow) bindEvents(ctx context.Context, cancel context.CancelFunc) {
	w.btnSync.On().BnClicked(func() {
		w.refreshFromBackend(ctx)
		w.refreshUI()
	})
	w.btnMore.On().BnClicked(func() {
		w.eventLimit += eventBatchStep
		w.refreshFromBackend(ctx)
		w.refreshUI()
	})
	w.btnObjHistory.On().BnClicked(func() {
		w.showSelectedObjectHistory(ctx)
	})
	w.btnObjDelete.On().BnClicked(func() {
		w.deleteSelectedObject(ctx)
	})
	w.btnEvtDetails.On().BnClicked(func() {
		w.showSelectedEventDetails()
	})
	w.btnSettings.On().BnClicked(func() {
		w.openSettingsModal(ctx)
	})

	w.objSearch.On().EnChange(func() {
		w.objQuery = strings.TrimSpace(w.objSearch.Text())
		w.refreshUI()
	})

	w.evtFilter.On().CbnSelChange(func() {
		w.eventType = strings.ToLower(strings.TrimSpace(w.evtFilter.CurrentText()))
		if w.eventType == "" {
			w.eventType = "all"
		}
		w.refreshUI()
	})
	w.hideTestsCB.On().BnClicked(func() {
		w.hideTests = w.hideTestsCB.IsChecked()
		w.refreshUI()
	})
	w.evtSearch.On().EnChange(func() {
		w.query = strings.TrimSpace(w.evtSearch.Text())
		w.refreshUI()
	})

	w.objList.On().NmDblClk(func(p *win.NMITEMACTIVATE) {
		_ = p
		w.openObjectHistoryModal(ctx)
	})
	w.evtList.On().NmDblClk(func(p *win.NMITEMACTIVATE) {
		_ = p
		w.showSelectedEventDetails()
	})

	w.wnd.On().WmClose(func() {
		cancel()
		_ = w.wnd.Hwnd().DestroyWindow()
	})
	w.wnd.On().WmSize(func(p ui.WmSize) {
		w.applyMainLayout(p.ClientAreaSize())
	})
}

func (w *appWindow) uiSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(uiTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hasChanges := w.flushPending()
			stats := w.rt.GetStats()
			w.mu.Lock()
			w.stats = stats
			w.mu.Unlock()
			if !hasChanges {
				w.wnd.UiThread(func() { w.updateHeader() })
				continue
			}
			w.wnd.UiThread(func() { w.refreshUI() })
		}
	}
}

func (w *appWindow) refreshFromBackend(ctx context.Context) {
	boot, err := w.rt.Bootstrap(ctx, w.eventLimit)
	if err != nil {
		_, _ = w.wnd.Hwnd().MessageBox("Помилка синхронізації: "+err.Error(), "CID Windigo", co.MB_ICONERROR)
		return
	}

	rows, err := w.rt.FilterEvents(ctx, w.eventLimit, w.eventType, w.hideTests, false, w.query)
	if err != nil {
		_, _ = w.wnd.Hwnd().MessageBox("Помилка завантаження подій: "+err.Error(), "CID Windigo", co.MB_ICONERROR)
		return
	}

	w.mu.Lock()
	w.devices = make(map[int]core.DeviceDTO, len(boot.Devices))
	for _, d := range boot.Devices {
		w.devices[d.ID] = d
	}
	w.events = append(w.events[:0], rows...)
	w.stats = w.rt.GetStats()
	w.mu.Unlock()
}

func (w *appWindow) flushPending() bool {
	changed := false
	w.mu.Lock()
	defer w.mu.Unlock()

	for i := 0; i < 2000; i++ {
		select {
		case id := <-w.pendingDeleted:
			delete(w.devices, id)
			changed = true
		default:
			i = 2000
		}
	}
	for i := 0; i < 6000; i++ {
		select {
		case d := <-w.pendingDevices:
			w.devices[d.ID] = d
			changed = true
		default:
			i = 6000
		}
	}
	for i := 0; i < 10000; i++ {
		select {
		case e := <-w.pendingEvents:
			w.events = append([]core.EventDTO{e}, w.events...)
			if len(w.events) > w.eventLimit {
				w.events = w.events[:w.eventLimit]
			}
			changed = true
		default:
			i = 10000
		}
	}
	return changed
}

func (w *appWindow) refreshUI() {
	w.updateHeader()
	w.renderObjects()
	w.renderEvents()
}

func (w *appWindow) updateHeader() {
	w.mu.RLock()
	stats := w.stats
	objCount := len(w.filteredDevicesLocked())
	evtCount := len(w.filteredEventsLocked())
	w.mu.RUnlock()

	status := "OFFLINE"
	if stats.Connected {
		status = "ONLINE"
	}
	w.lblStatus.SetTextAndResize(fmt.Sprintf("Стан: %s | Об'єкти: %d | Події: %d", status, objCount, evtCount))
	w.lblStats.SetTextAndResize(fmt.Sprintf("Клієнти: %d | Ack: %d | Nack: %d | Reconnects: %d | Швидкість: %d msg/min | Uptime: %s",
		stats.Clients, stats.Accepted, stats.Rejected, stats.Reconnects, stats.ReceivedPM, stats.Uptime))
	w.lblObjects.SetTextAndResize(fmt.Sprintf("Об'єкти (%d)", objCount))
	w.lblEvents.SetTextAndResize(fmt.Sprintf("Події (%d показано)", evtCount))
}

func (w *appWindow) renderObjects() {
	w.mu.RLock()
	devices := w.filteredDevicesLocked()
	timeout := w.activityTO
	w.mu.RUnlock()

	sort.Slice(devices, func(i, j int) bool { return devices[i].ID < devices[j].ID })

	w.objList.SetRedraw(false)
	w.objList.DeleteAllItems()
	for _, d := range devices {
		state := "Активний"
		if d.LastEventTime.IsZero() || time.Since(d.LastEventTime) > timeout {
			state = "Неактивний"
		}
		when := "-"
		if !d.LastEventTime.IsZero() {
			when = d.LastEventTime.Format("2006-01-02 15:04:05")
		}
		w.objList.AddItem(
			state,
			fmt.Sprintf("%03d", d.ID),
			firstNonEmpty(d.ClientAddr, "-"),
			firstNonEmpty(d.LastEvent, "-"),
			when,
		)
	}
	w.objList.SetRedraw(true)
}

func (w *appWindow) renderEvents() {
	w.mu.RLock()
	events := w.filteredEventsLocked()
	w.mu.RUnlock()

	w.evtList.SetRedraw(false)
	w.evtList.DeleteAllItems()
	for _, e := range events {
		when := "-"
		if !e.Time.IsZero() {
			when = e.Time.Format("2006-01-02 15:04:05")
		}
		w.evtList.AddItem(
			when,
			e.DeviceID,
			e.Code,
			e.Type,
			firstNonEmpty(e.Desc, "-"),
			firstNonEmpty(e.Zone, "-"),
		)
	}
	w.evtList.SetRedraw(true)
}

func (w *appWindow) filteredDevicesLocked() []core.DeviceDTO {
	devices := make([]core.DeviceDTO, 0, len(w.devices))
	q := strings.ToLower(strings.TrimSpace(w.objQuery))
	for _, d := range w.devices {
		if q != "" {
			hay := strings.ToLower(fmt.Sprintf("%03d %s %s", d.ID, d.ClientAddr, d.LastEvent))
			if !strings.Contains(hay, q) {
				continue
			}
		}
		devices = append(devices, d)
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].ID < devices[j].ID })
	return devices
}

func (w *appWindow) filteredEventsLocked() []core.EventDTO {
	filter := strings.ToLower(strings.TrimSpace(w.eventType))
	if filter == "" {
		filter = "all"
	}
	query := strings.ToLower(strings.TrimSpace(w.query))

	rows := make([]core.EventDTO, 0, len(w.events))
	for _, e := range w.events {
		if filter != "all" && !strings.EqualFold(e.Category, filter) {
			continue
		}
		if w.hideTests && strings.EqualFold(e.Category, "test") {
			continue
		}
		if query != "" {
			hay := strings.ToLower(strings.TrimSpace(e.DeviceID + " " + e.Code + " " + e.Type + " " + e.Desc + " " + e.Zone))
			if !strings.Contains(hay, query) {
				continue
			}
		}
		rows = append(rows, e)
	}
	return rows
}

func (w *appWindow) deleteSelectedObject(ctx context.Context) {
	selected := w.objList.SelectedItems()
	if len(selected) == 0 {
		_, _ = w.wnd.Hwnd().MessageBox("Спочатку виберіть об'єкт.", "CID Windigo", co.MB_ICONINFORMATION)
		return
	}
	idText := selected[0].Text(1)
	id, err := strconv.Atoi(strings.TrimSpace(idText))
	if err != nil {
		_, _ = w.wnd.Hwnd().MessageBox("Некоректний ID об'єкта.", "CID Windigo", co.MB_ICONERROR)
		return
	}

	res, _ := w.wnd.Hwnd().MessageBox(
		fmt.Sprintf("Видалити об'єкт %03d разом з історією?", id),
		"Підтвердження",
		co.MB_YESNO|co.MB_ICONQUESTION,
	)
	if res != co.ID_YES {
		return
	}

	if err := w.rt.DeleteDeviceWithHistory(ctx, id); err != nil {
		_, _ = w.wnd.Hwnd().MessageBox("Помилка видалення: "+err.Error(), "CID Windigo", co.MB_ICONERROR)
		return
	}

	w.mu.Lock()
	delete(w.devices, id)
	w.mu.Unlock()
	w.refreshUI()
}

func (w *appWindow) showSelectedObjectHistory(ctx context.Context) {
	w.openObjectHistoryModal(ctx)
}

func (w *appWindow) openObjectHistoryModal(ctx context.Context) {
	selected := w.objList.SelectedItems()
	if len(selected) == 0 {
		_, _ = w.wnd.Hwnd().MessageBox("Спочатку виберіть об'єкт.", "CID Windigo", co.MB_ICONINFORMATION)
		return
	}

	idText := selected[0].Text(1)
	id, err := strconv.Atoi(strings.TrimSpace(idText))
	if err != nil {
		_, _ = w.wnd.Hwnd().MessageBox("Некоректний ID об'єкта.", "CID Windigo", co.MB_ICONERROR)
		return
	}

	modal := ui.NewModal(
		w.wnd,
		ui.OptsModal().
			Title(fmt.Sprintf("Журнал об'єкта %03d", id)).
			Size(ui.Dpi(1120, 700)),
	)
	lbl := ui.NewStatic(
		modal,
		ui.OptsStatic().
			Text(fmt.Sprintf("Об'єкт: %03d", id)).
			Position(ui.Dpi(18, 14)).
			Size(ui.Dpi(320, 24)),
	)
	_ = lbl
	search := ui.NewEdit(
		modal,
		ui.OptsEdit().
			Position(ui.Dpi(18, 44)).
			Width(ui.DpiX(620)),
	)
	btnRefresh := ui.NewButton(
		modal,
		ui.OptsButton().
			Text("&Оновити").
			Position(ui.Dpi(650, 42)).
			Width(ui.DpiX(90)),
	)
	btnMore := ui.NewButton(
		modal,
		ui.OptsButton().
			Text("Ще &100").
			Position(ui.Dpi(748, 42)).
			Width(ui.DpiX(100)),
	)
	btnClose := ui.NewButton(
		modal,
		ui.OptsButton().
			Text("&Закрити").
			Position(ui.Dpi(858, 42)).
			Width(ui.DpiX(90)),
	)
	table := ui.NewListView(
		modal,
		ui.OptsListView().
			Position(ui.Dpi(18, 78)).
			Size(ui.Dpi(1080, 580)).
			Column("Час", ui.DpiX(136)).
			Column("PPK", ui.DpiX(62)).
			Column("Code", ui.DpiX(58)).
			Column("Тип", ui.DpiX(110)).
			Column("Опис", ui.DpiX(480)).
			Column("Зона", ui.DpiX(110)).
			Column("Категорія", ui.DpiX(96)),
	)
	table.SetView(co.LV_VIEW_DETAILS)
	table.SetExtendedStyle(true, co.LVS_EX_FULLROWSELECT|co.LVS_EX_GRIDLINES|co.LVS_EX_DOUBLEBUFFER)
	status := ui.NewStatic(
		modal,
		ui.OptsStatic().
			Text("Завантаження...").
			Position(ui.Dpi(18, 664)).
			Size(ui.Dpi(860, 24)),
	)

	layoutModal := func(size win.SIZE) {
		cw := int(size.Cx)
		ch := int(size.Cy)
		if cw <= 0 || ch <= 0 {
			return
		}

		margin := ui.DpiX(18)
		top := ui.DpiY(14)
		gap := ui.DpiX(10)
		rowH := ui.DpiY(26)
		btnW := ui.DpiX(90)
		btnMoreW := ui.DpiX(100)
		right := cw - margin

		lbl.Hwnd().SetWindowPos(win.HWND(0),
			win.POINT{X: int32(margin), Y: int32(top)},
			win.SIZE{Cx: int32(maxInt(220, cw-margin*2)), Cy: int32(rowH)},
			co.SWP_NOZORDER)

		searchY := top + rowH + ui.DpiY(8)
		closeX := right - btnW
		moreX := closeX - gap - btnMoreW
		refreshX := moreX - gap - btnW
		searchW := maxInt(200, refreshX-margin-gap)

		search.Hwnd().SetWindowPos(win.HWND(0),
			win.POINT{X: int32(margin), Y: int32(searchY)},
			win.SIZE{Cx: int32(searchW), Cy: int32(rowH)},
			co.SWP_NOZORDER)
		btnRefresh.Hwnd().SetWindowPos(win.HWND(0),
			win.POINT{X: int32(refreshX), Y: int32(searchY - ui.DpiY(2))},
			win.SIZE{Cx: int32(btnW), Cy: int32(rowH + ui.DpiY(4))},
			co.SWP_NOZORDER)
		btnMore.Hwnd().SetWindowPos(win.HWND(0),
			win.POINT{X: int32(moreX), Y: int32(searchY - ui.DpiY(2))},
			win.SIZE{Cx: int32(btnMoreW), Cy: int32(rowH + ui.DpiY(4))},
			co.SWP_NOZORDER)
		btnClose.Hwnd().SetWindowPos(win.HWND(0),
			win.POINT{X: int32(closeX), Y: int32(searchY - ui.DpiY(2))},
			win.SIZE{Cx: int32(btnW), Cy: int32(rowH + ui.DpiY(4))},
			co.SWP_NOZORDER)

		tableY := searchY + rowH + ui.DpiY(8)
		statusH := ui.DpiY(24)
		tableH := maxInt(140, ch-tableY-margin-statusH-ui.DpiY(6))
		tableW := maxInt(320, cw-margin*2)

		table.Hwnd().SetWindowPos(win.HWND(0),
			win.POINT{X: int32(margin), Y: int32(tableY)},
			win.SIZE{Cx: int32(tableW), Cy: int32(tableH)},
			co.SWP_NOZORDER)
		status.Hwnd().SetWindowPos(win.HWND(0),
			win.POINT{X: int32(margin), Y: int32(tableY + tableH + ui.DpiY(6))},
			win.SIZE{Cx: int32(tableW), Cy: int32(statusH)},
			co.SWP_NOZORDER)

		w.updateHistoryColumns(table, tableW)
	}

	limit := 100
	loadRows := func() {
		q := strings.TrimSpace(search.Text())
		go func(currentLimit int, query string) {
			rows, err := w.rt.FilterDeviceHistory(ctx, id, currentLimit, time.Time{}, time.Time{}, "all", false, false, query)
			modal.UiThread(func() {
				if err != nil {
					status.SetTextAndResize("Помилка: " + err.Error())
					return
				}
				table.SetRedraw(false)
				table.DeleteAllItems()
				for _, ev := range rows {
					when := "-"
					if !ev.Time.IsZero() {
						when = ev.Time.Format("2006-01-02 15:04:05")
					}
					table.AddItem(
						when,
						ev.DeviceID,
						ev.Code,
						ev.Type,
						firstNonEmpty(ev.Desc, "-"),
						firstNonEmpty(ev.Zone, "-"),
						firstNonEmpty(ev.Category, "-"),
					)
				}
				table.SetRedraw(true)
				status.SetTextAndResize(fmt.Sprintf("Записів: %d (ліміт: %d)", len(rows), currentLimit))
			})
		}(limit, q)
	}

	search.On().EnChange(func() { loadRows() })
	btnRefresh.On().BnClicked(func() { loadRows() })
	btnMore.On().BnClicked(func() {
		limit += 100
		loadRows()
	})
	btnClose.On().BnClicked(func() { _ = modal.Hwnd().DestroyWindow() })
	modal.On().WmClose(func() { _ = modal.Hwnd().DestroyWindow() })
	modal.On().WmSize(func(p ui.WmSize) { layoutModal(p.ClientAreaSize()) })

	loadRows()
	if rc, err := modal.Hwnd().GetClientRect(); err == nil {
		layoutModal(win.SIZE{Cx: rc.Right - rc.Left, Cy: rc.Bottom - rc.Top})
	}
	modal.ShowModal()
}

func (w *appWindow) applyMainLayout(size win.SIZE) {
	cw := int(size.Cx)
	ch := int(size.Cy)
	if cw <= 0 || ch <= 0 {
		return
	}

	margin := ui.DpiX(20)
	gap := ui.DpiX(20)
	topTitle := ui.DpiY(20)
	rowH := ui.DpiY(26)
	btnY := ui.DpiY(120)
	searchY := ui.DpiY(158)
	captionY := ui.DpiY(184)
	listY := ui.DpiY(212)
	bottomPad := ui.DpiY(20)

	leftW := maxInt(ui.DpiX(300), (cw-gap-margin*2)/2)
	rightW := maxInt(ui.DpiX(320), cw-margin*2-leftW-gap)
	leftX := margin
	rightX := leftX + leftW + gap

	w.lblTitle.Hwnd().SetWindowPos(win.HWND(0),
		win.POINT{X: int32(margin), Y: int32(topTitle)},
		win.SIZE{Cx: int32(maxInt(260, cw-margin*2)), Cy: int32(ui.DpiY(28))},
		co.SWP_NOZORDER)
	w.lblStatus.Hwnd().SetWindowPos(win.HWND(0),
		win.POINT{X: int32(margin), Y: int32(ui.DpiY(56))},
		win.SIZE{Cx: int32(maxInt(320, cw-margin*2)), Cy: int32(rowH)},
		co.SWP_NOZORDER)
	w.lblStats.Hwnd().SetWindowPos(win.HWND(0),
		win.POINT{X: int32(margin), Y: int32(ui.DpiY(86))},
		win.SIZE{Cx: int32(maxInt(400, cw-margin*2)), Cy: int32(rowH)},
		co.SWP_NOZORDER)

	btnW := []int{ui.DpiX(140), ui.DpiX(165), ui.DpiX(150), ui.DpiX(160), ui.DpiX(130), ui.DpiX(140)}
	btns := []*ui.Button{w.btnSync, w.btnMore, w.btnObjHistory, w.btnObjDelete, w.btnEvtDetails, w.btnSettings}
	x := margin
	for i, b := range btns {
		b.Hwnd().SetWindowPos(win.HWND(0),
			win.POINT{X: int32(x), Y: int32(btnY)},
			win.SIZE{Cx: int32(btnW[i]), Cy: int32(rowH + ui.DpiY(4))},
			co.SWP_NOZORDER)
		x += btnW[i] + ui.DpiX(10)
	}

	w.objSearch.Hwnd().SetWindowPos(win.HWND(0),
		win.POINT{X: int32(leftX), Y: int32(searchY)},
		win.SIZE{Cx: int32(leftW), Cy: int32(rowH)},
		co.SWP_NOZORDER)

	filterW := ui.DpiX(110)
	hideW := ui.DpiX(130)
	filterX := rightX
	hideX := filterX + filterW + ui.DpiX(8)
	searchEvtX := hideX + hideW + ui.DpiX(8)
	searchEvtW := maxInt(ui.DpiX(120), rightX+rightW-searchEvtX)

	w.evtFilter.Hwnd().SetWindowPos(win.HWND(0),
		win.POINT{X: int32(filterX), Y: int32(searchY)},
		win.SIZE{Cx: int32(filterW), Cy: int32(rowH)},
		co.SWP_NOZORDER)
	w.hideTestsCB.Hwnd().SetWindowPos(win.HWND(0),
		win.POINT{X: int32(hideX), Y: int32(searchY)},
		win.SIZE{Cx: int32(hideW), Cy: int32(rowH)},
		co.SWP_NOZORDER)
	w.evtSearch.Hwnd().SetWindowPos(win.HWND(0),
		win.POINT{X: int32(searchEvtX), Y: int32(searchY)},
		win.SIZE{Cx: int32(searchEvtW), Cy: int32(rowH)},
		co.SWP_NOZORDER)

	w.lblObjects.Hwnd().SetWindowPos(win.HWND(0),
		win.POINT{X: int32(leftX), Y: int32(captionY)},
		win.SIZE{Cx: int32(leftW), Cy: int32(rowH)},
		co.SWP_NOZORDER)
	w.lblEvents.Hwnd().SetWindowPos(win.HWND(0),
		win.POINT{X: int32(rightX), Y: int32(captionY)},
		win.SIZE{Cx: int32(rightW), Cy: int32(rowH)},
		co.SWP_NOZORDER)

	listH := maxInt(ui.DpiY(180), ch-listY-bottomPad)
	w.objList.Hwnd().SetWindowPos(win.HWND(0),
		win.POINT{X: int32(leftX), Y: int32(listY)},
		win.SIZE{Cx: int32(leftW), Cy: int32(listH)},
		co.SWP_NOZORDER)
	w.evtList.Hwnd().SetWindowPos(win.HWND(0),
		win.POINT{X: int32(rightX), Y: int32(listY)},
		win.SIZE{Cx: int32(rightW), Cy: int32(listH)},
		co.SWP_NOZORDER)

	w.updateObjectColumns(leftW)
	w.updateEventColumns(rightW)
}

func (w *appWindow) updateObjectColumns(listWidth int) {
	if w.objList == nil || w.objList.ColCount() < 5 {
		return
	}
	widths := fitColumnsToWidth(listWidth, ui.DpiX(20), []int{95, 72, 180, 170, 140}, []int{70, 56, 120, 110, 110}, 3)
	for i, colW := range widths {
		w.objList.Col(i).SetWidth(colW)
	}
}

func (w *appWindow) updateEventColumns(listWidth int) {
	if w.evtList == nil || w.evtList.ColCount() < 6 {
		return
	}
	widths := fitColumnsToWidth(listWidth, ui.DpiX(20), []int{136, 62, 60, 120, 280, 110}, []int{96, 52, 50, 90, 120, 80}, 4)
	for i, colW := range widths {
		w.evtList.Col(i).SetWidth(colW)
	}
}

func (w *appWindow) updateHistoryColumns(table *ui.ListView, listWidth int) {
	if table == nil || table.ColCount() < 7 {
		return
	}
	widths := fitColumnsToWidth(listWidth, ui.DpiX(20), []int{136, 62, 58, 110, 480, 110, 96}, []int{96, 52, 50, 90, 130, 80, 80}, 4)
	for i, colW := range widths {
		table.Col(i).SetWidth(colW)
	}
}

func fitColumnsToWidth(clientWidth, padding int, base, min []int, flexIdx int) []int {
	if len(base) == 0 || len(base) != len(min) {
		return append([]int(nil), base...)
	}
	available := clientWidth - padding
	if available < len(base)*18 {
		available = len(base) * 18
	}
	widths := append([]int(nil), base...)
	baseSum := sumInts(base)
	minSum := sumInts(min)
	if baseSum <= 0 {
		return widths
	}
	if available >= baseSum {
		return rebalanceColumns(widths, min, available, flexIdx)
	}
	if available <= minSum {
		return scaleColumns(min, available, 18)
	}
	needShrink := baseSum - available
	capacity := 0
	for i := range widths {
		capacity += maxInt(0, widths[i]-min[i])
	}
	if capacity <= 0 {
		return scaleColumns(widths, available, 18)
	}
	for i := range widths {
		canShrink := maxInt(0, widths[i]-min[i])
		shrink := needShrink * canShrink / capacity
		widths[i] -= shrink
		if widths[i] < min[i] {
			widths[i] = min[i]
		}
	}
	return rebalanceColumns(widths, min, available, flexIdx)
}

func scaleColumns(values []int, target, floor int) []int {
	widths := append([]int(nil), values...)
	sum := sumInts(values)
	if sum <= 0 {
		return widths
	}
	for i := range widths {
		widths[i] = maxInt(floor, target*values[i]/sum)
	}
	return rebalanceColumns(widths, nil, target, len(widths)-1)
}

func rebalanceColumns(widths, min []int, target, preferred int) []int {
	if len(widths) == 0 {
		return widths
	}
	if preferred < 0 || preferred >= len(widths) {
		preferred = len(widths) - 1
	}
	diff := target - sumInts(widths)
	for diff != 0 {
		if diff > 0 {
			widths[preferred]++
			diff--
			continue
		}
		adjusted := false
		for i := 0; i < len(widths) && diff < 0; i++ {
			idx := (preferred + i) % len(widths)
			limit := 18
			if min != nil && idx < len(min) {
				limit = min[idx]
			}
			if widths[idx] > limit {
				widths[idx]--
				diff++
				adjusted = true
			}
		}
		if !adjusted {
			break
		}
	}
	return widths
}

func sumInts(items []int) int {
	total := 0
	for _, v := range items {
		total += v
	}
	return total
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (w *appWindow) showSelectedEventDetails() {
	selected := w.evtList.SelectedItems()
	if len(selected) == 0 {
		_, _ = w.wnd.Hwnd().MessageBox("Спочатку виберіть подію.", "CID Windigo", co.MB_ICONINFORMATION)
		return
	}
	it := selected[0]
	msg := fmt.Sprintf(
		"Час: %s\nППК: %s\nКод: %s\nТип: %s\nОпис: %s\nЗона: %s",
		it.Text(0),
		it.Text(1),
		it.Text(2),
		it.Text(3),
		it.Text(4),
		it.Text(5),
	)
	_, _ = w.wnd.Hwnd().MessageBox(msg, "Деталі події", co.MB_ICONINFORMATION)
}

func (w *appWindow) openSettingsModal(ctx context.Context) {
	cfg := w.rt.GetConfig()
	modal := ui.NewModal(
		w.wnd,
		ui.OptsModal().
			Title("Налаштування (Windigo)").
			Size(ui.Dpi(720, 470)),
	)

	ui.NewStatic(modal, ui.OptsStatic().Text("Server host").Position(ui.Dpi(18, 20)).Size(ui.Dpi(120, 22)))
	serverHost := ui.NewEdit(modal, ui.OptsEdit().Text(cfg.Server.Host).Position(ui.Dpi(140, 18)).Width(ui.DpiX(220)))
	ui.NewStatic(modal, ui.OptsStatic().Text("Server port").Position(ui.Dpi(380, 20)).Size(ui.Dpi(100, 22)))
	serverPort := ui.NewEdit(modal, ui.OptsEdit().Text(cfg.Server.Port).Position(ui.Dpi(482, 18)).Width(ui.DpiX(200)))

	ui.NewStatic(modal, ui.OptsStatic().Text("Client host").Position(ui.Dpi(18, 56)).Size(ui.Dpi(120, 22)))
	clientHost := ui.NewEdit(modal, ui.OptsEdit().Text(cfg.Client.Host).Position(ui.Dpi(140, 54)).Width(ui.DpiX(220)))
	ui.NewStatic(modal, ui.OptsStatic().Text("Client port").Position(ui.Dpi(380, 56)).Size(ui.Dpi(100, 22)))
	clientPort := ui.NewEdit(modal, ui.OptsEdit().Text(cfg.Client.Port).Position(ui.Dpi(482, 54)).Width(ui.DpiX(200)))

	ui.NewStatic(modal, ui.OptsStatic().Text("Queue buffer").Position(ui.Dpi(18, 92)).Size(ui.Dpi(120, 22)))
	queueBuffer := ui.NewEdit(modal, ui.OptsEdit().Text(strconv.Itoa(cfg.Queue.BufferSize)).Position(ui.Dpi(140, 90)).Width(ui.DpiX(220)))
	ui.NewStatic(modal, ui.OptsStatic().Text("PPK timeout").Position(ui.Dpi(380, 92)).Size(ui.Dpi(100, 22)))
	ppkTimeout := ui.NewEdit(modal, ui.OptsEdit().Text(cfg.Monitoring.PpkTimeout).Position(ui.Dpi(482, 90)).Width(ui.DpiX(200)))

	ui.NewStatic(modal, ui.OptsStatic().Text("Log level").Position(ui.Dpi(18, 128)).Size(ui.Dpi(120, 22)))
	logLevel := ui.NewComboBox(
		modal,
		ui.OptsComboBox().
			Position(ui.Dpi(140, 126)).
			Width(ui.DpiX(220)).
			Texts("trace", "debug", "info", "warn", "error", "fatal"),
	)
	logLevel.SelectIndex(indexOf(strings.ToLower(cfg.Logging.Level), []string{"trace", "debug", "info", "warn", "error", "fatal"}))

	startMin := ui.NewCheckBox(
		modal,
		ui.OptsCheckBox().
			Text("Start minimized").
			Position(ui.Dpi(18, 170)).
			Size(ui.Dpi(180, 24)).
			State(boolToBst(cfg.UI.StartMinimized)),
	)
	minTray := ui.NewCheckBox(
		modal,
		ui.OptsCheckBox().
			Text("Minimize to tray").
			Position(ui.Dpi(210, 170)).
			Size(ui.Dpi(180, 24)).
			State(boolToBst(cfg.UI.MinimizeToTray)),
	)
	closeTray := ui.NewCheckBox(
		modal,
		ui.OptsCheckBox().
			Text("Close to tray").
			Position(ui.Dpi(402, 170)).
			Size(ui.Dpi(160, 24)).
			State(boolToBst(cfg.UI.CloseToTray)),
	)

	status := ui.NewStatic(modal, ui.OptsStatic().Text(" ").Position(ui.Dpi(18, 410)).Size(ui.Dpi(500, 22)))
	btnSave := ui.NewButton(modal, ui.OptsButton().Text("&Зберегти").Position(ui.Dpi(528, 406)).Width(ui.DpiX(90)))
	btnReset := ui.NewButton(modal, ui.OptsButton().Text("&Скинути").Position(ui.Dpi(624, 406)).Width(ui.DpiX(80)))

	save := func() {
		next := cfg
		next.Server.Host = strings.TrimSpace(serverHost.Text())
		next.Server.Port = strings.TrimSpace(serverPort.Text())
		next.Client.Host = strings.TrimSpace(clientHost.Text())
		next.Client.Port = strings.TrimSpace(clientPort.Text())
		next.Queue.BufferSize = atoiOr(next.Queue.BufferSize, queueBuffer.Text())
		next.Monitoring.PpkTimeout = strings.TrimSpace(ppkTimeout.Text())
		next.Logging.Level = strings.ToLower(strings.TrimSpace(logLevel.CurrentText()))
		next.UI.StartMinimized = startMin.IsChecked()
		next.UI.MinimizeToTray = minTray.IsChecked()
		next.UI.CloseToTray = closeTray.IsChecked()
		config.Normalize(&next)

		go func() {
			if err := w.rt.SaveConfig(ctx, next); err != nil {
				modal.UiThread(func() { status.SetTextAndResize("Помилка збереження: " + err.Error()) })
				return
			}
			cfg = w.rt.GetConfig()
			modal.UiThread(func() {
				status.SetTextAndResize("Налаштування збережено")
				w.activityTO = core.ParseDuration(cfg.Monitoring.PpkTimeout, 15*time.Minute)
				w.refreshFromBackend(ctx)
				w.refreshUI()
			})
		}()
	}

	btnSave.On().BnClicked(save)
	btnReset.On().BnClicked(func() {
		serverHost.SetText(cfg.Server.Host)
		serverPort.SetText(cfg.Server.Port)
		clientHost.SetText(cfg.Client.Host)
		clientPort.SetText(cfg.Client.Port)
		queueBuffer.SetText(strconv.Itoa(cfg.Queue.BufferSize))
		ppkTimeout.SetText(cfg.Monitoring.PpkTimeout)
		logLevel.SelectIndex(indexOf(strings.ToLower(cfg.Logging.Level), []string{"trace", "debug", "info", "warn", "error", "fatal"}))
		startMin.SetCheck(cfg.UI.StartMinimized)
		minTray.SetCheck(cfg.UI.MinimizeToTray)
		closeTray.SetCheck(cfg.UI.CloseToTray)
		status.SetTextAndResize("Значення скинуто")
	})

	modal.On().WmClose(func() { _ = modal.Hwnd().DestroyWindow() })
	modal.ShowModal()
}

func firstNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func atoiOr(def int, s string) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return v
}

func indexOf(v string, items []string) int {
	for i, it := range items {
		if strings.EqualFold(v, it) {
			return i
		}
	}
	return 0
}

func boolToBst(v bool) co.BST {
	if v {
		return co.BST_CHECKED
	}
	return co.BST_UNCHECKED
}

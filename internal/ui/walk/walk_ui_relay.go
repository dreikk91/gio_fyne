//go:build windows

package walk

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"cid_fyne/internal/core"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/rs/zerolog/log"
)

type rfObjectRow struct {
	ID       int
	Display  string
	Selected bool
}

type rfCodeRow struct {
	Code        string
	Type        string
	Description string
	Category    string
	Selected    bool
}

type rfSummaryRow struct {
	ID            int
	Display       string
	Global        bool
	SpecificCodes string
}

type relayFilterDialog struct {
	app *walkApp
	dlg *walk.Dialog

	enabled *walk.CheckBox
	groups  *walk.LineEdit

	objSearch *walk.LineEdit
	objTable  *walk.TableView
	objModel  *rfObjectTableModel

	codeSearch   *walk.LineEdit
	codeFilter   *walk.ComboBox
	codeTable    *walk.TableView
	codeModel    *rfCodeTableModel
	categoryBtns map[string]*walk.PushButton

	tabs        *walk.TabWidget
	summaryTable *walk.TableView
	summaryModel *rfSummaryTableModel

	statusLabel *walk.Label

	rule          core.RelayFilterRule
	objects       []*rfObjectRow
	codes         []*rfCodeRow
	filteredObjs  []*rfObjectRow
	filteredCodes []*rfCodeRow
	summary       []rfSummaryRow

	selectedObjs  map[int]bool
	selectedCodes map[string]bool
	
	busy    atomic.Bool
	closed  atomic.Bool
	lastCat string
}

func (a *walkApp) openRelayFilter() {
	if a.mw == nil {
		log.Warn().Msg("Cannot open Relay Filter: main window is nil")
		return
	}
	
	log.Info().Msg("Fetching relay filter rules from backend...")
	go func() {
		rule, err := a.rt.GetRelayFilterRule(a.ctx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get relay filter rules")
			a.mw.Synchronize(func() {
				walk.MsgBox(a.mw, "Фільтр реле", "Помилка завантаження правил: "+err.Error(), walk.MsgBoxIconError)
			})
			return
		}
		
		log.Info().Msg("Opening Relay Filter dialog...")
		a.mw.Synchronize(func() {
			d := &relayFilterDialog{
				app:      a,
				rule:     rule,
				busy:     atomic.Bool{},
				closed:   atomic.Bool{},
				lastCat:  "all",
				selectedObjs:  make(map[int]bool),
				selectedCodes: make(map[string]bool),
			}
			d.initData()
			if err := d.run(); err != nil {
				log.Error().Err(err).Msg("Failed to run Relay Filter dialog")
				walk.MsgBox(a.mw, "Фільтр реле", "Помилка створення вікна: "+err.Error(), walk.MsgBoxIconError)
				return
			}
			log.Info().Msg("Running Relay Filter dialog...")
			d.dlg.Run()
		})
	}()
}

func (d *relayFilterDialog) initData() {
	// Objects
	devices := d.app.rt.GetDevices()
	d.objects = make([]*rfObjectRow, 0, len(devices))
	globalIDs := make(map[int]bool)
	for _, id := range d.rule.ObjectIDs {
		globalIDs[id] = true
	}
	
	for _, dev := range devices {
		row := &rfObjectRow{
			ID:       dev.ID,
			Display:  fmt.Sprintf("%03d | %s", dev.ID, firstNonEmpty(dev.ClientAddr, "Disconnected")),
			Selected: globalIDs[dev.ID],
		}
		d.objects = append(d.objects, row)
		if row.Selected {
			d.selectedObjs[row.ID] = true
		}
	}

	sort.Slice(d.objects, func(i, j int) bool {
		return d.objects[i].ID < d.objects[j].ID
	})
	
	// Codes
	events := d.app.rt.GetEventList()
	sort.Slice(events, func(i, j int) bool {
		return events[i].ContactIDCode < events[j].ContactIDCode
	})
	
	d.codes = make([]*rfCodeRow, 0, len(events))
	seen := make(map[string]bool)
	for _, e := range events {
		code := strings.ToUpper(e.ContactIDCode)
		if seen[code] {
			continue
		}
		seen[code] = true
		cat := core.Classify(e.ContactIDCode, e.TypeCodeMesUK, e.CodeMesUK)
		d.codes = append(d.codes, &rfCodeRow{
			Code:        code,
			Type:        e.TypeCodeMesUK,
			Description: e.CodeMesUK,
			Category:    cat,
		})
	}
	
	d.syncCodesToSelection()
	d.updateFiltered()
	d.rebuildSummary()
}

func (d *relayFilterDialog) syncCodesToSelection() {
	selIDs := d.getSelectedObjIDs()
	
	if len(selIDs) == 0 {
		blocked := make(map[string]bool)
		for _, c := range d.rule.Codes {
			blocked[strings.ToUpper(c)] = true
		}
		for i := range d.codes {
			d.codes[i].Selected = blocked[d.codes[i].Code]
		}
	} else if len(selIDs) == 1 {
		id := selIDs[0]
		blocked := make(map[string]bool)
		if codes, ok := d.rule.ObjectCodes[id]; ok {
			for _, c := range codes {
				blocked[strings.ToUpper(c)] = true
			}
		}
		for i := range d.codes {
			d.codes[i].Selected = blocked[d.codes[i].Code]
		}
	} else {
		for i := range d.codes {
			code := d.codes[i].Code
			allHave := true
			for _, id := range selIDs {
				found := false
				if codes, ok := d.rule.ObjectCodes[id]; ok {
					for _, c := range codes {
						if strings.EqualFold(c, code) {
							found = true
							break
						}
					}
				}
				if !found {
					allHave = false
					break
				}
			}
			d.codes[i].Selected = allHave
		}
	}
	
	d.selectedCodes = make(map[string]bool)
	for _, c := range d.codes {
		if c.Selected {
			d.selectedCodes[c.Code] = true
		}
	}
}

func (d *relayFilterDialog) getSelectedObjIDs() []int {
	var ids []int
	for id, sel := range d.selectedObjs {
		if sel {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids
}

func (d *relayFilterDialog) updateFiltered() {
	objQ := ""
	if d.objSearch != nil {
		objQ = strings.ToLower(d.objSearch.Text())
	}
	d.filteredObjs = d.filteredObjs[:0]
	for _, it := range d.objects {
		if objQ == "" || strings.Contains(strings.ToLower(it.Display), objQ) {
			d.filteredObjs = append(d.filteredObjs, it)
		}
	}
	
	codeQ := ""
	if d.codeSearch != nil {
		codeQ = strings.ToLower(d.codeSearch.Text())
	}
	cat := d.lastCat
	d.filteredCodes = d.filteredCodes[:0]
	for _, it := range d.codes {
		catMatch := cat == "all" || strings.EqualFold(it.Category, cat)
		if catMatch && (codeQ == "" || 
			strings.Contains(strings.ToLower(it.Code), codeQ) ||
			strings.Contains(strings.ToLower(it.Type), codeQ) ||
			strings.Contains(strings.ToLower(it.Description), codeQ)) {
			d.filteredCodes = append(d.filteredCodes, it)
		}
	}
}

func (d *relayFilterDialog) rebuildSummary() {
	d.summary = d.summary[:0]
	for _, it := range d.objects {
		isGlobal := it.Selected
		specific := "-"
		if codes, ok := d.rule.ObjectCodes[it.ID]; ok && len(codes) > 0 {
			sort.Strings(codes)
			limit := 40
			items := codes
			if len(codes) > limit {
				items = codes[:limit]
			}
			specific = strings.Join(items, ", ")
			if len(codes) > limit {
				specific += fmt.Sprintf("... +%d", len(codes)-limit)
			}
		}
		
		if isGlobal || specific != "-" {
			d.summary = append(d.summary, rfSummaryRow{
				ID:            it.ID,
				Display:       it.Display,
				Global:        isGlobal,
				SpecificCodes: specific,
			})
		}
	}
}

func (d *relayFilterDialog) run() error {
	d.objModel = &rfObjectTableModel{d: d}
	d.codeModel = &rfCodeTableModel{d: d}
	d.summaryModel = &rfSummaryTableModel{d: d}
	
	return Dialog{
		AssignTo: &d.dlg,
		Title:    "Налаштування фільтрації реле",
		MinSize:  Size{Width: 900, Height: 650},
		Layout:   VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 10},
		Children: []Widget{
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 10},
				Children: []Widget{
					CheckBox{AssignTo: &d.enabled, Text: "Увімкнути фільтрацію", Checked: d.rule.Enabled},
					HSpacer{},
					Label{Text: "Групи (напр. 01, 05):"},
					LineEdit{AssignTo: &d.groups, MinSize: Size{Width: 150}, Text: strings.Join(intsToStrings(d.rule.GroupNumbers), ", ")},
				},
			},
			TabWidget{
				AssignTo: &d.tabs,
				Pages: []TabPage{
					{
						Title: "Правила фільтрації",
						Layout: HBox{Margins: Margins{Top: 8}, Spacing: 10},
						Children: []Widget{
							// Objects Pane
							Composite{
								Layout: VBox{MarginsZero: true, Spacing: 6},
								Children: []Widget{
									Label{Text: "1. Виберіть об'єкти", Font: Font{Bold: true}},
									LineEdit{
										AssignTo:  &d.objSearch,
										CueBanner: "Пошук ID/Інфо...",
										OnTextChanged: func() {
											d.updateFiltered()
											d.objModel.PublishRowsReset()
										},
									},
									TableView{
										AssignTo:         &d.objTable,
										AlternatingRowBG: true,
										CheckBoxes:       true,
										ColumnsOrderable: true,
										MultiSelection:   true,
										Model:            d.objModel,
										Columns: []TableViewColumn{
											{Title: "ID", Width: 60},
											{Title: "Інформація про об'єкт", Width: 250},
										},
										OnItemActivated: func() {
											d.handleObjectSelection()
										},
									},
									Composite{
										Layout: HBox{MarginsZero: true, Spacing: 8},
										Children: []Widget{
											PushButton{Text: "Усі", OnClicked: func() { d.selectAllObjs(true) }},
											PushButton{Text: "Нічого", OnClicked: func() { d.selectAllObjs(false) }},
										},
									},
								},
							},
							// Codes Pane
							Composite{
								Layout: VBox{MarginsZero: true, Spacing: 6},
								Children: []Widget{
									Label{Text: "2. Виберіть коди для блокування", Font: Font{Bold: true}},
									LineEdit{
										AssignTo:  &d.codeSearch,
										CueBanner: "Пошук коду/типу/опису...",
										OnTextChanged: func() {
											d.updateFiltered()
											d.codeModel.PublishRowsReset()
											if d.codeTable != nil {
												d.codeTable.Invalidate()
											}
										},
									},
									Composite{
										Layout: HBox{MarginsZero: true, Spacing: 4},
										Children: d.createCategoryButtons(),
									},
									TableView{
										AssignTo:         &d.codeTable,
										AlternatingRowBG: true,
										CheckBoxes:       true,
										ColumnsOrderable: true,
										Model:            d.codeModel,
										Columns: []TableViewColumn{
											{Title: "Код", Width: 60},
											{Title: "Тип", Width: 100},
											{Title: "Опис події", Width: 300},
										},
										StyleCell: func(style *walk.CellStyle) {
											row := style.Row()
											if row < 0 || row >= len(d.filteredCodes) {
												return
											}
											it := d.filteredCodes[row]
											bg, fg := priorityColors(d.app, it.Category, row)
											style.BackgroundColor = bg
											style.TextColor = fg
										},
										OnItemActivated: func() {
											d.handleCodeSelection()
										},
									},
									Composite{
										Layout: HBox{MarginsZero: true, Spacing: 8},
										Children: []Widget{
											PushButton{Text: "Усі вибрані", OnClicked: func() { d.selectAllCodes(true) }},
											PushButton{Text: "Нічого", OnClicked: func() { d.selectAllCodes(false) }},
										},
									},
								},
							},
						},
					},
					{
						Title: "Зведення (Summary)",
						Layout: VBox{Margins: Margins{Top: 8}, Spacing: 6},
						Children: []Widget{
							TableView{
								AssignTo:         &d.summaryTable,
								AlternatingRowBG: true,
								Model:            d.summaryModel,
								Columns: []TableViewColumn{
									{Title: "ID", Width: 50},
									{Title: "Об'єкт", Width: 150},
									{Title: "Глобально", Width: 80},
									{Title: "Специфічні заблоковані коди", Width: 550},
								},
							},
						},
					},
				},
			},
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 10},
				Children: []Widget{
					Label{AssignTo: &d.statusLabel, Text: d.statusDesc(), StretchFactor: 1},
					PushButton{Text: "Застосувати та зберегти", OnClicked: d.save},
					PushButton{Text: "Скасувати", OnClicked: func() { d.dlg.Close(walk.DlgCmdCancel) }},
				},
			},
		},
	}.Create(d.app.mw)
}

func (d *relayFilterDialog) createCategoryButtons() []Widget {
	btns := make([]Widget, 0, len(eventFilters))
	d.categoryBtns = make(map[string]*walk.PushButton)
	for _, cat := range eventFilters {
		c := cat
		btns = append(btns, PushButton{
			Text: strings.ToUpper(c),
			OnClicked: func() {
				d.lastCat = c
				d.updateFiltered()
				d.codeModel.PublishRowsReset()
				if d.codeTable != nil {
					d.codeTable.Invalidate()
				}
			},
		})
	}
	return btns
}

func (d *relayFilterDialog) statusDesc() string {
	sel := len(d.selectedObjs)
	if sel == 0 {
		return "Редагування глобальних кодів (для всіх об'єктів)"
	}
	if sel == 1 {
		var name string
		for id := range d.selectedObjs {
			for _, o := range d.objects {
				if o.ID == id {
					name = o.Display
					break
				}
			}
		}
		return fmt.Sprintf("Редагування кодів для об'єкта %s", name)
	}
	return fmt.Sprintf("Редагування кодів для %d вибраних об'єктів", sel)
}

func (d *relayFilterDialog) handleObjectSelection() {
	// Information updated via model.SetChecked
	d.syncCodesToSelection()
	d.codeModel.PublishRowsReset()
	d.statusLabel.SetText(d.statusDesc())
	d.rebuildSummary()
	d.summaryModel.PublishRowsReset()
}

func (d *relayFilterDialog) handleCodeSelection() {
	// Information updated via model.SetChecked
	d.applyCodesToRule()
	d.rebuildSummary()
	d.summaryModel.PublishRowsReset()
}

func (d *relayFilterDialog) indexOfObj(it *rfObjectRow) int {
	for i, o := range d.filteredObjs {
		if o == it {
			return i
		}
	}
	return -1
}

func (d *relayFilterDialog) indexOfCode(it *rfCodeRow) int {
	for i, c := range d.filteredCodes {
		if c == it {
			return i
		}
	}
	return -1
}

func (d *relayFilterDialog) selectAllObjs(sel bool) {
	for _, it := range d.filteredObjs {
		if sel {
			d.selectedObjs[it.ID] = true
		} else {
			delete(d.selectedObjs, it.ID)
		}
	}
	d.objModel.PublishRowsReset()
	if d.objTable != nil {
		d.objTable.Invalidate()
	}
	d.handleObjectSelection()
}

func (d *relayFilterDialog) selectAllCodes(sel bool) {
	for _, it := range d.filteredCodes {
		if sel {
			d.selectedCodes[it.Code] = true
		} else {
			delete(d.selectedCodes, it.Code)
		}
	}
	d.codeModel.PublishRowsReset()
	if d.codeTable != nil {
		d.codeTable.Invalidate()
	}
	d.handleCodeSelection()
}

func (d *relayFilterDialog) applyCodesToRule() {
	selCodes := make([]string, 0, len(d.selectedCodes))
	for c, ok := range d.selectedCodes {
		if ok {
			selCodes = append(selCodes, c)
		}
	}
	sort.Strings(selCodes)
	
	selIDs := d.getSelectedObjIDs()
	if len(selIDs) == 0 {
		d.rule.Codes = selCodes
	} else {
		for _, id := range selIDs {
			if d.rule.ObjectCodes == nil {
				d.rule.ObjectCodes = make(map[int][]string)
			}
			d.rule.ObjectCodes[id] = append([]string{}, selCodes...)
		}
	}
}

func (d *relayFilterDialog) save() {
	if d.busy.Swap(true) {
		return
	}
	
	d.rule.Enabled = d.enabled.Checked()
	d.rule.GroupNumbers = parseGroupsLine(d.groups.Text())
	d.rule.ObjectIDs = d.getSelectedObjIDs()
	
	go func() {
		err := d.app.rt.SaveRelayFilterRule(d.app.ctx, d.rule)
		d.app.mw.Synchronize(func() {
			d.busy.Store(false)
			if err != nil {
				walk.MsgBox(d.dlg, "Збереження", "Помилка: "+err.Error(), walk.MsgBoxIconError)
				return
			}
			d.dlg.Close(walk.DlgCmdOK)
		})
	}()
}

// Table Models

type rfObjectTableModel struct {
	walk.TableModelBase
	d *relayFilterDialog
}

func (m *rfObjectTableModel) RowCount() int {
	return len(m.d.filteredObjs)
}

func (m *rfObjectTableModel) Value(row, col int) interface{} {
	it := m.d.filteredObjs[row]
	switch col {
	case 0:
		return fmt.Sprintf("%03d", it.ID)
	case 1:
		return it.Display
	}
	return nil
}

func (m *rfObjectTableModel) Checked(row int) bool {
	return m.d.selectedObjs[m.d.filteredObjs[row].ID]
}

func (m *rfObjectTableModel) SetChecked(row int, checked bool) error {
	it := m.d.filteredObjs[row]
	if checked {
		m.d.selectedObjs[it.ID] = true
	} else {
		delete(m.d.selectedObjs, it.ID)
	}
	m.d.handleObjectSelection()
	return nil
}

type rfCodeTableModel struct {
	walk.TableModelBase
	d *relayFilterDialog
}

func (m *rfCodeTableModel) RowCount() int {
	return len(m.d.filteredCodes)
}

func (m *rfCodeTableModel) Value(row, col int) interface{} {
	it := m.d.filteredCodes[row]
	switch col {
	case 0:
		return it.Code
	case 1:
		return it.Type
	case 2:
		return it.Description
	}
	return nil
}

func (m *rfCodeTableModel) Checked(row int) bool {
	return m.d.selectedCodes[m.d.filteredCodes[row].Code]
}

func (m *rfCodeTableModel) SetChecked(row int, checked bool) error {
	it := m.d.filteredCodes[row]
	if checked {
		m.d.selectedCodes[it.Code] = true
	} else {
		delete(m.d.selectedCodes, it.Code)
	}
	m.d.handleCodeSelection()
	return nil
}

type rfSummaryTableModel struct {
	walk.TableModelBase
	d *relayFilterDialog
}

func (m *rfSummaryTableModel) RowCount() int {
	return len(m.d.summary)
}

func (m *rfSummaryTableModel) Value(row, col int) interface{} {
	it := m.d.summary[row]
	switch col {
	case 0:
		return fmt.Sprintf("%03d", it.ID)
	case 1:
		return it.Display
	case 2:
		return boolText(it.Global, "Так", "-")
	case 3:
		return it.SpecificCodes
	}
	return nil
}

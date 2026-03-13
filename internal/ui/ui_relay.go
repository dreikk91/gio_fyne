package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"cid_gio_gio/internal/core"
	"github.com/rs/zerolog/log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func (m *model) openRelayFilter() {
	m.rfBusy.Store(true)
	go func() {
		rule, err := m.rt.GetRelayFilterRule(context.Background())
		m.rfResult <- rfResult{rule, err}
	}()
}

func (m *model) openRelayFilterWindow() {
	if m.rfWin == nil {
		m.rfWin = m.app.NewWindow("Relay Filter Configuration")
		m.buildRelayFilterUI()
		m.rfWin.SetCloseIntercept(func() {
			m.rfOpen = false
			m.rfWin.Hide()
		})
	}
	m.rfOpen = true
	m.rfWin.Show()
}

func (m *model) buildRelayFilterUI() {
	m.rfEnabled = widget.NewCheck("Enable filtering", nil)
	m.rfGroups = widget.NewEntry()
	m.rfGroups.SetPlaceHolder("Groups (e.g. 01, 05)")

	header := container.NewBorder(nil, nil,
		container.NewVBox(
			canvas.NewText("Relay Filter Configuration", cAccent),
			widget.NewLabel(""),
		),
		container.NewHBox(m.rfEnabled, layout.NewSpacer(), m.rfGroups),
		layout.NewSpacer(),
	)

	m.rfObjQuery = widget.NewEntry()
	m.rfObjQuery.SetPlaceHolder("Search ID/IP/Info...")
	m.rfObjQuery.OnChanged = func(string) { m.updateRfFilters(); m.refreshRelayUI() }
	m.rfCodeQuery = widget.NewEntry()
	m.rfCodeQuery.SetPlaceHolder("Search Code/Type/Desc...")
	m.rfCodeQuery.OnChanged = func(string) { m.updateRfFilters(); m.refreshRelayUI() }

	m.rfSelectAllObjs = widget.NewButton("Select All", func() {
		for i := range m.rfFilteredObjs {
			m.rfFilteredObjs[i].Selected = true
		}
		m.rfCheckObjectSelectionChanges()
		m.rfSyncCodesPaneToSelectedObjects()
		m.rebuildRfSummary()
		m.refreshRelayUI()
	})
	m.rfClearObjs = widget.NewButton("Clear", func() {
		for i := range m.rfObjects {
			m.rfObjects[i].Selected = false
		}
		m.rfCheckObjectSelectionChanges()
		m.rfSyncCodesPaneToSelectedObjects()
		m.rebuildRfSummary()
		m.refreshRelayUI()
	})
	m.rfSelectAllCodes = widget.NewButton("Select All", func() {
		for i := range m.rfFilteredCd {
			m.rfFilteredCd[i].Selected = true
		}
		m.rfCheckCodeSelectionChanges()
		m.rfApplyCodesToSelectedObjects()
		m.rebuildRfSummary()
		m.refreshRelayUI()
	})
	m.rfClearCodes = widget.NewButton("Clear", func() {
		for i := range m.rfFilteredCd {
			m.rfFilteredCd[i].Selected = false
		}
		m.rfCheckCodeSelectionChanges()
		m.rfApplyCodesToSelectedObjects()
		m.rebuildRfSummary()
		m.refreshRelayUI()
	})

	m.rfObjList = widget.NewList(
		func() int { return len(m.rfFilteredObjs) },
		func() fyne.CanvasObject {
			return newRelayObjCell()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(m.rfFilteredObjs) {
				return
			}
			row := m.rfFilteredObjs[id]
			cell := obj.(*relayObjCell)
			cell.label.SetText(row.Display)
			cell.check.OnChanged = func(v bool) {
				row.Selected = v
				if m.rfCheckObjectSelectionChanges() {
					m.rfSyncCodesPaneToSelectedObjects()
				}
				m.rebuildRfSummary()
				m.refreshRelayUI()
			}
			cell.check.SetChecked(row.Selected)
		},
	)

	m.rfCodeList = widget.NewList(
		func() int { return len(m.rfFilteredCd) },
		func() fyne.CanvasObject {
			return newRelayCodeCell()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(m.rfFilteredCd) {
				return
			}
			row := m.rfFilteredCd[id]
			cell := obj.(*relayCodeCell)
			cell.label.SetText(fmt.Sprintf("%s | %s", row.Code, row.Description))
			cell.check.OnChanged = func(v bool) {
				row.Selected = v
				if m.rfCheckCodeSelectionChanges() {
					m.rfApplyCodesToSelectedObjects()
				}
				m.rebuildRfSummary()
				m.refreshRelayUI()
			}
			cell.check.SetChecked(row.Selected)
			cell.config.OnTapped = func() {
				m.openRfDetail(row.Code)
			}
			if row.Category != "alarm" {
				cell.config.Hide()
			} else {
				cell.config.Show()
			}
		},
	)

	m.rfSumList = widget.NewList(
		func() int { return len(m.rfSummary) },
		func() fyne.CanvasObject {
			cols := []fyne.CanvasObject{
				widget.NewLabel(""),
				widget.NewLabel(""),
				widget.NewLabel(""),
				widget.NewLabel(""),
			}
			return container.NewGridWithColumns(4, cols...)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(m.rfSummary) {
				return
			}
			r := m.rfSummary[id]
			box := obj.(*fyne.Container)
			box.Objects[0].(*widget.Label).SetText(fmt.Sprintf("%03d", r.ID))
			box.Objects[1].(*widget.Label).SetText(r.Display)
			box.Objects[2].(*widget.Label).SetText(boolText(r.Global, "Yes", "-"))
			box.Objects[3].(*widget.Label).SetText(r.SpecificCodes)
		},
	)

	objPane := cardContainer(cPanel, container.NewBorder(
		container.NewVBox(
			canvas.NewText("Select Objects", cText),
			m.rfObjQuery,
			container.NewHBox(m.rfSelectAllObjs, m.rfClearObjs),
			vGap(6),
		),
		nil, nil, nil,
		m.rfObjList,
	))
	codePane := cardContainer(cPanel, container.NewBorder(
		container.NewVBox(
			canvas.NewText("Select Blocked Codes", cText),
			m.rfCodeQuery,
			container.NewHBox(m.rfSelectAllCodes, m.rfClearCodes),
			vGap(6),
		),
		nil, nil, nil,
		m.rfCodeList,
	))

	rulesTab := container.NewGridWithColumns(2, objPane, codePane)
	summaryTab := cardContainer(cPanel, container.NewBorder(
		newTableHeader([]string{"ID", "Object", "Global", "Specific Blocked Codes"}),
		nil, nil, nil,
		m.rfSumList,
	))

	m.rfTabs = container.NewAppTabs(
		container.NewTabItem("Filter Rules", rulesTab),
		container.NewTabItem("Summary View", summaryTab),
	)

	m.rfSaveBtn = widget.NewButton("Apply & Save", func() { m.saveRelayFilter() })
	m.rfCancelBtn = widget.NewButton("Cancel", func() { m.rfWin.Hide(); m.rfOpen = false })

	m.rfStatusLabel = widget.NewLabel(m.rfStatusDesc())
	footer := container.NewBorder(nil, nil,
		canvas.NewText("", cSoft),
		container.NewHBox(m.rfSaveBtn, m.rfCancelBtn),
		m.rfStatusLabel,
	)

	content := container.NewVBox(
		cardContainer(cPanel, header),
		layout.NewSpacer(),
		m.rfTabs,
		layout.NewSpacer(),
		footer,
	)

	m.rfWin.SetContent(content)
	m.rfWin.Resize(fyne.NewSize(1100, 720))
	m.updateRfFilters()
	m.refreshRelayUI()
}

func (m *model) openRfDetail(code string) {
	m.rfDetailCode = code
	m.rfDetailObj = 0
	ids := []int{}
	for id, sel := range m.rfSelectedObjs {
		if sel {
			ids = append(ids, id)
		}
	}
	if len(ids) == 1 {
		m.rfDetailObj = ids[0]
	}

	var det core.RelayFilterDetail
	if m.rfDetailObj == 0 {
		det = m.rfRule.CodeDetails[m.rfDetailCode]
	} else {
		if m.rfRule.ObjCodeDetails[m.rfDetailObj] != nil {
			det = m.rfRule.ObjCodeDetails[m.rfDetailObj][m.rfDetailCode]
		}
	}

	m.rfDetailZones = widget.NewEntry()
	m.rfDetailZones.SetText(strings.Join(intsToStrings(det.Zones), ", "))
	m.rfDetailParts = widget.NewEntry()
	m.rfDetailParts.SetText(strings.Join(intsToStrings(det.Partitions), ", "))

	form := container.NewVBox(
		widget.NewLabel("Specific Zones (e.g. 1, 3, 10-20)"),
		m.rfDetailZones,
		widget.NewLabel("Specific Partitions (e.g. 1, 2)"),
		m.rfDetailParts,
	)
	d := dialog.NewCustomConfirm("Filter Details", "Apply", "Cancel", form, func(ok bool) {
		if ok {
			m.rfApplyDetail()
			m.rebuildRfSummary()
			m.refreshRelayUI()
		}
	}, m.rfWin)
	d.Show()
}

func (m *model) initRfRows() {
	devices := m.rt.GetDevices()
	log.Debug().Int("devices", len(devices)).Msg("relay filter devices loaded")
	m.rfObjects = make([]rfObjectRow, 0, len(devices))
	for _, d := range devices {
		m.rfObjects = append(m.rfObjects, rfObjectRow{
			ID:      d.ID,
			Display: fmt.Sprintf("%03d | %s", d.ID, firstNonEmpty(d.ClientAddr, "Disconnected")),
		})
	}

	events := m.rt.GetEventList()
	log.Debug().Int("events", len(events)).Msg("relay filter event catalog loaded")
	sort.Slice(events, func(i, j int) bool {
		return events[i].ContactIDCode < events[j].ContactIDCode
	})

	m.rfCodes = make([]rfCodeRow, 0, len(events))
	seen := make(map[string]bool)
	for _, e := range events {
		code := strings.ToUpper(e.ContactIDCode)
		if seen[code] {
			continue
		}
		seen[code] = true
		m.rfCodes = append(m.rfCodes, rfCodeRow{
			Code:        code,
			Type:        e.TypeCodeMesUK,
			Description: e.CodeMesUK,
			Category:    classifyEvent(code, e.TypeCodeMesUK, e.CodeMesUK),
		})
	}
}

func (m *model) loadRfRule(rule core.RelayFilterRule) {
	m.initRfRows()
	m.rfRule = rule
	if m.rfRule.ObjectCodes == nil {
		m.rfRule.ObjectCodes = make(map[int][]string)
	}
	if m.rfRule.CodeDetails == nil {
		m.rfRule.CodeDetails = make(map[string]core.RelayFilterDetail)
	}
	if m.rfRule.ObjCodeDetails == nil {
		m.rfRule.ObjCodeDetails = make(map[int]map[string]core.RelayFilterDetail)
	}
	if m.rfEnabled != nil {
		m.rfEnabled.SetChecked(rule.Enabled)
	}
	if m.rfGroups != nil {
		m.rfGroups.SetText(strings.Join(intsToStrings(rule.GroupNumbers), ", "))
	}

	globalObjs := make(map[int]bool)
	for _, id := range rule.ObjectIDs {
		globalObjs[id] = true
	}
	for i := range m.rfObjects {
		m.rfObjects[i].Selected = globalObjs[m.rfObjects[i].ID]
	}

	m.rfSelectedObjs = nil
	m.rfSelectedCodes = nil
	m.rfSyncCodesPaneToSelectedObjects()
	m.updateRfFilters()
	m.rebuildRfSummary()
}

func (m *model) updateRfFilters() {
	if m.rfObjQuery == nil || m.rfCodeQuery == nil {
		return
	}
	objQ := strings.ToLower(m.rfObjQuery.Text)
	m.rfFilteredObjs = m.rfFilteredObjs[:0]
	for i := range m.rfObjects {
		it := &m.rfObjects[i]
		if objQ == "" || strings.Contains(strings.ToLower(it.Display), objQ) {
			m.rfFilteredObjs = append(m.rfFilteredObjs, it)
		}
	}

	codeQ := strings.ToLower(m.rfCodeQuery.Text)
	m.rfFilteredCd = m.rfFilteredCd[:0]
	for i := range m.rfCodes {
		it := &m.rfCodes[i]
		if codeQ == "" ||
			strings.Contains(strings.ToLower(it.Code), codeQ) ||
			strings.Contains(strings.ToLower(it.Type), codeQ) ||
			strings.Contains(strings.ToLower(it.Description), codeQ) {
			m.rfFilteredCd = append(m.rfFilteredCd, it)
		}
	}
}

func (m *model) saveRelayFilter() {
	if m.rfBusy.CompareAndSwap(false, true) {
		rule := m.collectRfRule()
		go func() {
			err := m.rt.SaveRelayFilterRule(context.Background(), rule)
			m.rfResult <- rfResult{rule, err}
		}()
	}
}

func (m *model) collectRfRule() core.RelayFilterRule {
	rule := m.rfRule
	if m.rfEnabled != nil {
		rule.Enabled = m.rfEnabled.Checked
	}
	if m.rfGroups != nil {
		rule.GroupNumbers = parseGroupsLine(m.rfGroups.Text)
	}

	rule.ObjectIDs = []int{}
	for _, it := range m.rfObjects {
		if it.Selected {
			rule.ObjectIDs = append(rule.ObjectIDs, it.ID)
		}
	}
	return rule
}

func (m *model) rfStatusDesc() string {
	sel := 0
	for _, it := range m.rfObjects {
		if it.Selected {
			sel++
		}
	}
	if sel == 0 {
		return "Managing global blocked codes (for all active objects)"
	}
	if sel == 1 {
		for _, it := range m.rfObjects {
			if it.Selected {
				return fmt.Sprintf("Managing specific codes for Object %s", it.Display)
			}
		}
	}
	return fmt.Sprintf("Managing specific codes for %d selected objects", sel)
}

func (m *model) rfCheckObjectSelectionChanges() bool {
	changed := false
	if m.rfSelectedObjs == nil {
		m.rfSelectedObjs = make(map[int]bool)
		changed = true
	}
	current := make(map[int]bool)
	for _, it := range m.rfObjects {
		if it.Selected {
			current[it.ID] = true
		}
	}
	if len(current) != len(m.rfSelectedObjs) {
		changed = true
	} else {
		for id := range current {
			if !m.rfSelectedObjs[id] {
				changed = true
				break
			}
		}
	}
	if changed {
		m.rfSelectedObjs = current
	}
	return changed
}

func (m *model) rfCheckCodeSelectionChanges() bool {
	changed := false
	if m.rfSelectedCodes == nil {
		m.rfSelectedCodes = make(map[string]bool)
		changed = true
	}
	current := make(map[string]bool)
	for _, it := range m.rfCodes {
		if it.Selected {
			current[it.Code] = true
		}
	}
	if len(current) != len(m.rfSelectedCodes) {
		changed = true
	} else {
		for c := range current {
			if !m.rfSelectedCodes[c] {
				changed = true
				break
			}
		}
	}
	if changed {
		m.rfSelectedCodes = current
	}
	return changed
}

func (m *model) rfSyncCodesPaneToSelectedObjects() {
	selectedIDs := []int{}
	for id := range m.rfSelectedObjs {
		selectedIDs = append(selectedIDs, id)
	}

	if len(selectedIDs) == 0 {
		blocked := make(map[string]bool)
		for _, c := range m.rfRule.Codes {
			blocked[strings.ToUpper(c)] = true
		}
		for i := range m.rfCodes {
			m.rfCodes[i].Selected = blocked[m.rfCodes[i].Code]
		}
	} else if len(selectedIDs) == 1 {
		id := selectedIDs[0]
		blocked := make(map[string]bool)
		if codes, ok := m.rfRule.ObjectCodes[id]; ok {
			for _, c := range codes {
				blocked[strings.ToUpper(c)] = true
			}
		}
		for i := range m.rfCodes {
			m.rfCodes[i].Selected = blocked[m.rfCodes[i].Code]
		}
	} else {
		for i := range m.rfCodes {
			code := m.rfCodes[i].Code
			allHave := true
			for _, id := range selectedIDs {
				found := false
				if codes, ok := m.rfRule.ObjectCodes[id]; ok {
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
			m.rfCodes[i].Selected = allHave
		}
	}
	m.rfCheckCodeSelectionChanges()
}

func (m *model) rfApplyCodesToSelectedObjects() {
	selectedCodes := []string{}
	for c, sel := range m.rfSelectedCodes {
		if sel {
			selectedCodes = append(selectedCodes, c)
		}
	}
	sort.Strings(selectedCodes)

	selectedIDs := []int{}
	for id := range m.rfSelectedObjs {
		selectedIDs = append(selectedIDs, id)
	}

	if len(selectedIDs) == 0 {
		m.rfRule.Codes = selectedCodes
	} else {
		for _, id := range selectedIDs {
			m.rfRule.ObjectCodes[id] = append([]string{}, selectedCodes...)
		}
	}
}

func (m *model) rfApplyDetail() {
	det := core.RelayFilterDetail{
		Zones:      parseGroupsLine(m.rfDetailZones.Text),
		Partitions: parseGroupsLine(m.rfDetailParts.Text),
	}

	if m.rfDetailObj == 0 {
		if m.rfRule.CodeDetails == nil {
			m.rfRule.CodeDetails = make(map[string]core.RelayFilterDetail)
		}
		m.rfRule.CodeDetails[m.rfDetailCode] = det
	} else {
		for id, sel := range m.rfSelectedObjs {
			if sel {
				if m.rfRule.ObjCodeDetails == nil {
					m.rfRule.ObjCodeDetails = make(map[int]map[string]core.RelayFilterDetail)
				}
				if m.rfRule.ObjCodeDetails[id] == nil {
					m.rfRule.ObjCodeDetails[id] = make(map[string]core.RelayFilterDetail)
				}
				m.rfRule.ObjCodeDetails[id][m.rfDetailCode] = det
			}
		}
	}
}

func (m *model) rebuildRfSummary() {
	m.rfSummary = m.rfSummary[:0]
	devices := m.rfObjects
	rule := m.rfRule

	for _, d := range devices {
		isGlobal := false
		for _, id := range rule.ObjectIDs {
			if id == d.ID {
				isGlobal = true
				break
			}
		}
		specific := "-"
		if codes, ok := rule.ObjectCodes[d.ID]; ok && len(codes) > 0 {
			sort.Strings(codes)
			items := make([]string, 0, len(codes))
			for _, c := range codes {
				s := c
				if det, has := rule.ObjCodeDetails[d.ID][c]; has {
					z := ""
					if len(det.Zones) > 0 {
						z = fmt.Sprintf("Z:%v", det.Zones)
					}
					p := ""
					if len(det.Partitions) > 0 {
						p = fmt.Sprintf("P:%v", det.Partitions)
					}
					if z != "" || p != "" {
						s += "(" + strings.TrimSpace(z+" "+p) + ")"
					}
				}
				items = append(items, s)
			}
			specific = strings.Join(items, ", ")
		}

		if isGlobal || specific != "-" {
			m.rfSummary = append(m.rfSummary, rfSummaryRow{
				ID:            d.ID,
				Display:       d.Display,
				Global:        isGlobal,
				SpecificCodes: specific,
			})
		}
	}
}

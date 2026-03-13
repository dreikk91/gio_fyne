package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"sort"
	"strconv"
	"strings"

	"cid_gio_gio/internal/core"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func (m *model) relayFilterOverlay(gtx layout.Context) layout.Dimensions {
	fill(gtx, cOverlay)
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		w := min(gtx.Constraints.Max.X-gtx.Dp(20), gtx.Dp(1100))
		h := min(gtx.Constraints.Max.Y-gtx.Dp(20), gtx.Dp(720))
		if w < gtx.Dp(800) {
			w = max(gtx.Dp(600), gtx.Constraints.Max.X-gtx.Dp(8))
		}
		if h < gtx.Dp(500) {
			h = max(gtx.Dp(450), gtx.Constraints.Max.Y-gtx.Dp(8))
		}
		w = clamp(w, 1, gtx.Constraints.Max.X)
		h = clamp(h, 1, gtx.Constraints.Max.Y)
		inner := gtx
		inner.Constraints = layout.Exact(image.Pt(w, h))

		return layout.Stack{}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				return m.relayFilterPanel(inner)
			}),
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				if !m.rfDetailOpen {
					return layout.Dimensions{}
				}
				return m.rfDetailOverlay(gtx)
			}),
		)
	})
}

func (m *model) rfDetailOverlay(gtx layout.Context) layout.Dimensions {
	fill(gtx, cOverlay)
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		w := min(gtx.Constraints.Max.X-gtx.Dp(40), gtx.Dp(450))
		h := min(gtx.Constraints.Max.Y-gtx.Dp(40), gtx.Dp(380))
		inner := gtx
		inner.Constraints = layout.Exact(image.Pt(w, h))
		return m.rfDetailPanel(inner)
	})
}

func (m *model) rfDetailPanel(gtx layout.Context) layout.Dimensions {
	return modalCard(gtx, func(gtx layout.Context) layout.Dimensions {
		title := "Filter Details: " + m.rfDetailCode
		if m.rfDetailObj != 0 {
			title = fmt.Sprintf("Object %03d | %s", m.rfDetailObj, m.rfDetailCode)
		}

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.H6(m.th, title)
				l.Color = cAccent
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.Body2(m.th, "Specific Zones (e.g. 1, 3, 10-20)").Layout),
					layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return m.searchBar(gtx, &m.rfDetailZones, "All zones if empty...")
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
					layout.Rigid(material.Body2(m.th, "Specific Partitions (e.g. 1, 2)").Layout),
					layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return m.searchBar(gtx, &m.rfDetailParts, "All partitions if empty...")
					}),
				)
			}),
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return accentButton(gtx, m.th, &m.rfDetailSave, "Apply")
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return secondaryButton(gtx, m.th, &m.rfDetailClose, "Cancel")
					}),
				)
			}),
		)
	})
}

func (m *model) relayFilterPanel(gtx layout.Context) layout.Dimensions {
	// Sync interactions before layout to ensure latest state is used
	m.handleRfInteractions(gtx)

	return modalCard(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.H6(m.th, "Relay Filter Configuration")
						l.Color = cAccent
						return l.Layout(gtx)
					}),
					layout.Flexed(1, layout.Spacer{}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						c := material.CheckBox(m.th, &m.rfEnabled, "Enable Filtering")
						c.Color = cText
						c.IconColor = cAccent
						return c.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
					layout.Flexed(1, m.rfGroupsInput),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(m.rfTabBtn(0, "Filter Rules")),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(m.rfTabBtn(1, "Summary View")),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if m.rfTab == 0 {
					return m.rfRulesTab(gtx)
				}
				return m.rfSummaryTab(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Body2(m.th, m.rfStatusDesc())
						l.Color = cSoft
						return l.Layout(gtx)
					}),
					layout.Flexed(1, layout.Spacer{}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := "Apply & Save"
						if m.rfBusy.Load() {
							lbl = "Processing..."
						}
						return accentButton(gtx, m.th, &m.rfSave, lbl)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return secondaryButton(gtx, m.th, &m.rfCancel, "Cancel")
					}),
				)
			}),
		)
	})
}

func (m *model) rfGroupsInput(gtx layout.Context) layout.Dimensions {
	m.rfGroups.SingleLine = true
	e := material.Editor(m.th, &m.rfGroups, "Групи (наприклад 01, 05)")
	e.Color = cText
	e.HintColor = cSoft
	e.SelectionColor = cAccentSoft

	rowH := max(gtx.Dp(34), gtx.Dp(unit.Dp(int(m.th.TextSize)+18)))
	gtx.Constraints.Min.Y = rowH
	gtx.Constraints.Max.Y = rowH

	minW := min(gtx.Dp(120), gtx.Constraints.Max.X)
	if gtx.Constraints.Max.X < minW {
		minW = gtx.Constraints.Max.X
	}
	gtx.Constraints.Min.X = minW

	return editorSurface(gtx, e.Layout)
}

func (m *model) rfTabBtn(tab int, lbl string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		for m.rfTabs[tab].Clicked(gtx) {
			m.rfTab = tab
		}
		return tabBtn(gtx, m.th, &m.rfTabs[tab], lbl, m.rfTab == tab)
	}
}

func (m *model) rfRulesTab(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Flexed(1, m.rfObjectsPane),
		layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
		layout.Flexed(1, m.rfCodesPane),
	)
}

func (m *model) rfObjectsPane(gtx layout.Context) layout.Dimensions {
	return outlinedCard(gtx, cPanel, cBorder, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Body2(m.th, "Select Objects").Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return m.searchBar(gtx, &m.rfObjQuery, "Search ID/IP/Info...")
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return secondaryButton(gtx, m.th, &m.rfSelectAllObjs, "Select All")
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return secondaryButton(gtx, m.th, &m.rfClearObjs, "Clear")
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					list := material.List(m.th, &m.rfObjList)
					return list.Layout(gtx, len(m.rfFilteredObjs), func(gtx layout.Context, i int) layout.Dimensions {
						row := m.rfFilteredObjs[i]
						bg := firstColor(i%2 == 0, cPanel2, cPanel)
						return card(gtx, bg, unit.Dp(4), func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								cb := material.CheckBox(m.th, &row.Selected, "")
								cb.IconColor = cAccent
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(cb.Layout),
									layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
									layout.Rigid(material.Body2(m.th, row.Display).Layout),
								)
							})
						})
					})
				}),
			)
		})
	})
}

func (m *model) rfCodesPane(gtx layout.Context) layout.Dimensions {
	return outlinedCard(gtx, cPanel, cBorder, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.Body2(m.th, "Select Blocked Codes").Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return m.searchBar(gtx, &m.rfCodeQuery, "Search Code/Type/Desc...")
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return secondaryButton(gtx, m.th, &m.rfSelectAllCodes, "Select All")
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return secondaryButton(gtx, m.th, &m.rfClearCodes, "Clear")
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					list := material.List(m.th, &m.rfCodeList)
					return list.Layout(gtx, len(m.rfFilteredCd), func(gtx layout.Context, i int) layout.Dimensions {
						row := m.rfFilteredCd[i]
						bg := eventColor(row.Category, i)
						return card(gtx, bg, unit.Dp(4), func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								cb := material.CheckBox(m.th, &row.Selected, "")
								cb.IconColor = cAccent
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(cb.Layout),
									layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										l := material.Body2(m.th, fmt.Sprintf("%s | %s", row.Code, row.Description))
										l.TextSize = unit.Sp(12)
										return l.Layout(gtx)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if row.Category != "alarm" {
											return layout.Dimensions{}
										}
										b := material.Button(m.th, &row.Config, "⚙")
										b.Background = color.NRGBA{0, 0, 0, 0}
										b.Color = cAccent
										b.Inset = layout.UniformInset(unit.Dp(2))
										b.TextSize = unit.Sp(16)
										return b.Layout(gtx)
									}),
								)
							})
						})
					})
				}),
			)
		})
	})
}

func (m *model) rfSummaryTab(gtx layout.Context) layout.Dimensions {
	return outlinedCard(gtx, cPanel, cBorder, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return rowHeader(gtx, m.th, []string{"ID", "Object Display", "Global", "Specific Blocked Codes"})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					list := material.List(m.th, &m.rfSumList)
					return list.Layout(gtx, len(m.rfSummary), func(gtx layout.Context, i int) layout.Dimensions {
						r := m.rfSummary[i]
						isGlobal := "-"
						if r.Global {
							isGlobal = "Yes"
						}
						return m.rfSummaryRow(gtx, i, r.ID, r.Display, isGlobal, r.SpecificCodes)
					})
				}),
			)
		})
	})
}

func (m *model) rfSummaryRow(gtx layout.Context, i int, id int, display, global, codes string) layout.Dimensions {
	bg := firstColor(i%2 == 0, cPanel2, cPanel)
	weights := []float32{1, 4, 1.2, 8}
	cols := []string{strconv.Itoa(id), display, global, codes}

	return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return card(gtx, bg, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(cols)*2)
			for i, t := range cols {
				if i > 0 {
					children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
				}
				w, txt := weights[i], t
				children = append(children, layout.Flexed(w, func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(m.th, txt)
					l.Color = cText
					if i == 1 {
						l.Font.Weight = 600
					}
					// Allow multi-line for the last column (codes)
					if i < 3 {
						l.MaxLines = 1
					}
					return l.Layout(gtx)
				}))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

func (m *model) rfStatusDesc() string {
	sel := 0
	for _, it := range m.rfObjects {
		if it.Selected.Value {
			sel++
		}
	}
	if sel == 0 {
		return "Managing global blocked codes (for all active objects)"
	}
	if sel == 1 {
		for _, it := range m.rfObjects {
			if it.Selected.Value {
				return fmt.Sprintf("Managing specific codes for Object %s", it.Display)
			}
		}
	}
	return fmt.Sprintf("Managing specific codes for %d selected objects", sel)
}

func (m *model) handleRfInteractions(gtx layout.Context) {
	if !m.rfOpen {
		return
	}

	// Detect code selection changes and update current rule BEFORE sync
	if m.rfCheckCodeSelectionChanges() {
		m.rfApplyCodesToSelectedObjects()
	}

	// Detect object selection changes and sync codes list
	if m.rfCheckObjectSelectionChanges() {
		m.rfSyncCodesPaneToSelectedObjects()
	}

	for m.rfCancel.Clicked(gtx) {
		m.rfOpen = false
	}
	for m.rfSave.Clicked(gtx) {
		m.saveRelayFilter()
	}

	// Handle detail Config button clicks
	for i := range m.rfFilteredCd {
		row := m.rfFilteredCd[i]
		for row.Config.Clicked(gtx) {
			m.rfDetailOpen = true
			m.rfDetailCode = row.Code
			m.rfDetailObj = 0

			// If exactly one object is selected, we manage specific codes for it
			ids := []int{}
			for id, sel := range m.rfSelectedObjs {
				if sel {
					ids = append(ids, id)
				}
			}
			if len(ids) == 1 {
				m.rfDetailObj = ids[0]
			}

			// Load existing details
			var det core.RelayFilterDetail
			if m.rfDetailObj == 0 {
				det = m.rfRule.CodeDetails[m.rfDetailCode]
			} else {
				if m.rfRule.ObjCodeDetails[m.rfDetailObj] != nil {
					det = m.rfRule.ObjCodeDetails[m.rfDetailObj][m.rfDetailCode]
				}
			}
			m.rfDetailZones.SetText(strings.Join(intsToStrings(det.Zones), ", "))
			m.rfDetailParts.SetText(strings.Join(intsToStrings(det.Partitions), ", "))
		}
	}

	if m.rfDetailOpen {
		for m.rfDetailClose.Clicked(gtx) {
			m.rfDetailOpen = false
		}
		for m.rfDetailSave.Clicked(gtx) {
			m.rfApplyDetail()
			m.rfDetailOpen = false
		}
	}

	// Bulk selections
	for m.rfSelectAllObjs.Clicked(gtx) {
		for i := range m.rfFilteredObjs {
			m.rfFilteredObjs[i].Selected.Value = true
		}
	}
	for m.rfClearObjs.Clicked(gtx) {
		for i := range m.rfObjects {
			m.rfObjects[i].Selected.Value = false
		}
	}
	for m.rfSelectAllCodes.Clicked(gtx) {
		for i := range m.rfFilteredCd {
			m.rfFilteredCd[i].Selected.Value = true
		}
	}
	for m.rfClearCodes.Clicked(gtx) {
		// When no objects selected, clears global codes
		// When objects selected, clears specific codes for THEM
		for i := range m.rfFilteredCd {
			m.rfFilteredCd[i].Selected.Value = false
		}
	}

	m.updateRfFilters()
	m.rebuildRfSummary()
}

func (m *model) rfCheckObjectSelectionChanges() bool {
	changed := false
	if m.rfSelectedObjs == nil {
		m.rfSelectedObjs = make(map[int]bool)
		changed = true
	}
	current := make(map[int]bool)
	for _, it := range m.rfObjects {
		if it.Selected.Value {
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
		if it.Selected.Value {
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
			m.rfCodes[i].Selected.Value = blocked[m.rfCodes[i].Code]
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
			m.rfCodes[i].Selected.Value = blocked[m.rfCodes[i].Code]
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
			m.rfCodes[i].Selected.Value = allHave
		}
	}
	// Update the "previous state" tracker so we don't trigger a recursive change
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

func (m *model) updateRfFilters() {
	objQ := strings.ToLower(m.rfObjQuery.Text())
	m.rfFilteredObjs = m.rfFilteredObjs[:0]
	for i := range m.rfObjects {
		it := &m.rfObjects[i]
		if objQ == "" || strings.Contains(strings.ToLower(it.Display), objQ) {
			m.rfFilteredObjs = append(m.rfFilteredObjs, it)
		}
	}

	codeQ := strings.ToLower(m.rfCodeQuery.Text())
	m.rfFilteredCd = m.rfFilteredCd[:0]
	for i := range m.rfCodes {
		it := &m.rfCodes[i]
		if codeQ == "" || strings.Contains(strings.ToLower(it.Code), codeQ) || strings.Contains(strings.ToLower(it.Description), codeQ) {
			m.rfFilteredCd = append(m.rfFilteredCd, it)
		}
	}
}

func (m *model) openRelayFilter() {
	m.rfBusy.Store(true)
	go func() {
		rule, err := m.rt.GetRelayFilterRule(context.Background())
		m.rfResult <- rfResult{rule, err}
	}()
}

func (m *model) initRfRows() {
	devices := m.rt.GetDevices()
	m.rfObjects = make([]rfObjectRow, 0, len(devices))
	for _, d := range devices {
		m.rfObjects = append(m.rfObjects, rfObjectRow{
			ID:      d.ID,
			Display: fmt.Sprintf("%03d | %s", d.ID, firstNonEmpty(d.ClientAddr, "Disconnected")),
		})
	}

	events := m.rt.GetEventList()
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

func classifyEvent(code, typeNm, desc string) string {
	text := strings.ToLower(fmt.Sprintf("%s %s %s", code, typeNm, desc))
	if strings.Contains(text, "тест") || strings.Contains(text, "test") {
		return "test"
	}
	if strings.Contains(text, "трив") || strings.Contains(text, "alarm") {
		return "alarm"
	}
	if strings.Contains(text, "помил") || strings.Contains(text, "несправ") || strings.Contains(text, "fault") {
		return "fault"
	}
	if strings.Contains(text, "постан") || strings.Contains(text, "guard") {
		return "guard"
	}
	if strings.Contains(text, "знят") || strings.Contains(text, "disarm") {
		return "disarm"
	}
	return "other"
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
	m.rfEnabled.Value = rule.Enabled
	m.rfGroups.SetText(strings.Join(intsToStrings(rule.GroupNumbers), ", "))

	globalObjs := make(map[int]bool)
	for _, id := range rule.ObjectIDs {
		globalObjs[id] = true
	}
	for i := range m.rfObjects {
		m.rfObjects[i].Selected.Value = globalObjs[m.rfObjects[i].ID]
	}

	m.rfSelectedObjs = nil
	m.rfSelectedCodes = nil
	m.rfSyncCodesPaneToSelectedObjects()
}

func (m *model) rfApplyDetail() {
	det := core.RelayFilterDetail{
		Zones:      parseGroupsLine(m.rfDetailZones.Text()),
		Partitions: parseGroupsLine(m.rfDetailParts.Text()),
	}

	if m.rfDetailObj == 0 {
		if m.rfRule.CodeDetails == nil {
			m.rfRule.CodeDetails = make(map[string]core.RelayFilterDetail)
		}
		m.rfRule.CodeDetails[m.rfDetailCode] = det
	} else {
		// Apply to all selected objects to be consistent with bulk code assignment
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
	rule.Enabled = m.rfEnabled.Value
	rule.GroupNumbers = parseGroupsLine(m.rfGroups.Text())

	rule.ObjectIDs = []int{}
	for _, it := range m.rfObjects {
		if it.Selected.Value {
			rule.ObjectIDs = append(rule.ObjectIDs, it.ID)
		}
	}
	return rule
}

func (m *model) rebuildRfSummary() {
	m.rfSummary = m.rfSummary[:0]
	devices := m.rfObjects // Use objects from model for ordering
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

func intsToStrings(ints []int) []string {
	out := make([]string, len(ints))
	for i, v := range ints {
		out[i] = strconv.Itoa(v)
	}
	return out
}

func parseGroupsLine(text string) []int {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	out := []int{}
	for _, p := range parts {
		if v, err := strconv.Atoi(p); err == nil {
			out = append(out, v)
		}
	}
	return out
}

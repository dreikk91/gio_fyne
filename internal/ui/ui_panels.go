package ui

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"cid_gio_gio/internal/core"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func (m *model) draw(gtx layout.Context) layout.Dimensions {
	pageInset := unit.Dp(14)
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			fill(gtx, cBg)
			return layout.UniformInset(pageInset).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(m.topBar),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(m.statusStrip),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						switch m.tab {
						case 0:
							return m.objectsTab(gtx)
						case 1:
							return m.eventsTab(gtx)
						default:
							return m.settingsTab(gtx)
						}
					}),
				)
			})
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if !m.hOpen {
				return layout.Dimensions{}
			}
			return m.historyOverlay(gtx)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if !m.rfOpen {
				return layout.Dimensions{}
			}
			return m.relayFilterOverlay(gtx)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if !m.delOpen {
				return layout.Dimensions{}
			}
			return m.deleteOverlay(gtx)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if !m.ctxMenuOpen {
				return layout.Dimensions{}
			}
			return m.contextMenuOverlay(gtx)
		}),
	)
}

func (m *model) topBar(gtx layout.Context) layout.Dimensions {
	return panel(gtx, cPanel, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(m.headerBar),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(m.navigationRow),
		)
	})
}

func (m *model) headerBar(gtx layout.Context) layout.Dimensions {
	total := len(m.devices)
	visible := len(m.filteredDevices)
	subtitle := fmt.Sprintf("Objects: %d (visible %d) | Events loaded: %d", total, visible, len(m.events))

	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.H6(m.th, "CID Retranslator")
					l.Color = cText
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(m.th, subtitle)
					l.Color = cSoft
					return l.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			status := boolText(m.stats.Connected, "Online", "Offline")
			statusBg := firstColor(m.stats.Connected, cGoodSoft, cBadSoft)
			statusFg := firstColor(m.stats.Connected, cGood, cBad)
			return infoChip(gtx, m.th, "Status", status, statusFg, statusBg)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return infoChip(gtx, m.th, "Uptime", m.stats.Uptime, cText, cPanel2)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return infoChip(gtx, m.th, "Clients", strconv.Itoa(m.stats.Clients), cText, cPanel2)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return infoChip(gtx, m.th, "Accepted", strconv.FormatInt(m.stats.Accepted, 10), cText, cPanel2)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return infoChip(gtx, m.th, "Rejected", strconv.FormatInt(m.stats.Rejected, 10), cText, cPanel2)
		}),
		// 		footerStat(m.th, "Accepted", strconv.FormatInt(m.stats.Accepted, 10), cGood),
// 		footerStat(m.th, "Rejected", strconv.FormatInt(m.stats.Rejected, 10), cBad),
	)
}

func (m *model) statusStrip(gtx layout.Context) layout.Dimensions {
	if m.statusErr != "" {
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return statusBanner(gtx, m.th, m.statusErr, true)
		})
	}
	if m.statusMsg != "" {
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return statusBanner(gtx, m.th, m.statusMsg, false)
		})
	}
	return layout.Dimensions{}
}

func (m *model) navigationRow(gtx layout.Context) layout.Dimensions {
	labels := []string{"Objects", "Events", "Settings"}
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return tabBtn(gtx, m.th, &m.tabBtn[0], labels[0], m.tab == 0)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return tabBtn(gtx, m.th, &m.tabBtn[1], labels[1], m.tab == 1)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return tabBtn(gtx, m.th, &m.tabBtn[2], labels[2], m.tab == 2)
		}),
	)
}

// func (m *model) statsFooter(gtx layout.Context) layout.Dimensions {
// 	baseStats := []func(layout.Context) layout.Dimensions{
// 		footerStat(m.th, "Enabled", boolText(m.stats.Connected, "On", "Off"), firstColor(m.stats.Connected, cGood, cBad)),
// 		footerStat(m.th, "Uptime", m.stats.Uptime, cText),
// 		footerStat(m.th, "Accepted", strconv.FormatInt(m.stats.Accepted, 10), cGood),
// 		footerStat(m.th, "Rejected", strconv.FormatInt(m.stats.Rejected, 10), cBad),
// 	}
// 	extraStats := []func(layout.Context) layout.Dimensions{
// 		footerStat(m.th, "Msg/min", strconv.FormatInt(m.stats.ReceivedPS*60, 10), cAccent),
// 		footerStat(m.th, "Reconnects", strconv.FormatInt(m.stats.Reconnects, 10), cSoft),
// 		footerStat(m.th, "Clients", strconv.Itoa(m.stats.Clients), cText),
// 		footerStat(m.th, "Active", strconv.Itoa(m.activeDevices), cGood),
// 		footerStat(m.th, "Inactive", strconv.Itoa(m.inactiveDevices), cBad),
// 	}

// 	// toggleLabel := "Details"
// 	// if m.statsExpanded {
// 	// 	toggleLabel = "Hide"
// 	// }
// 	for m.statsToggle.Clicked(gtx) {
// 		m.statsExpanded = !m.statsExpanded
// 	}


// 	return outlinedCard(gtx, cPanel, cBorder, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
// 		rowH := clamp(gtx.Dp(unit.Dp(int(m.th.TextSize)+18)), gtx.Dp(unit.Dp(30)), gtx.Dp(unit.Dp(44)))
// 		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
// 			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
// 				gtx.Constraints.Min.Y = rowH
// 				gtx.Constraints.Max.Y = rowH
// 				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
// 					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
// 						return statsRow(gtx, baseStats)
// 					}),
// 					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
// 					// layout.Rigid(func(gtx layout.Context) layout.Dimensions {
// 					// 	return subtleButton(gtx, m.th, &m.statsToggle, toggleLabel)
// 					// }),
// 				)
// 			}),
// 			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
// 				if !m.statsExpanded {
// 					return layout.Dimensions{}
// 				}
// 				return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
// 					return card(gtx, cPanel2, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
// 						return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
// 							return statsRow(gtx, extraStats)
// 						})
// 					})
// 				})
// 			}),
// 		)
// 	})
// }

func statsRow(gtx layout.Context, stats []func(layout.Context) layout.Dimensions) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(stats)*2)
	for i, stat := range stats {
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout))
		}
		children = append(children, layout.Flexed(1, stat))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func (m *model) objectsTab(gtx layout.Context) layout.Dimensions {
	m.ensureRows(len(m.filteredDevices))
	return panel(gtx, cPanel, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(m.objectsSummaryRow),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(m.objectsToolbar),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(m.objectsHeader),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Flexed(1, m.objectsList),
		)
	})
}

func (m *model) objectsSummaryRow(gtx layout.Context) layout.Dimensions {
	total := len(m.devices)
	visible := len(m.filteredDevices)
	active := m.activeDevices
	inactive := m.inactiveDevices

	cards := []func(layout.Context) layout.Dimensions{
		func(gtx layout.Context) layout.Dimensions {
			return metricCard(gtx, m.th, "Total objects", strconv.Itoa(total), cPanel2, cText)
		},
		func(gtx layout.Context) layout.Dimensions {
			return metricCard(gtx, m.th, "Visible", strconv.Itoa(visible), cAccent2, cAccent)
		},
		func(gtx layout.Context) layout.Dimensions {
			return metricCard(gtx, m.th, "Active", strconv.Itoa(active), cGoodSoft, cGood)
		},
		func(gtx layout.Context) layout.Dimensions {
			return metricCard(gtx, m.th, "Inactive", strconv.Itoa(inactive), cBadSoft, cBad)
		},
	}

	return statsRow(gtx, cards)
}

func (m *model) objectsToolbar(gtx layout.Context) layout.Dimensions {
	total := len(m.devices)
	visible := len(m.filteredDevices)
	info := fmt.Sprintf("%d / %d", visible, total)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return m.searchBar(gtx, &m.objSearch, "Search objects by ID, client or last event...")
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return infoChip(gtx, m.th, "Showing", info, cText, cPanel2)
		}),
	)
}

func (m *model) searchBar(gtx layout.Context, editor *widget.Editor, hint string) layout.Dimensions {
	return editorSurface(gtx, func(gtx layout.Context) layout.Dimensions {
		e := material.Editor(m.th, editor, hint)
		e.Color = cText
		e.HintColor = cSoft
		e.SelectionColor = cAccentSoft
		return e.Layout(gtx)
	})
}

func (m *model) objectsHeader(gtx layout.Context) layout.Dimensions {
	return rowHeader(gtx, m.th, []string{"State", "PPK", "Client", "Last Event", "Date/Time"})
}

func (m *model) objectsList(gtx layout.Context) layout.Dimensions {
	list := material.List(m.th, &m.objList)
	return list.Layout(gtx, len(m.filteredDevices), func(gtx layout.Context, i int) layout.Dimensions {
		d := m.filteredDevices[i]
		for {
			_, ok := m.objRows[i].Update(gtx)
			if !ok {
				break
			}
			m.openHistory(d)
		}
		// Right-click detection
		for {
			ev, ok := gtx.Event(pointer.Filter{
				Target: &m.objRCTags[i],
				Kinds:  pointer.Press,
			})
			if !ok {
				break
			}
			if pe, ok := ev.(pointer.Event); ok && pe.Buttons.Contain(pointer.ButtonSecondary) {
				m.ctxMenuOpen = true
				m.ctxMenuDevice = d
			}
		}

		// Record the row layout, then overlay right-click hit area
		rec := op.Record(gtx.Ops)
		dims := m.objRows[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			stale, ts := isStale(d.LastEventTime, m.activityTO), "-"
			if !d.LastEventTime.IsZero() {
				ts = d.LastEventTime.Format("2006-01-02 15:04:05")
			}
			stateColor := firstColor(stale, cBad, cGood)
			return rowWithColors(
				gtx,
				firstColor(i%2 == 0, cPanel2, cPanel),
				m.th,
				[]string{boolText(stale, "Inactive", "Active"), fmt.Sprintf("%03d", d.ID), firstNonEmpty(d.ClientAddr, "-"), d.LastEvent, ts},
				[]color.NRGBA{stateColor, cText, cSoft, cText, cText},
			)
		})
		call := rec.Stop()

		call.Add(gtx.Ops)

		// Register right-click pointer handler on the row area (pass-through)
		area := clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops)
		pass := pointer.PassOp{}.Push(gtx.Ops)
		event.Op(gtx.Ops, &m.objRCTags[i])
		pass.Pop()
		area.Pop()
		return dims
	})
}

func (m *model) eventsTab(gtx layout.Context) layout.Dimensions {
	return panel(gtx, cPanel, func(gtx layout.Context) layout.Dimensions {
		toolbar := func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					children := make([]layout.FlexChild, 0, len(eventFilters)*2)
					for i, f := range eventFilters {
						if i > 0 {
							children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout))
						}
						ff := f
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return eventFilterBtn(gtx, m.th, m.filterBtns[ff], ff, m.eventFilter == ff)
						}))
					}
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return m.searchBar(gtx, &m.evtSearch, "Search events by code, description, zone...")
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return card(gtx, cPanel2, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
						c := material.CheckBox(m.th, &m.hideTestsBox, "Hide tests")
						c.Color = cText
						c.IconColor = cAccent
						return c.Layout(gtx)
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return card(gtx, cPanel2, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
						c := material.CheckBox(m.th, &m.hideBlockedBox, "Only non-blocked")
						c.Color = cText
						c.IconColor = cAccent
						return c.Layout(gtx)
					})
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(m.eventsSummaryRow),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(toolbar),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return rowHeader(gtx, m.th, []string{"Time", "PPK", "Code", "Type", "Description", "Zone", "Relay"})
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if m.eventsBusy.Load() && len(m.filteredEvents) == 0 {
					return loadingPlaceholder(gtx, m.th, "Loading events", "Preparing object event list...")
				}
				return m.eventsList(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !m.eventsBusy.Load() || len(m.filteredEvents) == 0 {
					return layout.Dimensions{}
				}
				return loadingFooter(gtx, m.th, "Loading more events...")
			}),
		)
	})
}

func (m *model) eventsSummaryRow(gtx layout.Context) layout.Dimensions {
	visible := len(m.filteredEvents)
	loaded := len(m.events)
	filter := strings.ToUpper(m.eventFilter)
	bg, fg := filterTone(m.eventFilter)

	cards := []func(layout.Context) layout.Dimensions{
		func(gtx layout.Context) layout.Dimensions {
			return metricCard(gtx, m.th, "Visible", strconv.Itoa(visible), cPanel2, cText)
		},
		func(gtx layout.Context) layout.Dimensions {
			return metricCard(gtx, m.th, "Loaded", strconv.Itoa(loaded), cAccent2, cAccent)
		},
		func(gtx layout.Context) layout.Dimensions {
			return metricCard(gtx, m.th, "Filter", filter, bg, fg)
		},
		func(gtx layout.Context) layout.Dimensions {
			return metricCard(gtx, m.th, "Msg/min", strconv.FormatInt(m.stats.ReceivedPS*60, 10), cPanel2, cAccent)
		},
	}
	return statsRow(gtx, cards)
}

func (m *model) eventFiltersRow(gtx layout.Context) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(eventFilters)*2+2)
	for i, f := range eventFilters {
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout))
		}
		ff := f
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return eventFilterBtn(gtx, m.th, m.filterBtns[ff], ff, m.eventFilter == ff)
		}))
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Flexed(1, layout.Spacer{}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return card(gtx, cPanel2, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
				c := material.CheckBox(m.th, &m.hideTestsBox, "Hide tests")
				c.Color = cText
				c.IconColor = cAccent
				return c.Layout(gtx)
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return card(gtx, cPanel2, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
				c := material.CheckBox(m.th, &m.hideBlockedBox, "Only non-blocked")
				c.Color = cText
				c.IconColor = cAccent
				return c.Layout(gtx)
			})
		}),
	)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func (m *model) eventsList(gtx layout.Context) layout.Dimensions {
	list := material.List(m.th, &m.evtList)
	dims := list.Layout(gtx, len(m.filteredEvents), func(gtx layout.Context, i int) layout.Dimensions {
		e := m.filteredEvents[i]
		relay := "OK"
		if e.RelayBlocked {
			relay = "Blocked"
		}
		return row(gtx, eventColor(e.Category, i), m.th, []string{
			e.Time.Format("2006-01-02 15:04:05"), e.DeviceID, e.Code, e.Type, e.Desc, e.Zone, relay,
		})
	})
	m.maybeLoadMoreEvents()
	return dims
}

func (m *model) historyPanel(gtx layout.Context) layout.Dimensions {
	return modalCard(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(m.historyHeader),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			layout.Rigid(m.historyFiltersRow),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(m.historyToolbar),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return rowHeader(gtx, m.th, []string{"Time", "PPK", "Code", "Type", "Description", "Zone", "Relay"})
			}),
			layout.Flexed(1, m.historyList),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !m.historyBusy.Load() || len(m.hRows) == 0 {
					return layout.Dimensions{}
				}
				return loadingFooter(gtx, m.th, "Loading more history records...")
			}),
		)
	})
}

func (m *model) historyHeader(gtx layout.Context) layout.Dimensions {
	return card(gtx, cModalH, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.H6(m.th, fmt.Sprintf("Device Event Journal - %03d", m.hDevice.ID))
				l.Color = cText
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(m.th, fmt.Sprintf("Records: %d", len(m.hRows)))
				l.Color = cSoft
				return l.Layout(gtx)
			}),
		)
	})
}

func (m *model) historyFiltersRow(gtx layout.Context) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(eventFilters)*2+2)
	for i, f := range eventFilters {
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout))
		}
		ff := f
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return eventFilterBtn(gtx, m.th, m.hFilterBtns[ff], ff, m.hEventType == ff)
		}))
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Flexed(1, layout.Spacer{}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return card(gtx, cPanel2, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
				c := material.CheckBox(m.th, &m.hHideBox, "Hide tests")
				c.Color = cText
				c.IconColor = cAccent
				return c.Layout(gtx)
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return card(gtx, cPanel2, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
				c := material.CheckBox(m.th, &m.hHideBlockedBox, "Only non-blocked")
				c.Color = cText
				c.IconColor = cAccent
				return c.Layout(gtx)
			})
		}),
	)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func (m *model) historyToolbar(gtx layout.Context) layout.Dimensions {
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return m.searchBar(gtx, &m.hSearch, "History search...")
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return accentButton(gtx, m.th, &m.hReload, "Reload")
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return secondaryButton(gtx, m.th, &m.hClose, "Close")
		}),
	)
}

func (m *model) historyList(gtx layout.Context) layout.Dimensions {
	if m.historyBusy.Load() && len(m.hRows) == 0 {
		return loadingPlaceholder(gtx, m.th, "Loading history", fmt.Sprintf("Loading events for object %03d...", m.hDevice.ID))
	}
	list := material.List(m.th, &m.hList)
	dims := list.Layout(gtx, len(m.hRows), func(gtx layout.Context, i int) layout.Dimensions {
		e := m.hRows[i]
		relay := "OK"
		if e.RelayBlocked {
			relay = "Blocked"
		}
		return row(gtx, eventColor(e.Category, i), m.th, []string{
			e.Time.Format("2006-01-02 15:04:05"), e.DeviceID, e.Code, e.Type, e.Desc, e.Zone, relay,
		})
	})
	m.maybeLoadMoreHistory()
	return dims
}

func (m *model) historyOverlay(gtx layout.Context) layout.Dimensions {
	fill(gtx, cOverlay)
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		maxW := min(gtx.Constraints.Max.X-gtx.Dp(20), gtx.Dp(1240))
		maxH := min(gtx.Constraints.Max.Y-gtx.Dp(20), gtx.Dp(780))
		if maxW < gtx.Dp(760) {
			maxW = max(gtx.Dp(520), gtx.Constraints.Max.X-gtx.Dp(8))
		}
		if maxH < gtx.Dp(420) {
			maxH = max(gtx.Dp(360), gtx.Constraints.Max.Y-gtx.Dp(8))
		}
		maxW = clamp(maxW, 1, gtx.Constraints.Max.X)
		maxH = clamp(maxH, 1, gtx.Constraints.Max.Y)
		inner := gtx
		inner.Constraints = layout.Exact(image.Pt(maxW, maxH))
		return m.historyPanel(inner)
	})
}

func (m *model) deletePanel(gtx layout.Context) layout.Dimensions {
	return modalCard(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						title := material.H6(m.th, fmt.Sprintf("Delete Object %03d", m.delDevice.ID))
						title.Color = cText
						return title.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return secondaryButton(gtx, m.th, &m.delCancel, "Close")
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(m.th, "This will permanently remove the object, journal and history records.")
				l.Color = cSoft
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(m.th, "This action cannot be undone.")
				l.Color = cBad
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := boolText(m.delBusy.Load(), "Deleting...", "Delete permanently")
						return dangerButton(gtx, m.th, &m.delConfirm, label)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return secondaryButton(gtx, m.th, &m.delCancel, "Cancel")
					}),
				)
			}),
		)
	})
}

func (m *model) deleteOverlay(gtx layout.Context) layout.Dimensions {
	fill(gtx, cOverlay)
	// Backdrop click closes the modal.
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return m.delBackdrop.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			inner := gtx
			inner.Constraints.Min = image.Point{}
			return layout.Center.Layout(inner, func(gtx layout.Context) layout.Dimensions {
				w := min(gtx.Constraints.Max.X-gtx.Dp(28), gtx.Dp(620))
				h := min(gtx.Constraints.Max.Y-gtx.Dp(28), gtx.Dp(260))
				if w < gtx.Dp(460) {
					w = max(gtx.Dp(340), gtx.Constraints.Max.X-gtx.Dp(12))
				}
				if h < gtx.Dp(220) {
					h = max(gtx.Dp(200), gtx.Constraints.Max.Y-gtx.Dp(12))
				}
				w = clamp(w, 1, gtx.Constraints.Max.X)
				h = clamp(h, 1, gtx.Constraints.Max.Y)
				inner := gtx
				inner.Constraints = layout.Exact(image.Pt(w, h))

				rec := op.Record(inner.Ops)
				dims := m.deletePanel(inner)
				call := rec.Stop()

				// Block backdrop clicks inside the panel area.
				area := clip.Rect(image.Rectangle{Max: dims.Size}).Push(inner.Ops)
				event.Op(inner.Ops, &m.delPanelTag)
				area.Pop()

				call.Add(inner.Ops)
				return dims
			})
		}),
	)
}

func (m *model) contextMenuOverlay(gtx layout.Context) layout.Dimensions {
	fill(gtx, cOverlay)
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		w := min(gtx.Constraints.Max.X-gtx.Dp(40), gtx.Dp(280))
		w = clamp(w, gtx.Dp(200), gtx.Constraints.Max.X)
		inner := gtx
		inner.Constraints.Min.X = w
		inner.Constraints.Max.X = w
		return m.contextMenuPanel(inner)
	})
}

func (m *model) contextMenuPanel(gtx layout.Context) layout.Dimensions {
	return modalCard(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						title := material.Body1(m.th, fmt.Sprintf("Object %03d", m.ctxMenuDevice.ID))
						title.Color = cText
						return title.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return secondaryButton(gtx, m.th, &m.ctxMenuClose, "Close")
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return accentButton(gtx, m.th, &m.ctxMenuHist, "View History")
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return dangerButton(gtx, m.th, &m.ctxMenuDel, "Delete Object")
			}),
		)
	})
}

func filterTone(filter string) (color.NRGBA, color.NRGBA) {
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "alarm":
		return cBadSoft, cBad
	case "test":
		return cWarnSoft, cWarn
	case "fault":
		return color.NRGBA{R: 255, G: 238, B: 214, A: 255}, color.NRGBA{R: 168, G: 95, B: 0, A: 255}
	case "guard":
		return cGoodSoft, cGood
	case "disguard":
		return cAccentSoft, cAccent
	case "other":
		return cPanel3, cSoft
	default:
		return cAccent2, cAccent
	}
}

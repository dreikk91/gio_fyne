package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func rowHeader(gtx layout.Context, th *material.Theme, cols []string) layout.Dimensions {
	return row(gtx, cPanel3, th, cols)
}

func row(gtx layout.Context, bg color.NRGBA, th *material.Theme, cols []string) layout.Dimensions {
	return rowWithColors(gtx, bg, th, cols, nil)
}

func rowWithColors(gtx layout.Context, bg color.NRGBA, th *material.Theme, cols []string, colors []color.NRGBA) layout.Dimensions {
	weights := []float32{2, 1, 1, 2, 5, 2}
	if len(cols) == 4 {
		weights = []float32{1, 4, 1.2, 8}
	} else if len(cols) == 5 {
		weights = []float32{1.1, 1, 3.1, 2.3, 2}
	} else if len(cols) == 7 {
		weights = []float32{1.4, 0.8, 0.8, 1.2, 4.5, 2.0, 0.8}
	} else if len(cols) == 8 {
		weights = []float32{1.4, 0.8, 0.8, 1.2, 4.5, 2.0, 1.0, 0.8}
	}
	if len(weights) < len(cols) {
		for len(weights) < len(cols) {
			weights = append(weights, 1.0)
		}
	}
	return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		rowH := max(gtx.Dp(34), gtx.Dp(unit.Dp(int(th.TextSize)+20)))
		gtx.Constraints.Min.Y = rowH
		if gtx.Constraints.Max.Y > rowH {
			gtx.Constraints.Max.Y = rowH
		}
		return card(gtx, bg, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(cols)*2)
			for i, t := range cols {
				if i > 0 {
					children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
				}
				w, txt := weights[i], t
				children = append(children, layout.Flexed(w, func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(th, txt)
					l.Color = cText
					if i < len(colors) {
						l.Color = colors[i]
					}
					l.MaxLines = 1
					return l.Layout(gtx)
				}))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

func field(m *model, title, key string) func(layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			editor := func(gtx layout.Context) layout.Dimensions {
				return editorSurface(gtx, func(gtx layout.Context) layout.Dimensions {
					e := material.Editor(m.th, m.cfgFields[key], "")
					e.Color = cText
					e.HintColor = cSoft
					e.SelectionColor = cAccentSoft
					return e.Layout(gtx)
				})
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(180), gtx.Dp(180)
					l := material.Body2(m.th, title)
					l.Color = cSoft
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
				layout.Flexed(1, editor),
			)
		})
	}
}

func logLevelField(m *model) func(layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		current := strings.ToUpper(strings.TrimSpace(m.cfgFields["Logging.Level"].Text()))
		if current == "" {
			current = strings.ToUpper(m.cfg.Logging.Level)
		}
		if current == "" {
			current = "INFO"
		}
		return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(m.th, "Log level")
					l.Color = cSoft
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return secondaryButton(gtx, m.th, &m.logLevelBtn, current+"  v")
				}),
			}
			if m.logLevelOpen {
				children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout))
				for _, lvl := range logLevels {
					value := lvl
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						active := strings.EqualFold(value, m.cfgFields["Logging.Level"].Text())
						if active {
							return accentButton(gtx, m.th, m.logLevelBtnMap[value], strings.ToUpper(value))
						}
						return subtleButton(gtx, m.th, m.logLevelBtnMap[value], strings.ToUpper(value))
					}))
				}
			}
			return card(gtx, cPanel2, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
	}
}

func multilineField(m *model, title, key string, heightDp int) func(layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(m.th, title)
					l.Color = cSoft
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(heightDp))
					gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(heightDp))
					return editorSurface(gtx, func(gtx layout.Context) layout.Dimensions {
						e := material.Editor(m.th, m.cfgFields[key], "")
						e.Color = cText
						e.HintColor = cSoft
						e.SelectionColor = cAccentSoft
						return e.Layout(gtx)
					})
				}),
			)
		})
	}
}

func flag(m *model, key, label string) func(layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		return card(gtx, cPanel2, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
			c := material.CheckBox(m.th, m.cfgFlags[key], label)
			c.Color = cText
			c.IconColor = cAccent
			return c.Layout(gtx)
		})
	}
}

func footerStat(th *material.Theme, title, value string, valueColor color.NRGBA) func(layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, title+":")
				l.Color = cSoft
				l.MaxLines = 1
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(3)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, value)
				l.Color = valueColor
				return l.Layout(gtx)
			}),
		)
	}
}

func tabBtn(gtx layout.Context, th *material.Theme, clk *widget.Clickable, text string, active bool) layout.Dimensions {
	if active {
		return accentButton(gtx, th, clk, text)
	}
	return subtleButton(gtx, th, clk, text)
}

func eventFilterBtn(gtx layout.Context, th *material.Theme, clk *widget.Clickable, text string, active bool) layout.Dimensions {
	b := material.Button(th, clk, text)
	if !active {
		b.Background = cPanel2
		b.Color = cText
		return b.Layout(gtx)
	}
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "alarm":
		b.Background = cBad
		b.Color = cPanel
	case "test":
		b.Background = cWarn
		b.Color = cPanel
	case "fault":
		b.Background = color.NRGBA{R: 214, G: 126, B: 0, A: 255}
		b.Color = cPanel
	case "guard":
		b.Background = cGood
		b.Color = cPanel
	case "disguard":
		b.Background = cAccent
		b.Color = cPanel
	case "other":
		b.Background = cSoft
		b.Color = cPanel
	default:
		b.Background = cAccent
		b.Color = cPanel
	}
	return b.Layout(gtx)
}

func panel(gtx layout.Context, col color.NRGBA, w layout.Widget) layout.Dimensions {
	return card(gtx, col, unit.Dp(12), w)
}

func modalCard(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return outlinedCard(gtx, cModal, cModalB, unit.Dp(18), w)
}

func settingsSection(th *material.Theme, gtx layout.Context, title string, content layout.Widget) layout.Dimensions {
	return card(gtx, cPanel, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.H6(th, title)
				lbl.Color = cText
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			layout.Rigid(content),
		)
	})
}

func loadingPlaceholder(gtx layout.Context, th *material.Theme, title, subtitle string) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = min(gtx.Constraints.Max.X, gtx.Dp(360))
		return outlinedCard(gtx, cPanel, cBorder, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					loader := material.Loader(th)
					loader.Color = cAccent
					return loader.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Body1(th, title)
					l.Color = cText
					return l.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(th, subtitle)
					l.Color = cSoft
					return l.Layout(gtx)
				}),
			)
		})
	})
}

func loadingFooter(gtx layout.Context, th *material.Theme, text string) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return card(gtx, cPanel2, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					loader := material.Loader(th)
					loader.Color = cAccent
					return loader.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Body2(th, text)
					l.Color = cSoft
					return l.Layout(gtx)
				}),
			)
		})
	})
}

func fontSlider(m *model) func(layout.Context) layout.Dimensions {
	return func(gtx layout.Context) layout.Dimensions {
		current := fontSizeFromSlider(m.fontSize.Value)
		return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return card(gtx, cPanel2, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Body2(m.th, "Font size")
								l.Color = cSoft
								return l.Layout(gtx)
							}),
							layout.Flexed(1, layout.Spacer{}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Body2(m.th, fmt.Sprintf("%d", current))
								l.Color = cText
								return l.Layout(gtx)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						before := current
						s := material.Slider(m.th, &m.fontSize)
						s.Color = cAccent
						dims := s.Layout(gtx)
						after := fontSizeFromSlider(m.fontSize.Value)
						if after != before {
							m.cfgFields["UI.FontSize"].SetText(fmt.Sprintf("%d", after))
							m.applyFontSize(after)
						}
						return dims
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Caption(m.th, "7 .. 30")
						l.Color = cSoft
						return l.Layout(gtx)
					}),
				)
			})
		})
	}
}

func fill(gtx layout.Context, col color.NRGBA) {
	paint.FillShape(gtx.Ops, col, clip.Rect{Max: gtx.Constraints.Max}.Op())
}

func card(gtx layout.Context, col color.NRGBA, inset unit.Dp, w layout.Widget) layout.Dimensions {
	return outlinedCard(gtx, col, cBorder, inset, w)
}

func editorSurface(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return outlinedCard(gtx, cPanel, cBorder, unit.Dp(10), w)
}

func outlinedCard(gtx layout.Context, bg, border color.NRGBA, inset unit.Dp, w layout.Widget) layout.Dimensions {
	rec := op.Record(gtx.Ops)
	dims := layout.UniformInset(inset).Layout(gtx, w)
	call := rec.Stop()
	if dims.Size.X > 0 && dims.Size.Y > 0 {
		rect := image.Rectangle{Max: dims.Size}
		radius := gtx.Dp(unit.Dp(10))
		paint.FillShape(gtx.Ops, bg, clip.UniformRRect(rect, radius).Op(gtx.Ops))
		stroke := clip.Stroke{
			Path:  clip.UniformRRect(rect, radius).Path(gtx.Ops),
			Width: float32(gtx.Dp(unit.Dp(1))),
		}.Op()
		paint.FillShape(gtx.Ops, border, stroke)
	}
	call.Add(gtx.Ops)
	return dims
}

func accentButton(gtx layout.Context, th *material.Theme, clk *widget.Clickable, text string) layout.Dimensions {
	b := material.Button(th, clk, text)
	b.Background = cAccent
	b.Color = cPanel
	return b.Layout(gtx)
}

func secondaryButton(gtx layout.Context, th *material.Theme, clk *widget.Clickable, text string) layout.Dimensions {
	b := material.Button(th, clk, text)
	b.Background = cAccent2
	b.Color = cAccent
	return b.Layout(gtx)
}

func subtleButton(gtx layout.Context, th *material.Theme, clk *widget.Clickable, text string) layout.Dimensions {
	b := material.Button(th, clk, text)
	b.Background = cPanel2
	b.Color = cText
	return b.Layout(gtx)
}

func dangerButton(gtx layout.Context, th *material.Theme, clk *widget.Clickable, text string) layout.Dimensions {
	b := material.Button(th, clk, text)
	b.Background = cBad
	b.Color = cPanel
	return b.Layout(gtx)
}

func metricCard(gtx layout.Context, th *material.Theme, title, value string, toneBg, toneFg color.NRGBA) layout.Dimensions {
	return outlinedCard(gtx, toneBg, cBorder, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, title+":")
				l.Color = cSoft
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, value)
				l.Color = toneFg
				return l.Layout(gtx)
			}),
		)
	})
}

func infoChip(gtx layout.Context, th *material.Theme, label, value string, fg, bg color.NRGBA) layout.Dimensions {
	return outlinedCard(gtx, bg, cBorder, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, label+":")
				l.Color = cSoft
				l.MaxLines = 1
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, value)
				l.Color = fg
				l.MaxLines = 1
				return l.Layout(gtx)
			}),
		)
	})
}

func statusBanner(gtx layout.Context, th *material.Theme, text string, isError bool) layout.Dimensions {
	bg := cAccentSoft
	fg := cAccent
	if isError {
		bg = cBadSoft
		fg = cBad
	}
	return outlinedCard(gtx, bg, cBorder, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
		l := material.Body2(th, text)
		l.Color = fg
		return l.Layout(gtx)
	})
}

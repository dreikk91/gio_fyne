//go:build windows

package walk

import (
	"fmt"
	"strconv"
	"strings"

	"cid_fyne/internal/core"

	"github.com/lxn/walk"
	"github.com/rs/zerolog/log"
)

func (a *walkApp) updateStatusBar() {
	if a.statusLabel == nil || a.transportLabel == nil || a.uptimeLabel == nil {
		return
	}
	msg := a.status
	if strings.TrimSpace(a.statusErr) != "" {
		msg = a.statusErr
	}

	if a.headerTitle != nil {
		a.headerTitle.SetText("CID Walk")
	}
	if a.headerSubtitle != nil {
		a.headerSubtitle.SetText(fmt.Sprintf("Objects: %d | Visible events: %d | Uptime: %s", len(a.filteredDevices), a.visibleEvents, a.stats.Uptime))
	}
	if a.headerStatus != nil {
		state := boolText(a.stats.Connected, "ONLINE", "OFFLINE")
		a.headerStatus.SetText(state)
		if brush, err := walk.NewSolidColorBrush(firstColorByState(a.stats.Connected)); err == nil {
			a.headerStatus.SetBackground(brush)
		}
	}
	if a.headerClients != nil {
		a.headerClients.SetText(fmt.Sprintf("Clients: %d", a.stats.Clients))
	}
	if a.headerEvents != nil {
		a.headerEvents.SetText(fmt.Sprintf("Events: %d", a.visibleEvents))
	}

	a.statusLabel.SetText(fmt.Sprintf("Стан: %s [A:%d I:%d]", msg, a.activeDevices, a.inactiveDevices))
	a.transportLabel.SetText(fmt.Sprintf("Мережа: %s", boolText(a.stats.Connected, "OK", "OFF")))
	a.uptimeLabel.SetText(fmt.Sprintf("Up: %s", a.stats.Uptime))
	if strings.TrimSpace(a.statusErr) != "" {
		a.statusLabel.SetTextColor(colorBadText)
	} else {
		a.statusLabel.SetTextColor(colorText)
	}
	if a.stats.Connected {
		a.transportLabel.SetTextColor(colorGoodText)
	} else {
		a.transportLabel.SetTextColor(colorBadText)
	}

	if a.acceptedLabel != nil {
		a.acceptedLabel.SetText(fmt.Sprintf("Ack: %d", a.stats.Accepted))
	}
	if a.rejectedLabel != nil {
		a.rejectedLabel.SetText(fmt.Sprintf("Nack: %d", a.stats.Rejected))
	}
	if a.rateLabel != nil {
		a.rateLabel.SetText(fmt.Sprintf("%d msg/min", a.stats.ReceivedPM))
	}

	statusTip := fmt.Sprintf("Об'єкти: %d активних / %d неактивних", a.activeDevices, a.inactiveDevices)
	transportTip := fmt.Sprintf("Uptime: %s | Clients: %d | Msg/s: %d", a.stats.Uptime, a.stats.Clients, a.stats.ReceivedPS)
	metricsTip := fmt.Sprintf("Reconnects: %d | Evs: %d | Drop: %d/%d", a.stats.Reconnects, a.visibleEvents, a.dropDevices.Load(), a.dropEvents.Load())

	a.statusLabel.SetToolTipText(statusTip)
	a.transportLabel.SetToolTipText(transportTip)
	if a.acceptedLabel != nil {
		a.acceptedLabel.SetToolTipText(metricsTip)
	}
}

func firstColorByState(connected bool) walk.Color {
	if connected {
		return colorHeroChipOnline
	}
	return colorHeroChipOffline
}

func (a *walkApp) styleDeviceCell(style *walk.CellStyle) {
	d, ok := a.deviceModel.Row(style.Row())
	if !ok {
		return
	}
	if a.objTable != nil && style.Row() == a.objTable.CurrentIndex() {
		return
	}
	if style.Row()%2 == 0 {
		style.BackgroundColor = colorRowAlt
	} else {
		style.BackgroundColor = colorWhite
	}
	style.TextColor = colorText
	if isStaleTime(d.LastEventTime, a.activityTO) {
		style.BackgroundColor = colorBadBg
		style.TextColor = colorBadText
	}
}

func (a *walkApp) styleEventCell(style *walk.CellStyle) {
	e, ok := a.eventModel.Row(style.Row())
	if !ok {
		return
	}
	if a.eventTable != nil && style.Row() == a.eventTable.CurrentIndex() {
		return
	}
	style.BackgroundColor, style.TextColor = priorityColors(a, e.Category, style.Row())
}

func (a *walkApp) isDeviceInactive(d core.DeviceDTO) bool {
	return isStaleTime(d.LastEventTime, a.activityTO)
}

func (a *walkApp) applyUIFont(size int) {
	if a.mw == nil {
		return
	}
	size = clampInt(size, 7, 30)
	font, err := walk.NewFont("Segoe UI", size, 0)
	if err != nil {
		log.Warn().Err(err).Int("font_size", size).Msg("failed to apply ui font")
		return
	}

	// Avoid unnecessary updates if the font hasn't changed
	if curFont := a.mw.Font(); curFont != nil && curFont.PointSize() == size {
		return
	}

	a.mw.SetSuspended(true)
	defer a.mw.SetSuspended(false)

	a.mw.SetFont(font)
	a.applyUILayoutScale(size)
	if a.uiFontValue != nil {
		a.uiFontValue.SetText(fmt.Sprintf("%d", size))
	}
	if a.uiFontCombo != nil {
		if val, _ := strconv.Atoi(a.uiFontCombo.Text()); val != size {
			a.uiFontCombo.SetText(strconv.Itoa(size))
		}
	}
	a.updateEventTableColumns()
	a.updateObjectTableColumns()
	a.repaintTables()
}

func (a *walkApp) applyUILayoutScale(size int) {
	scale := clampInt(size-10, 0, 18)
	toolbarH := 42 + scale
	footerH := 28 + scale/2
	tableMinH := 170 + scale*4
	inputH := 30 + scale/2
	checkboxH := 28 + scale/2

	setMinHeight(a.objToolbar, toolbarH)
	setMinHeight(a.eventToolbar, toolbarH)
	setMinHeight(a.footerBar, footerH)

	setMinHeight(a.objSearch, inputH)
	setMinHeight(a.eventSearch, inputH)
	setMinHeight(a.eventFilterBox, inputH)
	setMinHeight(a.hideTestsBox, checkboxH)
	setMinHeight(a.statusLabel, footerH)
	setMinHeight(a.transportLabel, footerH)
	setMinHeight(a.uptimeLabel, footerH)
	setMinHeight(a.acceptedLabel, footerH)
	setMinHeight(a.rejectedLabel, footerH)
	setMinHeight(a.rateLabel, footerH)
	setMinHeight(a.uiFontCombo, inputH)

	setMinHeight(a.objTable, tableMinH)
	setMinHeight(a.eventTable, tableMinH)
}

type minMaxSizer interface {
	SetMinMaxSize(min, max walk.Size) error
}

func setMinHeight(w minMaxSizer, height int) {
	if w == nil || height <= 0 {
		return
	}
	_ = w.SetMinMaxSize(walk.Size{Height: height}, walk.Size{})
}

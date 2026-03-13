package ui

import (
	"context"
	"fmt"
	"image/color"
	"time"

	"cid_gio_gio/internal/core"
	appLog "cid_gio_gio/internal/logger"
	appRuntime "cid_gio_gio/internal/runtime"

	"github.com/rs/zerolog/log"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var colorInk = color.NRGBA{R: 9, G: 14, B: 18, A: 255}

func Run(ctx context.Context, rt *appRuntime.Runtime) error {
	defer appLog.RecoverPanic("ui-run")
	uiCtx, cancel := context.WithCancel(ctx)
	w := new(app.Window)
	w.Option(app.Title("CID Retranslator - Gio"), app.Size(unit.Dp(800), unit.Dp(600)))

	m := newModel(uiCtx, cancel, rt, w)
	defer cancel()

	if err := m.start(); err != nil {
		log.Error().Err(err).Msg("ui start failed")
		return err
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		_ = rt.Stop(stopCtx)
	}()
	go m.uiWakeLoop()
	go m.statsLoop()

	for {
		m.pullAsyncResults()
		e := w.Event()
		switch e := e.(type) {
		case app.Win32ViewEvent:
			m.onWin32ViewEvent(e)
		case app.DestroyEvent:
			return m.handleDestroyEvent(e)
		case app.FrameEvent:
			gtx := app.NewContext(&m.ops, e)
			m.handleInputs(gtx)
			m.draw(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func newModel(ctx context.Context, cancel context.CancelFunc, rt *appRuntime.Runtime, w *app.Window) *model {
	m := &model{
		ctx:            ctx,
		cancel:         cancel,
		rt:             rt,
		w:              w,
		th:             material.NewTheme(),
		devices:        map[int]core.DeviceDTO{},
		events:         make([]core.EventDTO, 0, 4096),
		eventFilter:    "all",
		filterBtns:     map[string]*widget.Clickable{},
		logLevelBtnMap: map[string]*widget.Clickable{},
		cfgFields:      map[string]*widget.Editor{},
		cfgFlags:       map[string]*widget.Bool{},
		hFilterBtns:    map[string]*widget.Clickable{},
		hResult:        make(chan historyResult, 1),
		bootCh:         make(chan bootResult, 1),
		eventsC:        make(chan eventsResult, 1),
		statsCh:        make(chan core.StatsDTO, 1),
		saveCh:         make(chan saveResult, 1),
		deleteCh:       make(chan deleteResult, 1),
		rfResult:       make(chan rfResult, 1),
		pendingDevices: make(chan core.DeviceDTO, maxPendingUiDevices),
		pendingEvents:  make(chan core.EventDTO, maxPendingUiEvents),
		statusMsg:      "Starting runtime...",
	}

	m.objSearch.SingleLine, m.evtSearch.SingleLine, m.hSearch.SingleLine = true, true, true
	m.rfObjQuery.SingleLine, m.rfCodeQuery.SingleLine, m.rfGroups.SingleLine = true, true, true
	m.rfDetailZones.SingleLine, m.rfDetailParts.SingleLine = true, true
	m.settingsList.Axis = layout.Vertical
	m.objList.Axis, m.evtList.Axis = layout.Vertical, layout.Vertical
	m.hList.Axis = layout.Vertical
	m.rfObjList.Axis, m.rfCodeList.Axis, m.rfSumList.Axis = layout.Vertical, layout.Vertical, layout.Vertical
	m.hEventType = "all"
	for _, f := range eventFilters {
		m.filterBtns[f] = new(widget.Clickable)
		m.hFilterBtns[f] = new(widget.Clickable)
	}
	for _, lvl := range logLevels {
		m.logLevelBtnMap[lvl] = new(widget.Clickable)
	}
	m.th.Palette.Bg, m.th.Palette.Fg = cBg, cText
	m.th.Palette.ContrastBg, m.th.Palette.ContrastFg = cAccent, colorInk
	return m
}

func (m *model) handleDestroyEvent(e app.DestroyEvent) error {
	m.cancel()
	m.stopAsyncLoaders()
	m.mu.RLock()
	devicesCount := len(m.devices)
	eventsCount := len(m.events)
	m.mu.RUnlock()
	droppedDevices := m.dropDevices.Load()
	droppedEvents := m.dropEvents.Load()
	if e.Err != nil {
		log.Error().
			Err(e.Err).
			Int("devices", devicesCount).
			Int("events", eventsCount).
			Int64("dropped_devices", droppedDevices).
			Int64("dropped_events", droppedEvents).
			Msg("ui destroy event")
	} else {
		log.Warn().
			Int("devices", devicesCount).
			Int("events", eventsCount).
			Int64("dropped_devices", droppedDevices).
			Int64("dropped_events", droppedEvents).
			Msg("ui destroy event without error")
	}
	m.shutdownPlatform()
	return e.Err
}

func (m *model) uiWakeLoop() {
	t := time.NewTicker(uiRefreshTick)
	defer t.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-t.C:
			if len(m.pendingDevices) > 0 || len(m.pendingEvents) > 0 {
				m.w.Invalidate()
			}
		}
	}
}

func (m *model) start() error {
	if err := m.rt.Start(m.ctx); err != nil {
		return err
	}
	m.cfg = m.rt.GetConfig()
	m.historyLimit = m.initialGlobalLimit()
	m.eventsLimit = m.historyLimit
	m.activityTO = core.ParseDuration(m.cfg.Monitoring.PpkTimeout, 15*time.Minute)
	m.applyFontSize(m.cfg.UI.FontSize)
	m.loadCfgEditors(m.cfg)
	boot, err := m.rt.Bootstrap(m.ctx, m.eventsLimit)
	if err != nil {
		return err
	}
	for _, d := range boot.Devices {
		m.devices[d.ID] = d
	}
	m.events = append(m.events, boot.Events...)
	m.applyFiltersLocked()
	m.stats = m.rt.GetStats()
	m.statusMsg = "Running"
	log.Info().Msg("ui started")
	m.rt.SubscribeDevice(func(d core.DeviceDTO) {
		select {
		case m.pendingDevices <- d:
		default:
			m.dropDevices.Add(1)
		}
	})
	m.rt.SubscribeEvent(func(e core.EventDTO) {
		select {
		case m.pendingEvents <- e:
		default:
			m.dropEvents.Add(1)
		}
	})
	return nil
}

func (m *model) pullAsyncResults() {
	m.maybeRefreshTables()
	m.pullBootstrapResult()
	m.pullEventsResult()
	m.pullSaveResult()
	m.pullStatsResult()
	m.pullHistoryResult()
	m.pullDeleteResult()
	m.pullRfResult()
}

func (m *model) pullBootstrapResult() {
	select {
	case r := <-m.bootCh:
		if r.reqID != m.bootReqID.Load() {
			return
		}
		if r.err != nil {
			m.statusErr = "Reload failed: " + r.err.Error()
			log.Error().Err(r.err).Msg("reload all failed")
		} else {
			m.mu.Lock()
			m.devices = map[int]core.DeviceDTO{}
			for _, d := range r.boot.Devices {
				m.devices[d.ID] = d
			}
			m.events = append([]core.EventDTO{}, r.boot.Events...)
			m.applyFiltersLocked()
			m.mu.Unlock()
			if len(r.boot.Events) > m.eventsLimit {
				m.eventsLimit = len(r.boot.Events)
			}
			m.statusErr = ""
			log.Info().Int("devices", len(r.boot.Devices)).Int("events", len(r.boot.Events)).Msg("reload all completed")
		}
		m.w.Invalidate()
	default:
	}
}

func (m *model) pullEventsResult() {
	select {
	case r := <-m.eventsC:
		if r.reqID != m.eventsReqID.Load() {
			return
		}
		m.eventsBusy.Store(false)
		if r.err != nil {
			m.statusErr = "Events reload failed: " + r.err.Error()
			log.Error().Err(r.err).Msg("events reload failed")
		} else {
			m.mu.Lock()
			m.events = append([]core.EventDTO{}, r.events...)
			m.applyFiltersLocked()
			m.mu.Unlock()
			if r.limit > m.eventsLimit {
				m.eventsLimit = r.limit
			}
			m.statusErr = ""
			log.Info().Int("events", len(r.events)).Int("limit", r.limit).Msg("events reload completed")
		}
		m.w.Invalidate()
	default:
	}
}

func (m *model) pullSaveResult() {
	select {
	case r := <-m.saveCh:
		if r.err != nil {
			m.statusErr = "Save failed: " + r.err.Error()
			log.Error().Err(r.err).Msg("save config failed")
		} else {
			m.cfg = r.cfg
			m.historyLimit = m.initialGlobalLimit()
			m.eventsLimit = m.historyLimit
			m.activityTO = core.ParseDuration(m.cfg.Monitoring.PpkTimeout, 15*time.Minute)
			m.applyFontSize(m.cfg.UI.FontSize)
			m.loadCfgEditors(m.cfg)
			m.requestBootstrapReload(m.eventsLimit)
			m.statusErr = ""
			m.statusMsg = "Config saved"
			log.Info().Str("log_level", m.cfg.Logging.Level).Msg("config saved")
		}
		m.w.Invalidate()
	default:
	}
}

func (m *model) pullStatsResult() {
	select {
	case stats := <-m.statsCh:
		m.stats = stats
		m.w.Invalidate()
	default:
	}
}

func (m *model) pullHistoryResult() {
	select {
	case r := <-m.hResult:
		if r.reqID != m.historyReqID.Load() {
			return
		}
		m.historyBusy.Store(false)
		if r.err != nil {
			m.statusErr = "History load failed: " + r.err.Error()
			log.Error().Err(r.err).Int("device_id", r.id).Msg("history load failed")
		} else if m.hOpen && m.hDevice.ID == r.id {
			m.hRows = r.events
			if r.limit > m.hLimit {
				m.hLimit = r.limit
			}
			m.statusErr = ""
			log.Debug().Int("device_id", r.id).Int("rows", len(r.events)).Int("limit", r.limit).Msg("history loaded")
		}
		m.w.Invalidate()
	default:
	}
}

func (m *model) pullDeleteResult() {
	select {
	case r := <-m.deleteCh:
		m.delBusy.Store(false)
		m.delOpen = false
		if r.err != nil {
			m.statusErr = "Delete failed: " + r.err.Error()
			log.Error().Err(r.err).Int("device_id", r.id).Msg("delete device failed")
		} else {
			m.removeDeletedDevice(r.id)
		}
		m.w.Invalidate()
	default:
	}
}

func (m *model) removeDeletedDevice(deviceID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.devices, deviceID)
	filtered := m.events[:0]
	for _, e := range m.events {
		if !eventBelongsToDevice(e.DeviceID, deviceID) {
			filtered = append(filtered, e)
		}
	}
	m.events = filtered
	if m.hOpen && m.hDevice.ID == deviceID {
		m.hOpen = false
		m.historyBusy.Store(false)
	}
	m.applyFiltersLocked()
	m.statusErr = ""
	m.statusMsg = fmt.Sprintf("Object %03d deleted with history", deviceID)
	log.Info().Int("device_id", deviceID).Msg("device deleted from ui")
}

func (m *model) pullRfResult() {
	select {
	case r := <-m.rfResult:
		m.rfBusy.Store(false)
		if r.err != nil {
			m.statusErr = "Relay filter error: " + r.err.Error()
			log.Error().Err(r.err).Msg("relay filter result error")
		} else {
			if !m.rfOpen {
				m.rfOpen = true
				m.loadRfRule(r.rule)
			} else {
				m.statusMsg = "Relay filter rule saved"
				m.statusErr = ""
				m.rfOpen = false
				log.Info().Msg("relay filter rule updated from ui")
			}
		}
		m.w.Invalidate()
	default:
	}
}

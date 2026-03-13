package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cid_gio_gio/internal/config"
	"cid_gio_gio/internal/core"
	appLog "cid_gio_gio/internal/logger"
	appRuntime "cid_gio_gio/internal/runtime"

	"github.com/rs/zerolog/log"
)

const historyReloadDebounce = 250 * time.Millisecond

func (m *model) stopAsyncLoaders() {
	if m.historyDebounce != nil {
		m.historyDebounce.Stop()
		m.historyDebounce = nil
	}
	if m.bootCancel != nil {
		m.bootCancel()
		m.bootCancel = nil
	}
	if m.eventsCancel != nil {
		m.eventsCancel()
		m.eventsCancel = nil
	}
	if m.historyCancel != nil {
		m.historyCancel()
		m.historyCancel = nil
	}
}

func (m *model) maybeRefreshTables() {
	now := time.Now()
	if !m.lastUiRefresh.IsZero() && now.Sub(m.lastUiRefresh) < uiRefreshTick {
		return
	}
	m.lastUiRefresh = now
	m.drainPendingQueues()
}

func (m *model) drainPendingQueues() {
	upd := map[int]core.DeviceDTO{}
	eventBatch := make([]core.EventDTO, 0, uiDrainBatchSize)
	for pass := 0; pass < 4; pass++ {
		for i := 0; i < uiDrainBatchSize; i++ {
			select {
			case d := <-m.pendingDevices:
				upd[d.ID] = d
			default:
				i = uiDrainBatchSize
			}
		}
		for i := 0; i < uiDrainBatchSize; i++ {
			select {
			case e := <-m.pendingEvents:
				eventBatch = append(eventBatch, e)
			default:
				i = uiDrainBatchSize
			}
		}
		if len(m.pendingDevices) == 0 && len(m.pendingEvents) == 0 {
			break
		}
	}
	if len(upd) == 0 && len(eventBatch) == 0 {
		return
	}
	m.mu.Lock()
	for _, d := range upd {
		m.devices[d.ID] = d
	}
	if len(eventBatch) > 0 {
		now := time.Now()
		if m.liveWindowStart.IsZero() || now.Sub(m.liveWindowStart) >= time.Second {
			m.liveWindowStart = now
			m.liveWindowCount = 0
		}
		allow := liveEventsPerSecond - m.liveWindowCount
		if allow > 0 {
			if len(eventBatch) > allow {
				eventBatch = eventBatch[len(eventBatch)-allow:]
			}
			m.liveWindowCount += len(eventBatch)
			capN := max(1000, m.eventsLimit)
			capN = min(capN, maxAutoLoadEvents)
			m.events = prepend(m.events, eventBatch, capN)
		}
	}
	m.applyFiltersLocked()
	m.mu.Unlock()
	m.w.Invalidate()
}

func (m *model) statsLoop() {
	t := time.NewTicker(1500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-t.C:
			stats := m.rt.GetStats()
			select {
			case m.statsCh <- stats:
			default:
			}
		}
	}
}

func (m *model) requestBootstrapReload(limit int) {
	if m.bootCancel != nil {
		m.bootCancel()
	}
	reqID := m.bootReqID.Add(1)
	ctx, cancel := context.WithCancel(m.ctx)
	m.bootCancel = cancel
	go func() {
		boot, err := m.rt.Bootstrap(ctx, limit)
		select {
		case m.bootCh <- bootResult{reqID: reqID, boot: boot, err: err}:
		default:
		}
	}()
}

func (m *model) requestEventsReload(limit int) {
	if m.eventsCancel != nil {
		m.eventsCancel()
	}
	reqID := m.eventsReqID.Add(1)
	ctx, cancel := context.WithCancel(m.ctx)
	m.eventsCancel = cancel
	m.eventsBusy.Store(true)
	go func() {
		events, err := m.rt.FilterEvents(ctx, limit, "all", false, m.hideBlocked, "")
		select {
		case m.eventsC <- eventsResult{reqID: reqID, events: events, limit: limit, err: err}:
		default:
		}
	}()
}

func (m *model) saveConfigRemote(cfg config.AppConfig) {
	var err error
	if err := appLog.SetLevel(cfg.Logging.Level); err != nil {
		log.Warn().Err(err).Str("level", cfg.Logging.Level).Msg("pre-apply log level failed")
	}
	err = m.rt.SaveConfig(m.ctx, cfg)
	if err == nil {
		cfg = m.rt.GetConfig()
	}
	select {
	case m.saveCh <- saveResult{cfg: cfg, err: err}:
	default:
	}
}

func (m *model) openHistory(d core.DeviceDTO) {
	m.hOpen, m.hDevice, m.hRows, m.hQueryCache = true, d, nil, ""
	m.hSearch.SetText("")
	m.hLimit = m.initialDeviceHistoryLimit()
	m.historyBusy.Store(false)
	m.hEventType = "all"
	m.hHideBox.Value = false
	m.hHideTests = false
	m.hHideBlockedBox.Value = false
	m.hHideBlocked = false
	m.statusMsg = fmt.Sprintf("Open history for object %03d", d.ID)
	log.Debug().Int("device_id", d.ID).Msg("open history")
	m.w.Invalidate()
	m.requestHistoryReloadNow()
}

func (m *model) requestHistoryReload() {
	m.scheduleHistoryReload(historyReloadDebounce)
}

func (m *model) requestHistoryReloadNow() {
	m.scheduleHistoryReload(0)
}

func (m *model) scheduleHistoryReload(delay time.Duration) {
	if !m.hOpen {
		return
	}
	req := historyRequest{
		id:        m.hDevice.ID,
		limit:     m.hLimit,
		eventType: m.hEventType,
		hideTests: m.hHideTests,
		hideBlocked: m.hHideBlocked,
		query:     strings.TrimSpace(m.hSearch.Text()),
	}
	if m.historyDebounce != nil {
		m.historyDebounce.Stop()
		m.historyDebounce = nil
	}
	if m.historyCancel != nil {
		m.historyCancel()
	}
	reqID := m.historyReqID.Add(1)
	ctx, cancel := context.WithCancel(m.ctx)
	m.historyCancel = cancel
	m.historyBusy.Store(true)
	run := func() {
		m.reloadHistory(ctx, reqID, req)
	}
	if delay <= 0 {
		go run()
		return
	}
	m.historyDebounce = time.AfterFunc(delay, run)
}

func (m *model) reloadHistory(ctx context.Context, reqID uint64, req historyRequest) {
	e, err := m.rt.FilterDeviceHistory(
		ctx,
		req.id,
		req.limit,
		time.Time{},
		time.Time{},
		req.eventType,
		req.hideTests,
		req.hideBlocked,
		req.query,
	)
	select {
	case m.hResult <- historyResult{reqID: reqID, id: req.id, events: e, limit: req.limit, err: err}:
	default:
	}
}

func (m *model) deleteDeviceRemote(deviceID int) {
	err := m.rt.DeleteDeviceWithHistory(m.ctx, deviceID)
	select {
	case m.deleteCh <- deleteResult{id: deviceID, err: err}:
	default:
	}
}

func (m *model) maybeLoadMoreEvents() {
	capLimit := m.globalEventsCap()
	if m.eventsBusy.Load() {
		return
	}
	pos := m.evtList.Position
	remaining := len(m.filteredEvents) - (pos.First + pos.Count)
	if remaining > 10 {
		return
	}
	if m.eventsLimit >= capLimit {
		return
	}
	next := min(capLimit, m.eventsLimit+eventsLoadChunk)
	if next <= m.eventsLimit {
		return
	}
	m.requestEventsReload(next)
}

func (m *model) maybeLoadMoreHistory() {
	capLimit := m.deviceHistoryCap()
	if !m.hOpen || m.historyBusy.Load() {
		return
	}
	pos := m.hList.Position
	remaining := len(m.hRows) - (pos.First + pos.Count)
	if remaining > 8 {
		return
	}
	if m.hLimit >= capLimit {
		return
	}
	next := min(capLimit, m.hLimit+historyLoadChunk)
	if next <= m.hLimit {
		return
	}
	m.hLimit = next
	m.requestHistoryReloadNow()
}

func (m *model) applyFilters() { m.mu.Lock(); m.applyFiltersLocked(); m.mu.Unlock() }

func (m *model) globalEventsCap() int {
	if m.cfg.Server.MaxGlobalEvents > 0 {
		return min(maxAutoLoadEvents, m.cfg.Server.MaxGlobalEvents)
	}
	return maxAutoLoadEvents
}

func (m *model) deviceHistoryCap() int {
	if m.cfg.Server.MaxDeviceEvents > 0 {
		return min(maxAutoLoadHistory, m.cfg.Server.MaxDeviceEvents)
	}
	return maxAutoLoadHistory
}

func (m *model) initialGlobalLimit() int {
	return min(m.globalEventsCap(), max(1, m.cfg.History.GlobalLimit))
}

func (m *model) initialDeviceHistoryLimit() int {
	base := max(1, m.cfg.History.LogLimit)
	if base < historyLoadChunk {
		base = historyLoadChunk
	}
	return min(m.deviceHistoryCap(), base)
}

func (m *model) applyFiltersLocked() {
	q := strings.ToLower(strings.TrimSpace(m.deviceFilter))
	devs := make([]core.DeviceDTO, 0, len(m.devices))
	a, ia := 0, 0
	for _, d := range m.devices {
		if isStale(d.LastEventTime, m.activityTO) {
			ia++
		} else {
			a++
		}
		if q == "" || strings.Contains(strings.ToLower(d.Name+" "+d.ClientAddr+" "+d.LastEvent), q) {
			devs = append(devs, d)
		}
	}
	sort.Slice(devs, func(i, j int) bool { return devs[i].ID < devs[j].ID })
	fEvents := make([]core.EventDTO, 0, min(len(m.events), m.historyLimit))
	for _, e := range m.events {
		if appRuntime.EventMatchesFilter(e, m.eventFilter, m.hideTests, m.eventQuery) {
			if m.hideBlocked && e.RelayBlocked {
				continue
			}
			fEvents = append(fEvents, e)
			if len(fEvents) >= m.historyLimit {
				break
			}
		}
	}
	m.filteredDevices, m.filteredEvents, m.activeDevices, m.inactiveDevices, m.visibleEvents = devs, fEvents, a, ia, len(fEvents)
}

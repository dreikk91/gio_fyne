package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cid_fyne/internal/config"
	"cid_fyne/internal/core"
	"cid_fyne/internal/data"
	appLog "cid_fyne/internal/logger"
	"cid_fyne/internal/netrelay"

	"github.com/rs/zerolog/log"
)

type Runtime struct {
	configStore *config.Store
	config      config.AppConfig

	metrics  *core.Metrics
	queue    *core.MessageQueue
	repo     *data.Repository
	server   *netrelay.TCPServer
	relay    *netrelay.RelayClient
	eventMap core.CIDEventMap

	deviceQueue chan netrelay.DeviceEvent
	eventQueue  chan netrelay.DeviceEvent
	runCtx      context.Context
	cancel      context.CancelFunc

	lifecycle sync.Mutex
	wg        sync.WaitGroup
	started   bool

	onDeviceMu sync.RWMutex
	onEventMu  sync.RWMutex
	onDeviceDelMu sync.RWMutex
	onDevice   []func(core.DeviceDTO)
	onEvent    []func(core.EventDTO)
	onDeviceDel []func(int)

	droppedDevice atomic.Int64
	droppedEvent  atomic.Int64
}

const (
	eventPersistBatchSize  = 1000
	eventPersistInterval   = 200 * time.Millisecond
	eventShutdownFlushWait = 30 * time.Second
	eventShutdownDrainMax  = 10000
	dbMaintenanceTimeout   = 10 * time.Minute
	maxBootstrapEvents     = 50000
	catalogSyncDelay       = 30 * time.Second
	minWorkerBuffer        = 5000
	maxWorkerBuffer        = 50000
	queueBacklogMultiplier = 10
)

func NewRuntime(configPath string) *Runtime {
	return &Runtime{
		configStore: config.NewStore(configPath),
		config:      config.DefaultConfig(),
		metrics:     core.NewMetrics(),
		queue:       core.NewMessageQueue(5000),
		eventMap:    core.CIDEventMap{},
	}
}

func (r *Runtime) SubscribeDevice(fn func(core.DeviceDTO)) {
	r.onDeviceMu.Lock()
	defer r.onDeviceMu.Unlock()
	r.onDevice = append(r.onDevice, fn)
}

func (r *Runtime) SubscribeEvent(fn func(core.EventDTO)) {
	r.onEventMu.Lock()
	defer r.onEventMu.Unlock()
	r.onEvent = append(r.onEvent, fn)
}

func (r *Runtime) SubscribeDeviceDeleted(fn func(int)) {
	r.onDeviceDelMu.Lock()
	defer r.onDeviceDelMu.Unlock()
	r.onDeviceDel = append(r.onDeviceDel, fn)
}

func (r *Runtime) Start(ctx context.Context) error {
	defer appLog.RecoverPanic("runtime-start")
	r.lifecycle.Lock()
	defer r.lifecycle.Unlock()
	if r.started {
		log.Debug().Msg("runtime start skipped: already started")
		return nil
	}
	cfg, err := r.configStore.Load()
	if err != nil {
		log.Error().Err(err).Msg("runtime config load failed")
		return err
	}
	r.config = cfg
	if err := appLog.SetupFromAppConfig(r.config.Logging); err != nil {
		return fmt.Errorf("setup logger: %w", err)
	}
	log.Info().
		Str("server", fmt.Sprintf("%s:%s", r.config.Server.Host, r.config.Server.Port)).
		Str("relay", fmt.Sprintf("%s:%s", r.config.Client.Host, r.config.Client.Port)).
		Int("queue", r.config.Queue.BufferSize).
		Msg("runtime starting")

	eventsPath := ResolveEventsPath()
	raw, err := os.ReadFile(eventsPath)
	if err != nil {
		log.Error().Err(err).Str("path", eventsPath).Msg("read events map failed")
		return fmt.Errorf("read events: %w", err)
	}
	list, err := core.LoadCIDEventList(string(raw))
	if err != nil {
		log.Error().Err(err).Str("path", eventsPath).Msg("parse events map failed")
		return fmt.Errorf("parse events: %w", err)
	}
	r.eventMap = core.BuildCIDEventMap(list)

	dbPath := ResolveDataDBPath()
	repo, err := data.NewRepository(dbPath)
	if err != nil {
		log.Error().Err(err).Str("db_path", dbPath).Msg("repository init failed")
		return err
	}
	r.repo = repo
	log.Info().Str("db_path", dbPath).Msg("repository connected")

	entries := make([]core.EventCatalogEntry, 0, len(list))
	for _, item := range list {
		entries = append(entries, core.EventCatalogEntry{
			Code:     item.ContactIDCode,
			Type:     item.TypeCodeMesUK,
			Desc:     item.CodeMesUK,
			Category: core.Classify(item.ContactIDCode, item.TypeCodeMesUK, item.CodeMesUK),
			Version:  1,
		})
	}
	queueSize := max(1000, r.config.Queue.BufferSize)
	queueSize = min(queueSize, maxWorkerBuffer)
	r.queue = core.NewMessageQueue(queueSize)
	r.server = netrelay.NewTCPServer(r.config.Server, r.config.CidRules, r.queue, r.metrics)
	rule, err := r.repo.GetRelayFilterRule(context.Background())
	if err == nil {
		r.server.UpdateRelayFilter(rule)
	}
	r.relay = netrelay.NewRelayClient(r.config.Client, r.queue, r.metrics)
	if err == nil {
		r.relay.UpdateRelayFilter(rule)
	}
	workerBuffer := r.config.Queue.BufferSize * queueBacklogMultiplier
	workerBuffer = max(minWorkerBuffer, workerBuffer)
	workerBuffer = min(maxWorkerBuffer, workerBuffer)
	r.deviceQueue = make(chan netrelay.DeviceEvent, workerBuffer)
	r.eventQueue = make(chan netrelay.DeviceEvent, workerBuffer)
	r.server.SetCallbacks(r.onDeviceUpdated, r.onEventCreated)

	r.runCtx, r.cancel = context.WithCancel(ctx)
	r.wg.Add(6)
	go func() {
		defer r.wg.Done()
		defer appLog.RecoverPanic("runtime-server-run")
		if err := r.server.Run(r.runCtx); err != nil {
			log.Error().Err(err).Msg("tcp server stopped with error")
		}
	}()
	go func() {
		defer r.wg.Done()
		r.relay.Run(r.runCtx)
	}()
	go func() {
		defer r.wg.Done()
		r.processDeviceUpdates(r.runCtx)
	}()
	go func() {
		defer r.wg.Done()
		r.processEvents(r.runCtx)
	}()
	go func() {
		defer r.wg.Done()
		r.maintenanceLoop(r.runCtx)
	}()
	go func() {
		defer r.wg.Done()
		r.deferredCatalogSync(r.runCtx, entries)
	}()

	r.started = true
	log.Info().Int("worker_buffer", workerBuffer).Msg("runtime started")
	return nil
}

func (r *Runtime) Stop(ctx context.Context) error {
	defer appLog.RecoverPanic("runtime-stop")
	r.lifecycle.Lock()
	defer r.lifecycle.Unlock()
	if !r.started {
		return nil
	}
	log.Info().Msg("runtime stopping")
	if r.cancel != nil {
		r.cancel()
	}
	if r.server != nil {
		r.server.Stop()
	}
	if r.relay != nil {
		r.relay.Stop()
	}
	waitDone := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-ctx.Done():
		return fmt.Errorf("runtime stop timeout/canceled: %w", ctx.Err())
	}
	if r.repo != nil {
		_ = r.repo.Close()
		r.repo = nil
	}
	r.started = false
	log.Info().Msg("runtime stopped")
	return nil
}

func (r *Runtime) Bootstrap(ctx context.Context, limit int) (core.BootstrapDTO, error) {
	if r.repo == nil {
		return core.BootstrapDTO{}, fmt.Errorf("runtime not started")
	}
	if limit > maxBootstrapEvents {
		limit = maxBootstrapEvents
	}
	devices, err := r.repo.GetDevices(ctx)
	if err != nil {
		return core.BootstrapDTO{}, err
	}
	events, err := r.repo.GetEvents(ctx, limit)
	if err != nil {
		return core.BootstrapDTO{}, err
	}
	log.Debug().Int("devices", len(devices)).Int("events", len(events)).Int("limit", limit).Msg("bootstrap repository loaded")
	return core.BootstrapDTO{Devices: devices, Events: events}, nil
}

func (r *Runtime) FilterEvents(ctx context.Context, limit int, eventType string, hideTests bool, hideBlocked bool, query string) ([]core.EventDTO, error) {
	if r.repo == nil {
		return nil, fmt.Errorf("runtime not started")
	}
	events, err := r.repo.GetEventsFiltered(ctx, limit, eventType, hideTests, hideBlocked, query)
	if err == nil {
		log.Debug().Int("events", len(events)).Int("limit", limit).Str("type", eventType).Bool("hide_tests", hideTests).Bool("hide_blocked", hideBlocked).Str("query", query).Msg("events filtered")
	}
	return events, err
}

func (r *Runtime) FilterDeviceHistory(ctx context.Context, deviceID, limit int, from, to time.Time, eventType string, hideTests bool, hideBlocked bool, query string) ([]core.EventDTO, error) {
	if r.repo == nil {
		return nil, fmt.Errorf("runtime not started")
	}
	events, err := r.repo.GetDeviceEventsFiltered(ctx, deviceID, limit, from, to, eventType, hideTests, hideBlocked, query)
	if err == nil {
		log.Debug().Int("device_id", deviceID).Int("events", len(events)).Int("limit", limit).Str("type", eventType).Bool("hide_tests", hideTests).Bool("hide_blocked", hideBlocked).Str("query", query).Msg("device history filtered")
	}
	return events, err
}

func (r *Runtime) DeleteDeviceWithHistory(ctx context.Context, deviceID int) error {
	if r.repo == nil {
		return fmt.Errorf("runtime not started")
	}
	err := r.repo.DeleteDeviceWithHistory(ctx, deviceID)
	if err != nil {
		log.Error().Err(err).Int("device_id", deviceID).Msg("delete device with history failed")
		return err
	}
	r.emitDeviceDeleted(deviceID)
	log.Info().Int("device_id", deviceID).Msg("device with history deleted")
	return nil
}

func (r *Runtime) emitDeviceDeleted(deviceID int) {
	r.onDeviceDelMu.RLock()
	subs := append([]func(int){}, r.onDeviceDel...)
	r.onDeviceDelMu.RUnlock()
	for _, fn := range subs {
		func(cb func(int)) {
			defer appLog.RecoverPanic("runtime-emit-device-deleted")
			cb(deviceID)
		}(fn)
	}
}

func (r *Runtime) GetConfig() config.AppConfig {
	return r.config
}

func (r *Runtime) SaveConfig(ctx context.Context, cfg config.AppConfig) error {
	config.Normalize(&cfg)
	if err := r.configStore.Save(cfg); err != nil {
		log.Error().Err(err).Msg("save config failed")
		return err
	}
	if err := appLog.SetLevel(cfg.Logging.Level); err != nil {
		log.Warn().Err(err).Str("level", cfg.Logging.Level).Msg("set log level failed")
	}
	if err := appLog.SetupFromAppConfig(cfg.Logging); err != nil {
		log.Warn().Err(err).Msg("reconfigure logger failed")
	}
	_ = r.Stop(ctx)
	r.config = cfg
	log.Info().Str("level", cfg.Logging.Level).Msg("config saved, restarting runtime")
	return r.Start(ctx)
}

func (r *Runtime) GetRelayFilterRule(ctx context.Context) (core.RelayFilterRule, error) {
	if r.repo == nil {
		return core.RelayFilterRule{}, fmt.Errorf("runtime not started")
	}
	return r.repo.GetRelayFilterRule(ctx)
}

func (r *Runtime) SaveRelayFilterRule(ctx context.Context, rule core.RelayFilterRule) error {
	if r.repo == nil {
		return fmt.Errorf("runtime not started")
	}
	if err := r.repo.SaveRelayFilterRule(ctx, rule); err != nil {
		return err
	}
	if r.server != nil {
		r.server.UpdateRelayFilter(rule)
	}
	if r.relay != nil {
		r.relay.UpdateRelayFilter(rule)
	}
	log.Info().Msg("relay filter rule saved")
	return nil
}

func (r *Runtime) GetEventTypes(ctx context.Context) ([]core.EventTypeDTO, error) {
	if r.repo == nil {
		return nil, fmt.Errorf("runtime not started")
	}
	return r.repo.GetEventTypes(ctx)
}

func (r *Runtime) SaveEventTypeColors(ctx context.Context, key, color, fontColor string) error {
	if r.repo == nil {
		return fmt.Errorf("runtime not started")
	}
	return r.repo.SaveEventTypeColors(ctx, key, color, fontColor)
}

func (r *Runtime) GetStats() core.StatsDTO {
	return r.metrics.Snapshot()
}

func (r *Runtime) GetEventList() []core.CIDEvent {
	r.lifecycle.Lock()
	defer r.lifecycle.Unlock()
	return r.eventMap.List()
}

func (r *Runtime) GetEventCatalogCategories() map[string]string {
	r.lifecycle.Lock()
	defer r.lifecycle.Unlock()
	if r.repo == nil {
		return nil
	}
	cats, err := r.repo.GetEventCatalogCategories(context.Background())
	if err != nil {
		return nil
	}
	return cats
}

func (r *Runtime) GetDevices() []core.DeviceDTO {
	r.lifecycle.Lock()
	defer r.lifecycle.Unlock()
	if r.repo == nil {
		return nil
	}
	devices, _ := r.repo.GetDevices(context.Background())
	return devices
}

func (r *Runtime) processDeviceUpdates(ctx context.Context) {
	defer appLog.RecoverPanic("runtime-process-device-updates")
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-r.deviceQueue:
			if ctx.Err() != nil {
				return // shutdown requested, do not attempt to write with canceled context
			}
			dto := core.DeviceDTO{
				ID:            e.DeviceID,
				Name:          fmt.Sprintf("%03d", e.DeviceID),
				ClientAddr:    strings.TrimSpace(e.Remote),
				LastEvent:     e.Data,
				LastEventTime: e.Time,
			}
			if r.repo != nil {
				if err := retry(ctx, 3, func() error { return r.repo.SaveDevice(ctx, dto) }); err != nil && ctx.Err() == nil {
					log.Error().Err(err).Int("device_id", dto.ID).Msg("save device failed")
				}
			}
			r.emitDevice(dto)
		}
	}
}

func (r *Runtime) processEvents(ctx context.Context) {
	defer appLog.RecoverPanic("runtime-process-events")
	ticker := time.NewTicker(eventPersistInterval)
	defer ticker.Stop()
	batch := make([]core.EventDTO, 0, eventPersistBatchSize)

	flush := func(flushCtx context.Context) {
		if len(batch) == 0 {
			return
		}
		toSave := make([]core.EventDTO, len(batch))
		copy(toSave, batch)
		batch = batch[:0]
		if err := r.flushEventBatch(flushCtx, toSave); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || flushCtx.Err() != nil {
				log.Debug().Int("count", len(toSave)).Msg("flush event batch interrupted by shutdown")
				return
			}
			log.Error().Err(err).Int("count", len(toSave)).Msg("flush event batch failed")
		}
	}

	for {
		select {
		case <-ctx.Done():
			// Drain a bounded number of queued events to avoid hanging shutdown.
			drained := 0
		DrainShutdown:
			for {
				if drained >= eventShutdownDrainMax {
					break
				}
				select {
				case e := <-r.eventQueue:
					drained++
					if len(e.Data) >= 20 && r.repo != nil {
						code := e.Data[11:15]
						hit, ok := r.eventMap.TryGet(code)
						evtType := "Unknown"
						evtDesc := "Unknown code"
						if ok {
							evtType = hit.TypeCodeMesUK
							evtDesc = hit.CodeMesUK
						}
						batch = append(batch, core.EventDTO{
							Time:         e.Time,
							DeviceID:     fmt.Sprintf("%d", e.DeviceID),
							Code:         code,
							Type:         evtType,
							Desc:         evtDesc,
							Zone:         fmt.Sprintf("Zone %s|Group %s", e.Data[17:20], e.Data[15:17]),
							Priority:     core.DeterminePriority(code),
							Category:     core.Classify(code, evtType, evtDesc),
							RelayBlocked: e.RelayBlocked,
						})
					}
				default:
					break DrainShutdown
				}
			}
			flushCtx, cancel := context.WithTimeout(context.Background(), eventShutdownFlushWait)
			flush(flushCtx)
			cancel()
			return
		case e := <-r.eventQueue:
			if ctx.Err() != nil {
				continue // context is canceled, let the case <-ctx.Done() handle the batch
			}
			if len(e.Data) < 20 || r.repo == nil {
				continue
			}
			code := e.Data[11:15]
			group := e.Data[15:17]
			zone := e.Data[17:20]
			hit, ok := r.eventMap.TryGet(code)
			evtType := "Unknown"
			evtDesc := "Unknown code"
			if ok {
				evtType = hit.TypeCodeMesUK
				evtDesc = hit.CodeMesUK
			}
			category := core.Classify(code, evtType, evtDesc)
			dto := core.EventDTO{
				Time:         e.Time,
				DeviceID:     fmt.Sprintf("%d", e.DeviceID),
				Code:         code,
				Type:         evtType,
				Desc:         evtDesc,
				Zone:         fmt.Sprintf("Zone %s|Group %s", zone, group),
				Priority:     core.DeterminePriority(code),
				Category:     category,
				RelayBlocked: e.RelayBlocked,
			}
			batch = append(batch, dto)
			if len(batch) >= eventPersistBatchSize {
				flush(ctx)
			}
			r.emitEvent(dto)
		case <-ticker.C:
			if ctx.Err() == nil {
				flush(ctx)
			}
		}
	}
}

func (r *Runtime) flushEventBatch(ctx context.Context, batch []core.EventDTO) error {
	if len(batch) == 0 || r.repo == nil {
		return nil
	}
	err := retry(ctx, 3, func() error { return r.repo.SaveEventsBatch(ctx, batch) })
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
		return err
	}
	var firstErr error
	for _, evt := range batch {
		if ctx.Err() != nil {
			if firstErr == nil {
				firstErr = ctx.Err()
			}
			break
		}
		if err := retry(ctx, 3, func() error { return r.repo.SaveEvent(ctx, evt) }); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				if firstErr == nil {
					firstErr = err
				}
				break
			}
			if firstErr == nil {
				firstErr = err
			}
			log.Error().Err(err).Str("device_id", evt.DeviceID).Str("code", evt.Code).Msg("save event fallback failed")
		}
	}
	return firstErr
}

func (r *Runtime) maintenanceLoop(ctx context.Context) {
	defer appLog.RecoverPanic("runtime-maintenance-loop")
	interval := time.Duration(max(1, r.config.History.CleanupIntervalHours)) * time.Hour
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if r.repo == nil {
				continue
			}
			mxCtx, cancel := context.WithTimeout(ctx, dbMaintenanceTimeout)
			err := r.repo.RunMaintenance(mxCtx, data.MaintenanceOptions{
				RetentionDays:  r.config.History.RetentionDays,
				ArchiveEnabled: r.config.History.ArchiveEnabled,
				ArchivePath:    ResolveArchiveDBPath(r.config.History.ArchiveDBPath),
				BatchSize:      r.config.History.MaintenanceBatch,
			})
			cancel()
			if err != nil {
				log.Warn().Err(err).Dur("interval", interval).Msg("db maintenance failed")
				continue
			}
			log.Info().Dur("interval", interval).Msg("db maintenance completed")
		}
	}
}

func (r *Runtime) deferredCatalogSync(ctx context.Context, entries []core.EventCatalogEntry) {
	defer appLog.RecoverPanic("runtime-catalog-sync")
	if len(entries) == 0 {
		return
	}
	timer := time.NewTimer(catalogSyncDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	if r.repo == nil || ctx.Err() != nil {
		return
	}
	syncCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := r.repo.UpsertEventCatalog(syncCtx, entries); err != nil {
		log.Warn().Err(err).Int("entries", len(entries)).Msg("deferred event catalog upsert failed")
		return
	}
	log.Info().Int("entries", len(entries)).Msg("deferred event catalog upsert completed")
}

func (r *Runtime) onDeviceUpdated(e netrelay.DeviceEvent) {
	before := r.droppedDevice.Load()
	tryEnqueue(r.deviceQueue, e, &r.droppedDevice)
	after := r.droppedDevice.Load()
	if after > before && after%100 == 0 {
		log.Warn().Int64("dropped_device", after).Msg("device queue dropping updates")
	}
}

func (r *Runtime) onEventCreated(e netrelay.DeviceEvent) {
	before := r.droppedEvent.Load()
	tryEnqueue(r.eventQueue, e, &r.droppedEvent)
	after := r.droppedEvent.Load()
	if after > before && after%100 == 0 {
		log.Warn().Int64("dropped_event", after).Msg("event queue dropping updates")
	}
}

func (r *Runtime) emitDevice(dto core.DeviceDTO) {
	r.onDeviceMu.RLock()
	subs := append([]func(core.DeviceDTO){}, r.onDevice...)
	r.onDeviceMu.RUnlock()
	for _, fn := range subs {
		func(cb func(core.DeviceDTO)) {
			defer appLog.RecoverPanic("runtime-emit-device")
			cb(dto)
		}(fn)
	}
}

func (r *Runtime) emitEvent(dto core.EventDTO) {
	r.onEventMu.RLock()
	subs := append([]func(core.EventDTO){}, r.onEvent...)
	r.onEventMu.RUnlock()
	for _, fn := range subs {
		func(cb func(core.EventDTO)) {
			defer appLog.RecoverPanic("runtime-emit-event")
			cb(dto)
		}(fn)
	}
}

func retry(ctx context.Context, maxAttempts int, fn func() error) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var err error
	for i := 1; i <= maxAttempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		if i == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(100*i) * time.Millisecond):
		}
	}
	return err
}

func tryEnqueue(ch chan netrelay.DeviceEvent, item netrelay.DeviceEvent, dropped *atomic.Int64) {
	select {
	case ch <- item:
		return
	default:
	}
	// Queue is full: drop one oldest item and push the freshest update.
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- item:
	default:
		dropped.Add(1)
	}
}

func EventMatchesFilter(evt core.EventDTO, filter string, hideTests bool, query string) bool {
	if !strings.EqualFold(filter, "all") && !strings.EqualFold(evt.Category, filter) {
		return false
	}
	if hideTests && strings.EqualFold(evt.Category, "test") {
		return false
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	hay := strings.ToLower(strings.TrimSpace(evt.DeviceID + " " + evt.Code + " " + evt.Type + " " + evt.Desc + " " + evt.Zone))
	return strings.Contains(hay, q)
}

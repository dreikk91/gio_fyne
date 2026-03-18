package ui

import (
	"context"
	"image/color"
	"strconv"
	"time"

	"cid_fyne/internal/core"
	appLog "cid_fyne/internal/logger"

	"github.com/rs/zerolog/log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
)

func Run(ctx context.Context, rt core.Backend) error {
	defer appLog.RecoverPanic("ui-run")
	uiCtx, cancel := context.WithCancel(ctx)

	a := app.NewWithID("cid.retranslator.fyne")
	w := a.NewWindow("CID Retranslator - Fyne")

	m := newModel(uiCtx, cancel, rt, a, w)
	if err := m.start(); err != nil {
		log.Error().Err(err).Msg("ui start failed")
		return err
	}

	m.buildMainUI()
	m.installCloseIntercept()
	m.startTrayIfNeeded()

	go m.uiWakeLoop()
	go m.statsLoop()

	w.Show()
	if m.cfg.UI.StartMinimized {
		go func() {
			// Wait for tray to be fully ready before showing notification
			// Check trayReady flag with timeout
			for i := 0; i < 50; i++ {
				if m.trayReady.Load() {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			m.hideWindow()
			m.notifyHiddenToTray()
		}()
	}

	a.Run()
	m.cancel()
	m.stopAsyncLoaders()
	m.shutdownPlatform()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()
	_ = rt.Stop(stopCtx)

	return nil
}

func newModel(ctx context.Context, cancel context.CancelFunc, rt core.Backend, appInst fyne.App, win fyne.Window) *model {
	m := &model{
		ctx:                 ctx,
		cancel:              cancel,
		rt:                  rt,
		app:                 appInst,
		win:                 win,
		theme:               newModernTheme(12),
		devices:             map[int]core.DeviceDTO{},
		events:              make([]core.EventDTO, 0, 4096),
		eventFilter:         "all",
		eventFilterBtns:     map[string]*filterButtonRef{},
		cfgEntries:          map[string]*widget.Entry{},
		cfgChecks:           map[string]*widget.Check{},
		hFilterBtns:         map[string]*filterButtonRef{},
		hResult:             make(chan historyResult, 1),
		bootCh:              make(chan bootResult, 1),
		eventsCh:            make(chan eventsResult, 1),
		statsCh:             make(chan core.StatsDTO, 1),
		saveCh:              make(chan saveResult, 1),
		deleteCh:            make(chan deleteResult, 1),
		rfResult:            make(chan rfResult, 1),
		pendingDevices:      make(chan core.DeviceDTO, maxPendingUiDevices),
		pendingEvents:       make(chan core.EventDTO, maxPendingUiEvents),
		statusMsg:           "Starting runtime...",
		selObjRow:           -1,
		selEvtRow:           -1,
		selHistRow:          -1,
		deviceCategoryCache: map[int]string{},
		categoryColors:      map[string]color.NRGBA{},
		categoryFontColors:  map[string]color.NRGBA{},
		deletedDevices:      make(chan int, 100),
	}
	appInst.Settings().SetTheme(m.theme)
	rt.SubscribeDeviceDeleted(func(id int) {
		select {
		case m.deletedDevices <- id:
		default:
		}
	})
	return m
}

func (m *model) start() error {
	if err := m.rt.Start(m.ctx); err != nil {
		return err
	}
	m.cfg = m.rt.GetConfig()
	m.historyLimit = m.initialGlobalLimit()
	m.eventsLimit = m.historyLimit
	m.activityTO = core.ParseDuration(m.cfg.Monitoring.PpkTimeout, 15*time.Minute)
	m.theme = newModernTheme(m.cfg.UI.FontSize)
	m.app.Settings().SetTheme(m.theme)
	m.loadCfgEditors(m.cfg)
	m.loadCategoryColors()

	boot, err := m.rt.Bootstrap(m.ctx, m.eventsLimit)
	if err != nil {
		return err
	}
	for _, d := range boot.Devices {
		m.devices[d.ID] = d
	}
	m.events = append(m.events, boot.Events...)
	// Populate category cache from initial events
	for _, e := range m.events {
		if id, err := strconv.Atoi(e.DeviceID); err == nil {
			if _, ok := m.deviceCategoryCache[id]; !ok {
				m.deviceCategoryCache[id] = e.Category
			}
		}
	}
	m.applyFiltersLocked()
	m.stats = m.rt.GetStats()
	m.statusMsg = "Running"
	log.Info().
		Int("devices", len(boot.Devices)).
		Int("events", len(boot.Events)).
		Msg("ui bootstrap loaded")

	log.Info().Msg("ui started (fyne)")
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

func (m *model) uiWakeLoop() {
	t := time.NewTicker(uiRefreshTick)
	defer t.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-t.C:
			m.pullAsyncResults()
		}
	}
}

package ui

import (
	"context"
	"time"

	"cid_gio_gio/internal/core"
	appLog "cid_gio_gio/internal/logger"
	appRuntime "cid_gio_gio/internal/runtime"

	"github.com/rs/zerolog/log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
)

func Run(ctx context.Context, rt *appRuntime.Runtime) error {
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

func newModel(ctx context.Context, cancel context.CancelFunc, rt *appRuntime.Runtime, appInst fyne.App, win fyne.Window) *model {
	m := &model{
		ctx:             ctx,
		cancel:          cancel,
		rt:              rt,
		app:             appInst,
		win:             win,
		theme:           newWin10Theme(12),
		devices:         map[int]core.DeviceDTO{},
		events:          make([]core.EventDTO, 0, 4096),
		eventFilter:     "all",
		eventFilterBtns: map[string]*widget.Button{},
		cfgEntries:      map[string]*widget.Entry{},
		cfgChecks:       map[string]*widget.Check{},
		hFilterBtns:     map[string]*widget.Button{},
		hResult:         make(chan historyResult, 1),
		bootCh:          make(chan bootResult, 1),
		eventsCh:        make(chan eventsResult, 1),
		statsCh:         make(chan core.StatsDTO, 1),
		saveCh:          make(chan saveResult, 1),
		deleteCh:        make(chan deleteResult, 1),
		rfResult:        make(chan rfResult, 1),
		pendingDevices:  make(chan core.DeviceDTO, maxPendingUiDevices),
		pendingEvents:   make(chan core.EventDTO, maxPendingUiEvents),
		statusMsg:       "Starting runtime...",
		selObjRow:       -1,
		selEvtRow:       -1,
		selHistRow:      -1,
	}
	appInst.Settings().SetTheme(m.theme)
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

//go:build windows

package walk

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"cid_fyne/internal/config"
	"cid_fyne/internal/core"
	appLog "cid_fyne/internal/logger"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"github.com/rs/zerolog/log"
)

const (
	maxPendingUiDevices = 20000
	maxPendingUiEvents  = 120000
	uiDrainBatchSize    = 5000
	uiRefreshTick       = 1 * time.Second
	uiScrollTick        = 250 * time.Millisecond
	eventsLoadChunk     = 500
	historyLoadChunk    = 250

	dwmwaWindowCornerPreference = 33
	dwmwcpRound                 = 2
)

var (
	dwmapiDLL               = syscall.NewLazyDLL("dwmapi.dll")
	dwmSetWindowAttributeFn = dwmapiDLL.NewProc("DwmSetWindowAttribute")
)

type walkApp struct {
	ctx    context.Context
	cancel context.CancelFunc
	rt     core.Backend

	mw         *walk.MainWindow
	notifyIcon *walk.NotifyIcon

	mu         sync.RWMutex
	devices    map[int]core.DeviceDTO
	events     []core.EventDTO
	stats      core.StatsDTO
	cfg        config.AppConfig
	status     string
	statusErr  string
	activityTO time.Duration

	filteredDevices []core.DeviceDTO
	filteredEvents  []core.EventDTO
	activeDevices   int
	inactiveDevices int
	visibleEvents   int

	deviceFilter string
	eventFilter  string
	eventQuery   string
	hideTests    bool
	historyLimit int
	eventsLimit  int

	pendingDevices chan core.DeviceDTO
	pendingEvents  chan core.EventDTO
	pendingDeleted chan int
	dropDevices    atomic.Int64
	dropEvents     atomic.Int64
	eventsLoading  atomic.Bool
	eventsAllShown atomic.Bool

	deviceModel *deviceTableModel
	eventModel  *eventTableModel

	objSearch      *walk.LineEdit
	objToolbar     *walk.Composite
	objTable       *walk.TableView
	eventSearch    *walk.LineEdit
	eventTable     *walk.TableView
	eventToolbar   *walk.Composite
	eventFilterBox *walk.ComboBox
	hideTestsBox   *walk.CheckBox
	footerBar      *walk.Composite
	headerBar      *walk.Composite
	headerTitle    *walk.Label
	headerSubtitle *walk.Label
	headerStatus   *walk.Label
	headerClients  *walk.Label
	headerEvents   *walk.Label
	statusLabel    *walk.Label
	transportLabel *walk.Label
	statsLabel     *walk.Label

	serverHost         *walk.LineEdit
	serverPort         *walk.LineEdit
	clientHost         *walk.LineEdit
	clientPort         *walk.LineEdit
	reconnectInit      *walk.LineEdit
	reconnectMax       *walk.LineEdit
	queueBuffer        *walk.LineEdit
	ppkTimeout         *walk.LineEdit
	logDir             *walk.LineEdit
	logFilename        *walk.LineEdit
	logMaxSize         *walk.LineEdit
	logMaxBackups      *walk.LineEdit
	logMaxAge          *walk.LineEdit
	logLevel           *walk.ComboBox
	logConsole         *walk.CheckBox
	logFile            *walk.CheckBox
	logPretty          *walk.CheckBox
	logSampling        *walk.CheckBox
	historyGlobal      *walk.LineEdit
	historyLog         *walk.LineEdit
	historyRetention   *walk.LineEdit
	historyCleanup     *walk.LineEdit
	historyArchivePath *walk.LineEdit
	historyBatch       *walk.LineEdit
	historyArchive     *walk.CheckBox
	uiStartMin         *walk.CheckBox
	uiMinTray          *walk.CheckBox
	uiCloseTray        *walk.CheckBox
	uiFontCombo        *walk.ComboBox
	uiFontValue        *walk.Label
	uptimeLabel        *walk.Label
	acceptedLabel      *walk.Label
	rejectedLabel      *walk.Label
	rateLabel          *walk.Label
	requiredPrefix     *walk.LineEdit
	validLength        *walk.LineEdit
	accountRanges      *walk.TextEdit

	bootReqID atomic.Uint64
	saveReqID atomic.Uint64

	categoryColors     map[string]walk.Color
	categoryFontColors map[string]walk.Color
	eventTypes         []core.EventTypeDTO
}

type historyDialog struct {
	app       *walkApp
	dlg       *walk.Dialog
	table     *walk.TableView
	search    *walk.LineEdit
	filter    *walk.ComboBox
	hide      *walk.CheckBox
	status    *walk.Label
	model     *eventTableModel
	device    core.DeviceDTO
	limit     int
	reqID     atomic.Uint64
	closed    atomic.Bool
	loading   atomic.Bool
	allShown  atomic.Bool
	eventType string
	hideTests bool
	query     string
}

func Run(ctx context.Context, rt core.Backend) error {
	defer appLog.RecoverPanic("walk-ui-run")
	uiCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	app := newWalkApp(uiCtx, cancel, rt)
	if err := app.start(); err != nil {
		return err
	}
	defer app.shutdown()

	if err := app.createMainWindow(); err != nil {
		return err
	}

	go app.flushLoop()
	go app.statsLoop()
	go app.eventsScrollLoop()

	app.refreshUI()
	app.applyStartVisibility()
	app.mw.Run()
	return nil
}

func newWalkApp(ctx context.Context, cancel context.CancelFunc, rt core.Backend) *walkApp {
	a := &walkApp{
		ctx:                ctx,
		cancel:             cancel,
		rt:                 rt,
		devices:            make(map[int]core.DeviceDTO),
		eventFilter:        "all",
		status:             "Запуск runtime...",
		deviceModel:        &deviceTableModel{},
		eventModel:         &eventTableModel{},
		pendingDevices:     make(chan core.DeviceDTO, maxPendingUiDevices),
		pendingEvents:      make(chan core.EventDTO, maxPendingUiEvents),
		pendingDeleted:     make(chan int, 100),
		categoryColors:     make(map[string]walk.Color),
		categoryFontColors: make(map[string]walk.Color),
	}
	a.deviceModel.app = a
	return a
}

func (a *walkApp) start() error {
	if err := a.rt.Start(a.ctx); err != nil {
		return err
	}
	a.cfg = a.rt.GetConfig()
	a.activityTO = core.ParseDuration(a.cfg.Monitoring.PpkTimeout, 15*time.Minute)
	a.historyLimit = a.initialGlobalLimit()
	a.eventsLimit = a.historyLimit

	boot, err := a.rt.Bootstrap(a.ctx, a.eventsLimit)
	if err != nil {
		return err
	}
	for _, d := range boot.Devices {
		a.devices[d.ID] = d
	}
	a.loadCategoryColors()
	a.events = append(a.events, boot.Events...)
	a.stats = a.rt.GetStats()
	a.status = "Працює"
	a.applyFiltersLocked()

	a.rt.SubscribeDevice(func(d core.DeviceDTO) {
		select {
		case a.pendingDevices <- d:
		default:
			a.dropDevices.Add(1)
		}
	})
	a.rt.SubscribeEvent(func(e core.EventDTO) {
		select {
		case a.pendingEvents <- e:
		default:
			a.dropEvents.Add(1)
		}
	})
	a.rt.SubscribeDeviceDeleted(func(id int) {
		select {
		case a.pendingDeleted <- id:
		default:
		}
	})
	return nil
}

func (a *walkApp) shutdown() {
	a.cancel()
	if a.notifyIcon != nil {
		a.notifyIcon.Dispose()
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()
	_ = a.rt.Stop(stopCtx)
}

func (a *walkApp) createMainWindow() error {
	var mw *walk.MainWindow

	err := MainWindow{
		AssignTo: &mw,
		Title:    "CID Windigo - Центр моніторингу",
		MinSize:  Size{Width: 800, Height: 540},
		Size:     Size{Width: 800, Height: 600},
		Font:     Font{Family: "Segoe UI", PointSize: 10},
		Background: SolidColorBrush{
			Color: colorWindow,
		},
		Layout: VBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 6}, Spacing: 8},
		Children: []Widget{
			Composite{
				AssignTo:   &a.headerBar,
				Background: SolidColorBrush{Color: colorHeroStart},
				Layout:     HBox{Margins: Margins{Left: 12, Top: 10, Right: 12, Bottom: 10}, Spacing: 12},
				Children: []Widget{
					Composite{
						Layout: VBox{MarginsZero: true, Spacing: 2},
						Children: []Widget{
							Label{AssignTo: &a.headerTitle, Text: "CID Windigo", TextColor: colorHeroTitle, Font: Font{Family: "Segoe UI Semibold", PointSize: 13}},
							Label{AssignTo: &a.headerSubtitle, Text: "Realtime relay control", TextColor: colorHeroSubtitle},
						},
					},
					HSpacer{},
					Label{
						AssignTo:   &a.headerStatus,
						Text:       "OFFLINE",
						TextColor:  colorHeroChipText,
						Background: SolidColorBrush{Color: colorHeroChipOffline},
					},
					Label{
						AssignTo:   &a.headerClients,
						Text:       "Clients: 0",
						TextColor:  colorHeroChipText,
						Background: SolidColorBrush{Color: colorHeroChipMetric},
					},
					Label{
						AssignTo:   &a.headerEvents,
						Text:       "Events: 0",
						TextColor:  colorHeroChipText,
						Background: SolidColorBrush{Color: colorHeroChipMetric},
					},
				},
			},
			TabWidget{
				Background:    SolidColorBrush{Color: colorWindow},
				StretchFactor: 1,
				Pages: []TabPage{
					a.objectsPage(),
					a.eventsPage(),
					a.settingsPage(),
				},
			},
			Composite{
				AssignTo: &a.footerBar,
				Background: SolidColorBrush{
					Color: colorSurface,
				},
				Layout: HBox{Margins: Margins{Left: 8, Top: 4, Right: 8, Bottom: 4}, Spacing: 10},
				Children: []Widget{
					Label{AssignTo: &a.statusLabel, Text: "Стан: -", TextColor: colorSoft},
					VSeparator{},
					Label{AssignTo: &a.transportLabel, Text: "Мережа: -", TextColor: colorSoft},
					Label{AssignTo: &a.uptimeLabel, Text: "Up: -", TextColor: colorSoft},
					VSeparator{},
					Label{AssignTo: &a.acceptedLabel, Text: "Ack: 0", TextColor: colorGoodText},
					Label{AssignTo: &a.rejectedLabel, Text: "Nack: 0", TextColor: colorBadText},
					VSeparator{},
					Label{AssignTo: &a.rateLabel, Text: "0 msg/min", TextColor: colorAccentText},
				},
			},
		},
	}.Create()
	if err != nil {
		return err
	}

	a.mw = mw
	if a.objTable != nil {
		a.deviceModel.SetTableView(a.objTable)
		a.objTable.SetGridlines(true)
		applyTableGridlineColor(a.objTable, win.RGB(214, 224, 236))
		a.updateObjectTableColumns()
	}
	if a.eventTable != nil {
		a.eventModel.SetTableView(a.eventTable)
		a.eventTable.SetGridlines(true)
		applyTableGridlineColor(a.eventTable, win.RGB(214, 224, 236))
		a.updateEventTableColumns()
	}
	a.deviceModel.SetRows(a.filteredDevices)
	a.eventModel.SetRows(a.filteredEvents)
	if a.eventFilterBox != nil {
		a.eventFilterBox.SetCurrentIndex(0)
	}
	if a.hideTestsBox != nil {
		a.hideTestsBox.SetChecked(false)
	}
	a.loadConfigEditors()
	a.applyUIFont(a.cfg.UI.FontSize)
	a.updateStatusBar()
	return a.configureWindow()
}

func (a *walkApp) configureWindow() error {
	if err := a.applyWindows11WindowChrome(); err != nil {
		log.Warn().Err(err).Msg("windows 11 chrome not available")
	}
	if err := a.setupNotifyIcon(); err != nil {
		log.Warn().Err(err).Msg("notify icon setup failed")
	}
	a.mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if a.cfg.UI.CloseToTray && reason == walk.CloseReasonUser {
			*canceled = true
			a.mw.Hide()
		}
	})
	a.mw.SizeChanged().Attach(func() {
		if a.cfg.UI.MinimizeToTray && win.IsIconic(a.mw.Handle()) && a.mw.Visible() {
			a.mw.Hide()
			a.showTrayMinimizedNotification()
		}
	})
	return nil
}

func (a *walkApp) showTrayMinimizedNotification() {
	if a.notifyIcon == nil {
		return
	}
	_ = a.notifyIcon.ShowInfo(
		"CID Ретранслятор",
		"Програму згорнуто в трей. Натисніть іконку в треї, щоб відкрити вікно.",
	)
}

func (a *walkApp) applyWindows11WindowChrome() error {
	if a.mw == nil {
		return nil
	}
	preference := uint32(dwmwcpRound)
	return setDwmWindowAttribute(
		uintptr(a.mw.Handle()),
		dwmwaWindowCornerPreference,
		unsafe.Pointer(&preference),
		unsafe.Sizeof(preference),
	)
}

func (a *walkApp) loadCategoryColors() {
	types, err := a.rt.GetEventTypes(a.ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to load event types for colors")
		return
	}
	a.eventTypes = types
	for _, et := range types {
		if et.Color != "" {
			a.categoryColors[strings.ToLower(et.Key)] = hexToColor(et.Color)
		}
		if et.FontColor != "" {
			a.categoryFontColors[strings.ToLower(et.Key)] = hexToColor(et.FontColor)
		}
	}
}

func hexToColor(hex string) walk.Color {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return walk.Color(0)
	}
	var r, g, b uint8
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return walk.RGB(r, g, b)
}

func setDwmWindowAttribute(hwnd uintptr, attribute uint32, value unsafe.Pointer, valueSize uintptr) error {
	if hwnd == 0 {
		return nil
	}
	if err := dwmSetWindowAttributeFn.Find(); err != nil {
		return err
	}
	hr, _, callErr := dwmSetWindowAttributeFn.Call(
		hwnd,
		uintptr(attribute),
		uintptr(value),
		valueSize,
	)
	if hr == 0 {
		return nil
	}
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return fmt.Errorf("DwmSetWindowAttribute failed: hresult=0x%X", uint32(hr))
}

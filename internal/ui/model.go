package ui

import (
	"context"
	"image/color"
	"sync"
	"sync/atomic"
	"time"

	"cid_gio_gio/internal/config"
	"cid_gio_gio/internal/core"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const (
	maxPendingUiDevices  = 5000
	maxPendingUiEvents   = 20000
	uiDrainBatchSize     = 5000
	uiRefreshTick        = 1 * time.Second
	liveEventsPerSecond  = 100
	eventsLoadChunk      = 500
	historyLoadChunk     = 250
	maxAutoLoadEvents    = 50000
	maxAutoLoadHistory   = 20000
	scrollPageSize       = 100
	scrollBufferSize     = 100
	objRowHeightDefault  = 30
	evtRowHeightDefault  = 26
	histRowHeightDefault = 26
)

var eventFilters = []string{"all", "alarm", "test", "fault", "guard", "disguard", "other"}
var logLevels = []string{"trace", "debug", "info", "warn", "error", "fatal"}
var eventColumns = []string{"Time", "PPK", "Code", "Type", "Description", "Zone", "Category"}

var (
	cBg         = color.NRGBA{R: 240, G: 242, B: 245, A: 255} // Light grey-blue background
	cPanel      = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	cPanel2     = color.NRGBA{R: 248, G: 250, B: 252, A: 255}
	cPanel3     = color.NRGBA{R: 241, G: 245, B: 249, A: 255}
	cBorder     = color.NRGBA{R: 226, G: 232, B: 240, A: 255} // Sleeker border
	cOverlay    = color.NRGBA{R: 15, G: 23, B: 42, A: 160}  // Darker overlay
	cModal      = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	cModalH     = color.NRGBA{R: 248, G: 250, B: 252, A: 255}
	cModalB     = color.NRGBA{R: 203, G: 213, B: 225, A: 255}
	cText       = color.NRGBA{R: 15, G: 23, B: 42, A: 255}   // Slate-900 for text
	cSoft       = color.NRGBA{R: 100, G: 116, B: 139, A: 255} // Slate-500 for secondary text
	cAccent     = color.NRGBA{R: 37, G: 99, B: 235, A: 255}  // Vibrant Blue
	cAccent2    = color.NRGBA{R: 219, G: 234, B: 254, A: 255} // Light Blue
	cAccentSoft = color.NRGBA{R: 239, G: 246, B: 255, A: 255} // Extra Light Blue
	cGood       = color.NRGBA{R: 22, G: 163, B: 74, A: 255}  // Green-600
	cGoodSoft   = color.NRGBA{R: 220, G: 252, B: 231, A: 255} // Green-100
	cWarn       = color.NRGBA{R: 217, G: 119, B: 6, A: 255}  // Amber-600
	cWarnSoft   = color.NRGBA{R: 254, G: 243, B: 199, A: 255} // Amber-100
	cBad        = color.NRGBA{R: 220, G: 38, B: 38, A: 255}  // Red-600
	cBadSoft    = color.NRGBA{R: 254, G: 226, B: 226, A: 255} // Red-100
)

type model struct {
	ctx    context.Context
	cancel context.CancelFunc
	rt     core.Backend

	app   fyne.App
	win   fyne.Window
	theme *modernTheme

	mu      sync.RWMutex
	devices map[int]core.DeviceDTO
	events  []core.EventDTO

	filteredDevices []core.DeviceDTO
	filteredEvents  []core.EventDTO
	activityTO      time.Duration
	historyLimit    int
	eventsLimit     int

	cfg config.AppConfig

	deviceFilter string
	eventFilter  string
	eventQuery   string
	hideTests    bool
	hideBlocked  bool

	activeDevices   int
	inactiveDevices int
	visibleEvents   int
	stats           core.StatsDTO
	statusMsg       string
	statusErr       string

	dropDevices atomic.Int64
	dropEvents  atomic.Int64

	liveWindowStart  time.Time
	liveWindowCount  int
	lastUiRefresh    time.Time
	lastEventsReload time.Time
	eventsDirty      bool

	pendingDevices chan core.DeviceDTO
	pendingEvents  chan core.EventDTO

	deviceCategoryCache map[int]string // New cache for optimized O(1) category lookup

	bootCh   chan bootResult
	eventsCh chan eventsResult
	statsCh  chan core.StatsDTO
	saveCh   chan saveResult
	deleteCh chan deleteResult
	hResult  chan historyResult
	rfResult chan rfResult
	deletedDevices chan int

	bootReqID       atomic.Uint64
	eventsReqID     atomic.Uint64
	historyReqID    atomic.Uint64
	bootCancel      context.CancelFunc
	eventsCancel    context.CancelFunc
	historyCancel   context.CancelFunc
	historyDebounce *time.Timer

	eventsBusy  atomic.Bool
	historyBusy atomic.Bool
	delBusy     atomic.Bool
	rfBusy      atomic.Bool

	trayStarted  atomic.Bool
	trayReady    atomic.Bool
	allowClose   atomic.Bool
	trayNoticeAt atomic.Int64

	// Main UI refs
	headerTitle    *canvas.Text
	headerSubtitle *canvas.Text
	statusBanner   *statusBannerRef
	chipStatus     *chipRef
	chipUptime     *chipRef
	chipClients    *chipRef
	chipAccepted   *chipRef
	chipRejected   *chipRef
	chipRate       *chipRef

	objSearchEntry   *widget.Entry
	evtSearchEntry   *widget.Entry
	hideTestsCheck   *widget.Check
	hideBlockedCheck *widget.Check
	eventFilterBtns  map[string]*filterButtonRef

	objTable  *widget.Table
	evtList   *widget.List
	objScroll *container.Scroll
	evtScroll *container.Scroll
	selObjRow int
	selEvtRow int
	objStart  int
	objCount  int
	evtStart  int
	evtCount  int

	objMetricTotal    *metricCardRef
	objMetricVisible  *metricCardRef
	objMetricActive   *metricCardRef
	objMetricInactive *metricCardRef

	evtMetricVisible *metricCardRef
	evtMetricLoaded  *metricCardRef
	evtMetricFilter  *metricCardRef
	evtMetricRate    *metricCardRef

	objShowingChip *chipRef

	// Settings UI
	cfgEntries     map[string]*widget.Entry
	cfgChecks      map[string]*widget.Check
	logLevelSelect *widget.Select
	fontSizeSlider *widget.Slider
	fontSizeLabel  *canvas.Text

	// History window
	hOpen             bool
	hDevice           core.DeviceDTO
	hRows             []core.EventDTO
	hLimit            int
	hSearchEntry      *widget.Entry
	hHideTestsCheck   *widget.Check
	hHideBlockedCheck *widget.Check
	hEventType        string
	hQueryCache       string
	hFilterBtns       map[string]*filterButtonRef
	hList             *widget.List
	hScroll           *container.Scroll
	selHistRow        int
	hStart            int
	hCount            int
	hWin              fyne.Window
	hHeaderTitle      *canvas.Text
	hHeaderSubtitle   *canvas.Text

	// Relay filter window
	rfOpen        bool
	rfTab         int
	rfRule        core.RelayFilterRule
	rfEnabled     *widget.Check
	rfGroups      *widget.Entry
	rfObjQuery    *widget.Entry
	rfCodeQuery   *widget.Entry
	rfSaveBtn     *widget.Button
	rfCancelBtn   *widget.Button
	rfOpenBtn     *widget.Button
	rfStatusLabel *widget.Label

	rfObjects      []rfObjectRow
	rfCodes        []rfCodeRow
	rfSummary      []rfSummaryRow
	rfFilteredObjs []*rfObjectRow
	rfFilteredCd   []*rfCodeRow
	rfObjList      *widget.Table
	rfCodeList     *widget.Table
	rfSumList      *widget.List
	rfTabs         *container.AppTabs

	rfSelectAllObjs  *widget.Button
	rfClearObjs      *widget.Button
	rfSelectAllCodes *widget.Button
	rfClearCodes     *widget.Button

	rfSelectedObjs       map[int]bool
	rfSelectedCodes      map[string]bool
	rfCategoryFilter     string
	rfCategoryFilterBtns map[string]*filterButtonRef

	rfDetailObj   int
	rfDetailCode  string
	rfDetailZones *widget.Entry
	rfDetailParts *widget.Entry
	rfWin         fyne.Window
}

type metricCardRef struct {
	bg        *canvas.Rectangle
	label     *canvas.Text
	value     *canvas.Text
	container fyne.CanvasObject
}

type chipRef struct {
	bg        *canvas.Rectangle
	label     *canvas.Text
	value     *canvas.Text
	container fyne.CanvasObject
}

type statusBannerRef struct {
	bg   *canvas.Rectangle
	text *canvas.Text
	box  fyne.CanvasObject
}

type bootResult struct {
	reqID uint64
	boot  core.BootstrapDTO
	err   error
}

type eventsResult struct {
	reqID  uint64
	events []core.EventDTO
	limit  int
	err    error
}

type historyRequest struct {
	id          int
	limit       int
	eventType   string
	hideTests   bool
	hideBlocked bool
	query       string
}

type historyResult struct {
	reqID  uint64
	id     int
	events []core.EventDTO
	limit  int
	err    error
}

type saveResult struct {
	cfg config.AppConfig
	err error
}

type deleteResult struct {
	id  int
	err error
}

type rfResult struct {
	rule core.RelayFilterRule
	err  error
}

type rfObjectRow struct {
	ID       int
	Display  string
	Selected bool
}

type rfCodeRow struct {
	Code        string
	Type        string
	Description string
	Category    string
	Selected    bool
}

type rfSummaryRow struct {
	ID            int
	Display       string
	Global        bool
	SpecificCodes string
}

package ui

import (
	"context"
	"image/color"
	"sync"
	"sync/atomic"
	"time"

	"cid_gio_gio/internal/config"
	"cid_gio_gio/internal/core"
	appRuntime "cid_gio_gio/internal/runtime"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const (
	maxPendingUiDevices = 20000
	maxPendingUiEvents  = 120000
	uiDrainBatchSize    = 5000
	uiRefreshTick       = 1 * time.Second
	liveEventsPerSecond = 100
	eventsLoadChunk     = 500
	historyLoadChunk    = 250
	maxAutoLoadEvents   = 50000
	maxAutoLoadHistory  = 20000
	objectsActionColDp  = 80
)

var eventFilters = []string{"all", "alarm", "test", "fault", "guard", "disguard", "other"}
var logLevels = []string{"trace", "debug", "info", "warn", "error", "fatal"}

var (
	cBg         = color.NRGBA{R: 243, G: 246, B: 251, A: 255}
	cPanel      = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	cPanel2     = color.NRGBA{R: 247, G: 249, B: 252, A: 255}
	cPanel3     = color.NRGBA{R: 237, G: 242, B: 249, A: 255}
	cBorder     = color.NRGBA{R: 214, G: 221, B: 231, A: 255}
	cOverlay    = color.NRGBA{R: 17, G: 24, B: 39, A: 130}
	cModal      = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	cModalH     = color.NRGBA{R: 244, G: 247, B: 252, A: 255}
	cModalB     = color.NRGBA{R: 189, G: 198, B: 212, A: 255}
	cText       = color.NRGBA{R: 31, G: 41, B: 55, A: 255}
	cSoft       = color.NRGBA{R: 82, G: 96, B: 114, A: 255}
	cAccent     = color.NRGBA{R: 0, G: 120, B: 212, A: 255}
	cAccent2    = color.NRGBA{R: 228, G: 237, B: 249, A: 255}
	cAccentSoft = color.NRGBA{R: 232, G: 242, B: 252, A: 255}
	cGood       = color.NRGBA{R: 17, G: 124, B: 65, A: 255}
	cGoodSoft   = color.NRGBA{R: 230, G: 246, B: 237, A: 255}
	cWarn       = color.NRGBA{R: 168, G: 95, B: 0, A: 255}
	cWarnSoft   = color.NRGBA{R: 252, G: 243, B: 221, A: 255}
	cBad        = color.NRGBA{R: 196, G: 43, B: 28, A: 255}
	cBadSoft    = color.NRGBA{R: 251, G: 231, B: 229, A: 255}
)

type model struct {
	ctx    context.Context
	cancel context.CancelFunc
	rt     *appRuntime.Runtime
	w      *app.Window
	th     *material.Theme
	ops    op.Ops

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
	statsExpanded   bool

	dropDevices atomic.Int64
	dropEvents  atomic.Int64

	liveWindowStart time.Time
	liveWindowCount int
	lastUiRefresh   time.Time

	pendingDevices chan core.DeviceDTO
	pendingEvents  chan core.EventDTO

	tab          int
	tabBtn       [3]widget.Clickable
	objSearch    widget.Editor
	evtSearch    widget.Editor
	hideTestsBox widget.Bool
	hideBlockedBox widget.Bool
	fontSize     widget.Float
	settingsList widget.List
	objList      widget.List
	evtList      widget.List
	objRows      []widget.Clickable
	objRCTags    []bool

	statsToggle widget.Clickable
	ctxMenuOpen   bool
	ctxMenuDevice core.DeviceDTO
	ctxMenuDel    widget.Clickable
	ctxMenuHist   widget.Clickable
	ctxMenuClose  widget.Clickable

	saveCfg  widget.Clickable
	resetCfg widget.Clickable

	filterBtns     map[string]*widget.Clickable
	logLevelBtn    widget.Clickable
	logLevelBtnMap map[string]*widget.Clickable
	logLevelOpen   bool

	cfgFields map[string]*widget.Editor
	cfgFlags  map[string]*widget.Bool

	hOpen       bool
	hDevice     core.DeviceDTO
	hRows       []core.EventDTO
	hLimit      int
	hList       widget.List
	hSearch     widget.Editor
	hHideTests  bool
	hHideBlocked bool
	hHideBox    widget.Bool
	hHideBlockedBox widget.Bool
	hEventType  string
	hReload     widget.Clickable
	hClose      widget.Clickable
	hQueryCache string
	hFilterBtns map[string]*widget.Clickable
	hResult     chan historyResult
	eventsBusy  atomic.Bool
	historyBusy atomic.Bool
	delOpen     bool
	delBusy     atomic.Bool
	delDevice   core.DeviceDTO
	delBackdrop widget.Clickable
	delConfirm  widget.Clickable
	delCancel   widget.Clickable
	delPanelTag bool
	platform    *platformState

	bootCh   chan bootResult
	eventsC  chan eventsResult
	statsCh  chan core.StatsDTO
	saveCh   chan saveResult
	deleteCh chan deleteResult

	bootReqID       atomic.Uint64
	eventsReqID     atomic.Uint64
	historyReqID    atomic.Uint64
	bootCancel      context.CancelFunc
	eventsCancel    context.CancelFunc
	historyCancel   context.CancelFunc
	historyDebounce *time.Timer

	rfOpen      bool
	rfTab       int
	rfRule      core.RelayFilterRule
	rfEnabled   widget.Bool
	rfGroups    widget.Editor
	rfObjQuery  widget.Editor
	rfCodeQuery widget.Editor
	rfSave      widget.Clickable
	rfCancel    widget.Clickable
	rfOpenBtn   widget.Clickable
	rfBusy      atomic.Bool
	rfResult    chan rfResult

	rfObjects      []rfObjectRow
	rfCodes        []rfCodeRow
	rfSummary      []rfSummaryRow
	rfFilteredObjs []*rfObjectRow
	rfFilteredCd   []*rfCodeRow
	rfObjList      widget.List
	rfCodeList     widget.List
	rfSumList      widget.List

	rfTabs           [2]widget.Clickable
	rfSelectAllObjs  widget.Clickable
	rfClearObjs      widget.Clickable
	rfSelectAllCodes widget.Clickable
	rfClearCodes     widget.Clickable

	rfSelectedObjs  map[int]bool
	rfSelectedCodes map[string]bool

	rfDetailOpen  bool
	rfDetailCode  string
	rfDetailObj   int // 0 for global
	rfDetailZones widget.Editor
	rfDetailParts widget.Editor
	rfDetailSave  widget.Clickable
	rfDetailClose widget.Clickable
}

type rfResult struct {
	rule core.RelayFilterRule
	err  error
}

type rfObjectRow struct {
	ID       int
	Display  string
	Selected widget.Bool
}

type rfCodeRow struct {
	Code        string
	Type        string
	Description string
	Category    string
	Selected    widget.Bool
	Config      widget.Clickable
}

type rfSummaryRow struct {
	ID            int
	Display       string
	Global        bool
	SpecificCodes string
}

type historyRequest struct {
	id        int
	limit     int
	eventType string
	hideTests bool
	hideBlocked bool
	query     string
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

type saveResult struct {
	cfg config.AppConfig
	err error
}

type historyResult struct {
	reqID  uint64
	id     int
	events []core.EventDTO
	limit  int
	err    error
}

type deleteResult struct {
	id  int
	err error
}

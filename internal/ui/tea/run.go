package tea

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"cid_fyne/internal/config"
	"cid_fyne/internal/core"
	appLog "cid_fyne/internal/logger"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	maxPendingUiDevices = 20000
	maxPendingUiEvents  = 120000
	uiDrainBatchSize    = 5000
	eventsLoadChunk     = 500
	historyLoadChunk    = 250
)

const (
	tabObjects = iota
	tabEvents
	tabSettings
	tabRelay
	tabColors
)

const (
	modeNormal = iota
	modeSearch
	modeEdit
	modeHistory
)

type tickMsg time.Time

type bootMsg struct {
	boot   core.BootstrapDTO
	events []core.EventDTO
	err    error
}

type moreEventsMsg struct {
	events []core.EventDTO
	limit  int
	err    error
}

type historyMsg struct {
	events []core.EventDTO
	limit  int
	err    error
}

type configSaveMsg struct {
	cfg config.AppConfig
	err error
}

type relayMsg struct {
	rule core.RelayFilterRule
	err  error
}

type relaySaveMsg struct {
	err error
}

type colorSaveMsg struct {
	key string
	err error
}

type deleteMsg struct {
	id  int
	err error
}

type settingField struct {
	Section string
	Key   string
	Label string
	Value string
	Bool  bool
}

type colorField struct {
	Key       string
	Title     string
	Color     string
	FontColor string
}

type model struct {
	ctx    context.Context
	cancel context.CancelFunc
	rt     core.Backend

	width  int
	height int

	cfg        config.AppConfig
	stats      core.StatsDTO
	status     string
	statusErr  string
	activityTO time.Duration

	tab       int
	mode      int
	textInput textinput.Model
	editKey   string

	devices         map[int]core.DeviceDTO
	events          []core.EventDTO
	filteredDevices []core.DeviceDTO
	filteredEvents  []core.EventDTO

	pendingDevices chan core.DeviceDTO
	pendingEvents  chan core.EventDTO
	pendingDeleted chan int

	deviceFilter string
	eventFilter  string
	eventQuery   string
	hideTests    bool
	eventsLimit  int
	historyLimit int

	activeDevices   int
	inactiveDevices int
	visibleEvents   int

	selObj int
	selEvt int

	historyDevice   core.DeviceDTO
	historyRows     []core.EventDTO
	historySel      int
	historyFilter   string
	historyHideTest bool
	historyQuery    string
	historyBusy     bool
	historyAllShown bool

	settings   []settingField
	selSetting int
	settingsQuery string

	relayFields []settingField
	selRelay    int
	relayQuery  string

	colors   []colorField
	selColor int
	catBg    map[string]string
	catFg    map[string]string

	loadingEvents bool
	confirmDelete int
}

func Run(ctx context.Context, rt core.Backend) error {
	defer appLog.RecoverPanic("tea-ui-run")
	uiCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	m, err := newModel(uiCtx, cancel, rt)
	if err != nil {
		return err
	}
	defer m.shutdown()

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func newModel(ctx context.Context, cancel context.CancelFunc, rt core.Backend) (*model, error) {
	if err := rt.Start(ctx); err != nil {
		return nil, err
	}

	cfg := rt.GetConfig()
	activity := core.ParseDuration(cfg.Monitoring.PpkTimeout, 15*time.Minute)
	limit := maxInt(1, cfg.History.GlobalLimit)

	boot, err := rt.Bootstrap(ctx, limit)
	if err != nil {
		return nil, err
	}
	events, err := rt.FilterEvents(ctx, limit, "all", false, false, "")
	if err != nil {
		return nil, err
	}

	ti := textinput.New()
	ti.Prompt = "> "
	ti.CharLimit = 4096

	m := &model{
		ctx:            ctx,
		cancel:         cancel,
		rt:             rt,
		cfg:            cfg,
		status:         "Працює",
		activityTO:     activity,
		eventsLimit:    limit,
		historyLimit:   maxInt(historyLoadChunk, cfg.History.LogLimit),
		eventFilter:    "all",
		historyFilter:  "all",
		devices:        make(map[int]core.DeviceDTO, len(boot.Devices)),
		events:         append([]core.EventDTO(nil), events...),
		pendingDevices: make(chan core.DeviceDTO, maxPendingUiDevices),
		pendingEvents:  make(chan core.EventDTO, maxPendingUiEvents),
		pendingDeleted: make(chan int, 256),
		textInput:      ti,
		catBg:          map[string]string{},
		catFg:          map[string]string{},
	}

	for _, d := range boot.Devices {
		m.devices[d.ID] = d
	}

	m.settings = buildSettings(cfg)
	m.relayFields = buildRelayFields(core.RelayFilterRule{})
	m.loadColors()
	m.applyFilters()
	m.stats = rt.GetStats()

	rt.SubscribeDevice(func(d core.DeviceDTO) {
		select {
		case m.pendingDevices <- d:
		default:
		}
	})
	rt.SubscribeEvent(func(e core.EventDTO) {
		select {
		case m.pendingEvents <- e:
		default:
		}
	})
	rt.SubscribeDeviceDeleted(func(id int) {
		select {
		case m.pendingDeleted <- id:
		default:
		}
	})

	return m, nil
}

func (m *model) shutdown() {
	m.cancel()
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = m.rt.Stop(stopCtx)
}

func (m *model) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		m.flushPending()
		m.stats = m.rt.GetStats()
		return m, tickCmd()
	case bootMsg:
		if msg.err != nil {
			m.statusErr = msg.err.Error()
			return m, nil
		}
		m.devices = make(map[int]core.DeviceDTO, len(msg.boot.Devices))
		for _, d := range msg.boot.Devices {
			m.devices[d.ID] = d
		}
		m.events = append([]core.EventDTO(nil), msg.events...)
		m.applyFilters()
		m.status = "Дані синхронізовано"
		m.statusErr = ""
		return m, nil
	case moreEventsMsg:
		m.loadingEvents = false
		if msg.err != nil {
			m.statusErr = msg.err.Error()
			return m, nil
		}
		m.eventsLimit = msg.limit
		m.events = append([]core.EventDTO(nil), msg.events...)
		m.applyFilters()
		m.status = fmt.Sprintf("Завантажено %d подій", len(msg.events))
		m.statusErr = ""
		return m, nil
	case historyMsg:
		m.historyBusy = false
		if msg.err != nil {
			m.statusErr = msg.err.Error()
			return m, nil
		}
		m.historyRows = msg.events
		m.historyLimit = msg.limit
		m.historyAllShown = len(msg.events) < msg.limit
		if m.historySel >= len(m.historyRows) {
			m.historySel = maxInt(0, len(m.historyRows)-1)
		}
		m.status = fmt.Sprintf("Історія %03d: %d записів", m.historyDevice.ID, len(msg.events))
		m.statusErr = ""
		return m, nil
	case configSaveMsg:
		if msg.err != nil {
			m.statusErr = msg.err.Error()
			return m, nil
		}
		m.cfg = msg.cfg
		m.settings = buildSettings(msg.cfg)
		m.ensureSettingSelectionVisible()
		m.activityTO = core.ParseDuration(msg.cfg.Monitoring.PpkTimeout, 15*time.Minute)
		m.eventsLimit = maxInt(1, msg.cfg.History.GlobalLimit)
		m.historyLimit = maxInt(historyLoadChunk, msg.cfg.History.LogLimit)
		m.status = "Налаштування збережено"
		m.statusErr = ""
		return m, m.reloadCmd()
	case relayMsg:
		if msg.err != nil {
			m.statusErr = msg.err.Error()
			return m, nil
		}
		m.relayFields = buildRelayFields(msg.rule)
		m.ensureRelaySelectionVisible()
		m.status = "Relay Filter завантажено"
		m.statusErr = ""
		return m, nil
	case relaySaveMsg:
		if msg.err != nil {
			m.statusErr = msg.err.Error()
			return m, nil
		}
		m.status = "Relay Filter збережено"
		m.statusErr = ""
		return m, nil
	case colorSaveMsg:
		if msg.err != nil {
			m.statusErr = msg.err.Error()
			return m, nil
		}
		m.status = fmt.Sprintf("Кольори %q збережено", msg.key)
		m.statusErr = ""
		m.loadColors()
		return m, nil
	case deleteMsg:
		if msg.err != nil {
			m.statusErr = msg.err.Error()
			return m, nil
		}
		m.status = fmt.Sprintf("Об'єкт %03d видалено", msg.id)
		m.statusErr = ""
		return m, nil
	}

	if m.mode == modeEdit {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "enter":
				m.applyEditedValue(strings.TrimSpace(m.textInput.Value()))
				m.mode = modeNormal
				return m, nil
			case "esc":
				m.mode = modeNormal
				return m, nil
			}
		}
		return m, cmd
	}

	if m.mode == modeSearch {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "enter":
				cmd := m.applySearch(strings.TrimSpace(m.textInput.Value()))
				m.mode = modeNormal
				return m, cmd
			case "esc":
				m.mode = modeNormal
				return m, nil
			}
		}
		return m, cmd
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.mode == modeHistory {
		return m.handleHistoryKeys(key)
	}

	switch key.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "left", "shift+tab":
		m.tab = (m.tab + 4) % 5
		return m, nil
	case "right", "tab":
		m.tab = (m.tab + 1) % 5
		return m, nil
	case "1", "2", "3", "4", "5":
		if len(key.Runes) == 1 {
			m.tab = int(key.Runes[0] - '1')
		}
		return m, nil
	case "/":
		m.mode = modeSearch
		m.editKey = "search"
		m.textInput.SetValue("")
		m.textInput.Focus()
		switch m.tab {
		case tabObjects:
			m.editKey = "search_objects"
			m.textInput.Placeholder = "Пошук об'єктів"
		case tabEvents:
			m.editKey = "search_events"
			m.textInput.Placeholder = "Пошук подій"
		case tabSettings:
			m.editKey = "search_settings"
			m.textInput.SetValue(m.settingsQuery)
			m.textInput.Placeholder = "Пошук налаштувань"
		case tabRelay:
			m.editKey = "search_relay"
			m.textInput.SetValue(m.relayQuery)
			m.textInput.Placeholder = "Пошук relay-фільтрів"
		default:
			m.textInput.Placeholder = "Пошук"
		}
		return m, textinput.Blink
	case "r":
		return m, m.reloadCmd()
	}

	switch m.tab {
	case tabObjects:
		return m.handleObjectKeys(key)
	case tabEvents:
		return m.handleEventKeys(key)
	case tabSettings:
		return m.handleSettingsKeys(key)
	case tabRelay:
		return m.handleRelayKeys(key)
	case tabColors:
		return m.handleColorKeys(key)
	}

	return m, nil
}

func (m *model) handleObjectKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up", "k":
		m.selObj = maxInt(0, m.selObj-1)
	case "down", "j":
		m.selObj = minInt(len(m.filteredDevices)-1, m.selObj+1)
	case "h", "enter":
		d, ok := m.selectedDevice()
		if !ok {
			return m, nil
		}
		m.historyDevice = d
		m.mode = modeHistory
		m.historyRows = nil
		m.historySel = 0
		m.historyFilter = "all"
		m.historyHideTest = false
		m.historyQuery = ""
		m.historyAllShown = false
		m.historyLimit = maxInt(historyLoadChunk, m.cfg.History.LogLimit)
		return m, m.loadHistoryCmd(m.historyLimit)
	case "d":
		d, ok := m.selectedDevice()
		if !ok {
			return m, nil
		}
		if m.confirmDelete != d.ID {
			m.confirmDelete = d.ID
			m.statusErr = ""
			m.status = fmt.Sprintf("Натисніть D ще раз для видалення об'єкта %03d", d.ID)
			return m, nil
		}
		m.confirmDelete = 0
		return m, m.deleteDeviceCmd(d.ID)
	case "D":
		d, ok := m.selectedDevice()
		if !ok {
			return m, nil
		}
		m.confirmDelete = 0
		return m, m.deleteDeviceCmd(d.ID)
	}
	return m, nil
}

func (m *model) handleEventKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up", "k":
		m.selEvt = maxInt(0, m.selEvt-1)
	case "down", "j":
		m.selEvt = minInt(len(m.filteredEvents)-1, m.selEvt+1)
	case "f":
		idx := 0
		for i, it := range eventFilters {
			if it == m.eventFilter {
				idx = i
				break
			}
		}
		idx = (idx + 1) % len(eventFilters)
		m.eventFilter = eventFilters[idx]
		m.applyFilters()
	case "t":
		m.hideTests = !m.hideTests
		m.applyFilters()
	case "l":
		if m.loadingEvents {
			return m, nil
		}
		m.loadingEvents = true
		return m, m.loadMoreEventsCmd(m.eventsLimit + eventsLoadChunk)
	}
	return m, nil
}

func (m *model) handleSettingsKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up", "k":
		m.moveSettingSelection(-1)
	case "down", "j":
		m.moveSettingSelection(1)
	case "enter", "e":
		if len(m.settings) == 0 || !m.isSettingVisible(m.selSetting) {
			return m, nil
		}
		m.mode = modeEdit
		m.editKey = "settings"
		m.textInput.SetValue(m.settings[m.selSetting].Value)
		m.textInput.Placeholder = m.settings[m.selSetting].Label
		m.textInput.Focus()
		return m, textinput.Blink
	case " ":
		if len(m.settings) == 0 || !m.isSettingVisible(m.selSetting) || !m.settings[m.selSetting].Bool {
			return m, nil
		}
		m.settings[m.selSetting].Value = boolText(!parseBool(m.settings[m.selSetting].Value))
	case "s":
		cfg, err := m.collectConfig()
		if err != nil {
			m.statusErr = err.Error()
			return m, nil
		}
		return m, m.saveConfigCmd(cfg)
	case "R":
		m.settings = buildSettings(m.cfg)
		m.ensureSettingSelectionVisible()
	}
	return m, nil
}

func (m *model) handleRelayKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "g":
		return m, m.loadRelayCmd()
	case "1":
		for i := range m.relayFields {
			if m.relayFields[i].Key == "Enabled" {
				m.relayFields[i].Value = boolText(!parseBool(m.relayFields[i].Value))
				m.selRelay = i
				break
			}
		}
	case "up", "k":
		m.moveRelaySelection(-1)
	case "down", "j":
		m.moveRelaySelection(1)
	case "enter", "e":
		if len(m.relayFields) == 0 || !m.isRelayVisible(m.selRelay) {
			return m, nil
		}
		m.mode = modeEdit
		m.editKey = "relay"
		m.textInput.SetValue(m.relayFields[m.selRelay].Value)
		m.textInput.Placeholder = m.relayFields[m.selRelay].Label
		m.textInput.Focus()
		return m, textinput.Blink
	case " ":
		if len(m.relayFields) == 0 || !m.isRelayVisible(m.selRelay) || !m.relayFields[m.selRelay].Bool {
			return m, nil
		}
		m.relayFields[m.selRelay].Value = boolText(!parseBool(m.relayFields[m.selRelay].Value))
	case "x":
		if len(m.relayFields) == 0 || !m.isRelayVisible(m.selRelay) {
			return m, nil
		}
		switch m.relayFields[m.selRelay].Key {
		case "Enabled":
			m.relayFields[m.selRelay].Value = "no"
		default:
			m.relayFields[m.selRelay].Value = ""
		}
		m.statusErr = ""
		m.status = "Поле relay-фільтра очищено"
	case "p":
		if len(m.relayFields) == 0 || !m.isRelayVisible(m.selRelay) {
			return m, nil
		}
		switch m.relayFields[m.selRelay].Key {
		case "GroupNumbers", "ObjectIDs":
			m.relayFields[m.selRelay].Value = formatIntCSV(parseIntCSV(m.relayFields[m.selRelay].Value))
		case "Codes":
			m.relayFields[m.selRelay].Value = formatCodeCSV(parseCodesCSV(m.relayFields[m.selRelay].Value))
		case "ObjectCodes":
			m.relayFields[m.selRelay].Value = formatObjectCodesMap(parseObjectCodesMap(m.relayFields[m.selRelay].Value))
		}
		m.statusErr = ""
		m.status = "Поле relay-фільтра нормалізовано"
	case "s":
		rule := collectRelayFields(m.relayFields)
		return m, m.saveRelayCmd(rule)
	}
	return m, nil
}

func (m *model) handleColorKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up", "k":
		m.selColor = maxInt(0, m.selColor-1)
	case "down", "j":
		m.selColor = minInt(len(m.colors)-1, m.selColor+1)
	case "b":
		if len(m.colors) == 0 {
			return m, nil
		}
		m.mode = modeEdit
		m.editKey = "color_bg"
		m.textInput.SetValue(m.colors[m.selColor].Color)
		m.textInput.Placeholder = "#RRGGBB"
		m.textInput.Focus()
		return m, textinput.Blink
	case "n":
		if len(m.colors) == 0 {
			return m, nil
		}
		m.mode = modeEdit
		m.editKey = "color_fg"
		m.textInput.SetValue(m.colors[m.selColor].FontColor)
		m.textInput.Placeholder = "#RRGGBB"
		m.textInput.Focus()
		return m, textinput.Blink
	case "w":
		if len(m.colors) == 0 {
			return m, nil
		}
		c := m.colors[m.selColor]
		bg := parseHexColor(c.Color)
		fg := parseHexColor(c.FontColor)
		if bg == "" || fg == "" {
			m.statusErr = "Некоректний колір. Формат: #RRGGBB"
			return m, nil
		}
		return m, m.saveColorCmd(c.Key, bg, fg)
	case "g":
		m.loadColors()
		return m, nil
	}
	return m, nil
}

func (m *model) handleHistoryKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q":
		m.mode = modeNormal
		return m, nil
	case "up", "k":
		m.historySel = maxInt(0, m.historySel-1)
	case "down", "j":
		m.historySel = minInt(len(m.historyRows)-1, m.historySel+1)
	case "f":
		idx := 0
		for i, it := range eventFilters {
			if it == m.historyFilter {
				idx = i
				break
			}
		}
		idx = (idx + 1) % len(eventFilters)
		m.historyFilter = eventFilters[idx]
		return m, m.loadHistoryCmd(maxInt(historyLoadChunk, m.cfg.History.LogLimit))
	case "t":
		m.historyHideTest = !m.historyHideTest
		return m, m.loadHistoryCmd(maxInt(historyLoadChunk, m.cfg.History.LogLimit))
	case "l":
		if m.historyAllShown || m.historyBusy {
			return m, nil
		}
		return m, m.loadHistoryCmd(m.historyLimit + historyLoadChunk)
	case "/":
		m.mode = modeSearch
		m.editKey = "search_history"
		m.textInput.SetValue(m.historyQuery)
		m.textInput.Placeholder = "Пошук історії"
		m.textInput.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func (m *model) applyEditedValue(v string) {
	switch m.editKey {
	case "settings":
		if len(m.settings) > 0 {
			m.settings[m.selSetting].Value = v
		}
	case "relay":
		if len(m.relayFields) > 0 {
			m.relayFields[m.selRelay].Value = v
		}
	case "color_bg":
		if len(m.colors) > 0 {
			m.colors[m.selColor].Color = v
		}
	case "color_fg":
		if len(m.colors) > 0 {
			m.colors[m.selColor].FontColor = v
		}
	}
}

func (m *model) applySearch(v string) tea.Cmd {
	if m.editKey == "search_history" {
		m.historyQuery = v
		return m.loadHistoryCmd(maxInt(historyLoadChunk, m.cfg.History.LogLimit))
	}
	switch m.editKey {
	case "search_objects":
		m.deviceFilter = v
		m.applyFilters()
	case "search_events":
		m.eventQuery = v
		m.applyFilters()
	case "search_settings":
		m.settingsQuery = v
		m.ensureSettingSelectionVisible()
	case "search_relay":
		m.relayQuery = v
		m.ensureRelaySelectionVisible()
	}
	return nil
}

func (m *model) selectedDevice() (core.DeviceDTO, bool) {
	if m.selObj < 0 || m.selObj >= len(m.filteredDevices) {
		return core.DeviceDTO{}, false
	}
	return m.filteredDevices[m.selObj], true
}

func (m *model) flushPending() {
	upd := map[int]core.DeviceDTO{}
	deleteIDs := map[int]struct{}{}
	eventBatch := make([]core.EventDTO, 0, uiDrainBatchSize)

	for pass := 0; pass < 4; pass++ {
		for i := 0; i < uiDrainBatchSize; i++ {
			select {
			case id := <-m.pendingDeleted:
				deleteIDs[id] = struct{}{}
			default:
				i = uiDrainBatchSize
			}
		}
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
		if len(m.pendingDevices) == 0 && len(m.pendingEvents) == 0 && len(m.pendingDeleted) == 0 {
			break
		}
	}

	if len(upd) == 0 && len(deleteIDs) == 0 && len(eventBatch) == 0 {
		return
	}
	for id := range deleteIDs {
		delete(m.devices, id)
	}
	for _, d := range upd {
		m.devices[d.ID] = d
	}
	if len(eventBatch) > 0 {
		capN := maxInt(1000, m.eventsLimit)
		m.events = prependEvents(m.events, eventBatch, capN)
	}
	m.applyFilters()
}

func (m *model) applyFilters() {
	devs, active, inactive := filterDevices(m.devices, m.deviceFilter, func(d core.DeviceDTO) bool {
		return isStaleTime(d.LastEventTime, m.activityTO)
	})
	events := filterEvents(m.events, m.eventFilter, m.hideTests, m.eventQuery, m.eventsLimit)
	m.filteredDevices = devs
	m.filteredEvents = events
	m.activeDevices = active
	m.inactiveDevices = inactive
	m.visibleEvents = len(events)
	if m.selObj >= len(devs) {
		m.selObj = maxInt(0, len(devs)-1)
	}
	if m.selEvt >= len(events) {
		m.selEvt = maxInt(0, len(events)-1)
	}
}

func (m *model) View() string {
	if m.width == 0 {
		m.width = 120
	}

	header := m.renderHeader()
	body := m.renderBody()
	footer := m.renderFooter()

	if m.mode == modeEdit {
		return lipgloss.JoinVertical(lipgloss.Left, header, body, m.renderEditBar(), footer)
	}
	if m.mode == modeSearch {
		return lipgloss.JoinVertical(lipgloss.Left, header, body, m.renderSearchBar(), footer)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m *model) renderHeader() string {
	tabNames := []string{"1 Objects", "2 Events", "3 Settings", "4 Relay", "5 Colors"}
	tabIdle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94A3B8")).
		Background(lipgloss.Color("#111827")).
		Padding(0, 1)
	tabActive := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E2E8F0")).
		Background(lipgloss.Color("#0F4C81")).
		Padding(0, 1)

	tabs := make([]string, 0, len(tabNames))
	for i, name := range tabNames {
		if i == m.tab {
			tabs = append(tabs, tabActive.Render(name))
		} else {
			tabs = append(tabs, tabIdle.Render(name))
		}
	}

	statusText := firstNonEmpty(m.statusErr, m.status)
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D1FAE5")).
		Background(lipgloss.Color("#064E3B")).
		Bold(true).
		Padding(0, 1)
	if strings.TrimSpace(m.statusErr) != "" {
		statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FEE2E2")).
			Background(lipgloss.Color("#7F1D1D")).
			Bold(true).
			Padding(0, 1)
	}

	chip := func(label, value string) string {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CBD5E1")).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1).
			Render(fmt.Sprintf("%s %s", label, value))
	}

	modeName := "NORMAL"
	switch m.mode {
	case modeSearch:
		modeName = "SEARCH"
	case modeEdit:
		modeName = "EDIT"
	case modeHistory:
		modeName = "HISTORY"
	}
	modeChip := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FDE68A")).
		Background(lipgloss.Color("#78350F")).
		Bold(true).
		Padding(0, 1).
		Render(modeName)

	line1 := lipgloss.JoinHorizontal(lipgloss.Left, strings.Join(tabs, " "), " ", modeChip)
	line2 := strings.Join([]string{
		statusStyle.Render(statusText),
		chip("Clients", strconv.Itoa(m.stats.Clients)),
		chip("Ack", strconv.FormatInt(m.stats.Accepted, 10)),
		chip("Nack", strconv.FormatInt(m.stats.Rejected, 10)),
		chip("Rate", fmt.Sprintf("%d msg/min", m.stats.ReceivedPM)),
		chip("Up", firstNonEmpty(m.stats.Uptime, "-")),
	}, " ")
	return line1 + "\n" + line2
}

func (m *model) renderBody() string {
	switch {
	case m.mode == modeHistory:
		return m.renderHistory()
	case m.tab == tabObjects:
		return m.renderObjects()
	case m.tab == tabEvents:
		return m.renderEvents()
	case m.tab == tabSettings:
		return m.renderSettings()
	case m.tab == tabRelay:
		return m.renderRelay()
	case m.tab == tabColors:
		return m.renderColors()
	default:
		return ""
	}
}

func (m *model) renderFooter() string {
	help := "q quit | tab/shift+tab switch tabs | / search | r reload"
	switch {
	case m.mode == modeHistory:
		help = "History: esc close | f filter | t hide tests | l more | / search"
	case m.tab == tabObjects:
		help += " | h/enter history | d delete(confirm)"
	case m.tab == tabEvents:
		help += " | f category | t hide tests | l load more"
	case m.tab == tabSettings:
		help += " | / search | enter edit | space toggle bool | s save | Shift+R reset"
	case m.tab == tabRelay:
		help += " | / search | 1 enabled | enter edit | x clear | p normalize | g load | s save"
	case m.tab == tabColors:
		help += " | b edit bg | n edit font | w save | g reload"
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94A3B8")).
		Background(lipgloss.Color("#0B1220")).
		Padding(0, 1).
		Render(help)
}

func (m *model) renderSearchBar() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8FAFC")).
		Background(lipgloss.Color("#1E293B")).
		Padding(0, 1).
		Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Render("Search") + " " + m.textInput.View())
}

func (m *model) renderEditBar() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8FAFC")).
		Background(lipgloss.Color("#1E293B")).
		Padding(0, 1).
		Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Render("Edit") + " " + m.textInput.View())
}

func (m *model) renderObjects() string {
	var b strings.Builder
	meta := fmt.Sprintf("Objects: visible %d | active %d | inactive %d | filter: %q",
		len(m.filteredDevices), m.activeDevices, m.inactiveDevices, m.deviceFilter)
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")).Bold(true).Render(meta) + "\n")
	w := m.tableWidth()
	stW, idW, clientW, eventW, timeW := m.objectColumnWidths(w - 4)
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#C7D2FE"))
	b.WriteString("    " + headerStyle.Render(m.renderObjectHeader(stW, idW, clientW, eventW, timeW)) + "\n")
	limit := m.contentRows()
	start := maxInt(0, m.selObj-limit+1)
	if m.selObj < start {
		start = m.selObj
	}
	end := minInt(len(m.filteredDevices), start+limit)
	for i := start; i < end; i++ {
		d := m.filteredDevices[i]
		status := "ON"
		if isStaleTime(d.LastEventTime, m.activityTO) {
			status = "OFF"
		}
		prefix := "   "
		if i == m.selObj {
			prefix = " ->"
		}
		line := prefix + " " + m.renderObjectRow(d, status, stW, idW, clientW, eventW, timeW)
		if i == m.selObj {
			line = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#E2E8F0")).
				Background(lipgloss.Color("#1E3A5F")).
				Render(line)
		} else if i%2 == 1 {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("#C8D3E1")).Render(line)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m *model) renderEvents() string {
	var b strings.Builder
	meta := fmt.Sprintf("Events: visible %d / loaded %d | filter: %s | hide tests: %s | query: %q",
		len(m.filteredEvents), len(m.events), m.eventFilter, boolText(m.hideTests), m.eventQuery)
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")).Bold(true).Render(meta) + "\n")
	w := m.tableWidth()
	timeW, idW, codeW, typeW, descW, zoneW := m.eventColumnWidths(w - 4)
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#C7D2FE"))
	b.WriteString("    " + headerStyle.Render(m.renderEventHeader(timeW, idW, codeW, typeW, descW, zoneW)) + "\n")
	limit := m.contentRows()
	start := maxInt(0, m.selEvt-limit+1)
	if m.selEvt < start {
		start = m.selEvt
	}
	end := minInt(len(m.filteredEvents), start+limit)
	for i := start; i < end; i++ {
		e := m.filteredEvents[i]
		prefix := "   "
		if i == m.selEvt {
			prefix = " ->"
		}
		line := prefix + " " + m.renderEventRow(e, timeW, idW, codeW, typeW, descW, zoneW)
		if i == m.selEvt {
			line = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#E2E8F0")).
				Background(lipgloss.Color("#1E3A5F")).
				Render(line)
		} else {
			line = m.styleEventLine(e.Category).Render(line)
		}
		b.WriteString(line + "\n")
	}
	if m.loadingEvents {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FCD34D")).Render("Loading more events...") + "\n")
	}
	return b.String()
}

func (m *model) renderHistory() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")).Bold(true).Render(fmt.Sprintf("History of %03d | filter: %s | hide tests: %s | query: %q | loaded: %d",
		m.historyDevice.ID,
		m.historyFilter,
		boolText(m.historyHideTest),
		m.historyQuery,
		len(m.historyRows),
	)) + "\n")
	w := m.tableWidth()
	timeW, idW, codeW, typeW, descW, zoneW := m.eventColumnWidths(w - 4)
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#C7D2FE"))
	b.WriteString("    " + headerStyle.Render(m.renderEventHeader(timeW, idW, codeW, typeW, descW, zoneW)) + "\n")
	limit := m.contentRows()
	start := maxInt(0, m.historySel-limit+1)
	if m.historySel < start {
		start = m.historySel
	}
	end := minInt(len(m.historyRows), start+limit)
	for i := start; i < end; i++ {
		e := m.historyRows[i]
		prefix := "   "
		if i == m.historySel {
			prefix = " ->"
		}
		line := prefix + " " + m.renderEventRow(e, timeW, idW, codeW, typeW, descW, zoneW)
		if i == m.historySel {
			line = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#E2E8F0")).
				Background(lipgloss.Color("#1E3A5F")).
				Render(line)
		} else {
			line = m.styleEventLine(e.Category).Render(line)
		}
		b.WriteString(line + "\n")
	}
	if m.historyBusy {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FCD34D")).Render("Loading history...") + "\n")
	}
	return b.String()
}

func (m *model) renderSettings() string {
	var b strings.Builder
	visible := 0
	for i := range m.settings {
		if m.isSettingVisible(i) {
			visible++
		}
	}
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")).Bold(true).Render(
		fmt.Sprintf("Settings: visible %d / total %d | query: %q", visible, len(m.settings), m.settingsQuery),
	) + "\n")
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A5B4FC"))
	lastSection := ""
	rendered := 0
	for i, f := range m.settings {
		if !m.isSettingVisible(i) {
			continue
		}
		if f.Section != lastSection {
			lastSection = f.Section
			b.WriteString(sectionStyle.Render("  " + firstNonEmpty(f.Section, "General")) + "\n")
		}
		prefix := "   "
		if i == m.selSetting {
			prefix = " ->"
		}
		line := fmt.Sprintf("%s %-24s %s", prefix, trimText(f.Label, 24), f.Value)
		if i == m.selSetting {
			line = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0")).Background(lipgloss.Color("#1E3A5F")).Render(line)
		} else {
			line = keyStyle.Render(line)
		}
		b.WriteString(line + "\n")
		rendered++
	}
	if rendered == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render("Нічого не знайдено. Очистьте пошук (/).") + "\n")
	}
	return b.String()
}

func (m *model) renderRelay() string {
	var b strings.Builder
	visible := 0
	for i := range m.relayFields {
		if m.isRelayVisible(i) {
			visible++
		}
	}
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")).Bold(true).Render(
		fmt.Sprintf("Relay Filter: visible %d / total %d | query: %q", visible, len(m.relayFields), m.relayQuery),
	) + "\n")
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A5B4FC"))
	lastSection := ""
	rendered := 0
	for i, f := range m.relayFields {
		if !m.isRelayVisible(i) {
			continue
		}
		if f.Section != lastSection {
			lastSection = f.Section
			b.WriteString(sectionStyle.Render("  " + firstNonEmpty(f.Section, "General")) + "\n")
		}
		prefix := "   "
		if i == m.selRelay {
			prefix = " ->"
		}
		line := fmt.Sprintf("%s %-24s %s", prefix, trimText(f.Label, 24), f.Value)
		if i == m.selRelay {
			line = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0")).Background(lipgloss.Color("#1E3A5F")).Render(line)
		} else {
			line = keyStyle.Render(line)
		}
		b.WriteString(line + "\n")
		rendered++
	}
	if rendered == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render("Нічого не знайдено. Очистьте пошук (/).") + "\n")
	}
	b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render("Формат object-codes: 123:E101,E102 (по одному рядку на об'єкт)") + "\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render("Швидкі дії: 1 toggle enabled | x clear field | p normalize field") + "\n")
	return b.String()
}

func (m *model) renderColors() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")).Bold(true).Render("Event Colors (B edit bg, N edit text, W save):") + "\n")
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1"))
	for i, c := range m.colors {
		prefix := "   "
		if i == m.selColor {
			prefix = " ->"
		}
		line := fmt.Sprintf("%s %-16s bg=%-8s text=%-8s", prefix, trimText(firstNonEmpty(c.Title, c.Key), 16), c.Color, c.FontColor)
		if i == m.selColor {
			line = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0")).Background(lipgloss.Color("#1E3A5F")).Render(line)
		} else {
			line = keyStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m *model) contentRows() int {
	h := m.height - 8
	if m.mode == modeEdit || m.mode == modeSearch {
		h--
	}
	if h < 6 {
		h = 6
	}
	return h
}

func (m *model) tableWidth() int {
	w := m.width - 4
	if w < 80 {
		w = 80
	}
	return w
}

func (m *model) objectColumnWidths(content int) (stW, idW, clientW, eventW, timeW int) {
	stW, timeW = 4, 19
	idW = clampInt(m.objectIDWidth(), 3, 8)
	sep := 4 * 3
	minClient, minEvent := 12, 18
	fixed := stW + idW + timeW + sep
	remaining := content - fixed
	if remaining < minClient+minEvent {
		remaining = minClient + minEvent
	}
	clientW = int(float64(remaining) * 0.35)
	if clientW < minClient {
		clientW = minClient
	}
	eventW = remaining - clientW
	if eventW < minEvent {
		eventW = minEvent
		clientW = remaining - eventW
		if clientW < minClient {
			clientW = minClient
		}
	}
	return
}

func (m *model) objectIDWidth() int {
	maxW := 3
	for _, d := range m.filteredDevices {
		w := len(strconv.Itoa(d.ID))
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

func (m *model) eventColumnWidths(content int) (timeW, idW, codeW, typeW, descW, zoneW int) {
	timeW, idW, codeW, typeW, zoneW = 19, 4, 4, 10, 20
	sep := 5 * 2
	minDesc := 10
	fixed := timeW + idW + codeW + typeW + zoneW + sep
	descW = content - fixed
	if descW < minDesc {
		descW = minDesc
	}
	return
}

func (m *model) renderObjectHeader(stW, idW, clientW, eventW, timeW int) string {
	return strings.Join([]string{
		fitText("ST", stW),
		fitText("ID", idW),
		fitText("CLIENT", clientW),
		fitText("LAST EVENT", eventW),
		fitText("TIME", timeW),
	}, " | ")
}

func (m *model) renderObjectRow(d core.DeviceDTO, state string, stW, idW, clientW, eventW, timeW int) string {
	ts := "-"
	if !d.LastEventTime.IsZero() {
		ts = d.LastEventTime.Format("2006-01-02 15:04:05")
	}
	return strings.Join([]string{
		fitText(state, stW),
		fitText(strconv.Itoa(d.ID), idW),
		fitText(firstNonEmpty(d.ClientAddr, "-"), clientW),
		fitText(firstNonEmpty(d.LastEvent, "-"), eventW),
		fitText(ts, timeW),
	}, " | ")
}

func (m *model) renderEventHeader(timeW, idW, codeW, typeW, descW, zoneW int) string {
	return strings.Join([]string{
		fitText("TIME", timeW),
		fitText("ID", idW),
		fitText("CODE", codeW),
		fitText("TYPE", typeW),
		fitText("DESCRIPTION", descW),
		fitText("ZONE", zoneW),
	}, " | ")
}

func (m *model) renderEventRow(e core.EventDTO, timeW, idW, codeW, typeW, descW, zoneW int) string {
	t := "-"
	if !e.Time.IsZero() {
		t = e.Time.Format("2006-01-02 15:04:05")
	}
	return strings.Join([]string{
		fitText(strings.TrimSpace(t), timeW),
		fitText(strings.TrimSpace(e.DeviceID), idW),
		fitText(strings.TrimSpace(e.Code), codeW),
		fitText(strings.TrimSpace(e.Type), typeW),
		fitText(strings.TrimSpace(e.Desc), descW),
		fitText(strings.TrimSpace(e.Zone), zoneW),
	}, " | ")
}

func (m *model) styleEventLine(category string) lipgloss.Style {
	key := strings.ToLower(strings.TrimSpace(category))
	if fg, ok := m.catFg[key]; ok && fg != "" {
		if isTooDarkHex(fg) {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(m.defaultEventFG(key)))
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color(fg))
	}

	// TUI-friendly defaults: foreground accents only, no full-row backgrounds.
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.defaultEventFG(key)))
}

func (m *model) defaultEventFG(key string) string {
	switch key {
	case "alarm":
		return "#FB7185"
	case "test":
		return "#FBBF24"
	case "fault":
		return "#F59E0B"
	case "guard":
		return "#34D399"
	case "disguard":
		return "#60A5FA"
	default:
		// "other" (System/Unknown) should stay bright and neutral in terminal.
		return "#CBD5E1"
	}
}

func isTooDarkHex(hex string) bool {
	c := parseHexColor(hex)
	if c == "" || len(c) != 7 || c[0] != '#' {
		return false
	}
	r, errR := strconv.ParseInt(c[1:3], 16, 64)
	g, errG := strconv.ParseInt(c[3:5], 16, 64)
	b, errB := strconv.ParseInt(c[5:7], 16, 64)
	if errR != nil || errG != nil || errB != nil {
		return false
	}
	// Simple luminance approximation for dark terminal background.
	luma := 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
	return luma < 90
}

func fitText(s string, width int) string {
	if width <= 0 {
		return ""
	}
	rs := []rune(strings.TrimSpace(s))
	if len(rs) > width {
		// if width > 3 {
		// 	return string(rs[:width-3]) + "..."
		// }
		return string(rs[:width])
	}
	if len(rs) < width {
		return string(rs) + strings.Repeat(" ", width-len(rs))
	}
	return string(rs)
}

func trimText(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s + strings.Repeat(" ", n-len(s))
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "..."
}

func (m *model) reloadCmd() tea.Cmd {
	limit := m.eventsLimit
	return func() tea.Msg {
		boot, err := m.rt.Bootstrap(m.ctx, limit)
		if err != nil {
			return bootMsg{err: err}
		}
		events, err := m.rt.FilterEvents(m.ctx, limit, "all", false, false, "")
		return bootMsg{boot: boot, events: events, err: err}
	}
}

func (m *model) loadMoreEventsCmd(limit int) tea.Cmd {
	return func() tea.Msg {
		events, err := m.rt.FilterEvents(m.ctx, limit, "all", false, false, "")
		return moreEventsMsg{events: events, limit: limit, err: err}
	}
}

func (m *model) loadHistoryCmd(limit int) tea.Cmd {
	m.historyBusy = true
	id := m.historyDevice.ID
	f := m.historyFilter
	hide := m.historyHideTest
	q := m.historyQuery
	return func() tea.Msg {
		events, err := m.rt.FilterDeviceHistory(m.ctx, id, limit, time.Time{}, time.Time{}, f, hide, false, q)
		return historyMsg{events: events, limit: limit, err: err}
	}
}

func (m *model) saveConfigCmd(cfg config.AppConfig) tea.Cmd {
	return func() tea.Msg {
		err := m.rt.SaveConfig(m.ctx, cfg)
		if err != nil {
			return configSaveMsg{err: err}
		}
		return configSaveMsg{cfg: m.rt.GetConfig()}
	}
}

func (m *model) loadRelayCmd() tea.Cmd {
	return func() tea.Msg {
		rule, err := m.rt.GetRelayFilterRule(m.ctx)
		return relayMsg{rule: rule, err: err}
	}
}

func (m *model) saveRelayCmd(rule core.RelayFilterRule) tea.Cmd {
	return func() tea.Msg {
		err := m.rt.SaveRelayFilterRule(m.ctx, rule)
		return relaySaveMsg{err: err}
	}
}

func (m *model) saveColorCmd(key, bg, fg string) tea.Cmd {
	return func() tea.Msg {
		err := m.rt.SaveEventTypeColors(m.ctx, key, bg, fg)
		return colorSaveMsg{key: key, err: err}
	}
}

func (m *model) deleteDeviceCmd(id int) tea.Cmd {
	return func() tea.Msg {
		err := m.rt.DeleteDeviceWithHistory(m.ctx, id)
		return deleteMsg{id: id, err: err}
	}
}

func (m *model) loadColors() {
	types, err := m.rt.GetEventTypes(m.ctx)
	if err != nil {
		return
	}
	rows := make([]colorField, 0, len(types))
	bgMap := make(map[string]string, len(types))
	fgMap := make(map[string]string, len(types))
	for _, t := range types {
		bg := strings.TrimSpace(t.Color)
		if bg == "" {
			bg = "#FFFFFF"
		}
		fg := strings.TrimSpace(t.FontColor)
		if fg == "" {
			fg = "#000000"
		}
		rows = append(rows, colorField{Key: t.Key, Title: t.Title, Color: bg, FontColor: fg})
		key := strings.ToLower(strings.TrimSpace(t.Key))
		if key != "" {
			// Ignore DB/UI placeholders (#FFFFFF/#000000) so they don't force harsh styles in TUI.
			rawBg := strings.TrimSpace(t.Color)
			rawFg := strings.TrimSpace(t.FontColor)
			if parsed := parseHexColor(rawBg); parsed != "" {
				bgMap[key] = parsed
			}
			if parsed := parseHexColor(rawFg); parsed != "" {
				fgMap[key] = parsed
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Key < rows[j].Key
	})
	m.colors = rows
	m.catBg = bgMap
	m.catFg = fgMap
	if m.selColor >= len(rows) {
		m.selColor = maxInt(0, len(rows)-1)
	}
}

func buildSettings(cfg config.AppConfig) []settingField {
	return []settingField{
		{Section: "Network", Key: "Server.Host", Label: "Server host", Value: cfg.Server.Host},
		{Section: "Network", Key: "Server.Port", Label: "Server port", Value: cfg.Server.Port},
		{Section: "Network", Key: "Client.Host", Label: "Client host", Value: cfg.Client.Host},
		{Section: "Network", Key: "Client.Port", Label: "Client port", Value: cfg.Client.Port},
		{Section: "Network", Key: "Client.ReconnectInitial", Label: "Reconnect initial", Value: cfg.Client.ReconnectInitial},
		{Section: "Network", Key: "Client.ReconnectMax", Label: "Reconnect max", Value: cfg.Client.ReconnectMax},
		{Section: "Runtime", Key: "Queue.BufferSize", Label: "Queue buffer", Value: strconv.Itoa(cfg.Queue.BufferSize)},
		{Section: "Runtime", Key: "Monitoring.PpkTimeout", Label: "PPK timeout", Value: cfg.Monitoring.PpkTimeout},
		{Section: "Logging", Key: "Logging.LogDir", Label: "Log dir", Value: cfg.Logging.LogDir},
		{Section: "Logging", Key: "Logging.Filename", Label: "Log filename", Value: cfg.Logging.Filename},
		{Section: "Logging", Key: "Logging.MaxSize", Label: "Log max size", Value: strconv.Itoa(cfg.Logging.MaxSize)},
		{Section: "Logging", Key: "Logging.MaxBackups", Label: "Log max backups", Value: strconv.Itoa(cfg.Logging.MaxBackups)},
		{Section: "Logging", Key: "Logging.MaxAge", Label: "Log max age", Value: strconv.Itoa(cfg.Logging.MaxAge)},
		{Section: "Logging", Key: "Logging.Level", Label: "Log level", Value: cfg.Logging.Level},
		{Section: "Logging", Key: "Logging.EnableConsole", Label: "Log console", Value: boolText(cfg.Logging.EnableConsole), Bool: true},
		{Section: "Logging", Key: "Logging.EnableFile", Label: "Log file", Value: boolText(cfg.Logging.EnableFile), Bool: true},
		{Section: "Logging", Key: "Logging.PrettyConsole", Label: "Log pretty", Value: boolText(cfg.Logging.PrettyConsole), Bool: true},
		{Section: "Logging", Key: "Logging.SamplingEnabled", Label: "Log sampling", Value: boolText(cfg.Logging.SamplingEnabled), Bool: true},
		{Section: "History", Key: "History.GlobalLimit", Label: "History global", Value: strconv.Itoa(cfg.History.GlobalLimit)},
		{Section: "History", Key: "History.LogLimit", Label: "History log", Value: strconv.Itoa(cfg.History.LogLimit)},
		{Section: "History", Key: "History.RetentionDays", Label: "Retention days", Value: strconv.Itoa(cfg.History.RetentionDays)},
		{Section: "History", Key: "History.CleanupIntervalHours", Label: "Cleanup hours", Value: strconv.Itoa(cfg.History.CleanupIntervalHours)},
		{Section: "History", Key: "History.ArchiveDBPath", Label: "Archive path", Value: cfg.History.ArchiveDBPath},
		{Section: "History", Key: "History.MaintenanceBatch", Label: "Maintenance batch", Value: strconv.Itoa(cfg.History.MaintenanceBatch)},
		{Section: "History", Key: "History.ArchiveEnabled", Label: "Archive enabled", Value: boolText(cfg.History.ArchiveEnabled), Bool: true},
		{Section: "UI", Key: "UI.StartMinimized", Label: "Start minimized", Value: boolText(cfg.UI.StartMinimized), Bool: true},
		{Section: "UI", Key: "UI.MinimizeToTray", Label: "Minimize to tray", Value: boolText(cfg.UI.MinimizeToTray), Bool: true},
		{Section: "UI", Key: "UI.CloseToTray", Label: "Close to tray", Value: boolText(cfg.UI.CloseToTray), Bool: true},
		{Section: "UI", Key: "UI.FontSize", Label: "UI font", Value: strconv.Itoa(cfg.UI.FontSize)},
		{Section: "CID Rules", Key: "CidRules.RequiredPrefix", Label: "CID prefix", Value: cfg.CidRules.RequiredPrefix},
		{Section: "CID Rules", Key: "CidRules.ValidLength", Label: "CID length", Value: strconv.Itoa(cfg.CidRules.ValidLength)},
		{Section: "CID Rules", Key: "CidRules.AccountRanges", Label: "CID account ranges", Value: formatAccountRanges(cfg.CidRules.AccountRanges)},
	}
}

func (m *model) collectConfig() (config.AppConfig, error) {
	cfg := m.cfg
	for _, f := range m.settings {
		switch f.Key {
		case "Server.Host":
			cfg.Server.Host = strings.TrimSpace(f.Value)
		case "Server.Port":
			cfg.Server.Port = strings.TrimSpace(f.Value)
		case "Client.Host":
			cfg.Client.Host = strings.TrimSpace(f.Value)
		case "Client.Port":
			cfg.Client.Port = strings.TrimSpace(f.Value)
		case "Client.ReconnectInitial":
			cfg.Client.ReconnectInitial = strings.TrimSpace(f.Value)
		case "Client.ReconnectMax":
			cfg.Client.ReconnectMax = strings.TrimSpace(f.Value)
		case "Queue.BufferSize":
			cfg.Queue.BufferSize = atoiOr(cfg.Queue.BufferSize, f.Value)
		case "Monitoring.PpkTimeout":
			cfg.Monitoring.PpkTimeout = strings.TrimSpace(f.Value)
		case "Logging.LogDir":
			cfg.Logging.LogDir = strings.TrimSpace(f.Value)
		case "Logging.Filename":
			cfg.Logging.Filename = strings.TrimSpace(f.Value)
		case "Logging.MaxSize":
			cfg.Logging.MaxSize = atoiOr(cfg.Logging.MaxSize, f.Value)
		case "Logging.MaxBackups":
			cfg.Logging.MaxBackups = atoiOr(cfg.Logging.MaxBackups, f.Value)
		case "Logging.MaxAge":
			cfg.Logging.MaxAge = atoiOr(cfg.Logging.MaxAge, f.Value)
		case "Logging.Level":
			cfg.Logging.Level = strings.TrimSpace(strings.ToLower(f.Value))
		case "Logging.EnableConsole":
			cfg.Logging.EnableConsole = parseBool(f.Value)
		case "Logging.EnableFile":
			cfg.Logging.EnableFile = parseBool(f.Value)
		case "Logging.PrettyConsole":
			cfg.Logging.PrettyConsole = parseBool(f.Value)
		case "Logging.SamplingEnabled":
			cfg.Logging.SamplingEnabled = parseBool(f.Value)
		case "History.GlobalLimit":
			cfg.History.GlobalLimit = atoiOr(cfg.History.GlobalLimit, f.Value)
		case "History.LogLimit":
			cfg.History.LogLimit = atoiOr(cfg.History.LogLimit, f.Value)
		case "History.RetentionDays":
			cfg.History.RetentionDays = atoiOr(cfg.History.RetentionDays, f.Value)
		case "History.CleanupIntervalHours":
			cfg.History.CleanupIntervalHours = atoiOr(cfg.History.CleanupIntervalHours, f.Value)
		case "History.ArchiveDBPath":
			cfg.History.ArchiveDBPath = strings.TrimSpace(f.Value)
		case "History.MaintenanceBatch":
			cfg.History.MaintenanceBatch = atoiOr(cfg.History.MaintenanceBatch, f.Value)
		case "History.ArchiveEnabled":
			cfg.History.ArchiveEnabled = parseBool(f.Value)
		case "UI.StartMinimized":
			cfg.UI.StartMinimized = parseBool(f.Value)
		case "UI.MinimizeToTray":
			cfg.UI.MinimizeToTray = parseBool(f.Value)
		case "UI.CloseToTray":
			cfg.UI.CloseToTray = parseBool(f.Value)
		case "UI.FontSize":
			cfg.UI.FontSize = clampInt(atoiOr(cfg.UI.FontSize, f.Value), 7, 30)
		case "CidRules.RequiredPrefix":
			cfg.CidRules.RequiredPrefix = strings.TrimSpace(f.Value)
		case "CidRules.ValidLength":
			cfg.CidRules.ValidLength = atoiOr(cfg.CidRules.ValidLength, f.Value)
		case "CidRules.AccountRanges":
			ranges, err := parseAccountRanges(f.Value)
			if err != nil {
				return cfg, err
			}
			if len(ranges) > 0 {
				cfg.CidRules.AccountRanges = ranges
			}
		}
	}
	config.Normalize(&cfg)
	return cfg, nil
}

func buildRelayFields(rule core.RelayFilterRule) []settingField {
	if rule.ObjectCodes == nil {
		rule.ObjectCodes = map[int][]string{}
	}
	return []settingField{
		{Section: "General", Key: "Enabled", Label: "Enabled", Value: boolText(rule.Enabled), Bool: true},
		{Section: "Scope", Key: "GroupNumbers", Label: "Groups CSV", Value: formatIntCSV(rule.GroupNumbers)},
		{Section: "Scope", Key: "ObjectIDs", Label: "Global object IDs", Value: formatIntCSV(rule.ObjectIDs)},
		{Section: "Codes", Key: "Codes", Label: "Global blocked codes", Value: formatCodeCSV(rule.Codes)},
		{Section: "Codes", Key: "ObjectCodes", Label: "Object specific codes", Value: formatObjectCodesMap(rule.ObjectCodes)},
	}
}

func (m *model) isSettingVisible(idx int) bool {
	if idx < 0 || idx >= len(m.settings) {
		return false
	}
	q := strings.ToLower(strings.TrimSpace(m.settingsQuery))
	if q == "" {
		return true
	}
	f := m.settings[idx]
	hay := strings.ToLower(strings.TrimSpace(f.Section + " " + f.Key + " " + f.Label + " " + f.Value))
	return strings.Contains(hay, q)
}

func (m *model) isRelayVisible(idx int) bool {
	if idx < 0 || idx >= len(m.relayFields) {
		return false
	}
	q := strings.ToLower(strings.TrimSpace(m.relayQuery))
	if q == "" {
		return true
	}
	f := m.relayFields[idx]
	hay := strings.ToLower(strings.TrimSpace(f.Section + " " + f.Key + " " + f.Label + " " + f.Value))
	return strings.Contains(hay, q)
}

func (m *model) ensureSettingSelectionVisible() {
	if len(m.settings) == 0 {
		m.selSetting = 0
		return
	}
	if m.isSettingVisible(m.selSetting) {
		return
	}
	for i := range m.settings {
		if m.isSettingVisible(i) {
			m.selSetting = i
			return
		}
	}
	m.selSetting = 0
}

func (m *model) ensureRelaySelectionVisible() {
	if len(m.relayFields) == 0 {
		m.selRelay = 0
		return
	}
	if m.isRelayVisible(m.selRelay) {
		return
	}
	for i := range m.relayFields {
		if m.isRelayVisible(i) {
			m.selRelay = i
			return
		}
	}
	m.selRelay = 0
}

func (m *model) moveSettingSelection(dir int) {
	if len(m.settings) == 0 {
		return
	}
	m.ensureSettingSelectionVisible()
	start := m.selSetting
	n := len(m.settings)
	for step := 0; step < n; step++ {
		start = (start + dir + n) % n
		if m.isSettingVisible(start) {
			m.selSetting = start
			return
		}
	}
}

func (m *model) moveRelaySelection(dir int) {
	if len(m.relayFields) == 0 {
		return
	}
	m.ensureRelaySelectionVisible()
	start := m.selRelay
	n := len(m.relayFields)
	for step := 0; step < n; step++ {
		start = (start + dir + n) % n
		if m.isRelayVisible(start) {
			m.selRelay = start
			return
		}
	}
}

func collectRelayFields(fields []settingField) core.RelayFilterRule {
	rule := core.RelayFilterRule{
		ObjectCodes: map[int][]string{},
	}
	for _, f := range fields {
		switch f.Key {
		case "Enabled":
			rule.Enabled = parseBool(f.Value)
		case "GroupNumbers":
			rule.GroupNumbers = parseIntCSV(f.Value)
		case "ObjectIDs":
			rule.ObjectIDs = parseIntCSV(f.Value)
		case "Codes":
			rule.Codes = parseCodesCSV(f.Value)
		case "ObjectCodes":
			rule.ObjectCodes = parseObjectCodesMap(f.Value)
		}
	}
	return rule
}

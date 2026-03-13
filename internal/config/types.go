package config

import "strings"

type AppConfig struct {
	Server     ServerConfig     `yaml:"Server"`
	Client     ClientConfig     `yaml:"Client"`
	Queue      QueueConfig      `yaml:"Queue"`
	Logging    LoggingConfig    `yaml:"Logging"`
	CidRules   CidRulesConfig   `yaml:"CidRules"`
	Monitoring MonitoringConfig `yaml:"Monitoring"`
	UI         UIConfig         `yaml:"UI"`
	History    HistoryConfig    `yaml:"History"`
}

type ServerConfig struct {
	Host            string `yaml:"Host"`
	Port            string `yaml:"Port"`
	MaxDeviceEvents int    `yaml:"MaxDeviceEvents"`
	MaxGlobalEvents int    `yaml:"MaxGlobalEvents"`
}

type ClientConfig struct {
	Host             string `yaml:"Host"`
	Port             string `yaml:"Port"`
	ReconnectInitial string `yaml:"ReconnectInitial"`
	ReconnectMax     string `yaml:"ReconnectMax"`
}

type QueueConfig struct {
	BufferSize int `yaml:"BufferSize"`
}

type LoggingConfig struct {
	LogDir          string `yaml:"LogDir"`
	Filename        string `yaml:"Filename"`
	MaxSize         int    `yaml:"MaxSize"`
	MaxBackups      int    `yaml:"MaxBackups"`
	MaxAge          int    `yaml:"MaxAge"`
	Compress        bool   `yaml:"Compress"`
	Level           string `yaml:"Level"`
	EnableConsole   bool   `yaml:"EnableConsole"`
	EnableFile      bool   `yaml:"EnableFile"`
	PrettyConsole   bool   `yaml:"PrettyConsole"`
	SamplingEnabled bool   `yaml:"SamplingEnabled"`
}

type CidRulesConfig struct {
	RequiredPrefix string            `yaml:"RequiredPrefix"`
	ValidLength    int               `yaml:"ValidLength"`
	TestCodeMap    map[string]string `yaml:"TestCodeMap"`
	AccNumOffset   int               `yaml:"AccNumOffset"`
	AccNumAdd      int               `yaml:"AccNumAdd"`
	AccountRanges  []AccountRange    `yaml:"AccountRanges"`
}

type AccountRange struct {
	From  int `yaml:"From"`
	To    int `yaml:"To"`
	Delta int `yaml:"Delta"`
}

type MonitoringConfig struct {
	PpkTimeout string `yaml:"PpkTimeout"`
}

type UIConfig struct {
	StartMinimized bool `yaml:"StartMinimized"`
	MinimizeToTray bool `yaml:"MinimizeToTray"`
	CloseToTray    bool `yaml:"CloseToTray"`
	FontSize       int  `yaml:"FontSize"`
}

type HistoryConfig struct {
	LogLimit             int    `yaml:"LogLimit"`
	GlobalLimit          int    `yaml:"GlobalLimit"`
	RetentionDays        int    `yaml:"RetentionDays"`
	CleanupIntervalHours int    `yaml:"CleanupIntervalHours"`
	ArchiveEnabled       bool   `yaml:"ArchiveEnabled"`
	ArchiveDBPath        string `yaml:"ArchiveDbPath"`
	MaintenanceBatch     int    `yaml:"MaintenanceBatch"`
}

func DefaultConfig() AppConfig {
	return AppConfig{
		Server: ServerConfig{Host: "0.0.0.0", Port: "20005", MaxDeviceEvents: 100, MaxGlobalEvents: 500},
		Client: ClientConfig{Host: "10.32.1.49", Port: "20004", ReconnectInitial: "1s", ReconnectMax: "1m"},
		Queue:  QueueConfig{BufferSize: 100},
		Logging: LoggingConfig{
			LogDir:          "log",
			Filename:        "app.log",
			MaxSize:         10,
			MaxBackups:      5,
			MaxAge:          28,
			Compress:        true,
			Level:           "info",
			EnableConsole:   true,
			EnableFile:      true,
			PrettyConsole:   true,
			SamplingEnabled: false,
		},
		CidRules: CidRulesConfig{
			RequiredPrefix: "5",
			ValidLength:    20,
			TestCodeMap:    map[string]string{"E603": "E602"},
			AccNumOffset:   2100,
			AccNumAdd:      2100,
			AccountRanges:  []AccountRange{{From: 2000, To: 2200, Delta: 2100}},
		},
		Monitoring: MonitoringConfig{PpkTimeout: "15m"},
		UI:         UIConfig{StartMinimized: false, MinimizeToTray: false, CloseToTray: false, FontSize: 14},
		History: HistoryConfig{
			LogLimit:             100,
			GlobalLimit:          500,
			RetentionDays:        30,
			CleanupIntervalHours: 1,
			ArchiveEnabled:       true,
			ArchiveDBPath:        "archive.db",
			MaintenanceBatch:     5000,
		},
	}
}

func Normalize(cfg *AppConfig) {
	if cfg == nil {
		return
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	cfg.Logging.Level = normalizeLogLevel(cfg.Logging.Level)
	if cfg.History.RetentionDays <= 0 {
		cfg.History.RetentionDays = 30
	}
	if cfg.History.CleanupIntervalHours <= 0 {
		cfg.History.CleanupIntervalHours = 1
	}
	if cfg.History.MaintenanceBatch <= 0 {
		cfg.History.MaintenanceBatch = 5000
	}
	if strings.TrimSpace(cfg.History.ArchiveDBPath) == "" {
		cfg.History.ArchiveDBPath = "archive.db"
	}
	if cfg.History.GlobalLimit <= 0 {
		cfg.History.GlobalLimit = 500
	}
	if cfg.History.LogLimit <= 0 {
		cfg.History.LogLimit = 100
	}
	if cfg.Queue.BufferSize <= 0 {
		cfg.Queue.BufferSize = 100
	}
	if cfg.CidRules.ValidLength <= 0 {
		cfg.CidRules.ValidLength = 20
	}
	if cfg.CidRules.AccNumAdd == 0 && cfg.CidRules.AccNumOffset != 0 {
		cfg.CidRules.AccNumAdd = cfg.CidRules.AccNumOffset
	}
	if cfg.CidRules.AccNumOffset == 0 && cfg.CidRules.AccNumAdd != 0 {
		cfg.CidRules.AccNumOffset = cfg.CidRules.AccNumAdd
	}
	if cfg.Server.MaxDeviceEvents <= 0 {
		cfg.Server.MaxDeviceEvents = 100
	}
	if cfg.Server.MaxGlobalEvents <= 0 {
		cfg.Server.MaxGlobalEvents = 500
	}
	if cfg.CidRules.TestCodeMap == nil {
		cfg.CidRules.TestCodeMap = map[string]string{}
	}
	if len(cfg.CidRules.AccountRanges) == 0 {
		cfg.CidRules.AccountRanges = []AccountRange{{From: 2000, To: 2200, Delta: cfg.CidRules.AccNumAdd}}
	}
	normalized := make([]AccountRange, 0, len(cfg.CidRules.AccountRanges))
	for _, r := range cfg.CidRules.AccountRanges {
		if r.From > r.To {
			r.From, r.To = r.To, r.From
		}
		if r.From == 0 && r.To == 0 && r.Delta == 0 {
			continue
		}
		normalized = append(normalized, r)
	}
	if len(normalized) == 0 {
		normalized = []AccountRange{{From: 2000, To: 2200, Delta: cfg.CidRules.AccNumAdd}}
	}
	cfg.CidRules.AccountRanges = normalized
	if cfg.UI.FontSize < 7 {
		cfg.UI.FontSize = 7
	}
	if cfg.UI.FontSize > 30 {
		cfg.UI.FontSize = 30
	}
}

func normalizeLogLevel(level string) string {
	v := strings.ToLower(strings.TrimSpace(level))
	switch v {
	case "trace", "debug", "info", "warn", "error", "fatal":
		return v
	default:
		return "info"
	}
}

//go:build windows

package appmain

import (
	"context"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"cid_gio_gio/internal/config"
	appLog "cid_gio_gio/internal/logger"
	appRuntime "cid_gio_gio/internal/runtime"
	"cid_gio_gio/internal/ui"
	"github.com/rs/zerolog/log"
)

func Run() {
	defer appLog.RecoverPanic("main")
	configPath := appRuntime.ResolveConfigPath()
	cfg, cfgErr := config.NewStore(configPath).Load()
	if cfgErr != nil {
		cfg = config.DefaultConfig()
	}
	if err := appLog.SetupFromAppConfig(cfg.Logging); err != nil {
		panic(err)
	}
	if cfgErr != nil {
		log.Warn().Err(cfgErr).Str("config_path", configPath).Msg("failed to load config before logger setup, defaults applied")
	}
	startPprof(cfg.Profiling)

	startedAt := time.Now()
	rt := appRuntime.NewRuntime(configPath)
	err := ui.Run(context.Background(), rt)
	uptime := time.Since(startedAt).Round(time.Second)
	if err != nil {
		log.Error().Err(err).Dur("uptime", uptime).Msg("application exited with error")
		appLog.Close()
		os.Exit(1)
	}
	log.Info().Dur("uptime", uptime).Msg("application exited")
	appLog.Close()
	os.Exit(0)
}

func startPprof(cfg config.ProfilingConfig) {
	if !cfg.Enabled {
		return
	}
	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	go func() {
		log.Info().Str("addr", addr).Msg("pprof server started")
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Error().Err(err).Str("addr", addr).Msg("pprof server stopped")
		}
	}()
}

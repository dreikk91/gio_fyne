package main

import (
	"context"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"

	"cid_fyne/internal/config"
	appLog "cid_fyne/internal/logger"
	appRuntime "cid_fyne/internal/runtime"
	"cid_fyne/internal/ui/tea"
	"github.com/rs/zerolog/log"
)

func main() {
	defer appLog.RecoverPanic("main-tea")
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

	rt := appRuntime.NewRuntime(configPath)
	if err := tea.Run(context.Background(), rt); err != nil {
		log.Error().Err(err).Msg("application exited with error")
		appLog.Close()
		os.Exit(1)
	}
	log.Info().Msg("application exited")
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

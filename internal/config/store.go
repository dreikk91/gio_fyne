package config

import (
	"os"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (AppConfig, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			Normalize(&cfg)
			if saveErr := s.Save(cfg); saveErr != nil {
				return cfg, saveErr
			}
			log.Info().Str("path", s.path).Msg("config file created with defaults")
			return cfg, nil
		}
		log.Error().Err(err).Str("path", s.path).Msg("config read failed")
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Error().Err(err).Str("path", s.path).Msg("config parse failed")
		return cfg, err
	}
	Normalize(&cfg)
	log.Debug().Str("path", s.path).Msg("config loaded")
	return cfg, nil
}

func (s *Store) Save(cfg AppConfig) error {
	Normalize(&cfg)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		log.Error().Err(err).Msg("config marshal failed")
		return err
	}
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		log.Error().Err(err).Str("path", s.path).Msg("config write failed")
		return err
	}
	log.Info().Str("path", s.path).Msg("config saved")
	return nil
}

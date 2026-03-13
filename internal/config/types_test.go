package config

import "testing"

func TestNormalizeUsesLegacyAccNumOffsetAsAlias(t *testing.T) {
	cfg := AppConfig{
		CidRules: CidRulesConfig{
			AccNumOffset: 1700,
		},
	}

	Normalize(&cfg)

	if cfg.CidRules.AccNumAdd != 1700 {
		t.Fatalf("AccNumAdd = %d, want 1700", cfg.CidRules.AccNumAdd)
	}
	if cfg.CidRules.AccNumOffset != 1700 {
		t.Fatalf("AccNumOffset = %d, want 1700", cfg.CidRules.AccNumOffset)
	}
}

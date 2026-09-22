package gologger_test

import (
	"testing"

	"github.com/assurrussa/gologger"
)

func TestConfigEnvironmentPredicates(t *testing.T) {
	for _, test := range []struct {
		env        string
		production bool
		staging    bool
		dev        bool
		local      bool
	}{
		{env: "production", production: true},
		{env: "staging", staging: true},
		{env: "stage", staging: true},
		{env: "development", dev: true, local: true},
		{env: "local", local: true},
		{env: ""},
		{env: "unknown"},
	} {
		t.Run(test.env, func(t *testing.T) {
			cfg := gologger.Config{Env: test.env}
			if cfg.IsProduction() != test.production || cfg.IsStaging() != test.staging ||
				cfg.IsDev() != test.dev || cfg.IsLocal() != test.local {
				t.Fatalf("unexpected environment classification for %q: production=%t staging=%t dev=%t local=%t",
					test.env, cfg.IsProduction(), cfg.IsStaging(), cfg.IsDev(), cfg.IsLocal())
			}
		})
	}
}

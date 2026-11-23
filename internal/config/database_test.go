package config

import (
	"testing"
	"time"
)

func TestDatabasePoolConfig_TuningDefaults(t *testing.T) {
	cfg := Config{
		DatabaseURL:      "postgres://postgres:postgres@localhost:5432/wecratfs_test?sslmode=disable",
		DatabaseMaxConns: 30,
		DatabaseMinConns: 8,
	}

	poolCfg, err := cfg.DatabasePoolConfig()
	if err != nil {
		t.Fatalf("unexpected error parsing DatabasePoolConfig: %v", err)
	}

	if poolCfg.MaxConns != 30 {
		t.Errorf("expected MaxConns=30, got %d", poolCfg.MaxConns)
	}
	if poolCfg.MinConns != 8 {
		t.Errorf("expected MinConns=8, got %d", poolCfg.MinConns)
	}
	if poolCfg.MaxConnLifetime != time.Hour {
		t.Errorf("expected MaxConnLifetime=1h, got %v", poolCfg.MaxConnLifetime)
	}
	if poolCfg.MaxConnIdleTime != 15*time.Minute {
		t.Errorf("expected MaxConnIdleTime=15m, got %v", poolCfg.MaxConnIdleTime)
	}
	if poolCfg.HealthCheckPeriod != time.Minute {
		t.Errorf("expected HealthCheckPeriod=1m, got %v", poolCfg.HealthCheckPeriod)
	}
}

func TestDatabasePoolConfig_ZeroDefaults(t *testing.T) {
	cfg := Config{
		DatabaseURL: "postgres://postgres:postgres@localhost:5432/wecratfs_test?sslmode=disable",
	}

	poolCfg, err := cfg.DatabasePoolConfig()
	if err != nil {
		t.Fatalf("unexpected error parsing DatabasePoolConfig: %v", err)
	}

	if poolCfg.MaxConns != 25 {
		t.Errorf("expected default MaxConns=25, got %d", poolCfg.MaxConns)
	}
	if poolCfg.MinConns != 5 {
		t.Errorf("expected default MinConns=5, got %d", poolCfg.MinConns)
	}
}

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
	if poolCfg.ConnConfig.ConnectTimeout != 5*time.Second {
		t.Errorf("expected ConnectTimeout=5s, got %v", poolCfg.ConnConfig.ConnectTimeout)
	}
	if poolCfg.ConnConfig.RuntimeParams["statement_timeout"] != "15000" {
		t.Errorf("expected statement_timeout=15000, got %s", poolCfg.ConnConfig.RuntimeParams["statement_timeout"])
	}
	if poolCfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] != "10000" {
		t.Errorf("expected idle_in_transaction_session_timeout=10000, got %s", poolCfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"])
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

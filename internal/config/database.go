package config

import (
	"crypto/x509"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabasePoolConfig trusts a managed database's CA without disabling hostname
// or certificate verification. DATABASE_CA_CERT contains a public PEM certificate.
func (c Config) DatabasePoolConfig() (*pgxpool.Config, error) {
	pool, err := pgxpool.ParseConfig(c.DatabaseURL)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL configuration")
	}

	maxConns := int32(c.DatabaseMaxConns)
	if maxConns <= 0 {
		maxConns = 25
	}
	minConns := int32(c.DatabaseMinConns)
	if minConns <= 0 {
		minConns = 5
	}
	pool.MaxConns = maxConns
	pool.MinConns = minConns
	pool.MaxConnLifetime = time.Hour
	pool.MaxConnIdleTime = 15 * time.Minute
	pool.HealthCheckPeriod = time.Minute
	pool.ConnConfig.ConnectTimeout = 5 * time.Second
	if pool.ConnConfig.RuntimeParams == nil {
		pool.ConnConfig.RuntimeParams = make(map[string]string)
	}
	pool.ConnConfig.RuntimeParams["statement_timeout"] = "15000"
	pool.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "10000"
	if c.DatabaseCACert == "" {
		return pool, nil
	}
	if pool.ConnConfig.TLSConfig == nil {
		return nil, errors.New("DATABASE_CA_CERT requires a TLS database connection")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(c.DatabaseCACert)) {
		return nil, errors.New("DATABASE_CA_CERT must contain a valid PEM certificate")
	}
	pool.ConnConfig.TLSConfig.RootCAs = roots
	for _, fallback := range pool.ConnConfig.Fallbacks {
		if fallback.TLSConfig == nil {
			return nil, errors.New("DATABASE_CA_CERT cannot allow plaintext database fallback")
		}
		fallback.TLSConfig.RootCAs = roots
	}
	return pool, nil
}

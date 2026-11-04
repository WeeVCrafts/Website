package config

import (
	"crypto/x509"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabasePoolConfig trusts a managed database's CA without disabling hostname
// or certificate verification. DATABASE_CA_CERT contains a public PEM certificate.
func (c Config) DatabasePoolConfig() (*pgxpool.Config, error) {
	pool, err := pgxpool.ParseConfig(c.DatabaseURL)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL configuration")
	}
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

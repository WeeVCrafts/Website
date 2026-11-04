package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func TestDatabasePoolConfigManagedCA(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	cfg := Config{DatabaseURL: "postgres://app:secret@postgres.railway.internal/db?sslmode=verify-full", DatabaseCACert: ca}
	pool, err := cfg.DatabasePoolConfig()
	if err != nil {
		t.Fatal(err)
	}
	tls := pool.ConnConfig.TLSConfig
	if tls == nil || tls.InsecureSkipVerify || tls.ServerName != "postgres.railway.internal" || tls.RootCAs == nil {
		t.Fatal("managed CA must preserve full TLS verification")
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parsed.Verify(x509.VerifyOptions{Roots: tls.RootCAs}); err != nil {
		t.Fatalf("CA is not trusted: %v", err)
	}
	if len(pool.ConnConfig.Fallbacks) != 0 {
		t.Fatal("verify-full must not permit fallbacks")
	}
	t.Setenv("DATABASE_CA_CERT", ca)
	if FromEnv().DatabaseCACert != ca {
		t.Fatal("CA environment variable not loaded")
	}
}

func TestDatabasePoolConfigRejectsUnsafeCA(t *testing.T) {
	for _, url := range []string{"postgres://app:secret@localhost/db?sslmode=disable", "postgres://app:secret@localhost/db?sslmode=verify-full"} {
		if _, err := (Config{DatabaseURL: url, DatabaseCACert: "invalid certificate"}).DatabasePoolConfig(); err == nil {
			t.Fatalf("accepted invalid CA for %s", url)
		}
	}
	pool, err := (Config{DatabaseURL: "postgres://app:secret@localhost/db?sslmode=disable"}).DatabasePoolConfig()
	if err != nil || pool.ConnConfig.TLSConfig != nil {
		t.Fatalf("local configuration changed: %v", err)
	}
}

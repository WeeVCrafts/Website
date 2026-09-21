package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestCreateCustomerRecordsRequiredRegistrationConsents(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	email := fmt.Sprintf("registration-consent-%d@example.invalid", time.Now().UnixNano())
	user, err := NewUserRepository(pool).CreateCustomer(ctx, email, "Consent Fixture", "fixture-hash")
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID)
	})
	rows, err := pool.Query(ctx, `SELECT purpose, policy_version, granted, source FROM consent_records WHERE user_id=$1 AND source='registration'`, user.ID)
	if err != nil {
		t.Fatalf("read registration consent rows: %v", err)
	}
	defer rows.Close()
	consents := map[string]bool{}
	for rows.Next() {
		var purpose, policyVersion, source string
		var granted bool
		if err := rows.Scan(&purpose, &policyVersion, &granted, &source); err != nil {
			t.Fatalf("scan registration consent: %v", err)
		}
		if policyVersion != "privacy-2026-09" || source != "registration" || !granted {
			t.Fatalf("unexpected registration consent metadata: %s %s %v %s", purpose, policyVersion, granted, source)
		}
		consents[purpose] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate registration consent rows: %v", err)
	}
	if !consents["necessary"] || !consents["terms"] || len(consents) != 2 {
		t.Fatalf("required consent rows not stored: %+v", consents)
	}
}

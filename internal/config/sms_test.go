package config

import (
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

func validSMSConfig() SMSConfig {
	c := SMSConfig{Enabled: true, KYCVerified: true, DLTVerified: true, AuthKey: "fixture-only-authentication-key", SenderID: "WEEVCR", EntityID: "1234567890123456789", Templates: map[string]ports.SMSTemplate{}}
	for _, key := range []string{"customer_order", "seller_order", "customer_support", "seller_support"} {
		c.Templates[key] = ports.SMSTemplate{ID: "0123456789abcdef01234567", DLTID: "1234567890123456789", Variables: map[string]string{"VAR1": "reference", "VAR2": "status"}}
	}
	return c
}

func TestSMSRequiresActualAccountApprovalAndProduction(t *testing.T) {
	c := validSMSConfig()
	if c.Validate("production", false) != nil {
		t.Fatal("valid configuration rejected")
	}
	for _, change := range []func(*SMSConfig){func(c *SMSConfig) { c.KYCVerified = false }, func(c *SMSConfig) { c.DLTVerified = false }, func(c *SMSConfig) { c.AuthKey = "" }, func(c *SMSConfig) { c.EntityID = "pending" }, func(c *SMSConfig) { c.SenderID = "FAKE" }, func(c *SMSConfig) { delete(c.Templates, "seller_order") }, func(c *SMSConfig) { c.Templates["customer_order"] = ports.SMSTemplate{ID: "draft"} }} {
		c := validSMSConfig()
		change(&c)
		if c.Validate("production", false) == nil {
			t.Fatal("incomplete live SMS configuration accepted")
		}
	}
	if c.Validate("development", false) == nil || c.Validate("production", true) == nil {
		t.Fatal("isolated environment could send SMS")
	}
	c.Enabled = false
	c.KYCVerified = false
	c.DLTVerified = false
	if c.Validate("production", false) != nil {
		t.Fatal("draft configuration prevented normal application startup")
	}
}

func TestSMSRejectsDuplicateOrReservedTemplateVariables(t *testing.T) {
	for _, vars := range []map[string]string{{"VAR1": "reference", "VAR2": "reference"}, {"mobiles": "reference", "VAR2": "status"}, {"VAR1": "reference", "VAR2": "body"}} {
		c := validSMSConfig()
		v := c.Templates["customer_order"]
		v.Variables = vars
		c.Templates["customer_order"] = v
		if c.Validate("production", false) == nil {
			t.Fatalf("unsafe mapping accepted: %v", vars)
		}
	}
}

package config

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/wecratfs/commerce/internal/ports"
)

type SMSConfig struct {
	Enabled, KYCVerified, DLTVerified        bool
	AuthKey, AuthKeyFile, SenderID, EntityID string
	Templates                                map[string]ports.SMSTemplate
	parseError                               bool
}

func parseSMSConfig() SMSConfig {
	c := SMSConfig{Enabled: boolEnv("WECRATFS_SMS_ENABLED", false), KYCVerified: boolEnv("MSG91_KYC_VERIFIED", false), DLTVerified: boolEnv("MSG91_DLT_VERIFIED", false), AuthKey: env("MSG91_AUTHKEY", ""), AuthKeyFile: env("MSG91_AUTHKEY_FILE", ""), SenderID: env("MSG91_SENDER_ID", ""), EntityID: env("MSG91_ENTITY_ID", "")}
	c.parseError = json.Unmarshal([]byte(env("MSG91_SMS_TEMPLATES", "{}")), &c.Templates) != nil
	return c
}

func (c SMSConfig) Validate(environment string, staging bool) error {
	if !c.Enabled {
		return nil
	}
	if environment != "production" || staging {
		return errors.New("live transactional SMS requires production; isolated environments must not contact real recipients")
	}
	if !c.KYCVerified || !c.DLTVerified {
		return errors.New("MSG91 live SMS requires verified KYC and DLT sender/template mappings")
	}
	if c.AuthKey == "" && c.AuthKeyFile == "" || !regexp.MustCompile(`^[A-Z]{6}$`).MatchString(c.SenderID) || !regexp.MustCompile(`^[0-9]{19}$`).MatchString(c.EntityID) || c.parseError {
		return errors.New("MSG91 live SMS configuration is incomplete")
	}
	for _, kind := range []string{"customer_order", "seller_order", "customer_support", "seller_support"} {
		t, ok := c.Templates[kind]
		if !ok || !regexp.MustCompile(`^[a-fA-F0-9]{24}$`).MatchString(t.ID) || !regexp.MustCompile(`^[0-9]{19}$`).MatchString(t.DLTID) || len(t.Variables) != 2 {
			return errors.New("MSG91 requires four approved transactional template mappings")
		}
		seen := map[string]bool{}
		for key, value := range t.Variables {
			if strings.EqualFold(key, "mobiles") || !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,31}$`).MatchString(key) || value != "reference" && value != "status" || seen[value] {
				return errors.New("MSG91 template variables must map reference and status exactly once")
			}
			seen[value] = true
		}
	}
	return nil
}

func (c SMSConfig) ReadAuthKey() (string, error) {
	key := strings.TrimSpace(c.AuthKey)
	if c.AuthKeyFile != "" {
		data, err := os.ReadFile(c.AuthKeyFile)
		if err != nil {
			return "", errors.New("MSG91 private credential file unavailable")
		}
		key = strings.TrimSpace(string(data))
	}
	if len(key) < 16 || len(key) > 256 || strings.ContainsAny(key, "\r\n\x00\t ") {
		return "", errors.New("MSG91 authentication key invalid")
	}
	return key, nil
}

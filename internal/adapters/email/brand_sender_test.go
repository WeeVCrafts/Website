package email

import (
	"net/mail"
	"testing"
)

func TestPreferredSenderBrandRetainsConfiguredAddress(t *testing.T) {
	sender, err := NewResend("re_test_brand", "WeevCrafts Support <sender@example.test>", "")
	if err != nil {
		t.Fatal(err)
	}
	address, err := mail.ParseAddress(sender.from)
	if err != nil || address.Name != "Weevcrafts Support" || address.Address != "sender@example.test" {
		t.Fatalf("sender normalization changed identity: %+v %v", address, err)
	}
}

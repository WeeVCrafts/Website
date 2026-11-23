package main

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMerchantFeed_XMLNamespaceAndStructure(t *testing.T) {
	handler := merchantFeedHandler(nil, "https://weevcrafts.com")
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest("GET", "/feeds/google-merchant.xml", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.HasPrefix(body, xml.Header) {
		t.Errorf("expected xml header prefix")
	}
	if !strings.Contains(body, `xmlns:g="http://base.google.com/ns/1.0"`) {
		t.Errorf("missing google base xml namespace")
	}
	if !strings.Contains(body, `<title>Weevcrafts Product Feed</title>`) {
		t.Errorf("missing channel title")
	}
	if !strings.Contains(body, `<link>https://weevcrafts.com</link>`) {
		t.Errorf("missing channel link")
	}
}

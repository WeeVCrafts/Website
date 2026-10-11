package main

import (
	"encoding/xml"
	"net/http/httptest"
	"testing"
)

func TestSitemapProtocolNamespace(t *testing.T) {
	_, sitemap := seoHandler(nil, "https://weevcrafts.com")
	w := httptest.NewRecorder()
	sitemap(w, httptest.NewRequest("GET", "/sitemap.xml", nil))
	var document struct {
		XMLName xml.Name
		URLs    []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.XMLName.Space != "http://www.sitemaps.org/schemas/sitemap/0.9" || document.XMLName.Local != "urlset" {
		t.Fatalf("invalid sitemap namespace: %v", document.XMLName)
	}
	if len(document.URLs) == 0 || document.URLs[0].Loc != "https://weevcrafts.com/" {
		t.Fatal("missing canonical homepage")
	}
}

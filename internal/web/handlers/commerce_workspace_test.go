package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCommerceSetupCSRFSupportsSharedFormEncoding(t *testing.T) {
	for _, encoding := range []string{"urlencoded", "multipart"} {
		t.Run(encoding, func(t *testing.T) {
			var b bytes.Buffer
			kind := "application/x-www-form-urlencoded"
			if encoding == "multipart" {
				writer := multipart.NewWriter(&b)
				_ = writer.WriteField("csrf_token", "owner-session-csrf")
				_ = writer.WriteField("legal_name", "Recorded legal name")
				_ = writer.Close()
				kind = writer.FormDataContentType()
			} else {
				b.WriteString("csrf_token=owner-session-csrf&legal_name=Recorded+legal+name")
			}
			request := httptest.NewRequest("POST", "/seller-admin/setup", &b)
			request.Header.Set("Content-Type", kind)
			if !commerceSafePost(request, "owner-session-csrf") || request.PostForm.Get("legal_name") != "Recorded legal name" {
				t.Fatal("shared form payload not parsed")
			}
			bad := httptest.NewRequest("POST", "/seller-admin/setup", strings.NewReader("csrf_token=another-session"))
			bad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if commerceSafePost(bad, "owner-session-csrf") {
				t.Fatal("different session token accepted")
			}
		})
	}
}

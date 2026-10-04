package commerce

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"testing"
)

func TestSupportAttachmentValidation(t *testing.T) {
	service := NewSupportAttachmentService(nil, nil, nil)
	validPNG := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82}
	if err := service.Validate([]*multipart.FileHeader{supportAttachmentHeader(t, "proof.png", validPNG)}); err != nil {
		t.Fatalf("valid PNG rejected: %v", err)
	}
	if err := service.Validate([]*multipart.FileHeader{supportAttachmentHeader(t, "proof.gif", []byte("GIF89a"))}); err == nil {
		t.Fatal("unsupported GIF accepted")
	}
	headers := make([]*multipart.FileHeader, 0, maxSupportAttachments+1)
	for index := 0; index <= maxSupportAttachments; index++ {
		headers = append(headers, supportAttachmentHeader(t, "proof.png", validPNG))
	}
	if err := service.Validate(headers); err == nil {
		t.Fatal("more than three attachments accepted")
	}
}

func supportAttachmentHeader(t *testing.T, name string, body []byte) *multipart.FileHeader {
	t.Helper()
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	part, err := writer.CreateFormFile("attachments[]", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/support/create", &payload)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	return request.MultipartForm.File["attachments[]"][0]
}

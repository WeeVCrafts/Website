package handlers

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestAvatarValidationAndNormalization(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewRGBA(image.Rect(0, 0, 900, 600))); err != nil {
		t.Fatal(err)
	}
	output, err := normalizeAvatar(input.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(output))
	if err != nil || format != "jpeg" || config.Width != 512 || config.Height > 512 {
		t.Fatalf("invalid output: %v %s %v", config, format, err)
	}
	for _, invalid := range [][]byte{nil, []byte("<svg onload='alert(1)'></svg>"), make([]byte, maxAvatarBytes+1), input.Bytes()[:16]} {
		if _, err := normalizeAvatar(invalid); err == nil {
			t.Fatal("invalid upload accepted")
		}
	}
}
func TestProfileCompletionRequiresNameAndPhone(t *testing.T) {
	for _, tc := range []struct {
		name, phone string
		want        bool
	}{{"Ananya Sharma", "+91 98765 43210", true}, {"A", "9876543210", false}, {"Ananya", "", false}, {"Ananya", "123<script>", false}, {"Ananya", "1234567890123456", false}} {
		if validProfile(tc.name, tc.phone) != tc.want {
			t.Fatalf("incorrect completion for %q", tc.name)
		}
	}
}

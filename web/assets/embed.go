package assets

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"sync"
)

// FS contains the checked-in CSS, JavaScript, and local preview imagery.
//
//go:embed css js images fonts
var FS embed.FS

var versions sync.Map

// URL changes when an embedded asset changes, preserving immutable caching.
func URL(path string) string {
	if value, ok := versions.Load(path); ok {
		return value.(string)
	}
	data, err := FS.ReadFile(path)
	if err != nil {
		return "/assets/" + path
	}
	hash := sha256.Sum256(data)
	url := fmt.Sprintf("/assets/%s?v=%x", path, hash[:8])
	versions.Store(path, url)
	return url
}

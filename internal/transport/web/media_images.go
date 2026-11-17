package web

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/wecratfs/commerce/internal/adapters/photograph"
	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
)

// Disposable, bounded derivatives. Authorization is checked on every request
// before this cache is consulted; originals and catalogue records never change.
var productPhotographs = struct {
	sync.Mutex
	entries map[string]productPhotograph
	bytes   int
}{entries: make(map[string]productPhotograph)}

type productPhotograph struct {
	data        []byte
	contentType string
	expires     time.Time
}

var photographLoads = make(chan struct{}, 4)
var photographRequests singleflight.Group

func responsivePhotograph(ctx context.Context, media applicationcommerce.PublicMedia, width int) (productPhotograph, error) {
	key := fmt.Sprintf("%s/%d", media.CacheKey(), width)
	if photo, ok := cachedProductPhotograph(key); ok {
		return photo, nil
	}
	result := photographRequests.DoChan(key, func() (any, error) {
		if photo, ok := cachedProductPhotograph(key); ok {
			return photo, nil
		}
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
		defer cancel()
		select {
		case photographLoads <- struct{}{}:
			defer func() { <-photographLoads }()
		case <-loadCtx.Done():
			return nil, loadCtx.Err()
		}
		body, info, err := media.OpenDerivative(loadCtx, width)
		if err == nil {
			defer body.Close()
			data, readErr := io.ReadAll(io.LimitReader(body, info.ByteSize+1))
			if readErr == nil && int64(len(data)) == info.ByteSize {
				photo := productPhotograph{data: data, contentType: info.ContentType}
				rememberProductPhotograph(key, photo)
				return photo, nil
			}
		}
		body, info, err = media.Open(loadCtx)
		if err != nil {
			return nil, err
		}
		defer body.Close()
		data, err := io.ReadAll(io.LimitReader(body, info.ByteSize+1))
		if err != nil || int64(len(data)) != info.ByteSize {
			return nil, fmt.Errorf("invalid photograph size")
		}
		photo := productPhotograph{data: data, contentType: info.ContentType}
		resized, err := productPhotographVariant(data, width)
		if err == nil {
			if len(resized) < len(data) {
				photo.data, photo.contentType = resized, "image/jpeg"
			}
			rememberProductPhotograph(key, photo)
		}
		return photo, nil
	})
	select {
	case <-ctx.Done():
		return productPhotograph{}, ctx.Err()
	case loaded := <-result:
		if loaded.Err != nil {
			return productPhotograph{}, loaded.Err
		}
		return loaded.Val.(productPhotograph), nil
	}
}

func cachedProductPhotograph(key string) (productPhotograph, bool) {
	productPhotographs.Lock()
	defer productPhotographs.Unlock()
	entry, ok := productPhotographs.entries[key]
	if ok && time.Now().Before(entry.expires) {
		return entry, true
	}
	if ok {
		delete(productPhotographs.entries, key)
		productPhotographs.bytes -= len(entry.data)
	}
	return productPhotograph{}, false
}
func rememberProductPhotograph(key string, photo productPhotograph) {
	if len(photo.data) > 1<<20 {
		return
	}
	productPhotographs.Lock()
	defer productPhotographs.Unlock()
	if _, ok := productPhotographs.entries[key]; ok {
		return
	}
	for k, v := range productPhotographs.entries {
		if time.Now().After(v.expires) || len(productPhotographs.entries) >= 128 || productPhotographs.bytes+len(photo.data) > 32<<20 {
			delete(productPhotographs.entries, k)
			productPhotographs.bytes -= len(v.data)
		}
	}
	photo.expires = time.Now().Add(5 * time.Minute)
	productPhotographs.entries[key] = photo
	productPhotographs.bytes += len(photo.data)
}

var productPhotographVariant = photograph.Resize
var jpegOrientation = photograph.JPEGOrientation
var orientPhotograph = photograph.Orient

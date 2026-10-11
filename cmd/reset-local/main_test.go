package main

import (
	"github.com/wecratfs/commerce/internal/config"
	"testing"
)

func TestLocalDatabaseGuard(t *testing.T) {
	for _, tc := range []struct {
		url     string
		allowed bool
	}{
		{"postgres://u:p@localhost:5432/wecratfs", true},
		{"postgres://u:p@postgres:5432/wecratfs", true},
		{"postgresql://u:p@127.0.0.1:5432/wecratfs", true},
		{"postgres://u:p@db.example.com/wecratfs", false},
		{"postgres://u:p@localhost/production", false},
		{"https://localhost/wecratfs", false},
	} {
		if (validateLocalDatabase(tc.url) == nil) != tc.allowed {
			t.Errorf("guard for %s", tc.url)
		}
	}
}
func TestDestructiveServiceGuard(t *testing.T) {
	local := config.Config{RedisAddress: "localhost:6379", MeiliAddress: "localhost:7700", S3Address: "localhost:3900", S3Bucket: "wecratfs-media"}
	if err := validateLocalServices(local); err != nil {
		t.Fatal(err)
	}
	remote := local
	remote.S3Address = "storage.example.com:443"
	if validateLocalServices(remote) == nil {
		t.Fatal("remote storage accepted")
	}
	otherBucket := local
	otherBucket.S3Bucket = "production"
	if validateLocalServices(otherBucket) == nil {
		t.Fatal("unrelated bucket accepted")
	}
	remoteRedis := local
	remoteRedis.RedisURL = "redis://remote.example.com:6379"
	if validateLocalServices(remoteRedis) == nil {
		t.Fatal("remote Redis accepted")
	}
}

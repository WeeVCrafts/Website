package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func searchHealthProbe(address string) func(context.Context) error {
	address = strings.TrimRight(address, "/")
	if address != "" && !strings.Contains(address, "://") {
		address = "http://" + address
	}
	return func(ctx context.Context) error {
		if address == "" {
			return fmt.Errorf("search is not configured")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address+"/health", nil)
		if err != nil {
			return err
		}
		resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("search health returned %d", resp.StatusCode)
		}
		return nil
	}
}

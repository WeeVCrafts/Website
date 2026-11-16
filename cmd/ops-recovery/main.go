// ops-recovery streams a database and object-store recovery point to an
// encrypting caller, or uploads and verifies an already encrypted archive.
package main

import (
	"context"
	"fmt"
	"os"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 22*time.Minute)
	defer cancel()
	var err error
	switch {
	case len(os.Args) == 2 && os.Args[1] == "collect":
		err = collect(ctx, os.Stdout)
	case len(os.Args) == 3 && os.Args[1] == "upload":
		err = upload(ctx, os.Args[2])
	case len(os.Args) == 4 && os.Args[1] == "download":
		err = download(ctx, os.Args[2], os.Args[3])
	default:
		err = fmt.Errorf("usage: ops-recovery collect | upload ENCRYPTED_ARCHIVE | download ENCRYPTED_ARCHIVE EXPECTED_SHA256")
	}
	if err != nil {
		// The collectors return stage names and bounded counts, never keys,
		// configuration values, or HTTP and database error bodies.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

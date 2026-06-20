//go:build noui

package main

import "io/fs"

func getStaticFS() fs.FS {
	// When built with -tags noui, no embedded UI is available.
	// The server will return 404 for non-API routes.
	return nil
}
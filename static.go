package main

import (
	"embed"
	"io/fs"
	"net/http"
)

// webDist holds the built Astro dashboard. The dist/ directory is created
// by `make web-build` (bun run build) before `go build` runs.
//
// The `all:` prefix tells go:embed to include hidden files (Astro emits
// some). The empty file `web/dist/.gitkeep` keeps go:embed from failing on
// a fresh checkout before the first web build.
//
//go:embed all:web/dist
var webDist embed.FS

// staticFS returns the embedded dashboard as an http.FileSystem rooted at
// web/dist. Returns nil (and the api server skips dashboard routes) if no
// files were embedded, which can happen when running `go build` before
// `make web-build` for the first time.
func staticFS() http.FileSystem {
	sub, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		return nil
	}
	// If the embedded dir is empty, http.FS still works but every request
	// returns 404; not worth special-casing.
	return http.FS(sub)
}

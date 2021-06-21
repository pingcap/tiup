package uiserver

import (
	"io/fs"
	"net/http"
)

// Assets returns the ui assets
func Assets() http.FileSystem {
	stripped, err := fs.Sub(uiAssets, "ui-build")
	if err != nil {
		panic(err)
	}
	return http.FS(stripped)
}

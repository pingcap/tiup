// +build ui_server

package uiserver

import (
	"embed"
)

// uiAssets represent the frontend ui assets
//go:embed ui-build
var uiAssets embed.FS

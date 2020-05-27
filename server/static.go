package main

import (
	"net/http"
)

// staticServer start a static web server
func staticServer(fs http.FileSystem) http.Handler {
	return http.FileServer(fs)
}

func (s *server) static(prefix, root string) http.Handler {
	return http.StripPrefix(prefix, staticServer(http.Dir(root)))
}

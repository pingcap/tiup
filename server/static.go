package main

import (
	"net/http"
	"os"
	"path"
)

// WebServer start a static web server
func staticServer(fs http.FileSystem) http.Handler {
	fsh := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f, err := fs.Open(path.Clean(r.URL.Path)); err == nil {
			f.Close()
		} else if os.IsNotExist(err) {
			r.URL.Path = "/"
		}
		fsh.ServeHTTP(w, r)
	})
}

func (s *server) static(prefix, root string) http.Handler {
	return http.StripPrefix(prefix, staticServer(http.Dir(root)))
}

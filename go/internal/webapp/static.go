package webapp

import (
	"bytes"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

func NewStaticHandler(assets fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if strings.ContainsAny(name, "\\\x00") || strings.Contains(strings.ToLower(r.URL.EscapedPath()), "%2f") {
			http.Error(w, "Invalid path", 400)
			return
		}
		for _, segment := range strings.Split(name, "/") {
			if segment == "." || segment == ".." {
				http.Error(w, "Invalid path", 400)
				return
			}
		}
		name = strings.TrimSuffix(name, "/")
		if name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) {
			http.Error(w, "Invalid path", 400)
			return
		}
		candidates := []string{name}
		if path.Ext(name) == "" {
			candidates = []string{name + ".html", name + "/index.html"}
		}
		for _, candidate := range candidates {
			data, err := fs.ReadFile(assets, candidate)
			if err != nil {
				continue
			}
			if content := mime.TypeByExtension(path.Ext(candidate)); content != "" {
				w.Header().Set("Content-Type", content)
			}
			w.Header().Set("X-Content-Type-Options", "nosniff")
			http.ServeContent(w, r, candidate, time.Time{}, bytes.NewReader(data))
			return
		}
		http.NotFound(w, r)
	})
}

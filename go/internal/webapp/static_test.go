package webapp

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func fixtureFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":              {Data: []byte("<html>index</html>")},
		"exploration/system.html": {Data: []byte("<html>exploration</html>")},
		"_next/static/app.js":     {Data: []byte("console.log('fixture')")},
		"_next/static/app.css":    {Data: []byte("body{color:white}")},
	}
}

func TestStaticRoutes(t *testing.T) {
	handler := NewStaticHandler(fixtureFS())
	for _, tc := range []struct{ path, body, content string }{
		{"/", "<html>index</html>", "text/html"},
		{"/exploration/system", "<html>exploration</html>", "text/html"},
		{"/exploration/system/", "<html>exploration</html>", "text/html"},
		{"/_next/static/app.js", "console.log('fixture')", "javascript"},
		{"/_next/static/app.css", "body{color:white}", "text/css"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", tc.path, nil))
			if response.Code != 200 || response.Body.String() != tc.body || !strings.Contains(response.Header().Get("Content-Type"), tc.content) {
				t.Fatalf("wrong static response: %d %s %s", response.Code, response.Header(), response.Body.String())
			}
		})
	}
	for _, target := range []string{"/missing", "/../index.html", "/%2e%2e/index.html", "/exploration%2fsystem", "/%5cindex.html", "/%00index.html"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", target, nil))
		if response.Code < 400 {
			t.Errorf("unsafe/unknown path accepted: %s %d", target, response.Code)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("HEAD", "/exploration/system", nil))
	bytes, _ := io.ReadAll(response.Result().Body)
	if response.Code != 200 || len(bytes) != 0 {
		t.Fatal("HEAD must not send the asset body")
	}
}

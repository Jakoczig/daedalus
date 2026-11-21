package webapp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAtlasControlsRequireOwner(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, _, viewer, url, _ := g2eFixture(t)
	c := pairedSocket(t, viewer, url)
	defer c.CloseNow()
	for _, name := range []string{"configureGoAtlas", "checkGoAtlas", "installGoAtlas", "rollbackGoAtlas", "clearGoAtlasDetails", "clearGoAtlas", "getGoSpatialStatus", "configureGoSpatial", "installGoSpatial", "installGoBubble", "removeGoBubble", "rollbackGoSpatial", "retryGoSpatial"} {
		if len(rpcMessage(t, ctx, c, name, nil)["error"]) == 0 {
			t.Fatal("viewer reached atlas control", name)
		}
		r := httptest.NewRequest(http.MethodGet, "/atlas/assets/0123456789abcdef01234567/xz/0/0-0.png", nil)
		r.Host = strings.TrimPrefix(s.URL(), "http://")
		r.RemoteAddr = "127.0.0.1:40000"
		w := httptest.NewRecorder()
		s.http.Handler.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("%s unauthenticated atlas read = %d", name, w.Code)
		}
	}
}

package atlas

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSpatialRecoveredDownloadClearsStaleFailure(t *testing.T) {
	b := []byte("synthetic asset")
	n := spatialNeed{path: spatialPath("blocks", digest(b)), bytes: int64(len(b)), hash: digest(b)}
	c := &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, ContentLength: int64(len(b)), Body: io.NopCloser(strings.NewReader(string(b)))}, nil
	})}
	m := newSpatial(context.Background(), t.TempDir(), c, make([]byte, 32), time.Now)
	defer m.Close()
	m.Configure(SpatialPreferences{Downloads: true})
	m.lastError = "Spatial download failed; cached data retained, retry after backoff"
	m.backoff[n.path] = time.Now().Add(-time.Second)
	m.downloadAsset(context.Background(), n)
	if m.Status().Error != "" {
		t.Fatal("recovered download retained stale failure")
	}
}

package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DimaOshchepkov/url_shortener/internal/lib/logger/handlers/slogdiscard"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testLogger = slogdiscard.NewDiscardLogger()

// stubCacheInfo implements CacheInfo for testing.
type stubCacheInfo struct {
	hitRate float64
	length  int
}

func (s *stubCacheInfo) HitRate() float64 { return s.hitRate }
func (s *stubCacheInfo) Len() int         { return s.length }

func TestHealth_OK(t *testing.T) {
	cache := &stubCacheInfo{hitRate: 0.95, length: 42}

	r := chi.NewRouter()
	r.Get("/health", New(testLogger, cache))

	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "enabled", body["cache"])
	assert.InDelta(t, 0.95, body["hit_rate"].(float64), 0.01)
	assert.Equal(t, float64(42), body["size"].(float64))
}

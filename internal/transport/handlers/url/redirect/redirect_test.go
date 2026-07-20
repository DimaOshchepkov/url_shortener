package redirect

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DimaOshchepkov/url_shortener/internal/lib/logger/handlers/slogdiscard"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testLogger = slogdiscard.NewDiscardLogger()

// stubRedirector implements URLRedirector for testing.
type stubRedirector struct {
	urls            map[string]string
	getURLErr       error
	incClicksErr    error
	incrementCalls  int
	incrementedWith string
}

func newStubRedirector() *stubRedirector {
	return &stubRedirector{
		urls: make(map[string]string),
	}
}

func (s *stubRedirector) GetURL(_ context.Context, alias string) (string, error) {
	if s.getURLErr != nil {
		return "", s.getURLErr
	}
	url, ok := s.urls[alias]
	if !ok {
		return "", errors.New("url not found")
	}
	return url, nil
}

func (s *stubRedirector) IncrementClicks(_ context.Context, alias string) error {
	s.incrementCalls++
	s.incrementedWith = alias
	return s.incClicksErr
}

func setupRouter(sr URLRedirector) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/{alias}", New(testLogger, sr))
	return r
}

func TestRedirect_Success(t *testing.T) {
	sr := newStubRedirector()
	sr.urls["abc123"] = "https://example.com/page"

	ts := httptest.NewServer(setupRouter(sr))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/abc123", nil)
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse // don't follow redirect
	}}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "https://example.com/page", resp.Header.Get("Location"))
	assert.Equal(t, 1, sr.incrementCalls)
	assert.Equal(t, "abc123", sr.incrementedWith)
}

func TestRedirect_NotFound(t *testing.T) {
	sr := newStubRedirector()

	ts := httptest.NewServer(setupRouter(sr))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/nonexistent")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode) // JSON response still 200
	body := readBody(resp)
	assert.Contains(t, body, `"status":"Error"`)
}

func TestRedirect_EmptyAlias(t *testing.T) {
	sr := newStubRedirector()

	ts := httptest.NewServer(setupRouter(sr))
	defer ts.Close()

	// Request without alias path param — chi returns 404 for no matching route.
	resp, err := http.Get(ts.URL + "/")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// chi returns 404 when no route matches; that's fine — the handler isn't reached.
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestRedirect_InternalError(t *testing.T) {
	sr := newStubRedirector()
	sr.getURLErr = assert.AnError

	ts := httptest.NewServer(setupRouter(sr))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/abc123")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	body := readBody(resp)
	assert.Contains(t, body, `"status":"Error"`)
}

func TestRedirect_InvalidURLScheme(t *testing.T) {
	sr := newStubRedirector()
	sr.urls["bad"] = "ftp://evil.com"

	ts := httptest.NewServer(setupRouter(sr))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/bad", nil)
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// Should return JSON error, not redirect.
	body := readBody(resp)
	assert.Contains(t, body, `"status":"Error"`)
}

func readBody(resp *http.Response) string {
	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	return string(buf[:n])
}

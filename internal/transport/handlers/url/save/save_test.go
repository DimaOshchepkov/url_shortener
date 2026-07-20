package save

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	resp "github.com/DimaOshchepkov/url_shortener/internal/lib/api/response"
	"github.com/DimaOshchepkov/url_shortener/internal/lib/logger/handlers/slogdiscard"
	"github.com/DimaOshchepkov/url_shortener/internal/storage"
	get "github.com/DimaOshchepkov/url_shortener/internal/transport/middleware/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testLogger = slogdiscard.NewDiscardLogger()

// stubURLSaver implements URLSaver for testing.
type stubURLSaver struct {
	saveErr    error
	savedURL   string
	savedAlias string
}

func (s *stubURLSaver) SaveURL(_ context.Context, urlToSave string, alias string) error {
	s.savedURL = urlToSave
	s.savedAlias = alias
	return s.saveErr
}

// contextWithUID returns a context with the given UID set.
func contextWithUID(ctx context.Context, uid uint64) context.Context {
	return context.WithValue(ctx, get.UidKey, uid)
}

func TestSaveURL_Success(t *testing.T) {
	saver := &stubURLSaver{}

	handler := New(testLogger, saver)

	body := `{"url": "https://example.com", "alias": "custom"}`
	req := httptest.NewRequest(http.MethodPost, "/url", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(contextWithUID(req.Context(), 42))

	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, resp.StatusOK, response.Status)
	assert.Equal(t, "custom", response.Alias)
	assert.Equal(t, "https://example.com", saver.savedURL)
	assert.Equal(t, "custom", saver.savedAlias)
}

func TestSaveURL_RandomAlias(t *testing.T) {
	saver := &stubURLSaver{}

	handler := New(testLogger, saver)

	body := `{"url": "https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/url", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(contextWithUID(req.Context(), 42))

	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response Response
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, resp.StatusOK, response.Status)
	assert.Len(t, response.Alias, aliasLength) // random alias length
}

func TestSaveURL_Unauthenticated(t *testing.T) {
	saver := &stubURLSaver{}

	handler := New(testLogger, saver)

	body := `{"url": "https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/url", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No UID in context — simulates unauthenticated request.

	w := httptest.NewRecorder()
	handler(w, req)

	// Should return error (not internal, since no ErrorKey is set either).
	bodyBytes := w.Body.Bytes()
	assert.Contains(t, string(bodyBytes), "not logged")
}

func TestSaveURL_DuplicateURL(t *testing.T) {
	saver := &stubURLSaver{saveErr: storage.ErrURLExists}

	handler := New(testLogger, saver)

	body := `{"url": "https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/url", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(contextWithUID(req.Context(), 42))

	w := httptest.NewRecorder()
	handler(w, req)

	assert.Contains(t, w.Body.String(), "already exists")
}

func TestSaveURL_InvalidJSON(t *testing.T) {
	saver := &stubURLSaver{}

	handler := New(testLogger, saver)

	req := httptest.NewRequest(http.MethodPost, "/url", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(contextWithUID(req.Context(), 42))

	w := httptest.NewRecorder()
	handler(w, req)

	assert.Contains(t, w.Body.String(), "failed to decode")
}

func TestSaveURL_InvalidURL(t *testing.T) {
	saver := &stubURLSaver{}

	handler := New(testLogger, saver)

	body := `{"url": "not-a-url"}`
	req := httptest.NewRequest(http.MethodPost, "/url", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(contextWithUID(req.Context(), 42))

	w := httptest.NewRecorder()
	handler(w, req)

	assert.Contains(t, w.Body.String(), "not a valid URL")
}

func TestSaveURL_MissingURL(t *testing.T) {
	saver := &stubURLSaver{}

	handler := New(testLogger, saver)

	// Empty body with no URL field.
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/url", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(contextWithUID(req.Context(), 42))

	w := httptest.NewRecorder()
	handler(w, req)

	assert.Contains(t, w.Body.String(), "required")
}

// Test that the handler can receive and parse a JSON body correctly.
// Use json.NewDecoder for more control in test assertions.
func TestSaveURL_RequestBodyFormat(t *testing.T) {
	saver := &stubURLSaver{}

	handler := New(testLogger, saver)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(Request{URL: "https://example.com", Alias: "myalias"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/url", &buf)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(contextWithUID(req.Context(), 42))

	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://example.com", saver.savedURL)
	assert.Equal(t, "myalias", saver.savedAlias)
}

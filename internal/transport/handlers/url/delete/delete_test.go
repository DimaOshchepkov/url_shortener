package delete

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DimaOshchepkov/url_shortener/internal/lib/logger/handlers/slogdiscard"
	"github.com/DimaOshchepkov/url_shortener/internal/storage"
	get "github.com/DimaOshchepkov/url_shortener/internal/transport/middleware/context"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

var testLogger = slogdiscard.NewDiscardLogger()

// stubURLDeleter implements URLDeleter for testing.
type stubURLDeleter struct {
	deleteErr error
	deleted   string // alias that was deleted
}

func (s *stubURLDeleter) DeleteURL(_ context.Context, alias string) error {
	s.deleted = alias
	return s.deleteErr
}

// contextWithAdmin returns a context with IsAdmin=true.
func contextWithAdmin(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, get.IsAdminKey, true)
	ctx = context.WithValue(ctx, get.UidKey, uint64(1))
	return ctx
}

// contextWithNonAdmin returns a context with IsAdmin=false.
func contextWithNonAdmin(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, get.IsAdminKey, false)
	ctx = context.WithValue(ctx, get.UidKey, uint64(1))
	return ctx
}

// putAliasInContext sets the chi route context so chi.URLParam can extract the alias.
func putAliasInContext(ctx context.Context, alias string) context.Context {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("alias", alias)
	return context.WithValue(ctx, chi.RouteCtxKey, rctx)
}

func TestDeleteURL_Success(t *testing.T) {
	deleter := &stubURLDeleter{}
	handler := New(testLogger, deleter)

	req := httptest.NewRequest(http.MethodDelete, "/url/abc123", nil)
	ctx := contextWithAdmin(req.Context())
	ctx = putAliasInContext(ctx, "abc123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "abc123", deleter.deleted)
}

func TestDeleteURL_NotAdmin(t *testing.T) {
	deleter := &stubURLDeleter{}
	handler := New(testLogger, deleter)

	req := httptest.NewRequest(http.MethodDelete, "/url/abc123", nil)
	ctx := contextWithNonAdmin(req.Context())
	ctx = putAliasInContext(ctx, "abc123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler(w, req)

	assert.Contains(t, w.Body.String(), "not admin")
	assert.Empty(t, deleter.deleted)
}

func TestDeleteURL_NotFound(t *testing.T) {
	deleter := &stubURLDeleter{deleteErr: storage.ErrAliasNotFound}
	handler := New(testLogger, deleter)

	req := httptest.NewRequest(http.MethodDelete, "/url/abc123", nil)
	ctx := contextWithAdmin(req.Context())
	ctx = putAliasInContext(ctx, "abc123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler(w, req)

	assert.Contains(t, w.Body.String(), "not found")
}

func TestDeleteURL_EmptyAlias(t *testing.T) {
	deleter := &stubURLDeleter{}
	handler := New(testLogger, deleter)

	req := httptest.NewRequest(http.MethodDelete, "/url/", nil)
	ctx := contextWithAdmin(req.Context())
	ctx = putAliasInContext(ctx, "") // empty alias
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler(w, req)

	assert.Contains(t, w.Body.String(), "invalid request")
}

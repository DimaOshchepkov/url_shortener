package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DimaOshchepkov/url_shortener/internal/lib/logger/handlers/slogdiscard"
	get "github.com/DimaOshchepkov/url_shortener/internal/transport/middleware/context"
	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testLogger = slogdiscard.NewDiscardLogger()

const testSecret = "test-secret-key"

// generateToken creates a valid JWT with the given claims for testing.
func generateToken(secret string, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		panic(err)
	}
	return signed
}

func TestAuth_ValidToken(t *testing.T) {
	token := generateToken(testSecret, jwt.MapClaims{
		"uid":    float64(42),
		"app_id": float64(7),
	})

	var capturedUID, capturedAppID uint64
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := get.UIDFromContext(r.Context())
		assert.True(t, ok)
		capturedUID = uid

		appID, ok := get.APPIDFromContext(r.Context())
		assert.True(t, ok)
		capturedAppID = appID

		w.WriteHeader(http.StatusOK)
	})

	mw := New(testLogger, testSecret)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, uint64(42), capturedUID)
	assert.Equal(t, uint64(7), capturedAppID)
}

func TestAuth_NoToken(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	mw := New(testLogger, testSecret)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_InvalidToken(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	mw := New(testLogger, testSecret)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")

	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_WrongSecret(t *testing.T) {
	// Token signed with a different secret.
	token := generateToken("wrong-secret", jwt.MapClaims{
		"uid":    float64(1),
		"app_id": float64(1),
	})

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	mw := New(testLogger, testSecret)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_MissingClaims(t *testing.T) {
	// Token without required claims.
	token := generateToken(testSecret, jwt.MapClaims{
		"some": "data",
	})

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	mw := New(testLogger, testSecret)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_WrongSigningMethod(t *testing.T) {
	// Use a non-HMAC signing method.
	tokenStr, err := jwt.New(jwt.SigningMethodNone).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})

	mw := New(testLogger, testSecret)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ExtractBearerToken_Valid(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer mytoken123")

	token := extractBearerToken(req)
	assert.Equal(t, "mytoken123", token)
}

func TestAuth_ExtractBearerToken_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	token := extractBearerToken(req)
	assert.Empty(t, token)
}

func TestAuth_ExtractBearerToken_NoPrefix(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "mytoken123") // no "Bearer " prefix

	token := extractBearerToken(req)
	assert.Empty(t, token)
}

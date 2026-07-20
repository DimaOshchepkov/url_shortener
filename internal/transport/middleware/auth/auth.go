package auth

import (
	"context"
	"fmt"
	"github.com/DimaOshchepkov/url_shortener/internal/lib/logger/sl"
	get "github.com/DimaOshchepkov/url_shortener/internal/transport/middleware/context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt"
)

func New(log *slog.Logger, appSecret string) func(next http.Handler) http.Handler {
	const op = "middleware.auth.New"
	log = log.With(slog.String("op", op))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearerToken(r)
			if tokenStr == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			tokenParsed, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(appSecret), nil
			})
			if err != nil {
				log.Warn("failed to parse token", sl.Err(err))
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			claims, ok := tokenParsed.Claims.(jwt.MapClaims)
			if !ok {
				log.Warn("invalid token claims")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			uid, ok := claims["uid"].(float64)
			if !ok {
				log.Warn("invalid uid in token")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			appID, ok := claims["app_id"].(float64)
			if !ok {
				log.Warn("invalid app_id in token")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			log.Info("user authorized", slog.Any("uid", uid), slog.Any("app_id", appID))
			ctx := context.WithValue(r.Context(), get.UidKey, uint64(uid))
			ctx = context.WithValue(ctx, get.AppIDKey, uint64(appID))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	splitToken := strings.Split(authHeader, "Bearer ")
	if len(splitToken) != 2 {
		return ""
	}
	return splitToken[1]
}

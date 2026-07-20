// Package health provides health check HTTP handler.
package health

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/render"
)

// CacheInfo provides cache statistics for the health endpoint.
type CacheInfo interface {
	HitRate() float64
	Len() int
}

// @Summary		Проверка здоровья сервиса
// @Description	Возвращает статус сервиса и статистику кеша.
// @Tags			system
// @Produce		json
// @Success		200	{object}	map[string]any
// @Router			/health [get]
func New(_ *slog.Logger, cache CacheInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, map[string]any{
			"status":   "ok",
			"cache":    "enabled",
			"hit_rate": cache.HitRate(),
			"size":     cache.Len(),
		})
	}
}

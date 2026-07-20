package redirect

import (
	"context"
	"errors"
	"strings"
	"time"

	resp "github.com/DimaOshchepkov/url_shortener/internal/lib/api/response"
	"github.com/DimaOshchepkov/url_shortener/internal/lib/logger/sl"
	"github.com/DimaOshchepkov/url_shortener/internal/lib/metrics"
	"github.com/DimaOshchepkov/url_shortener/internal/storage"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

//go:generate go run github.com/vektra/mockery/v2@v2.42.2 --name=URLRedirector
type URLRedirector interface {
	GetURL(ctx context.Context, alias string) (string, error)
	IncrementClicks(ctx context.Context, alias string) error
}

// @Summary		Редирект по короткой ссылке
// @Description	Перенаправляет на оригинальный URL по alias. Счётчик кликов инкрементируется асинхронно.
// @Tags			urls
// @Param			alias	path	string	true	"Короткий alias"
// @Success		302
// @Failure		404	{object}	response.Response
// @Router			/{alias} [get]
func New(log *slog.Logger, urlRedirector URLRedirector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.redirect.New"

		start := time.Now()

		// add to log op and reqID
		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		// get alias from url
		alias := chi.URLParam(r, "alias")
		if alias == "" {
			log.Warn("alias is empty")
			metrics.RedirectRequestsTotal.WithLabelValues("error").Inc()
			metrics.RedirectDurationSeconds.Observe(time.Since(start).Seconds())
			render.JSON(w, r, resp.Error("invalid request"))
			return
		}
		log.Info("alias was get from url", slog.String("alias", alias))

		// get resURL by alias
		resURL, err := urlRedirector.GetURL(r.Context(), alias)
		if err != nil {
			if errors.Is(err, storage.ErrURLNotFound) {
				log.Warn("wrong alias", slog.String("alias", alias))
				metrics.RedirectRequestsTotal.WithLabelValues("not_found").Inc()
				metrics.RedirectDurationSeconds.Observe(time.Since(start).Seconds())
				render.JSON(w, r, resp.Error("wrong alias"))
				return
			}
			log.Error("failed to get url", sl.Err(err))
			metrics.RedirectRequestsTotal.WithLabelValues("error").Inc()
			metrics.RedirectDurationSeconds.Observe(time.Since(start).Seconds())
			render.JSON(w, r, resp.Error("internal error"))
			return
		}
		log.Info("got url", slog.String("url", resURL))

		// validate URL scheme before redirect
		if !strings.HasPrefix(resURL, "http://") && !strings.HasPrefix(resURL, "https://") {
			log.Warn("invalid URL scheme for redirect", slog.String("url", resURL))
			metrics.RedirectRequestsTotal.WithLabelValues("error").Inc()
			metrics.RedirectDurationSeconds.Observe(time.Since(start).Seconds())
			render.JSON(w, r, resp.Error("invalid redirect URL"))
			return
		}

		// redirect to resURL
		http.Redirect(w, r, resURL, http.StatusFound)

		// Increment click counter (best-effort, non-fatal).
		// This runs after the redirect response is sent because:
		//   - The redirect is the critical path; the user gets their 302 ASAP.
		//   - A click-increment failure should not block or fail the redirect.
		//   - Click counts are per-link business data stored in PostgreSQL,
		//     separate from operational Prometheus metrics.
		if err := urlRedirector.IncrementClicks(r.Context(), alias); err != nil {
			log.Error("failed to increment clicks", sl.Err(err))
		}

		metrics.RedirectRequestsTotal.WithLabelValues("success").Inc()
		metrics.RedirectDurationSeconds.Observe(time.Since(start).Seconds())
	}
}

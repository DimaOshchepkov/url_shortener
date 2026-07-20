package delete

import (
	"context"
	"errors"
	resp "github.com/DimaOshchepkov/url_shortener/internal/lib/api/response"
	"github.com/DimaOshchepkov/url_shortener/internal/lib/logger/sl"
	"github.com/DimaOshchepkov/url_shortener/internal/storage"
	get "github.com/DimaOshchepkov/url_shortener/internal/transport/middleware/context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

//go:generate go run github.com/vektra/mockery/v2@v2.42.2 --name=URLDeleter
type URLDeleter interface {
	DeleteURL(ctx context.Context, alias string) error
}

// @Summary		Удалить короткую ссылку
// @Description	Удаляет alias из базы. Требует права администратора.
// @Tags			urls
// @Produce		json
// @Param			alias	path	string	true	"Короткий alias"
// @Success		200		{object}	response.Response
// @Failure		401		{object}	response.Response
// @Failure		403		{object}	response.Response
// @Failure		404		{object}	response.Response
// @Security		BearerAuth
// @Router			/url/{alias} [delete]
func New(log *slog.Logger, urlDeleter URLDeleter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.delete.New"

		// add to log op and reqID
		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		IsAdmin, ok := get.IsAdminFromContext(r.Context())
		if !ok {
			if err, ok := get.ErrorFromContext(r.Context()); ok {
				log.Error("failed to get IsAdminBool", sl.Err(err))
				render.JSON(w, r, resp.Error("Internal error"))
				return
			}
			log.Info("user without logging")
			render.JSON(w, r, resp.Error("you are not logged into your account"))
			return
		}
		if !IsAdmin {
			log.Info("user aren't admin")
			render.JSON(w, r, resp.Error("you are not admin to delete this"))
			return
		}

		// get alias from url
		alias := chi.URLParam(r, "alias")
		if alias == "" {
			log.Warn("alias is empty")
			render.JSON(w, r, resp.Error("invalid request"))
			return
		}
		log.Info("alias was get from url", slog.String("alias", alias))

		// delete URL by alias
		err := urlDeleter.DeleteURL(r.Context(), alias)
		if err != nil {
			if errors.Is(err, storage.ErrAliasNotFound) {
				log.Warn("url by alias was not found", slog.String("alias", alias))
				render.JSON(w, r, resp.Error("url by alias was not found"))
				return
			}
			log.Error("failed to delete url", sl.Err(err))
			render.JSON(w, r, resp.Error("internal error"))
			return
		}
		log.Info("delete alias", slog.String("alias", alias))

		// respone OK
		render.JSON(w, r, resp.OK())
	}
}

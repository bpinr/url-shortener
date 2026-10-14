package update

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"

	resp "github.com/bpinr/url-shortener/internal/lib/api/response"
	"github.com/bpinr/url-shortener/internal/lib/logger/sl"
	"github.com/bpinr/url-shortener/internal/service"
)

type Request struct {
	URL string `json:"url"`
}

type Response struct {
	resp.Response
	Alias string `json:"alias,omitempty"`
	URL   string `json:"url,omitempty"`
}

type URLUpdater interface {
	Update(ctx context.Context, alias, newURL string) error
}

func New(log *slog.Logger, urlUpdater URLUpdater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const fn = "handlers.url.update.New"

		log := log.With(
			slog.String("fn", fn),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		alias := chi.URLParam(r, "alias")
		if alias == "" {
			log.Info("empty alias")
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error("invalid request"))
			return
		}

		var req Request

		if err := render.DecodeJSON(r.Body, &req); err != nil {
			log.Error("failed to decode request body", sl.Err(err))
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error("invalid request body"))
			return
		}

		err := urlUpdater.Update(r.Context(), alias, req.URL)
		switch {
		case errors.Is(err, service.ErrInvalidURL):
			log.Info("invalid url", slog.String("url", req.URL))
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error("invalid url"))
			return
		case errors.Is(err, service.ErrURLNotFound):
			log.Info("url not found", slog.String("alias", alias))
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, resp.Error("not found"))
			return
		case err != nil:
			log.Error("failed to update url", slog.String("alias", alias), sl.Err(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("internal error"))
			return
		}

		log.Info("url updated", slog.String("alias", alias))

		render.Status(r, http.StatusOK)
		render.JSON(w, r, Response{
			Response: resp.OK(),
			Alias:    alias,
			URL:      req.URL,
		})
	}
}

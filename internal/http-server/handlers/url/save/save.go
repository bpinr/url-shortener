package save

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"

	resp "github.com/bpinr/url-shortener/internal/lib/api/response"
	"github.com/bpinr/url-shortener/internal/lib/logger/sl"
	"github.com/bpinr/url-shortener/internal/service"
)

type Request struct {
	URL   string `json:"url"`
	Alias string `json:"alias,omitempty"`
}

type Response struct {
	resp.Response
	Alias string `json:"alias,omitempty"`
}

type URLSaver interface {
	Save(ctx context.Context, rawURL, alias string) (string, error)
}

func New(log *slog.Logger, urlSaver URLSaver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const fn = "handlers.url.save.New"

		log := log.With(
			slog.String("fn", fn),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req Request

		if err := render.DecodeJSON(r.Body, &req); err != nil {
			log.Error("failed to decode request body", sl.Err(err))
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error("invalid request body"))
			return
		}

		alias, err := urlSaver.Save(r.Context(), req.URL, req.Alias)
		switch {
		case errors.Is(err, service.ErrInvalidURL):
			log.Info("invalid url", slog.String("url", req.URL))
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error("invalid url"))
			return
		case errors.Is(err, service.ErrInvalidAlias):
			log.Info("invalid alias", slog.String("alias", req.Alias))
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error("invalid alias"))
			return
		case errors.Is(err, service.ErrAliasExists):
			log.Info("alias already exists", slog.String("alias", req.Alias))
			render.Status(r, http.StatusConflict)
			render.JSON(w, r, resp.Error("alias already exists"))
			return
		case err != nil:
			log.Error("failed to save url", sl.Err(err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("internal error"))
			return
		}

		log.Info("url saved", slog.String("alias", alias))

		render.Status(r, http.StatusCreated)
		render.JSON(w, r, Response{
			Response: resp.OK(),
			Alias:    alias,
		})
	}
}

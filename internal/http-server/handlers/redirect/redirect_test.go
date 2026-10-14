package redirect_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/bpinr/url-shortener/internal/http-server/handlers/redirect"
	resp "github.com/bpinr/url-shortener/internal/lib/api/response"
	"github.com/bpinr/url-shortener/internal/lib/logger/handlers/slogdiscard"
	"github.com/bpinr/url-shortener/internal/service"
)

type getterStub struct {
	url string
	err error

	gotAlias string
}

func (s *getterStub) Get(_ context.Context, alias string) (string, error) {
	s.gotAlias = alias
	return s.url, s.err
}

func doRedirect(stub *getterStub, alias string) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	r.Get("/{alias}", redirect.New(slogdiscard.NewDiscardLogger(), stub))

	req := httptest.NewRequest(http.MethodGet, "/"+alias, nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	return rr
}

func decodeError(t *testing.T, rr *httptest.ResponseRecorder) resp.Response {
	t.Helper()

	var body resp.Response
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), "body: %s", rr.Body.String())

	return body
}

func TestRedirect_Success(t *testing.T) {
	stub := &getterStub{url: "https://go.dev"}

	rr := doRedirect(stub, "golang")

	require.Equal(t, http.StatusFound, rr.Code)
	require.Equal(t, "https://go.dev", rr.Header().Get("Location"))
	require.Equal(t, "golang", stub.gotAlias)
}

func TestRedirect_NotFound(t *testing.T) {
	stub := &getterStub{err: service.ErrURLNotFound}

	rr := doRedirect(stub, "missing")

	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Equal(t, "not found", decodeError(t, rr).Error)
}

func TestRedirect_InternalError(t *testing.T) {
	stub := &getterStub{err: errors.New("connection refused")}

	rr := doRedirect(stub, "golang")

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, "internal error", decodeError(t, rr).Error)
}

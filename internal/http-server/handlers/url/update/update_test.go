package update_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/bpinr/url-shortener/internal/http-server/handlers/url/update"
	"github.com/bpinr/url-shortener/internal/lib/logger/handlers/slogdiscard"
	"github.com/bpinr/url-shortener/internal/service"
)

type updaterStub struct {
	err error

	called   bool
	gotAlias string
	gotURL   string
}

func (s *updaterStub) Update(_ context.Context, alias, newURL string) error {
	s.called = true
	s.gotAlias = alias
	s.gotURL = newURL
	return s.err
}

func doUpdate(t *testing.T, stub *updaterStub, alias, body string) (*httptest.ResponseRecorder, update.Response) {
	t.Helper()

	r := chi.NewRouter()
	r.Put("/url/{alias}", update.New(slogdiscard.NewDiscardLogger(), stub))

	req := httptest.NewRequest(http.MethodPut, "/url/"+alias, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	var resp update.Response
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp), "body: %s", rr.Body.String())

	return rr, resp
}

func TestUpdate_Success(t *testing.T) {
	stub := &updaterStub{}

	rr, resp := doUpdate(t, stub, "golang", `{"url": "https://go.dev/doc"}`)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "OK", resp.Status)
	require.Equal(t, "golang", resp.Alias)
	require.Equal(t, "https://go.dev/doc", resp.URL)
	require.Equal(t, "golang", stub.gotAlias)
	require.Equal(t, "https://go.dev/doc", stub.gotURL)
}

func TestUpdate_BadRequest(t *testing.T) {
	stub := &updaterStub{}

	rr, resp := doUpdate(t, stub, "golang", `{"url": "https://go.dev",`)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, "invalid request body", resp.Error)
	require.False(t, stub.called, "service must not be called on broken JSON")
}

func TestUpdate_InvalidURL(t *testing.T) {
	stub := &updaterStub{err: service.ErrInvalidURL}

	rr, resp := doUpdate(t, stub, "golang", `{"url": "not a url"}`)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, "invalid url", resp.Error)
}

func TestUpdate_NotFound(t *testing.T) {
	stub := &updaterStub{err: service.ErrURLNotFound}

	rr, resp := doUpdate(t, stub, "missing", `{"url": "https://go.dev"}`)

	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Equal(t, "not found", resp.Error)
}

func TestUpdate_InternalError(t *testing.T) {
	stub := &updaterStub{err: errors.New("connection refused")}

	rr, resp := doUpdate(t, stub, "golang", `{"url": "https://go.dev"}`)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, "internal error", resp.Error)
}

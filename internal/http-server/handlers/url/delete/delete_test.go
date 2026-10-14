package delete_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/bpinr/url-shortener/internal/http-server/handlers/url/delete"
	resp "github.com/bpinr/url-shortener/internal/lib/api/response"
	"github.com/bpinr/url-shortener/internal/lib/logger/handlers/slogdiscard"
	"github.com/bpinr/url-shortener/internal/service"
)

type deleterStub struct {
	err error

	gotAlias string
}

func (s *deleterStub) Delete(_ context.Context, alias string) error {
	s.gotAlias = alias
	return s.err
}

func doDelete(t *testing.T, stub *deleterStub, alias string) (*httptest.ResponseRecorder, resp.Response) {
	t.Helper()

	r := chi.NewRouter()
	r.Delete("/url/{alias}", delete.New(slogdiscard.NewDiscardLogger(), stub))

	req := httptest.NewRequest(http.MethodDelete, "/url/"+alias, nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	var body resp.Response
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), "body: %s", rr.Body.String())

	return rr, body
}

func TestDelete_Success(t *testing.T) {
	stub := &deleterStub{}

	rr, body := doDelete(t, stub, "golang")

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "OK", body.Status)
	require.Equal(t, "golang", stub.gotAlias)
}

func TestDelete_NotFound(t *testing.T) {
	stub := &deleterStub{err: service.ErrURLNotFound}

	rr, body := doDelete(t, stub, "missing")

	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Equal(t, "not found", body.Error)
}

func TestDelete_InternalError(t *testing.T) {
	stub := &deleterStub{err: errors.New("connection refused")}

	rr, body := doDelete(t, stub, "golang")

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, "internal error", body.Error)
}

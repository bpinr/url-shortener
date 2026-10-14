package save_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bpinr/url-shortener/internal/http-server/handlers/url/save"
	"github.com/bpinr/url-shortener/internal/lib/logger/handlers/slogdiscard"
	"github.com/bpinr/url-shortener/internal/service"
)

type saverStub struct {
	alias string
	err   error

	called   bool
	gotURL   string
	gotAlias string
}

func (s *saverStub) Save(_ context.Context, rawURL, alias string) (string, error) {
	s.called = true
	s.gotURL = rawURL
	s.gotAlias = alias
	return s.alias, s.err
}

func doSave(t *testing.T, stub *saverStub, body string) (*httptest.ResponseRecorder, save.Response) {
	t.Helper()

	handler := save.New(slogdiscard.NewDiscardLogger(), stub)

	req := httptest.NewRequest(http.MethodPost, "/url", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	var resp save.Response
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp), "body: %s", rr.Body.String())

	return rr, resp
}

func TestSave_Success(t *testing.T) {
	stub := &saverStub{alias: "golang"}

	rr, resp := doSave(t, stub, `{"url": "https://go.dev", "alias": "golang"}`)

	require.Equal(t, http.StatusCreated, rr.Code)
	require.Equal(t, "OK", resp.Status)
	require.Equal(t, "golang", resp.Alias)
	require.Equal(t, "https://go.dev", stub.gotURL)
	require.Equal(t, "golang", stub.gotAlias)
}

func TestSave_SuccessGeneratedAlias(t *testing.T) {
	stub := &saverStub{alias: "aB3xZ9"}

	rr, resp := doSave(t, stub, `{"url": "https://go.dev"}`)

	require.Equal(t, http.StatusCreated, rr.Code)
	require.Equal(t, "aB3xZ9", resp.Alias)
	require.Empty(t, stub.gotAlias, "empty alias must be passed to service as is")
}

func TestSave_BadRequest(t *testing.T) {
	stub := &saverStub{}

	rr, resp := doSave(t, stub, `{"url": "https://go.dev`)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, "invalid request body", resp.Error)
	require.False(t, stub.called, "service must not be called on broken JSON")
}

func TestSave_InvalidURL(t *testing.T) {
	stub := &saverStub{err: service.ErrInvalidURL}

	rr, resp := doSave(t, stub, `{"url": "not a url"}`)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, "invalid url", resp.Error)
}

func TestSave_InvalidAlias(t *testing.T) {
	stub := &saverStub{err: service.ErrInvalidAlias}

	rr, resp := doSave(t, stub, `{"url": "https://go.dev", "alias": "bad alias!"}`)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, "invalid alias", resp.Error)
}

func TestSave_Conflict(t *testing.T) {
	stub := &saverStub{err: service.ErrAliasExists}

	rr, resp := doSave(t, stub, `{"url": "https://go.dev", "alias": "golang"}`)

	require.Equal(t, http.StatusConflict, rr.Code)
	require.Equal(t, "alias already exists", resp.Error)
}

func TestSave_InternalError(t *testing.T) {
	stub := &saverStub{err: errors.New("connection refused")}

	rr, resp := doSave(t, stub, `{"url": "https://go.dev", "alias": "golang"}`)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	require.Equal(t, "internal error", resp.Error)
}

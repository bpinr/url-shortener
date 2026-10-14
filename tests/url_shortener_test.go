package tests

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gavv/httpexpect/v2"
	"github.com/joho/godotenv"

	"github.com/bpinr/url-shortener/internal/http-server/handlers/url/save"
	"github.com/bpinr/url-shortener/internal/http-server/handlers/url/update"
	"github.com/bpinr/url-shortener/internal/lib/random"
)

const (
	host    = "localhost:8082"
	baseURL = "http://" + host
	envPath = "../.env"
)

type credentials struct {
	user     string
	password string
}

func loadCredentials(t *testing.T) credentials {
	t.Helper()

	env, err := godotenv.Read(envPath)
	if err != nil {
		t.Fatalf("cannot read %s (run: cp .env.example .env): %v", envPath, err)
	}

	c := credentials{user: env["AUTH_USER"], password: env["AUTH_PASSWORD"]}
	if c.user == "" || c.password == "" {
		t.Fatalf("AUTH_USER and AUTH_PASSWORD must be set in %s", envPath)
	}

	return c
}

func newExpect(t *testing.T) *httpexpect.Expect {
	t.Helper()

	conn, err := net.DialTimeout("tcp", host, time.Second)
	if err != nil {
		t.Skipf("service is not running on %s (run: docker compose up): %v", host, err)
	}
	_ = conn.Close()

	return httpexpect.WithConfig(httpexpect.Config{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Reporter: httpexpect.NewAssertReporter(t),
		Printers: []httpexpect.Printer{httpexpect.NewDebugPrinter(t, true)},
	})
}

func createURL(t *testing.T, e *httpexpect.Expect, creds credentials, target string) string {
	t.Helper()

	alias := random.NewRandomString(10)

	e.POST("/url").
		WithBasicAuth(creds.user, creds.password).
		WithJSON(save.Request{URL: target, Alias: alias}).
		Expect().
		Status(http.StatusCreated)

	t.Cleanup(func() {
		e.DELETE("/url/"+alias).WithBasicAuth(creds.user, creds.password).Expect()
	})

	return alias
}

func TestE2E_Save_Success(t *testing.T) {
	e := newExpect(t)
	creds := loadCredentials(t)

	alias := createURL(t, e, creds, "https://go.dev")

	e.GET("/" + alias).
		Expect().
		Status(http.StatusFound).
		Header("Location").IsEqual("https://go.dev")
}

func TestE2E_Save_GeneratedAlias(t *testing.T) {
	e := newExpect(t)
	creds := loadCredentials(t)

	alias := e.POST("/url").
		WithBasicAuth(creds.user, creds.password).
		WithJSON(save.Request{URL: "https://go.dev"}).
		Expect().
		Status(http.StatusCreated).
		JSON().Object().
		Value("alias").String().NotEmpty().Raw()

	t.Cleanup(func() {
		e.DELETE("/url/"+alias).WithBasicAuth(creds.user, creds.password).Expect()
	})

	e.GET("/" + alias).Expect().Status(http.StatusFound)
}

func TestE2E_Save_BadRequest(t *testing.T) {
	e := newExpect(t)
	creds := loadCredentials(t)

	e.POST("/url").
		WithBasicAuth(creds.user, creds.password).
		WithHeader("Content-Type", "application/json").
		WithText(`{"url": "https://go.dev`).
		Expect().
		Status(http.StatusBadRequest).
		JSON().Object().
		Value("error").String().IsEqual("invalid request body")
}

func TestE2E_Save_InvalidURL(t *testing.T) {
	e := newExpect(t)
	creds := loadCredentials(t)

	e.POST("/url").
		WithBasicAuth(creds.user, creds.password).
		WithJSON(save.Request{URL: "not a url", Alias: random.NewRandomString(10)}).
		Expect().
		Status(http.StatusBadRequest).
		JSON().Object().
		Value("error").String().IsEqual("invalid url")
}

func TestE2E_Save_Conflict(t *testing.T) {
	e := newExpect(t)
	creds := loadCredentials(t)

	alias := createURL(t, e, creds, "https://go.dev")

	e.POST("/url").
		WithBasicAuth(creds.user, creds.password).
		WithJSON(save.Request{URL: "https://example.com", Alias: alias}).
		Expect().
		Status(http.StatusConflict).
		JSON().Object().
		Value("error").String().IsEqual("alias already exists")
}

func TestE2E_Save_Unauthorized(t *testing.T) {
	e := newExpect(t)
	creds := loadCredentials(t)

	e.POST("/url").
		WithJSON(save.Request{URL: "https://go.dev"}).
		Expect().
		Status(http.StatusUnauthorized)

	e.POST("/url").
		WithBasicAuth(creds.user, creds.password+"-wrong").
		WithJSON(save.Request{URL: "https://go.dev"}).
		Expect().
		Status(http.StatusUnauthorized)
}

func TestE2E_Redirect_NotFound(t *testing.T) {
	e := newExpect(t)

	e.GET("/" + random.NewRandomString(12)).
		Expect().
		Status(http.StatusNotFound).
		JSON().Object().
		Value("error").String().IsEqual("not found")
}

func TestE2E_Update_Success(t *testing.T) {
	e := newExpect(t)
	creds := loadCredentials(t)

	alias := createURL(t, e, creds, "https://go.dev")

	e.PUT("/url/"+alias).
		WithBasicAuth(creds.user, creds.password).
		WithJSON(update.Request{URL: "https://pkg.go.dev"}).
		Expect().
		Status(http.StatusOK)

	e.GET("/" + alias).
		Expect().
		Status(http.StatusFound).
		Header("Location").IsEqual("https://pkg.go.dev")
}

func TestE2E_Update_NotFound(t *testing.T) {
	e := newExpect(t)
	creds := loadCredentials(t)

	e.PUT("/url/"+random.NewRandomString(12)).
		WithBasicAuth(creds.user, creds.password).
		WithJSON(update.Request{URL: "https://go.dev"}).
		Expect().
		Status(http.StatusNotFound)
}

func TestE2E_Update_BadRequest(t *testing.T) {
	e := newExpect(t)
	creds := loadCredentials(t)

	alias := createURL(t, e, creds, "https://go.dev")

	e.PUT("/url/"+alias).
		WithBasicAuth(creds.user, creds.password).
		WithHeader("Content-Type", "application/json").
		WithText(`{"url": `).
		Expect().
		Status(http.StatusBadRequest)
}

func TestE2E_Update_Unauthorized(t *testing.T) {
	e := newExpect(t)

	e.PUT("/url/whatever").
		WithJSON(update.Request{URL: "https://go.dev"}).
		Expect().
		Status(http.StatusUnauthorized)
}

func TestE2E_Delete_Success(t *testing.T) {
	e := newExpect(t)
	creds := loadCredentials(t)

	alias := createURL(t, e, creds, "https://go.dev")

	e.DELETE("/url/"+alias).
		WithBasicAuth(creds.user, creds.password).
		Expect().
		Status(http.StatusOK)

	e.GET("/" + alias).Expect().Status(http.StatusNotFound)
}

func TestE2E_Delete_NotFound(t *testing.T) {
	e := newExpect(t)
	creds := loadCredentials(t)

	e.DELETE("/url/"+random.NewRandomString(12)).
		WithBasicAuth(creds.user, creds.password).
		Expect().
		Status(http.StatusNotFound)
}

func TestE2E_Delete_Unauthorized(t *testing.T) {
	e := newExpect(t)

	e.DELETE("/url/whatever").
		Expect().
		Status(http.StatusUnauthorized)
}

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bpinr/url-shortener/internal/lib/logger/handlers/slogdiscard"
	"github.com/bpinr/url-shortener/internal/storage"
)

type fakeStorage struct {
	urls     map[string]string
	err      error
	saveCall int
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{urls: map[string]string{}}
}

func (f *fakeStorage) SaveURL(_ context.Context, urlToSave, alias string) (int64, error) {
	f.saveCall++
	if f.err != nil {
		return 0, f.err
	}
	if _, ok := f.urls[alias]; ok {
		return 0, storage.ErrAliasExists
	}
	f.urls[alias] = urlToSave
	return int64(len(f.urls)), nil
}

func (f *fakeStorage) GetURL(_ context.Context, alias string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	u, ok := f.urls[alias]
	if !ok {
		return "", storage.ErrURLNotFound
	}
	return u, nil
}

func (f *fakeStorage) UpdateURL(_ context.Context, alias, newURL string) error {
	if f.err != nil {
		return f.err
	}
	if _, ok := f.urls[alias]; !ok {
		return storage.ErrURLNotFound
	}
	f.urls[alias] = newURL
	return nil
}

func (f *fakeStorage) DeleteURL(_ context.Context, alias string) error {
	if f.err != nil {
		return f.err
	}
	if _, ok := f.urls[alias]; !ok {
		return storage.ErrURLNotFound
	}
	delete(f.urls, alias)
	return nil
}

func newTestService(store URLStorage) *URLService {
	return New(slogdiscard.NewDiscardLogger(), store, 6)
}

var errDB = errors.New("connection refused")

func TestService_Save_WithAlias(t *testing.T) {
	store := newFakeStorage()
	svc := newTestService(store)

	alias, err := svc.Save(context.Background(), "https://go.dev", "golang")

	require.NoError(t, err)
	require.Equal(t, "golang", alias)
	require.Equal(t, "https://go.dev", store.urls["golang"])
}

func TestService_Save_GeneratesAlias(t *testing.T) {
	svc := newTestService(newFakeStorage())

	alias, err := svc.Save(context.Background(), "https://go.dev", "")

	require.NoError(t, err)
	require.Len(t, alias, 6)
}

func TestService_Save_RetriesOnGeneratedCollision(t *testing.T) {
	store := newFakeStorage()
	store.urls["taken"] = "https://example.com"

	svc := newTestService(store)
	generated := []string{"taken", "free"}
	svc.newAlias = func(int) string {
		a := generated[0]
		generated = generated[1:]
		return a
	}

	alias, err := svc.Save(context.Background(), "https://go.dev", "")

	require.NoError(t, err)
	require.Equal(t, "free", alias)
	require.Equal(t, 2, store.saveCall)
}

func TestService_Save_GiveUpAfterMaxAttempts(t *testing.T) {
	store := newFakeStorage()
	store.urls["taken"] = "https://example.com"

	svc := newTestService(store)
	svc.newAlias = func(int) string { return "taken" }

	_, err := svc.Save(context.Background(), "https://go.dev", "")

	require.Error(t, err)
	require.Equal(t, maxGenerateAttempts, store.saveCall)
}

func TestService_Save_AliasExists(t *testing.T) {
	store := newFakeStorage()
	store.urls["golang"] = "https://example.com"

	_, err := newTestService(store).Save(context.Background(), "https://go.dev", "golang")

	require.ErrorIs(t, err, ErrAliasExists)
}

func TestService_Save_InvalidURL(t *testing.T) {
	for _, raw := range []string{"", "not a url", "go.dev", "ftp://go.dev", "https://"} {
		t.Run(raw, func(t *testing.T) {
			store := newFakeStorage()

			_, err := newTestService(store).Save(context.Background(), raw, "alias")

			require.ErrorIs(t, err, ErrInvalidURL)
			require.Zero(t, store.saveCall, "storage must not be called for invalid input")
		})
	}
}

func TestService_Save_InvalidAlias(t *testing.T) {
	store := newFakeStorage()

	_, err := newTestService(store).Save(context.Background(), "https://go.dev", "bad alias/!")

	require.ErrorIs(t, err, ErrInvalidAlias)
	require.Zero(t, store.saveCall)
}

func TestService_Save_StorageError(t *testing.T) {
	store := newFakeStorage()
	store.err = errDB

	_, err := newTestService(store).Save(context.Background(), "https://go.dev", "golang")

	require.ErrorIs(t, err, errDB)
}

func TestService_Get_Success(t *testing.T) {
	store := newFakeStorage()
	store.urls["golang"] = "https://go.dev"

	u, err := newTestService(store).Get(context.Background(), "golang")

	require.NoError(t, err)
	require.Equal(t, "https://go.dev", u)
}

func TestService_Get_NotFound(t *testing.T) {
	_, err := newTestService(newFakeStorage()).Get(context.Background(), "missing")

	require.ErrorIs(t, err, ErrURLNotFound)
}

func TestService_Update_Success(t *testing.T) {
	store := newFakeStorage()
	store.urls["golang"] = "https://example.com"

	err := newTestService(store).Update(context.Background(), "golang", "https://go.dev")

	require.NoError(t, err)
	require.Equal(t, "https://go.dev", store.urls["golang"])
}

func TestService_Update_NotFound(t *testing.T) {
	err := newTestService(newFakeStorage()).Update(context.Background(), "missing", "https://go.dev")

	require.ErrorIs(t, err, ErrURLNotFound)
}

func TestService_Update_InvalidURL(t *testing.T) {
	store := newFakeStorage()
	store.urls["golang"] = "https://example.com"

	err := newTestService(store).Update(context.Background(), "golang", "not a url")

	require.ErrorIs(t, err, ErrInvalidURL)
	require.Equal(t, "https://example.com", store.urls["golang"])
}

func TestService_Delete_Success(t *testing.T) {
	store := newFakeStorage()
	store.urls["golang"] = "https://go.dev"

	err := newTestService(store).Delete(context.Background(), "golang")

	require.NoError(t, err)
	require.NotContains(t, store.urls, "golang")
}

func TestService_Delete_NotFound(t *testing.T) {
	err := newTestService(newFakeStorage()).Delete(context.Background(), "missing")

	require.ErrorIs(t, err, ErrURLNotFound)
}

func TestService_Delete_StorageError(t *testing.T) {
	store := newFakeStorage()
	store.err = errDB

	err := newTestService(store).Delete(context.Background(), "golang")

	require.ErrorIs(t, err, errDB)
	require.NotErrorIs(t, err, ErrURLNotFound)
}

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"

	"github.com/bpinr/url-shortener/internal/lib/random"
	"github.com/bpinr/url-shortener/internal/storage"
)

var (
	ErrInvalidURL   = errors.New("invalid url")
	ErrInvalidAlias = errors.New("invalid alias")
	ErrAliasExists  = errors.New("alias already exists")
	ErrURLNotFound  = errors.New("url not found")
)

const maxGenerateAttempts = 5

var aliasRegexp = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

type URLStorage interface {
	SaveURL(ctx context.Context, urlToSave, alias string) (int64, error)
	GetURL(ctx context.Context, alias string) (string, error)
	UpdateURL(ctx context.Context, alias, newURL string) error
	DeleteURL(ctx context.Context, alias string) error
}

type URLService struct {
	log         *slog.Logger
	store       URLStorage
	aliasLength int
	newAlias    func(size int) string
}

func New(log *slog.Logger, store URLStorage, aliasLength int) *URLService {
	return &URLService{
		log:         log,
		store:       store,
		aliasLength: aliasLength,
		newAlias:    random.NewRandomString,
	}
}

func (s *URLService) Save(ctx context.Context, rawURL, alias string) (string, error) {
	const fn = "service.URLService.Save"

	if err := validateURL(rawURL); err != nil {
		return "", err
	}

	if alias != "" {
		if !aliasRegexp.MatchString(alias) {
			return "", ErrInvalidAlias
		}

		if _, err := s.store.SaveURL(ctx, rawURL, alias); err != nil {
			if errors.Is(err, storage.ErrAliasExists) {
				return "", ErrAliasExists
			}

			return "", fmt.Errorf("%s: %w", fn, err)
		}

		return alias, nil
	}

	for range maxGenerateAttempts {
		generated := s.newAlias(s.aliasLength)

		_, err := s.store.SaveURL(ctx, rawURL, generated)
		if errors.Is(err, storage.ErrAliasExists) {
			s.log.Debug("generated alias collision, retrying", slog.String("alias", generated))
			continue
		}
		if err != nil {
			return "", fmt.Errorf("%s: %w", fn, err)
		}

		return generated, nil
	}

	return "", fmt.Errorf("%s: could not generate unique alias after %d attempts", fn, maxGenerateAttempts)
}

func (s *URLService) Get(ctx context.Context, alias string) (string, error) {
	const fn = "service.URLService.Get"

	u, err := s.store.GetURL(ctx, alias)
	if err != nil {
		if errors.Is(err, storage.ErrURLNotFound) {
			return "", ErrURLNotFound
		}

		return "", fmt.Errorf("%s: %w", fn, err)
	}

	return u, nil
}

func (s *URLService) Update(ctx context.Context, alias, newURL string) error {
	const fn = "service.URLService.Update"

	if err := validateURL(newURL); err != nil {
		return err
	}

	if err := s.store.UpdateURL(ctx, alias, newURL); err != nil {
		if errors.Is(err, storage.ErrURLNotFound) {
			return ErrURLNotFound
		}

		return fmt.Errorf("%s: %w", fn, err)
	}

	return nil
}

func (s *URLService) Delete(ctx context.Context, alias string) error {
	const fn = "service.URLService.Delete"

	if err := s.store.DeleteURL(ctx, alias); err != nil {
		if errors.Is(err, storage.ErrURLNotFound) {
			return ErrURLNotFound
		}

		return fmt.Errorf("%s: %w", fn, err)
	}

	return nil
}

func validateURL(raw string) error {
	if raw == "" {
		return ErrInvalidURL
	}

	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ErrInvalidURL
	}

	return nil
}

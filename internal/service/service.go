package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	e "pastebin/internal/errors"
	m "pastebin/internal/models"
	"pastebin/internal/repository"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	base62Alphabet  = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" // 62 symbols
	shortCodeLength = 8

	maxContentBytes = 100 * 1024
	maxExpiresIn    = 365 * 24 * 60 * 60 // paste ttl

	cacheTTL     = 5 * time.Minute
	cacheTimeout = 200 * time.Millisecond
)

type Cache interface {
	GetJSON(ctx context.Context, key string, dst any) (bool, error)
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
}

type Service interface {
	CreatePaste(ctx context.Context, req *CreatePasteInput) (*m.Paste, error)
	GetPaste(ctx context.Context, shortCode string) (*m.Paste, error)
}

type service struct {
	repo  repository.Repository
	cache Cache
}

func NewService(repo repository.Repository, cache Cache) Service {
	return &service{
		repo:  repo,
		cache: cache,
	}
}

func (s *service) CreatePaste(ctx context.Context, req *CreatePasteInput) (*m.Paste, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: request is required", e.ErrInvalidInput)
	}

	key, err := uuid.Parse(req.IdempotencyKey)
	if err != nil || key == uuid.Nil {
		return nil, fmt.Errorf("%w: Idempotency-Key must be a non-zero UUID", e.ErrInvalidInput)
	}

	if strings.TrimSpace(req.Content) == "" {
		return nil, fmt.Errorf("%w: content is empty", e.ErrInvalidInput)
	}

	if len(req.Content) > maxContentBytes {
		return nil, fmt.Errorf("%w: content exceeds 100 KiB", e.ErrInvalidInput)
	}

	if req.Visibility != "public" {
		return nil, fmt.Errorf("%w: only public pastes are supported", e.ErrInvalidInput)
	}

	if req.ExpiresIn <= 0 || req.ExpiresIn > maxExpiresIn {
		return nil, fmt.Errorf("%w: expires_in must be between 1 and %d seconds", e.ErrInvalidInput, maxExpiresIn)
	}

	requestHash, err := hashCreatePasteInput(req)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	paste := &m.Paste{
		ID:         uuid.New(),
		Content:    req.Content,
		Visibility: req.Visibility,
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Duration(req.ExpiresIn) * time.Second),
	}

	for attempt := 0; attempt < 5; attempt++ { // на случай если будет дубликат существующего base62 в бд
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		code, err := generateShortCode()
		if err != nil {
			return nil, fmt.Errorf("generate short code: %w", err)
		}

		paste.ShortCode = code

		storedPaste, err := s.repo.CreatePaste(ctx, paste, key.String(), requestHash)
		if errors.Is(err, e.ErrShortCodeConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}

		return storedPaste, nil
	}

	return nil, fmt.Errorf("cannot allocate short code: %w", e.ErrShortCodeConflict)
}

func (s *service) GetPaste(ctx context.Context, shortCode string) (*m.Paste, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !validShortCode(shortCode) {
		return nil, fmt.Errorf("%w: invalid short code", e.ErrInvalidInput)
	}

	key := "paste:" + shortCode

	if s.cache != nil {
		var paste m.Paste

		cacheCtx, cancel := context.WithTimeout(ctx, cacheTimeout)
		found, err := s.cache.GetJSON(cacheCtx, key, &paste)
		cancel()

		if err != nil {
			log.Printf("cache read failed: %v", err)
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if err == nil && found && paste.ShortCode == shortCode {
			if err := checkPaste(&paste); err != nil {
				return nil, err
			}

			return &paste, nil
		}
	}

	paste, err := s.repo.GetPasteByCode(ctx, shortCode)
	if err != nil {
		return nil, err
	}

	if err := checkPaste(paste); err != nil {
		return nil, err
	}

	ttl := time.Until(paste.ExpiresAt)
	if ttl > cacheTTL {
		ttl = cacheTTL
	}

	if s.cache != nil && ttl >= time.Millisecond {
		cacheCtx, cancel := context.WithTimeout(ctx, cacheTimeout)
		err := s.cache.SetJSON(cacheCtx, key, paste, ttl)
		cancel()

		if err != nil {
			log.Printf("cache write failed: %v", err)
		}
	}

	if err := checkPaste(paste); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return paste, nil
}

func checkPaste(paste *m.Paste) error { // check paste for validation
	if !time.Now().Before(paste.ExpiresAt) || paste.Visibility != "public" {
		return e.ErrPasteNotFound
	}

	return nil
}

func generateShortCode() (string, error) {
	code := make([]byte, shortCodeLength)
	limit := big.NewInt(int64(len(base62Alphabet)))

	for i := range code {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}

		code[i] = base62Alphabet[n.Int64()]
	}

	return string(code), nil
}

func validShortCode(code string) bool {
	if len(code) != shortCodeLength {
		return false
	}

	for _, char := range code {
		if !strings.ContainsRune(base62Alphabet, char) {
			return false
		}
	}

	return true
}

func hashCreatePasteInput(req *CreatePasteInput) (string, error) {
	payload := struct {
		Content    string `json:"content"`
		Visibility string `json:"visibility"`
		ExpiresIn  int    `json:"expires_in"`
	}{
		Content:    req.Content,
		Visibility: req.Visibility,
		ExpiresIn:  req.ExpiresIn,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode create input: %w", err)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

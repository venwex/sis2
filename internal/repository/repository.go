package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	e "pastebin/internal/errors"
	m "pastebin/internal/models"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreatePaste(ctx context.Context, paste *m.Paste, key string, requestHash string) (*m.Paste, error)
	GetPasteByCode(ctx context.Context, code string) (*m.Paste, error)
}

type repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{
		db: db,
	}
}

func (r *repository) CreatePaste(ctx context.Context, paste *m.Paste, key string, requestHash string) (*m.Paste, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_ = tx.Rollback(cleanupCtx)
	}()

	// Пытаемся зарегистрировать ключ этой операции
	const reserveQuery = `
		INSERT INTO paste_creation_keys (idempotency_key, request_hash)
		VALUES ($1, $2)
		ON CONFLICT (idempotency_key) DO NOTHING
	`

	result, err := tx.Exec(ctx, reserveQuery, key, requestHash)
	if err != nil {
		return nil, fmt.Errorf("reserve idempotency key: %w", err)
	}

	// Ключ уже существует — читаем прежний результат
	if result.RowsAffected() == 0 {
		var storedHash string
		var storedResult []byte

		const lookupQuery = `
			SELECT request_hash, result
			FROM paste_creation_keys
			WHERE idempotency_key = $1
		`

		err := tx.QueryRow(ctx, lookupQuery, key).Scan(&storedHash, &storedResult)
		if err != nil {
			return nil, fmt.Errorf("read idempotency result: %w", err)
		}

		if storedHash != requestHash {
			return nil, e.ErrIdempotencyConflict
		}

		var savedPaste m.Paste

		if err := json.Unmarshal(storedResult, &savedPaste); err != nil {
			return nil, fmt.Errorf("decode idempotency result: %w", err)
		}

		return &savedPaste, nil
	}

	// Ключ новый — создаём пасту
	const insertQuery = `
		INSERT INTO pastes (id, short_code, user_id, content, visibility, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (short_code) DO NOTHING
	`

	result, err = tx.Exec(ctx, insertQuery, paste.ID, paste.ShortCode, paste.UserID, paste.Content,
		paste.Visibility, paste.CreatedAt, paste.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("insert paste: %w", err)
	}

	if result.RowsAffected() == 0 {
		return nil, e.ErrShortCodeConflict
	}

	// Запоминаем результат для будущих повторов.
	data, err := json.Marshal(paste)
	if err != nil {
		return nil, fmt.Errorf("encode idempotency result: %w", err)
	}

	const saveResultQuery = `
		UPDATE paste_creation_keys
		SET result = $2
		WHERE idempotency_key = $1
	`

	if _, err := tx.Exec(ctx, saveResultQuery, key, string(data)); err != nil {
		return nil, fmt.Errorf("save idempotency result: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create paste: %w", err)
	}

	return paste, nil
}

func (r *repository) GetPasteByCode(ctx context.Context, code string) (*m.Paste, error) {
	query := `SELECT id, short_code, user_id, content, visibility, created_at, expires_at 
			  FROM pastes 
			  WHERE short_code = $1
	`

	var paste m.Paste

	err := r.db.QueryRow(ctx, query, code).Scan(
		&paste.ID,
		&paste.ShortCode,
		&paste.UserID,
		&paste.Content,
		&paste.Visibility,
		&paste.CreatedAt,
		&paste.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, e.ErrPasteNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get paste by code: %w", err)
	}

	return &paste, nil
}

package repository

import (
	"context"
	"errors"
	"fmt"
	e "pastebin/internal/errors"
	m "pastebin/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreatePaste(ctx context.Context, paste *m.Paste) error
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

func (r *repository) CreatePaste(ctx context.Context, paste *m.Paste) error {
	query := `
		INSERT INTO 
		    pastes(id, short_code, user_id, content, visibility, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)                  
	`

	_, err := r.db.Exec(ctx, query,
		paste.ID,
		paste.ShortCode,
		paste.UserID,
		paste.Content,
		paste.Visibility,
		paste.CreatedAt,
		paste.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create paste: %w", err)
	}

	return nil
}

func (r *repository) GetPasteByCode(ctx context.Context, code string) (*m.Paste, error) {
	query := `SELECT id, short_code, user_id, content, visibility, created_at from pastes
			  WHERE code = $1
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

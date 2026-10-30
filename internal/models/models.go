package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Paste struct {
	ID         uuid.UUID  `json:"id"`
	ShortCode  string     `json:"short_code"`
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	Content    string     `json:"content"`
	Visibility string     `json:"visibility"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
}

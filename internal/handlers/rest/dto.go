package rest

import "time"

type CreatePasteRequest struct {
	Content    string `json:"content"`
	Visibility string `json:"visibility"`
	ExpiresIn  int    `json:"expires_in"`
}

type CreatePasteResponse struct {
	ShortCode string `json:"short_code"`
	URL       string `json:"url"`
}

type GetPasteResponse struct {
	ShortCode  string    `json:"short_code"`
	Content    string    `json:"content"`
	Visibility string    `json:"visibility"`
	ExpiresAt  time.Time `json:"expires_at"`
}

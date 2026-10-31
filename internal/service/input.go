package service

type CreatePasteInput struct {
	IdempotencyKey string
	Content        string
	Visibility     string
	ExpiresIn      int
}

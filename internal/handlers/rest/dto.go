package rest

type CreatePasteRequest struct {
	Content    string `json:"content"`
	Visibility string `json:"visibility"`
	ExpiresIn  int    `json:"expires_in"`
}

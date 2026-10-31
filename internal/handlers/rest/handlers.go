package rest

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	e "pastebin/internal/errors"
	"pastebin/internal/service"
	"pastebin/internal/utils"
	"time"
)

const (
	maxRequestBytes = 1024 * 1024
	requestTimeout  = 3 * time.Second
)

type RestHandler struct {
	service service.Service
	baseURL string
}

func NewHandler(svc service.Service, baseURL string) *RestHandler {
	return &RestHandler{
		service: svc,
		baseURL: baseURL,
	}
}

func (h *RestHandler) CreatePaste(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	defer r.Body.Close()

	var req CreatePasteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			utils.WriteError(w, http.StatusRequestEntityTooLarge, "request body is too large")
			return
		}

		utils.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	input := service.CreatePasteInput{
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
		Content:        req.Content,
		Visibility:     req.Visibility,
		ExpiresIn:      req.ExpiresIn,
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	paste, err := h.service.CreatePaste(ctx, &input)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	response := CreatePasteResponse{
		ShortCode: paste.ShortCode,
		URL:       h.baseURL + "/api/v1/pastes/" + paste.ShortCode,
	}

	utils.WriteJSON(w, http.StatusCreated, response)
}

func (h *RestHandler) GetPaste(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	paste, err := h.service.GetPaste(ctx, code)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	response := GetPasteResponse{
		ShortCode:  paste.ShortCode,
		Content:    paste.Content,
		Visibility: paste.Visibility,
		ExpiresAt:  paste.ExpiresAt,
	}

	// Пока не настраиваем отдельное HTTP-кэширование в браузере.
	w.Header().Set("Cache-Control", "no-store")
	utils.WriteJSON(w, http.StatusOK, response)
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	if r.Context().Err() != nil {
		return
	}

	switch {
	case errors.Is(err, e.ErrInvalidInput):
		utils.WriteError(w, http.StatusBadRequest, err.Error())

	case errors.Is(err, e.ErrPasteNotFound):
		utils.WriteError(w, http.StatusNotFound, "paste not found")

	case errors.Is(err, context.DeadlineExceeded):
		utils.WriteError(w, http.StatusServiceUnavailable, "operation timed out")

	case errors.Is(err, e.ErrIdempotencyConflict):
		utils.WriteError(w, http.StatusConflict, "idempotency key was used with different input")

	default:
		log.Printf("request failed: %v", err)
		utils.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

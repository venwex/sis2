package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 1024 * 1024

type RESTClient struct {
	baseURL string
	http    *http.Client
}

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

type HTTPError struct {
	StatusCode int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP request failed: %d %s", e.StatusCode, http.StatusText(e.StatusCode))
}

func NewRESTClient(baseURL string) *RESTClient {
	return &RESTClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{},
	}
}

func (c *RESTClient) CreatePaste(ctx context.Context, input CreatePasteRequest, idempotencyKey string) (*CreatePasteResponse, error) {
	if idempotencyKey == "" {
		return nil, errors.New("idempotency key is required")
	}

	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode create request: %w", err)
	}

	return doWithRetry(ctx, func(attemptCtx context.Context) (*CreatePasteResponse, error) {
		// Каждая попытка получает новый запрос и новый reader
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, c.baseURL+"/api/v1/pastes", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		return executeJSON[CreatePasteResponse](c.http, req, http.StatusCreated)
	}, isRetryableREST)
}

func (c *RESTClient) GetPaste(ctx context.Context, shortCode string) (*GetPasteResponse, error) {
	return doWithRetry(ctx, func(attemptCtx context.Context) (*GetPasteResponse, error) {
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, c.baseURL+"/api/v1/pastes/"+url.PathEscape(shortCode), nil)
		if err != nil {
			return nil, err
		}

		return executeJSON[GetPasteResponse](c.http, req, http.StatusOK)
	}, isRetryableREST)
}

func executeJSON[T any](httpClient *http.Client, req *http.Request, expectedStatus int) (*T, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		// Читаем небольшое тело ошибки для переиспользования соединения.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		return nil, &HTTPError{StatusCode: resp.StatusCode}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if len(data) > maxResponseBytes {
		return nil, errors.New("response body is too large")
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

func isRetryableREST(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == http.StatusServiceUnavailable
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.IsTimeout || dnsErr.IsTemporary
	}

	var netErr *net.OpError
	return errors.As(err, &netErr)
}

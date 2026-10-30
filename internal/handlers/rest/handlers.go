package rest

import (
	"net/http"
	"pastebin/internal/service"
)

type RestHandler struct {
	service service.Service
}

func NewHandler(svc service.Service) *RestHandler {
	return &RestHandler{
		service: svc,
	}
}

func (h *RestHandler) CreatePaste(w http.ResponseWriter, r *http.Request) {

}

func (h *RestHandler) GetPaste(w http.ResponseWriter, r *http.Request) {

}

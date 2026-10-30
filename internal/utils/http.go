package utils

import (
	"encoding/json"
	"log"
	"net/http"
)

type H map[string]any

func WriteJSON(w http.ResponseWriter, statusCode int, data any) {
	if statusCode == http.StatusNoContent {
		w.WriteHeader(statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Println("error during writing json:", err)
	}
}

func WriteError(w http.ResponseWriter, status int, text string) {
	WriteJSON(w, status, H{"error": text})
}

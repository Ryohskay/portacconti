package handler

import (
	"encoding/json"
	"net/http"
)

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}

func respondOK(w http.ResponseWriter, data any) {
	respond(w, http.StatusOK, data)
}

func respondCreated(w http.ResponseWriter, data any) {
	respond(w, http.StatusCreated, data)
}

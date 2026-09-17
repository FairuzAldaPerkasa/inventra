package operation

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"time"
)

var idPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if !idPattern.MatchString(id) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "id operasi harus berformat UUID",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	op, err := h.repo.GetByID(ctx, id)

	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "operasi tidak ditemukan",
		})
		return
	}
	if err != nil {
		log.Printf("Gagal mengambil operasi %s: %v", id, err)

		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "gagal mengambil status operasi",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": op,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Gagal menulis respons operasi: %v", err)
	}
}

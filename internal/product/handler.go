package product

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit, err := parseInteger(r.URL.Query().Get("limit"), 20)
	if err != nil || limit < 1 || limit > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "limit harus berupa angka antara 1 dan 100",
		})
		return
	}

	offset, err := parseInteger(r.URL.Query().Get("offset"), 0)
	if err != nil || offset < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "offset harus berupa angka nol atau lebih",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	products, err := h.repo.List(ctx, limit, offset)
	if err != nil {
		log.Printf("Gagal mengambil daftar produk: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "gagal mengambil daftar produk",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": products,
		"pagination": map[string]int{
			"limit":  limit,
			"offset": offset,
			"count":  len(products),
		},
	})
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "id produk harus berupa angka positif",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	p, err := h.repo.GetByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "produk tidak ditemukan",
		})
		return
	}
	if err != nil {
		log.Printf("Gagal mengambil produk %d: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "gagal mengambil detail produk",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": p,
	})
}

func parseInteger(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}

	return strconv.Atoi(value)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Gagal menulis respons produk: %v", err)
	}
}

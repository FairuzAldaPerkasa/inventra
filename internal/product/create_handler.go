package product

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"
)

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	// Batasi request body menjadi 64 KiB.
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req CreateRequest

	if err := decoder.Decode(&req); err != nil {
		writeCreateDecodeError(w, err)
		return
	}

	// Request harus berisi tepat satu nilai JSON.
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			writeCreateDecodeError(w, err)
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "body harus berisi satu objek JSON",
			})
		}
		return
	}

	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	operationID, err := h.repo.EnqueueCreate(ctx, req)
	if err != nil {
		log.Printf("Gagal menerima permintaan create produk: %v", err)

		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "gagal mengonfirmasi penerimaan permintaan",
		})
		return
	}

	// Alamat untuk memeriksa hasil pemrosesan operasi.
	statusURL := "/api/operations/" + operationID
	w.Header().Set("Location", statusURL)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"operation_id": operationID,
		"status":       "pending",
		"status_url":   statusURL,
		"message":      "permintaan pembuatan produk diterima",
	})
}

func writeCreateDecodeError(w http.ResponseWriter, err error) {
	var sizeErr *http.MaxBytesError
	if errors.As(err, &sizeErr) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
			"error": "body maksimal 64 KiB",
		})
		return
	}

	writeJSON(w, http.StatusBadRequest, map[string]string{
		"error": "JSON tidak valid; gunakan field yang sesuai dan price berupa string",
	})
}

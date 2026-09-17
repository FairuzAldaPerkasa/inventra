package product

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || productID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "id produk harus berupa angka positif",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req UpdateRequest

	if err := decoder.Decode(&req); err != nil {
		writeCreateDecodeError(w, err)
		return
	}

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

	operationID, err := h.repo.EnqueueUpdate(ctx, productID, req)
	if err != nil {
		log.Printf("Gagal menerima update produk %d: %v", productID, err)

		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "gagal mengonfirmasi penerimaan permintaan",
		})
		return
	}

	statusURL := "/api/operations/" + operationID
	w.Header().Set("Location", statusURL)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"operation_id": operationID,
		"status":       "pending",
		"status_url":   statusURL,
		"message":      "permintaan perubahan produk diterima",
	})
}

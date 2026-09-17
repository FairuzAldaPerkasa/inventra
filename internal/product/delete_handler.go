package product

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"
)

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || productID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "id produk harus berupa angka positif",
		})
		return
	}

	version, err := strconv.Atoi(r.URL.Query().Get("version"))
	if err != nil || version < 1 || version >= 2147483647 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "parameter version wajib berupa angka antara 1 dan 2147483646",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	operationID, err := h.repo.EnqueueDelete(ctx, productID, version)
	if err != nil {
		log.Printf("Gagal menerima delete produk %d: %v", productID, err)

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
		"message":      "permintaan penghapusan produk diterima",
	})
}

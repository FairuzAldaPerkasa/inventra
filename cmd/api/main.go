package main

import (
	"encoding/json"
	"inventra/internal/config"
	"log"
	"net"
	"net/http"
	"time"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Konfigurasi tidak valid: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler(cfg.AppName))

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		log.Fatalf("Gagal membuka alamat server: %v", err)
	}

	log.Printf("%s berjalan di http://%s", cfg.AppName, listener.Addr())

	if err := server.Serve(listener); err != nil &&
		err != http.ErrServerClosed {
		log.Fatalf("Server berhenti karena error: %v", err)
	}
}

func healthHandler(appName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		response := map[string]string{
			"status":  "ok",
			"service": appName,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Gagal menulis respons health: %v", err)
		}
	}
}

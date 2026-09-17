package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"inventra/internal/config"
	"inventra/internal/database"

	"github.com/jackc/pgx/v5/pgxpool"

	"inventra/internal/product"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pool, err := database.Open(ctx, cfg.DBPassword)
	if err != nil {
		return err
	}
	defer pool.Close()

	log.Println("Koneksi PostgreSQL berhasil")

	productRepo := product.NewRepository(pool)
	productHandler := product.NewHandler(productRepo)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler(cfg.AppName))
	mux.HandleFunc("/ready", readyHandler(pool))

	mux.HandleFunc("GET /api/products", productHandler.List)
	mux.HandleFunc("GET /api/products/{id}", productHandler.GetByID)

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
		return err
	}

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- server.Serve(listener)
	}()

	log.Printf("%s berjalan di http://%s", cfg.AppName, listener.Addr())

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err

	case <-ctx.Done():
		log.Println("Menghentikan server...")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}

		return nil
	}
}

func healthHandler(appName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !allowGET(w, r) {
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": appName,
		})
	}
}

func readyHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !allowGET(w, r) {
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			log.Printf("Pemeriksaan database gagal: %v", err)

			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status":   "not_ready",
				"database": "unavailable",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status":   "ready",
			"database": "ok",
		})
	}
}

func allowGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}

	return true
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Gagal menulis respons JSON: %v", err)
	}
}

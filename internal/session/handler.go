package session

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"inventra/internal/auth"
	"inventra/internal/user"
)

const CookieName = "inventra_session"

// dummyPasswordHash dipakai untuk menjaga waktu respons login tetap
// konsisten ketika email tidak ditemukan / akun nonaktif, agar tidak
// membocorkan keberadaan akun lewat timing.
var dummyPasswordHash = func() string {
	hash, err := auth.HashPassword([]byte("kata-sandi-dummy-untuk-timing-000"))
	if err != nil {
		panic(err)
	}
	return hash
}()

type Handler struct {
	users        *user.Repository
	sessions     *Repository
	sessionTTL   time.Duration
	cookieSecure bool
	cookieDomain string
}

func NewHandler(
	users *user.Repository,
	sessions *Repository,
	sessionTTL time.Duration,
	cookieSecure bool,
	cookieDomain string,
) *Handler {
	return &Handler{
		users:        users,
		sessions:     sessions,
		sessionTTL:   sessionTTL,
		cookieSecure: cookieSecure,
		cookieDomain: cookieDomain,
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4*1024))
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Payload JSON tidak valid")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := []byte(req.Password)

	if len(email) == 0 || len(email) > 254 {
		writeError(w, http.StatusUnauthorized, "Email atau password salah")
		return
	}
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		writeError(w, http.StatusUnauthorized, "Email atau password salah")
		return
	}
	if !utf8.Valid(password) || len(password) == 0 || utf8.RuneCount(password) > 128 {
		writeError(w, http.StatusUnauthorized, "Email atau password salah")
		return
	}

	ctx := r.Context()

	u, err := h.users.FindByEmail(ctx, email)
	if errors.Is(err, user.ErrNotFound) {
		_, _ = auth.VerifyPassword(password, dummyPasswordHash)
		writeError(w, http.StatusUnauthorized, "Email atau password salah")
		return
	}
	if err != nil {
		log.Printf("Login: gagal mengambil pengguna: %v", err)
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server")
		return
	}

	if !u.IsActive {
		_, _ = auth.VerifyPassword(password, dummyPasswordHash)
		writeError(w, http.StatusUnauthorized, "Email atau password salah")
		return
	}

	match, err := auth.VerifyPassword(password, u.PasswordHash)
	if err != nil {
		log.Printf("Login: hash password tidak didukung untuk user %s: %v", u.ID, err)
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server")
		return
	}
	if !match {
		writeError(w, http.StatusUnauthorized, "Email atau password salah")
		return
	}

	rawToken, tokenHash, err := GenerateToken()
	if err != nil {
		log.Printf("Login: gagal membuat token sesi: %v", err)
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server")
		return
	}

	expiresAt := time.Now().Add(h.sessionTTL)

	if err := h.sessions.Create(ctx, tokenHash, u.ID, expiresAt); err != nil {
		log.Printf("Login: gagal menyimpan sesi: %v", err)
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server")
		return
	}

	h.setSessionCookie(w, rawToken, expiresAt)

	writeJSON(w, http.StatusOK, map[string]any{
		"user": userResponse{
			ID:    u.ID,
			Name:  u.Name,
			Email: u.Email,
			Role:  u.Role,
		},
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(CookieName); err == nil && cookie.Value != "" {
		if err := h.sessions.Revoke(r.Context(), HashToken(cookie.Value)); err != nil {
			log.Printf("Logout: gagal mencabut sesi: %v", err)
		}
	}

	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	authUser, err := h.authenticate(r)
	if errors.Is(err, ErrNotFound) {
		h.clearSessionCookie(w)
		writeError(w, http.StatusUnauthorized, "Sesi tidak valid atau sudah berakhir")
		return
	}
	if err != nil {
		log.Printf("Me: gagal memeriksa sesi: %v", err)
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": userResponse{
			ID:    authUser.ID,
			Name:  authUser.Name,
			Email: authUser.Email,
			Role:  authUser.Role,
		},
	})
}

// authenticate mengambil pengguna dari cookie sesi pada request.
// Diekspos lewat metode terpisah supaya bisa dipakai ulang oleh
// middleware pembatasan role pada tahap "Hak akses dan audit".
func (h *Handler) authenticate(r *http.Request) (*AuthenticatedUser, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil || cookie.Value == "" {
		return nil, ErrNotFound
	}

	return h.sessions.FindActiveByTokenHash(r.Context(), HashToken(cookie.Value))
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, rawToken string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    rawToken,
		Path:     "/",
		Domain:   h.cookieDomain,
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Domain:   h.cookieDomain,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Gagal menulis respons JSON: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

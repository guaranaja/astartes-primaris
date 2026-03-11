package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	sessionsMu sync.RWMutex
	sessions   = make(map[string]time.Time) // token → expiry
	authPass   = os.Getenv("PRIMARCH_AUTH_PASSWORD")
)

const sessionTTL = 24 * time.Hour

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func validSession(token string) bool {
	sessionsMu.RLock()
	defer sessionsMu.RUnlock()
	exp, ok := sessions[token]
	return ok && time.Now().Before(exp)
}

func createSession() string {
	token := generateToken()
	sessionsMu.Lock()
	sessions[token] = time.Now().Add(sessionTTL)
	sessionsMu.Unlock()
	return token
}

// handleLogin validates the password and returns a session cookie.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if authPass == "" || body.Password != authPass {
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}
	token := createSession()
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "authenticated"})
}

// handleLogout clears the session.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session"); err == nil {
		sessionsMu.Lock()
		delete(sessions, c.Value)
		sessionsMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// handleAuthCheck returns 200 if authenticated, 401 if not.
func (s *Server) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "authenticated"})
}

// authMiddleware protects API routes with session-based auth.
// Public routes: /health, /api/v1/login
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth if no password is configured
		if authPass == "" {
			next.ServeHTTP(w, r)
			return
		}
		// Public routes
		path := r.URL.Path
		if path == "/health" || path == "/api/v1/login" {
			next.ServeHTTP(w, r)
			return
		}
		// Static files (served by Aurum, but in case Primarch serves them)
		if path == "/" || path == "/login" || path == "/app.js" || path == "/style.css" || path == "/favicon.ico" {
			next.ServeHTTP(w, r)
			return
		}
		// Check session cookie
		c, err := r.Cookie("session")
		if err != nil || !validSession(c.Value) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	sessionsMu sync.RWMutex
	sessions   = make(map[string]time.Time) // token → expiry

	googleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	adminEmails    = parseAdminEmails(os.Getenv("ADMIN_EMAILS"))
	serviceToken   = os.Getenv("PRIMARCH_SERVICE_TOKEN")
)

const sessionTTL = 24 * time.Hour

func parseAdminEmails(s string) map[string]bool {
	m := make(map[string]bool)
	for _, e := range strings.Split(s, ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			m[e] = true
		}
	}
	return m
}

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

func validBearerToken(r *http.Request) bool {
	if serviceToken == "" {
		return false
	}
	auth := r.Header.Get("Authorization")
	return strings.HasPrefix(auth, "Bearer ") && strings.TrimPrefix(auth, "Bearer ") == serviceToken
}

func createSession() string {
	token := generateToken()
	sessionsMu.Lock()
	sessions[token] = time.Now().Add(sessionTTL)
	sessionsMu.Unlock()
	return token
}

// verifyGoogleToken validates a Google ID token via Google's tokeninfo endpoint.
func verifyGoogleToken(idToken string) (string, error) {
	resp, err := http.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + idToken)
	if err != nil {
		return "", fmt.Errorf("failed to verify token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("invalid token: %s", string(body))
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Aud           string `json:"aud"`
	}
	if err := json.Unmarshal(body, &claims); err != nil {
		return "", fmt.Errorf("failed to parse token claims: %w", err)
	}

	if googleClientID != "" && claims.Aud != googleClientID {
		return "", fmt.Errorf("token audience mismatch")
	}
	if claims.EmailVerified != "true" {
		return "", fmt.Errorf("email not verified")
	}
	return claims.Email, nil
}

// handleLogin validates a Google ID token and returns a session cookie.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Credential string `json:"credential"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	email, err := verifyGoogleToken(body.Credential)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid google token")
		return
	}
	if !adminEmails[email] {
		writeError(w, http.StatusForbidden, "unauthorized email: "+email)
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
	writeJSON(w, http.StatusOK, map[string]string{"status": "authenticated", "email": email})
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

// authMiddleware protects API routes with Google session-based auth.
// Public routes: /health, /api/v1/login
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth if no admin emails configured (dev mode)
		if len(adminEmails) == 0 {
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
		// Check bearer token for engine protocol routes
		if strings.HasPrefix(path, "/api/v1/engine/") {
			if validBearerToken(r) {
				next.ServeHTTP(w, r)
				return
			}
			writeError(w, http.StatusUnauthorized, "invalid service token")
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

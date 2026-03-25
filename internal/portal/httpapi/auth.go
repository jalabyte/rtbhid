package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const sessionCookieName = "rtbhd_session"

type AuthConfig struct {
	Enabled       bool
	Username      string
	Password      string
	SessionSecret string
	SessionTTL    time.Duration
	CookieSecure  bool
}

type authManager struct {
	cfg       AuthConfig
	secretKey []byte
}

func newAuthManager(cfg AuthConfig) (*authManager, error) {
	m := &authManager{cfg: cfg}
	if !cfg.Enabled {
		return m, nil
	}
	if cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("auth enabled but username/password is empty")
	}
	if cfg.SessionTTL <= 0 {
		m.cfg.SessionTTL = 24 * time.Hour
	}
	if cfg.SessionSecret != "" {
		m.secretKey = []byte(cfg.SessionSecret)
		return m, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate session secret: %w", err)
	}
	m.secretKey = key
	return m, nil
}

func (m *authManager) requireAuth(next http.Handler) http.Handler {
	if !m.cfg.Enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		_, ok := m.currentUser(r)
		if ok {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html") {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
	})
}

func (m *authManager) currentUser(r *http.Request) (string, bool) {
	if !m.cfg.Enabled {
		return "", true
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	username, expiresAt, ok := m.parseSession(cookie.Value)
	if !ok {
		return "", false
	}
	if username != m.cfg.Username {
		return "", false
	}
	if time.Now().UTC().After(expiresAt) {
		return "", false
	}
	return username, true
}

func (m *authManager) issueSession(w http.ResponseWriter, username string) {
	expires := time.Now().UTC().Add(m.cfg.SessionTTL)
	token := m.makeSession(username, expires)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   m.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *authManager) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *authManager) checkCredentials(username, password string) bool {
	if !m.cfg.Enabled {
		return true
	}
	if subtle.ConstantTimeCompare([]byte(username), []byte(m.cfg.Username)) != 1 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(password), []byte(m.cfg.Password)) == 1
}

func (m *authManager) makeSession(username string, expiresAt time.Time) string {
	payload := username + "|" + strconv.FormatInt(expiresAt.Unix(), 10)
	sig := hmacSHA256Hex(m.secretKey, payload)
	raw := payload + "|" + sig
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func (m *authManager) parseSession(token string) (string, time.Time, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", time.Time{}, false
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return "", time.Time{}, false
	}
	username := strings.TrimSpace(parts[0])
	if username == "" {
		return "", time.Time{}, false
	}
	expUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", time.Time{}, false
	}
	givenSig := parts[2]
	expectedSig := hmacSHA256Hex(m.secretKey, parts[0]+"|"+parts[1])
	if subtle.ConstantTimeCompare([]byte(givenSig), []byte(expectedSig)) != 1 {
		return "", time.Time{}, false
	}
	return username, time.Unix(expUnix, 0).UTC(), true
}

func hmacSHA256Hex(secret []byte, payload string) string {
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

func isPublicPath(path string) bool {
	switch path {
	case "/login", "/docs", "/api/v1/health", "/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/auth/me":
		return true
	default:
		return false
	}
}

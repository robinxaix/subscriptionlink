package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	cookieName         = "admin_session"
	defaultSessionTTL  = 30 * time.Minute
	maxLoginFailures   = 5
	loginBlockDuration = 2 * time.Minute
	loginFailureWindow = 5 * time.Minute
)

var (
	ErrNotConfigured = errors.New("admin auth unavailable")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrRateLimited   = errors.New("too many attempts")
)

type Session struct {
	ID        string
	CSRFToken string
	ExpiresAt time.Time
}

type failureState struct {
	Count      int
	WindowFrom time.Time
	BlockUntil time.Time
}

type Manager struct {
	adminToken string
	secure     bool
	ttl        time.Duration

	mu       sync.Mutex
	sessions map[string]Session
	failures map[string]failureState
}

func NewManager(adminToken string, secureCookie bool) *Manager {
	return &Manager{
		adminToken: adminToken,
		secure:     secureCookie,
		ttl:        defaultSessionTTL,
		sessions:   make(map[string]Session),
		failures:   make(map[string]failureState),
	}
}

func (m *Manager) IsConfigured() bool {
	return m.adminToken != ""
}

func (m *Manager) Login(remoteAddr, token string) (Session, error) {
	if !m.IsConfigured() {
		return Session{}, ErrNotConfigured
	}

	ip := normalizeIP(remoteAddr)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(time.Now())

	f := m.failures[ip]
	now := time.Now()
	if f.BlockUntil.After(now) {
		return Session{}, ErrRateLimited
	}

	if token != m.adminToken {
		if f.WindowFrom.IsZero() || now.Sub(f.WindowFrom) > loginFailureWindow {
			f.WindowFrom = now
			f.Count = 0
		}
		f.Count++
		if f.Count >= maxLoginFailures {
			f.BlockUntil = now.Add(loginBlockDuration)
		}
		m.failures[ip] = f
		return Session{}, ErrUnauthorized
	}

	delete(m.failures, ip)
	s := Session{
		ID:        randomHex(32),
		CSRFToken: randomHex(32),
		ExpiresAt: now.Add(m.ttl),
	}
	m.sessions[s.ID] = s
	return s, nil
}

func (m *Manager) WriteSessionCookie(w http.ResponseWriter, sessionID string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sessionID,
		Path:     "/api/admin",
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
	})
}

func (m *Manager) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/api/admin",
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func (m *Manager) SessionFromRequest(r *http.Request) (Session, bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return Session{}, false
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(time.Now())

	s, ok := m.sessions[cookie.Value]
	if !ok {
		return Session{}, false
	}
	if time.Now().After(s.ExpiresAt) {
		delete(m.sessions, cookie.Value)
		return Session{}, false
	}
	return s, true
}

func (m *Manager) LogoutByRequest(r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return
	}

	m.mu.Lock()
	delete(m.sessions, cookie.Value)
	m.mu.Unlock()
}

func (m *Manager) Require(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.IsConfigured() {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		s, ok := m.SessionFromRequest(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete:
			if r.Header.Get("X-CSRF-Token") != s.CSRFToken {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}

		next(w, r)
	}
}

func (m *Manager) cleanupLocked(now time.Time) {
	for id, s := range m.sessions {
		if now.After(s.ExpiresAt) {
			delete(m.sessions, id)
		}
	}
	for ip, f := range m.failures {
		if f.BlockUntil.IsZero() && now.Sub(f.WindowFrom) > loginFailureWindow {
			delete(m.failures, ip)
		}
		if !f.BlockUntil.IsZero() && now.After(f.BlockUntil) {
			delete(m.failures, ip)
		}
	}
}

func normalizeIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

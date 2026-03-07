package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"subscriptionlink/internal/api"
	"subscriptionlink/internal/auth"
	"subscriptionlink/internal/stats"
)

type loginRequest struct {
	Token string `json:"token"`
}

type sessionResponse struct {
	CSRFToken string `json:"csrf_token"`
	ExpiresAt int64  `json:"expires_at"`
}

func main() {
	adminToken := os.Getenv("ADMIN_TOKEN")
	secureCookie := strings.EqualFold(os.Getenv("ADMIN_COOKIE_SECURE"), "true")
	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = "127.0.0.1:8081"
	}
	authManager := auth.NewManager(adminToken, secureCookie)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/", api.SubHandler)

	mux.HandleFunc("/api/admin/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authManager.IsConfigured() {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		s, err := authManager.Login(r.RemoteAddr, req.Token)
		if err != nil {
			switch err {
			case auth.ErrRateLimited:
				http.Error(w, "too many attempts", http.StatusTooManyRequests)
			case auth.ErrNotConfigured:
				http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			default:
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			}
			return
		}

		authManager.WriteSessionCookie(w, s.ID, s.ExpiresAt)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sessionResponse{CSRFToken: s.CSRFToken, ExpiresAt: s.ExpiresAt.Unix()})
	})

	mux.HandleFunc("/api/admin/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s, ok := authManager.SessionFromRequest(r)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sessionResponse{CSRFToken: s.CSRFToken, ExpiresAt: s.ExpiresAt.Unix()})
	})

	mux.HandleFunc("/api/admin/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authManager.LogoutByRequest(r)
		authManager.ClearSessionCookie(w)
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/api/admin/users", authManager.Require(api.UserHandler))
	mux.HandleFunc("/api/admin/nodes", authManager.Require(api.NodeHandler))
	mux.HandleFunc("/api/admin/stats", authManager.Require(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stats.Get())
	}))

	fs := http.FileServer(http.Dir("./web"))
	mux.Handle("/", fs)

	handler := withSecurityHeaders(mux)

	fmt.Printf("Subscription server running on %s\n", listenAddr)
	if err := http.ListenAndServe(listenAddr, handler); err != nil {
		fmt.Printf("server stopped: %v\n", err)
		os.Exit(1)
	}
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'self'")
		next.ServeHTTP(w, r)
	})
}

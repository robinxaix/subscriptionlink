package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"subscriptionlink/internal/api"
	"subscriptionlink/internal/auth"
	"subscriptionlink/internal/stats"
	"subscriptionlink/internal/store"
	"subscriptionlink/internal/xray"

	"embed"
)

//go:embed embedded_assets/web
var webAssets embed.FS

func embeddedAssetsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" || path == "index.html" {
			http.NotFound(w, r)
			return
		}

		data, err := webAssets.ReadFile("embedded_assets/web/" + path)
		if err != nil {
			fmt.Printf("Error reading embedded_assets/web/%s: %v\n", path, err)
			http.NotFound(w, r)
			return
		}
		contentType := mimeType(path)
		w.Header().Set("Content-Type", contentType)
		w.Write(data)
	})
}

func mimeType(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	default:
		return "text/plain; charset=utf-8"
	}
}

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
	listenAddrFlag := flag.String("listen_addr", envOrDefault("LISTEN_ADDR", "127.0.0.1:8081"), "http listen address")
	dataDirFlag := flag.String("data_dir", envOrDefault("DATA_DIR", "data"), "runtime data directory")
	xrayConfigPathFlag := flag.String("xray_config_path", envOrDefault("XRAY_CONFIG_PATH", "/usr/local/etc/xray/config.json"), "xray config file path")
	flag.Parse()

	listenAddr := *listenAddrFlag
	store.SetDataDir(*dataDirFlag)
	if err := os.Setenv("XRAY_CONFIG_PATH", strings.TrimSpace(*xrayConfigPathFlag)); err != nil {
		fmt.Printf("failed to set XRAY_CONFIG_PATH: %v\n", err)
		os.Exit(1)
	}

	// Load existing users from users.json
	existingUsers := store.LoadUsers()

	// Load clients from xray config and merge
	xrayUsers, err := xray.LoadClientsFromConfig()
	if err != nil {
		fmt.Printf("warning: failed to load xray clients: %v\n", err)
	}
	if len(xrayUsers) > 0 {
		mergedUsers := store.MergeUsers(existingUsers, xrayUsers)
		store.SaveUsers(mergedUsers)
		fmt.Printf("Loaded %d clients from xray config, total %d users\n", len(xrayUsers), len(mergedUsers))
	}

	resolvedToken, generated, err := resolveAdminToken(adminToken)
	if err != nil {
		fmt.Printf("failed to resolve admin token: %v\n", err)
		os.Exit(1)
	}
	if generated {
		fmt.Printf("ADMIN_TOKEN not set, generated key saved to %s\n", store.DataFile("admin.key"))
	}
	adminToken = resolvedToken
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

	fmt.Println("dataDir:" + *dataDirFlag)
	mux.Handle("/", embeddedAssetsHandler())

	handler := withSecurityHeaders(mux)

	fmt.Printf("Subscription server running on %s (data dir: %s)\n", listenAddr, store.DataDir())
	if err := http.ListenAndServe(listenAddr, handler); err != nil {
		fmt.Printf("server stopped: %v\n", err)
		os.Exit(1)
	}
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func resolveAdminToken(raw string) (token string, generated bool, err error) {
	if err := os.MkdirAll(store.DataDir(), 0o755); err != nil {
		return "", false, err
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed != "" {
		return trimmed, false, nil
	}

	token, err = randomUUID()
	if err != nil {
		return "", false, err
	}
	if err := os.WriteFile(store.DataFile("admin.key"), []byte(token+"\n"), 0o600); err != nil {
		return "", false, err
	}
	return token, true, nil
}

func randomUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[0:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:16]), nil
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

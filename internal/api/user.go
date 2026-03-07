package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"subscriptionlink/internal/model"
	"subscriptionlink/internal/store"
	"subscriptionlink/internal/xray"
)

func UserHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(store.LoadUsers())
	case http.MethodPost:
		var u model.User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if u.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if u.Email == "" {
			u.Email = fallbackEmail(u.Name, u.Token)
		}
		if !strings.Contains(u.Email, "@") {
			http.Error(w, "invalid email", http.StatusBadRequest)
			return
		}
		if u.Token == "" {
			u.Token = randomHex(16)
		}
		if u.UUID == "" {
			u.UUID = randomUUID()
		}

		users := store.LoadUsers()
		beforeUsers := cloneUsers(users)
		for _, existed := range users {
			if existed.Token == u.Token {
				http.Error(w, "token already exists", http.StatusConflict)
				return
			}
		}
		users = append(users, u)
		store.SaveUsers(users)
		if err := xray.SyncUsers(users); err != nil {
			store.SaveUsers(beforeUsers)
			http.Error(w, "sync xray failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(u)
	case http.MethodPut:
		var in model.User
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if in.Token == "" {
			http.Error(w, "token is required", http.StatusBadRequest)
			return
		}
		if in.Email != "" && !strings.Contains(in.Email, "@") {
			http.Error(w, "invalid email", http.StatusBadRequest)
			return
		}

		users := store.LoadUsers()
		beforeUsers := cloneUsers(users)
		for i := range users {
			if users[i].Token != in.Token {
				continue
			}
			if in.Name != "" {
				users[i].Name = in.Name
			}
			if in.UUID != "" {
				users[i].UUID = in.UUID
			}
			if in.Email != "" {
				users[i].Email = in.Email
			}
			users[i].Expire = in.Expire
			store.SaveUsers(users)
			if err := xray.SyncUsers(users); err != nil {
				store.SaveUsers(beforeUsers)
				http.Error(w, "sync xray failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(users[i])
			return
		}
		http.Error(w, "user not found", http.StatusNotFound)
	case http.MethodDelete:
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "token is required", http.StatusBadRequest)
			return
		}
		users := store.LoadUsers()
		beforeUsers := cloneUsers(users)
		for i := range users {
			if users[i].Token == token {
				users = append(users[:i], users[i+1:]...)
				store.SaveUsers(users)
				if err := xray.SyncUsers(users); err != nil {
					store.SaveUsers(beforeUsers)
					http.Error(w, "sync xray failed: "+err.Error(), http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		http.Error(w, "user not found", http.StatusNotFound)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randomUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[0:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:16])
}

func cloneUsers(in []model.User) []model.User {
	out := make([]model.User, len(in))
	copy(out, in)
	return out
}

func fallbackEmail(name, token string) string {
	base := strings.TrimSpace(name)
	if base == "" {
		base = token
	}
	if base == "" {
		base = "user"
	}
	base = strings.ReplaceAll(strings.ToLower(base), " ", "-")
	return base + "@example.com"
}

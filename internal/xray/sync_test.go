package xray

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"subscriptionlink/internal/model"
)

func TestSyncUsersReplacesClientsByTag(t *testing.T) {
	t.Setenv("XRAY_INBOUND_TAG", "main-in")
	t.Setenv("XRAY_RELOAD_CMD", "")

	configPath := writeTempConfig(t, map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "main-in",
				"protocol": "vless",
				"settings": map[string]interface{}{
					"clients": []interface{}{
						map[string]interface{}{"id": "old-id", "email": "old@example.com"},
					},
				},
			},
			map[string]interface{}{
				"tag":      "other",
				"protocol": "vmess",
				"settings": map[string]interface{}{"clients": []interface{}{}},
			},
		},
	})
	t.Setenv("XRAY_CONFIG_PATH", configPath)

	users := []model.User{
		{Name: "alice", Email: "alice@example.com", UUID: "uuid-a"},
		{Name: "bob", UUID: "uuid-b"},
		{Name: "dup", Email: "dup@example.com", UUID: "uuid-a"},
	}

	if err := SyncUsers(users); err != nil {
		t.Fatalf("SyncUsers() error = %v", err)
	}

	cfg := readConfig(t, configPath)
	clients := getClientsByTag(t, cfg, "main-in")
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}

	assertClient(t, clients[0], "uuid-a", "alice@example.com")
	assertClient(t, clients[1], "uuid-b", "bob@example.com")
}

func TestSyncUsersUsesFirstVlessInboundWithoutTag(t *testing.T) {
	t.Setenv("XRAY_INBOUND_TAG", "")
	t.Setenv("XRAY_RELOAD_CMD", "")

	configPath := writeTempConfig(t, map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{"tag": "a", "protocol": "trojan", "settings": map[string]interface{}{}},
			map[string]interface{}{"tag": "b", "protocol": "vless", "settings": map[string]interface{}{"clients": []interface{}{}}},
		},
	})
	t.Setenv("XRAY_CONFIG_PATH", configPath)

	if err := SyncUsers([]model.User{{Name: "neo", Email: "neo@example.com", UUID: "uuid-neo"}}); err != nil {
		t.Fatalf("SyncUsers() error = %v", err)
	}

	cfg := readConfig(t, configPath)
	clients := getClientsByTag(t, cfg, "b")
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	assertClient(t, clients[0], "uuid-neo", "neo@example.com")
}

func TestSyncUsersUpdatesAllVlessInboundsWhenNoTag(t *testing.T) {
	t.Setenv("XRAY_INBOUND_TAG", "")
	t.Setenv("XRAY_RELOAD_CMD", "")

	configPath := writeTempConfig(t, map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{"tag": "vless-a", "protocol": "vless", "settings": map[string]interface{}{"clients": []interface{}{} }},
			map[string]interface{}{"tag": "vmess-b", "protocol": "vmess", "settings": map[string]interface{}{"clients": []interface{}{} }},
			map[string]interface{}{"tag": "vless-c", "protocol": "vless", "settings": map[string]interface{}{"clients": []interface{}{} }},
		},
	})
	t.Setenv("XRAY_CONFIG_PATH", configPath)

	users := []model.User{
		{Name: "u1", Email: "u1@example.com", UUID: "uuid-1"},
		{Name: "u2", Email: "u2@example.com", UUID: "uuid-2"},
	}
	if err := SyncUsers(users); err != nil {
		t.Fatalf("SyncUsers() error = %v", err)
	}

	cfg := readConfig(t, configPath)
	clientsA := getClientsByTag(t, cfg, "vless-a")
	clientsC := getClientsByTag(t, cfg, "vless-c")
	clientsB := getClientsByTag(t, cfg, "vmess-b")

	if len(clientsA) != 2 || len(clientsC) != 2 {
		t.Fatalf("expected vless inbounds to have 2 clients, got a=%d c=%d", len(clientsA), len(clientsC))
	}
	if len(clientsB) != 0 {
		t.Fatalf("expected non-vless inbound unchanged, got %d clients", len(clientsB))
	}
}

func TestSyncUsersReturnsErrorWhenTagNotFound(t *testing.T) {
	t.Setenv("XRAY_INBOUND_TAG", "missing")
	t.Setenv("XRAY_RELOAD_CMD", "")

	configPath := writeTempConfig(t, map[string]interface{}{
		"inbounds": []interface{}{
			map[string]interface{}{"tag": "exists", "protocol": "vless", "settings": map[string]interface{}{}},
		},
	})
	t.Setenv("XRAY_CONFIG_PATH", configPath)

	if err := SyncUsers([]model.User{{UUID: "u1", Email: "u1@example.com"}}); err == nil {
		t.Fatalf("expected error when inbound tag is missing")
	}
}

func TestSyncUsersNoConfigPathNoop(t *testing.T) {
	t.Setenv("XRAY_CONFIG_PATH", "")
	t.Setenv("XRAY_RELOAD_CMD", "")
	if err := SyncUsers([]model.User{{UUID: "u1", Email: "u1@example.com"}}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func writeTempConfig(t *testing.T, cfg map[string]interface{}) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "xray.json")
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func readConfig(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	return cfg
}

func getClientsByTag(t *testing.T, cfg map[string]interface{}, tag string) []map[string]interface{} {
	t.Helper()
	inbounds, _ := cfg["inbounds"].([]interface{})
	for _, it := range inbounds {
		inbound, _ := it.(map[string]interface{})
		if inbound["tag"] != tag {
			continue
		}
		settings, _ := inbound["settings"].(map[string]interface{})
		clientsAny, _ := settings["clients"].([]interface{})
		clients := make([]map[string]interface{}, 0, len(clientsAny))
		for _, c := range clientsAny {
			m, _ := c.(map[string]interface{})
			clients = append(clients, m)
		}
		return clients
	}
	t.Fatalf("inbound with tag %q not found", tag)
	return nil
}

func assertClient(t *testing.T, client map[string]interface{}, id, email string) {
	t.Helper()
	if client["id"] != id {
		t.Fatalf("expected id %q, got %v", id, client["id"])
	}
	if client["email"] != email {
		t.Fatalf("expected email %q, got %v", email, client["email"])
	}
}

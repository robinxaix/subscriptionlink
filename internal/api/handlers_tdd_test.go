package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"subscriptionlink/internal/model"
	"subscriptionlink/internal/stats"
)

func TestUserHandler_AllowsResetExpireToZero(t *testing.T) {
	t.Setenv("XRAY_CONFIG_PATH", "")
	withTempDataDir(t, []model.User{{Name: "u", Token: "tok", UUID: "uuid-1", Expire: 12345}}, nil)

	body := []byte(`{"token":"tok","expire":0}`)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	UserHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	users := readUsersFromFile(t)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Expire != 0 {
		t.Fatalf("expected expire reset to 0, got %d", users[0].Expire)
	}
}

func TestNodeHandler_RejectsDuplicateNameOnCreate(t *testing.T) {
	withTempDataDir(t, nil, []model.Node{{Name: "n1", Server: "a.example", Port: 443}})

	body := []byte(`{"name":"n1","server":"b.example","port":8443}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/nodes", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	NodeHandler(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d body=%s", rec.Code, rec.Body.String())
	}

	nodes := readNodesFromFile(t)
	if len(nodes) != 1 {
		t.Fatalf("expected node list unchanged with 1 entry, got %d", len(nodes))
	}
}

func TestSubHandler_DoesNotRecordStatsForUnsupportedFormat(t *testing.T) {
	withTempDataDir(t, []model.User{{Name: "u", Token: "tok", UUID: "uuid-1", Expire: 0}}, []model.Node{{Name: "n1", Server: "a.example", Port: 443}})

	before := stats.Get()
	req := httptest.NewRequest(http.MethodGet, "/api/unknown/tok", nil)
	rec := httptest.NewRecorder()

	SubHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	after := stats.Get()
	if after.RequestCount != before.RequestCount {
		t.Fatalf("expected request_count unchanged, before=%d after=%d", before.RequestCount, after.RequestCount)
	}
	if after.ByFormat["unknown"] != before.ByFormat["unknown"] {
		t.Fatalf("expected unknown format counter unchanged, before=%d after=%d", before.ByFormat["unknown"], after.ByFormat["unknown"])
	}
}

func withTempDataDir(t *testing.T, users []model.User, nodes []model.Node) {
	t.Helper()
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	if users == nil {
		users = []model.User{}
	}
	if nodes == nil {
		nodes = []model.Node{}
	}
	mustWriteJSON(t, filepath.Join(dataDir, "users.json"), users)
	mustWriteJSON(t, filepath.Join(dataDir, "nodes.json"), nodes)
	if err := os.WriteFile(filepath.Join(dataDir, "clash.yaml"), []byte("proxies: []\n"), 0o644); err != nil {
		t.Fatalf("write clash template: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
}

func mustWriteJSON(t *testing.T, path string, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func readUsersFromFile(t *testing.T) []model.User {
	t.Helper()
	b, err := os.ReadFile("data/users.json")
	if err != nil {
		t.Fatalf("read users: %v", err)
	}
	var users []model.User
	if err := json.Unmarshal(b, &users); err != nil {
		t.Fatalf("unmarshal users: %v", err)
	}
	return users
}

func readNodesFromFile(t *testing.T) []model.Node {
	t.Helper()
	b, err := os.ReadFile("data/nodes.json")
	if err != nil {
		t.Fatalf("read nodes: %v", err)
	}
	var nodes []model.Node
	if err := json.Unmarshal(b, &nodes); err != nil {
		t.Fatalf("unmarshal nodes: %v", err)
	}
	return nodes
}

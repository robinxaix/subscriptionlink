package xray

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"subscriptionlink/internal/model"
)

func SyncUsers(users []model.User) error {
	configPath := os.Getenv("XRAY_CONFIG_PATH")
	if configPath == "" {
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read xray config: %w", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse xray config: %w", err)
	}

	inboundsAny, ok := cfg["inbounds"].([]interface{})
	if !ok {
		return fmt.Errorf("xray config missing inbounds")
	}

	clients := buildClients(users)
	updatedCount, err := replaceInboundClients(inboundsAny, os.Getenv("XRAY_INBOUND_TAG"), clients)
	if err != nil {
		return err
	}
	if updatedCount == 0 {
		return fmt.Errorf("no matching inbound to update clients")
	}

	updated, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal xray config: %w", err)
	}

	if err := atomicWrite(configPath, updated); err != nil {
		return fmt.Errorf("write xray config: %w", err)
	}

	reloadCmd, exists := os.LookupEnv("XRAY_RELOAD_CMD")
	reloadCmd = strings.TrimSpace(reloadCmd)
	if !exists {
		reloadCmd = "sudo systemctl reload xray"
	}
	if reloadCmd != "" {
		cmd := exec.Command("sh", "-c", reloadCmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("reload xray failed: %v (%s)", err, strings.TrimSpace(string(out)))
		}
	}

	return nil
}

func replaceInboundClients(inbounds []interface{}, tag string, clients []interface{}) (int, error) {
	updated := 0

	if tag != "" {
		for _, it := range inbounds {
			m, _ := it.(map[string]interface{})
			if m == nil {
				continue
			}
			if tagVal, _ := m["tag"].(string); tagVal == tag {
				replaceClientsOnInbound(m, clients)
				updated++
				return updated, nil
			}
		}
		return 0, fmt.Errorf("xray inbound tag not found: %s", tag)
	}

	for _, it := range inbounds {
		m, _ := it.(map[string]interface{})
		if m == nil {
			continue
		}
		if protocol, _ := m["protocol"].(string); strings.EqualFold(protocol, "vless") {
			replaceClientsOnInbound(m, clients)
			updated++
		}
	}

	if updated == 0 {
		return 0, fmt.Errorf("no vless inbound found in xray config")
	}

	return updated, nil
}

func replaceClientsOnInbound(inbound map[string]interface{}, clients []interface{}) {
	settings, _ := inbound["settings"].(map[string]interface{})
	if settings == nil {
		settings = map[string]interface{}{}
	}
	settings["clients"] = clients
	inbound["settings"] = settings
}

func buildClients(users []model.User) []interface{} {
	seen := make(map[string]struct{})
	keys := make([]string, 0, len(users))
	clientByID := make(map[string]map[string]interface{})

	for _, u := range users {
		if strings.TrimSpace(u.UUID) == "" {
			continue
		}
		email := strings.TrimSpace(u.Email)
		if email == "" {
			email = fallbackEmail(u.Name, u.Token)
		}
		id := strings.TrimSpace(u.UUID)
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		keys = append(keys, id)
		clientByID[id] = map[string]interface{}{
			"id":    id,
			"email": email,
		}
	}

	sort.Strings(keys)
	clients := make([]interface{}, 0, len(keys))
	for _, k := range keys {
		clients = append(clients, clientByID[k])
	}
	return clients
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

func atomicWrite(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "xray-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	defer func() {
		tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(content); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

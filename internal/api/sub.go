package api

import (
	"net/http"
	"os"
	"strings"
	"time"

	"subscriptionlink/internal/generator"
	"subscriptionlink/internal/model"
	"subscriptionlink/internal/stats"
	"subscriptionlink/internal/store"
)

func SubHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(path) < 3 || path[0] != "api" {
		http.Error(w, "invalid path", http.StatusNotFound)
		return
	}
	format := path[1]
	token := path[2]

	users := store.LoadUsers()
	var user *model.User
	for i := range users {
		if users[i].Token == token {
			user = &users[i]
			break
		}
	}
	if user == nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if user.Expire > 0 && user.Expire < time.Now().Unix() {
		http.Error(w, "token expired", http.StatusForbidden)
		return
	}

	nodes := store.LoadNodes()

	switch format {
	case "subscription":
		stats.Record(format, token)
		if len(nodes) == 0 {
			http.Error(w, "no available nodes", http.StatusServiceUnavailable)
			return
		}
		templateBytes, err := os.ReadFile(store.DataFile("clash.yaml"))
		if err != nil {
			http.Error(w, "clash template not found", http.StatusServiceUnavailable)
			return
		}
		selectedNode := pickNode(nodes, token)
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		_, _ = w.Write([]byte(generator.ClashSub(string(templateBytes), *user, selectedNode)))
	case "v2ray":
		stats.Record(format, token)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(generator.V2raySub(*user, nodes)))
	case "singbox":
		stats.Record(format, token)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(generator.SingboxSub(*user, nodes)))
	default:
		http.Error(w, "unsupported format", http.StatusNotFound)
	}
}

func pickNode(nodes []model.Node, key string) model.Node {
	if len(nodes) == 1 {
		return nodes[0]
	}
	var sum int
	for _, c := range []byte(key) {
		sum += int(c)
	}
	return nodes[sum%len(nodes)]
}

package api

import (
	"encoding/json"
	"net/http"

	"subscriptionlink/internal/model"
	"subscriptionlink/internal/store"
)

func normalizeNodeDefaults(n *model.Node) {
	if n.Protocol == "" {
		n.Protocol = "vless"
	}
	if n.Network == "" {
		n.Network = "ws"
	}
	if n.Security == "" {
		n.Security = "none"
	}
	if n.Path == "" {
		n.Path = "/xhttp"
	}
}

func NodeHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(store.LoadNodes())
	case http.MethodPost:
		var n model.Node
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if n.Name == "" || n.Server == "" || n.Port <= 0 {
			http.Error(w, "name/server/port are required", http.StatusBadRequest)
			return
		}
		normalizeNodeDefaults(&n)
		nodes := store.LoadNodes()
		for _, existed := range nodes {
			if existed.Name == n.Name {
				http.Error(w, "node name already exists", http.StatusConflict)
				return
			}
		}
		nodes = append(nodes, n)
		store.SaveNodes(nodes)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(n)
	case http.MethodPut:
		var in model.Node
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if in.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		nodes := store.LoadNodes()
		for i := range nodes {
			if nodes[i].Name != in.Name {
				continue
			}
			if in.Server != "" {
				nodes[i].Server = in.Server
			}
			if in.Port > 0 {
				nodes[i].Port = in.Port
			}
			if in.Protocol != "" {
				nodes[i].Protocol = in.Protocol
			}
			if in.Network != "" {
				nodes[i].Network = in.Network
			}
			if in.Security != "" {
				nodes[i].Security = in.Security
			}
			if in.Path != "" {
				nodes[i].Path = in.Path
			}
			if in.Host != "" {
				nodes[i].Host = in.Host
			}
			normalizeNodeDefaults(&nodes[i])
			store.SaveNodes(nodes)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(nodes[i])
			return
		}
		http.Error(w, "node not found", http.StatusNotFound)
	case http.MethodDelete:
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		nodes := store.LoadNodes()
		for i := range nodes {
			if nodes[i].Name == name {
				nodes = append(nodes[:i], nodes[i+1:]...)
				store.SaveNodes(nodes)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		http.Error(w, "node not found", http.StatusNotFound)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

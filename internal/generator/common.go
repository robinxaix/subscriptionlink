package generator

import (
	"fmt"
	"net/url"
	"strings"

	"subscriptionlink/internal/model"
)

func normalizeNode(n model.Node) model.Node {
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
	return n
}

func useTLS(security string) bool {
	switch strings.ToLower(security) {
	case "tls", "xtls", "reality":
		return true
	default:
		return false
	}
}

func buildVLESSLink(user model.User, node model.Node) string {
	n := normalizeNode(node)
	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("type", n.Network)
	q.Set("security", n.Security)
	if n.Path != "" {
		q.Set("path", n.Path)
	}
	if n.Host != "" {
		q.Set("host", n.Host)
	}
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", user.UUID, n.Server, n.Port, q.Encode(), url.QueryEscape(n.Name))
}

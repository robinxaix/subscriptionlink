package generator

import (
	"encoding/base64"
	"strings"

	"subscriptionlink/internal/model"
)

func V2raySub(user model.User, nodes []model.Node) string {
	var links []string
	for _, n := range nodes {
		links = append(links, buildVLESSLink(user, n))
	}
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(links, "\n")))
}

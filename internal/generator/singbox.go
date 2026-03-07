package generator

import (
	"strings"

	"subscriptionlink/internal/model"
)

func SingboxSub(user model.User, nodes []model.Node) string {
	var links []string
	for _, n := range nodes {
		links = append(links, buildVLESSLink(user, n))
	}
	return strings.TrimSpace(strings.Join(links, "\n"))
}

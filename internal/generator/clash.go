package generator

import (
	"strconv"
	"strings"

	"subscriptionlink/internal/model"
)

func ClashSub(template string, user model.User, node model.Node) string {
	n := normalizeNode(node)
	replacer := strings.NewReplacer(
		"{{UUID}}", user.UUID,
		"{{SERVER}}", n.Server,
		"{{PORT}}", strconv.Itoa(n.Port),
		"{{NODE_NAME}}", n.Name,
		"{{NETWORK}}", n.Network,
		"{{PATH}}", n.Path,
		"{{HOST}}", n.Host,
		"{{TLS}}", "true",
	)
	return replacer.Replace(template)
}

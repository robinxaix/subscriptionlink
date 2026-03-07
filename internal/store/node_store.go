package store

import (
	"encoding/json"
	"os"
	"subscriptionlink/internal/model"
)

func LoadNodes() []model.Node {
	data, _ := os.ReadFile("data/nodes.json")
	var nodes []model.Node
	json.Unmarshal(data, &nodes)
	if nodes == nil {
		nodes = []model.Node{}
	}
	return nodes
}

func SaveNodes(nodes []model.Node) {
	data, _ := json.MarshalIndent(nodes, "", "  ")
	os.WriteFile("data/nodes.json", data, 0644)
}

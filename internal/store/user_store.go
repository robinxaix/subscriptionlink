package store

import (
	"encoding/json"
	"os"
	"subscriptionlink/internal/model"
)

func LoadUsers() []model.User {
	data, _ := os.ReadFile(DataFile("users.json"))
	var users []model.User
	json.Unmarshal(data, &users)
	if users == nil {
		users = []model.User{}
	}
	return users
}

func SaveUsers(users []model.User) {
	data, _ := json.MarshalIndent(users, "", "  ")
	os.WriteFile(DataFile("users.json"), data, 0644)
}

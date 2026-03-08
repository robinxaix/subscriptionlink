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

// MergeUsers merges xray clients into existing users
// Users with matching UUID are preserved, new clients from xray are added
// Email from xray config is merged if existing user has no email
func MergeUsers(existingUsers, xrayUsers []model.User) []model.User {
	existingByUUID := make(map[string]model.User)
	for _, u := range existingUsers {
		if u.UUID != "" {
			existingByUUID[u.UUID] = u
		}
	}

	var result []model.User
	seen := make(map[string]bool)

	// First add all xray users, merging with existing data
	for _, xu := range xrayUsers {
		if xu.UUID == "" || seen[xu.UUID] {
			continue
		}
		seen[xu.UUID] = true

		if eu, ok := existingByUUID[xu.UUID]; ok {
			// Use existing user data but merge email from xray if missing
			if eu.Email == "" && xu.Email != "" {
				eu.Email = xu.Email
			}
			result = append(result, eu)
		} else {
			// New user from xray config
			result = append(result, xu)
		}
	}

	// Then add any existing users not in xray config (they will be synced later)
	for _, eu := range existingUsers {
		if eu.UUID != "" && !seen[eu.UUID] {
			result = append(result, eu)
		}
	}

	return result
}

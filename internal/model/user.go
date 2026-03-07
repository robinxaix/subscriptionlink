package model

type User struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Token  string `json:"token"`
	UUID   string `json:"uuid"`
	Expire int64  `json:"expire"`
}

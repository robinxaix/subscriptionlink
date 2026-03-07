package model

type Node struct {
	Name     string `json:"name"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Network  string `json:"network"`
	Security string `json:"security"`
	Path     string `json:"path"`
	Host     string `json:"host"`
}

package store

import (
	"path/filepath"
	"strings"
)

var dataDir = "data"

func SetDataDir(dir string) {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		dataDir = "data"
		return
	}
	dataDir = trimmed
}

func DataDir() string {
	return dataDir
}

func DataFile(name string) string {
	return filepath.Join(dataDir, name)
}

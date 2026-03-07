package stats

import (
	"sync"
	"time"
)

type Snapshot struct {
	RequestCount int64            `json:"request_count"`
	ByFormat     map[string]int64 `json:"by_format"`
	ByToken      map[string]int64 `json:"by_token"`
	LastAccess   int64            `json:"last_access"`
}

var (
	mu sync.Mutex
	s  = Snapshot{
		ByFormat: make(map[string]int64),
		ByToken:  make(map[string]int64),
	}
)

func Record(format, token string) {
	mu.Lock()
	defer mu.Unlock()

	s.RequestCount++
	s.ByFormat[format]++
	s.ByToken[token]++
	s.LastAccess = time.Now().Unix()
}

func Get() Snapshot {
	mu.Lock()
	defer mu.Unlock()

	out := Snapshot{
		RequestCount: s.RequestCount,
		LastAccess:   s.LastAccess,
		ByFormat:     make(map[string]int64, len(s.ByFormat)),
		ByToken:      make(map[string]int64, len(s.ByToken)),
	}
	for k, v := range s.ByFormat {
		out.ByFormat[k] = v
	}
	for k, v := range s.ByToken {
		out.ByToken[k] = v
	}
	return out
}

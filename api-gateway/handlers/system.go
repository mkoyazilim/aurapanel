package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"strings"
)

// SystemStatus holds the master status of the node
type SystemStatus struct {
	OS            string `json:"os"`
	Architecture  string `json:"architecture"`
	GoVersion     string `json:"go_version"`
	Goroutines    int    `json:"goroutines"`
	Status        string `json:"status"`
}

// GetSystemStatus info
func GetSystemStatus(w http.ResponseWriter, r *http.Request) {
	status := SystemStatus{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		GoVersion:    runtime.Version(),
		Goroutines:   runtime.NumGoroutine(),
		Status:       "operational",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// sensitiveEnvPatterns contains substrings that identify environment variables
// which must never be exposed via the debug env handler.
var sensitiveEnvPatterns = []string{
	"SECRET", "PASSWORD", "PASS", "TOKEN", "KEY", "CREDENTIAL",
	"DSN", "DATABASE_URL", "AUTH", "ENCRYPT", "SALT", "PRIVATE",
}

func isSensitiveEnv(key string) bool {
	upper := strings.ToUpper(key)
	for _, pattern := range sensitiveEnvPatterns {
		if strings.Contains(upper, pattern) {
			return true
		}
	}
	return false
}

// GetEnv returns a filtered view of the process environment.
// Variables matching sensitive patterns (SECRET, PASSWORD, TOKEN, KEY, etc.)
// are redacted to prevent credential exposure through the API.
func GetEnv(w http.ResponseWriter, r *http.Request) {
	raw := os.Environ()
	filtered := make([]string, 0, len(raw))
	for _, entry := range raw {
		if idx := strings.IndexByte(entry, '='); idx >= 0 {
			key := entry[:idx]
			if isSensitiveEnv(key) {
				filtered = append(filtered, key+"=***REDACTED***")
				continue
			}
		}
		filtered = append(filtered, entry)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

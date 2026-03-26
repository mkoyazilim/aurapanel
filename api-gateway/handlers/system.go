package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
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

// GetEnv is just a debug handler
func GetEnv(w http.ResponseWriter, r *http.Request) {
    env := os.Environ()
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(env)
}

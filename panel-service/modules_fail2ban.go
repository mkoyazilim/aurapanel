package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

func (s *service) handleFail2banList(w http.ResponseWriter) {
	cmd := exec.Command("fail2ban-client", "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: map[string]interface{}{"status": "not installed or inactive", "raw": string(output)}})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Data: map[string]interface{}{"status": "active", "raw": string(output)}})
}

func (s *service) handleFail2banUnban(w http.ResponseWriter, r *http.Request) {
	ip := strings.TrimSpace(r.URL.Query().Get("ip"))
	if ip == "" {
		writeError(w, http.StatusBadRequest, "IP is required.")
		return
	}
	cmd := exec.Command("fail2ban-client", "unban", ip)
	if output, err := cmd.CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to unban IP: %s", string(output)))
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Status: "success", Message: fmt.Sprintf("IP %s unbanned successfully.", ip)})
}
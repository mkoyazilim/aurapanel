package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type ResellerTokenResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Token   string `json:"token"`
}

type UpdateResellerTokenRequest struct {
	Token string `json:"token"`
}

func resellerTokenFilePath() string {
	stateDir := strings.TrimSpace(os.Getenv("AURAPANEL_STATE_DIR"))
	if stateDir == "" {
		stateDir = "/var/lib/aurapanel"
	}
	return filepath.Join(stateDir, "reseller.token")
}

func GetResellerToken(w http.ResponseWriter, r *http.Request) {
	tokenFile := resellerTokenFilePath()
	var token string
	if b, err := os.ReadFile(tokenFile); err == nil {
		token = strings.TrimSpace(string(b))
	} else {
		token = strings.TrimSpace(os.Getenv("AURAPANEL_RESELLER_TOKEN"))
	}

	writeJSON(w, http.StatusOK, ResellerTokenResponse{
		Status:  "success",
		Message: "Token retrieved successfully",
		Token:   token,
	})
}

func UpdateResellerToken(w http.ResponseWriter, r *http.Request) {
	var req UpdateResellerTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
		return
	}

	tokenFile := resellerTokenFilePath()
	if err := os.MkdirAll(filepath.Dir(tokenFile), 0750); err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Could not create state directory"})
		return
	}

	if err := os.WriteFile(tokenFile, []byte(strings.TrimSpace(req.Token)), 0600); err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Could not save token"})
		return
	}

	writeJSON(w, http.StatusOK, BaseResponse{Status: "success", Message: "Token updated successfully"})
}

func DeleteResellerToken(w http.ResponseWriter, r *http.Request) {
	tokenFile := resellerTokenFilePath()
	if err := os.Remove(tokenFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Could not delete token"})
		return
	}

	writeJSON(w, http.StatusOK, BaseResponse{Status: "success", Message: "Token deleted successfully"})
}

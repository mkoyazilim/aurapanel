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

func GetResellerToken(w http.ResponseWriter, r *http.Request) {
	tokenFile := "data/reseller.token"
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

	tokenFile := "data/reseller.token"
	if err := os.MkdirAll(filepath.Dir(tokenFile), 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Could not create data directory"})
		return
	}

	if err := os.WriteFile(tokenFile, []byte(strings.TrimSpace(req.Token)), 0600); err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Could not save token"})
		return
	}

	writeJSON(w, http.StatusOK, BaseResponse{Status: "success", Message: "Token updated successfully"})
}

func DeleteResellerToken(w http.ResponseWriter, r *http.Request) {
	tokenFile := "data/reseller.token"
	if err := os.Remove(tokenFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Could not delete token"})
		return
	}

	writeJSON(w, http.StatusOK, BaseResponse{Status: "success", Message: "Token deleted successfully"})
}

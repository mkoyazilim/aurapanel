package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type BaseResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ListWebsites
func ListWebsites(w http.ResponseWriter, r *http.Request) {
	sites := []map[string]interface{}{
		{"id": 1, "domain": "example.com", "php": "8.1", "status": "active"},
		{"id": 2, "domain": "test.com", "php": "8.3", "status": "active"},
	}
	
	resp := BaseResponse{Status: "success", Message: "Websites retrieved", Data: sites}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// CreateWebsite
func CreateWebsite(w http.ResponseWriter, r *http.Request) {
	// Parse body
	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Prepare payload for Rust Core
	// e.g. {"domain": "example.com", "user": "admin", "php_version": "8.1"}
	payloadBytes, _ := json.Marshal(reqBody)
	
	respRust, err := http.Post("http://127.0.0.1:3000/vhost", "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		resp := BaseResponse{Status: "error", Message: "Core API ile iletişim kurulamadı: " + err.Error()}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}
	defer respRust.Body.Close()

	var rustResult map[string]interface{}
	json.NewDecoder(respRust.Body).Decode(&rustResult)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rustResult)
}

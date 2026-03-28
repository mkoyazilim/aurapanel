package controllers

import (
	"bytes"
	"io"
	"net/http"
)

type BaseResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func forwardService(w http.ResponseWriter, r *http.Request, method, path string, body io.Reader) {
	req, err := http.NewRequest(method, serviceBaseURL()+path, body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	if auth := r.Header.Get("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, BaseResponse{Status: "error", Message: "Service API request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// ListWebsites proxies website list to the panel service.
func ListWebsites(w http.ResponseWriter, r *http.Request) {
	forwardService(w, r, http.MethodGet, "/api/v1/vhost/list", nil)
}

// CreateWebsite proxies website creation to the panel service.
func CreateWebsite(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "invalid body"})
		return
	}
	forwardService(w, r, http.MethodPost, "/api/v1/vhost", bytes.NewReader(payload))
}

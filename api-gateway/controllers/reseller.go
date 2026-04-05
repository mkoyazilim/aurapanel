package controllers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
)

type ResellerCreateAccountReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Domain   string `json:"domain"`
	Package  string `json:"package"`
}

func doServiceRequest(method, path string, payload interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, serviceBaseURL()+path, bodyReader)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Aura-Proxy-Token", strings.TrimSpace(os.Getenv("AURAPANEL_INTERNAL_PROXY_TOKEN")))
	req.Header.Set("X-Aura-Auth-Email", "reseller@aurapanel.local")
	req.Header.Set("X-Aura-Auth-Role", "admin")
	req.Header.Set("X-Aura-Auth-Username", "reseller_api")

	return http.DefaultClient.Do(req)
}

func ResellerCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req ResellerCreateAccountReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
		return
	}

	// 1. Create User
	userPayload := map[string]interface{}{
		"username": req.Username,
		"email":    req.Email,
		"password": req.Password,
		"role":     "user",
		"package":  req.Package,
	}
	respUser, err := doServiceRequest(http.MethodPost, "/api/v1/users/create", userPayload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	defer respUser.Body.Close()

	if respUser.StatusCode != http.StatusOK && respUser.StatusCode != http.StatusCreated {
		var errResp BaseResponse
		_ = json.NewDecoder(respUser.Body).Decode(&errResp)
		writeJSON(w, respUser.StatusCode, errResp)
		return
	}

	// 2. Create Website (vhost)
	vhostPayload := map[string]interface{}{
		"domain":      req.Domain,
		"username":    req.Username,
		"php_version": "8.1", // Default PHP version for now
	}
	respVhost, err := doServiceRequest(http.MethodPost, "/api/v1/vhost", vhostPayload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	defer respVhost.Body.Close()

	if respVhost.StatusCode != http.StatusOK && respVhost.StatusCode != http.StatusCreated {
		var errResp BaseResponse
		_ = json.NewDecoder(respVhost.Body).Decode(&errResp)
		writeJSON(w, respVhost.StatusCode, errResp)
		return
	}

	writeJSON(w, http.StatusOK, BaseResponse{
		Status:  "success",
		Message: "Account and website created successfully",
		Data: map[string]string{
			"ns1": "ns1.aurapanel.info",
			"ns2": "ns2.aurapanel.info",
		},
	})
}

func ResellerSuspendAccount(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	
	payload := map[string]interface{}{
		"username": req["username"],
		"active":   false,
	}
	resp, err := doServiceRequest(http.MethodPost, "/api/v1/users/update", payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	
	var apiResp BaseResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiResp)
	writeJSON(w, resp.StatusCode, apiResp)
}

func ResellerUnsuspendAccount(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	
	payload := map[string]interface{}{
		"username": req["username"],
		"active":   true,
	}
	resp, err := doServiceRequest(http.MethodPost, "/api/v1/users/update", payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	
	var apiResp BaseResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiResp)
	writeJSON(w, resp.StatusCode, apiResp)
}

func ResellerTerminateAccount(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	
	payload := map[string]interface{}{
		"username": req["username"],
	}
	resp, err := doServiceRequest(http.MethodPost, "/api/v1/users/delete", payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	
	var apiResp BaseResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiResp)
	writeJSON(w, resp.StatusCode, apiResp)
}

func ResellerChangePassword(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	
	payload := map[string]interface{}{
		"username": req["username"],
		"password": req["password"],
	}
	resp, err := doServiceRequest(http.MethodPost, "/api/v1/users/update", payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	
	var apiResp BaseResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiResp)
	writeJSON(w, resp.StatusCode, apiResp)
}

func ResellerChangePackage(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	
	payload := map[string]interface{}{
		"username": req["username"],
		"package":  req["package"],
	}
	resp, err := doServiceRequest(http.MethodPost, "/api/v1/users/update", payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	
	var apiResp BaseResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiResp)
	writeJSON(w, resp.StatusCode, apiResp)
}

func ResellerListPackages(w http.ResponseWriter, r *http.Request) {
	resp, err := doServiceRequest(http.MethodGet, "/api/v1/packages/list", nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	
	var apiResp BaseResponse
	_ = json.NewDecoder(resp.Body).Decode(&apiResp)
	writeJSON(w, resp.StatusCode, apiResp)
}

func ResellerSSO(w http.ResponseWriter, r *http.Request) {
	// Not fully implemented yet in panel-service, placeholder for future
	writeJSON(w, http.StatusOK, BaseResponse{
		Status:  "success",
		Message: "SSO endpoint reached",
		Data: map[string]string{
			"url": "/",
		},
	})
}

package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aurapanel/api-gateway/middleware"
	"github.com/golang-jwt/jwt/v5"
)

type ResellerCreateAccountReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Domain   string `json:"domain"`
	Package  string `json:"package"`
}

type resellerAccountMutationReq struct {
	Username string `json:"username"`
	Domain   string `json:"domain"`
	Package  string `json:"package"`
	Password string `json:"password"`
}

type resellerSSORequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Redirect string `json:"redirect"`
}

type vhostItem struct {
	Domain string `json:"domain"`
	Owner  string `json:"owner"`
}

type vhostListResponse struct {
	Status     string      `json:"status"`
	Message    string      `json:"message"`
	Data       []vhostItem `json:"data"`
	Pagination struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
	} `json:"pagination"`
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

func sanitizeResellerValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parseServiceResponse(resp *http.Response) BaseResponse {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return BaseResponse{Status: "error", Message: "Failed to read service response"}
	}
	if len(body) == 0 {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return BaseResponse{Status: "success"}
		}
		return BaseResponse{Status: "error", Message: "Empty service response"}
	}

	var apiResp BaseResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = "Invalid service response payload"
		}
		return BaseResponse{Status: "error", Message: message}
	}

	if apiResp.Status == "" {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			apiResp.Status = "success"
		} else {
			apiResp.Status = "error"
		}
	}

	return apiResp
}

func callService(method, path string, payload interface{}) (int, BaseResponse, error) {
	resp, err := doServiceRequest(method, path, payload)
	if err != nil {
		return 0, BaseResponse{}, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, parseServiceResponse(resp), nil
}

func isSuccessStatus(statusCode int, apiResp BaseResponse) bool {
	return statusCode >= 200 && statusCode < 300 && strings.EqualFold(strings.TrimSpace(apiResp.Status), "success")
}

func resolveOwnerDomains(owner string) ([]string, error) {
	owner = sanitizeResellerValue(owner)
	if owner == "" {
		return nil, nil
	}

	seen := make(map[string]struct{})
	domains := make([]string, 0, 4)

	page := 1
	for {
		path := fmt.Sprintf("/api/v1/vhost/list?search=%s&page=%d&per_page=200", url.QueryEscape(owner), page)
		resp, err := doServiceRequest(http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}

		var parsed vhostListResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("invalid vhost list response")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 || !strings.EqualFold(parsed.Status, "success") {
			msg := strings.TrimSpace(parsed.Message)
			if msg == "" {
				msg = "failed to list websites"
			}
			return nil, fmt.Errorf(msg)
		}

		for _, site := range parsed.Data {
			domain := strings.TrimSpace(site.Domain)
			if domain == "" {
				continue
			}
			if sanitizeResellerValue(site.Owner) != owner {
				continue
			}
			if _, ok := seen[domain]; ok {
				continue
			}
			seen[domain] = struct{}{}
			domains = append(domains, domain)
		}

		totalPages := parsed.Pagination.TotalPages
		if totalPages < 1 || page >= totalPages || len(parsed.Data) == 0 {
			break
		}
		page++
	}

	return domains, nil
}

func resolveDomainsForMutation(req resellerAccountMutationReq) ([]string, error) {
	domain := strings.TrimSpace(req.Domain)
	if domain != "" {
		return []string{domain}, nil
	}
	return resolveOwnerDomains(req.Username)
}

func applyDomainAction(domains []string, path string, extra map[string]interface{}) error {
	failed := make([]string, 0)
	for _, domain := range domains {
		payload := map[string]interface{}{"domain": domain}
		for key, value := range extra {
			payload[key] = value
		}

		statusCode, apiResp, err := callService(http.MethodPost, path, payload)
		if err != nil || !isSuccessStatus(statusCode, apiResp) {
			failed = append(failed, domain)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("domain actions failed: %s", strings.Join(failed, ", "))
	}
	return nil
}

func nsValue(envKey, fallback string) string {
	value := strings.TrimSpace(os.Getenv(envKey))
	if value == "" {
		return fallback
	}
	return value
}

func issueResellerSSOToken(user User, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := gatewayClaims{
		Email:    user.Email,
		Name:     user.Name,
		Role:     user.Role,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.Email,
			Issuer:    middleware.JwtIssuer(),
			Audience:  jwt.ClaimStrings{middleware.JwtAudience()},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(middleware.JwtSecret()))
}

func requestScheme(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		if idx := strings.Index(forwarded, ","); idx >= 0 {
			forwarded = forwarded[:idx]
		}
		if strings.EqualFold(strings.TrimSpace(forwarded), "https") {
			return "https"
		}
		return "http"
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func normalizeRedirectPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/"
	}
	if strings.HasPrefix(strings.ToLower(value), "http://") || strings.HasPrefix(strings.ToLower(value), "https://") {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return value
}

func authCookieName() string {
	name := strings.TrimSpace(os.Getenv("AURAPANEL_AUTH_COOKIE_NAME"))
	if name == "" {
		return "aurapanel_session"
	}
	return name
}

func requestIsSecure(r *http.Request) bool {
	return requestScheme(r) == "https"
}

func setResellerSSOCookie(w http.ResponseWriter, r *http.Request, token string, exp time.Time) {
	maxAge := int(time.Until(exp).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName(),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Expires:  exp,
	})
}

func ResellerCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req ResellerCreateAccountReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)
	req.Domain = strings.TrimSpace(req.Domain)
	req.Package = strings.TrimSpace(req.Package)

	if req.Username == "" || req.Email == "" || req.Password == "" || req.Domain == "" {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "username, email, password and domain are required"})
		return
	}

	userPayload := map[string]interface{}{
		"username": req.Username,
		"email":    req.Email,
		"password": req.Password,
		"role":     "user",
		"package":  firstNonEmpty(req.Package, "default"),
	}
	respUserStatus, respUserBody, err := callService(http.MethodPost, "/api/v1/users/create", userPayload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	if !isSuccessStatus(respUserStatus, respUserBody) {
		writeJSON(w, respUserStatus, respUserBody)
		return
	}

	vhostPayload := map[string]interface{}{
		"domain":      req.Domain,
		"owner":       req.Username,
		"user":        req.Username,
		"php_version": "8.3",
		"package":     firstNonEmpty(req.Package, "default"),
		"email":       req.Email,
	}
	respVhostStatus, respVhostBody, err := callService(http.MethodPost, "/api/v1/vhost", vhostPayload)
	if err != nil || !isSuccessStatus(respVhostStatus, respVhostBody) {
		_, _, _ = callService(http.MethodPost, "/api/v1/users/delete", map[string]interface{}{"username": req.Username})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Website provisioning failed and account rollback attempted: " + err.Error()})
			return
		}
		if respVhostBody.Message == "" {
			respVhostBody.Message = "Website provisioning failed and user rollback was executed"
		}
		writeJSON(w, respVhostStatus, respVhostBody)
		return
	}

	writeJSON(w, http.StatusOK, BaseResponse{
		Status:  "success",
		Message: "Account and website created successfully",
		Data: map[string]string{
			"ns1": nsValue("AURAPANEL_RESELLER_NS1", "ns1.aurapanel.info"),
			"ns2": nsValue("AURAPANEL_RESELLER_NS2", "ns2.aurapanel.info"),
		},
	})
}

func ResellerSuspendAccount(w http.ResponseWriter, r *http.Request) {
	var req resellerAccountMutationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "username is required"})
		return
	}

	statusCode, apiResp, err := callService(http.MethodPost, "/api/v1/users/update", map[string]interface{}{
		"username": req.Username,
		"active":   false,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	if !isSuccessStatus(statusCode, apiResp) {
		writeJSON(w, statusCode, apiResp)
		return
	}

	domains, err := resolveDomainsForMutation(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, BaseResponse{Status: "error", Message: err.Error()})
		return
	}
	if err := applyDomainAction(domains, "/api/v1/vhost/suspend", nil); err != nil {
		writeJSON(w, http.StatusBadGateway, BaseResponse{Status: "error", Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, BaseResponse{Status: "success", Message: "Account suspended"})
}

func ResellerUnsuspendAccount(w http.ResponseWriter, r *http.Request) {
	var req resellerAccountMutationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "username is required"})
		return
	}

	statusCode, apiResp, err := callService(http.MethodPost, "/api/v1/users/update", map[string]interface{}{
		"username": req.Username,
		"active":   true,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	if !isSuccessStatus(statusCode, apiResp) {
		writeJSON(w, statusCode, apiResp)
		return
	}

	domains, err := resolveDomainsForMutation(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, BaseResponse{Status: "error", Message: err.Error()})
		return
	}
	if err := applyDomainAction(domains, "/api/v1/vhost/unsuspend", nil); err != nil {
		writeJSON(w, http.StatusBadGateway, BaseResponse{Status: "error", Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, BaseResponse{Status: "success", Message: "Account unsuspended"})
}

func ResellerTerminateAccount(w http.ResponseWriter, r *http.Request) {
	var req resellerAccountMutationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "username is required"})
		return
	}

	domains, err := resolveDomainsForMutation(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, BaseResponse{Status: "error", Message: err.Error()})
		return
	}
	if err := applyDomainAction(domains, "/api/v1/vhost/delete", nil); err != nil {
		writeJSON(w, http.StatusBadGateway, BaseResponse{Status: "error", Message: err.Error()})
		return
	}

	statusCode, apiResp, err := callService(http.MethodPost, "/api/v1/users/delete", map[string]interface{}{
		"username": req.Username,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	if !isSuccessStatus(statusCode, apiResp) {
		writeJSON(w, statusCode, apiResp)
		return
	}

	writeJSON(w, http.StatusOK, BaseResponse{Status: "success", Message: "Account terminated"})
}

func ResellerChangePassword(w http.ResponseWriter, r *http.Request) {
	var req resellerAccountMutationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
		return
	}

	username := strings.TrimSpace(req.Username)
	newPassword := strings.TrimSpace(req.Password)
	if username == "" || newPassword == "" {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "username and password are required"})
		return
	}

	statusCode, apiResp, err := callService(http.MethodPost, "/api/v1/users/change-password", map[string]interface{}{
		"username":     username,
		"new_password": newPassword,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	if !isSuccessStatus(statusCode, apiResp) {
		writeJSON(w, statusCode, apiResp)
		return
	}

	writeJSON(w, http.StatusOK, BaseResponse{Status: "success", Message: "Password updated"})
}

func ResellerChangePackage(w http.ResponseWriter, r *http.Request) {
	var req resellerAccountMutationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
		return
	}

	username := strings.TrimSpace(req.Username)
	pkg := strings.TrimSpace(req.Package)
	if username == "" || pkg == "" {
		writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "username and package are required"})
		return
	}

	statusCode, apiResp, err := callService(http.MethodPost, "/api/v1/users/update", map[string]interface{}{
		"username": username,
		"package":  pkg,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}
	if !isSuccessStatus(statusCode, apiResp) {
		writeJSON(w, statusCode, apiResp)
		return
	}

	domains, err := resolveDomainsForMutation(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, BaseResponse{Status: "error", Message: err.Error()})
		return
	}
	if err := applyDomainAction(domains, "/api/v1/vhost/update", map[string]interface{}{
		"package": pkg,
		"owner":   username,
		"user":    username,
	}); err != nil {
		writeJSON(w, http.StatusBadGateway, BaseResponse{Status: "error", Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, BaseResponse{Status: "success", Message: "Package updated"})
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

func ResellerListVhosts(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	page, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page")))
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("per_page")))
	if err != nil || perPage < 1 {
		perPage = 20
	}
	if perPage > 200 {
		perPage = 200
	}

	params := url.Values{}
	params.Set("page", strconv.Itoa(page))
	params.Set("per_page", strconv.Itoa(perPage))
	if search != "" {
		params.Set("search", search)
	}

	path := "/api/v1/vhost/list?" + params.Encode()
	statusCode, apiResp, err := callService(http.MethodGet, path, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Service error: " + err.Error()})
		return
	}

	writeJSON(w, statusCode, apiResp)
}

func ResellerSSO(w http.ResponseWriter, r *http.Request) {
	var req resellerSSORequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, BaseResponse{Status: "error", Message: "Invalid request body"})
			return
		}
	}

	username := sanitizeResellerValue(req.Username)
	if username == "" {
		username = "reseller_api"
	}
	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role != "reseller" && role != "admin" && role != "user" {
		role = "admin"
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		email = fmt.Sprintf("%s@aurapanel.local", username)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Reseller API SSO"
	}

	token, err := issueResellerSSOToken(User{
		ID:       1,
		Name:     name,
		Email:    email,
		Role:     role,
		Username: username,
	}, 5*time.Minute)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, BaseResponse{Status: "error", Message: "Failed to issue SSO token"})
		return
	}

	redirect := normalizeRedirectPath(req.Redirect)
	base := fmt.Sprintf("%s://%s", requestScheme(r), r.Host)
	consumeURL := fmt.Sprintf("%s/api/v1/reseller/sso/consume?token=%s&redirect=%s", base, url.QueryEscape(token), url.QueryEscape(redirect))

	writeJSON(w, http.StatusOK, BaseResponse{
		Status:  "success",
		Message: "SSO link generated",
		Data: map[string]string{
			"url": consumeURL,
		},
	})
}

func ResellerSSOConsume(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		middleware.WriteError(w, r, http.StatusBadRequest, "AUTH_MISSING_TOKEN", "SSO token is required")
		return
	}

	parsedToken, err := jwt.ParseWithClaims(token, &gatewayClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(middleware.JwtSecret()), nil
	},
		jwt.WithIssuer(middleware.JwtIssuer()),
		jwt.WithAudience(middleware.JwtAudience()),
		jwt.WithLeeway(15*time.Second),
	)
	if err != nil || !parsedToken.Valid {
		middleware.WriteError(w, r, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "SSO token is invalid or expired")
		return
	}

	claims, ok := parsedToken.Claims.(*gatewayClaims)
	if !ok || claims.ExpiresAt == nil {
		middleware.WriteError(w, r, http.StatusUnauthorized, "AUTH_INVALID_CLAIMS", "SSO token claims are invalid")
		return
	}

	setResellerSSOCookie(w, r, token, claims.ExpiresAt.Time)
	redirect := normalizeRedirectPath(r.URL.Query().Get("redirect"))
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, redirect, http.StatusFound)
}

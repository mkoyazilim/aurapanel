package api

import (
	"encoding/json"
	"net/http"
	"net/url"
)

// verifyCaptcha verifies a Turnstile or reCAPTCHA token against the given provider.
func verifyCaptcha(provider, secret, token, ip string) (bool, error) {
	if secret == "" || token == "" {
		return false, nil
	}

	endpoint := "https://www.google.com/recaptcha/api/siteverify"
	if provider == "turnstile" {
		endpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	}

	data := url.Values{
		"secret":   []string{secret},
		"response": []string{token},
	}
	if ip != "" {
		data.Set("remoteip", ip)
	}

	resp, err := http.PostForm(endpoint, data)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	return result.Success, nil
}

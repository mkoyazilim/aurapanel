package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/aurapanel/api-gateway/middleware"
)

func dbToolsUpstreamURL() string {
	base := strings.TrimSpace(os.Getenv("AURAPANEL_DBTOOLS_UPSTREAM_URL"))
	if base == "" {
		base = "http://127.0.0.1"
	}
	return strings.TrimRight(base, "/")
}

func NewDBToolsProxy() (http.Handler, error) {
	target, err := url.Parse(dbToolsUpstreamURL())
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		authUser, hasAuthUser := middleware.GetAuthUser(req.Context())
		incomingHost := strings.TrimSpace(req.Host)
		forwardedProto := firstForwardedValue(req.Header.Get("X-Forwarded-Proto"))
		if forwardedProto == "" {
			if req.TLS != nil {
				forwardedProto = "https"
			} else {
				forwardedProto = "http"
			}
		}

		req.Header.Del("X-Forwarded-For")
		req.Header.Del("X-Forwarded-Host")
		req.Header.Del("X-Forwarded-Proto")

		originalDirector(req)
		req.Host = target.Host
		if incomingHost != "" {
			req.Header.Set("X-Forwarded-Host", incomingHost)
		}
		req.Header.Set("X-Forwarded-Proto", forwardedProto)
		// Force loopback identity for db-tools upstream to keep public path blocked
		// while allowing authenticated gateway traffic.
		req.Header.Set("X-Forwarded-For", "127.0.0.1")

		req.Header.Del("X-Aura-Auth-Email")
		req.Header.Del("X-Aura-Auth-Role")
		req.Header.Del("X-Aura-Auth-Name")
		req.Header.Del("X-Aura-Auth-Username")
		req.Header.Del("X-Aura-Proxy-Token")
		if hasAuthUser {
			req.Header.Set("X-Aura-Auth-Email", strings.TrimSpace(authUser.Email))
			req.Header.Set("X-Aura-Auth-Role", strings.TrimSpace(authUser.Role))
			req.Header.Set("X-Aura-Auth-Name", strings.TrimSpace(authUser.Name))
			req.Header.Set("X-Aura-Auth-Username", strings.TrimSpace(authUser.Username))
		}
		if token := strings.TrimSpace(os.Getenv("AURAPANEL_INTERNAL_PROXY_TOKEN")); token != "" {
			req.Header.Set("X-Aura-Proxy-Token", token)
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "db tools request failed: " + err.Error(),
		})
	}

	return proxy, nil
}

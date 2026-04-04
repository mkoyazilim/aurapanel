package controllers

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/aurapanel/api-gateway/middleware"
)

func serviceBaseURL() string {
	base := strings.TrimSpace(os.Getenv("AURAPANEL_SERVICE_URL"))
	if base == "" {
		return "http://127.0.0.1:8081"
	}
	return strings.TrimRight(base, "/")
}

func gatewayOnlyEnabled() bool {
	normalized := strings.ToLower(strings.TrimSpace(os.Getenv("AURAPANEL_GATEWAY_ONLY")))
	if normalized == "" {
		return true
	}
	return normalized == "1" || normalized == "true" || normalized == "yes" || normalized == "on"
}

func isLoopbackServiceTarget(u *url.URL) bool {
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func NewServiceProxy() (http.Handler, error) {
	target, err := url.Parse(serviceBaseURL())
	if err != nil {
		return nil, err
	}
	if gatewayOnlyEnabled() && !isLoopbackServiceTarget(target) {
		return nil, &url.Error{
			Op:  "parse",
			URL: target.String(),
			Err: ErrNonLoopbackServiceTarget,
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		authUser, hasAuthUser := middleware.GetAuthUser(req.Context())
		incomingHost := strings.TrimSpace(req.Host)
		incomingRemoteIP := remoteAddrHost(req.RemoteAddr)
		forwardedProto := firstForwardedValue(req.Header.Get("X-Forwarded-Proto"))
		if forwardedProto == "" {
			if req.TLS != nil {
				forwardedProto = "https"
			} else {
				forwardedProto = "http"
			}
		}

		// Drop any user-supplied forwarding headers before proxying.
		req.Header.Del("X-Forwarded-For")
		req.Header.Del("X-Forwarded-Host")
		req.Header.Del("X-Forwarded-Proto")

		originalDirector(req)
		req.Host = target.Host
		if incomingHost != "" {
			req.Header.Set("X-Forwarded-Host", incomingHost)
		}
		req.Header.Set("X-Forwarded-Proto", forwardedProto)
		if incomingRemoteIP != "" {
			req.Header.Set("X-Forwarded-For", incomingRemoteIP)
		}

		req.Header.Del("X-Aura-Auth-Email")
		req.Header.Del("X-Aura-Auth-Role")
		req.Header.Del("X-Aura-Auth-Name")
		req.Header.Del("X-Aura-Auth-Username")
		req.Header.Del("X-Aura-Auth-Permissions")
		req.Header.Del("X-Aura-Auth-Role-Policy-Id")
		req.Header.Del("X-Aura-Auth-Role-Policy-Name")
		req.Header.Del("X-Aura-Proxy-Token")
		if hasAuthUser {
			req.Header.Set("X-Aura-Auth-Email", strings.TrimSpace(authUser.Email))
			req.Header.Set("X-Aura-Auth-Role", strings.TrimSpace(authUser.Role))
			req.Header.Set("X-Aura-Auth-Name", strings.TrimSpace(authUser.Name))
			req.Header.Set("X-Aura-Auth-Username", strings.TrimSpace(authUser.Username))
			req.Header.Set("X-Aura-Auth-Permissions", strings.Join(authUser.Permissions, ","))
			req.Header.Set("X-Aura-Auth-Role-Policy-Id", strings.TrimSpace(authUser.RolePolicyID))
			req.Header.Set("X-Aura-Auth-Role-Policy-Name", strings.TrimSpace(authUser.RolePolicy))
		}
		if token := strings.TrimSpace(os.Getenv("AURAPANEL_INTERNAL_PROXY_TOKEN")); token != "" {
			req.Header.Set("X-Aura-Proxy-Token", token)
		}

		// The standard ReverseProxy handles websockets automatically in Go 1.12+,
		// we just need to make sure we don't accidentally buffer or block the upgrade.
		// DO NOT rewrite scheme to ws/wss, as http.Transport doesn't support them.
	}

	// Add websocket explicit support to proxy transport if needed
	// The standard ReverseProxy handles websockets automatically in Go 1.12+,
	// but we need to make sure we don't accidentally buffer or block the upgrade.

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "service request failed: " + err.Error(),
		})
	}

	return proxy, nil
}

func isWebsocketUpgrade(r *http.Request) bool {
	if strings.HasSuffix(strings.TrimSpace(r.URL.Path), "/terminal/ws") {
		return true
	}
	upgrade := strings.ToLower(strings.TrimSpace(r.Header.Get("Upgrade")))
	connection := strings.ToLower(strings.TrimSpace(r.Header.Get("Connection")))
	return upgrade == "websocket" || strings.Contains(connection, "upgrade")
}

func firstForwardedValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, ","); idx >= 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}

func remoteAddrHost(remoteAddr string) string {
	value := strings.TrimSpace(remoteAddr)
	if value == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		return strings.TrimSpace(host)
	}
	return value
}

var ErrNonLoopbackServiceTarget = &serviceProxyPolicyError{msg: "gateway-only mode requires loopback AURAPANEL_SERVICE_URL"}

type serviceProxyPolicyError struct {
	msg string
}

func (e *serviceProxyPolicyError) Error() string {
	return e.msg
}

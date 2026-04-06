package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aurapanel/api-gateway/controllers"
	"github.com/aurapanel/api-gateway/handlers"
	"github.com/aurapanel/api-gateway/middleware"
)

type Response struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

func gatewayAddr() string {
	addr := strings.TrimSpace(os.Getenv("AURAPANEL_GATEWAY_ADDR"))
	if addr == "" {
		return ":8090"
	}
	return addr
}

func main() {
	if err := middleware.RequireSecurityConfig(); err != nil {
		log.Fatalf("security configuration error: %v", err)
	}

	serviceProxy, err := controllers.NewServiceProxy()
	if err != nil {
		log.Fatalf("failed to initialize service proxy: %v", err)
	}
	dbToolsProxy, err := controllers.NewDBToolsProxy()
	if err != nil {
		log.Fatalf("failed to initialize db tools proxy: %v", err)
	}

	publicMux := http.NewServeMux()
	protectedMux := http.NewServeMux()

	// Public routes
	publicMux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response{
			Message: "AuraPanel API Gateway is operational.",
			Status:  "ok",
		})
	})
	// Login is delegated to the panel service as the single auth code path.
	publicMux.Handle("/api/v1/auth/login", serviceProxy)
	// Roundcube SSO bridge consumes one-time token without panel bearer auth.
	publicMux.Handle("/api/v1/mail/webmail/sso/consume", serviceProxy)
	// DB tool launch bridges consume one-time tokens without panel bearer auth.
	publicMux.Handle("/api/v1/db/tools/phpmyadmin/sso/consume", serviceProxy)
	publicMux.Handle("/api/v1/db/tools/pgadmin/sso/consume", serviceProxy)
	// Billing reseller SSO consume endpoint (tokenized URL).
	publicMux.HandleFunc("/api/v1/reseller/sso/consume", controllers.ResellerSSOConsume)

	// Protected auth/me routes
	protectedMux.HandleFunc("/api/v1/auth/me", controllers.Me)

	// Legacy compatibility routes
	protectedMux.HandleFunc("/api/system/status", handlers.GetSystemStatus)
	protectedMux.HandleFunc("/api/system/env", handlers.GetEnv)
	protectedMux.HandleFunc("/api/v1/system/status", handlers.GetSystemStatus)
	protectedMux.HandleFunc("/api/v1/system/env", handlers.GetEnv)
	protectedMux.HandleFunc("/api/system/reseller-token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			controllers.GetResellerToken(w, r)
			return
		}
		if r.Method == http.MethodPost {
			controllers.UpdateResellerToken(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			controllers.DeleteResellerToken(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})
	protectedMux.HandleFunc("/api/v1/system/reseller-token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			controllers.GetResellerToken(w, r)
			return
		}
		if r.Method == http.MethodPost {
			controllers.UpdateResellerToken(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			controllers.DeleteResellerToken(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})
	protectedMux.HandleFunc("/api/websites", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			controllers.ListWebsites(w, r)
			return
		}
		if r.Method == http.MethodPost {
			controllers.CreateWebsite(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	// Main proxy surface for frontend/service communication
	protectedMux.Handle("/api/v1/", serviceProxy)
	protectedMux.Handle("/phpmyadmin", dbToolsProxy)
	protectedMux.Handle("/phpmyadmin/", dbToolsProxy)
	protectedMux.Handle("/pgadmin4", dbToolsProxy)
	protectedMux.Handle("/pgadmin4/", dbToolsProxy)

	publicHandler := middleware.RequestIDMiddleware(
		middleware.CorsMiddleware(
			middleware.Logger(
				middleware.DDoSGuardMiddleware(publicMux),
			),
		),
	)
	protectedHandler := middleware.RequestIDMiddleware(
		middleware.CorsMiddleware(
			middleware.Logger(
				middleware.DDoSGuardMiddleware(
					middleware.AuthMiddleware(
						middleware.RBACMiddleware(protectedMux),
					),
				),
			),
		),
	)

	resellerMux := http.NewServeMux()
	resellerMux.HandleFunc("/api/v1/reseller/account/create", controllers.ResellerCreateAccount)
	resellerMux.HandleFunc("/api/v1/reseller/account/suspend", controllers.ResellerSuspendAccount)
	resellerMux.HandleFunc("/api/v1/reseller/account/unsuspend", controllers.ResellerUnsuspendAccount)
	resellerMux.HandleFunc("/api/v1/reseller/account/terminate", controllers.ResellerTerminateAccount)
	resellerMux.HandleFunc("/api/v1/reseller/account/password", controllers.ResellerChangePassword)
	resellerMux.HandleFunc("/api/v1/reseller/account/package", controllers.ResellerChangePackage)
	resellerMux.HandleFunc("/api/v1/reseller/packages", controllers.ResellerListPackages)
	resellerMux.HandleFunc("/api/v1/reseller/sso", controllers.ResellerSSO)

	resellerHandler := middleware.RequestIDMiddleware(
		middleware.CorsMiddleware(
			middleware.Logger(
				middleware.DDoSGuardMiddleware(
					middleware.ResellerAuthMiddleware(resellerMux),
				),
			),
		),
	)

	mainRouter := http.NewServeMux()

	// Public
	mainRouter.Handle("/api/health", publicHandler)
	mainRouter.Handle("/api/v1/auth/login", publicHandler)
	mainRouter.Handle("/api/v1/mail/webmail/sso/consume", publicHandler)
	mainRouter.Handle("/api/v1/db/tools/phpmyadmin/sso/consume", publicHandler)
	mainRouter.Handle("/api/v1/db/tools/pgadmin/sso/consume", publicHandler)
	mainRouter.Handle("/api/v1/reseller/sso/consume", publicHandler)

	// Reseller integration endpoints (token-authenticated).
	// Keep these explicit so panel-authenticated reseller module routes like
	// /api/v1/reseller/quotas and /api/v1/reseller/whitelabel continue through
	// normal protected auth instead of reseller-token auth.
	mainRouter.Handle("/api/v1/reseller/account/create", resellerHandler)
	mainRouter.Handle("/api/v1/reseller/account/suspend", resellerHandler)
	mainRouter.Handle("/api/v1/reseller/account/unsuspend", resellerHandler)
	mainRouter.Handle("/api/v1/reseller/account/terminate", resellerHandler)
	mainRouter.Handle("/api/v1/reseller/account/password", resellerHandler)
	mainRouter.Handle("/api/v1/reseller/account/package", resellerHandler)
	mainRouter.Handle("/api/v1/reseller/packages", resellerHandler)
	mainRouter.Handle("/api/v1/reseller/sso", resellerHandler)

	// Protected
	mainRouter.Handle("/api/v1/auth/me", protectedHandler)
	mainRouter.Handle("/api/system/", protectedHandler)
	mainRouter.Handle("/api/websites", protectedHandler)
	mainRouter.Handle("/api/v1/", protectedHandler)
	mainRouter.Handle("/phpmyadmin", protectedHandler)
	mainRouter.Handle("/phpmyadmin/", protectedHandler)
	mainRouter.Handle("/pgadmin4", protectedHandler)
	mainRouter.Handle("/pgadmin4/", protectedHandler)
	mainRouter.Handle("/", middleware.Logger(controllers.PanelStaticHandler()))

	listenAddr := gatewayAddr()
	fmt.Printf("API Gateway listening on %s\n", listenAddr)
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           mainRouter,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

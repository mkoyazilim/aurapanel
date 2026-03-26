package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

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

	coreProxy, err := controllers.NewCoreProxy()
	if err != nil {
		log.Fatalf("failed to initialize core proxy: %v", err)
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
	publicMux.HandleFunc("/api/auth/login", controllers.Login)
	publicMux.HandleFunc("/api/v1/auth/login", controllers.Login)

	// Protected auth/me routes
	protectedMux.HandleFunc("/api/auth/me", controllers.Me)
	protectedMux.HandleFunc("/api/v1/auth/me", controllers.Me)

	// Legacy compatibility routes
	protectedMux.HandleFunc("/api/system/status", handlers.GetSystemStatus)
	protectedMux.HandleFunc("/api/system/env", handlers.GetEnv)
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

	// Main proxy surface for frontend/core communication
	protectedMux.Handle("/api/v1/", coreProxy)

	publicHandler := middleware.RequestIDMiddleware(
		middleware.CorsMiddleware(
			middleware.Logger(publicMux),
		),
	)
	protectedHandler := middleware.RequestIDMiddleware(
		middleware.CorsMiddleware(
			middleware.Logger(
				middleware.AuthMiddleware(protectedMux),
			),
		),
	)

	mainRouter := http.NewServeMux()

	// Public
	mainRouter.Handle("/api/health", publicHandler)
	mainRouter.Handle("/api/auth/login", publicHandler)
	mainRouter.Handle("/api/v1/auth/login", publicHandler)

	// Protected
	mainRouter.Handle("/api/auth/me", protectedHandler)
	mainRouter.Handle("/api/v1/auth/me", protectedHandler)
	mainRouter.Handle("/api/system/", protectedHandler)
	mainRouter.Handle("/api/websites", protectedHandler)
	mainRouter.Handle("/api/v1/", protectedHandler)
	mainRouter.Handle("/", middleware.Logger(controllers.PanelStaticHandler()))

	listenAddr := gatewayAddr()
	fmt.Printf("API Gateway listening on %s\n", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mainRouter))
}

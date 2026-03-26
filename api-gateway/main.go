package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/aurapanel/api-gateway/controllers"
	"github.com/aurapanel/api-gateway/handlers"
	"github.com/aurapanel/api-gateway/middleware"
)

type Response struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

func main() {
	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Message: "AuraPanel API Gateway is operational.",
			Status:  "ok",
		})
	})
	
	// Auth Routes
	mux.HandleFunc("/api/auth/login", controllers.Login)

	// Protected routes Wrapper
	protectedMux := http.NewServeMux()
	
	// System API
	protectedMux.HandleFunc("/api/system/status", handlers.GetSystemStatus)
	protectedMux.HandleFunc("/api/system/env", handlers.GetEnv)

	// Auth User Details
	protectedMux.HandleFunc("/api/auth/me", controllers.Me)

	// Websites API
	protectedMux.HandleFunc("/api/websites", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			controllers.ListWebsites(w, r)
		} else if r.Method == http.MethodPost {
			controllers.CreateWebsite(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	// Combine Middlewares For Public Endpoints
	publicHandler := middleware.CorsMiddleware(middleware.Logger(mux))

	// Combine Middlewares For Protected Endpoints
	protectedHandler := middleware.CorsMiddleware(middleware.Logger(middleware.AuthMiddleware(protectedMux)))

	// Main Router
	mainRouter := http.NewServeMux()
	
	// Map public
	mainRouter.Handle("/api/health", publicHandler)
	mainRouter.Handle("/api/auth/login", publicHandler)
	
	// Map protected by mapping their prefixes
	mainRouter.Handle("/api/system/", protectedHandler)
	mainRouter.Handle("/api/auth/me", protectedHandler)
	mainRouter.Handle("/api/websites", protectedHandler)

	fmt.Println("API Gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mainRouter))
}

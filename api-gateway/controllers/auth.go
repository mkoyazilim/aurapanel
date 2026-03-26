package controllers

import (
	"encoding/json"
	"net/http"
	"time"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Login handles user authentication and JWT token generation
func Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// TODO: Replace with Real DB Check against Rust Core
	if req.Email == "admin@server.com" && req.Password == "password123" {
		// Mock Token Generation (Replace with real JWT library logic)
		token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.MockAuraPanelToken." + time.Now().Format("20060102150405")
		
		response := AuthResponse{
			Token: token,
			User: User{
				ID:    1,
				Name:  "Sunucu Yöneticisi",
				Email: req.Email,
				Role:  "admin",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	http.Error(w, "Invalid credentials", http.StatusUnauthorized)
}

// Me returns current logged in user details
func Me(w http.ResponseWriter, r *http.Request) {
	// Dummy profile implementation
	user := User{
		ID:    1,
		Name:  "Sunucu Yöneticisi",
		Email: "admin@server.com",
		Role:  "admin",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

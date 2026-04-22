package model

import "time"

// User represents an authenticated user in the system.
// This struct maps directly to the "users" table in PostgreSQL.
//
// WHY separate from the User Service's profile model?
// Auth owns credentials (email, password hash).
// User Service owns profile data (display name, preferences).
// If auth is compromised, attackers get hashes — not personal data.
type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // "-" means NEVER serialize to JSON
	CreatedAt    time.Time `json:"created_at"`
}

// LoginRequest is what the client sends to POST /api/auth/login.
// Separating request/response types from the DB model prevents
// accidentally exposing internal fields (like password hashes).
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterRequest is what the client sends to POST /api/auth/register.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// TokenResponse is what we send back after successful auth.
// The client stores this token and sends it in the Authorization header.
type TokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

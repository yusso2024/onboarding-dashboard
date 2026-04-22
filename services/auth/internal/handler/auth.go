package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"auth-service/internal/middleware"
	"auth-service/internal/model"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler holds dependencies for auth endpoints.
//
// WHY struct with dependencies instead of global variables?
// - Testability: you can inject mock DB/Redis in tests
// - Explicit dependencies: you see exactly what auth needs
// - No hidden state: everything is passed explicitly
//
// This is "dependency injection" — a core pattern in systems design.
// The handler doesn't create its own DB connection; it receives one.
type AuthHandler struct {
	DB    *sql.DB
	Redis *redis.Client
}

// Register creates a new user account.
// POST /api/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, http.StatusBadRequest)
		return
	}

	// WHY bcrypt?
	// - It's intentionally slow (cost factor 12 = ~250ms per hash)
	// - This makes brute-force attacks impractical
	// - The salt is embedded in the hash output, so each password
	//   produces a different hash even if passwords are identical
	// - Compare to MD5/SHA: those are fast (millions/sec) = insecure for passwords
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		log.Printf("ERROR: failed to hash password: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	var userID int
	err = h.DB.QueryRow(
		"INSERT INTO users (email, password_hash, created_at) VALUES ($1, $2, $3) RETURNING id",
		req.Email, string(hash), time.Now(),
	).Scan(&userID)

	if err != nil {
		// WHY check for unique violation specifically?
		// Postgres returns a specific error code (23505) for duplicate keys.
		// We translate DB errors into user-friendly HTTP responses.
		// This is "error translation" across system boundaries.
		if isDuplicateKeyError(err) {
			http.Error(w, `{"error":"email already registered"}`, http.StatusConflict)
			return
		}
		log.Printf("ERROR: failed to insert user: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	token, expiresAt, err := middleware.GenerateToken(userID)
	if err != nil {
		log.Printf("ERROR: failed to generate token: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(model.TokenResponse{
		Token:     token,
		ExpiresAt: expiresAt,
	})
}

// Login authenticates a user and returns a JWT.
// POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	var user model.User
	err := h.DB.QueryRow(
		"SELECT id, email, password_hash FROM users WHERE email = $1",
		req.Email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash)

	if err == sql.ErrNoRows {
		// WHY same error for "user not found" and "wrong password"?
		// Telling the attacker which part is wrong leaks information:
		// "user not found" = they know the email doesn't exist
		// "wrong password" = they know the email DOES exist
		// Same error for both = no information leaked.
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}
	if err != nil {
		log.Printf("ERROR: database query failed: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	// WHY CompareHashAndPassword instead of hashing the input and comparing?
	// bcrypt embeds the salt in the hash. You can't just hash the input
	// with a new salt and compare — you need to extract the original salt
	// from the stored hash. CompareHashAndPassword handles this correctly.
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	token, expiresAt, err := middleware.GenerateToken(user.ID)
	if err != nil {
		log.Printf("ERROR: failed to generate token: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Cache the token in Redis for potential blacklisting later
	h.Redis.Set(r.Context(), "token:"+token, user.ID, 24*time.Hour)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.TokenResponse{
		Token:     token,
		ExpiresAt: expiresAt,
	})
}

// Health returns service health status.
// GET /api/auth/health
//
// WHY a health endpoint?
// - Docker healthchecks call this to know if the service is alive
// - Load balancers use it to decide whether to route traffic here
// - Prometheus scrapes it for up/down metrics
// - It checks BOTH the service AND its dependencies (DB, Redis)
//
// A service that's "up" but can't reach its database is NOT healthy.
func (h *AuthHandler) Health(w http.ResponseWriter, r *http.Request) {
	// Check database
	if err := h.DB.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status":   "unhealthy",
			"database": err.Error(),
		})
		return
	}

	// Check Redis
	if err := h.Redis.Ping(r.Context()).Err(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"redis":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// isDuplicateKeyError checks if a Postgres error is a unique constraint violation.
func isDuplicateKeyError(err error) bool {
	return err != nil && (contains(err.Error(), "duplicate key") || contains(err.Error(), "23505"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

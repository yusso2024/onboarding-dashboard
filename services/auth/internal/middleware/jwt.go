package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey is an unexported type to prevent collisions in context values.
// WHY a custom type? If you use a plain string like "userID",
// any package could accidentally overwrite it. An unexported type
// makes the key impossible to access from outside this package.
type contextKey string

const userIDKey contextKey = "userID"

// GenerateToken creates a signed JWT for the given user ID.
//
// WHY JWT instead of session cookies?
// - Stateless: no server-side session storage needed
// - Scalable: any service can validate the token without calling auth
// - Self-contained: the token carries the user ID inside it
//
// The tradeoff: you can't revoke a JWT before it expires
// (without a blacklist, which re-introduces state).
func GenerateToken(userID int) (string, time.Time, error) {
	expiresAt := time.Now().Add(24 * time.Hour)

	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     expiresAt.Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", time.Time{}, fmt.Errorf("JWT_SECRET not set")
	}

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return signed, expiresAt, nil
}

// AuthMiddleware validates the JWT from the Authorization header.
//
// WHY middleware instead of checking in every handler?
// - DRY: write auth logic once, apply to any route
// - Separation of concerns: handlers focus on business logic
// - Composable: you can stack middlewares (auth → rate-limit → handler)
//
// This is the "chain of responsibility" pattern in systems design.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		// Expect format: "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
			return
		}

		secret := os.Getenv("JWT_SECRET")
		token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
			// WHY check the signing method?
			// Without this, an attacker can change the JWT header to "alg: none"
			// and the parser would accept an unsigned token. This is a real CVE.
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, `{"error":"invalid claims"}`, http.StatusUnauthorized)
			return
		}

		userID := int(claims["user_id"].(float64))

		// Store userID in request context so handlers can access it
		// without parsing the token again.
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext extracts the user ID set by AuthMiddleware.
// Returns 0 if not found (unauthenticated request).
func UserIDFromContext(ctx context.Context) int {
	id, ok := ctx.Value(userIDKey).(int)
	if !ok {
		return 0
	}
	return id
}

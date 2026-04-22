package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"user-service/internal/model"
	pb "user-service/proto/inventorypb"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type UserHandler struct {
	DB    *sql.DB
	Redis *redis.Client
}

func (h *UserHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	userID := userIDFromRequest(r)
	if userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req model.CreateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.DisplayName == "" {
		http.Error(w, `{"error":"display_name required"}`, http.StatusBadRequest)
		return
	}

	var profile model.Profile
	err := h.DB.QueryRow(`
		INSERT INTO profiles (user_id, display_name, role, onboarding_step, onboarding_done, created_at, updated_at)
		VALUES ($1, $2, $3, 1, false, $4, $4)
		RETURNING id, user_id, display_name, role, onboarding_step, onboarding_done, created_at, updated_at`,
		userID, req.DisplayName, req.Role, time.Now(),
	).Scan(&profile.ID, &profile.UserID, &profile.DisplayName, &profile.Role,
		&profile.OnboardingStep, &profile.OnboardingDone, &profile.CreatedAt, &profile.UpdatedAt)

	if err != nil {
		log.Printf("ERROR: failed to create profile: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	profileJSON, _ := json.Marshal(profile)
	h.Redis.Set(r.Context(), profileCacheKey(userID), profileJSON, 15*time.Minute)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(profile)
}

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	userID := userIDFromRequest(r)
	if userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	cached, err := h.Redis.Get(r.Context(), profileCacheKey(userID)).Result()
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write([]byte(cached))
		return
	}

	var profile model.Profile
	err = h.DB.QueryRow(`
		SELECT id, user_id, display_name, role, onboarding_step, onboarding_done, created_at, updated_at
		FROM profiles WHERE user_id = $1`, userID,
	).Scan(&profile.ID, &profile.UserID, &profile.DisplayName, &profile.Role,
		&profile.OnboardingStep, &profile.OnboardingDone, &profile.CreatedAt, &profile.UpdatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"profile not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("ERROR: database query failed: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	profileJSON, _ := json.Marshal(profile)
	h.Redis.Set(r.Context(), profileCacheKey(userID), profileJSON, 15*time.Minute)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(profile)
}

func (h *UserHandler) UpdateOnboarding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	userID := userIDFromRequest(r)
	if userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req model.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.OnboardingStep == nil {
		http.Error(w, `{"error":"onboarding_step required"}`, http.StatusBadRequest)
		return
	}

	onboardingDone := *req.OnboardingStep >= 5

	var profile model.Profile
	err := h.DB.QueryRow(`
		UPDATE profiles
		SET onboarding_step = $1, onboarding_done = $2, updated_at = $3
		WHERE user_id = $4
		RETURNING id, user_id, display_name, role, onboarding_step, onboarding_done, created_at, updated_at`,
		*req.OnboardingStep, onboardingDone, time.Now(), userID,
	).Scan(&profile.ID, &profile.UserID, &profile.DisplayName, &profile.Role,
		&profile.OnboardingStep, &profile.OnboardingDone, &profile.CreatedAt, &profile.UpdatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"profile not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("ERROR: failed to update onboarding: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	h.Redis.Del(r.Context(), profileCacheKey(userID))

	if onboardingDone {
		go assignStarterPack(userID)
	}

	response := map[string]interface{}{
		"profile": profile,
	}
	if onboardingDone {
		response["message"] = "Onboarding complete! Starter pack is being assigned."
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func assignStarterPack(userID int) {
	conn, err := grpc.NewClient("inventory-service:4000",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Printf("ERROR: failed to connect to inventory gRPC: %v", err)
		return
	}
	defer conn.Close()

	client := pb.NewInventoryGrpcClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.AssignStarterPack(ctx, &pb.AssignStarterPackRequest{
		UserId: int32(userID),
	})
	if err != nil {
		log.Printf("ERROR: gRPC AssignStarterPack failed: %v", err)
		return
	}

	log.Printf("gRPC: %s (assigned %d assets)", resp.Message, len(resp.Assets))
}

func (h *UserHandler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.DB.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "database": err.Error()})
		return
	}
	if err := h.Redis.Ping(r.Context()).Err(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "redis": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func userIDFromRequest(r *http.Request) int {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return 0
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return 0
	}
	secret := os.Getenv("JWT_SECRET")
	token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return 0
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0
	}
	return int(claims["user_id"].(float64))
}

func profileCacheKey(userID int) string {
	return fmt.Sprintf("profile:%d", userID)
}

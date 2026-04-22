package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"inventory-service/internal/circuitbreaker"
	"inventory-service/internal/model"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type InventoryHandler struct {
	Collection   *mongo.Collection
	Redis        *redis.Client
	RedisBreaker *circuitbreaker.Breaker
}

// cacheGet wraps Redis GET through the circuit breaker.
// If Redis is down, the breaker opens and we skip Redis entirely.
// This is the FIX for the chaos test failure we saw earlier.
func (h *InventoryHandler) cacheGet(ctx context.Context, key string) (string, error) {
	var result string
	err := h.RedisBreaker.Execute(func() error {
		var err error
		result, err = h.Redis.Get(ctx, key).Result()
		return err
	})
	return result, err
}

// cacheSet wraps Redis SET through the circuit breaker.
func (h *InventoryHandler) cacheSet(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	h.RedisBreaker.Execute(func() error {
		return h.Redis.Set(ctx, key, value, ttl).Err()
	})
}

// cacheDel wraps Redis DEL through the circuit breaker.
func (h *InventoryHandler) cacheDel(ctx context.Context, key string) {
	h.RedisBreaker.Execute(func() error {
		return h.Redis.Del(ctx, key).Err()
	})
}

func (h *InventoryHandler) ListAssets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	filter := bson.M{}
	if category := r.URL.Query().Get("category"); category != "" {
		filter["category"] = category
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filter["status"] = status
	}

	cacheKey := "assets:all"
	if len(filter) == 0 {
		// TRY cache — if breaker is open, this returns immediately
		cached, err := h.cacheGet(r.Context(), cacheKey)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.Header().Set("X-Circuit-Breaker", h.RedisBreaker.GetState().String())
			w.Write([]byte(cached))
			return
		}
		// Cache miss OR circuit open — either way, fall through to DB
	}

	cursor, err := h.Collection.Find(r.Context(), filter)
	if err != nil {
		log.Printf("ERROR: MongoDB query failed: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(r.Context())

	var assets []model.Asset
	if err := cursor.All(r.Context(), &assets); err != nil {
		log.Printf("ERROR: failed to decode assets: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	if assets == nil {
		assets = []model.Asset{}
	}

	result, _ := json.Marshal(assets)

	if len(filter) == 0 {
		h.cacheSet(r.Context(), cacheKey, result, 5*time.Minute)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Header().Set("X-Circuit-Breaker", h.RedisBreaker.GetState().String())
	json.NewEncoder(w).Encode(assets)
}

func (h *InventoryHandler) CreateAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	userID := userIDFromRequest(r)
	if userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req model.CreateAssetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Category == "" {
		http.Error(w, `{"error":"name and category required"}`, http.StatusBadRequest)
		return
	}

	asset := model.Asset{
		Name:       req.Name,
		Category:   req.Category,
		Status:     "available",
		AssignedTo: 0,
		Metadata:   req.Metadata,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if asset.Metadata == nil {
		asset.Metadata = map[string]interface{}{}
	}

	result, err := h.Collection.InsertOne(r.Context(), asset)
	if err != nil {
		log.Printf("ERROR: failed to insert asset: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	asset.ID = fmt.Sprintf("%v", result.InsertedID)

	h.cacheDel(r.Context(), "assets:all")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(asset)
}

func (h *InventoryHandler) AssignAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	userID := userIDFromRequest(r)
	if userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		AssetID string `json:"asset_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	objectID, err := bson.ObjectIDFromHex(req.AssetID)
	if err != nil {
		http.Error(w, `{"error":"invalid asset_id"}`, http.StatusBadRequest)
		return
	}

	filter := bson.M{
		"_id":    objectID,
		"status": "available",
	}
	update := bson.M{
		"$set": bson.M{
			"status":      "assigned",
			"assigned_to": userID,
			"updated_at":  time.Now(),
		},
	}

	var asset model.Asset
	err = h.Collection.FindOneAndUpdate(r.Context(), filter, update).Decode(&asset)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, `{"error":"asset not available or not found"}`, http.StatusConflict)
			return
		}
		log.Printf("ERROR: failed to assign asset: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	h.cacheDel(r.Context(), "assets:all")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(asset)
}

func (h *InventoryHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	status := map[string]string{"status": "healthy"}
	httpCode := http.StatusOK

	if err := h.Collection.Database().Client().Ping(ctx, nil); err != nil {
		status["status"] = "unhealthy"
		status["mongodb"] = err.Error()
		httpCode = http.StatusServiceUnavailable
	}

	// Report Redis + circuit breaker state
	status["redis_circuit_breaker"] = h.RedisBreaker.GetState().String()

	var redisErr error
	h.RedisBreaker.Execute(func() error {
		redisErr = h.Redis.Ping(ctx).Err()
		return redisErr
	})
	if redisErr != nil {
		status["redis"] = "down (circuit breaker active — service degraded but functional)"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(status)
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

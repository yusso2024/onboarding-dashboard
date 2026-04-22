package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"inventory-service/internal/circuitbreaker"
	"inventory-service/internal/handler"
	"inventory-service/internal/tracing"
	pb "inventory-service/proto/inventorypb"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Inventory Service...")

	jaegerEndpoint := os.Getenv("JAEGER_ENDPOINT")
	if jaegerEndpoint != "" {
		cleanup, err := tracing.Init("inventory-service", jaegerEndpoint)
		if err != nil {
			log.Printf("WARNING: Tracing init failed: %v", err)
		} else {
			defer cleanup()
		}
	}

	mongoURI := fmt.Sprintf("mongodb://%s:%s@%s:%s",
		os.Getenv("INVENTORY_DB_USER"),
		os.Getenv("INVENTORY_DB_PASSWORD"),
		os.Getenv("INVENTORY_DB_HOST"),
		os.Getenv("INVENTORY_DB_PORT"),
	)

	var client *mongo.Client
	var err error
	for i := 0; i < 5; i++ {
		client, err = mongo.Connect(options.Client().ApplyURI(mongoURI).SetBSONOptions(&options.BSONOptions{ObjectIDAsHexString: true}))
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err = client.Ping(ctx, nil)
			cancel()
		}
		if err == nil {
			break
		}
		log.Printf("Waiting for MongoDB (attempt %d/5): %v", i+1, err)
		time.Sleep(time.Duration(i+1) * 2 * time.Second)
	}
	if err != nil {
		log.Fatalf("FATAL: Could not connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	log.Println("Connected to MongoDB")

	collection := client.Database(os.Getenv("INVENTORY_DB_NAME")).Collection("assets")

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s",
			os.Getenv("REDIS_HOST"),
			os.Getenv("REDIS_PORT"),
		),
	})

	redisBreaker := circuitbreaker.New("redis", 5, 30*time.Second)
	log.Println("Circuit breaker initialized for Redis")

	invHandler := &handler.InventoryHandler{
		Collection:   collection,
		Redis:        rdb,
		RedisBreaker: redisBreaker,
	}

	// ---- Start gRPC server in a goroutine ----
	// WHY a separate goroutine?
	// The HTTP server and gRPC server listen on different ports
	// but share the same database connection and business logic.
	// They run concurrently in the same process.
	//
	// WHY port 4000 for gRPC?
	// Port 3000 is HTTP (for the gateway). Port 4000 is gRPC
	// (for internal service-to-service calls). Different protocols,
	// different ports. External clients never reach port 4000.
	go func() {
		lis, err := net.Listen("tcp", ":4000")
		if err != nil {
			log.Fatalf("FATAL: Failed to listen on gRPC port: %v", err)
		}

		grpcServer := grpc.NewServer()
		pb.RegisterInventoryGrpcServer(grpcServer, handler.NewGrpcHandler(invHandler))

		log.Println("gRPC server listening on :4000")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("FATAL: gRPC server failed: %v", err)
		}
	}()

	// ---- HTTP server (existing) ----
	mux := http.NewServeMux()
	mux.HandleFunc("/api/inventory/assets", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			invHandler.ListAssets(w, r)
		case http.MethodPost:
			invHandler.CreateAsset(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/inventory/assets/assign", invHandler.AssignAsset)
	mux.HandleFunc("/api/inventory/health", invHandler.Health)

	port := os.Getenv("INVENTORY_PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("HTTP server listening on :%s", port)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      otelhttp.NewHandler(mux, "inventory-service"),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("FATAL: Server failed: %v", err)
	}
}

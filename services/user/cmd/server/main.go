package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"user-service/internal/handler"
	"user-service/internal/tracing"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting User Service...")

	jaegerEndpoint := os.Getenv("JAEGER_ENDPOINT")
	if jaegerEndpoint != "" {
		cleanup, err := tracing.Init("user-service", jaegerEndpoint)
		if err != nil {
			log.Printf("WARNING: Tracing init failed: %v", err)
		} else {
			defer cleanup()
		}
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("USER_DB_HOST"),
		os.Getenv("USER_DB_PORT"),
		os.Getenv("USER_DB_USER"),
		os.Getenv("USER_DB_PASSWORD"),
		os.Getenv("USER_DB_NAME"),
	)

	var db *sql.DB
	var err error
	for i := 0; i < 5; i++ {
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			break
		}
		log.Printf("Waiting for database (attempt %d/5): %v", i+1, err)
		time.Sleep(time.Duration(i+1) * 2 * time.Second)
	}
	if err != nil {
		log.Fatalf("FATAL: Could not connect to database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("Connected to PostgreSQL")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS profiles (
			id SERIAL PRIMARY KEY,
			user_id INTEGER UNIQUE NOT NULL,
			display_name VARCHAR(255) NOT NULL,
			role VARCHAR(100) DEFAULT '',
			onboarding_step INTEGER DEFAULT 1,
			onboarding_done BOOLEAN DEFAULT false,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Fatalf("FATAL: Could not create tables: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s",
			os.Getenv("REDIS_HOST"),
			os.Getenv("REDIS_PORT"),
		),
	})

	userHandler := &handler.UserHandler{
		DB:    db,
		Redis: rdb,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/users/profile", userHandler.CreateProfile)
	mux.HandleFunc("/api/users/profile/me", userHandler.GetProfile)
	mux.HandleFunc("/api/users/onboarding", userHandler.UpdateOnboarding)
	mux.HandleFunc("/api/users/health", userHandler.Health)

	port := os.Getenv("USER_PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("User Service listening on :%s", port)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      otelhttp.NewHandler(mux, "user-service"),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("FATAL: Server failed: %v", err)
	}
}

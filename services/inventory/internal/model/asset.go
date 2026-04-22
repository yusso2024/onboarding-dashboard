package model

import "time"

// Asset represents a demo resource in the onboarding inventory.
//
// WHY map[string]interface{} for Metadata?
// Each asset type has different attributes:
//   - VM template: {"cpu": 4, "ram_gb": 16, "os": "ubuntu"}
//   - Document:    {"format": "pdf", "pages": 42}
//   - API key:     {"scope": "read-only", "rate_limit": 1000}
//
// In MongoDB, this maps to a nested document with arbitrary fields.
// This is the flexibility advantage over PostgreSQL's rigid columns.
// Tradeoff: no compile-time type safety on metadata fields.
type Asset struct {
	ID          string                 `json:"id" bson:"_id,omitempty"`
	Name        string                 `json:"name" bson:"name"`
	Category    string                 `json:"category" bson:"category"`
	Status      string                 `json:"status" bson:"status"`
	AssignedTo  int                    `json:"assigned_to" bson:"assigned_to"`
	Metadata    map[string]interface{} `json:"metadata" bson:"metadata"`
	CreatedAt   time.Time              `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" bson:"updated_at"`
}

// WHY both `json` and `bson` tags?
// `json` controls how Go serializes to/from HTTP JSON responses.
// `bson` controls how Go serializes to/from MongoDB's binary format.
// They can differ: MongoDB uses `_id`, but our API returns `id`.

// CreateAssetRequest is what clients send to add a new asset.
type CreateAssetRequest struct {
	Name     string                 `json:"name"`
	Category string                 `json:"category"`
	Metadata map[string]interface{} `json:"metadata"`
}

// AssetFilter is used for querying assets by criteria.
type AssetFilter struct {
	Category   string `json:"category"`
	Status     string `json:"status"`
	AssignedTo int    `json:"assigned_to"`
}

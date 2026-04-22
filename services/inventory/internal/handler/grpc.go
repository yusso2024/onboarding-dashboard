package handler

import (
	"context"
	"fmt"
	"log"
	"time"

	"inventory-service/internal/model"
	pb "inventory-service/proto/inventorypb"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// GrpcHandler implements the InventoryGrpc service defined in the proto file.
//
// WHY a separate handler for gRPC?
// HTTP handlers deal with http.Request/ResponseWriter.
// gRPC handlers receive proto messages and return proto messages.
// Different transport, different handler — but same business logic (MongoDB).
type GrpcHandler struct {
	pb.UnimplementedInventoryGrpcServer
	httpHandler *InventoryHandler
}

func NewGrpcHandler(h *InventoryHandler) *GrpcHandler {
	return &GrpcHandler{httpHandler: h}
}

// AssignStarterPack creates and assigns a set of default assets
// when a user completes onboarding.
//
// WHY is this a gRPC call instead of the user service writing to MongoDB?
// Because the Inventory Service OWNS its data. The User Service should
// never write directly to the inventory database. This is the
// "data ownership" principle in microservices.
//
// The User Service says "assign a starter pack for user X."
// The Inventory Service decides WHAT to assign and HOW to store it.
func (g *GrpcHandler) AssignStarterPack(ctx context.Context, req *pb.AssignStarterPackRequest) (*pb.AssignStarterPackResponse, error) {
	log.Printf("gRPC: AssignStarterPack for user_id=%d", req.UserId)

	// Define the starter pack — in production this would come from config
	starterAssets := []model.Asset{
		{
			Name:       "Dev VM - Ubuntu 24.04",
			Category:   "vm_template",
			Status:     "assigned",
			AssignedTo: int(req.UserId),
			Metadata:   map[string]interface{}{"cpu": 2, "ram_gb": 8, "os": "ubuntu"},
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			Name:       "Onboarding Guide",
			Category:   "document",
			Status:     "assigned",
			AssignedTo: int(req.UserId),
			Metadata:   map[string]interface{}{"format": "pdf", "pages": 25},
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
		{
			Name:       "Staging API Key",
			Category:   "api_key",
			Status:     "assigned",
			AssignedTo: int(req.UserId),
			Metadata:   map[string]interface{}{"scope": "read-only", "environment": "staging"},
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	}

	var responseAssets []*pb.Asset
	for _, asset := range starterAssets {
		result, err := g.httpHandler.Collection.InsertOne(ctx, asset)
		if err != nil {
			log.Printf("ERROR: failed to insert starter asset: %v", err)
			continue
		}

		id := fmt.Sprintf("%v", result.InsertedID)
		if oid, ok := result.InsertedID.(bson.ObjectID); ok {
			id = oid.Hex()
		}

		responseAssets = append(responseAssets, &pb.Asset{
			Id:       id,
			Name:     asset.Name,
			Category: asset.Category,
			Status:   asset.Status,
		})
	}

	// Invalidate cache
	g.httpHandler.cacheDel(ctx, "assets:all")

	return &pb.AssignStarterPackResponse{
		Assets:  responseAssets,
		Message: fmt.Sprintf("Assigned %d starter assets to user %d", len(responseAssets), req.UserId),
	}, nil
}

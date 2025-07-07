package store

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/piske-alex/margin-offer-system/proto/gen/go/proto"
	"github.com/piske-alex/margin-offer-system/types"
)

// GRPCServer implements the MarginOfferService gRPC interface
type GRPCServer struct {
	pb.UnimplementedMarginOfferServiceServer
	store  types.MarginOfferStore
	server *grpc.Server
}

// NewGRPCServer creates a new gRPC server
func NewGRPCServer(store types.MarginOfferStore) *GRPCServer {
	return &GRPCServer{
		store: store,
	}
}

// Start starts the gRPC server on the specified address
func (s *GRPCServer) Start(address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.server = grpc.NewServer()
	pb.RegisterMarginOfferServiceServer(s.server, s)

	fmt.Printf("gRPC server listening on %s\n", address)
	return s.server.Serve(lis)
}

// Stop gracefully stops the gRPC server
func (s *GRPCServer) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
}

// CreateMarginOffer creates a new margin offer
func (s *GRPCServer) CreateMarginOffer(ctx context.Context, req *pb.CreateMarginOfferRequest) (*pb.CreateMarginOfferResponse, error) {
	if req.Offer == nil {
		return nil, status.Error(codes.InvalidArgument, "offer is required")
	}

	offer, err := s.protoToMarginOffer(req.Offer)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.store.Create(ctx, offer); err != nil {
		return nil, s.handleStoreError(err)
	}

	protoOffer, err := s.marginOfferToProto(offer)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to convert offer")
	}

	return &pb.CreateMarginOfferResponse{Offer: protoOffer}, nil
}

// GetMarginOffer retrieves a margin offer by ID
func (s *GRPCServer) GetMarginOffer(ctx context.Context, req *pb.GetMarginOfferRequest) (*pb.GetMarginOfferResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	offer, err := s.store.GetByID(ctx, req.Id)
	if err != nil {
		return nil, s.handleStoreError(err)
	}

	protoOffer, err := s.marginOfferToProto(offer)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to convert offer")
	}

	return &pb.GetMarginOfferResponse{Offer: protoOffer}, nil
}

// UpdateMarginOffer updates an existing margin offer
func (s *GRPCServer) UpdateMarginOffer(ctx context.Context, req *pb.UpdateMarginOfferRequest) (*pb.UpdateMarginOfferResponse, error) {
	if req.Offer == nil {
		return nil, status.Error(codes.InvalidArgument, "offer is required")
	}

	offer, err := s.protoToMarginOffer(req.Offer)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.store.Update(ctx, offer); err != nil {
		return nil, s.handleStoreError(err)
	}

	protoOffer, err := s.marginOfferToProto(offer)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to convert offer")
	}

	return &pb.UpdateMarginOfferResponse{Offer: protoOffer}, nil
}

// DeleteMarginOffer deletes a margin offer
func (s *GRPCServer) DeleteMarginOffer(ctx context.Context, req *pb.DeleteMarginOfferRequest) (*emptypb.Empty, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	if err := s.store.Delete(ctx, req.Id); err != nil {
		return nil, s.handleStoreError(err)
	}

	return &emptypb.Empty{}, nil
}

// ListMarginOffers lists margin offers with pagination and filtering
func (s *GRPCServer) ListMarginOffers(ctx context.Context, req *pb.ListMarginOffersRequest) (*pb.ListMarginOffersResponse, error) {
	listReq, err := s.protoToListRequest(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	offers, err := s.store.List(ctx, listReq)
	if err != nil {
		return nil, s.handleStoreError(err)
	}

	total, err := s.store.Count(ctx)
	if err != nil {
		return nil, s.handleStoreError(err)
	}

	protoOffers := make([]*pb.MarginOffer, len(offers))
	for i, offer := range offers {
		protoOffer, err := s.marginOfferToProto(offer)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to convert offer")
		}
		protoOffers[i] = protoOffer
	}

	hasMore := int64(listReq.Offset+listReq.Limit) < total
	var nextOffset *int32
	if hasMore {
		next := listReq.Offset + listReq.Limit
		nextOffset = &next
	}

	return &pb.ListMarginOffersResponse{
		Offers:     protoOffers,
		Total:      total,
		Limit:      listReq.Limit,
		Offset:     listReq.Offset,
		HasMore:    hasMore,
		NextOffset: nextOffset,
	}, nil
}

// BulkCreateMarginOffers creates multiple margin offers
func (s *GRPCServer) BulkCreateMarginOffers(ctx context.Context, req *pb.BulkCreateMarginOffersRequest) (*pb.BulkOperationResponse, error) {
	if len(req.Offers) == 0 {
		return nil, status.Error(codes.InvalidArgument, "offers are required")
	}

	offers := make([]*types.MarginOffer, len(req.Offers))
	for i, protoOffer := range req.Offers {
		offer, err := s.protoToMarginOffer(protoOffer)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid offer at index %d: %v", i, err))
		}
		offers[i] = offer
	}

	err := s.store.BulkCreate(ctx, offers)

	if err != nil {
		return &pb.BulkOperationResponse{
			SuccessCount: 0,
			FailureCount: int32(len(offers)),
			Errors: []*pb.BulkError{{
				Index: 0,
				Error: err.Error(),
			}},
		}, nil
	}

	processedIDs := make([]string, len(offers))
	for i, offer := range offers {
		processedIDs[i] = offer.ID
	}

	return &pb.BulkOperationResponse{
		SuccessCount: int32(len(offers)),
		FailureCount: 0,
		ProcessedIds: processedIDs,
	}, nil
}

// CreateOrUpdateMarginOffer creates or updates a margin offer (upsert)
func (s *GRPCServer) CreateOrUpdateMarginOffer(ctx context.Context, req *pb.CreateOrUpdateMarginOfferRequest) (*pb.CreateOrUpdateMarginOfferResponse, error) {
	if req.Offer == nil {
		return nil, status.Error(codes.InvalidArgument, "offer is required")
	}

	offer, err := s.protoToMarginOffer(req.Offer)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Check if offer exists to determine if it's a create or update
	_, err = s.store.GetByID(ctx, offer.ID)
	wasCreated := false

	if err != nil {
		if err == types.ErrOfferNotFound {
			// Offer doesn't exist, create it
			if err := s.store.Create(ctx, offer); err != nil {
				return nil, s.handleStoreError(err)
			}
			wasCreated = true
		} else {
			return nil, s.handleStoreError(err)
		}
	} else {
		// Offer exists, update it
		if err := s.store.Update(ctx, offer); err != nil {
			return nil, s.handleStoreError(err)
		}
		wasCreated = false
	}

	protoOffer, err := s.marginOfferToProto(offer)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to convert offer")
	}

	return &pb.CreateOrUpdateMarginOfferResponse{
		Offer:      protoOffer,
		WasCreated: wasCreated,
	}, nil
}

// BulkUpdateMarginOffers updates multiple margin offers
func (s *GRPCServer) BulkUpdateMarginOffers(ctx context.Context, req *pb.BulkUpdateMarginOffersRequest) (*pb.BulkOperationResponse, error) {
	if len(req.Offers) == 0 {
		return nil, status.Error(codes.InvalidArgument, "offers are required")
	}

	offers := make([]*types.MarginOffer, len(req.Offers))
	for i, protoOffer := range req.Offers {
		offer, err := s.protoToMarginOffer(protoOffer)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid offer at index %d: %v", i, err))
		}
		offers[i] = offer
	}

	err := s.store.BulkUpdate(ctx, offers)
	if err != nil {
		return &pb.BulkOperationResponse{
			SuccessCount: 0,
			FailureCount: int32(len(offers)),
			Errors: []*pb.BulkError{{
				Index: 0,
				Error: err.Error(),
			}},
		}, nil
	}

	processedIDs := make([]string, len(offers))
	for i, offer := range offers {
		processedIDs[i] = offer.ID
	}

	return &pb.BulkOperationResponse{
		SuccessCount: int32(len(offers)),
		FailureCount: 0,
		ProcessedIds: processedIDs,
	}, nil
}

// BulkDeleteMarginOffers deletes multiple margin offers
func (s *GRPCServer) BulkDeleteMarginOffers(ctx context.Context, req *pb.BulkDeleteMarginOffersRequest) (*pb.BulkOperationResponse, error) {
	if len(req.Ids) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ids are required")
	}

	err := s.store.BulkDelete(ctx, req.Ids)
	if err != nil {
		return &pb.BulkOperationResponse{
			SuccessCount: 0,
			FailureCount: int32(len(req.Ids)),
			Errors: []*pb.BulkError{{
				Index: 0,
				Error: err.Error(),
			}},
		}, nil
	}

	return &pb.BulkOperationResponse{
		SuccessCount: int32(len(req.Ids)),
		FailureCount: 0,
		ProcessedIds: req.Ids,
	}, nil
}

// BulkCreateOrUpdateMarginOffers creates or updates multiple margin offers (bulk upsert)
func (s *GRPCServer) BulkCreateOrUpdateMarginOffers(ctx context.Context, req *pb.BulkCreateOrUpdateMarginOffersRequest) (*pb.BulkOperationResponse, error) {
	if len(req.Offers) == 0 {
		return nil, status.Error(codes.InvalidArgument, "offers are required")
	}

	offers := make([]*types.MarginOffer, len(req.Offers))
	for i, protoOffer := range req.Offers {
		offer, err := s.protoToMarginOffer(protoOffer)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid offer at index %d: %v", i, err))
		}
		offers[i] = offer
	}

	err := s.store.BulkCreateOrUpdate(ctx, offers)
	if err != nil {
		return &pb.BulkOperationResponse{
			SuccessCount: 0,
			FailureCount: int32(len(offers)),
			Errors: []*pb.BulkError{{
				Index: 0,
				Error: err.Error(),
			}},
		}, nil
	}

	processedIDs := make([]string, len(offers))
	for i, offer := range offers {
		processedIDs[i] = offer.ID
	}

	return &pb.BulkOperationResponse{
		SuccessCount: int32(len(offers)),
		FailureCount: 0,
		ProcessedIds: processedIDs,
	}, nil
}

// BulkOverwriteMarginOffers overwrites margin offers based on filter
func (s *GRPCServer) BulkOverwriteMarginOffers(ctx context.Context, req *pb.BulkOverwriteMarginOffersRequest) (*pb.BulkOverwriteResponse, error) {
	if len(req.Offers) == 0 && req.Filter == nil {
		return nil, status.Error(codes.InvalidArgument, "either offers or filter must be provided")
	}

	offers := make([]*types.MarginOffer, len(req.Offers))
	for i, protoOffer := range req.Offers {
		offer, err := s.protoToMarginOffer(protoOffer)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid offer at index %d: %v", i, err))
		}
		offers[i] = offer
	}

	// Convert filter if provided
	var filter *types.OverwriteFilter
	if req.Filter != nil {
		var err error
		filter, err = s.protoToOverwriteFilter(req.Filter)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid filter: %v", err))
		}
	}

	// Get stats before overwrite for preview
	var deletedCount int64
	var deletedIDs []string
	var err error
	if filter != nil {
		deletedCount, err = s.store.GetOverwriteStats(ctx, filter)
		if err != nil {
			return nil, s.handleStoreError(err)
		}
		// Note: In a real implementation, you might want to get the actual IDs that would be deleted
		// For now, we'll just use the count
	}

	// Perform the overwrite
	if filter != nil {
		err := s.store.BulkOverwriteByFilter(ctx, offers, filter)
		if err != nil {
			return nil, s.handleStoreError(err)
		}
	} else {
		err := s.store.BulkOverwrite(ctx, offers)
		if err != nil {
			return nil, s.handleStoreError(err)
		}
	}

	createdIDs := make([]string, len(offers))
	for i, offer := range offers {
		createdIDs[i] = offer.ID
	}

	return &pb.BulkOverwriteResponse{
		DeletedCount: int32(deletedCount),
		CreatedCount: int32(len(offers)),
		DeletedIds:   deletedIDs,
		CreatedIds:   createdIDs,
	}, nil
}

// OverwritePreview previews what would be affected by an overwrite operation
func (s *GRPCServer) OverwritePreview(ctx context.Context, req *pb.OverwritePreviewRequest) (*pb.OverwritePreviewResponse, error) {
	var filter *types.OverwriteFilter
	if req.Filter != nil {
		var err error
		filter, err = s.protoToOverwriteFilter(req.Filter)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid filter: %v", err))
		}
	}

	affectedCount, err := s.store.GetOverwriteStats(ctx, filter)
	if err != nil {
		return nil, s.handleStoreError(err)
	}

	// Note: In a real implementation, you might want to get the actual IDs that would be affected
	// For now, we'll just return the count
	var affectedIDs []string

	return &pb.OverwritePreviewResponse{
		AffectedCount: int32(affectedCount),
		AffectedIds:   affectedIDs,
	}, nil
}

// HealthCheck performs a health check
func (s *GRPCServer) HealthCheck(ctx context.Context, req *emptypb.Empty) (*pb.HealthCheckResponse, error) {
	status := "healthy"
	checks := make(map[string]string)

	if err := s.store.HealthCheck(ctx); err != nil {
		status = "unhealthy"
		checks["store"] = err.Error()
	} else {
		checks["store"] = "healthy"
	}

	return &pb.HealthCheckResponse{
		Status:    status,
		Timestamp: timestamppb.Now(),
		Version:   "1.0.0",
		Checks:    checks,
	}, nil
}

// Helper method to handle store errors
func (s *GRPCServer) handleStoreError(err error) error {
	switch err {
	case types.ErrOfferNotFound:
		return status.Error(codes.NotFound, err.Error())
	case types.ErrOfferAlreadyExists:
		return status.Error(codes.AlreadyExists, err.Error())
	case types.ErrMissingID, types.ErrInvalidEventData:
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

// SubscribeMarginOffers implements the subscription service
func (s *GRPCServer) SubscribeMarginOffers(req *pb.SubscribeMarginOffersRequest, stream pb.MarginOfferService_SubscribeMarginOffersServer) error {
	// Get the notification service from the store
	memoryStore, ok := s.store.(*MemoryStore)
	if !ok {
		return status.Errorf(codes.Internal, "store does not support notifications")
	}

	// Create subscription
	subscription, err := memoryStore.notificationService.Subscribe(req)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create subscription: %v", err)
	}
	defer memoryStore.notificationService.Unsubscribe(subscription.ID)

	// Send initial confirmation
	initialResponse := &pb.SubscribeMarginOffersResponse{
		Event: &pb.SubscribeMarginOffersResponse_Change{
			Change: &pb.MarginOfferChange{
				ChangeType:  pb.MarginOfferChange_UNKNOWN,
				Timestamp:   timestamppb.Now(),
				Source:      "subscription_service",
				OperationId: subscription.ID,
				Metadata: map[string]string{
					"message":   "subscription_established",
					"client_id": subscription.ClientID,
				},
			},
		},
	}

	if err := stream.Send(initialResponse); err != nil {
		return status.Errorf(codes.Internal, "failed to send initial response: %v", err)
	}

	// Stream notifications
	for {
		select {
		case response := <-subscription.Channel:
			if err := stream.Send(response); err != nil {
				return status.Errorf(codes.Internal, "failed to send notification: %v", err)
			}
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "client disconnected")
		}
	}
}

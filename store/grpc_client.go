package store

import (
	"context"
	"fmt"

	pb "github.com/piske-alex/margin-offer-system/proto/gen/go/proto"
	"github.com/piske-alex/margin-offer-system/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCClient implements the MarginOfferStore interface using gRPC
type GRPCClient struct {
	client pb.MarginOfferServiceClient
	conn   *grpc.ClientConn
	logger types.Logger
}

// NewGRPCClient creates a new gRPC client for the margin offer store
func NewGRPCClient(addr string, logger types.Logger) (*GRPCClient, error) {
	// Create gRPC connection
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to store service: %w", err)
	}

	// Create gRPC client
	client := pb.NewMarginOfferServiceClient(conn)

	return &GRPCClient{
		client: client,
		conn:   conn,
		logger: logger,
	}, nil
}

// Close closes the gRPC connection
func (g *GRPCClient) Close() error {
	return g.conn.Close()
}

// HealthCheck checks if the store service is healthy
func (g *GRPCClient) HealthCheck(ctx context.Context) error {
	_, err := g.client.HealthCheck(ctx, &emptypb.Empty{})
	return err
}

// Create adds a new margin offer
func (g *GRPCClient) Create(ctx context.Context, offer *types.MarginOffer) error {
	req := &pb.CreateMarginOfferRequest{
		Offer: convertToProtoMarginOffer(offer),
	}

	_, err := g.client.CreateMarginOffer(ctx, req)
	return err
}

// GetByID retrieves a margin offer by ID
func (g *GRPCClient) GetByID(ctx context.Context, id string) (*types.MarginOffer, error) {
	req := &pb.GetMarginOfferRequest{Id: id}

	resp, err := g.client.GetMarginOffer(ctx, req)
	if err != nil {
		return nil, err
	}

	return convertFromProtoMarginOffer(resp.Offer), nil
}

// Update modifies an existing margin offer
func (g *GRPCClient) Update(ctx context.Context, offer *types.MarginOffer) error {
	req := &pb.UpdateMarginOfferRequest{
		Offer: convertToProtoMarginOffer(offer),
	}

	_, err := g.client.UpdateMarginOffer(ctx, req)
	return err
}

// Delete removes a margin offer
func (g *GRPCClient) Delete(ctx context.Context, id string) error {
	req := &pb.DeleteMarginOfferRequest{Id: id}

	_, err := g.client.DeleteMarginOffer(ctx, req)
	return err
}

// CreateOrUpdate creates or updates a margin offer (upsert)
func (g *GRPCClient) CreateOrUpdate(ctx context.Context, offer *types.MarginOffer) error {
	req := &pb.CreateOrUpdateMarginOfferRequest{
		Offer: convertToProtoMarginOffer(offer),
	}

	_, err := g.client.CreateOrUpdateMarginOffer(ctx, req)
	return err
}

// List retrieves margin offers with filtering and pagination
func (g *GRPCClient) List(ctx context.Context, req *types.ListRequest) ([]*types.MarginOffer, error) {
	pbReq := &pb.ListMarginOffersRequest{
		Limit:  req.Limit,
		Offset: req.Offset,
	}

	if req.SortBy != "" {
		pbReq.SortBy = &req.SortBy
	}
	if req.SortOrder != "" {
		pbReq.SortOrder = &req.SortOrder
	}

	resp, err := g.client.ListMarginOffers(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	offers := make([]*types.MarginOffer, len(resp.Offers))
	for i, pbOffer := range resp.Offers {
		offers[i] = convertFromProtoMarginOffer(pbOffer)
	}

	return offers, nil
}

// ListByLender retrieves margin offers by lender address
func (g *GRPCClient) ListByLender(ctx context.Context, lenderAddress string, req *types.ListRequest) ([]*types.MarginOffer, error) {
	pbReq := &pb.ListMarginOffersRequest{
		Limit:         req.Limit,
		Offset:        req.Offset,
		LenderAddress: &lenderAddress,
	}

	if req.SortBy != "" {
		pbReq.SortBy = &req.SortBy
	}
	if req.SortOrder != "" {
		pbReq.SortOrder = &req.SortOrder
	}

	resp, err := g.client.ListMarginOffers(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	offers := make([]*types.MarginOffer, len(resp.Offers))
	for i, pbOffer := range resp.Offers {
		offers[i] = convertFromProtoMarginOffer(pbOffer)
	}

	return offers, nil
}

// ListByToken retrieves margin offers by token pair
func (g *GRPCClient) ListByToken(ctx context.Context, collateralToken, borrowToken string, req *types.ListRequest) ([]*types.MarginOffer, error) {
	pbReq := &pb.ListMarginOffersRequest{
		Limit:           req.Limit,
		Offset:          req.Offset,
		CollateralToken: &collateralToken,
		BorrowToken:     &borrowToken,
	}

	if req.SortBy != "" {
		pbReq.SortBy = &req.SortBy
	}
	if req.SortOrder != "" {
		pbReq.SortOrder = &req.SortOrder
	}

	resp, err := g.client.ListMarginOffers(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	offers := make([]*types.MarginOffer, len(resp.Offers))
	for i, pbOffer := range resp.Offers {
		offers[i] = convertFromProtoMarginOffer(pbOffer)
	}

	return offers, nil
}

// ListByOfferType retrieves margin offers by offer type
func (g *GRPCClient) ListByOfferType(ctx context.Context, offerType types.OfferType, req *types.ListRequest) ([]*types.MarginOffer, error) {
	offerTypeStr := string(offerType)
	pbReq := &pb.ListMarginOffersRequest{
		Limit:     req.Limit,
		Offset:    req.Offset,
		OfferType: &offerTypeStr,
	}

	if req.SortBy != "" {
		pbReq.SortBy = &req.SortBy
	}
	if req.SortOrder != "" {
		pbReq.SortOrder = &req.SortOrder
	}

	resp, err := g.client.ListMarginOffers(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	offers := make([]*types.MarginOffer, len(resp.Offers))
	for i, pbOffer := range resp.Offers {
		offers[i] = convertFromProtoMarginOffer(pbOffer)
	}

	return offers, nil
}

// BulkCreate creates multiple margin offers
func (g *GRPCClient) BulkCreate(ctx context.Context, offers []*types.MarginOffer) error {
	pbOffers := make([]*pb.MarginOffer, len(offers))
	for i, offer := range offers {
		pbOffers[i] = convertToProtoMarginOffer(offer)
	}

	req := &pb.BulkCreateMarginOffersRequest{
		Offers: pbOffers,
	}

	_, err := g.client.BulkCreateMarginOffers(ctx, req)
	return err
}

// BulkUpdate updates multiple margin offers
func (g *GRPCClient) BulkUpdate(ctx context.Context, offers []*types.MarginOffer) error {
	pbOffers := make([]*pb.MarginOffer, len(offers))
	for i, offer := range offers {
		pbOffers[i] = convertToProtoMarginOffer(offer)
	}

	req := &pb.BulkUpdateMarginOffersRequest{
		Offers: pbOffers,
	}

	_, err := g.client.BulkUpdateMarginOffers(ctx, req)
	return err
}

// BulkDelete deletes multiple margin offers
func (g *GRPCClient) BulkDelete(ctx context.Context, ids []string) error {
	req := &pb.BulkDeleteMarginOffersRequest{
		Ids: ids,
	}

	_, err := g.client.BulkDeleteMarginOffers(ctx, req)
	return err
}

// BulkCreateOrUpdate creates or updates multiple margin offers
func (g *GRPCClient) BulkCreateOrUpdate(ctx context.Context, offers []*types.MarginOffer) error {
	pbOffers := make([]*pb.MarginOffer, len(offers))
	for i, offer := range offers {
		pbOffers[i] = convertToProtoMarginOffer(offer)
	}

	req := &pb.BulkCreateOrUpdateMarginOffersRequest{
		Offers: pbOffers,
	}

	_, err := g.client.BulkCreateOrUpdateMarginOffers(ctx, req)
	return err
}

// BulkOverwrite replaces all margin offers with new ones
func (g *GRPCClient) BulkOverwrite(ctx context.Context, offers []*types.MarginOffer) error {
	pbOffers := make([]*pb.MarginOffer, len(offers))
	for i, offer := range offers {
		pbOffers[i] = convertToProtoMarginOffer(offer)
	}

	req := &pb.BulkOverwriteMarginOffersRequest{
		Offers: pbOffers,
	}

	_, err := g.client.BulkOverwriteMarginOffers(ctx, req)
	return err
}

// BulkOverwriteByFilter replaces margin offers matching the filter criteria
func (g *GRPCClient) BulkOverwriteByFilter(ctx context.Context, offers []*types.MarginOffer, filter *types.OverwriteFilter) error {
	pbOffers := make([]*pb.MarginOffer, len(offers))
	for i, offer := range offers {
		pbOffers[i] = convertToProtoMarginOffer(offer)
	}

	req := &pb.BulkOverwriteMarginOffersRequest{
		Offers: pbOffers,
	}

	if filter != nil {
		req.Filter = convertToProtoOverwriteFilter(filter)
	}

	_, err := g.client.BulkOverwriteMarginOffers(ctx, req)
	return err
}

// Count returns the total number of margin offers
func (g *GRPCClient) Count(ctx context.Context) (int64, error) {
	req := &pb.ListMarginOffersRequest{
		Limit:  1,
		Offset: 0,
	}

	resp, err := g.client.ListMarginOffers(ctx, req)
	if err != nil {
		return 0, err
	}

	return resp.Total, nil
}

// CountByOfferType returns the number of margin offers by offer type
func (g *GRPCClient) CountByOfferType(ctx context.Context, offerType types.OfferType) (int64, error) {
	offerTypeStr := string(offerType)
	req := &pb.ListMarginOffersRequest{
		Limit:     1,
		Offset:    0,
		OfferType: &offerTypeStr,
	}

	resp, err := g.client.ListMarginOffers(ctx, req)
	if err != nil {
		return 0, err
	}

	return resp.Total, nil
}

// GetTotalLiquidity calculates the total liquidity for a specific borrow token
func (g *GRPCClient) GetTotalLiquidity(ctx context.Context, borrowToken string) (float64, error) {
	req := &pb.ListMarginOffersRequest{
		Limit:       10000, // Large limit to get all offers
		Offset:      0,
		BorrowToken: &borrowToken,
	}

	resp, err := g.client.ListMarginOffers(ctx, req)
	if err != nil {
		return 0, err
	}

	var totalLiquidity float64
	for _, pbOffer := range resp.Offers {
		totalLiquidity += pbOffer.AvailableBorrowAmount
	}

	return totalLiquidity, nil
}

// GetOverwriteStats returns statistics about what would be affected by an overwrite operation
func (g *GRPCClient) GetOverwriteStats(ctx context.Context, filter *types.OverwriteFilter) (int64, error) {
	req := &pb.OverwritePreviewRequest{}
	if filter != nil {
		req.Filter = convertToProtoOverwriteFilter(filter)
	}

	resp, err := g.client.OverwritePreview(ctx, req)
	if err != nil {
		return 0, err
	}

	return int64(resp.AffectedCount), nil
}

// Helper functions for converting between types and protobuf messages

func convertToProtoMarginOffer(offer *types.MarginOffer) *pb.MarginOffer {
	pbOffer := &pb.MarginOffer{
		Id:                    offer.ID,
		OfferType:             string(offer.OfferType),
		CollateralToken:       offer.CollateralToken,
		BorrowToken:           offer.BorrowToken,
		AvailableBorrowAmount: offer.AvailableBorrowAmount,
		MaxOpenLtv:            offer.MaxOpenLTV,
		LiquidationLtv:        offer.LiquidationLTV,
		InterestRate:          offer.InterestRate,
		InterestModel:         string(offer.InterestModel),
		LiquiditySource:       offer.LiquiditySource,
		CreatedTimestamp:      timestamppb.New(offer.CreatedTimestamp),
		UpdatedTimestamp:      timestamppb.New(offer.UpdatedTimestamp),
	}

	if offer.TermDays != nil {
		pbOffer.TermDays = offer.TermDays
	}
	if offer.LenderAddress != nil {
		pbOffer.LenderAddress = offer.LenderAddress
	}
	if offer.LastBorrowedTimestamp != nil {
		pbOffer.LastBorrowedTimestamp = timestamppb.New(*offer.LastBorrowedTimestamp)
	}

	return pbOffer
}

func convertFromProtoMarginOffer(pbOffer *pb.MarginOffer) *types.MarginOffer {
	offer := &types.MarginOffer{
		ID:                    pbOffer.Id,
		OfferType:             types.OfferType(pbOffer.OfferType),
		CollateralToken:       pbOffer.CollateralToken,
		BorrowToken:           pbOffer.BorrowToken,
		AvailableBorrowAmount: pbOffer.AvailableBorrowAmount,
		MaxOpenLTV:            pbOffer.MaxOpenLtv,
		LiquidationLTV:        pbOffer.LiquidationLtv,
		InterestRate:          pbOffer.InterestRate,
		InterestModel:         types.InterestModel(pbOffer.InterestModel),
		LiquiditySource:       pbOffer.LiquiditySource,
		CreatedTimestamp:      pbOffer.CreatedTimestamp.AsTime(),
		UpdatedTimestamp:      pbOffer.UpdatedTimestamp.AsTime(),
	}

	if pbOffer.TermDays != nil {
		offer.TermDays = pbOffer.TermDays
	}
	if pbOffer.LenderAddress != nil {
		offer.LenderAddress = pbOffer.LenderAddress
	}
	if pbOffer.LastBorrowedTimestamp != nil {
		lastBorrowed := pbOffer.LastBorrowedTimestamp.AsTime()
		offer.LastBorrowedTimestamp = &lastBorrowed
	}

	return offer
}

func convertToProtoOverwriteFilter(filter *types.OverwriteFilter) *pb.OverwriteFilter {
	pbFilter := &pb.OverwriteFilter{}

	if filter.OfferType != nil {
		offerTypeStr := string(*filter.OfferType)
		pbFilter.OfferType = &offerTypeStr
	}
	if filter.CollateralToken != nil {
		pbFilter.CollateralToken = filter.CollateralToken
	}
	if filter.BorrowToken != nil {
		pbFilter.BorrowToken = filter.BorrowToken
	}
	if filter.LenderAddress != nil {
		pbFilter.LenderAddress = filter.LenderAddress
	}
	if filter.LiquiditySource != nil {
		pbFilter.LiquiditySource = filter.LiquiditySource
	}
	if filter.CreatedAfter != nil {
		pbFilter.CreatedAfter = timestamppb.New(*filter.CreatedAfter)
	}
	if filter.CreatedBefore != nil {
		pbFilter.CreatedBefore = timestamppb.New(*filter.CreatedBefore)
	}
	if filter.UpdatedAfter != nil {
		pbFilter.UpdatedAfter = timestamppb.New(*filter.UpdatedAfter)
	}
	if filter.UpdatedBefore != nil {
		pbFilter.UpdatedBefore = timestamppb.New(*filter.UpdatedBefore)
	}

	return pbFilter
}

package store

import (
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/piske-alex/margin-offer-system/proto/gen/go/proto"
	"github.com/piske-alex/margin-offer-system/types"
)

// protoToMarginOffer converts a protobuf MarginOffer to types.MarginOffer
func (s *GRPCServer) protoToMarginOffer(proto *pb.MarginOffer) (*types.MarginOffer, error) {
	if proto == nil {
		return nil, fmt.Errorf("proto offer is nil")
	}

	// Validate and convert offer type
	var offerType types.OfferType
	switch proto.OfferType {
	case "V1":
		offerType = types.OfferTypeV1
	case "BP":
		offerType = types.OfferTypeBP
	case "LV":
		offerType = types.OfferTypeLV
	default:
		return nil, fmt.Errorf("invalid offer type: %s", proto.OfferType)
	}

	// Validate and convert interest model
	var interestModel types.InterestModel
	switch proto.InterestModel {
	case "fixed":
		interestModel = types.InterestModelFixed
	case "floating":
		interestModel = types.InterestModelFloating
	default:
		return nil, fmt.Errorf("invalid interest model: %s", proto.InterestModel)
	}

	offer := &types.MarginOffer{
		ID:                    proto.Id,
		OfferType:             offerType,
		CollateralToken:       proto.CollateralToken,
		BorrowToken:           proto.BorrowToken,
		AvailableBorrowAmount: proto.AvailableBorrowAmount,
		MaxOpenLTV:            proto.MaxOpenLtv,
		LiquidationLTV:        proto.LiquidationLtv,
		InterestRate:          proto.InterestRate,
		InterestModel:         interestModel,
		LiquiditySource:       proto.LiquiditySource,
	}

	// Handle optional fields
	if proto.TermDays != nil {
		term := *proto.TermDays
		offer.TermDays = &term
	}

	if proto.LenderAddress != nil {
		lender := *proto.LenderAddress
		offer.LenderAddress = &lender
	}

	if proto.LastBorrowedTimestamp != nil {
		lastBorrowed := proto.LastBorrowedTimestamp.AsTime()
		offer.LastBorrowedTimestamp = &lastBorrowed
	}

	if proto.Source != nil {
		source := *proto.Source
		offer.Source = &source
	}

	if proto.CreatedTimestamp != nil {
		offer.CreatedTimestamp = proto.CreatedTimestamp.AsTime()
	}

	if proto.UpdatedTimestamp != nil {
		offer.UpdatedTimestamp = proto.UpdatedTimestamp.AsTime()
	}

	return offer, nil
}

// marginOfferToProto converts types.MarginOffer to protobuf MarginOffer
func (s *GRPCServer) marginOfferToProto(offer *types.MarginOffer) (*pb.MarginOffer, error) {
	if offer == nil {
		return nil, fmt.Errorf("offer is nil")
	}

	proto := &pb.MarginOffer{
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

	// Handle optional fields
	if offer.TermDays != nil {
		proto.TermDays = offer.TermDays
	}

	if offer.LenderAddress != nil {
		proto.LenderAddress = offer.LenderAddress
	}

	if offer.LastBorrowedTimestamp != nil {
		proto.LastBorrowedTimestamp = timestamppb.New(*offer.LastBorrowedTimestamp)
	}

	if offer.Source != nil {
		proto.Source = offer.Source
	}

	return proto, nil
}

// protoToOverwriteFilter converts protobuf OverwriteFilter to types.OverwriteFilter
func (s *GRPCServer) protoToOverwriteFilter(proto *pb.OverwriteFilter) (*types.OverwriteFilter, error) {
	if proto == nil {
		return nil, nil
	}

	filter := &types.OverwriteFilter{}

	// Handle offer type
	if proto.OfferType != nil {
		switch *proto.OfferType {
		case "V1":
			offerType := types.OfferTypeV1
			filter.OfferType = &offerType
		case "BP":
			offerType := types.OfferTypeBP
			filter.OfferType = &offerType
		case "LV":
			offerType := types.OfferTypeLV
			filter.OfferType = &offerType
		default:
			return nil, fmt.Errorf("invalid offer type: %s", *proto.OfferType)
		}
	}

	// Handle string fields
	if proto.CollateralToken != nil {
		filter.CollateralToken = proto.CollateralToken
	}
	if proto.BorrowToken != nil {
		filter.BorrowToken = proto.BorrowToken
	}
	if proto.LenderAddress != nil {
		filter.LenderAddress = proto.LenderAddress
	}
	if proto.LiquiditySource != nil {
		filter.LiquiditySource = proto.LiquiditySource
	}
	if proto.Source != nil {
		filter.Source = proto.Source
	}

	// Handle time fields
	if proto.CreatedAfter != nil {
		createdAfter := proto.CreatedAfter.AsTime()
		filter.CreatedAfter = &createdAfter
	}
	if proto.CreatedBefore != nil {
		createdBefore := proto.CreatedBefore.AsTime()
		filter.CreatedBefore = &createdBefore
	}
	if proto.UpdatedAfter != nil {
		updatedAfter := proto.UpdatedAfter.AsTime()
		filter.UpdatedAfter = &updatedAfter
	}
	if proto.UpdatedBefore != nil {
		updatedBefore := proto.UpdatedBefore.AsTime()
		filter.UpdatedBefore = &updatedBefore
	}

	return filter, nil
}

// protoToListRequest converts protobuf ListRequest to types.ListRequest
func (s *GRPCServer) protoToListRequest(proto *pb.ListMarginOffersRequest) (*types.ListRequest, error) {
	if proto == nil {
		return &types.ListRequest{Limit: 100, Offset: 0}, nil
	}

	req := &types.ListRequest{
		Limit:          proto.Limit,
		Offset:         proto.Offset,
		IncludeExpired: proto.IncludeExpired,
		ActiveOnly:     proto.ActiveOnly,
	}

	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.Limit > 1000 {
		req.Limit = 1000
	}

	// Handle optional string fields
	if proto.SortBy != nil {
		req.SortBy = *proto.SortBy
	}
	if proto.SortOrder != nil {
		req.SortOrder = *proto.SortOrder
	}
	if proto.CollateralToken != nil {
		req.CollateralToken = proto.CollateralToken
	}
	if proto.BorrowToken != nil {
		req.BorrowToken = proto.BorrowToken
	}
	if proto.LiquiditySource != nil {
		req.LiquiditySource = proto.LiquiditySource
	}
	if proto.LenderAddress != nil {
		req.LenderAddress = proto.LenderAddress
	}
	if proto.Source != nil {
		req.Source = proto.Source
	}

	// Handle offer type
	if proto.OfferType != nil {
		switch *proto.OfferType {
		case "V1":
			offerType := types.OfferTypeV1
			req.OfferType = &offerType
		case "BP":
			offerType := types.OfferTypeBP
			req.OfferType = &offerType
		case "LV":
			offerType := types.OfferTypeLV
			req.OfferType = &offerType
		default:
			return nil, fmt.Errorf("invalid offer type: %s", *proto.OfferType)
		}
	}

	// Handle interest model
	if proto.InterestModel != nil {
		switch *proto.InterestModel {
		case "fixed":
			interestModel := types.InterestModelFixed
			req.InterestModel = &interestModel
		case "floating":
			interestModel := types.InterestModelFloating
			req.InterestModel = &interestModel
		default:
			return nil, fmt.Errorf("invalid interest model: %s", *proto.InterestModel)
		}
	}

	// Handle range filters
	if proto.MinBorrowAmount != nil {
		req.MinBorrowAmount = proto.MinBorrowAmount
	}
	if proto.MaxBorrowAmount != nil {
		req.MaxBorrowAmount = proto.MaxBorrowAmount
	}
	if proto.MinInterestRate != nil {
		req.MinInterestRate = proto.MinInterestRate
	}
	if proto.MaxInterestRate != nil {
		req.MaxInterestRate = proto.MaxInterestRate
	}

	// Handle time filters
	if proto.CreatedAfter != nil {
		createdAfter := proto.CreatedAfter.AsTime()
		req.CreatedAfter = &createdAfter
	}
	if proto.CreatedBefore != nil {
		createdBefore := proto.CreatedBefore.AsTime()
		req.CreatedBefore = &createdBefore
	}

	return req, nil
}

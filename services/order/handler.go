package main

import (
	"context"

	"github.com/google/uuid"
	order "github.com/pavelmaksimov25/go-oms/pkg/proto/order/v1"
	saga "github.com/pavelmaksimov25/go-oms/pkg/proto/saga/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	topicSagaOrderCreated = "saga.order.created"
	eventOrderCreated     = "order.created"
)

type Publisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

type Handler struct {
	order.UnimplementedOrderServiceServer
	store     *Store
	publisher Publisher
}

func NewHandler(store *Store, publisher Publisher) *Handler {
	return &Handler{store: store, publisher: publisher}
}

func (h *Handler) CreateOrder(ctx context.Context, req *order.CreateOrderRequest) (*order.CreateOrderResponse, error) {
	orderID := uuid.NewString()

	o := &order.Order{
		OrderId:     orderID,
		Items:       req.GetItems(),
		TotalAmount: req.GetTotalAmount(),
		Status:      order.OrderStatus_ORDER_STATUS_PENDING,
	}
	h.store.Create(o)

	envelope, err := buildOrderCreatedEnvelope(orderID, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal event: %v", err)
	}
	if err := h.publisher.Publish(ctx, topicSagaOrderCreated, []byte(orderID), envelope); err != nil {
		return nil, status.Errorf(codes.Internal, "publish event: %v", err)
	}

	return &order.CreateOrderResponse{
		OrderId: orderID,
		Status:  order.OrderStatus_ORDER_STATUS_PENDING,
	}, nil
}

func (h *Handler) GetOrder(_ context.Context, req *order.GetOrderRequest) (*order.GetOrderResponse, error) {
	o, ok := h.store.Get(req.GetOrderId())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "order %s not found", req.GetOrderId())
	}
	return &order.GetOrderResponse{Order: o}, nil
}

func buildOrderCreatedEnvelope(orderID string, req *order.CreateOrderRequest) ([]byte, error) {
	sagaItems := make([]*saga.OrderItem, 0, len(req.GetItems()))
	for _, it := range req.GetItems() {
		sagaItems = append(sagaItems, &saga.OrderItem{ItemId: it.GetItemId(), Quantity: it.GetQuantity()})
	}
	payload, err := proto.Marshal(&saga.OrderCreatedEvent{
		OrderId:     orderID,
		Items:       sagaItems,
		TotalAmount: req.GetTotalAmount(),
	})
	if err != nil {
		return nil, err
	}
	return proto.Marshal(&saga.SagaEvent{
		SagaId:    orderID,
		EventType: eventOrderCreated,
		Payload:   payload,
		CreatedAt: timestamppb.Now(),
	})
}

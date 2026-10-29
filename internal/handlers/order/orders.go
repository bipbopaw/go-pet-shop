package order

import (
	"context"
	"errors"
	"go-pet-shop/internal/models"
	"go-pet-shop/internal/storage"
	"log/slog"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type Orders interface {
	PlaceOrder(ctx context.Context, userEmail string, items []models.OrderItem) (int, error)
	CreateOrder(ctx context.Context, order models.Order) (int, error)
	AddOrderItem(ctx context.Context, item models.OrderItem) (int, error)
	GetOrderByID(ctx context.Context, id int) (models.Order, error)
	GetOrdersByUserEmail(ctx context.Context, email string) ([]models.Order, error)
	GetOrderItemsByOrderID(ctx context.Context, id int) ([]models.OrderItem, error)
}

type Handler struct {
	log     *slog.Logger
	storage Orders
}

func New(log *slog.Logger, s Orders) *Handler { return &Handler{log: log, storage: s} }

func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var request struct {
		Email string             `json:"email"`
		Items []models.OrderItem `json:"items"`
	}
	if err := render.DecodeJSON(r.Body, &request); err != nil || request.Email == "" || len(request.Items) == 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"message": "email and at least one order item are required"})
		return
	}
	request.Email = strings.TrimSpace(request.Email)
	address, err := mail.ParseAddress(request.Email)
	if err != nil || address.Address != request.Email {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"message": "A valid email is required"})
		return
	}
	for _, item := range request.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, map[string]string{"message": "product_id and quantity must be positive"})
			return
		}
	}

	id, err := h.storage.PlaceOrder(r.Context(), request.Email, request.Items)
	if errors.Is(err, storage.ErrNotFound) {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{"message": "User or product not found"})
		return
	}
	if errors.Is(err, storage.ErrInsufficientStock) {
		render.Status(r, http.StatusConflict)
		render.JSON(w, r, map[string]string{"message": "Insufficient product stock"})
		return
	}
	if err != nil {
		h.log.Error("failed to place order", slog.Any("error", err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"message": "Failed to place order"})
		return
	}
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]int{"order_id": id})
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var order models.Order
	if err := render.DecodeJSON(r.Body, &order); err != nil || order.UserID <= 0 || order.TotalPrice < 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"message": "user_id must be positive and total_price cannot be negative"})
		return
	}
	id, err := h.storage.CreateOrder(r.Context(), order)
	if err != nil {
		h.log.Error("failed to create order", slog.Any("error", err))
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"message": "Failed to create order; check that the user exists"})
		return
	}
	order.ID = id
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, order)
}

func (h *Handler) AddOrderItem(w http.ResponseWriter, r *http.Request) {
	orderID, ok := parseID(w, r)
	if !ok {
		return
	}
	var item models.OrderItem
	if err := render.DecodeJSON(r.Body, &item); err != nil || item.ProductID <= 0 || item.Quantity <= 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"message": "product_id and quantity must be positive"})
		return
	}
	item.OrderID = orderID
	id, err := h.storage.AddOrderItem(r.Context(), item)
	if err != nil {
		h.log.Error("failed to add order item", slog.Any("error", err))
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"message": "Failed to add item; check that the order and product exist"})
		return
	}
	item.ID = id
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, item)
}

func (h *Handler) GetOrderByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	order, err := h.storage.GetOrderByID(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{"message": "Order not found"})
		return
	}
	if err != nil {
		h.log.Error("failed to get order", slog.Any("error", err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"message": "Failed to retrieve order"})
		return
	}
	render.JSON(w, r, order)
}

func (h *Handler) GetOrdersByUserEmail(w http.ResponseWriter, r *http.Request) {
	email := chi.URLParam(r, "email")
	orders, err := h.storage.GetOrdersByUserEmail(r.Context(), email)
	if err != nil {
		h.log.Error("failed to get user orders", slog.Any("error", err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"message": "Failed to retrieve orders"})
		return
	}
	render.JSON(w, r, orders)
}

func (h *Handler) GetUserOrderHistory(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(chi.URLParam(r, "email"))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"message": "A valid email is required"})
		return
	}
	historyStorage, ok := h.storage.(interface {
		GetUserOrderHistory(context.Context, string) ([]models.OrderDetail, error)
	})
	if !ok {
		h.log.Error("order history is not supported by storage")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"message": "Failed to retrieve order history"})
		return
	}
	history, err := historyStorage.GetUserOrderHistory(r.Context(), email)
	if err != nil {
		h.log.Error("failed to get user order history", slog.Any("error", err))
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"message": "Failed to retrieve order history"})
		return
	}
	render.JSON(w, r, history)
}

func parseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"message": "ID must be a positive number"})
		return 0, false
	}
	return id, true
}

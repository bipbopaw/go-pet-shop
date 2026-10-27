package order

import (
	"bytes"
	"context"
	"errors"
	"go-pet-shop/internal/models"
	"go-pet-shop/internal/storage"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
)

type mockOrders struct{ mock.Mock }

func (m *mockOrders) PlaceOrder(ctx context.Context, email string, items []models.OrderItem) (int, error) {
	args := m.Called(ctx, email, items)
	return args.Int(0), args.Error(1)
}

func (m *mockOrders) CreateOrder(ctx context.Context, order models.Order) (int, error) {
	args := m.Called(ctx, order)
	return args.Int(0), args.Error(1)
}

func (m *mockOrders) AddOrderItem(ctx context.Context, item models.OrderItem) (int, error) {
	args := m.Called(ctx, item)
	return args.Int(0), args.Error(1)
}

func (m *mockOrders) GetOrderByID(ctx context.Context, id int) (models.Order, error) {
	args := m.Called(ctx, id)
	var order models.Order
	if args.Get(0) != nil {
		order = args.Get(0).(models.Order)
	}
	return order, args.Error(1)
}

func (m *mockOrders) GetOrdersByUserEmail(ctx context.Context, email string) ([]models.Order, error) {
	args := m.Called(ctx, email)
	var orders []models.Order
	if args.Get(0) != nil {
		orders = args.Get(0).([]models.Order)
	}
	return orders, args.Error(1)
}

func (m *mockOrders) GetOrderItemsByOrderID(ctx context.Context, id int) ([]models.OrderItem, error) {
	args := m.Called(ctx, id)
	var items []models.OrderItem
	if args.Get(0) != nil {
		items = args.Get(0).([]models.OrderItem)
	}
	return items, args.Error(1)
}

func newMockOrders(t *testing.T) *mockOrders {
	t.Helper()
	m := &mockOrders{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

func withOrderParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestCreateOrder_Success(t *testing.T) {
	m := newMockOrders(t)
	order := models.Order{UserID: 4, TotalPrice: 32.5}
	m.On("CreateOrder", mock.Anything, order).Return(12, nil)
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`{"user_id":4,"total_price":32.5}`))
	w := httptest.NewRecorder()

	New(slog.Default(), m).CreateOrder(w, req)

	if w.Code != http.StatusCreated || !bytes.Contains(w.Body.Bytes(), []byte(`"id":12`)) {
		t.Fatalf("expected status 201 with order ID, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateOrder_InvalidInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{"user_id":`},
		{name: "missing user", body: `{"total_price":10}`},
		{name: "negative price", body: `{"user_id":4,"total_price":-1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockOrders(t)
			req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			New(slog.Default(), m).CreateOrder(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestCreateOrder_StorageError(t *testing.T) {
	m := newMockOrders(t)
	order := models.Order{UserID: 4, TotalPrice: 32.5}
	m.On("CreateOrder", mock.Anything, order).Return(0, errors.New("DB error"))
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`{"user_id":4,"total_price":32.5}`))
	w := httptest.NewRecorder()

	New(slog.Default(), m).CreateOrder(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestAddOrderItem_Success(t *testing.T) {
	m := newMockOrders(t)
	item := models.OrderItem{OrderID: 12, ProductID: 7, Quantity: 2}
	m.On("AddOrderItem", mock.Anything, item).Return(23, nil)
	req := httptest.NewRequest(http.MethodPost, "/orders/12/items", bytes.NewBufferString(`{"product_id":7,"quantity":2}`))
	req = withOrderParam(req, "id", "12")
	w := httptest.NewRecorder()

	New(slog.Default(), m).AddOrderItem(w, req)

	if w.Code != http.StatusCreated || !bytes.Contains(w.Body.Bytes(), []byte(`"id":23`)) || !bytes.Contains(w.Body.Bytes(), []byte(`"order_id":12`)) {
		t.Fatalf("expected status 201 with order item, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestAddOrderItem_InvalidInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{"product_id":`},
		{name: "missing product", body: `{"quantity":2}`},
		{name: "non-positive quantity", body: `{"product_id":7,"quantity":0}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockOrders(t)
			req := httptest.NewRequest(http.MethodPost, "/orders/12/items", bytes.NewBufferString(tt.body))
			req = withOrderParam(req, "id", "12")
			w := httptest.NewRecorder()

			New(slog.Default(), m).AddOrderItem(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestAddOrderItem_BadOrderID(t *testing.T) {
	m := newMockOrders(t)
	req := httptest.NewRequest(http.MethodPost, "/orders/nope/items", bytes.NewBufferString(`{"product_id":7,"quantity":1}`))
	req = withOrderParam(req, "id", "nope")
	w := httptest.NewRecorder()

	New(slog.Default(), m).AddOrderItem(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestAddOrderItem_StorageError(t *testing.T) {
	m := newMockOrders(t)
	item := models.OrderItem{OrderID: 12, ProductID: 7, Quantity: 2}
	m.On("AddOrderItem", mock.Anything, item).Return(0, errors.New("DB error"))
	req := httptest.NewRequest(http.MethodPost, "/orders/12/items", bytes.NewBufferString(`{"product_id":7,"quantity":2}`))
	req = withOrderParam(req, "id", "12")
	w := httptest.NewRecorder()

	New(slog.Default(), m).AddOrderItem(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetOrderByID_Success(t *testing.T) {
	m := newMockOrders(t)
	order := models.Order{ID: 12, UserID: 4, TotalPrice: 32.5, Items: []models.OrderItem{{ProductID: 7, Quantity: 2}}}
	m.On("GetOrderByID", mock.Anything, 12).Return(order, nil)
	req := withOrderParam(httptest.NewRequest(http.MethodGet, "/orders/12", nil), "id", "12")
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetOrderByID(w, req)

	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"quantity":2`)) {
		t.Fatalf("expected status 200 with order details, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetOrderByID_NotFound(t *testing.T) {
	m := newMockOrders(t)
	m.On("GetOrderByID", mock.Anything, 12).Return(models.Order{}, storage.ErrNotFound)
	req := withOrderParam(httptest.NewRequest(http.MethodGet, "/orders/12", nil), "id", "12")
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetOrderByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetOrderByID_BadID(t *testing.T) {
	m := newMockOrders(t)
	req := withOrderParam(httptest.NewRequest(http.MethodGet, "/orders/nope", nil), "id", "nope")
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetOrderByID(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetOrderByID_StorageError(t *testing.T) {
	m := newMockOrders(t)
	m.On("GetOrderByID", mock.Anything, 12).Return(models.Order{}, errors.New("DB error"))
	req := withOrderParam(httptest.NewRequest(http.MethodGet, "/orders/12", nil), "id", "12")
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetOrderByID(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetOrdersByUserEmail_Success(t *testing.T) {
	m := newMockOrders(t)
	orders := []models.Order{{ID: 12, UserID: 4, TotalPrice: 32.5}}
	m.On("GetOrdersByUserEmail", mock.Anything, "remy@example.com").Return(orders, nil)
	req := withOrderParam(httptest.NewRequest(http.MethodGet, "/users/remy@example.com/orders", nil), "email", "remy@example.com")
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetOrdersByUserEmail(w, req)

	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"id":12`)) {
		t.Fatalf("expected status 200 with orders, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetOrdersByUserEmail_StorageError(t *testing.T) {
	m := newMockOrders(t)
	m.On("GetOrdersByUserEmail", mock.Anything, "remy@example.com").Return(nil, errors.New("DB error"))
	req := withOrderParam(httptest.NewRequest(http.MethodGet, "/users/remy@example.com/orders", nil), "email", "remy@example.com")
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetOrdersByUserEmail(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCheckout_Success(t *testing.T) {
	m := newMockOrders(t)
	items := []models.OrderItem{{ProductID: 7, Quantity: 2}}
	m.On("PlaceOrder", mock.Anything, "remy@example.com", items).Return(23, nil)
	req := httptest.NewRequest(http.MethodPost, "/checkout", bytes.NewBufferString(`{"email":"remy@example.com","items":[{"product_id":7,"quantity":2}]}`))
	w := httptest.NewRecorder()

	New(slog.Default(), m).Checkout(w, req)

	if w.Code != http.StatusCreated || !bytes.Contains(w.Body.Bytes(), []byte(`"order_id":23`)) {
		t.Fatalf("expected status 201 with order ID, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCheckout_InvalidInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{"email":`},
		{name: "missing email", body: `{"items":[{"product_id":7,"quantity":1}]}`},
		{name: "invalid email", body: `{"email":"invalid","items":[{"product_id":7,"quantity":1}]}`},
		{name: "empty items", body: `{"email":"remy@example.com","items":[]}`},
		{name: "invalid item", body: `{"email":"remy@example.com","items":[{"product_id":7,"quantity":0}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockOrders(t)
			req := httptest.NewRequest(http.MethodPost, "/checkout", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			New(slog.Default(), m).Checkout(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestCheckout_InsufficientStock(t *testing.T) {
	m := newMockOrders(t)
	items := []models.OrderItem{{ProductID: 7, Quantity: 2}}
	m.On("PlaceOrder", mock.Anything, "remy@example.com", items).Return(0, storage.ErrInsufficientStock)
	req := httptest.NewRequest(http.MethodPost, "/checkout", bytes.NewBufferString(`{"email":"remy@example.com","items":[{"product_id":7,"quantity":2}]}`))
	w := httptest.NewRecorder()

	New(slog.Default(), m).Checkout(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCheckout_UserOrProductNotFound(t *testing.T) {
	m := newMockOrders(t)
	items := []models.OrderItem{{ProductID: 7, Quantity: 2}}
	m.On("PlaceOrder", mock.Anything, "remy@example.com", items).Return(0, storage.ErrNotFound)
	req := httptest.NewRequest(http.MethodPost, "/checkout", bytes.NewBufferString(`{"email":"remy@example.com","items":[{"product_id":7,"quantity":2}]}`))
	w := httptest.NewRecorder()

	New(slog.Default(), m).Checkout(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCheckout_StorageError(t *testing.T) {
	m := newMockOrders(t)
	items := []models.OrderItem{{ProductID: 7, Quantity: 2}}
	m.On("PlaceOrder", mock.Anything, "remy@example.com", items).Return(0, errors.New("DB error"))
	req := httptest.NewRequest(http.MethodPost, "/checkout", bytes.NewBufferString(`{"email":"remy@example.com","items":[{"product_id":7,"quantity":2}]}`))
	w := httptest.NewRecorder()

	New(slog.Default(), m).Checkout(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body: %s", w.Code, w.Body.String())
	}
}

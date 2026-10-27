package product

import (
	"bytes"
	"context"
	"errors"
	"go-pet-shop/internal/models"
	"go-pet-shop/internal/storage"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	mock "github.com/stretchr/testify/mock"
)

func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestGetAllProducts_Success(t *testing.T) {
	m := NewMockProducts(t)
	m.On("GetAllProducts", context.Background(), 20, 0).Return([]models.Product{
		{ID: 1, Name: "Dog Food"},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	w := httptest.NewRecorder()

	handler := New(slog.Default(), m)

	handler.GetAllProducts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}
func TestGetAllProducts_Error(t *testing.T) {
	m := NewMockProducts(t)
	m.On("GetAllProducts", context.Background(), 20, 0).Return(nil, errors.New("DB error"))

	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	w := httptest.NewRecorder()

	handler := New(slog.Default(), m)
	handler.GetAllProducts(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGetAllProducts_Pagination(t *testing.T) {
	m := NewMockProducts(t)
	m.On("GetAllProducts", mock.Anything, 5, 10).Return([]models.Product{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/products?limit=5&offset=10", nil)
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetAllProducts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetAllProducts_InvalidPagination(t *testing.T) {
	m := NewMockProducts(t)
	req := httptest.NewRequest(http.MethodGet, "/products?limit=1000", nil)
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetAllProducts(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetProductByID_Success(t *testing.T) {
	m := NewMockProducts(t)
	product := models.Product{ID: 7, Name: "Dog Food", Price: 12.5, Stock: 4}
	m.On("GetProductByID", mock.Anything, 7).Return(product, nil)
	req := httptest.NewRequest(http.MethodGet, "/products/7", nil)
	req = withURLParam(req, "id", "7")
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetProductByID(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"name":"Dog Food"`)) {
		t.Fatalf("expected product in response, got %s", w.Body.String())
	}
}

func TestGetProductByID_NotFound(t *testing.T) {
	m := NewMockProducts(t)
	m.On("GetProductByID", mock.Anything, 7).Return(models.Product{}, storage.ErrNotFound)
	req := httptest.NewRequest(http.MethodGet, "/products/7", nil)
	req = withURLParam(req, "id", "7")
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetProductByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetProductByID_BadID(t *testing.T) {
	m := NewMockProducts(t)
	req := httptest.NewRequest(http.MethodGet, "/products/nope", nil)
	req = withURLParam(req, "id", "nope")
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetProductByID(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetProductByID_StorageError(t *testing.T) {
	m := NewMockProducts(t)
	m.On("GetProductByID", mock.Anything, 7).Return(models.Product{}, errors.New("DB error"))
	req := httptest.NewRequest(http.MethodGet, "/products/7", nil)
	req = withURLParam(req, "id", "7")
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetProductByID(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateProduct_Success(t *testing.T) {
	m := NewMockProducts(t)
	m.On("CreateProduct", context.Background(), models.Product{Name: "Cat Food", Price: 10.5, Stock: 5}).Return(42, nil)

	body := bytes.NewBufferString(`{"name":"Cat Food","price":10.5,"stock":5}`)
	req := httptest.NewRequest(http.MethodPost, "/products", body)
	w := httptest.NewRecorder()

	handler := New(slog.Default(), m)
	handler.CreateProduct(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateProduct_BadRequest(t *testing.T) {
	m := NewMockProducts(t)

	body := bytes.NewBufferString(`"name":`)
	req := httptest.NewRequest(http.MethodPost, "/products", body)
	w := httptest.NewRecorder()

	handler := New(slog.Default(), m)
	handler.CreateProduct(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body %s", w.Code, w.Body.String())
	}
}

func TestCreateProduct_InvalidFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing name", body: `{"price":1,"stock":1}`},
		{name: "negative price", body: `{"name":"Food","price":-1,"stock":1}`},
		{name: "negative stock", body: `{"name":"Food","price":1,"stock":-1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMockProducts(t)
			req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			New(slog.Default(), m).CreateProduct(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestCreateProduct_BodyTooLarge(t *testing.T) {
	m := NewMockProducts(t)
	body := `{"name":"` + strings.Repeat("x", 1<<20) + `","price":1,"stock":1}`
	req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(body))
	w := httptest.NewRecorder()

	New(slog.Default(), m).CreateProduct(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateProduct_Fail(t *testing.T) {
	m := NewMockProducts(t)
	m.On("CreateProduct", context.Background(), models.Product{Name: "Cat Food", Price: 10.5, Stock: 5}).Return(0, errors.New("DB error"))

	body := bytes.NewBufferString(`{"name":"Cat Food","price":10.5,"stock":5}`)
	req := httptest.NewRequest(http.MethodPost, "/products", body)
	w := httptest.NewRecorder()

	handler := New(slog.Default(), m)
	handler.CreateProduct(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestUpdateProduct_Success(t *testing.T) {
	m := NewMockProducts(t)
	m.On("UpdateProduct", mock.Anything, models.Product{ID: 1, Name: "Cat Food", Price: 10.5, Stock: 5}).
		Return(nil)

	body := bytes.NewBufferString(`{"name":"Cat Food","price":10.5,"stock":5}`)
	req := httptest.NewRequest(http.MethodPut, "/products/1", body)
	req = withURLParam(req, "id", "1")
	w := httptest.NewRecorder()

	handler := New(slog.Default(), m)
	handler.UpdateProduct(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestUpdateProduct_BadRequest(t *testing.T) {
	m := NewMockProducts(t)

	body := bytes.NewBufferString(`"{"name":}`)
	req := httptest.NewRequest(http.MethodPut, "/products/1", body)
	req = withURLParam(req, "id", "1")
	w := httptest.NewRecorder()

	handler := New(slog.Default(), m)
	handler.UpdateProduct(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestUpdateProduct_Fail(t *testing.T) {
	m := NewMockProducts(t)
	m.On("UpdateProduct", mock.Anything, models.Product{ID: 1, Name: "Cat Food", Price: 10.5, Stock: 5}).
		Return(errors.New("DB error"))

	body := bytes.NewBufferString(`{"name":"Cat Food","price":10.5,"stock":5}`)
	req := httptest.NewRequest(http.MethodPut, "/products/1", body)
	req = withURLParam(req, "id", "1")
	w := httptest.NewRecorder()

	handler := New(slog.Default(), m)
	handler.UpdateProduct(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestUpdateProduct_NotFound(t *testing.T) {
	m := NewMockProducts(t)
	m.On("UpdateProduct", mock.Anything, models.Product{ID: 1, Name: "Cat Food", Price: 10.5, Stock: 5}).Return(storage.ErrNotFound)
	req := httptest.NewRequest(http.MethodPut, "/products/1", strings.NewReader(`{"name":"Cat Food","price":10.5,"stock":5}`))
	req = withURLParam(req, "id", "1")
	w := httptest.NewRecorder()

	New(slog.Default(), m).UpdateProduct(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestDeleteProduct_Success(t *testing.T) {
	m := NewMockProducts(t)
	m.On("DeleteProduct", mock.Anything, 1).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/products/1", nil)
	req = withURLParam(req, "id", "1")
	w := httptest.NewRecorder()

	handler := New(slog.Default(), m)
	handler.DeleteProduct(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestDeleteProduct_BadRequest(t *testing.T) {
	m := NewMockProducts(t)

	req := httptest.NewRequest(http.MethodDelete, "/products/", nil)
	w := httptest.NewRecorder()

	handler := New(slog.Default(), m)
	handler.DeleteProduct(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestDeleteProduct_Fail(t *testing.T) {
	m := NewMockProducts(t)
	m.On("DeleteProduct", mock.Anything, 1).Return(errors.New("DB error"))

	req := httptest.NewRequest(http.MethodDelete, "/products/1", nil)
	req = withURLParam(req, "id", "1")
	w := httptest.NewRecorder()

	handler := New(slog.Default(), m)
	handler.DeleteProduct(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestDeleteProduct_NotFound(t *testing.T) {
	m := NewMockProducts(t)
	m.On("DeleteProduct", mock.Anything, 1).Return(storage.ErrNotFound)
	req := withURLParam(httptest.NewRequest(http.MethodDelete, "/products/1", nil), "id", "1")
	w := httptest.NewRecorder()

	New(slog.Default(), m).DeleteProduct(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d, body: %s", w.Code, w.Body.String())
	}
}

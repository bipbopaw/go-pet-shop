package product

import (
	"context"
	"errors"
	"fmt"
	"go-pet-shop/internal/lib/httpx"
	"go-pet-shop/internal/models"
	"go-pet-shop/internal/storage"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

type Products interface {
	GetAllProducts(ctx context.Context, limit, offset int) ([]models.Product, error)
	GetProductByID(ctx context.Context, id int) (models.Product, error)
	CreateProduct(ctx context.Context, product models.Product) (int, error)
	DeleteProduct(ctx context.Context, id int) error
	UpdateProduct(ctx context.Context, product models.Product) error
}

const (
	defaultPageLimit = 20
	maxPageLimit     = 100
	maxJSONBodyBytes = 1 << 20
)

type Handler struct {
	log     *slog.Logger
	storage Products
}

func New(log *slog.Logger, storage Products) *Handler {
	return &Handler{
		log:     log,
		storage: storage,
	}
}
func (h *Handler) GetAllProducts(w http.ResponseWriter, r *http.Request) {
	const fn = "handlers.products.GetAllProducts"
	log := h.log.With(
		slog.String("fn", fn),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	limit, offset, err := httpx.ParsePagination(r.URL.Query(), defaultPageLimit, maxPageLimit)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "Bad request", err.Error())
		return
	}

	items, err := h.storage.GetAllProducts(r.Context(), limit, offset)

	if err != nil {
		log.Error("failed to get products", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error":   "Internal server error",
			"message": "Failed to retrieve products",
		})
		return
	}

	log.Info("Retrieved products successfully",
		slog.Int("count", len(items)),
	)

	render.JSON(w, r, items)
}

func (h *Handler) GetProductByID(w http.ResponseWriter, r *http.Request) {
	const fn = "handlers.products.GetProductByID"
	log := h.log.With(
		slog.String("fn", fn),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusBadRequest, "Bad request", "Product ID must be a positive number")
		return
	}

	item, err := h.storage.GetProductByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, "Not found", fmt.Sprintf("Product with ID %d does not exist", id))
			return
		}
		log.Error("failed to get product", slog.Any("error", err))
		writeError(w, r, http.StatusInternalServerError, "Internal server error", "Failed to retrieve product")
		return
	}

	render.JSON(w, r, item)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, title, message string) {
	render.Status(r, status)
	render.JSON(w, r, map[string]string{"error": title, "message": message})
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	const fn = "handlers.products.CreateProduct"

	log := h.log.With(
		slog.String("fn", fn),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	log.Info("Creating new product")

	var product models.Product
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	if err := render.DecodeJSON(r.Body, &product); err != nil {
		log.Error("failed to decode request body", slog.Any("error", err))
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "Payload too large", "Payload exceeds the 1 MiB limit")
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Invalid JSON payload",
		})
		return
	}

	if strings.TrimSpace(product.Name) == "" {
		log.Error("product name is empty")
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product name is required",
		})
		return
	}

	if product.Price < 0 {
		log.Error("product price is negative", slog.Float64("price", product.Price))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product price cannot be negative",
		})
		return
	}

	if product.Stock < 0 {
		log.Error("product stock is negative", slog.Int("stock", product.Stock))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product stock cannot be negative",
		})
		return
	}

	productID, err := h.storage.CreateProduct(r.Context(), product)
	if err != nil {
		log.Error("failed to create product", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error":   "Internal server error",
			"message": "Failed to create product",
		})
		return
	}

	log.Info("Product created successfully",
		slog.Int("id", productID),
		slog.String("name", product.Name),
	)

	product.ID = productID
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]interface{}{
		"status":  "Product created successfully",
		"id":      productID,
		"product": product,
	})
}

func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	const fn = "handlers.products.DeleteProduct"

	log := h.log.With(
		slog.String("fn", fn),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	log.Info("Deleting product")

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		log.Error("empty id")
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product ID is required",
		})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Error("invalid id format", slog.Any("error", err), slog.String("id", idStr))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product ID must be a number",
		})
		return
	}

	if err := h.storage.DeleteProduct(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			log.Warn("product not found for deletion", slog.Int("id", id))
			w.WriteHeader(http.StatusNotFound)
			render.JSON(w, r, map[string]interface{}{
				"error":   "Not found",
				"message": fmt.Sprintf("Product with ID %d does not exist", id),
				"id":      id,
			})
			return
		}

		log.Error("failed to delete product", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error":   "Internal server error",
			"message": "Failed to delete product",
		})
		return
	}

	log.Info("Deleted product successfully",
		slog.Int("id", id),
	)

	render.JSON(w, r, map[string]interface{}{
		"status":  "Product deleted successfully",
		"id":      id,
		"message": fmt.Sprintf("Product with ID %d has been deleted", id),
	})
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	const fn = "handlers.products.UpdateProduct"

	log := h.log.With(
		slog.String("fn", fn),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	log.Info("Updating product")

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		log.Error("empty id in URL")
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product ID is required",
		})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Error("invalid id format", slog.Any("error", err), slog.String("id", idStr))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product ID must be a number",
		})
		return
	}

	var product models.Product
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	if err := render.DecodeJSON(r.Body, &product); err != nil {
		log.Error("failed to decode request body", slog.Any("error", err))
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "Payload too large", "Payload exceeds the 1 MiB limit")
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Invalid JSON payload",
		})
		return
	}

	product.ID = id

	if strings.TrimSpace(product.Name) == "" {
		log.Error("product name is empty")
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product name is required",
		})
		return
	}

	if product.Price < 0 {
		log.Error("product price is negative", slog.Float64("price", product.Price))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product price cannot be negative",
		})
		return
	}

	if product.Stock < 0 {
		log.Error("product stock is negative", slog.Int("stock", product.Stock))
		w.WriteHeader(http.StatusBadRequest)
		render.JSON(w, r, map[string]string{
			"error":   "Bad request",
			"message": "Product stock cannot be negative",
		})
		return
	}

	if err := h.storage.UpdateProduct(r.Context(), product); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			log.Warn("product not found for update", slog.Int("id", id))
			w.WriteHeader(http.StatusNotFound)
			render.JSON(w, r, map[string]interface{}{
				"error":   "Not found",
				"message": fmt.Sprintf("Product with ID %d does not exist", id),
				"id":      id,
			})
			return
		}

		log.Error("failed to update product", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{
			"error":   "Internal server error",
			"message": "Failed to update product",
		})
		return
	}

	log.Info("Product updated successfully",
		slog.Int("id", id),
		slog.String("name", product.Name),
	)

	render.JSON(w, r, map[string]interface{}{
		"status":  "Product updated successfully",
		"id":      id,
		"product": product,
	})
}

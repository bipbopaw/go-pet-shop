package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"go-pet-shop/internal/models"
	"go-pet-shop/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newPostgresTestStorage(t *testing.T) (*Storage, context.Context) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Fatalf("ping test database: %v", err)
	}

	schemaBytes := make([]byte, 8)
	if _, err := rand.Read(schemaBytes); err != nil {
		admin.Close()
		t.Fatalf("generate test schema name: %v", err)
	}
	schema := "test_" + hex.EncodeToString(schemaBytes)
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		admin.Close()
		t.Fatalf("create test schema: %v", err)
	}

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		_, _ = admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
		t.Fatalf("connect to isolated test schema: %v", err)
	}

	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "0001_create_tables.up.sql"))
	if err != nil {
		pool.Close()
		_, _ = admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
		t.Fatalf("read schema migration: %v", err)
	}
	for _, statement := range strings.Split(string(migration), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := pool.Exec(ctx, statement); err != nil {
			pool.Close()
			_, _ = admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
			admin.Close()
			t.Fatalf("apply schema migration: %v", err)
		}
	}

	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	})
	return &Storage{db: pool}, ctx
}

func TestPlaceOrder_CommitAndPersist(t *testing.T) {
	s, ctx := newPostgresTestStorage(t)
	if err := s.CreateUser(ctx, models.User{Name: "Order User", Email: "order@example.com"}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	firstProductID, err := s.CreateProduct(ctx, models.Product{Name: "Food", Price: 12.5, Stock: 5})
	if err != nil {
		t.Fatalf("create first product: %v", err)
	}
	secondProductID, err := s.CreateProduct(ctx, models.Product{Name: "Toy", Price: 7, Stock: 4})
	if err != nil {
		t.Fatalf("create second product: %v", err)
	}

	orderID, err := s.PlaceOrder(ctx, "order@example.com", []models.OrderItem{
		{ProductID: firstProductID, Quantity: 2},
		{ProductID: secondProductID, Quantity: 1},
	})
	if err != nil {
		t.Fatalf("place order: %v", err)
	}
	order, err := s.GetOrderByID(ctx, orderID)
	if err != nil {
		t.Fatalf("get placed order: %v", err)
	}
	if order.TotalPrice != 32 || len(order.Items) != 2 {
		t.Fatalf("expected total 32 and two items, got total=%v items=%d", order.TotalPrice, len(order.Items))
	}
	firstProduct, err := s.GetProductByID(ctx, firstProductID)
	if err != nil || firstProduct.Stock != 3 {
		t.Fatalf("expected first stock to be 3, got product=%+v err=%v", firstProduct, err)
	}
	secondProduct, err := s.GetProductByID(ctx, secondProductID)
	if err != nil || secondProduct.Stock != 3 {
		t.Fatalf("expected second stock to be 3, got product=%+v err=%v", secondProduct, err)
	}
	var transactionCount int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM transactions WHERE order_id = $1 AND amount = 32 AND status = 'completed'`, orderID).Scan(&transactionCount); err != nil {
		t.Fatalf("query payment transaction: %v", err)
	}
	if transactionCount != 1 {
		t.Fatalf("expected one completed payment transaction, got %d", transactionCount)
	}
}

func TestPlaceOrder_RollsBackWhenStockIsInsufficient(t *testing.T) {
	s, ctx := newPostgresTestStorage(t)
	if err := s.CreateUser(ctx, models.User{Name: "Order User", Email: "order@example.com"}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	availableProductID, err := s.CreateProduct(ctx, models.Product{Name: "Available", Price: 10, Stock: 5})
	if err != nil {
		t.Fatalf("create available product: %v", err)
	}
	shortProductID, err := s.CreateProduct(ctx, models.Product{Name: "Almost sold out", Price: 20, Stock: 1})
	if err != nil {
		t.Fatalf("create short product: %v", err)
	}

	_, err = s.PlaceOrder(ctx, "order@example.com", []models.OrderItem{
		{ProductID: availableProductID, Quantity: 2},
		{ProductID: shortProductID, Quantity: 2},
	})
	if !errors.Is(err, storage.ErrInsufficientStock) {
		t.Fatalf("expected insufficient stock error, got %v", err)
	}
	availableProduct, err := s.GetProductByID(ctx, availableProductID)
	if err != nil || availableProduct.Stock != 5 {
		t.Fatalf("expected stock update to roll back to 5, got product=%+v err=%v", availableProduct, err)
	}
	var orderCount, transactionCount int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&orderCount); err != nil {
		t.Fatalf("query orders: %v", err)
	}
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM transactions`).Scan(&transactionCount); err != nil {
		t.Fatalf("query transactions: %v", err)
	}
	if orderCount != 0 || transactionCount != 0 {
		t.Fatalf("expected no persisted order or transaction, got orders=%d transactions=%d", orderCount, transactionCount)
	}
}

func TestPlaceOrder_RejectsUnknownUserAndProduct(t *testing.T) {
	s, ctx := newPostgresTestStorage(t)
	if err := s.CreateUser(ctx, models.User{Name: "Order User", Email: "order@example.com"}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	productID, err := s.CreateProduct(ctx, models.Product{Name: "Food", Price: 12.5, Stock: 5})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if _, err := s.PlaceOrder(ctx, "missing@example.com", []models.OrderItem{{ProductID: productID, Quantity: 1}}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected not found for unknown user, got %v", err)
	}
	if _, err := s.PlaceOrder(ctx, "order@example.com", []models.OrderItem{{ProductID: productID + 99999, Quantity: 1}}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected not found for unknown product, got %v", err)
	}
	product, err := s.GetProductByID(ctx, productID)
	if err != nil || product.Stock != 5 {
		t.Fatalf("expected stock to remain 5, got product=%+v err=%v", product, err)
	}
}

func TestUserAndProductStorage(t *testing.T) {
	s, ctx := newPostgresTestStorage(t)
	user := models.User{Name: "Storage User", Email: "storage@example.com"}
	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := s.CreateUser(ctx, user); !errors.Is(err, storage.ErrAlreadyExists) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
	gotUser, err := s.GetUserByEmail(ctx, user.Email)
	if err != nil || gotUser.Email != user.Email {
		t.Fatalf("get user returned user=%+v err=%v", gotUser, err)
	}
	users, err := s.GetAllUsers(ctx, 10, 0)
	if err != nil || len(users) != 1 {
		t.Fatalf("expected one user, got users=%v err=%v", users, err)
	}
	if _, err := s.GetUserByEmail(ctx, "missing@example.com"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected user not found, got %v", err)
	}

	productID, err := s.CreateProduct(ctx, models.Product{Name: "Food", Price: 12.5, Stock: 5})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if err := s.UpdateProduct(ctx, models.Product{ID: productID, Name: "Premium Food", Price: 15, Stock: 4}); err != nil {
		t.Fatalf("update product: %v", err)
	}
	products, err := s.GetAllProducts(ctx, 10, 0)
	if err != nil || len(products) != 1 || products[0].Name != "Premium Food" {
		t.Fatalf("expected updated product, got products=%+v err=%v", products, err)
	}
	if err := s.UpdateProduct(ctx, models.Product{ID: productID + 99999, Name: "Missing", Price: 1, Stock: 1}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected product not found on update, got %v", err)
	}
	if err := s.DeleteProduct(ctx, productID); err != nil {
		t.Fatalf("delete product: %v", err)
	}
	if err := s.DeleteProduct(ctx, productID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected product not found on delete, got %v", err)
	}
	if _, err := s.GetProductByID(ctx, productID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected product not found on lookup, got %v", err)
	}
}

func TestOrderReadStorage(t *testing.T) {
	s, ctx := newPostgresTestStorage(t)
	if err := s.CreateUser(ctx, models.User{Name: "Order User", Email: "order@example.com"}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	productID, err := s.CreateProduct(ctx, models.Product{Name: "Food", Price: 12.5, Stock: 5})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	user, err := s.GetUserByEmail(ctx, "order@example.com")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	orderID, err := s.CreateOrder(ctx, models.Order{UserID: user.ID, TotalPrice: 25})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, err := s.AddOrderItem(ctx, models.OrderItem{OrderID: orderID, ProductID: productID, Quantity: 2}); err != nil {
		t.Fatalf("add order item: %v", err)
	}
	orders, err := s.GetOrdersByUserEmail(ctx, "order@example.com")
	if err != nil || len(orders) != 1 || orders[0].ID != orderID {
		t.Fatalf("expected one order, got orders=%+v err=%v", orders, err)
	}
	items, err := s.GetOrderItemsByOrderID(ctx, orderID)
	if err != nil || len(items) != 1 || items[0].ProductName != "Food" || items[0].UnitPrice != 12.5 {
		t.Fatalf("expected item details joined with product, got items=%+v err=%v", items, err)
	}
}

package postgres

import (
	"context"
	"errors"
	"fmt"
	"go-pet-shop/internal/models"
	"go-pet-shop/internal/storage"

	"github.com/jackc/pgx/v5"
)

func (s *Storage) PlaceOrder(ctx context.Context, userEmail string, items []models.OrderItem) (int, error) {
	const fn = "storage.postgres.order.PlaceOrder"
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("%s: begin transaction: %w", fn, err)
	}
	defer tx.Rollback(ctx)

	var userID int
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, userEmail).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("%s: user: %w", fn, storage.ErrNotFound)
	}
	if err != nil {
		return 0, fmt.Errorf("%s: find user: %w", fn, err)
	}

	type pricedItem struct {
		item  models.OrderItem
		price float64
	}
	pricedItems := make([]pricedItem, 0, len(items))
	var totalPrice float64
	for _, item := range items {
		var price float64
		err := tx.QueryRow(ctx,
			`UPDATE products SET stock = stock - $1 WHERE id = $2 AND stock >= $1 RETURNING price`,
			item.Quantity, item.ProductID,
		).Scan(&price)
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if checkErr := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM products WHERE id = $1)`, item.ProductID).Scan(&exists); checkErr != nil {
				return 0, fmt.Errorf("%s: check product: %w", fn, checkErr)
			}
			if !exists {
				return 0, fmt.Errorf("%s: product id=%d: %w", fn, item.ProductID, storage.ErrNotFound)
			}
			return 0, fmt.Errorf("%s: product id=%d: %w", fn, item.ProductID, storage.ErrInsufficientStock)
		}
		if err != nil {
			return 0, fmt.Errorf("%s: update stock: %w", fn, err)
		}
		pricedItems = append(pricedItems, pricedItem{item: item, price: price})
		totalPrice += price * float64(item.Quantity)
	}

	var orderID int
	err = tx.QueryRow(ctx,
		`INSERT INTO orders (user_id, total_price) VALUES ($1, $2) RETURNING id`, userID, totalPrice,
	).Scan(&orderID)
	if err != nil {
		return 0, fmt.Errorf("%s: create order: %w", fn, err)
	}
	for _, priced := range pricedItems {
		_, err = tx.Exec(ctx,
			`INSERT INTO order_items (order_id, product_id, quantity) VALUES ($1, $2, $3)`,
			orderID, priced.item.ProductID, priced.item.Quantity,
		)
		if err != nil {
			return 0, fmt.Errorf("%s: create order item: %w", fn, err)
		}
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO transactions (order_id, amount, status) VALUES ($1, $2, $3)`, orderID, totalPrice, "completed",
	)
	if err != nil {
		return 0, fmt.Errorf("%s: create payment transaction: %w", fn, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("%s: commit transaction: %w", fn, err)
	}
	return orderID, nil
}

func (s *Storage) CreateOrder(ctx context.Context, order models.Order) (int, error) {
	const fn = "storage.postgres.order.CreateOrder"
	var id int
	err := s.db.QueryRow(ctx,
		`INSERT INTO orders (user_id, total_price) VALUES ($1, $2) RETURNING id`,
		order.UserID, order.TotalPrice,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", fn, err)
	}
	return id, nil
}

func (s *Storage) AddOrderItem(ctx context.Context, item models.OrderItem) (int, error) {
	const fn = "storage.postgres.order.AddOrderItem"
	var id int
	err := s.db.QueryRow(ctx,
		`INSERT INTO order_items (order_id, product_id, quantity) VALUES ($1, $2, $3) RETURNING id`,
		item.OrderID, item.ProductID, item.Quantity,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", fn, err)
	}
	return id, nil
}

func (s *Storage) GetOrderByID(ctx context.Context, id int) (models.Order, error) {
	const fn = "storage.postgres.order.GetOrderByID"
	var order models.Order
	err := s.db.QueryRow(ctx,
		`SELECT o.id, o.user_id, o.total_price, o.created_at
		 FROM orders o WHERE o.id = $1`, id,
	).Scan(&order.ID, &order.UserID, &order.TotalPrice, &order.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Order{}, fmt.Errorf("%s: %w: id=%d", fn, storage.ErrNotFound, id)
	}
	if err != nil {
		return models.Order{}, fmt.Errorf("%s: %w", fn, err)
	}

	rows, err := s.db.Query(ctx,
		`SELECT oi.id, oi.order_id, oi.product_id, p.name, p.price, oi.quantity
		 FROM order_items oi
		 JOIN products p ON p.id = oi.product_id
		 WHERE oi.order_id = $1 ORDER BY oi.id`, id,
	)
	if err != nil {
		return models.Order{}, fmt.Errorf("%s: load items: %w", fn, err)
	}
	defer rows.Close()
	order.Items = make([]models.OrderItem, 0)
	for rows.Next() {
		var item models.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.ProductName, &item.UnitPrice, &item.Quantity); err != nil {
			return models.Order{}, fmt.Errorf("%s: scan item: %w", fn, err)
		}
		order.Items = append(order.Items, item)
	}
	if err := rows.Err(); err != nil {
		return models.Order{}, fmt.Errorf("%s: iterate items: %w", fn, err)
	}
	return order, nil
}

func (s *Storage) GetOrdersByUserEmail(ctx context.Context, email string) ([]models.Order, error) {
	const fn = "storage.postgres.order.GetOrdersByUserEmail"
	rows, err := s.db.Query(ctx,
		`SELECT o.id, o.user_id, o.total_price, o.created_at
		 FROM orders o JOIN users u ON u.id = o.user_id
		 WHERE u.email = $1 ORDER BY o.created_at DESC, o.id DESC`, email,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}
	defer rows.Close()
	orders := make([]models.Order, 0)
	for rows.Next() {
		var order models.Order
		if err := rows.Scan(&order.ID, &order.UserID, &order.TotalPrice, &order.CreatedAt); err != nil {
			return nil, fmt.Errorf("%s: %w", fn, err)
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}
	return orders, nil
}

func (s *Storage) GetOrderItemsByOrderID(ctx context.Context, orderID int) ([]models.OrderItem, error) {
	const fn = "storage.postgres.order.GetOrderItemsByOrderID"
	rows, err := s.db.Query(ctx,
		`SELECT oi.id, oi.order_id, oi.product_id, p.name, p.price, oi.quantity
		 FROM order_items oi JOIN products p ON p.id = oi.product_id
		 WHERE oi.order_id = $1 ORDER BY oi.id`, orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}
	defer rows.Close()
	items := make([]models.OrderItem, 0)
	for rows.Next() {
		var item models.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.ProductName, &item.UnitPrice, &item.Quantity); err != nil {
			return nil, fmt.Errorf("%s: %w", fn, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}
	return items, nil
}

package postgres

import (
	"context"
	"errors"
	"fmt"
	"go-pet-shop/internal/models"
	"go-pet-shop/internal/storage"

	"github.com/jackc/pgx/v5"
)

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

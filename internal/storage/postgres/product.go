package postgres

import (
	"context"
	"errors"
	"fmt"
	"go-pet-shop/internal/models"
	"go-pet-shop/internal/storage"

	"github.com/jackc/pgx/v5"
)

func (s *Storage) GetPopularProducts(ctx context.Context) ([]models.PopularProduct, error) {
	const fn = "storage.postgres.product.GetPopularProducts"
	rows, err := s.db.Query(ctx,
		`SELECT p.id, p.name, p.price, p.stock, SUM(oi.quantity)::int AS total_sold
		 FROM products p
		 JOIN order_items oi ON oi.product_id = p.id
		 GROUP BY p.id, p.name, p.price, p.stock
		 ORDER BY total_sold DESC, p.id`,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}
	defer rows.Close()
	products := make([]models.PopularProduct, 0)
	for rows.Next() {
		var product models.PopularProduct
		if err := rows.Scan(&product.ID, &product.Name, &product.Price, &product.Stock, &product.TotalSold); err != nil {
			return nil, fmt.Errorf("%s: scan row: %w", fn, err)
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: iterate rows: %w", fn, err)
	}
	return products, nil
}

func (s *Storage) GetAllProducts(ctx context.Context, limit, offset int) ([]models.Product, error) {
	const fn = "storage.postgres.product.GetAllProducts"

	rows, err := s.db.Query(ctx,
		`SELECT id, name, price, stock FROM products ORDER BY id LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock); err != nil {
			return nil, fmt.Errorf("%s: %w", fn, err)
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}

	return products, nil
}

func (s *Storage) GetProductByID(ctx context.Context, id int) (models.Product, error) {
	const fn = "storage.postgres.product.GetProductByID"

	var p models.Product
	err := s.db.QueryRow(ctx,
		`SELECT id, name, price, stock FROM products WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Price, &p.Stock)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Product{}, fmt.Errorf("%s: %w: id=%d", fn, storage.ErrNotFound, id)
	}
	if err != nil {
		return models.Product{}, fmt.Errorf("%s: %w", fn, err)
	}
	return p, nil
}

func (s *Storage) CreateProduct(ctx context.Context, p models.Product) (int, error) {
	const fn = "storage.postgres.product.CreateProduct"

	var id int
	err := s.db.QueryRow(ctx,
		`INSERT INTO products (name, price, stock) VALUES ($1, $2, $3) RETURNING id`,
		p.Name, p.Price, p.Stock).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("%s: %w", fn, err)
	}

	return id, nil
}

func (s *Storage) DeleteProduct(ctx context.Context, id int) error {
	const fn = "storage.postgres.product.DeleteProduct"

	cmd, err := s.db.Exec(ctx,
		`DELETE FROM products WHERE id = $1`,
		id)
	if err != nil {
		return fmt.Errorf("%s: %w", fn, err)
	}

	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w: id=%d", fn, storage.ErrNotFound, id)
	}

	return nil
}

func (s *Storage) UpdateProduct(ctx context.Context, p models.Product) error {
	const fn = "storage.postgres.product.UpdateProduct"

	cmd, err := s.db.Exec(ctx,
		`UPDATE products SET name = $1, price = $2, stock = $3 WHERE id = $4`,
		p.Name, p.Price, p.Stock, p.ID)
	if err != nil {
		return fmt.Errorf("%s: %w", fn, err)
	}

	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w: id=%d", fn, storage.ErrNotFound, p.ID)
	}

	return nil
}

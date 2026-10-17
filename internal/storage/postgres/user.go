package postgres

import (
	"context"
	"errors"
	"fmt"
	"go-pet-shop/internal/models"
	"go-pet-shop/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Storage) CreateUser(ctx context.Context, user models.User) error {
	const fn = "storage.postgres.user.CreateUser"
	_, err := s.db.Exec(ctx,
		`INSERT INTO users (name, email) VALUES ($1, $2)`, user.Name, user.Email,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%s: %w: email=%s", fn, storage.ErrAlreadyExists, user.Email)
		}
		return fmt.Errorf("%s: %w", fn, err)
	}
	return nil
}

func (s *Storage) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	const fn = "storage.postgres.user.GetUserByEmail"
	var user models.User
	err := s.db.QueryRow(ctx,
		`SELECT id, name, email FROM users WHERE email = $1`, email,
	).Scan(&user.ID, &user.Name, &user.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, fmt.Errorf("%s: %w: email=%s", fn, storage.ErrNotFound, email)
	}
	if err != nil {
		return models.User{}, fmt.Errorf("%s: %w", fn, err)
	}
	return user, nil
}

func (s *Storage) GetAllUsers(ctx context.Context, limit, offset int) ([]models.User, error) {
	const fn = "storage.postgres.user.GetAllUsers"
	rows, err := s.db.Query(ctx,
		`SELECT id, name, email FROM users ORDER BY id LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}
	defer rows.Close()

	users := make([]models.User, 0)
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
			return nil, fmt.Errorf("%s: %w", fn, err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}
	return users, nil
}

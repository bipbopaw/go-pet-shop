package models

import "time"

type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Order struct {
	ID         int         `json:"id"`
	UserID     int         `json:"user_id"`
	TotalPrice float64     `json:"total_price"`
	CreatedAt  time.Time   `json:"created_at"`
	Items      []OrderItem `json:"items,omitempty"`
}

type OrderItem struct {
	ID          int     `json:"id"`
	OrderID     int     `json:"order_id"`
	ProductID   int     `json:"product_id"`
	ProductName string  `json:"product_name,omitempty"`
	UnitPrice   float64 `json:"unit_price,omitempty"`
	Quantity    int     `json:"quantity"`
}

// OrderDetail is one order line together with its order and payment details.
type OrderDetail struct {
	OrderID              int       `json:"order_id"`
	UserID               int       `json:"user_id"`
	TotalPrice           float64   `json:"total_price"`
	CreatedAt            time.Time `json:"created_at"`
	ItemID               int       `json:"item_id"`
	ProductID            int       `json:"product_id"`
	ProductName          string    `json:"product_name"`
	UnitPrice            float64   `json:"unit_price"`
	Quantity             int       `json:"quantity"`
	TransactionID        int       `json:"transaction_id"`
	TransactionAmount    float64   `json:"transaction_amount"`
	TransactionStatus    string    `json:"transaction_status"`
	TransactionCreatedAt time.Time `json:"transaction_created_at"`
}

type PopularProduct struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Stock     int     `json:"stock"`
	TotalSold int     `json:"total_sold"`
}

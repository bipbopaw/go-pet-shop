package user

import (
	"context"
	"errors"
	"go-pet-shop/internal/lib/httpx"
	"go-pet-shop/internal/models"
	"go-pet-shop/internal/storage"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

type Users interface {
	CreateUser(ctx context.Context, user models.User) error
	GetUserByEmail(ctx context.Context, email string) (models.User, error)
	GetAllUsers(ctx context.Context, limit, offset int) ([]models.User, error)
}

const (
	defaultPageLimit = 20
	maxPageLimit     = 100
	maxJSONBodyBytes = 1 << 20
)

type Handler struct {
	log     *slog.Logger
	storage Users
}

func New(log *slog.Logger, storage Users) *Handler {
	return &Handler{log: log, storage: storage}
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	log := h.requestLogger(r, "handlers.users.CreateUser")
	var user models.User
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	if err := render.DecodeJSON(r.Body, &user); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "Payload exceeds the 1 MiB limit")
			return
		}
		writeError(w, r, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	user.Name = strings.TrimSpace(user.Name)
	user.Email = strings.TrimSpace(user.Email)
	if user.Name == "" {
		writeError(w, r, http.StatusBadRequest, "User name is required")
		return
	}
	address, err := mail.ParseAddress(user.Email)
	if err != nil || address.Address != user.Email {
		writeError(w, r, http.StatusBadRequest, "A valid email is required")
		return
	}
	if err := h.storage.CreateUser(r.Context(), user); err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			writeError(w, r, http.StatusConflict, "A user with this email already exists")
			return
		}
		log.Error("failed to create user", slog.Any("error", err))
		writeError(w, r, http.StatusInternalServerError, "Failed to create user")
		return
	}
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, map[string]string{
		"status": "User created successfully",
		"name":   user.Name,
		"email":  user.Email,
	})
}

func (h *Handler) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	log := h.requestLogger(r, "handlers.users.GetAllUsers")
	limit, offset, err := httpx.ParsePagination(r.URL.Query(), defaultPageLimit, maxPageLimit)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	users, err := h.storage.GetAllUsers(r.Context(), limit, offset)
	if err != nil {
		log.Error("failed to get users", slog.Any("error", err))
		writeError(w, r, http.StatusInternalServerError, "Failed to retrieve users")
		return
	}
	render.JSON(w, r, users)
}

func (h *Handler) GetUserByEmail(w http.ResponseWriter, r *http.Request) {
	log := h.requestLogger(r, "handlers.users.GetUserByEmail")
	email := strings.TrimSpace(chi.URLParam(r, "email"))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		writeError(w, r, http.StatusBadRequest, "A valid email is required")
		return
	}
	user, err := h.storage.GetUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, "User not found")
			return
		}
		log.Error("failed to get user", slog.Any("error", err))
		writeError(w, r, http.StatusInternalServerError, "Failed to retrieve user")
		return
	}
	render.JSON(w, r, user)
}

func (h *Handler) requestLogger(r *http.Request, fn string) *slog.Logger {
	return h.log.With(
		slog.String("fn", fn),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, message string) {
	render.Status(r, status)
	render.JSON(w, r, map[string]string{"error": http.StatusText(status), "message": message})
}

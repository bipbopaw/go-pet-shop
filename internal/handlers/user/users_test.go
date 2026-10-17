package user

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
	"github.com/stretchr/testify/mock"
)

type mockUsers struct{ mock.Mock }

func (m *mockUsers) CreateUser(ctx context.Context, user models.User) error {
	return m.Called(ctx, user).Error(0)
}

func (m *mockUsers) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	args := m.Called(ctx, email)
	var user models.User
	if args.Get(0) != nil {
		user = args.Get(0).(models.User)
	}
	return user, args.Error(1)
}

func (m *mockUsers) GetAllUsers(ctx context.Context, limit, offset int) ([]models.User, error) {
	args := m.Called(ctx, limit, offset)
	var users []models.User
	if args.Get(0) != nil {
		users = args.Get(0).([]models.User)
	}
	return users, args.Error(1)
}

func newMockUsers(t *testing.T) *mockUsers {
	t.Helper()
	m := &mockUsers{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}

func withEmailParam(r *http.Request, email string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("email", email)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestCreateUser_Success(t *testing.T) {
	m := newMockUsers(t)
	user := models.User{Name: "Remy Fox", Email: "remy@example.com"}
	m.On("CreateUser", mock.Anything, user).Return(nil)
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(`{"name":" Remy Fox ","email":" remy@example.com "}`))
	w := httptest.NewRecorder()

	New(slog.Default(), m).CreateUser(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"email":"remy@example.com"`)) {
		t.Fatalf("expected created user in response, got %s", w.Body.String())
	}
}

func TestCreateUser_InvalidInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{"name":`},
		{name: "missing name", body: `{"email":"remy@example.com"}`},
		{name: "invalid email", body: `{"name":"Remy","email":"bad"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockUsers(t)
			req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			New(slog.Default(), m).CreateUser(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestCreateUser_AlreadyExists(t *testing.T) {
	m := newMockUsers(t)
	user := models.User{Name: "Remy", Email: "remy@example.com"}
	m.On("CreateUser", mock.Anything, user).Return(storage.ErrAlreadyExists)
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(`{"name":"Remy","email":"remy@example.com"}`))
	w := httptest.NewRecorder()

	New(slog.Default(), m).CreateUser(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateUser_StorageError(t *testing.T) {
	m := newMockUsers(t)
	user := models.User{Name: "Remy", Email: "remy@example.com"}
	m.On("CreateUser", mock.Anything, user).Return(errors.New("DB error"))
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(`{"name":"Remy","email":"remy@example.com"}`))
	w := httptest.NewRecorder()

	New(slog.Default(), m).CreateUser(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateUser_BodyTooLarge(t *testing.T) {
	m := newMockUsers(t)
	body := `{"name":"` + strings.Repeat("x", 1<<20) + `","email":"remy@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
	w := httptest.NewRecorder()

	New(slog.Default(), m).CreateUser(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetAllUsers_Success(t *testing.T) {
	m := newMockUsers(t)
	m.On("GetAllUsers", mock.Anything, 20, 0).Return([]models.User{{ID: 1, Name: "Remy", Email: "remy@example.com"}}, nil)
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetAllUsers(w, req)

	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"id":1`)) {
		t.Fatalf("expected status 200 with user, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetAllUsers_Pagination(t *testing.T) {
	m := newMockUsers(t)
	m.On("GetAllUsers", mock.Anything, 5, 10).Return([]models.User{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/users?limit=5&offset=10", nil)
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetAllUsers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetAllUsers_InvalidPagination(t *testing.T) {
	m := newMockUsers(t)
	req := httptest.NewRequest(http.MethodGet, "/users?offset=-1", nil)
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetAllUsers(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetAllUsers_StorageError(t *testing.T) {
	m := newMockUsers(t)
	m.On("GetAllUsers", mock.Anything, 20, 0).Return(nil, errors.New("DB error"))
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetAllUsers(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetUserByEmail_Success(t *testing.T) {
	m := newMockUsers(t)
	want := models.User{ID: 3, Name: "Remy", Email: "remy@example.com"}
	m.On("GetUserByEmail", mock.Anything, want.Email).Return(want, nil)
	req := httptest.NewRequest(http.MethodGet, "/users/remy@example.com", nil)
	req = withEmailParam(req, want.Email)
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetUserByEmail(w, req)

	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"id":3`)) {
		t.Fatalf("expected status 200 with user, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	m := newMockUsers(t)
	m.On("GetUserByEmail", mock.Anything, "missing@example.com").Return(models.User{}, storage.ErrNotFound)
	req := httptest.NewRequest(http.MethodGet, "/users/missing@example.com", nil)
	req = withEmailParam(req, "missing@example.com")
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetUserByEmail(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetUserByEmail_InvalidEmail(t *testing.T) {
	m := newMockUsers(t)
	req := httptest.NewRequest(http.MethodGet, "/users/invalid", nil)
	req = withEmailParam(req, "invalid")
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetUserByEmail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestGetUserByEmail_StorageError(t *testing.T) {
	m := newMockUsers(t)
	m.On("GetUserByEmail", mock.Anything, "remy@example.com").Return(models.User{}, errors.New("DB error"))
	req := httptest.NewRequest(http.MethodGet, "/users/remy@example.com", nil)
	req = withEmailParam(req, "remy@example.com")
	w := httptest.NewRecorder()

	New(slog.Default(), m).GetUserByEmail(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body: %s", w.Code, w.Body.String())
	}
}

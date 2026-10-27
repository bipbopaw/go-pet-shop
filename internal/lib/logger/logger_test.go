package logger

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetupLogger(t *testing.T) {
	for _, env := range []string{"local", "dev", "prod", "unknown"} {
		t.Run(env, func(t *testing.T) {
			if logger := SetupLogger(env); logger == nil {
				t.Fatal("expected logger")
			}
		})
	}
}

func TestCustomLoggerRecordsResponse(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	handler := CustomLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	req := httptest.NewRequest(http.MethodPost, "/resource", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated || w.Body.String() != "created" {
		t.Fatalf("unexpected response: status=%d body=%q", w.Code, w.Body.String())
	}
	for _, field := range []string{"HTTP response", "method=POST", "status=201", "bytes=7"} {
		if !strings.Contains(output.String(), field) {
			t.Errorf("expected log to contain %q, got %s", field, output.String())
		}
	}
}

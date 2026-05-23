package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunHealthcheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{
			name:   "healthy status",
			status: http.StatusOK,
		},
		{
			name:   "redirect status",
			status: http.StatusTemporaryRedirect,
		},
		{
			name:    "bad request status",
			status:  http.StatusBadRequest,
			wantErr: true,
		},
		{
			name:    "not found status",
			status:  http.StatusNotFound,
			wantErr: true,
		},
		{
			name:    "unhealthy status",
			status:  http.StatusServiceUnavailable,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			t.Cleanup(server.Close)

			err := runHealthcheck(context.Background(), server.URL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("runHealthcheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunHealthcheckRejectsEmptyURL(t *testing.T) {
	t.Parallel()

	if err := runHealthcheck(context.Background(), ""); err == nil {
		t.Fatal("runHealthcheck() error = nil, want error")
	}
}

func TestRunHealthcheckUnreachableServer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close()

	if err := runHealthcheck(context.Background(), url); err == nil {
		t.Fatal("runHealthcheck() error = nil, want error for unreachable server")
	}
}

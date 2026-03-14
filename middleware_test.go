package httpkit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_Generated(t *testing.T) {
	var capturedID string

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedID == "" {
		t.Error("expected request ID in context, got empty string")
	}
	if rec.Header().Get("X-Request-ID") != capturedID {
		t.Errorf("expected X-Request-ID header %q, got %q", capturedID, rec.Header().Get("X-Request-ID"))
	}
}

func TestRequestID_PropagatesExistingID(t *testing.T) {
	const existingID = "my-upstream-request-id"
	var capturedID string

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", existingID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedID != existingID {
		t.Errorf("expected propagated ID %q, got %q", existingID, capturedID)
	}
	if rec.Header().Get("X-Request-ID") != existingID {
		t.Errorf("expected X-Request-ID header %q, got %q", existingID, rec.Header().Get("X-Request-ID"))
	}
}

func TestRequestID_UniquePerRequest(t *testing.T) {
	ids := make(map[string]struct{})

	handler := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ids[RequestIDFromContext(r.Context())] = struct{}{}
	}))

	for range 10 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	if len(ids) != 10 {
		t.Errorf("expected 10 unique request IDs, got %d", len(ids))
	}
}

func TestRequestIDFromContext_Empty(t *testing.T) {
	id := RequestIDFromContext(context.Background())
	if id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}

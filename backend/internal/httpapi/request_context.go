//go:build postgres

package httpapi

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
)

type requestIDContextKey struct{}

type requestIDWriter struct {
	http.ResponseWriter
	requestID string
	ctx       context.Context
}

func requestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		w.Header().Set("X-Request-ID", requestID)
		r = r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, requestID))
		next.ServeHTTP(&requestIDWriter{ResponseWriter: w, requestID: requestID, ctx: r.Context()}, r)
	})
}

func newRequestID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "request-id-unavailable"
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}

func requestIDFromWriter(w http.ResponseWriter) string {
	if value, ok := w.(*requestIDWriter); ok {
		return value.requestID
	}
	return ""
}

func contextFromWriter(w http.ResponseWriter) context.Context {
	if value, ok := w.(*requestIDWriter); ok && value.ctx != nil {
		return value.ctx
	}
	return context.Background()
}

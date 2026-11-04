package mux

import (
	"net/http"
	"testing"
)

func TestHandle_invalidPatternPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected a panic for an invalid route pattern")
		}
	}()
	DefaultEngine().Handle("/users/{id:[}", http.MethodGet, http.NotFoundHandler())
}

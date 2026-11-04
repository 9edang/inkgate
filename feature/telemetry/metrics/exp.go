package metrics

import (
	"net/http"

	"github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/exp"
)

// NewExpHandler creates an http.Handler that exposes all the collected metrics as JSON.
// Both router variants serve it on the stats endpoint.
func NewExpHandler(parent *metrics.Registry) http.Handler {
	return exp.ExpHandler(*parent)
}

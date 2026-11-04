package entry

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	corerouter "github.com/9edang/inkgate/engine/router"
)

// A wrapper added after withFlowContext (like CORS, OTel or a server plugin)
// must already see the flow context of the request.
func TestFlowContextIsOutermost(t *testing.T) {
	var served http.Handler
	server := func(_ context.Context, _ config.ServiceConfig, h http.Handler) error {
		served = h
		return nil
	}

	var seenByWrapper, seenByRouter *context.Flow
	later := func(next corerouter.RunServerFunc) corerouter.RunServerFunc {
		return func(ctx context.Context, cfg config.ServiceConfig, h http.Handler) error {
			return next(ctx, cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seenByWrapper = context.From(r.Context())
				h.ServeHTTP(w, r)
			}))
		}
	}
	router := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seenByRouter = context.From(r.Context())
	})

	run := later(withFlowContext(server, logging.NoOp))
	if err := run(context.Background(), config.ServiceConfig{}, router); err != nil {
		t.Fatal(err)
	}
	served.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

	if seenByWrapper == nil {
		t.Fatal("a wrapper added after withFlowContext must see the flow context")
	}
	if seenByRouter != seenByWrapper {
		t.Error("the router must see the same flow context")
	}
}

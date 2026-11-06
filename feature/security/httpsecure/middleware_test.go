package httpsecure

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/9edang/inkgate/engine/config"
)

func TestNewSecureMw(t *testing.T) {
	cfg := config.ExtraConfig{
		Namespace: map[string]any{
			"allowed_hosts": []any{"host1", "subdomain1.host2", "subdomain2.host2"},
		},
	}
	mw := NewSecureMw(cfg)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))

	for status, URLs := range map[int][]string{
		http.StatusOK: {
			"http://host1/",
			"https://host1/",
			"http://subdomain1.host2/",
			"https://subdomain2.host2/",
		},
		http.StatusInternalServerError: {
			"http://unknown/",
			"https://subdomain.host1/",
			"http://host2/",
			"https://subdomain3.host2/",
		},
	} {
		for _, URL := range URLs {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", URL, http.NoBody)
			handler.ServeHTTP(w, req)
			if w.Result().StatusCode != status {
				t.Errorf("request %s unexpected status code! want %d, have %d\n", URL, status, w.Result().StatusCode)
			}
		}
	}
}

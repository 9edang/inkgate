package mux

import (
	"net/http/httptest"
	"testing"

	"github.com/9edang/inkgate/engine/proxy"
)

func TestRender(t *testing.T) {
	w := httptest.NewRecorder()
	Render(w, &proxy.Response{Data: map[string]any{"user": map[string]any{"name": "a<b"}}})
	if ct := w.Header().Get("Content-Type"); ct != "application/xml" {
		t.Errorf("content type: %s", ct)
	}
	if got := w.Body.String(); got != "<user><name>a&lt;b</name></user>" {
		t.Errorf("body: %s", got)
	}
}

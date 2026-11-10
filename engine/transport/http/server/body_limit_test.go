package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLimitBody(t *testing.T) {
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	h := limitBody(echo, 10)

	for _, tc := range []struct {
		name    string
		body    string
		chunked bool
		want    int
	}{
		{"under the limit", "0123456789", false, http.StatusOK},
		{"declared too large", "0123456789A", false, http.StatusRequestEntityTooLarge},
		{"chunked too large", "0123456789A", true, http.StatusRequestEntityTooLarge},
	} {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
		if tc.chunked {
			r.ContentLength = -1
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, w.Code, tc.want)
		}
	}

	if limitBody(echo, -1) == nil {
		t.Fatal("no limit must still return a handler")
	}
}

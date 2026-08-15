package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesAppAndSPAPaths(t *testing.T) {
	for _, path := range []string{"/", "/assets/index-C-SI29Po.js"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		res := httptest.NewRecorder()
		Handler().ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("%s: status=%d body=%q", path, res.Code, res.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	res := httptest.NewRecorder()
	Handler().ServeHTTP(res, req)
	if res.Code != http.StatusNotFound || strings.Contains(res.Body.String(), "KyNotes") {
		t.Fatalf("missing path: status=%d body=%q", res.Code, res.Body.String())
	}
	if got := Handler(); got == nil {
		t.Fatal("handler is nil")
	}
}

package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtectedFailureUsesErrorEnvelope(t *testing.T) {
	for _, h := range []http.Handler{RequireSession(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})), RequireDevice(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("status %d", w.Code)
		}
		var v struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if json.Unmarshal(w.Body.Bytes(), &v) != nil || v.Error.Code != "unauthenticated" {
			t.Fatalf("bad envelope %s", w.Body.String())
		}
	}
}

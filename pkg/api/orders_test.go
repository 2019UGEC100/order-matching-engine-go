package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateOrderInvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewBuffer([]byte("{bad json")))
	w := httptest.NewRecorder()

	CreateOrderHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json; got %d", w.Code)
	}
}

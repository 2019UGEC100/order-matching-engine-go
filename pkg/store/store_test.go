package store

import (
	"testing"

	"github.com/2019UGEC100/order-matching-engine-go/pkg/model"
)

func TestStoreBasics(t *testing.T) {
	s := NewStore()

	o := &model.Order{
		ID:       "test-123",
		Symbol:   "ABC",
		Side:     model.BUY,
		Type:     model.LIMIT,
		Price:    100,
		Quantity: 10,
	}

	// Add
	s.Add(o)

	// Get
	got, err := s.Get("test-123")
	if err != nil {
		t.Fatalf("expected to find order, got error: %v", err)
	}
	if got.ID != o.ID {
		t.Fatalf("expected ID %s, got %s", o.ID, got.ID)
	}

	// Remove
	err = s.Remove("test-123")
	if err != nil {
		t.Fatalf("expected remove success, got %v", err)
	}

	// Get after remove = error
	_, err = s.Get("test-123")
	if err == nil {
		t.Fatalf("expected error for missing order, got nil")
	}
}

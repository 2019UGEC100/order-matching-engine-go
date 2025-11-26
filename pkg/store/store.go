package store

import (
	"errors"
	"sync"

	"github.com/2019UGEC100/order-matching-engine-go/pkg/model"
)

// Store provides thread-safe order lookup by order ID.
// This is needed for synchronous cancel & GET /orders/{id}.
type Store struct {
	mu     sync.RWMutex
	orders map[string]*model.Order
}

func NewStore() *Store {
	return &Store{
		orders: make(map[string]*model.Order),
	}
}

func (s *Store) Add(o *model.Order) {
	s.mu.Lock()
	s.orders[o.ID] = o
	s.mu.Unlock()
}

func (s *Store) Get(id string) (*model.Order, error) {
	s.mu.RLock()
	o, ok := s.orders[id]
	s.mu.RUnlock()
	if !ok {
		return nil, errors.New("order not found")
	}
	return o, nil
}

func (s *Store) Remove(id string) error {
	s.mu.Lock()
	_, ok := s.orders[id]
	if ok {
		delete(s.orders, id)
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	return errors.New("order not found")
}

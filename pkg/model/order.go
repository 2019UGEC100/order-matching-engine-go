package model

import "errors"

type Side string
type OrderType string

const (
	BUY  Side = "BUY"
	SELL Side = "SELL"

	LIMIT  OrderType = "LIMIT"
	MARKET OrderType = "MARKET"
)

type Order struct {
	ID        string    `json:"order_id,omitempty"`
	Symbol    string    `json:"symbol"`
	Side      Side      `json:"side"`
	Type      OrderType `json:"type"`
	Price     int64     `json:"price,omitempty"` // integer cents
	Quantity  int64     `json:"quantity"`
	Filled    int64     `json:"filled_quantity,omitempty"`
	Timestamp int64     `json:"timestamp,omitempty"` // unix ms
}

// Validate checks basic syntactic correctness of the order.
// It does NOT perform business checks like available liquidity.
func (o *Order) Validate() error {
	if o == nil {
		return errors.New("order is nil")
	}
	if o.Symbol == "" {
		return errors.New("symbol is required")
	}
	if o.Side != BUY && o.Side != SELL {
		return errors.New("invalid side: must be BUY or SELL")
	}
	if o.Type != LIMIT && o.Type != MARKET {
		return errors.New("invalid type: must be LIMIT or MARKET")
	}
	if o.Quantity <= 0 {
		return errors.New("quantity must be > 0")
	}
	if o.Type == LIMIT {
		if o.Price <= 0 {
			return errors.New("limit orders must have price > 0 (in cents)")
		}
	}
	// For MARKET orders we do not require/validate price here (it will be ignored by matching logic)
	return nil
}

package engine

import (
	"testing"

	"github.com/2019UGEC100/order-matching-engine-go/pkg/model"
)

func newOrder(symbol string, side model.Side, typ model.OrderType, price, qty int64) *model.Order {
	return &model.Order{
		Symbol:   symbol,
		Side:     side,
		Type:     typ,
		Price:    price,
		Quantity: qty,
	}
}

func TestLimitBuySellMatch(t *testing.T) {
	ob := NewOrderBook("ABC")

	// Add resting sell
	sell := newOrder("ABC", model.SELL, model.LIMIT, 100, 10)
	trades, err := ob.ProcessOrder(sell)
	if err != nil || len(trades) != 0 {
		t.Fatal("sell should rest, no trades expected")
	}

	// Incoming buy crosses
	buy := newOrder("ABC", model.BUY, model.LIMIT, 105, 10)
	trades, err = ob.ProcessOrder(buy)
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if buy.Filled != 10 {
		t.Fatalf("expected buy filled 10, got %d", buy.Filled)
	}
}

func TestPartialFill(t *testing.T) {
	ob := NewOrderBook("XYZ")

	// Sell order 10 qty at 100
	s1 := newOrder("XYZ", model.SELL, model.LIMIT, 100, 10)
	ob.ProcessOrder(s1)

	// Buy order 6 qty at 100
	b1 := newOrder("XYZ", model.BUY, model.LIMIT, 100, 6)
	trades, _ := ob.ProcessOrder(b1)

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	if s1.Filled != 6 || b1.Filled != 6 {
		t.Fatalf("expected both sides filled 6, got sell=%d buy=%d", s1.Filled, b1.Filled)
	}
}

func TestMarketOrderReject(t *testing.T) {
	ob := NewOrderBook("LMN")

	// no liquidity
	mkt := newOrder("LMN", model.BUY, model.MARKET, 0, 5)
	_, err := ob.ProcessOrder(mkt)
	if err == nil {
		t.Fatalf("expected rejection for insufficient liquidity")
	}
}

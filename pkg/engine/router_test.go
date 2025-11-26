package engine

import (
	"testing"

	"github.com/2019UGEC100/order-matching-engine-go/pkg/model"
)

func TestRouterShardConsistency(t *testing.T) {
	r := NewRouter(4, 128)
	defer r.Stop()

	// same symbol should map to same shard idx
	idx1 := r.routeIdx("SYM-A")
	idx2 := r.routeIdx("SYM-A")
	if idx1 != idx2 {
		t.Fatalf("same symbol mapped to different shards: %d vs %d", idx1, idx2)
	}

	// submit an order and ensure it's stored in the correct shard
	o := &model.Order{
		ID:        "o-1",
		Symbol:    "SYM-A",
		Side:      model.BUY,
		Type:      model.LIMIT,
		Price:     500,
		Quantity:  10,
		Timestamp: 1,
	}

	res := r.SubmitOrder(o)
	if res.Err != "" {
		t.Fatalf("submit error: %s", res.Err)
	}
	if res.StatusCode != 201 {
		t.Fatalf("expected status 201 for resting limit, got %d", res.StatusCode)
	}

	// verify shard has the order
	sh := r.shards[idx1]
	if _, ok := sh.orders["o-1"]; !ok {
		t.Fatalf("order not present in shard orders map")
	}

	// get order via router
	got := r.GetOrder("SYM-A", "o-1")
	if got.Err != "" || got.Order == nil {
		t.Fatalf("expected to get order, got err=%s", got.Err)
	}
	if got.Order.ID != "o-1" {
		t.Fatalf("expected id o-1, got %s", got.Order.ID)
	}

	// cancel order
	c := r.CancelOrder("SYM-A", "o-1")
	if !c.OK {
		t.Fatalf("expected cancel ok, got err=%s", c.Err)
	}

	// after cancel, get should fail
	got2 := r.GetOrder("SYM-A", "o-1")
	if got2.Err == "" {
		t.Fatalf("expected get after cancel to fail")
	}
}

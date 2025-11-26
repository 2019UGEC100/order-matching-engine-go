package model

import "testing"

func TestOrderValidate(t *testing.T) {
	cases := []struct {
		name string
		o    *Order
		ok   bool
	}{
		{
			"valid limit buy",
			&Order{Symbol: "ABC", Side: BUY, Type: LIMIT, Price: 100, Quantity: 10},
			true,
		},
		{
			"valid market sell",
			&Order{Symbol: "XYZ", Side: SELL, Type: MARKET, Quantity: 5},
			true,
		},
		{
			"missing symbol",
			&Order{Side: BUY, Type: LIMIT, Price: 100, Quantity: 1},
			false,
		},
		{
			"invalid side",
			&Order{Symbol: "A", Side: "BLAH", Type: LIMIT, Price: 100, Quantity: 1},
			false,
		},
		{
			"invalid type",
			&Order{Symbol: "A", Side: BUY, Type: "FLOP", Price: 100, Quantity: 1},
			false,
		},
		{
			"zero quantity",
			&Order{Symbol: "A", Side: BUY, Type: LIMIT, Price: 100, Quantity: 0},
			false,
		},
		{
			"limit with zero price",
			&Order{Symbol: "A", Side: SELL, Type: LIMIT, Price: 0, Quantity: 2},
			false,
		},
	}

	for _, c := range cases {
		err := c.o.Validate()
		if c.ok && err != nil {
			t.Fatalf("case %q: expected valid but got error: %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Fatalf("case %q: expected error but got nil", c.name)
		}
	}
}

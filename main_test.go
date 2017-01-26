package main

import (
	"testing"

	"github.com/NebulousLabs/Sia/types"
)

func TestScaleSize(t *testing.T) {
	tests := []struct {
		size       uint64
		pricePerTB uint64 // in SC
		exp        uint64
	}{
		{0, 0, 0},
		{0, 500, 0},
		{100, 0, 0},
		{100, 1, 0},
		{100, 5, 2},
		{100, 50, 20},
		{100, 125, 50},
		{100, 250, 100},
		{100, 500, 100},
	}
	for _, test := range tests {
		price := types.SiacoinPrecision.Mul64(test.pricePerTB).Mul64(test.size).Div64(1e12)
		if f := scaleSize(test.size, price); f != test.exp {
			t.Errorf("Expected scaleSize(%v, %v) == %v; got %v", test.size, test.pricePerTB, test.exp, f)
		}
	}
}

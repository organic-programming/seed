package internal

import "testing"

func TestSquare(t *testing.T) {
	if got := Square(12); got != 144 {
		t.Fatalf("Square(12) = %d, want 144", got)
	}
}

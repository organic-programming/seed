package internal

import "testing"

func TestReverse(t *testing.T) {
	if got := Reverse("hello"); got != "olleh" {
		t.Fatalf("Reverse(hello) = %q, want %q", got, "olleh")
	}
}

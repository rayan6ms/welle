package limits

import "testing"

func TestBudgetCharge(t *testing.T) {
	b := NewBudget(10)
	if err := b.Charge(4); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := b.Charge(6); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := b.Charge(1); err == nil {
		t.Fatalf("expected error")
	}
}

func TestBudgetUnlimited(t *testing.T) {
	b := NewBudget(0)
	if err := b.Charge(1_000_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

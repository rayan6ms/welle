package formatutil

import "testing"

func TestGroupDigits(t *testing.T) {
	got, err := GroupDigitsFromString("14_310_023", ",", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "14,310,023" {
		t.Fatalf("expected grouped digits, got %q", got)
	}

	got, err = GroupDigitsFromInt(-14310023, ",", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "-14,310,023" {
		t.Fatalf("expected grouped negative digits, got %q", got)
	}
}

func TestGroupDigitsErrors(t *testing.T) {
	if _, err := GroupDigitsFromString("12a3", ",", 3); err == nil {
		t.Fatalf("expected invalid digits error")
	}
	if _, err := GroupDigitsFromInt(123, ",", 0); err == nil {
		t.Fatalf("expected invalid group error")
	}
}

func TestFormatFloatAndPercent(t *testing.T) {
	got, err := FormatFloat(1.234, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.23" {
		t.Fatalf("expected 1.23, got %q", got)
	}

	got, err = FormatFloat(1, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "1.000" {
		t.Fatalf("expected 1.000, got %q", got)
	}

	got, err = FormatPercent(0.1234, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "12.3%" {
		t.Fatalf("expected 12.3%%, got %q", got)
	}
}

func TestFormatFloatErrors(t *testing.T) {
	if _, err := FormatFloat(1.2, -1); err == nil {
		t.Fatalf("expected negative decimals error")
	}
	if _, err := FormatPercent(1.2, -1); err == nil {
		t.Fatalf("expected negative decimals error")
	}
}

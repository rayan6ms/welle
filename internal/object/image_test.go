package object

import (
	"reflect"
	"testing"
)

func TestImageSetBounds(t *testing.T) {
	img, err := NewImage(2, 2)
	if err != nil {
		t.Fatalf("NewImage error: %v", err)
	}
	if err := img.SetPixel(1, 1, 0, 0, 0, 255); err != nil {
		t.Fatalf("expected valid set, got %v", err)
	}
	if err := img.SetPixel(2, 0, 0, 0, 0, 255); err == nil {
		t.Fatal("expected out of bounds error for x")
	}
	if err := img.SetPixel(0, 2, 0, 0, 0, 255); err == nil {
		t.Fatal("expected out of bounds error for y")
	}
	if err := img.SetPixel(-1, 0, 0, 0, 0, 255); err == nil {
		t.Fatal("expected out of bounds error for negative x")
	}
	if err := img.SetPixel(0, -1, 0, 0, 0, 255); err == nil {
		t.Fatal("expected out of bounds error for negative y")
	}
	if err := img.SetPixel(0, 0, -1, 0, 0, 255); err == nil {
		t.Fatal("expected channel bounds error")
	}
}

func TestImageFillAndSet(t *testing.T) {
	img, err := NewImage(2, 2)
	if err != nil {
		t.Fatalf("NewImage error: %v", err)
	}
	if err := img.Fill(1, 2, 3, 4); err != nil {
		t.Fatalf("Fill error: %v", err)
	}
	for i := 0; i < len(img.Data); i += 4 {
		if img.Data[i] != 1 || img.Data[i+1] != 2 || img.Data[i+2] != 3 || img.Data[i+3] != 4 {
			t.Fatalf("unexpected fill at %d: %v", i, img.Data[i:i+4])
		}
	}
	if err := img.SetPixel(1, 0, 9, 8, 7, 6); err != nil {
		t.Fatalf("SetPixel error: %v", err)
	}
	idx := (0*img.Width + 1) * 4
	if got := img.Data[idx : idx+4]; !reflect.DeepEqual(got, []uint8{9, 8, 7, 6}) {
		t.Fatalf("unexpected pixel bytes: %v", got)
	}
}

func TestImageByteLayout(t *testing.T) {
	img, err := NewImage(2, 1)
	if err != nil {
		t.Fatalf("NewImage error: %v", err)
	}
	if err := img.SetPixel(0, 0, 10, 20, 30, 40); err != nil {
		t.Fatalf("SetPixel error: %v", err)
	}
	if err := img.SetPixel(1, 0, 50, 60, 70, 80); err != nil {
		t.Fatalf("SetPixel error: %v", err)
	}
	expect := []uint8{10, 20, 30, 40, 50, 60, 70, 80}
	if !reflect.DeepEqual(img.Data, expect) {
		t.Fatalf("unexpected byte layout: %v", img.Data)
	}
}

func TestImageFillRect(t *testing.T) {
	img, err := NewImage(4, 3)
	if err != nil {
		t.Fatalf("NewImage error: %v", err)
	}
	if err := img.FillRect(1, 1, 2, 2, 9, 8, 7, 6); err != nil {
		t.Fatalf("FillRect error: %v", err)
	}
	idx := (1*img.Width + 1) * 4
	if got := img.Data[idx : idx+4]; !reflect.DeepEqual(got, []uint8{9, 8, 7, 6}) {
		t.Fatalf("unexpected rect pixel: %v", got)
	}
	if err := img.FillRect(-1, 0, 1, 1, 0, 0, 0, 0); err == nil {
		t.Fatal("expected out of bounds error for FillRect")
	}
}

func TestImageFade(t *testing.T) {
	img, err := NewImage(1, 1)
	if err != nil {
		t.Fatalf("NewImage error: %v", err)
	}
	if err := img.SetPixel(0, 0, 100, 200, 50, 255); err != nil {
		t.Fatalf("SetPixel error: %v", err)
	}
	if err := img.Fade(0.25); err != nil {
		t.Fatalf("Fade error: %v", err)
	}
	if got := img.Data[:4]; !reflect.DeepEqual(got, []uint8{75, 150, 38, 191}) {
		t.Fatalf("unexpected fade bytes: %v", got)
	}
	if err := img.Fade(-0.1); err == nil {
		t.Fatal("expected range error for Fade")
	}
}

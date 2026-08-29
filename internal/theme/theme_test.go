package theme

import "testing"

func TestHexAndANSI(t *testing.T) {
	if got := Red.Hex(); got != "#ed8796" {
		t.Errorf("= %q, want the Macchiato red", got)
	}
	if got := Green.ANSI(); got != "\x1b[38;2;166;218;149m" {
		t.Errorf("= %q, want a truecolor escape for the Macchiato green", got)
	}
}

func TestLerpEndsAndMiddle(t *testing.T) {
	if got := Lerp(Green, Red, 0); got != Green {
		t.Errorf("= %v, want the start colour", got)
	}
	if got := Lerp(Green, Red, 1); got != Red {
		t.Errorf("= %v, want the end colour", got)
	}
	mid := Lerp(Green, Red, 0.5)
	if mid == Green || mid == Red {
		t.Errorf("= %v, want a blend of the two", mid)
	}
	if mid.R <= Green.R || mid.R >= Red.R {
		t.Errorf("R = %d, want it between %d and %d", mid.R, Green.R, Red.R)
	}
}

func TestLerpClamps(t *testing.T) {
	if got := Lerp(Green, Red, -1); got != Green {
		t.Errorf("= %v, want the start colour", got)
	}
	if got := Lerp(Green, Red, 2); got != Red {
		t.Errorf("= %v, want the end colour", got)
	}
}

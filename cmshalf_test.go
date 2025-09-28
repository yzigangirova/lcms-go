package golcms

import (
	"math"
	"testing"
)

func TestCmsHalf2Float_Zero(t *testing.T) {
	got := cmsHalf2Float(0x0000)
	if got != 0.0 {
		t.Errorf("Expected 0.0, got %f", got)
	}
}

/*func TestCmsHalf2Float_One(t *testing.T) {
	got := cmsHalf2Float(0x3C00)
	if math.Abs(float64(got-1.0)) > 1e-6 {
		t.Errorf("Expected 1.0, got %f", got)
	}
}

func TestCmsFloat2Half_One(t *testing.T) {
	h := cmsFloat2Half(1.0)
	if h != 0x3C00 {
		t.Errorf("Expected 0x3C00, got 0x%04X", h)
	}
}*/

func TestCmsFloat2Half_Zero(t *testing.T) {
	h := cmsFloat2Half(0.0)
	if h != 0x0000 {
		t.Errorf("Expected 0x0000, got 0x%04X", h)
	}
}

/*func TestCmsFloat2cmsHalf2Float_Roundtrip(t *testing.T) {
	orig := 0.75
	half := cmsFloat2Half(float32(orig))
	back := cmsHalf2Float(half)

	if math.Abs(float64(back)-orig) > 0.001 {
		t.Errorf("Roundtrip error: got %f from original %f", back, orig)
	}
}*/

/*func TestCmsFloat2Half_NaN(t *testing.T) {
	h := cmsFloat2Half(float32(math.NaN()))
	f := cmsHalf2Float(h)
	if !math.IsNaN(float64(f)) {
		t.Errorf("Expected NaN roundtrip, got %f", f)
	}
}*/

/*func TestCmsFloat2Half_PosInfinity(t *testing.T) {
	h := cmsFloat2Half(float32(math.Inf(1)))
	if h != 0x7C00 {
		t.Errorf("Expected +Inf half as 0x7C00, got 0x%04X", h)
	}
}

func TestCmsFloat2Half_NegInfinity(t *testing.T) {
	h := cmsFloat2Half(float32(math.Inf(-1)))
	if h != 0xFC00 {
		t.Errorf("Expected -Inf half as 0xFC00, got 0x%04X", h)
	}
}

func TestCmsHalf2Float_PosInfinity(t *testing.T) {
	f := cmsHalf2Float(0x7C00)
	if !math.IsInf(float64(f), 1) {
		t.Errorf("Expected +Inf, got %f", f)
	}
}

func TestCmsHalf2Float_NegInfinity(t *testing.T) {
	f := cmsHalf2Float(0xFC00)
	if !math.IsInf(float64(f), -1) {
		t.Errorf("Expected -Inf, got %f", f)
	}
}*/

func TestCmsFloat2Half_SmallValue(t *testing.T) {
	f := 1.0e-8
	h := cmsFloat2Half(float32(f))
	back := cmsHalf2Float(h)
	if math.Abs(float64(back)-f) > 1e-5 {
		t.Errorf("Tiny float32 roundtrip failed: orig=%g, back=%g", f, back)
	}
}

/*func TestCmsFloat2Half_Negative(t *testing.T) {
	h := cmsFloat2Half(-2.0)
	f := cmsHalf2Float(h)
	if math.Abs(float64(f)+2.0) > 0.01 {
		t.Errorf("Expected -2.0 roundtrip, got %f", f)
	}
}*/

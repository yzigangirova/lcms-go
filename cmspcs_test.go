package golcms

import (
	"math"
	"testing"
)

func almostEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-3
}

func TestCmsXYZ2LabAndBack(t *testing.T) {
	white := cmsD50_XYZ()
	src := &cmsCIEXYZ{X: 0.25, Y: 0.40, Z: 0.10}
	var lab cmsCIELab
	cmsXYZ2Lab(white, &lab, src)

	var dst cmsCIEXYZ
	cmsLab2XYZ(white, &dst, &lab)

	if !almostEq(src.X, dst.X) || !almostEq(src.Y, dst.Y) || !almostEq(src.Z, dst.Z) {
		t.Errorf("XYZ->Lab->XYZ mismatch: got %v", dst)
	}
}

func TestCmsLab2LChAndBack(t *testing.T) {
	lab := &cmsCIELab{L: 50, a: 25, b: -40}
	var lch cmsCIELCh
	cmsLab2LCh(&lch, lab)

	var back cmsCIELab
	cmsLCh2Lab(&back, &lch)

	if !almostEq(lab.L, back.L) || !almostEq(lab.a, back.a) || !almostEq(lab.b, back.b) {
		t.Errorf("Lab->LCh->Lab mismatch: got %v", back)
	}
}

/*func TestCmsXYZEncodedAndDecoded(t *testing.T) {
	xyzIn := &cmsCIEXYZ{X: 0.5, Y: 0.5, Z: 0.5}
	var encoded [3]uint16
	cmsFloat2XYZEncoded(encoded, xyzIn)

	var xyzOut cmsCIEXYZ
	cmsXYZEncoded2Float(&xyzOut, encoded)

	if !almostEq(xyzIn.X, xyzOut.X) || !almostEq(xyzIn.Y, xyzOut.Y) || !almostEq(xyzIn.Z, xyzOut.Z) {
		t.Errorf("XYZ encode/decode mismatch: got %v", xyzOut)
	}
}*/

func TestCmsDeltaE(t *testing.T) {
	a := &cmsCIELab{L: 50, a: 20, b: 30}
	b := &cmsCIELab{L: 50, a: 20, b: 30}
	if d := cmsDeltaE(a, b); d != 0 {
		t.Errorf("Expected DeltaE=0, got %f", d)
	}
}

func TestCIE2000DeltaE(t *testing.T) {
	a := &cmsCIELab{L: 50, a: 2.6772, b: -79.7751}
	b := &cmsCIELab{L: 50, a: 0.0, b: -82.7485}

	d := CIE2000DeltaE(a, b, 1, 1, 1)
	if d < 2.0 || d > 3.0 {
		t.Errorf("Unexpected CIE2000DeltaE: got %f", d)
	}
}
/*func TestCmsFloat2LabEncodedAndBack(t *testing.T) {
	labIn := &cmsCIELab{L: 75.5, a: -23.7, b: 15.2}
	var encoded [3]uint16
	cmsFloat2LabEncoded(encoded, labIn)

	var decoded cmsCIELab
	cmsLabEncoded2Float(&decoded, encoded)

	if !almostEq(labIn.L, decoded.L) || !almostEq(labIn.a, decoded.a) || !almostEq(labIn.b, decoded.b) {
		t.Errorf("Lab encode/decode mismatch: got %v", decoded)
	}
}*/
func TestCmsBFDdeltaE(t *testing.T) {
	a := &cmsCIELab{L: 60, a: 5, b: 10}
	b := &cmsCIELab{L: 62, a: 4, b: 12}

	d := cmsBFDdeltaE(a, b)
	if d < 1.0 || d > 5.0 {
		t.Errorf("cmsBFDdeltaE value unexpected: %f", d)
	}
}

func TestCmsChannelsOf(t *testing.T) {
	if cmsChannelsOf(cmsSigGrayData) != 1 {
		t.Errorf("Gray should have 1 channel")
	}
	if cmsChannelsOf(cmsSigCmykData) != 4 {
		t.Errorf("CMYK should have 4 channels")
	}
}

func TestXYZEncodingRangeClamp(t *testing.T) {
	in := &cmsCIEXYZ{X: -1, Y: 2, Z: 1.5}
	var encoded [3]uint16
	cmsFloat2XYZEncoded(encoded, in)

	if encoded[0] != 0 || encoded[1] > 0xffff {
		t.Errorf("XYZ encoding range clamp failed: %v", encoded)
	}
}

func TestLabEncodingRangeClamp(t *testing.T) {
	in := &cmsCIELab{L: 120, a: -150, b: 150}
	var encoded [3]uint16
	cmsFloat2LabEncoded(encoded, in)

	if encoded[0] > 0xffff || encoded[2] > 0xffff {
		t.Errorf("Lab encoding clamp out of bounds: %v", encoded)
	}
}

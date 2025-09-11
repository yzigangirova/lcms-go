package golcms

import (
	"math"
	"testing"
)

func TestCmsD50_XYZ(t *testing.T) {
	d50 := cmsD50_XYZ()
	if !almostEq(d50.X, 0.9642) || !almostEq(d50.Y, 1.0) || !almostEq(d50.Z, 0.8249) {
		t.Errorf("cmsD50_XYZ incorrect: %v", d50)
	}
}

func TestCmsD50_xyY(t *testing.T) {
	d50xy := cmsD50_xyY()
	if d50xy.Y_large != 0.824900 {
		t.Errorf("cmsD50_xyY Y should be 0.824900, got %f", d50xy.Y_large)
	}
}

func TestCmsWhitePointFromTemp_Valid(t *testing.T) {
	var wp CmsCIExyY
	err := cmsWhitePointFromTemp(&wp, 6500)
	if err == false {
		t.Errorf("cmsWhitePointFromTemp failed: %v", err)
	}
	if wp.X_small <= 0 || wp.Y_small <= 0 {
		t.Errorf("Invalid white point result: %v", wp)
	}
}

func TestCmsWhitePointFromTemp_Invalid(t *testing.T) {
	var wp CmsCIExyY
	err := cmsWhitePointFromTemp(&wp, 3000)
	if err == true {
		t.Errorf("Expected error for invalid temperature")
	}
}

func TestCmsTempFromWhitePoint_Roundtrip(t *testing.T) {
	var wp CmsCIExyY
	err := cmsWhitePointFromTemp(&wp, 6500)
	if err == false {
		t.Fatalf("Temp -> WhitePoint failed: %v", err)
	}

	var temp float64
	ok := cmsTempFromWhitePoint(&temp, &wp)
	if !ok || math.Abs(temp-6500) > 300 {
		t.Errorf("WhitePoint -> Temp failed, got %f", temp)
	}
}

func TestComputeChromaticAdaptation_Success(t *testing.T) {
	var r cmsMAT3
	ok := cmsAdaptationMatrix(&r, nil, &cmsCIEXYZ{X: 0.95, Y: 1.0, Z: 1.08}, cmsD50_XYZ())
	if !ok {
		t.Errorf("cmsAdaptationMatrix failed")
	}
}

/*func TestCmsAdaptMatrixToD50_Basic(t *testing.T) {
	var adapted cmsMAT3
	src := &cmsCIEXYZ{X: 0.95, Y: 1.0, Z: 1.09}

	ok := cmsAdaptMatrixToD50(&adapted, nil, src)
	if !ok {
		t.Errorf("cmsAdaptMatrixToD50 failed")
	}

	// Check diagonal-ish adaptation matrix
	if math.Abs(adapted.V[0].N[0]-1) > 0.2 {
		t.Errorf("Unexpected matrix diagonal: %v", adapted)
	}
}

func TestCmsBuildRGB2XYZtransferMatrix_sRGB(t *testing.T) {
	primaries := [3]CmsCIExyY{
		{0.64, 0.33, 1.0}, // Red
		{0.30, 0.60, 1.0}, // Green
		{0.15, 0.06, 1.0}, // Blue
	}
	wp := cmsD50_xyY()

	var result cmsMAT3
	ok := cmsBuildRGB2XYZtransferMatrix(&result, primaries, &wp)
	if !ok {
		t.Errorf("cmsBuildRGB2XYZtransferMatrix failed")
	}
}*/

/*func TestCmsAdaptToIlluminant_Identity(t *testing.T) {
	// Adapting a white point to itself should yield the same result
	input := &cmsCIEXYZ{X: 0.96, Y: 1.0, Z: 0.83}
	var output cmsCIEXYZ

	cmsAdaptToIlluminant(&output, nil, input, input)
	if math.Abs(output.X-input.X) > 1e-4 || math.Abs(output.Z-input.Z) > 1e-4 {
		t.Errorf("cmsAdaptToIlluminant identity check failed: got %v", output)
	}
}*/

func TestCmsAdaptToIlluminant_Forward(t *testing.T) {
	source := &cmsCIEXYZ{X: 0.9505, Y: 1.0, Z: 1.089}
	//dest := cmsD50_XYZ()
	color := &cmsCIEXYZ{X: 0.30, Y: 0.40, Z: 0.25}
	var adapted cmsCIEXYZ

	cmsAdaptToIlluminant(&adapted, nil, source, color)

	// Just check that adapted differs from input (since the illuminants differ)
	if math.Abs(adapted.X-color.X) < 0.001 {
		t.Errorf("cmsAdaptToIlluminant did not produce change")
	}
}

/*func TestCmsBuildRGB2XYZtransferMatrix_Invalid(t *testing.T) {
	// Use zeroed primaries: this should fail
	var primaries [3]CmsCIExyY
	wp := cmsD50_xyY()
	var result cmsMAT3

	ok := cmsBuildRGB2XYZtransferMatrix(&result, primaries, &wp)
	if ok {
		t.Errorf("Expected failure for zeroed primaries")
	}
}

func TestCmsAdaptationMatrix_SameSrcDst(t *testing.T) {
	w := cmsD50_XYZ()
	var mat cmsMAT3
	ok := cmsAdaptationMatrix(&mat, nil, &w, &w)
	if !ok {
		t.Errorf("cmsAdaptationMatrix should succeed for identical WP")
	}
}*/

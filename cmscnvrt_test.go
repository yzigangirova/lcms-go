package golcms

import (
	"math"
	"testing"
	//"unsafe"
)

func TestTranslateNonICCIntents(t *testing.T) {
	tests := []struct {
		in, want uint32
	}{
		{INTENT_PRESERVE_K_ONLY_PERCEPTUAL, INTENT_PERCEPTUAL},
		{INTENT_PRESERVE_K_ONLY_RELATIVE_COLORIMETRIC, INTENT_RELATIVE_COLORIMETRIC},
		{INTENT_PRESERVE_K_ONLY_SATURATION, INTENT_SATURATION},
		{INTENT_PRESERVE_K_PLANE_PERCEPTUAL, INTENT_PERCEPTUAL},
		{INTENT_PRESERVE_K_PLANE_RELATIVE_COLORIMETRIC, INTENT_RELATIVE_COLORIMETRIC},
		{INTENT_PRESERVE_K_PLANE_SATURATION, INTENT_SATURATION},
		{INTENT_PERCEPTUAL, INTENT_PERCEPTUAL},
	}
	for _, tt := range tests {
		got := TranslateNonICCIntents(tt.in)
		if got != tt.want {
			t.Errorf("TranslateNonICCIntents(%#x) = %#x; want %#x", tt.in, got, tt.want)
		}
	}
}

func TestColorSpaceIsCompatible(t *testing.T) {
	tests := []struct {
		a, b   cmsColorSpaceSignature
		expect bool
	}{
		{CmsSigCmykData, CmsSigCmykData, true},
		{CmsSig4colorData, CmsSigCmykData, true},
		{CmsSigCmykData, CmsSig4colorData, true},
		{CmsSigXYZData, CmsSigLabData, true},
		{CmsSigLabData, CmsSigXYZData, true},
		{CmsSigRgbData, CmsSigCmykData, false},
	}
	for _, tt := range tests {
		if got := ColorSpaceIsCompatible(tt.a, tt.b); got != tt.expect {
			t.Errorf("ColorSpaceIsCompatible(%#x, %#x) = %v; want %v", tt.a, tt.b, got, tt.expect)
		}
	}
}

func TestIsEmptyLayer_NilBoth(t *testing.T) {
	if !IsEmptyLayer(nil, nil) {
		t.Error("IsEmptyLayer(nil, nil) should be true")
	}
}

func TestIsEmptyLayer_NilMatrixOnly(t *testing.T) {
	off := cmsVEC3{}
	if IsEmptyLayer(nil, &off) {
		t.Error("IsEmptyLayer(nil, non-nil) should be false")
	}
}
func TestComputeBlackPointCompensation(t *testing.T) {
	in := cmsCIEXYZ{X: 0.1, Y: 0.1, Z: 0.1}
	out := cmsCIEXYZ{X: 0.2, Y: 0.2, Z: 0.2}
	var m cmsMAT3
	var off cmsVEC3

	ComputeBlackPointCompensation(&in, &out, &m, &off)

	// Expect scaling matrix and offset to be finite
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if math.IsNaN(m.V[i].N[j]) || math.IsInf(m.V[i].N[j], 0) {
				t.Errorf("ComputeBlackPointCompensation matrix has invalid value at [%d][%d]", i, j)
			}
		}
		if math.IsNaN(off.N[i]) || math.IsInf(off.N[i], 0) {
			t.Errorf("ComputeBlackPointCompensation offset has invalid value at %d", i)
		}
	}
}

func TestComputeAbsoluteIntent_Identity(t *testing.T) {
	var m cmsMAT3
	in := &cmsCIEXYZ{X: 0.9642, Y: 1.0, Z: 0.8249}
	out := &cmsCIEXYZ{X: 0.9642, Y: 1.0, Z: 0.8249}
	var CHAD cmsMAT3
	cmsMAT3identity(&CHAD)

	ok := ComputeAbsoluteIntent(1.0, in, &CHAD, out, &CHAD, &m)
	if !ok {
		t.Errorf("ComputeAbsoluteIntent failed unexpectedly")
	}
}

func TestComputeAbsoluteIntent_IncompleteAdaptation(t *testing.T) {
	var m cmsMAT3
	in := &cmsCIEXYZ{X: 0.9642, Y: 1.0, Z: 0.8249}
	out := &cmsCIEXYZ{X: 0.9505, Y: 1.0, Z: 1.0890}
	var CHAD cmsMAT3
	cmsMAT3identity(&CHAD)

	ok := ComputeAbsoluteIntent(0.5, in, &CHAD, out, &CHAD, &m)
	if !ok {
		t.Errorf("ComputeAbsoluteIntent failed on intermediate adaptation")
	}
}

func TestTemp2CHAD_and_CHAD2Temp(t *testing.T) {
	var chad cmsMAT3
	Temp2CHAD(&chad, 6500.0)

	temp := CHAD2Temp(&chad)

	if temp < 1000 || temp > 20000 {
		t.Errorf("CHAD2Temp returned unrealistic value: %f", temp)
	}
}

func TestSearchIntent_DefaultList(t *testing.T) {
	// Make sure the default list is initialized
	initDefaultIntents()

	// Try to find a known default
	intent := SearchIntent(nil, INTENT_PERCEPTUAL)
	if intent == nil {
		t.Errorf("SearchIntent failed to find INTENT_PERCEPTUAL")
	}
	if intent != nil && intent.Description != "Perceptual" {
		t.Errorf("SearchIntent returned incorrect description: %s", intent.Description)
	}
}
func TestIsEmptyLayer_IdentityZero(t *testing.T) {
	var m cmsMAT3
	var v cmsVEC3
	cmsMAT3identity(&m)
	cmsVEC3init(&v, 0, 0, 0)

	if !IsEmptyLayer(&m, &v) {
		t.Errorf("IsEmptyLayer should return true for identity matrix + zero offset")
	}
}

func TestAddConversion_XYZtoLab(t *testing.T) {
	var m cmsMAT3
	var v cmsVEC3
	cmsMAT3identity(&m)
	cmsVEC3init(&v, 0, 0, 0)

	p := cmsPipelineAlloc(nil, nil, 3, 3)
	defer cmsPipelineFree(nil, p)

	ok := AddConversion(nil, p, CmsSigXYZData, CmsSigLabData, &m, &v)
	if !ok {
		t.Errorf("AddConversion failed for XYZ → Lab")
	}
}

func TestAddConversion_LabToLabWithMatrix(t *testing.T) {
	var m cmsMAT3
	var v cmsVEC3
	cmsMAT3identity(&m)
	cmsVEC3init(&v, 1.0, 0.0, -1.0)

	p := cmsPipelineAlloc(nil, nil, 3, 3)
	defer cmsPipelineFree(nil, p)

	ok := AddConversion(nil, p, CmsSigLabData, CmsSigLabData, &m, &v)
	if !ok {
		t.Errorf("AddConversion failed for Lab → Lab with matrix")
	}
}

func TestComputeConversion_BPCEqualPoints(t *testing.T) {
	var m cmsMAT3
	var v cmsVEC3

	profiles := []CmsHPROFILE{CmsCreate_sRGBProfile(nil), CmsCreate_sRGBProfile(nil), CmsCreate_sRGBProfile(nil)}
	defer CmsCloseProfile(nil, profiles[0])
	defer CmsCloseProfile(nil, profiles[1])

	ok := ComputeConversion(nil, 1, profiles, INTENT_RELATIVE_COLORIMETRIC, true, 1.0, &m, &v)
	if !ok {
		t.Errorf("ComputeConversion failed on sRGB self-transform")
	}
}

func TestCmsLinkProfiles_Basic(t *testing.T) {
	profiles := []CmsHPROFILE{CmsCreate_sRGBProfile(nil), CmsCreate_sRGBProfile(nil)}
	defer CmsCloseProfile(nil, profiles[0])
	defer CmsCloseProfile(nil, profiles[1])

	intents := []uint32{INTENT_PERCEPTUAL, INTENT_PERCEPTUAL}
	bpc := []bool{false, false}
	adapt := []float64{1.0, 1.0}

	p := cmsLinkProfiles(nil, nil, 2, intents, profiles, bpc, adapt, 0)
	if p == nil {
		t.Errorf("cmsLinkProfiles returned nil unexpectedly")
	} else {
		cmsPipelineFree(nil, p)
	}
}

func TestCmsRegisterRenderingIntentPlugin_Reset(t *testing.T) {
	ok := cmsRegisterRenderingIntentPlugin(nil, nil, nil)
	if !ok {
		t.Errorf("cmsRegisterRenderingIntentPlugin(nil) should return true")
	}
}
func TestCmsDefaultICCintents_SimpleSRGB(t *testing.T) {
	profiles := []CmsHPROFILE{CmsCreate_sRGBProfile(nil)}
	defer CmsCloseProfile(nil, profiles[0])

	intents := []uint32{INTENT_PERCEPTUAL}
	bpc := []bool{false}
	adapt := []float64{1.0}

	p := cmsDefaultICCintents(nil, nil, 1, intents, profiles, bpc, adapt, 0)
	if p == nil {
		t.Errorf("cmsDefaultICCintents returned nil on sRGB profile")
	} else {
		cmsPipelineFree(nil, p)
	}
}

func TestBlackPreservingGrayOnlySampler_KOnly(t *testing.T) {
	var out = make([]uint16, 4)
	var in = []uint16{0, 0, 0, 32768} // K-only input

	kTone := CmsBuildGamma(nil, nil, 1.0)
	defer CmsFreeToneCurve(kTone)

	p := &GrayOnlyParams{
		Cmyk2Cmyk: nil,
		KTone:     kTone,
	}

	ok := BlackPreservingGrayOnlySampler(nil, in, out, p)
	if ok != 1 {
		t.Errorf("BlackPreservingGrayOnlySampler returned %d; want 1", ok)
	}
	if out[3] == 0 {
		t.Errorf("Expected transformed K value, got 0")
	}
}

func TestBlackPreservingKOnlyIntents_SingleSRGB(t *testing.T) {
	profiles := []CmsHPROFILE{CmsCreate_sRGBProfile(nil)}
	defer CmsCloseProfile(nil, profiles[0])

	intents := []uint32{INTENT_PRESERVE_K_ONLY_PERCEPTUAL}
	bpc := []bool{true}
	adapt := []float64{1.0}

	p := BlackPreservingKOnlyIntents(nil, nil, 1, intents, profiles, bpc, adapt, 0)
	if p == nil {
		t.Errorf("BlackPreservingKOnlyIntents returned nil unexpectedly")
	} else {
		cmsPipelineFree(nil, p)
	}
}

func TestBlackPreservingSampler_KOnly(t *testing.T) {
	in := []uint16{0, 0, 0, 40000}
	out := make([]uint16, 4)

	p := &PreserveKPlaneParams{
		Cmyk2Cmyk: nil,
		KTone:     CmsBuildGamma(nil, nil, 1.0),
	}

	defer CmsFreeToneCurve(p.KTone)

	got := BlackPreservingSampler(nil, in, out, p)
	if got != 1 {
		t.Errorf("BlackPreservingSampler should return 1 on success")
	}
	if out[3] == 0 {
		t.Errorf("BlackPreservingSampler did not produce output K")
	}
}

func TestCmsTempFromWhitePoint_RoundTrip(t *testing.T) {
	var temp float64
	D50 := cmsD50_xyY()

	ok := cmsTempFromWhitePoint(&temp, D50)
	if !ok || temp < 4000 || temp > 10000 {
		t.Errorf("cmsTempFromWhitePoint returned %f, expected valid daylight temp", temp)
	}
}

func TestCmsWhitePointFromTemp_D50(t *testing.T) {
	var xyy CmsCIExyY
	cmsWhitePointFromTemp(&xyy, 5000.0)

	if xyy.Y_large < 0.9 || xyy.Y_large > 1.1 {
		t.Errorf("cmsWhitePointFromTemp: bad Y=%f", xyy.Y_large)
	}
}

/*func TestTemp2CHAD_Bounds(t *testing.T) {
	var m cmsMAT3
	Temp2CHAD(&m, 3000.0)

	// Sanity check: no NaNs
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			if math.IsNaN(m.V[r].N[c]) {
				t.Errorf("Temp2CHAD produced NaN at [%d][%d]", r, c)
			}
		}
	}
}*/

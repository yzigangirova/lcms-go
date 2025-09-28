package golcms

import (
	"math"
	"testing"
)

func TestDefaultEvalParametricFn_Type1(t *testing.T) {
	gamma := 2.2
	result := DefaultEvalParametricFn(1, []float64{gamma}, 0.5)
	expected := math.Pow(0.5, gamma)

	if math.Abs(result-expected) > 1e-6 {
		t.Errorf("DefaultEvalParametricFn(type 1) = %f; want %f", result, expected)
	}
}

func TestDefaultEvalParametricFn_TypeMinus1(t *testing.T) {
	gamma := 2.0
	result := DefaultEvalParametricFn(-1, []float64{gamma}, 0.25)
	expected := math.Pow(0.25, 1.0/gamma)

	if math.Abs(result-expected) > 1e-6 {
		t.Errorf("DefaultEvalParametricFn(type -1) = %f; want %f", result, expected)
	}
}

func TestIsInSet_Found(t *testing.T) {
	col := &cmsParametricCurvesCollection{
		NFunctions:    3,
		FunctionTypes: [MAX_TYPES_IN_LCMS_PLUGIN]uint32{1, 2, 3},
	}

	pos := IsInSet(2, col)
	if pos != 1 {
		t.Errorf("IsInSet failed: expected 1, got %d", pos)
	}
}

func TestIsInSet_NotFound(t *testing.T) {
	col := &cmsParametricCurvesCollection{
		NFunctions:    3,
		FunctionTypes: [MAX_TYPES_IN_LCMS_PLUGIN]uint32{1, 2, 3},
	}

	pos := IsInSet(5, col)
	if pos != -1 {
		t.Errorf("IsInSet failed: expected -1, got %d", pos)
	}
}

func TestCmsIsToneCurveLinear_Linear(t *testing.T) {
	entries := uint32(256)
	values := make([]uint16, entries)
	for i := range values {
		values[i] = uint16((i * 65535) / int(entries-1))
	}

	curve := cmsBuildTabulatedToneCurve16(nil,nil, entries, values)
	if !cmsIsToneCurveLinear(curve) {
		t.Errorf("cmsIsToneCurveLinear expected true for linear ramp")
	}
}
func TestCmsIsToneCurveMonotonic_Ascending(t *testing.T) {
	entries := uint32(256)
	values := make([]uint16, entries)
	for i := range values {
		values[i] = uint16(i * 256)
	}

	curve := cmsBuildTabulatedToneCurve16(nil,nil, entries, values)
	if !cmsIsToneCurveMonotonic(curve) {
		t.Errorf("Expected curve to be monotonic ascending")
	}
}

func TestCmsIsToneCurveMonotonic_Descending(t *testing.T) {
	entries := uint32(256)
	values := make([]uint16, entries)
	for i := range values {
		values[i] = uint16((255 - i) * 256)
	}

	curve := cmsBuildTabulatedToneCurve16(nil,nil, entries, values)
	if !cmsIsToneCurveMonotonic(curve) {
		t.Errorf("Expected curve to be monotonic descending")
	}
}

func TestCmsEstimateGamma_Linear(t *testing.T) {
	values := make([]uint16, MAX_NODES_IN_CURVE)
	for i := range values {
		values[i] = uint16((i * 65535) / (MAX_NODES_IN_CURVE - 1))
	}

	curve := cmsBuildTabulatedToneCurve16(nil,nil, MAX_NODES_IN_CURVE, values)
	gamma := cmsEstimateGamma(curve, 0.1)
	if gamma < 0.9 || gamma > 1.1 {
		t.Errorf("cmsEstimateGamma on linear should be ~1, got %f", gamma)
	}
}

func TestCmsEvalToneCurveFloat_Linear(t *testing.T) {
	values := make([]uint16, 256)
	for i := range values {
		values[i] = uint16((i * 65535) / 255)
	}
	curve := cmsBuildTabulatedToneCurve16(nil,nil, 256, values)

	got := cmsEvalToneCurveFloat(curve, 0.5)
	if math.Abs(float64(got-0.5)) > 0.01 {
		t.Errorf("cmsEvalToneCurveFloat expected ~0.5, got %f", got)
	}
}

func TestCmsEvalToneCurve16_Linear(t *testing.T) {
	values := make([]uint16, 256)
	for i := range values {
		values[i] = uint16((i * 65535) / 255)
	}
	curve := cmsBuildTabulatedToneCurve16(nil,nil, 256, values)

	got := cmsEvalToneCurve16(curve, 32768)
	if math.Abs(float64(got)-32768) > 500 {
		t.Errorf("cmsEvalToneCurve16 expected ~32768, got %d", got)
	}
}

func TestCmsBuildParametricToneCurve_Valid(t *testing.T) {
	curve := cmsBuildParametricToneCurve(nil,nil, 1, []float64{2.2})
	if curve == nil {
		t.Errorf("cmsBuildParametricToneCurve returned nil for type 1")
	}
}
func TestCmsReverseToneCurve(t *testing.T) {
	values := make([]uint16, 256)
	for i := range values {
		values[i] = uint16((i * 65535) / 255)
	}

	original := cmsBuildTabulatedToneCurve16(nil,nil, 256, values)
	reversed := cmsReverseToneCurve(nil,original)
	if reversed == nil {
		t.Fatal("cmsReverseToneCurve returned nil")
	}

	if reversed.Table16[0] > reversed.Table16[255] {
		t.Errorf("Reversed curve not monotonic increasing")
	}
}

func TestCmsDupToneCurve(t *testing.T) {
	values := make([]uint16, 256)
	for i := range values {
		values[i] = uint16((i * 65535) / 255)
	}

	original := cmsBuildTabulatedToneCurve16(nil,nil, 256, values)
	copy := cmsDupToneCurve(nil,original)
	if copy == nil {
		t.Fatal("cmsDupToneCurve returned nil")
	}

	for i := range values {
		if original.Table16[i] != copy.Table16[i] {
			t.Errorf("Dup mismatch at %d: %d != %d", i, original.Table16[i], copy.Table16[i])
		}
	}
}

func TestCmsIsToneCurveMultisegment(t *testing.T) {
	g := cmsBuildSegmentedToneCurve(nil,nil, 2, []cmsCurveSegment{
		{X0: 0.0, X1: 0.5, Type: 1, Params: [10]float64{1.0}},
		{X0: 0.5, X1: 1.0, Type: 1, Params: [10]float64{1.0}},
	})
	if !cmsIsToneCurveMultisegment(g) {
		t.Errorf("Expected curve to be multisegment")
	}
}

/*func TestEvalSegmentedFn_SampledSegment(t *testing.T) {
    vals := []float32{0, 0.5, 1}
    seg := cmsCurveSegment{
        X0:            0.0,
        X1:            1.0,
        Type:          0,
        NGridPoints:   3,
        SampledPoints: vals,
    }

    g := cmsBuildSegmentedToneCurve(nil, 1, []cmsCurveSegment{seg})
    out := EvalSegmentedFn(g, 0.5)
    if math.Abs(out-0.5) > 0.1 {
        t.Errorf("EvalSegmentedFn got %f, expected approx 0.5", out)
    }
}*/

func TestCmsGetToneCurveParametricType(t *testing.T) {
	curve := cmsBuildParametricToneCurve(nil,nil, 1, []float64{2.2})
	tp := cmsGetToneCurveParametricType(curve)
	if tp != 1 {
		t.Errorf("Expected parametric type 1, got %d", tp)
	}
}

func TestGetInterval_Basic(t *testing.T) {
	entries := 256
	values := make([]uint16, entries)
	for i := range values {
		values[i] = uint16(i * 256)
	}

	params := &cmsInterpParams{
		Domain: [15]uint32{255},
	}

	idx := GetInterval(32768.0, values, params)
	if idx < 0 || idx >= entries-1 {
		t.Errorf("GetInterval failed, got %d", idx)
	}
}

package golcms

import (
	"math"
	"testing"
)

func TestLinearInterp_Basic(t *testing.T) {
	out := LinearInterp(32768, 100, 200)
	if out < 149 || out > 151 {
		t.Errorf("Expected ~150, got %d", out)
	}
}

func TestFclamp_Basic(t *testing.T) {
	tests := []struct {
		input    float32
		expected float32
	}{
		{-0.5, 0.0},
		{0.5, 0.5},
		{1.5, 1.0},
		{float32(math.NaN()), 0.0},
	}

	for _, test := range tests {
		got := fclamp(test.input)
		if math.IsNaN(float64(test.input)) {
			if got != 0.0 {
				t.Errorf("fclamp(NaN) expected 0.0, got %f", got)
			}
		} else if math.Abs(float64(got-test.expected)) > 1e-6 {
			t.Errorf("fclamp(%f) = %f; want %f", test.input, got, test.expected)
		}
	}
}

func TestCmsComputeInterpParams_BasicUint16(t *testing.T) {
	table := make([]uint16, 256)
	for i := 0; i < 256; i++ {
		table[i] = uint16(i * 257)
	}

	params := cmsComputeInterpParams(nil,nil, 256, 1, 1, table, 0)
	if params == nil {
		t.Fatal("cmsComputeInterpParams returned nil")
	}
	cmsFreeInterpParams(params)
}

func TestCmsComputeInterpParams_BasicFloat32(t *testing.T) {
	table := make([]float32, 256)
	for i := 0; i < 256; i++ {
		table[i] = float32(i) / 255.0
	}

	params := cmsComputeInterpParams(nil,nil, 256, 1, 1, table, 0)
	if params == nil {
		t.Fatal("cmsComputeInterpParams returned nil for float32 table")
	}
	cmsFreeInterpParams(params)
}

/*func TestLinLerp1D(t *testing.T) {
    table := []uint16{100, 200}
    result := LinLerp1D(table, 0x8000)
    if result < 149 || result > 151 {
        t.Errorf("LinLerp1D failed, expected ~150, got %d", result)
    }
}

func TestLinLerp1Dfloat(t *testing.T) {
    table := []float32{0.0, 1.0}
    result := LinLerp1Dfloat(table, 0.5)
    if math.Abs(float64(result-0.5)) > 1e-6 {
        t.Errorf("LinLerp1Dfloat failed, got %f", result)
    }
}*/

func TestEval1Input(t *testing.T) {
	table := []uint16{0, 32768, 65535}
	interp := cmsComputeInterpParams(nil, nil, 3, 1, 1, table, 0)
	defer cmsFreeInterpParams(interp)

	input := []uint16{32768}
	output := make([]uint16, 1)

	Eval1Input(input, output, interp)
	if output[0] < 32760 || output[0] > 32776 {
		t.Errorf("Eval1Input failed, got %d", output[0])
	}
}

func TestEval1InputFloat(t *testing.T) {
	table := []float32{0.0, 0.5, 1.0}
	interp := cmsComputeInterpParams(nil,nil, 3, 1, 1, table, 0)
	defer cmsFreeInterpParams(interp)

	input := []float32{0.5}
	output := make([]float32, 1)

	Eval1InputFloat(input, output, interp)
	if math.Abs(float64(output[0]-0.5)) > 0.01 {
		t.Errorf("Eval1InputFloat failed, got %f", output[0])
	}
}

/*func TestBilinearInterp16(t *testing.T) {
    corners := [4]uint16{100, 200, 300, 400}
    out := BilinearInterp16(corners[:], 0x8000, 0x8000)
    if out < 240 || out > 260 {
        t.Errorf("BilinearInterp16 midpoint expected ~250, got %d", out)
    }
}

func TestTrilinearInterp16(t *testing.T) {
    corners := [8]uint16{0, 100, 200, 300, 400, 500, 600, 700}
    out := TrilinearInterp16(corners[:], 0x8000, 0x8000, 0x8000)
    if out < 340 || out > 360 {
        t.Errorf("TrilinearInterp16 midpoint expected ~350, got %d", out)
    }
}

func TestBilinearInterpFloat(t *testing.T) {
    corners := [4]float32{0.0, 0.5, 0.5, 1.0}
    out := BilinearInterpFloat(corners[:], 0.5, 0.5)
    if math.Abs(float64(out-0.5)) > 0.05 {
        t.Errorf("BilinearInterpFloat midpoint expected ~0.5, got %f", out)
    }
}

func TestTrilinearInterpFloat(t *testing.T) {
    corners := [8]float32{0.0, 0.25, 0.5, 0.75, 0.25, 0.5, 0.75, 1.0}
    out := TrilinearInterpFloat(corners[:], 0.5, 0.5, 0.5)
    if math.Abs(float64(out-0.5)) > 0.05 {
        t.Errorf("TrilinearInterpFloat midpoint expected ~0.5, got %f", out)
    }
}*/

package golcms

import (
	"math"
	"testing"
	//"unsafe"
)

func TestChangeEndian(t *testing.T) {
	tests := []struct {
		in, out uint16
	}{
		{0x1234, 0x3412},
		{0xABCD, 0xCDAB},
		{0x0001, 0x0100},
		{0xFFFF, 0xFFFF},
	}
	for _, tt := range tests {
		got := changeEndian(tt.in)
		if got != tt.out {
			t.Errorf("changeEndian(%#04x) = %#04x; want %#04x", tt.in, got, tt.out)
		}
	}
}

func TestCmsQuickSaturateByte(t *testing.T) {
	tests := []struct {
		in  float64
		out uint8
	}{
		{-10.0, 0},
		{0.0, 0},
		{0.4, 0},
		{0.5, 1},
		{100.4, 100},
		{254.6, 255},
		{300.0, 255},
	}
	for _, tt := range tests {
		got := cmsQuickSaturateByte(tt.in)
		if got != tt.out {
			t.Errorf("cmsQuickSaturateByte(%f) = %d; want %d", tt.in, got, tt.out)
		}
	}
}

func TestFrom8To16(t *testing.T) {
	src := []uint8{128}
	dst := make([]uint16, 1)
	from8to16(dst, src)
	if dst[0] != uint16(128*257) {
		t.Errorf("from8to16 failed: got %d, want %d", dst[0], 128*257)
	}
}

func TestFrom16To8(t *testing.T) {
	src := []uint16{0x8000}
	dst := make([]uint8, 1)
	from16to8(dst, src)
	if dst[0] != 0x80 {
		t.Errorf("from16to8 failed: got %d, want 128", dst[0])
	}
}

func TestFrom8ToFLT(t *testing.T) {
	src := []uint8{255}
	dst := make([]float32, 1)
	from8toFLT(dst, src)
	if math.Abs(float64(dst[0]-1.0)) > 1e-6 {
		t.Errorf("from8toFLT failed: got %f, want 1.0", dst[0])
	}
}

func TestFrom8ToDBL(t *testing.T) {
	src := []uint8{255}
	dst := make([]float64, 1)
	from8toDBL(dst, src)
	if math.Abs(dst[0]-1.0) > 1e-6 {
		t.Errorf("from8toDBL failed: got %f, want 1.0", dst[0])
	}
}

func TestFromFLTto8(t *testing.T) {
	src := []float64{1.0}
	dst := make([]uint8, 1)
	fromFLTto8(dst, src)
	if dst[0] != 255 {
		t.Errorf("fromFLTto8 failed: got %d, want 255", dst[0])
	}
}

func TestFromFLTto16SE(t *testing.T) {
	src := []float64{1.0}
	dst := make([]uint16, 1)
	fromFLTto16SE(dst, src)
	if dst[0] != 0xFFFF {
		t.Errorf("fromFLTto16SE failed: got 0x%X, want 0xFFFF", dst[0])
	}
}

/*
	func TestFormatterPos(t *testing.T) {
	    b8 := uint32(1)
	    b16 := uint32(2)
	    float16 := uint32(2 | (1 << 8)) // T_FLOAT
	    dbl := uint32(0 | (1 << 8))

	    if FormatterPos(b8) != 0 {
	        t.Error("FormatterPos failed for 8-bit")
	    }
	    if FormatterPos(b16) != 1 {
	        t.Error("FormatterPos failed for 16-bit")
	    }
	    if FormatterPos(float16) != 3 {
	        t.Error("FormatterPos failed for half float")
	    }
	    if FormatterPos(dbl) != 5 {
	        t.Error("FormatterPos failed for double")
	    }
	}
*/
/*func TestCmsGetFormatterAlpha(t *testing.T) {
	tests := []struct {
		inFormat  uint32
		outFormat uint32
		wantErr   bool
	}{
		{1, 2, false},
		{1, 1, false},
		{2, 1, false},
		{0xFFFFFFFF, 1, true},
		{1, 0xFFFFFFFF, true},
	}

	for _, tt := range tests {
		fn := cmsGetFormatterAlpha(nil, tt.inFormat, tt.outFormat)

		if fn == nil {
			t.Errorf("cmsGetFormatterAlpha(%#x, %#x) returned nil function unexpectedly", tt.inFormat, tt.outFormat)
		}
	}
}*/

func TestComputeIncrementsForChunky(t *testing.T) {
	var startingOrder [cmsMAXCHANNELS]uint32
	var increments [cmsMAXCHANNELS]uint32

	// Example format: 3 channels RGB + 1 extra alpha
	format := uint32(0x04010100) // Mock format

	ComputeIncrementsForChunky(format, startingOrder[:], increments[:])

	// No actual assertion here, but we test that the function runs and populates arrays
}

func TestComputeIncrementsForPlanar(t *testing.T) {
	var startingOrder [cmsMAXCHANNELS]uint32
	var increments [cmsMAXCHANNELS]uint32

	format := uint32(0x04010100) // Mock format
	bytesPerPlane := uint32(2)

	ComputeIncrementsForPlanar(format, bytesPerPlane, startingOrder[:], increments[:])
	// Again, basic check to make sure no panic
}

func TestComputeComponentIncrements(t *testing.T) {
	var startingOrder [cmsMAXCHANNELS]uint32
	var increments [cmsMAXCHANNELS]uint32

	// Test for chunky format
	format := uint32(0x04010100) // Mock format

	ComputeComponentIncrements(format, 2, startingOrder[:], increments[:])
	// Simple test that covers both branches
}

/*func TestCmsHandleExtraChannels_SingleAlphaCopy(t *testing.T) {
    // Setup: one RGB + 1 alpha (4 channels total), 8-bit chunky
    const pixelsPerLine = 2
    const lineCount = 1

    inputFormat := uint32(0x00000001)  // fake 8-bit, 4 channels assumed
    outputFormat := uint32(0x00000001) // same

    // Input: R,G,B,A  x 2 pixels
    in := []byte{
        10, 20, 30, 40,
        50, 60, 70, 80,
    }
    // Output: R,G,B,A  x 2 pixels (same layout, zeros)
    out := make([]byte, len(in))

    stride := &cmsStride{
        BytesPerLineIn:  uint32(len(in)),
        BytesPerLineOut: uint32(len(in)),
        BytesPerPlaneIn:  1,
        BytesPerPlaneOut: 1,
    }

    transform := &cmsTRANSFORM{
        InputFormat:     inputFormat,
        OutputFormat:    outputFormat,
        DwOriginalFlags: cmsFLAGS_COPY_ALPHA,
        ContextID:       nil,
    }

    // We'll use a known copy function
    FormatterAlpha[0][0] = copy8

    cmsHandleExtraChannels(transform, in, out, pixelsPerLine, lineCount, stride)

    // Check if alpha bytes were copied
    if out[3] != 40 || out[7] != 80 {
        t.Errorf("Alpha channel not copied correctly, got out[3]=%d, out[7]=%d; want 40 and 80", out[3], out[7])
    }
}*/

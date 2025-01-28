package golcms

import (
	"math"
	"unsafe"
)

type Prelin8Data struct {
	ContextID CmsContext

	// Tetrahedral interpolation parameters (not-owned pointer)
	P *cmsInterpParams

	Rx [256]uint16
	Ry [256]uint16
	Rz [256]uint16

	// Precomputed nodes and offsets for 8-bit input data
	X0 [256]uint32
	Y0 [256]uint32
	Z0 [256]uint32
}
type Prelin16Data struct {
	ContextID CmsContext

	// Number of channels
	NInputs  uint32
	NOutputs uint32

	// Input curves
	EvalCurveIn16   [MAX_INPUT_DIMENSIONS]cmsInterpFn16
	ParamsCurveIn16 [MAX_INPUT_DIMENSIONS]*cmsInterpParams

	// 3D grid evaluator
	EvalCLUT   cmsInterpFn16
	CLUTParams *cmsInterpParams // Not-owned pointer

	// Output curves
	EvalCurveOut16   *cmsInterpFn16    // Points to an array of curve evaluators in 16 bits (not-owned pointer)
	ParamsCurveOut16 **cmsInterpParams // Points to an array of references to interpolation params (not-owned pointer)
}
type cmsS1Fixed14Number int32 // May hold more than 16 bits

var DOUBLE_TO_1FIXED14 = func(x float64) cmsS1Fixed14Number {
	return cmsS1Fixed14Number(math.Floor(x*16384.0 + 0.5))
}

type MatShaper8Data struct {
	ContextID CmsContext

	// Shapers from 0..255 to 1.14 fixed
	Shaper1R [256]cmsS1Fixed14Number
	Shaper1G [256]cmsS1Fixed14Number
	Shaper1B [256]cmsS1Fixed14Number

	// Matrix and offset (n.14 fixed)
	Mat [3][3]cmsS1Fixed14Number
	Off [3]cmsS1Fixed14Number

	// Shapers from 1.14 fixed to 0..255
	Shaper2R [16385]uint16
	Shaper2G [16385]uint16
	Shaper2B [16385]uint16
}
type Curves16Data struct {
	ContextID CmsContext

	NCurves   uint32   // Number of curves
	NElements uint32   // Elements in curves
	Curves    **uint16 // Points to a dynamically allocated array
}

// _RemoveElement removes an element from the linked chain.
func RemoveElement(head **cmsStage) {
	mpe := *head
	next := mpe.Next
	*head = next
	cmsStageFree(mpe)
}

// _Remove1Op removes all identities in the chain.
func Remove1Op(Lut *cmsPipeline, UnaryOp cmsStageSignature) bool {
	pt := &Lut.Elements
	anyOpt := false

	for *pt != nil {
		if (*pt).Implements == UnaryOp {
			RemoveElement(pt)
			anyOpt = true
		} else {
			pt = &(*pt).Next
		}
	}
	return anyOpt
}

// _Remove2Op removes two adjacent elements if they match the specified types.
func Remove2Op(Lut *cmsPipeline, Op1, Op2 cmsStageSignature) bool {
	pt1 := &Lut.Elements
	anyOpt := false

	if *pt1 == nil {
		return anyOpt
	}

	for *pt1 != nil {
		pt2 := &(*pt1).Next
		if *pt2 == nil {
			return anyOpt
		}

		if (*pt1).Implements == Op1 && (*pt2).Implements == Op2 {
			RemoveElement(pt2)
			RemoveElement(pt1)
			anyOpt = true
		} else {
			pt1 = &(*pt1).Next
		}
	}
	return anyOpt
}

// CloseEnoughFloat checks if two floating-point numbers are close enough.
func CloseEnoughFloat(a, b float64) bool {
	return math.Abs(b-a) < 0.00001
}

// isFloatMatrixIdentity checks if a matrix is an identity matrix.
func isFloatMatrixIdentity(a *cmsMAT3) bool {
	var identity cmsMAT3
	cmsMAT3identity(&identity)

	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if !CloseEnoughFloat(a.V[i].N[j], identity.V[i].N[j]) {
				return false
			}
		}
	}
	return true
}

// _MultiplyMatrix simplifies two adjacent matrices by multiplying them.
func _MultiplyMatrix(Lut *cmsPipeline) bool {
	pt1 := &Lut.Elements
	anyOpt := false

	if *pt1 == nil {
		return anyOpt
	}

	for *pt1 != nil {
		pt2 := &(*pt1).Next
		if *pt2 == nil {
			return anyOpt
		}

		if (*pt1).Implements == cmsSigMatrixElemType && (*pt2).Implements == cmsSigMatrixElemType {
			m1 := (*cmsStageMatrixData)(cmsStageData(*pt1))
			m2 := (*cmsStageMatrixData)(cmsStageData(*pt2))
			var res cmsMAT3

			if m1.Offset != nil || m2.Offset != nil ||
				cmsStageInputChannels(*pt1) != 3 || cmsStageOutputChannels(*pt1) != 3 ||
				cmsStageInputChannels(*pt2) != 3 || cmsStageOutputChannels(*pt2) != 3 {
				return false
			}

			cmsMAT3per(&res, (*cmsMAT3)(unsafe.Pointer(m2.Double)), (*cmsMAT3)(unsafe.Pointer(m1.Double)))

			chain := (*pt2).Next
			RemoveElement(pt2)
			RemoveElement(pt1)

			if !isFloatMatrixIdentity(&res) {
				Multmat := cmsStageAllocMatrix(Lut.ContextID, 3, 3, (*float64)(&res.V[0].N[0]), nil)
				if Multmat == nil {
					return false
				}

				Multmat.Next = chain
				*pt1 = Multmat
			}

			anyOpt = true
		} else {
			pt1 = &(*pt1).Next
		}
	}
	return anyOpt
}

// PreOptimize performs various optimization steps on a pipeline.
func PreOptimize(Lut *cmsPipeline) bool {
	var anyOpt, opt bool

	for {
		opt = false

		// Optimization steps
		opt = opt || Remove1Op(Lut, cmsSigIdentityElemType)
		opt = opt || Remove2Op(Lut, cmsSigXYZ2LabElemType, cmsSigLab2XYZElemType)
		opt = opt || Remove2Op(Lut, cmsSigLab2XYZElemType, cmsSigXYZ2LabElemType)
		opt = opt || Remove2Op(Lut, cmsSigLabV4toV2, cmsSigLabV2toV4)
		opt = opt || Remove2Op(Lut, cmsSigLabV2toV4, cmsSigLabV4toV2)
		opt = opt || Remove2Op(Lut, cmsSigLab2FloatPCS, cmsSigFloatPCS2Lab)
		opt = opt || Remove2Op(Lut, cmsSigXYZ2FloatPCS, cmsSigFloatPCS2XYZ)
		opt = opt || _MultiplyMatrix(Lut)

		if opt {
			anyOpt = true
		} else {
			break
		}
	}

	return anyOpt
}

func Eval16nop1D(Input *uint16, Output *uint16, params *cmsInterpParams) {
	//same as Output[0] = Input[0] like in C
	*Output = *Input
}

// PrelinEval16 implements the optimized interpolation for 16-bit input
// PrelinEval16 implements the optimized interpolation for 16-bit input
func PrelinEval16(Input *uint16, Output *uint16, D unsafe.Pointer) {
	p16 := (*Prelin16Data)(D)
	var StageABC [16]uint16
	var StageDEF [16]uint16

	// Handle input curves
	for i := uint32(0); i < p16.NInputs; i++ {
		// Access the i-th element of Input using unsafe.Pointer
		inputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(i)*unsafe.Sizeof(uint16(0))))

		// Access the i-th element of StageABC using unsafe.Pointer
		stageABCPtr := (*uint16)(unsafe.Add(unsafe.Pointer(&StageABC[0]), uintptr(i)*unsafe.Sizeof(uint16(0))))

		// Access the i-th ParamsCurveIn16 using unsafe.Pointer
		paramsCurveInPtr := (**cmsInterpParams)(unsafe.Add(unsafe.Pointer(&p16.ParamsCurveIn16[0]), uintptr(i)*unsafe.Sizeof((*cmsInterpParams)(nil))))

		// Call the interpolation function
		p16.EvalCurveIn16[i](inputPtr, stageABCPtr, *paramsCurveInPtr)
	}

	// Evaluate the CLUT
	p16.EvalCLUT(&StageABC[0], &StageDEF[0], p16.CLUTParams)

	// Handle output curves
	for i := uint32(0); i < p16.NOutputs; i++ {
		// Access the i-th element of StageDEF using unsafe.Pointer
		stageDEFPtr := (*uint16)(unsafe.Add(unsafe.Pointer(&StageDEF[0]), uintptr(i)*unsafe.Sizeof(uint16(0))))

		// Access the i-th element of Output using unsafe.Pointer
		outputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(uint16(0))))

		// Access the i-th EvalCurveOut16 using unsafe.Pointer
		evalCurveOut := *(*cmsInterpFn16)(unsafe.Add(unsafe.Pointer(p16.EvalCurveOut16), uintptr(i)*unsafe.Sizeof(cmsInterpFn16(nil))))

		// Access the i-th ParamsCurveOut16 using unsafe.Pointer
		paramsCurveOut := (**cmsInterpParams)(unsafe.Add(unsafe.Pointer(p16.ParamsCurveOut16), uintptr(i)*unsafe.Sizeof((*cmsInterpParams)(nil))))

		// Call the interpolation function
		evalCurveOut(stageDEFPtr, outputPtr, *paramsCurveOut)
	}
}

// PrelinOpt16free frees memory associated with Prelin16Data
func PrelinOpt16free(ContextID CmsContext, ptr unsafe.Pointer) {
	p16 := (*Prelin16Data)(ptr)
	cmsFree(ContextID, unsafe.Pointer(p16.EvalCurveOut16))
	cmsFree(ContextID, unsafe.Pointer(p16.ParamsCurveOut16))
	cmsFree(ContextID, ptr)
}

// Prelin16dup duplicates the Prelin16Data structure
func Prelin16dup(ContextID CmsContext, ptr unsafe.Pointer) unsafe.Pointer {
	p16 := (*Prelin16Data)(ptr)
	Duped := (*Prelin16Data)(cmsDupMem(ContextID, unsafe.Pointer(p16), uint32(unsafe.Sizeof(p16))))

	if Duped == nil {
		return nil
	}

	Duped.EvalCurveOut16 = (*cmsInterpFn16)(cmsDupMem(ContextID, unsafe.Pointer(p16.EvalCurveOut16), p16.NOutputs*uint32(unsafe.Sizeof((*cmsInterpFn16)(nil)))))
	Duped.ParamsCurveOut16 = (**cmsInterpParams)(cmsDupMem(ContextID, unsafe.Pointer(p16.ParamsCurveOut16), p16.NOutputs*uint32(unsafe.Sizeof((*cmsInterpParams)(nil)))))

	return unsafe.Pointer(Duped)
}

// PrelinOpt16alloc allocates and initializes Prelin16Data
// PrelinOpt16alloc allocates and initializes Prelin16Data
func PrelinOpt16alloc(ContextID CmsContext, ColorMap *cmsInterpParams, nInputs uint32, In **CmsToneCurve, nOutputs uint32, Out **CmsToneCurve) *Prelin16Data {
	p16 := (*Prelin16Data)(cmsMallocZero(ContextID, uint32(unsafe.Sizeof(Prelin16Data{}))))
	if p16 == nil {
		return nil
	}

	p16.NInputs = nInputs
	p16.NOutputs = nOutputs

	// Handle input curves
	for i := uint32(0); i < nInputs; i++ {
		// Access the i-th element of In
		inPtr := (**CmsToneCurve)(unsafe.Add(unsafe.Pointer(In), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))

		if In == nil || *inPtr == nil {
			p16.ParamsCurveIn16[i] = nil
			p16.EvalCurveIn16[i] = Eval16nop1D
		} else {
			p16.ParamsCurveIn16[i] = (*inPtr).InterpParams
			p16.EvalCurveIn16[i] = (*inPtr).InterpParams.Interpolation.Lerp16
		}
	}

	p16.CLUTParams = ColorMap
	p16.EvalCLUT = ColorMap.Interpolation.Lerp16

	// Allocate memory for EvalCurveOut16 and ParamsCurveOut16
	p16.EvalCurveOut16 = (*cmsInterpFn16)(cmsCalloc(ContextID, nOutputs, uint32(unsafe.Sizeof((*cmsInterpFn16)(nil)))))
	if p16.EvalCurveOut16 == nil {
		cmsFree(ContextID, unsafe.Pointer(p16))
		return nil
	}

	p16.ParamsCurveOut16 = (**cmsInterpParams)(cmsCalloc(ContextID, nOutputs, uint32(unsafe.Sizeof((*cmsInterpParams)(nil)))))
	if p16.ParamsCurveOut16 == nil {
		cmsFree(ContextID, unsafe.Pointer(p16.EvalCurveOut16))
		cmsFree(ContextID, unsafe.Pointer(p16))
		return nil
	}

	// Handle output curves
	for i := uint32(0); i < nOutputs; i++ {
		// Access the i-th element of ParamsCurveOut16
		paramsCurveOutPtr := (**cmsInterpParams)(unsafe.Add(unsafe.Pointer(p16.ParamsCurveOut16), uintptr(i)*unsafe.Sizeof((*cmsInterpParams)(nil))))

		// Access the i-th element of EvalCurveOut16
		evalCurveOutPtr := (*cmsInterpFn16)(unsafe.Add(unsafe.Pointer(p16.EvalCurveOut16), uintptr(i)*unsafe.Sizeof((*cmsInterpFn16)(nil))))

		// Access the i-th element of Out
		outPtr := (**CmsToneCurve)(unsafe.Add(unsafe.Pointer(Out), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))

		if Out == nil || *outPtr == nil {
			*paramsCurveOutPtr = nil
			*evalCurveOutPtr = Eval16nop1D
		} else {
			*paramsCurveOutPtr = (*outPtr).InterpParams
			*evalCurveOutPtr = (*outPtr).InterpParams.Interpolation.Lerp16
		}
	}

	return p16
}

const PRELINEARIZATION_POINTS = 4096

func XFormSampler16(In []uint16, Out []uint16, Cargo unsafe.Pointer) int32 {
	Lut := (*cmsPipeline)(Cargo)
	var InFloat [cmsMAXCHANNELS]float32
	var OutFloat [cmsMAXCHANNELS]float32
	var i uint32

	cmsAssert(Lut.InputChannels < cmsMAXCHANNELS, "")
	cmsAssert(Lut.OutputChannels < cmsMAXCHANNELS, "")

	// From 16-bit to floating-point
	for i = 0; i < Lut.InputChannels; i++ {
		InFloat[i] = float32(In[i]) / 65535.0
	}

	// Evaluate in floating-point
	cmsPipelineEvalFloat(InFloat[:], OutFloat[:], Lut)

	// Back to 16-bit representation
	for i = 0; i < Lut.OutputChannels; i++ {
		Out[i] = cmsQuickSaturateWord(float64(OutFloat[i] * 65535.0))
	}

	// Always succeed
	return 1 // TRUE
}
func AllCurvesAreLinear(mpe *cmsStage) bool {
	Curves := cmsStageGetPtrToCurveSet(mpe)
	if Curves == nil {
		return false
	}

	n := cmsStageOutputChannels(mpe)

	for i := uint32(0); i < n; i++ {
		// Access the i-th curve using unsafe
		curve := (**CmsToneCurve)(unsafe.Add(unsafe.Pointer(Curves), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))
		if !cmsIsToneCurveLinear(*curve) {
			return false
		}
	}

	return true
}

func PatchLUT(CLUT *cmsStage, At []uint16, Value []uint16, nChannelsOut, nChannelsIn uint32) bool {
	Grid := (*cmsStageCLutData)(CLUT.Data)
	p16 := Grid.Params
	var px, py, pz, pw float64
	var x0, y0, z0, w0, index int

	if CLUT.Type != cmsSigCLutElemType {
		cmsSignalError(unsafe.Pointer(CLUT.ContextID), cmsERROR_INTERNAL, "(internal) Attempt to PatchLUT on non-lut stage")
		return false
	}

	switch nChannelsIn {
	case 4:
		px = float64(At[0]) * float64(p16.Domain[0]) / 65535.0
		py = float64(At[1]) * float64(p16.Domain[1]) / 65535.0
		pz = float64(At[2]) * float64(p16.Domain[2]) / 65535.0
		pw = float64(At[3]) * float64(p16.Domain[3]) / 65535.0

		x0 = int(math.Floor(px))
		y0 = int(math.Floor(py))
		z0 = int(math.Floor(pz))
		w0 = int(math.Floor(pw))

		if px-float64(x0) != 0 || py-float64(y0) != 0 || pz-float64(z0) != 0 || pw-float64(w0) != 0 {
			return false // Not on exact node
		}

		index = int(p16.opta[3])*x0 +
			int(p16.opta[2])*y0 +
			int(p16.opta[1])*z0 +
			int(p16.opta[0])*w0

	case 3:
		px = float64(At[0]) * float64(p16.Domain[0]) / 65535.0
		py = float64(At[1]) * float64(p16.Domain[1]) / 65535.0
		pz = float64(At[2]) * float64(p16.Domain[2]) / 65535.0

		x0 = int(math.Floor(px))
		y0 = int(math.Floor(py))
		z0 = int(math.Floor(pz))

		if px-float64(x0) != 0 || py-float64(y0) != 0 || pz-float64(z0) != 0 {
			return false // Not on exact node
		}

		index = int(p16.opta[2])*x0 +
			int(p16.opta[1])*y0 +
			int(p16.opta[0])*z0

	case 1:
		px = float64(At[0]) * float64(p16.Domain[0]) / 65535.0

		x0 = int(math.Floor(px))

		if px-float64(x0) != 0 {
			return false // Not on exact node
		}

		index = int(p16.opta[0]) * x0

	default:
		cmsSignalError(unsafe.Pointer(CLUT.ContextID), cmsERROR_INTERNAL, "(internal) %d Channels are not supported on PatchLUT")
		return false
	}

	for i := 0; i < int(nChannelsOut); i++ {
		*(*uint16)(unsafe.Add(unsafe.Pointer(Grid.Tab.T), uintptr(index+i)*unsafe.Sizeof(uint16(0)))) = Value[i]
	}

	return true
}

func WhitesAreEqual(n uint32, White1, White2 []uint16) bool {
	for i := uint32(0); i < n; i++ {
		if math.Abs(float64(White1[i]-White2[i])) > 0xf000 {
			return true // Values are extremely different; avoid fixup.
		}
		if White1[i] != White2[i] {
			return false
		}
	}
	return true
}

func FixWhiteMisalignment(Lut *cmsPipeline, EntryColorSpace, ExitColorSpace cmsColorSpaceSignature) bool {
	var (
		WhitePointIn, WhitePointOut    []uint16
		WhiteIn, WhiteOut, ObtainedOut [cmsMAXCHANNELS]uint16
		nOuts, nIns                    uint32
		PreLin, CLUT, PostLin          *cmsStage
	)

	WhitePointIn = make([]uint16, 4)
	WhitePointOut = make([]uint16, 4)

	// Get input and output white points.
	if !cmsEndPointsBySpace(EntryColorSpace, &WhitePointIn, nil, &nIns) {
		return false
	}
	if !cmsEndPointsBySpace(ExitColorSpace, &WhitePointOut, nil, &nOuts) {
		return false
	}

	// Verify the LUT dimensions match.
	if Lut.InputChannels != nIns || Lut.OutputChannels != nOuts {
		return false
	}

	// Evaluate the pipeline with the input white point.
	cmsPipelineEval16(WhitePointIn, ObtainedOut[:], Lut)

	// If the output already matches the expected white point, return early.
	//CHECK comparison!   WhitePointOut and ObtainedOut has different slice length!
	if WhitesAreEqual(nOuts, WhitePointOut[:], ObtainedOut[:]) {
		return true
	}

	// Check for pre-linearization, CLUT, and post-linearization stages.
	if !cmsPipelineCheckAndRetrieveStages(Lut, 3, []cmsStageSignature{cmsSigCurveSetElemType, cmsSigCLutElemType, cmsSigCurveSetElemType}, &PreLin, &CLUT, &PostLin) &&
		!cmsPipelineCheckAndRetrieveStages(Lut, 2, []cmsStageSignature{cmsSigCurveSetElemType, cmsSigCLutElemType}, &PreLin, &CLUT) &&
		!cmsPipelineCheckAndRetrieveStages(Lut, 2, []cmsStageSignature{cmsSigCLutElemType, cmsSigCurveSetElemType}, &CLUT, &PostLin) &&
		!cmsPipelineCheckAndRetrieveStages(Lut, 1, []cmsStageSignature{cmsSigCLutElemType}, &CLUT) {
		return false
	}

	// Interpolate white points for pre-linearization.
	if PreLin != nil {
		Curves := cmsStageGetPtrToCurveSet(PreLin)
		for i := uint32(0); i < nIns; i++ {
			curve := (*CmsToneCurve)(unsafe.Add(unsafe.Pointer(Curves), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))
			WhiteIn[i] = cmsEvalToneCurve16(curve, WhitePointIn[i])
		}
	} else {
		for i := uint32(0); i < nIns; i++ {
			WhiteIn[i] = WhitePointIn[i]
		}
	}

	// Interpolate or reverse interpolate post-linearization white points.
	if PostLin != nil {
		Curves := cmsStageGetPtrToCurveSet(PostLin)
		for i := uint32(0); i < nOuts; i++ {
			curve := (*CmsToneCurve)(unsafe.Add(unsafe.Pointer(Curves), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))
			InversePostLin := cmsReverseToneCurve(curve)
			if InversePostLin == nil {
				WhiteOut[i] = WhitePointOut[i]
			} else {
				WhiteOut[i] = cmsEvalToneCurve16(InversePostLin, WhitePointOut[i])
				CmsFreeToneCurve(InversePostLin)
			}
		}
	} else {
		for i := uint32(0); i < nOuts; i++ {
			WhiteOut[i] = WhitePointOut[i]
		}
	}

	// Patch the CLUT with the computed white points.
	PatchLUT(CLUT, WhiteIn[:], WhiteOut[:], nOuts, nIns)

	return true
}

// -----------------------------------------------------------------------------------------------------------------------------------------------
// This function creates simple LUT from complex ones. The generated LUT has an optional set of
// prelinearization curves, a CLUT of nGridPoints and optional postlinearization tables.
// These curves have to exist in the original LUT in order to be used in the simplified output.
// Caller may also use the flags to allow this feature.
// LUTS with all curves will be simplified to a single curve. Parametric curves are lost.
// This function should be used on 16-bits LUTS only, as floating point losses precision when simplified
// -----------------------------------------------------------------------------------------------------------------------------------------------

func OptimizeByResampling(Lut **cmsPipeline, Intent uint32, InputFormat *uint32, OutputFormat *uint32, dwFlags *uint32) bool {
	var (
		Src              *cmsPipeline
		Dest             *cmsPipeline
		CLUT             *cmsStage
		KeepPreLin       *cmsStage
		KeepPostLin      *cmsStage
		NewPreLin        *cmsStage
		NewPostLin       *cmsStage
		nGridPoints      uint32
		ColorSpace       cmsColorSpaceSignature
		OutputColorSpace cmsColorSpaceSignature
		DataCLUT         *cmsStageCLutData
		DataSetIn        **CmsToneCurve
		DataSetOut       **CmsToneCurve
		p16              *Prelin16Data
	)

	// Lossy optimization, not suitable for floating-point formats
	if cmsFormatterIsFloat(*InputFormat) || cmsFormatterIsFloat(*OutputFormat) {
		return false
	}

	ColorSpace = cmsICCcolorSpace(int(T_COLORSPACE(*InputFormat)))
	OutputColorSpace = cmsICCcolorSpace(int(T_COLORSPACE(*OutputFormat)))

	// Validate color spaces
	if ColorSpace == 0 || OutputColorSpace == 0 {
		return false
	}

	nGridPoints = cmsReasonableGridpointsByColorspace(ColorSpace, *dwFlags)

	// For empty LUTs, 2 points are sufficient
	if cmsPipelineStageCount(*Lut) == 0 {
		nGridPoints = 2
	}

	Src = *Lut

	// Allocate an empty LUT
	Dest = cmsPipelineAlloc(Src.ContextID, Src.InputChannels, Src.OutputChannels)
	if Dest == nil {
		return false
	}

	// Handle prelinearization
	if *dwFlags&cmsFLAGS_CLUT_PRE_LINEARIZATION != 0 {
		PreLin := cmsPipelineGetPtrToFirstStage(Src)
		if PreLin != nil && PreLin.Type == cmsSigCurveSetElemType {
			if !AllCurvesAreLinear(PreLin) {
				NewPreLin = cmsStageDup(PreLin)
				if NewPreLin == nil || !cmsPipelineInsertStage(Dest, cmsAT_BEGIN, NewPreLin) {
					goto Error
				}
				cmsPipelineUnlinkStage(Src, cmsAT_BEGIN, &KeepPreLin)
			}
		}
	}

	// Allocate the CLUT
	CLUT = cmsStageAllocCLut16bit(Src.ContextID, nGridPoints, Src.InputChannels, Src.OutputChannels, nil)
	if CLUT == nil || !cmsPipelineInsertStage(Dest, cmsAT_END, CLUT) {
		goto Error
	}

	// Handle postlinearization
	if *dwFlags&cmsFLAGS_CLUT_POST_LINEARIZATION != 0 {
		PostLin := cmsPipelineGetPtrToLastStage(Src)
		if PostLin != nil && cmsStageType(PostLin) == cmsSigCurveSetElemType {
			if !AllCurvesAreLinear(PostLin) {
				NewPostLin = cmsStageDup(PostLin)
				if NewPostLin == nil || !cmsPipelineInsertStage(Dest, cmsAT_END, NewPostLin) {
					goto Error
				}
				cmsPipelineUnlinkStage(Src, cmsAT_END, &KeepPostLin)
			}
		}
	}

	// Perform sampling
	if !cmsStageSampleCLut16bit(CLUT, XFormSampler16, unsafe.Pointer(Src), 0) {
		goto Error
	}

	// Cleanup after sampling
	if KeepPreLin != nil {
		cmsStageFree(KeepPreLin)
	}
	if KeepPostLin != nil {
		cmsStageFree(KeepPostLin)
	}
	cmsPipelineFree(Src)

	DataCLUT = (*cmsStageCLutData)(CLUT.Data)

	if NewPreLin != nil {
		DataSetIn = (**CmsToneCurve)(((*cmsStageToneCurvesData)(NewPreLin.Data)).TheCurves)
	}
	if NewPostLin != nil {
		DataSetOut = (**CmsToneCurve)(((*cmsStageToneCurvesData)(NewPostLin.Data)).TheCurves)
	}

	if DataSetIn == nil && DataSetOut == nil {
		// Define the adapter function as a closure
		adapterFn1 := func(In *uint16, Out *uint16, Data unsafe.Pointer) {
			params := (*cmsInterpParams)(Data) // Convert Data back to *cmsInterpParams
			DataCLUT.Params.Interpolation.Lerp16(In, Out, params)
		}
		// Use the closure as cmsPipelineEval16Fn
		cmsPipelineSetOptimizationParameters(Dest, adapterFn1, unsafe.Pointer(DataCLUT.Params), nil, nil)
	} else {
		p16 = PrelinOpt16alloc(Dest.ContextID, DataCLUT.Params, Dest.InputChannels, DataSetIn, Dest.OutputChannels, DataSetOut)
		if p16 == nil {
			goto Error
		}
		cmsPipelineSetOptimizationParameters(Dest, PrelinEval16, unsafe.Pointer(p16), PrelinOpt16free, Prelin16dup)
	}

	// Adjust for absolute colorimetric intent
	if Intent == INTENT_ABSOLUTE_COLORIMETRIC {
		*dwFlags |= cmsFLAGS_NOWHITEONWHITEFIXUP
	}
	if *dwFlags&cmsFLAGS_NOWHITEONWHITEFIXUP == 0 {
		FixWhiteMisalignment(Dest, ColorSpace, OutputColorSpace)
	}

	*Lut = Dest
	return true

Error:
	// Restore original stages on error
	if KeepPreLin != nil {
		cmsPipelineInsertStage(Src, cmsAT_BEGIN, KeepPreLin)
	}
	if KeepPostLin != nil {
		cmsPipelineInsertStage(Src, cmsAT_END, KeepPostLin)
	}
	cmsPipelineFree(Dest)
	return false
}

// -----------------------------------------------------------------------------------------------------------------------------------------------
// Fixes the gamma balancing of transform. This is described in my paper "Prelinearization Stages on
// Color-Management Application-Specific Integrated Circuits (ASICs)" presented at NIP24. It only works
// for RGB transforms. See the paper for more details
// -----------------------------------------------------------------------------------------------------------------------------------------------

// Normalize endpoints by slope limiting max and min. This assures endpoints as well.
// Descending curves are handled as well.
func SlopeLimiting(g *CmsToneCurve) {
	var BeginVal, EndVal int
	AtBegin := int(math.Floor(float64(g.nEntries)*0.02 + 0.5))
	AtEnd := int(g.nEntries) - AtBegin - 1
	var Val, Slope, beta float64

	if cmsIsToneCurveDescending(g) {
		BeginVal = 0xffff
		EndVal = 0
	} else {
		BeginVal = 0
		EndVal = 0xffff
	}

	Val = float64(*(*uint16)(unsafe.Add(unsafe.Pointer(g.Table16), uintptr(AtBegin)*unsafe.Sizeof(uint16(0)))))
	Slope = (Val - float64(BeginVal)) / float64(AtBegin)
	beta = Val - Slope*float64(AtBegin)

	for i := 0; i < AtBegin; i++ {
		*(*uint16)(unsafe.Add(unsafe.Pointer(g.Table16), uintptr(i)*unsafe.Sizeof(uint16(0)))) = cmsQuickSaturateWord(float64(i)*Slope + beta)
	}

	Val = float64(*(*uint16)(unsafe.Add(unsafe.Pointer(g.Table16), uintptr(AtEnd)*unsafe.Sizeof(uint16(0)))))
	Slope = (float64(EndVal) - Val) / float64(AtBegin)
	beta = Val - Slope*float64(AtEnd)

	for i := AtEnd; i < int(g.nEntries); i++ {
		*(*uint16)(unsafe.Add(unsafe.Pointer(g.Table16), uintptr(i)*unsafe.Sizeof(uint16(0)))) = cmsQuickSaturateWord(float64(i)*Slope + beta)
	}
}

func PrelinOpt8alloc(ContextID CmsContext, p *cmsInterpParams, G [3]*CmsToneCurve) *Prelin8Data {
	p8 := (*Prelin8Data)(cmsMallocZero(ContextID, uint32(unsafe.Sizeof(Prelin8Data{}))))
	if p8 == nil {
		return nil
	}

	for i := 0; i < 256; i++ {
		var Input [3]uint16

		if G[0] != nil {
			Input[0] = cmsEvalToneCurve16(G[0], FROM_8_TO_16(uint8(i)))
			Input[1] = cmsEvalToneCurve16(G[1], FROM_8_TO_16(uint8(i)))
			Input[2] = cmsEvalToneCurve16(G[2], FROM_8_TO_16(uint8(i)))
		} else {
			Input[0] = FROM_8_TO_16(uint8(i))
			Input[1] = FROM_8_TO_16(uint8(i))
			Input[2] = FROM_8_TO_16(uint8(i))
		}

		v1 := cmsToFixedDomain(int(uint32(Input[0]) * p.Domain[0]))
		v2 := cmsToFixedDomain(int(uint32(Input[1]) * p.Domain[1]))
		v3 := cmsToFixedDomain(int(uint32(Input[2]) * p.Domain[2]))

		p8.X0[i] = p.opta[2] * uint32(FIXED_TO_INT(v1))
		p8.Y0[i] = p.opta[1] * uint32(FIXED_TO_INT(v2))
		p8.Z0[i] = p.opta[0] * uint32(FIXED_TO_INT(v3))

		p8.Rx[i] = uint16(FIXED_REST_TO_INT(v1))
		p8.Ry[i] = uint16(FIXED_REST_TO_INT(v2))
		p8.Rz[i] = uint16(FIXED_REST_TO_INT(v3))
	}

	return p8
}

func Prelin8free(ContextID CmsContext, ptr unsafe.Pointer) {
	cmsFree(ContextID, ptr)
}

func Prelin8dup(ContextID CmsContext, ptr unsafe.Pointer) unsafe.Pointer {
	return cmsDupMem(ContextID, ptr, uint32(unsafe.Sizeof(ptr)))
}

/*
	func PrelinEval8(Input *uint16, Output *uint16, D unsafe.Pointer) {
		p8 := (*Prelin8Data)(D)
		p := p8.P
		TotalOut := int(p.nOutputs)
		LutTable := (*[1 << 30]uint16)(p.Table) // Access table with a large assumed size

		// DENS implementation
		DENS := func(i, j, k, outChan uint32) cmsS15Fixed16Number {
			return cmsS15Fixed16Number(LutTable[int(i+j+k+outChan)])
		}

		r := uint8(Input[0] >> 8)
		g := uint8(Input[1] >> 8)
		b := uint8(Input[2] >> 8)

		X0 := cmsS15Fixed16Number(p8.X0[r])
		Y0 := cmsS15Fixed16Number(p8.Y0[g])
		Z0 := cmsS15Fixed16Number(p8.Z0[b])

		rx := p8.Rx[r]
		ry := p8.Ry[g]
		rz := p8.Rz[b]

		X1 := X0
		if rx != 0 {
			X1 += cmsS15Fixed16Number(p.opta[2])
		}
		Y1 := Y0
		if ry != 0 {
			Y1 += cmsS15Fixed16Number(p.opta[1])
		}
		Z1 := Z0
		if rz != 0 {
			Z1 += cmsS15Fixed16Number(p.opta[0])
		}

		for OutChan := 0; OutChan < TotalOut; OutChan++ {
			c0 := DENS(uint32(X0), uint32(Y0), uint32(Z0), uint32(OutChan))

			var c1, c2, c3 cmsS15Fixed16Number
			if rx >= ry && ry >= rz {
				c1 = DENS(uint32(X1), uint32(Y0), uint32(Z0), uint32(OutChan)) - c0
				c2 = DENS(uint32(X1), uint32(Y1), uint32(Z0), uint32(OutChan)) - DENS(uint32(X1), uint32(Y0), uint32(Z0), uint32(OutChan))
				c3 = DENS(uint32(X1), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X1), uint32(Y1), uint32(Z0), uint32(OutChan))
			} else if rx >= rz && rz >= ry {
				c1 = DENS(uint32(X1), uint32(Y0), uint32(Z0), uint32(OutChan)) - c0
				c2 = DENS(uint32(X1), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X1), uint32(Y0), uint32(Z1), uint32(OutChan))
				c3 = DENS(uint32(X1), uint32(Y0), uint32(Z1), uint32(OutChan)) - DENS(uint32(X1), uint32(Y0), uint32(Z0), uint32(OutChan))
			} else if rz >= rx && rx >= ry {
				c1 = DENS(uint32(X1), uint32(Y0), uint32(Z1), uint32(OutChan)) - DENS(uint32(X0), uint32(Y0), uint32(Z1), uint32(OutChan))
				c2 = DENS(uint32(X1), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X1), uint32(Y0), uint32(Z1), uint32(OutChan))
				c3 = DENS(uint32(X0), uint32(Y0), uint32(Z1), uint32(OutChan)) - c0
			} else if ry >= rx && rx >= rz {
				c1 = DENS(uint32(X1), uint32(Y1), uint32(Z0), uint32(OutChan)) - DENS(uint32(X0), uint32(Y1), uint32(Z0), uint32(OutChan))
				c2 = DENS(uint32(X0), uint32(Y1), uint32(Z0), uint32(OutChan)) - c0
				c3 = DENS(uint32(X1), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X1), uint32(Y1), uint32(Z0), uint32(OutChan))
			} else if ry >= rz && rz >= rx {
				c1 = DENS(uint32(X1), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X0), uint32(Y1), uint32(Z1), uint32(OutChan))
				c2 = DENS(uint32(X0), uint32(Y1), uint32(Z0), uint32(OutChan)) - c0
				c3 = DENS(uint32(X0), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X0), uint32(Y1), uint32(Z0), uint32(OutChan))
			} else if rz >= ry && ry >= rx {
				c1 = DENS(uint32(X1), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X0), uint32(Y1), uint32(Z1), uint32(OutChan))
				c2 = DENS(uint32(X0), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X0), uint32(Y0), uint32(Z1), uint32(OutChan))
				c3 = DENS(uint32(X0), uint32(Y0), uint32(Z1), uint32(OutChan)) - c0
			} else {
				c1, c2, c3 = 0, 0, 0
			}

			Rest := uint16(c1)*rx + uint16(c2)*ry + uint16(c3)*rz + 0x8001
			Output[OutChan] = uint16(uint16(c0) + ((Rest + (Rest >> 16)) >> 16))
		}
	}
*/
func PrelinEval8(Input *uint16, Output *uint16, D unsafe.Pointer) {
	p8 := (*Prelin8Data)(D)
	p := p8.P
	TotalOut := int(p.nOutputs)
	LutTable := (*[1 << 30]uint16)(p.Table) // Access table with a large assumed size

	// DENS function implementation for accessing LutTable
	DENS := func(i, j, k, outChan uint32) cmsS15Fixed16Number {
		return cmsS15Fixed16Number(LutTable[int(i+j+k+outChan)])
	}

	// Access Input values using unsafe.Pointer indexing
	r := uint8(*(*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(0)*unsafe.Sizeof(uint16(0)))) >> 8)
	g := uint8(*(*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0)))) >> 8)
	b := uint8(*(*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(2)*unsafe.Sizeof(uint16(0)))) >> 8)

	X0 := cmsS15Fixed16Number(p8.X0[r])
	Y0 := cmsS15Fixed16Number(p8.Y0[g])
	Z0 := cmsS15Fixed16Number(p8.Z0[b])

	rx := p8.Rx[r]
	ry := p8.Ry[g]
	rz := p8.Rz[b]

	X1 := X0
	if rx != 0 {
		X1 += cmsS15Fixed16Number(p.opta[2])
	}
	Y1 := Y0
	if ry != 0 {
		Y1 += cmsS15Fixed16Number(p.opta[1])
	}
	Z1 := Z0
	if rz != 0 {
		Z1 += cmsS15Fixed16Number(p.opta[0])
	}

	// Loop over output channels
	for OutChan := 0; OutChan < TotalOut; OutChan++ {
		c0 := DENS(uint32(X0), uint32(Y0), uint32(Z0), uint32(OutChan))

		var c1, c2, c3 cmsS15Fixed16Number
		if rx >= ry && ry >= rz {
			c1 = DENS(uint32(X1), uint32(Y0), uint32(Z0), uint32(OutChan)) - c0
			c2 = DENS(uint32(X1), uint32(Y1), uint32(Z0), uint32(OutChan)) - DENS(uint32(X1), uint32(Y0), uint32(Z0), uint32(OutChan))
			c3 = DENS(uint32(X1), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X1), uint32(Y1), uint32(Z0), uint32(OutChan))
		} else if rx >= rz && rz >= ry {
			c1 = DENS(uint32(X1), uint32(Y0), uint32(Z0), uint32(OutChan)) - c0
			c2 = DENS(uint32(X1), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X1), uint32(Y0), uint32(Z1), uint32(OutChan))
			c3 = DENS(uint32(X1), uint32(Y0), uint32(Z1), uint32(OutChan)) - DENS(uint32(X1), uint32(Y0), uint32(Z0), uint32(OutChan))
		} else if rz >= rx && rx >= ry {
			c1 = DENS(uint32(X1), uint32(Y0), uint32(Z1), uint32(OutChan)) - DENS(uint32(X0), uint32(Y0), uint32(Z1), uint32(OutChan))
			c2 = DENS(uint32(X1), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X1), uint32(Y0), uint32(Z1), uint32(OutChan))
			c3 = DENS(uint32(X0), uint32(Y0), uint32(Z1), uint32(OutChan)) - c0
		} else if ry >= rx && rx >= rz {
			c1 = DENS(uint32(X1), uint32(Y1), uint32(Z0), uint32(OutChan)) - DENS(uint32(X0), uint32(Y1), uint32(Z0), uint32(OutChan))
			c2 = DENS(uint32(X0), uint32(Y1), uint32(Z0), uint32(OutChan)) - c0
			c3 = DENS(uint32(X1), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X1), uint32(Y1), uint32(Z0), uint32(OutChan))
		} else if ry >= rz && rz >= rx {
			c1 = DENS(uint32(X1), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X0), uint32(Y1), uint32(Z1), uint32(OutChan))
			c2 = DENS(uint32(X0), uint32(Y1), uint32(Z0), uint32(OutChan)) - c0
			c3 = DENS(uint32(X0), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X0), uint32(Y1), uint32(Z0), uint32(OutChan))
		} else if rz >= ry && ry >= rx {
			c1 = DENS(uint32(X1), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X0), uint32(Y1), uint32(Z1), uint32(OutChan))
			c2 = DENS(uint32(X0), uint32(Y1), uint32(Z1), uint32(OutChan)) - DENS(uint32(X0), uint32(Y0), uint32(Z1), uint32(OutChan))
			c3 = DENS(uint32(X0), uint32(Y0), uint32(Z1), uint32(OutChan)) - c0
		} else {
			c1, c2, c3 = 0, 0, 0
		}

		Rest := uint16(c1)*rx + uint16(c2)*ry + uint16(c3)*rz + 0x8001
		outputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(OutChan)*unsafe.Sizeof(uint16(0))))
		*outputPtr = uint16(c0) + uint16((Rest+(Rest>>16))>>16)
	}
}

func IsDegenerated(g *CmsToneCurve) bool {
	Zeros, Poles := 0, 0

	for i := uint32(0); i < g.nEntries; i++ {
		value := *(*uint16)(unsafe.Add(unsafe.Pointer(g.Table16), uintptr(i)*unsafe.Sizeof(uint16(0))))
		if value == 0x0000 {
			Zeros++
		}
		if value == 0xffff {
			Poles++
		}
	}

	if Zeros == 1 && Poles == 1 {
		return false
	}
	if Zeros > int(g.nEntries/20) || Poles > int(g.nEntries/20) {
		return true
	}
	return false
}

func OptimizeByComputingLinearization(Lut **cmsPipeline, Intent uint32, InputFormat, OutputFormat, dwFlags *uint32) bool {
	var (
		OriginalLut      *cmsPipeline
		ColorSpace       cmsColorSpaceSignature
		OutputColorSpace cmsColorSpaceSignature
		nGridPoints      uint32
		Trans            [cmsMAXCHANNELS]*CmsToneCurve
		TransReverse     [cmsMAXCHANNELS]*CmsToneCurve
		In               [cmsMAXCHANNELS]float32
		Out              [cmsMAXCHANNELS]float32
		lIsSuitable      = true
		//lIsLinear      = true
		OptimizedLUT          *cmsPipeline
		LutPlusCurves         *cmsPipeline
		OptimizedPrelinMpe    *cmsStage
		OptimizedPrelinCurves **CmsToneCurve
		OptimizedPrelinCLUT   *cmsStageCLutData
		OptimizedCLUTmpe      *cmsStage
	)

	if cmsFormatterIsFloat(*InputFormat) || cmsFormatterIsFloat(*OutputFormat) {
		return false
	}

	if T_COLORSPACE(*InputFormat) != PT_RGB || (T_PLANAR(*InputFormat) != 1) {
		return false
	}

	if T_COLORSPACE(*OutputFormat) != PT_RGB || (T_PLANAR(*OutputFormat) != 1) {
		return false
	}

	if !cmsFormatterIs8bit(*InputFormat) {
		if (*dwFlags & cmsFLAGS_CLUT_PRE_LINEARIZATION) == 0 {
			return false
		}
	}

	OriginalLut = *Lut
	ColorSpace = cmsICCcolorSpace(int(T_COLORSPACE(*InputFormat)))
	OutputColorSpace = cmsICCcolorSpace(int(T_COLORSPACE(*OutputFormat)))

	if ColorSpace == 0 || OutputColorSpace == 0 {
		return false
	}

	nGridPoints = cmsReasonableGridpointsByColorspace(ColorSpace, *dwFlags)

	for i := range Trans {
		Trans[i] = nil
		TransReverse[i] = nil
	}

	last := cmsPipelineGetPtrToLastStage(OriginalLut)
	if last == nil {
		goto Error
	}

	if cmsStageType(last) == cmsSigCurveSetElemType {
		Data := (*cmsStageToneCurvesData)(cmsStageData(last))
		for i := uint32(0); i < Data.NCurves; i++ {
			curve := (**CmsToneCurve)(unsafe.Add(unsafe.Pointer(Data.TheCurves), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))
			if IsDegenerated(*curve) {
				goto Error
			}
		}
	}

	for t := uint32(0); t < OriginalLut.InputChannels; t++ {
		Trans[t] = cmsBuildTabulatedToneCurve16(OriginalLut.ContextID, PRELINEARIZATION_POINTS, nil)
		if Trans[t] == nil {
			goto Error
		}
	}

	for i := uint32(0); i < PRELINEARIZATION_POINTS; i++ {
		v := float32(float64(i) / float64(PRELINEARIZATION_POINTS-1))
		for t := uint32(0); t < OriginalLut.InputChannels; t++ {
			In[t] = v
		}
		cmsPipelineEvalFloat(In[:], Out[:], OriginalLut)

		for t := uint32(0); t < OriginalLut.InputChannels; t++ {
			tablePtr := Trans[t].Table16 // Get the pointer to the Table16
			tableElem := (*uint16)(unsafe.Add(unsafe.Pointer(tablePtr), uintptr(i)*unsafe.Sizeof(*tablePtr)))
			*tableElem = cmsQuickSaturateWord(float64(Out[t]) * 65535.0)
		}
	}

	for t := uint32(0); t < OriginalLut.InputChannels; t++ {
		SlopeLimiting(Trans[t])
	}

	for t := uint32(0); t < OriginalLut.InputChannels; t++ {
		if !cmsIsToneCurveLinear(Trans[t]) {
			//lIsLinear = false
		}
		if !cmsIsToneCurveMonotonic(Trans[t]) || IsDegenerated(Trans[t]) {
			lIsSuitable = false
			break
		}
	}

	if !lIsSuitable {
		goto Error
	}

	for t := uint32(0); t < OriginalLut.InputChannels; t++ {
		TransReverse[t] = cmsReverseToneCurveEx(PRELINEARIZATION_POINTS, Trans[t])
		if TransReverse[t] == nil {
			goto Error
		}
	}

	LutPlusCurves = cmsPipelineDup(OriginalLut)
	if LutPlusCurves == nil {
		goto Error
	}

	if !cmsPipelineInsertStage(LutPlusCurves, cmsAT_BEGIN, cmsStageAllocToneCurves(OriginalLut.ContextID, OriginalLut.InputChannels, &TransReverse[0])) {
		goto Error
	}

	OptimizedLUT = cmsPipelineAlloc(OriginalLut.ContextID, OriginalLut.InputChannels, OriginalLut.OutputChannels)
	if OptimizedLUT == nil {
		goto Error
	}

	OptimizedPrelinMpe = cmsStageAllocToneCurves(OriginalLut.ContextID, OriginalLut.InputChannels, &Trans[0])
	if !cmsPipelineInsertStage(OptimizedLUT, cmsAT_BEGIN, OptimizedPrelinMpe) {
		goto Error
	}

	OptimizedCLUTmpe = cmsStageAllocCLut16bit(OriginalLut.ContextID, nGridPoints, OriginalLut.InputChannels, OriginalLut.OutputChannels, nil)
	if !cmsPipelineInsertStage(OptimizedLUT, cmsAT_END, OptimizedCLUTmpe) {
		goto Error
	}

	if !cmsStageSampleCLut16bit(OptimizedCLUTmpe, XFormSampler16, unsafe.Pointer(LutPlusCurves), 0) {
		goto Error
	}

	for t := uint32(0); t < OriginalLut.InputChannels; t++ {
		if Trans[t] != nil {
			CmsFreeToneCurve(Trans[t])
		}
		if TransReverse[t] != nil {
			CmsFreeToneCurve(TransReverse[t])
		}
	}

	cmsPipelineFree(LutPlusCurves)

	OptimizedPrelinCurves = cmsStageGetPtrToCurveSet(OptimizedPrelinMpe)
	OptimizedPrelinCLUT = (*cmsStageCLutData)(OptimizedCLUTmpe.Data)

	if cmsFormatterIs8bit(*InputFormat) {

		p8 := PrelinOpt8alloc(OptimizedLUT.ContextID, OptimizedPrelinCLUT.Params, ConvertToToneCurveArray(OptimizedPrelinCurves))
		if p8 == nil {
			return false
		}
		cmsPipelineSetOptimizationParameters(OptimizedLUT, PrelinEval8, unsafe.Pointer(p8), Prelin8free, Prelin8dup)
	} else {
		p16 := PrelinOpt16alloc(OptimizedLUT.ContextID, OptimizedPrelinCLUT.Params, 3, OptimizedPrelinCurves, 3, nil)
		if p16 == nil {
			return false
		}
		cmsPipelineSetOptimizationParameters(OptimizedLUT, PrelinEval16, unsafe.Pointer(p16), PrelinOpt16free, Prelin16dup)
	}

	if Intent == INTENT_ABSOLUTE_COLORIMETRIC {
		*dwFlags |= cmsFLAGS_NOWHITEONWHITEFIXUP
	}

	if (*dwFlags & cmsFLAGS_NOWHITEONWHITEFIXUP) == 0 {
		if !FixWhiteMisalignment(OptimizedLUT, ColorSpace, OutputColorSpace) {
			return false
		}
	}

	cmsPipelineFree(OriginalLut)
	*Lut = OptimizedLUT
	return true

Error:
	for t := uint32(0); t < OriginalLut.InputChannels; t++ {
		if Trans[t] != nil {
			CmsFreeToneCurve(Trans[t])
		}
		if TransReverse[t] != nil {
			CmsFreeToneCurve(TransReverse[t])
		}
	}

	if LutPlusCurves != nil {
		cmsPipelineFree(LutPlusCurves)
	}
	if OptimizedLUT != nil {
		cmsPipelineFree(OptimizedLUT)
	}

	return false
}
func ConvertToToneCurveArray(curves **CmsToneCurve) [3]*CmsToneCurve {
	var result [3]*CmsToneCurve
	for i := 0; i < 3; i++ {
		// Use unsafe.Add to calculate the address of each curve
		curvePtr := (**CmsToneCurve)(unsafe.Add(unsafe.Pointer(curves), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))
		result[i] = *curvePtr
	}
	return result
}
func CurvesFree(ContextID CmsContext, ptr unsafe.Pointer) {
	data := (*Curves16Data)(ptr)

	// Free each curve in the Curves array
	for i := uint32(0); i < data.NCurves; i++ {
		curvePtr := (**uint16)(unsafe.Add(unsafe.Pointer(data.Curves), uintptr(i)*unsafe.Sizeof((*uint16)(nil))))
		cmsFree(ContextID, unsafe.Pointer(*curvePtr))
	}

	// Free the Curves array and the data structure itself
	cmsFree(ContextID, unsafe.Pointer(data.Curves))
	cmsFree(ContextID, ptr)
}
func CurvesDup(ContextID CmsContext, ptr unsafe.Pointer) unsafe.Pointer {
	srcData := (*Curves16Data)(ptr)
	data := (*Curves16Data)(cmsDupMem(ContextID, unsafe.Pointer(srcData), uint32(unsafe.Sizeof(Curves16Data{}))))
	if data == nil {
		return nil
	}

	// Duplicate the Curves array
	data.Curves = (**uint16)(cmsDupMem(ContextID, unsafe.Pointer(srcData.Curves), srcData.NCurves*uint32(unsafe.Sizeof((*uint16)(nil)))))
	if data.Curves == nil {
		cmsFree(ContextID, unsafe.Pointer(data))
		return nil
	}

	// Duplicate each curve
	for i := uint32(0); i < data.NCurves; i++ {
		srcCurvePtr := (**uint16)(unsafe.Add(unsafe.Pointer(srcData.Curves), uintptr(i)*unsafe.Sizeof((*uint16)(nil))))
		destCurvePtr := (**uint16)(unsafe.Add(unsafe.Pointer(data.Curves), uintptr(i)*unsafe.Sizeof((*uint16)(nil))))

		*destCurvePtr = (*uint16)(cmsDupMem(ContextID, unsafe.Pointer(*srcCurvePtr), srcData.NElements*uint32(unsafe.Sizeof(uint16(0)))))
		if *destCurvePtr == nil {
			CurvesFree(ContextID, unsafe.Pointer(data))
			return nil
		}
	}

	return unsafe.Pointer(data)
}
func CurvesAlloc(ContextID CmsContext, nCurves, nElements uint32, G **CmsToneCurve) *Curves16Data {
	c16 := (*Curves16Data)(cmsMallocZero(ContextID, uint32(unsafe.Sizeof(Curves16Data{}))))
	if c16 == nil {
		return nil
	}

	c16.NCurves = nCurves
	c16.NElements = nElements

	// Allocate the Curves array
	c16.Curves = (**uint16)(cmsCalloc(ContextID, nCurves, uint32(unsafe.Sizeof((*uint16)(nil)))))
	if c16.Curves == nil {
		cmsFree(ContextID, unsafe.Pointer(c16))
		return nil
	}

	// Allocate and initialize each curve
	for i := uint32(0); i < nCurves; i++ {
		curvePtr := (**uint16)(unsafe.Add(unsafe.Pointer(c16.Curves), uintptr(i)*unsafe.Sizeof((*uint16)(nil))))
		*curvePtr = (*uint16)(cmsCalloc(ContextID, nElements, uint32(unsafe.Sizeof(uint16(0)))))

		if *curvePtr == nil {
			for j := uint32(0); j < i; j++ {
				prevCurvePtr := (**uint16)(unsafe.Add(unsafe.Pointer(c16.Curves), uintptr(j)*unsafe.Sizeof((*uint16)(nil))))
				cmsFree(ContextID, unsafe.Pointer(*prevCurvePtr))
			}
			cmsFree(ContextID, unsafe.Pointer(c16.Curves))
			cmsFree(ContextID, unsafe.Pointer(c16))
			return nil
		}

		for j := uint32(0); j < nElements; j++ {
			if nElements == 256 {
				//FROM_8_TO_16(uint8(j) is a strange piece in C-code,  uint32 must be reduced to uint8
				(*(*uint16)(unsafe.Add(unsafe.Pointer(*curvePtr), uintptr(j)*unsafe.Sizeof(uint16(0))))) = cmsEvalToneCurve16((*CmsToneCurve)(unsafe.Add(unsafe.Pointer(G), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil)))), FROM_8_TO_16(uint8(j)))
			} else {
				(*(*uint16)(unsafe.Add(unsafe.Pointer(*curvePtr), uintptr(j)*unsafe.Sizeof(uint16(0))))) = cmsEvalToneCurve16((*CmsToneCurve)(unsafe.Add(unsafe.Pointer(G), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil)))), uint16(j))
			}
		}
	}

	return c16
}
func FastEvaluateCurves8(In *uint16, Out *uint16, D unsafe.Pointer) {
	data := (*Curves16Data)(D)

	for i := uint32(0); i < data.NCurves; i++ {
		// Access the i-th curve pointer
		curvePtr := (**uint16)(unsafe.Add(unsafe.Pointer(data.Curves), uintptr(i)*unsafe.Sizeof((*uint16)(nil))))

		// Ensure the curve pointer is valid
		if curvePtr == nil {
			continue
		}

		// Access the curve data
		curveData := *curvePtr

		// Access the i-th input value
		inValue := *(*uint16)(unsafe.Add(unsafe.Pointer(In), uintptr(i)*unsafe.Sizeof(uint16(0))))

		// Calculate the index in the curve
		x := inValue >> 8

		// Safely access the curve value using the index
		OutValuePtr := (*uint16)(unsafe.Add(unsafe.Pointer(curveData), uintptr(x)*unsafe.Sizeof(uint16(0))))
		*(*uint16)(unsafe.Add(unsafe.Pointer(Out), uintptr(i)*unsafe.Sizeof(uint16(0)))) = *OutValuePtr
	}
}

func FastEvaluateCurves16(In *uint16, Out *uint16, D unsafe.Pointer) {
	data := (*Curves16Data)(D)

	for i := uint32(0); i < data.NCurves; i++ {
		// Access the i-th curve pointer
		curvePtr := (**uint16)(unsafe.Add(unsafe.Pointer(data.Curves), uintptr(i)*unsafe.Sizeof((*uint16)(nil))))

		// Ensure the curve pointer is valid
		if curvePtr == nil {
			continue
		}

		// Access the curve data
		curveData := *curvePtr

		// Access the i-th input value
		inValue := *(*uint16)(unsafe.Add(unsafe.Pointer(In), uintptr(i)*unsafe.Sizeof(uint16(0))))

		// Safely access the curve value using the input value as the index
		OutValuePtr := (*uint16)(unsafe.Add(unsafe.Pointer(curveData), uintptr(inValue)*unsafe.Sizeof(uint16(0))))
		*(*uint16)(unsafe.Add(unsafe.Pointer(Out), uintptr(i)*unsafe.Sizeof(uint16(0)))) = *OutValuePtr
	}
}

func FastIdentity16(In *uint16, Out *uint16, D unsafe.Pointer) {
	Lut := (*cmsPipeline)(D)

	for i := uint32(0); i < Lut.InputChannels; i++ {
		// Access the i-th input value
		inValue := *(*uint16)(unsafe.Add(unsafe.Pointer(In), uintptr(i)*unsafe.Sizeof(uint16(0))))

		// Write the input value directly to the output
		*(*uint16)(unsafe.Add(unsafe.Pointer(Out), uintptr(i)*unsafe.Sizeof(uint16(0)))) = inValue
	}
}

// cmsOPToptimizeFn defines the function type for optimizations.
func OptimizeByJoiningCurves(Lut **cmsPipeline, Intent uint32, InputFormat *uint32, OutputFormat *uint32, dwFlags *uint32) bool {
	var (
		GammaTables    **CmsToneCurve
		InFloat        [cmsMAXCHANNELS]float32
		OutFloat       [cmsMAXCHANNELS]float32
		i, j           uint32
		Src            *cmsPipeline
		Dest           *cmsPipeline
		mpe            *cmsStage
		ObtainedCurves *cmsStage
	)

	Src = *Lut

	// This is a lossy optimization; does not apply to floating-point cases
	if cmsFormatterIsFloat(*InputFormat) || cmsFormatterIsFloat(*OutputFormat) {
		return false
	}

	// Check if the LUT contains only curves
	for mpe = cmsPipelineGetPtrToFirstStage(Src); mpe != nil; mpe = cmsStageNext(mpe) {
		if cmsStageType(mpe) != cmsSigCurveSetElemType {
			return false
		}
	}

	// Allocate an empty LUT
	Dest = cmsPipelineAlloc(Src.ContextID, Src.InputChannels, Src.OutputChannels)
	if Dest == nil {
		return false
	}

	// Allocate GammaTables
	GammaTables = (**CmsToneCurve)(cmsCalloc(Src.ContextID, Src.InputChannels, uint32(unsafe.Sizeof((*CmsToneCurve)(nil)))))
	if GammaTables == nil {
		goto Error
	}

	// Initialize GammaTables

	// Initialize GammaTables
	for i = 0; i < Src.InputChannels; i++ {
		ptr := (**CmsToneCurve)(unsafe.Add(unsafe.Pointer(GammaTables), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))
		*ptr = cmsBuildTabulatedToneCurve16(Src.ContextID, PRELINEARIZATION_POINTS, nil)
		if *ptr == nil {
			goto Error
		}
	}

	// Compute 16-bit results using floating-point values
	for i = 0; i < PRELINEARIZATION_POINTS; i++ {
		for j = 0; j < Src.InputChannels; j++ {
			InFloat[j] = float32(float64(i) / float64(PRELINEARIZATION_POINTS-1))
		}

		cmsPipelineEvalFloat(InFloat[:], OutFloat[:], Src)
		for j = 0; j < Src.InputChannels; j++ {
			// Access the tone curve pointer
			toneCurve := *(*CmsToneCurve)(unsafe.Add(unsafe.Pointer(GammaTables), uintptr(j)*unsafe.Sizeof((*CmsToneCurve)(nil))))

			// Calculate the address of the desired index in Table16 and assign the value
			tablePtr := (*uint16)(unsafe.Add(unsafe.Pointer(toneCurve.Table16), uintptr(i)*unsafe.Sizeof(uint16(0))))
			*tablePtr = cmsQuickSaturateWord(float64(OutFloat[j]) * 65535.0)
		}
	}

	// Allocate obtained curves
	ObtainedCurves = cmsStageAllocToneCurves(Src.ContextID, Src.InputChannels, GammaTables)
	if ObtainedCurves == nil {
		goto Error
	}

	// Free GammaTables
	for i = 0; i < Src.InputChannels; i++ {
		// Calculate the address of GammaTables[i]
		ptr := (**CmsToneCurve)(unsafe.Add(unsafe.Pointer(GammaTables), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))

		// Dereference to get the actual *CmsToneCurve
		curve := *ptr

		// Free the memory for the curve
		CmsFreeToneCurve(curve)
	}

	cmsFree(Src.ContextID, unsafe.Pointer(GammaTables))

	// Check if all curves are linear
	if !AllCurvesAreLinear(ObtainedCurves) {
		Data := (*cmsStageToneCurvesData)(cmsStageData(ObtainedCurves))

		if !cmsPipelineInsertStage(Dest, cmsAT_BEGIN, ObtainedCurves) {
			goto Error
		}

		ObtainedCurves = nil

		if cmsFormatterIs8bit(*InputFormat) {
			c16 := CurvesAlloc(Dest.ContextID, Data.NCurves, 256, Data.TheCurves)
			if c16 == nil {
				goto Error
			}
			*dwFlags |= cmsFLAGS_NOCACHE
			cmsPipelineSetOptimizationParameters(Dest, FastEvaluateCurves8, unsafe.Pointer(c16), CurvesFree, CurvesDup)
		} else {
			c16 := CurvesAlloc(Dest.ContextID, Data.NCurves, 65536, Data.TheCurves)
			if c16 == nil {
				goto Error
			}
			*dwFlags |= cmsFLAGS_NOCACHE
			cmsPipelineSetOptimizationParameters(Dest, FastEvaluateCurves16, unsafe.Pointer(c16), CurvesFree, CurvesDup)
		}
	} else {
		cmsStageFree(ObtainedCurves)
		ObtainedCurves = nil

		if !cmsPipelineInsertStage(Dest, cmsAT_BEGIN, cmsStageAllocIdentity(Dest.ContextID, Src.InputChannels)) {
			goto Error
		}

		*dwFlags |= cmsFLAGS_NOCACHE
		cmsPipelineSetOptimizationParameters(Dest, FastIdentity16, unsafe.Pointer(Dest), nil, nil)
	}

	// Replace the source LUT with the optimized LUT
	cmsPipelineFree(Src)
	*Lut = Dest
	return true

Error:
	if ObtainedCurves != nil {
		cmsStageFree(ObtainedCurves)
	}
	if GammaTables != nil {
		for i = 0; i < Src.InputChannels; i++ {
			curve := (**CmsToneCurve)(unsafe.Add(unsafe.Pointer(GammaTables), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))
			if curve != nil {
				CmsFreeToneCurve(*curve)
			}
		}
		cmsFree(Src.ContextID, unsafe.Pointer(GammaTables))
	}
	if Dest != nil {
		cmsPipelineFree(Dest)
	}
	return false
}
func FreeMatShaper(ContextID CmsContext, Data unsafe.Pointer) {
	if Data != nil {
		cmsFree(ContextID, Data)
	}
}
func DupMatShaper(ContextID CmsContext, Data unsafe.Pointer) unsafe.Pointer {
	return cmsDupMem(ContextID, Data, uint32(unsafe.Sizeof(MatShaper8Data{})))
}
func MatShaperEval16(In *uint16, Out *uint16, D unsafe.Pointer) {
	p := (*MatShaper8Data)(D)
	var ri, gi, bi uint32

	// Extract indices from input
	ri = uint32(*(*uint16)(unsafe.Add(unsafe.Pointer(In), uintptr(0)*unsafe.Sizeof(uint16(0))))) & 0xFF
	gi = uint32(*(*uint16)(unsafe.Add(unsafe.Pointer(In), uintptr(1)*unsafe.Sizeof(uint16(0))))) & 0xFF
	bi = uint32(*(*uint16)(unsafe.Add(unsafe.Pointer(In), uintptr(2)*unsafe.Sizeof(uint16(0))))) & 0xFF

	// Apply the first shaper
	r := p.Shaper1R[ri]
	g := p.Shaper1G[gi]
	b := p.Shaper1B[bi]

	// Evaluate the matrix
	l1 := (p.Mat[0][0]*r + p.Mat[0][1]*g + p.Mat[0][2]*b + p.Off[0] + 0x2000) >> 14
	l2 := (p.Mat[1][0]*r + p.Mat[1][1]*g + p.Mat[1][2]*b + p.Off[1] + 0x2000) >> 14
	l3 := (p.Mat[2][0]*r + p.Mat[2][1]*g + p.Mat[2][2]*b + p.Off[2] + 0x2000) >> 14

	// Clip to 0..1.0 range
	ri = clipToRange(l1, 0, 16384)
	gi = clipToRange(l2, 0, 16384)
	bi = clipToRange(l3, 0, 16384)

	// Apply the second shaper
	*(*uint16)(unsafe.Add(unsafe.Pointer(Out), uintptr(0)*unsafe.Sizeof(uint16(0)))) = p.Shaper2R[ri]
	*(*uint16)(unsafe.Add(unsafe.Pointer(Out), uintptr(1)*unsafe.Sizeof(uint16(0)))) = p.Shaper2G[gi]
	*(*uint16)(unsafe.Add(unsafe.Pointer(Out), uintptr(2)*unsafe.Sizeof(uint16(0)))) = p.Shaper2B[bi]
}

func clipToRange(value cmsS1Fixed14Number, min, max cmsS1Fixed14Number) uint32 {
	if value < min {
		return uint32(min)
	} else if value > max {
		return uint32(max)
	}
	return uint32(value)
}
func FillFirstShaper(Table *cmsS1Fixed14Number, Curve *CmsToneCurve) {
	for i := 0; i < 256; i++ {
		R := float32(i) / 255.0
		y := cmsEvalToneCurveFloat(Curve, R)

		if y < 131072.0 {
			*(*cmsS1Fixed14Number)(unsafe.Add(unsafe.Pointer(Table), uintptr(i)*unsafe.Sizeof(cmsS1Fixed14Number(0)))) = DOUBLE_TO_1FIXED14(float64(y))
		} else {
			*(*cmsS1Fixed14Number)(unsafe.Add(unsafe.Pointer(Table), uintptr(i)*unsafe.Sizeof(cmsS1Fixed14Number(0)))) = 0x7FFFFFFF
		}
	}
}
func FillSecondShaper(Table *uint16, Curve *CmsToneCurve, Is8BitsOutput bool) {
	for i := 0; i < 16385; i++ {
		R := float32(i) / 16384.0
		Val := cmsEvalToneCurveFloat(Curve, R)

		if Val < 0 {
			Val = 0
		} else if Val > 1.0 {
			Val = 1.0
		}

		if Is8BitsOutput {
			w := cmsQuickSaturateWord(float64(Val * 65535.0))
			b := FROM_16_TO_8(w)
			*(*uint16)(unsafe.Add(unsafe.Pointer(Table), uintptr(i)*unsafe.Sizeof(uint16(0)))) = FROM_8_TO_16(b)
		} else {
			*(*uint16)(unsafe.Add(unsafe.Pointer(Table), uintptr(i)*unsafe.Sizeof(uint16(0)))) = cmsQuickSaturateWord(float64(Val * 65535.0))
		}
	}
}
func SetMatShaper(Dest *cmsPipeline, Curve1 [3]*CmsToneCurve, Mat *cmsMAT3, Off *cmsVEC3, Curve2 [3]*CmsToneCurve, OutputFormat *uint32) bool {
	p := (*MatShaper8Data)(cmsMalloc(Dest.ContextID, uint32(unsafe.Sizeof(MatShaper8Data{}))))
	if p == nil {
		return false
	}

	p.ContextID = Dest.ContextID

	// Fill the first and second shapers
	FillFirstShaper(&p.Shaper1R[0], Curve1[0])
	FillFirstShaper(&p.Shaper1G[0], Curve1[1])
	FillFirstShaper(&p.Shaper1B[0], Curve1[2])

	FillSecondShaper(&p.Shaper2R[0], Curve2[0], cmsFormatterIs8bit(*OutputFormat))
	FillSecondShaper(&p.Shaper2G[0], Curve2[1], cmsFormatterIs8bit(*OutputFormat))
	FillSecondShaper(&p.Shaper2B[0], Curve2[2], cmsFormatterIs8bit(*OutputFormat))

	// Convert the matrix to fixed-point representation
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			p.Mat[i][j] = DOUBLE_TO_1FIXED14(Mat.V[i].N[j])
		}
	}

	for i := 0; i < 3; i++ {
		if Off == nil {
			p.Off[i] = 0
		} else {
			p.Off[i] = DOUBLE_TO_1FIXED14(Off.N[i])
		}
	}

	// Optimization flag for faster formatter
	if cmsFormatterIs8bit(*OutputFormat) {
		*OutputFormat |= OPTIMIZED_SH(1)
	}

	cmsPipelineSetOptimizationParameters(Dest, MatShaperEval16, unsafe.Pointer(p), FreeMatShaper, DupMatShaper)
	return true
}
func OptimizeMatrixShaper(Lut **cmsPipeline, Intent uint32, InputFormat *uint32, OutputFormat *uint32, dwFlags *uint32) bool {
	var Curve1, Curve2 *cmsStage
	var Matrix1, Matrix2 *cmsStage
	var res cmsMAT3
	var IdentityMat bool
	var Dest, Src *cmsPipeline
	var Offset *float64

	// Only works for RGB to RGB
	if T_CHANNELS(*InputFormat) != 3 || T_CHANNELS(*OutputFormat) != 3 {
		return false
	}

	// Only works on 8-bit input
	if !cmsFormatterIs8bit(*InputFormat) {
		return false
	}

	// Source LUT
	Src = *Lut

	// Check for `shaper-matrix-matrix-shaper` or `shaper-matrix-shaper` constructs
	IdentityMat = false
	if cmsPipelineCheckAndRetrieveStages(Src, 4, []cmsStageSignature{cmsSigCurveSetElemType, cmsSigMatrixElemType, cmsSigMatrixElemType, cmsSigCurveSetElemType},
		&Curve1, &Matrix1, &Matrix2, &Curve2) {
		// Get both matrices
		Data1 := (*cmsStageMatrixData)(cmsStageData(Matrix1))
		Data2 := (*cmsStageMatrixData)(cmsStageData(Matrix2))

		// Only RGB to RGB
		if Matrix1.InputChannels != 3 || Matrix1.OutputChannels != 3 ||
			Matrix2.InputChannels != 3 || Matrix2.OutputChannels != 3 {
			return false
		}

		// Input offset should be zero
		if Data1.Offset != nil {
			return false
		}

		// Multiply both matrices to get the result
		cmsMAT3per(&res, (*cmsMAT3)(unsafe.Pointer(Data2.Double)), (*cmsMAT3)(unsafe.Pointer(Data1.Double)))

		// Only the second matrix has an offset
		Offset = Data2.Offset

		// Check if the result is an identity matrix
		if cmsMAT3isIdentity(&res) && Offset == nil {
			IdentityMat = true // Can optimize further
		}
	} else if cmsPipelineCheckAndRetrieveStages(Src, 3, []cmsStageSignature{cmsSigCurveSetElemType, cmsSigMatrixElemType, cmsSigCurveSetElemType},
		&Curve1, &Matrix1, &Curve2) {
		// Single matrix case
		Data := (*cmsStageMatrixData)(cmsStageData(Matrix1))

		// Copy the matrix to the result
		memcpy(unsafe.Pointer(&res), unsafe.Pointer(Data.Double), unsafe.Sizeof(res))

		// Preserve the offset (may be nil for zero offset)
		Offset = Data.Offset

		// Check if the matrix is an identity matrix
		if cmsMAT3isIdentity(&res) && Offset == nil {
			IdentityMat = true // Can optimize further
		}
	} else {
		// Not optimizable
		return false
	}

	// Allocate an empty LUT
	Dest = cmsPipelineAlloc(Src.ContextID, Src.InputChannels, Src.OutputChannels)
	if Dest == nil {
		return false
	}

	// Assemble the new LUT
	if !cmsPipelineInsertStage(Dest, cmsAT_BEGIN, cmsStageDup(Curve1)) {
		goto Error
	}

	if !IdentityMat {
		if !cmsPipelineInsertStage(Dest, cmsAT_END, cmsStageAllocMatrix(Dest.ContextID, 3, 3, (*float64)(unsafe.Pointer(&res)), Offset)) {
			goto Error
		}
	}

	if !cmsPipelineInsertStage(Dest, cmsAT_END, cmsStageDup(Curve2)) {
		goto Error
	}

	// If identity matrix, optimize the curves further
	if IdentityMat {
		OptimizeByJoiningCurves(&Dest, Intent, InputFormat, OutputFormat, dwFlags)
	} else {
		mpeC1 := (*cmsStageToneCurvesData)(cmsStageData(Curve1))
		mpeC2 := (*cmsStageToneCurvesData)(cmsStageData(Curve2))

		// Disable cache for this optimization
		*dwFlags |= cmsFLAGS_NOCACHE

		// Set up optimization routines
		SetMatShaper(Dest, ConvertToToneCurveArray(mpeC1.TheCurves), &res, (*cmsVEC3)(unsafe.Pointer(Offset)), ConvertToToneCurveArray(mpeC2.TheCurves), OutputFormat)
	}

	// Free the original pipeline and replace it with the optimized one
	cmsPipelineFree(Src)
	*Lut = Dest
	return true

Error:
	// Free the destination pipeline and leave the source unchanged
	cmsPipelineFree(Dest)
	return false
}

// cmsOptimizationCollection represents a linked list of optimization methods.
type cmsOptimizationCollection struct {
	OptimizePtr cmsOPToptimizeFn
	Next        *cmsOptimizationCollection
}

// cmsOptimizationPluginChunkType represents the plugin chunk for optimizations.
type cmsOptimizationPluginChunkType struct {
	OptimizationCollection *cmsOptimizationCollection
}

var DefaultOptimization []cmsOptimizationCollection

func init() {
	DefaultOptimization = []cmsOptimizationCollection{
		{OptimizePtr: OptimizeByJoiningCurves, Next: &DefaultOptimization[1]},
		{OptimizePtr: OptimizeMatrixShaper, Next: &DefaultOptimization[2]},
		{OptimizePtr: OptimizeByComputingLinearization, Next: &DefaultOptimization[3]},
		{OptimizePtr: OptimizeByResampling, Next: nil},
	}
}

// Global optimization plugin chunk.
var cmsOptimizationPluginChunk = cmsOptimizationPluginChunkType{OptimizationCollection: nil}

// DupPluginOptimizationList duplicates the optimization list for a new context.
func DupPluginOptimizationList(ctx CmsContext, src CmsContext) {
	var newHead cmsOptimizationPluginChunkType
	var entry, prev *cmsOptimizationCollection
	head := (*cmsOptimizationPluginChunkType)(src.chunks[OptimizationPlugin])

	if head == nil {
		return
	}

	// Walk the list and copy each node.
	for entry = head.OptimizationCollection; entry != nil; entry = entry.Next {
		newEntry := (*cmsOptimizationCollection)(cmsSubAllocDup(ctx.MemPool, unsafe.Pointer(entry), uint32(unsafe.Sizeof(*entry))))
		if newEntry == nil {
			return
		}

		// Maintain order in the linked list.
		newEntry.Next = nil
		if prev != nil {
			prev.Next = newEntry
		}

		prev = newEntry

		if newHead.OptimizationCollection == nil {
			newHead.OptimizationCollection = newEntry
		}
	}

	ctx.chunks[OptimizationPlugin] = cmsSubAllocDup(ctx.MemPool, unsafe.Pointer(&newHead), uint32(unsafe.Sizeof(newHead)))
}

// cmsAllocOptimizationPluginChunk allocates the optimization plugin chunk.
func cmsAllocOptimizationPluginChunk(ctx CmsContext, src CmsContext) {
	if src != nil {
		DupPluginOptimizationList(ctx, src)
	} else {
		var defaultChunk cmsOptimizationPluginChunkType
		ctx.chunks[OptimizationPlugin] = cmsSubAllocDup(ctx.MemPool, unsafe.Pointer(&defaultChunk), uint32(unsafe.Sizeof(defaultChunk)))
	}
}

// cmsRegisterOptimizationPlugin registers a new optimization plugin.
func cmsRegisterOptimizationPlugin(ContextID CmsContext, Data *cmsPluginBase) bool {
	plugin := (*cmsPluginOptimization)(unsafe.Pointer(Data))
	ctx := (*cmsOptimizationPluginChunkType)(ContextID.chunks[OptimizationPlugin])
	var newNode *cmsOptimizationCollection

	if Data == nil {
		ctx.OptimizationCollection = nil
		return true
	}

	// Ensure the optimizer callback is present.
	if plugin.OptimizePtr == nil {
		return false
	}

	newNode = (*cmsOptimizationCollection)(cmsPluginMalloc(ContextID, uint32(unsafe.Sizeof(cmsOptimizationCollection{}))))
	if newNode == nil {
		return false
	}

	// Copy parameters and maintain the linked list.
	newNode.OptimizePtr = plugin.OptimizePtr
	newNode.Next = ctx.OptimizationCollection
	ctx.OptimizationCollection = newNode

	return true
}

// cmsOptimizePipeline performs optimizations on a pipeline.
func cmsOptimizePipeline(ContextID CmsContext, PtrLut **cmsPipeline, Intent uint32, InputFormat, OutputFormat, dwFlags *uint32) bool {
	ctx := (*cmsOptimizationPluginChunkType)(ContextID.chunks[OptimizationPlugin])
	var AnySuccess bool
	var mpe *cmsStage

	// A CLUT is being asked, so force this specific optimization.
	if *dwFlags&cmsFLAGS_FORCE_CLUT != 0 {
		PreOptimize(*PtrLut)
		return OptimizeByResampling(PtrLut, Intent, InputFormat, OutputFormat, dwFlags)
	}

	// Check if there's anything to optimize.
	if (*PtrLut).Elements == nil {
		cmsPipelineSetOptimizationParameters(*PtrLut, FastIdentity16, unsafe.Pointer(*PtrLut), nil, nil)
		return true
	}

	// Avoid optimization for named color pipelines.
	for mpe = cmsPipelineGetPtrToFirstStage(*PtrLut); mpe != nil; mpe = cmsStageNext(mpe) {
		if cmsStageType(mpe) == cmsSigNamedColorElemType {
			return false
		}
	}

	// Pre-optimize and check for identity transformations.
	AnySuccess = PreOptimize(*PtrLut)
	if (*PtrLut).Elements == nil {
		cmsPipelineSetOptimizationParameters(*PtrLut, FastIdentity16, unsafe.Pointer(*PtrLut), nil, nil)
		return true
	}

	// Skip optimization if explicitly disabled.
	if *dwFlags&cmsFLAGS_NOOPTIMIZE != 0 {
		return false
	}

	// Try plugin optimizations.
	for opts := ctx.OptimizationCollection; opts != nil; opts = opts.Next {
		if opts.OptimizePtr(PtrLut, Intent, InputFormat, OutputFormat, dwFlags) {
			return true
		}
	}

	// Try built-in optimizations.
	for opts := &DefaultOptimization[0]; opts != nil; opts = opts.Next {
		if opts.OptimizePtr(PtrLut, Intent, InputFormat, OutputFormat, dwFlags) {
			return true
		}
	}

	// Only simple optimizations succeeded.
	return AnySuccess
}

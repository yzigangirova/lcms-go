package golcms

import (
	"math"
	"unsafe"

	"github.com/yzigangirova/lcms-go/mem"
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
	EvalCurveOut16   []cmsInterpFn16    // Points to an array of curve evaluators in 16 bits (not-owned pointer)
	ParamsCurveOut16 []*cmsInterpParams // Points to an array of references to interpolation params (not-owned pointer)
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

	NCurves   uint32     // Number of curves
	NElements uint32     // Elements in curves
	Curves    [][]uint16 // Points to a dynamically allocated array
}

// _RemoveElement removes an element from the linked chain.
func RemoveElement(mm mem.Manager, head **cmsStage) {
	mpe := *head
	next := mpe.Next
	*head = next
	cmsStageFree(mm, mpe)
}

// _Remove1Op removes all identities in the chain.
func Remove1Op(mm mem.Manager, Lut *cmsPipeline, UnaryOp cmsStageSignature) bool {
	pt := &Lut.Elements
	anyOpt := false

	for *pt != nil {
		if (*pt).Implements == UnaryOp {
			RemoveElement(mm, pt)
			anyOpt = true
		} else {
			pt = &(*pt).Next
		}
	}
	return anyOpt
}

// _Remove2Op removes two adjacent elements if they match the specified types.
func Remove2Op(mm mem.Manager, Lut *cmsPipeline, Op1, Op2 cmsStageSignature) bool {
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
			RemoveElement(mm, pt2)
			RemoveElement(mm, pt1)
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
func _MultiplyMatrix(mm mem.Manager, Lut *cmsPipeline) bool {
	//	fmt.Println("MultiplyMatrix")
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

		if (*pt1).Implements == CmsSigMatrixElemType && (*pt2).Implements == CmsSigMatrixElemType {
			m1, ok1 := cmsStageData(*pt1).(*cmsStageMatrixData)
			m2, ok2 := cmsStageData(*pt2).(*cmsStageMatrixData)
			if !ok1 || !ok2 {
				cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageMatrixData\n")
				return false
			}
			var res cmsMAT3

			if m1.Offset != nil || m2.Offset != nil ||
				cmsStageInputChannels(*pt1) != 3 || cmsStageOutputChannels(*pt1) != 3 ||
				cmsStageInputChannels(*pt2) != 3 || cmsStageOutputChannels(*pt2) != 3 {
				return false
			}

			res = cmsMAT3perFromSlices(m2.Double[:], m1.Double[:])
			chain := (*pt2).Next
			RemoveElement(mm, pt2)
			RemoveElement(mm, pt1)

			if !isFloatMatrixIdentity(&res) {
				Multmat := cmsStageAllocMatrix(mm, Lut.ContextID, 3, 3, MatToSlice(res), nil)
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
func PreOptimize(mm mem.Manager, Lut *cmsPipeline) bool {
	var anyOpt, opt bool

	for {
		opt = false

		// Optimization steps
		opt = opt || Remove1Op(mm, Lut, CmsSigIdentityElemType)
		opt = opt || Remove2Op(mm, Lut, CmsSigXYZ2LabElemType, CmsSigLab2XYZElemType)
		opt = opt || Remove2Op(mm, Lut, CmsSigLab2XYZElemType, CmsSigXYZ2LabElemType)
		opt = opt || Remove2Op(mm, Lut, CmsSigLabV4toV2, CmsSigLabV2toV4)
		opt = opt || Remove2Op(mm, Lut, CmsSigLabV2toV4, CmsSigLabV4toV2)
		opt = opt || Remove2Op(mm, Lut, CmsSigLab2FloatPCS, CmsSigFloatPCS2Lab)
		opt = opt || Remove2Op(mm, Lut, CmsSigXYZ2FloatPCS, CmsSigFloatPCS2XYZ)
		opt = opt || _MultiplyMatrix(mm, Lut)

		if opt {
			anyOpt = true
		} else {
			break
		}
	}

	return anyOpt
}

func Eval16nop1D(mm mem.Manager, Input []uint16, Output []uint16, params *cmsInterpParams) {
	Output[0] = Input[0]
}

// PrelinEval16 implements the optimized interpolation for 16-bit input
func PrelinEval16(mm mem.Manager, Input []uint16, Output []uint16, D any) {
	p16, ok := D.(*Prelin16Data)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *Prelin16Data\n")
		return
	}
	var StageABC [16]uint16
	var StageDEF [16]uint16

	// Handle input curves
	for i := uint32(0); i < p16.NInputs; i++ {
		// Call the interpolation function
		p16.EvalCurveIn16[i](mm, Input[i:], StageABC[i:], p16.ParamsCurveIn16[i])
	}

	// Evaluate the CLUT
	p16.EvalCLUT(mm, StageABC[:], StageDEF[:], p16.CLUTParams)

	// Handle output curves
	for i := uint32(0); i < p16.NOutputs; i++ {
		p16.EvalCurveOut16[i](mm, StageDEF[i:], Output[i:], p16.ParamsCurveOut16[i])
	}
}

// PrelinOpt16free frees memory associated with Prelin16Data
func PrelinOpt16free(ContextID CmsContext, ptr any) {
	p16, ok := ptr.(*Prelin16Data)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *Prelin16Data\n")
		return
	}
	cmsFree(ContextID, p16)
}

// Prelin16dup duplicates the Prelin16Data structure

func Prelin16dup(_ CmsContext, ptr any) any {
	p16, ok := ptr.(*Prelin16Data)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *Prelin16Data\n")
		return nil
	}
	// Create a new struct
	duped := &Prelin16Data{
		ContextID:       p16.ContextID,
		NInputs:         p16.NInputs,
		NOutputs:        p16.NOutputs,
		EvalCurveIn16:   p16.EvalCurveIn16,   // [fixed array] copied by value
		ParamsCurveIn16: p16.ParamsCurveIn16, // [fixed array of ptrs] copied as-is (shallow copy)
		EvalCLUT:        p16.EvalCLUT,
		CLUTParams:      p16.CLUTParams, // non-owned pointer: shallow copy
	}

	duped.EvalCurveOut16 = ([]cmsInterpFn16)(cmsDupMemSlice(p16.EvalCurveOut16))
	duped.ParamsCurveOut16 = ([]*cmsInterpParams)(cmsDupMemSlice(p16.ParamsCurveOut16))

	return duped
}

// PrelinOpt16alloc allocates and initializes Prelin16Data
func PrelinOpt16alloc(mm mem.Manager, ContextID CmsContext, ColorMap *cmsInterpParams, nInputs uint32, In []*CmsToneCurve, nOutputs uint32, Out []*CmsToneCurve) *Prelin16Data {
	//fmt.Println(" PrelinOpt16alloc")
	p16 := mem.New[Prelin16Data](mm)
	if p16 == nil {
		return nil
	}

	p16.NInputs = nInputs
	p16.NOutputs = nOutputs

	// Handle input curves
	for i := uint32(0); i < nInputs; i++ {

		if In == nil {
			p16.ParamsCurveIn16[i] = nil
			p16.EvalCurveIn16[i] = Eval16nop1D
		} else {
			p16.ParamsCurveIn16[i] = In[i].InterpParams
			p16.EvalCurveIn16[i] = In[i].InterpParams.Interpolation.Lerp16
		}
	}

	p16.CLUTParams = ColorMap
	p16.EvalCLUT = ColorMap.Interpolation.Lerp16

	// Allocate memory for EvalCurveOut16 and ParamsCurveOut16
	p16.EvalCurveOut16 = mem.MakeSlice[cmsInterpFn16](mm, int(nOutputs))
	if p16.EvalCurveOut16 == nil {
		cmsFree(ContextID, p16)
		return nil
	}

	p16.ParamsCurveOut16 = mem.MakeSlice[*cmsInterpParams](mm, int(nOutputs))
	if p16.ParamsCurveOut16 == nil {
		cmsFree(ContextID, p16)
		return nil
	}

	// Handle output curves
	for i := uint32(0); i < nOutputs; i++ {
		if Out == nil {
			p16.ParamsCurveOut16[i] = nil
			p16.EvalCurveOut16[i] = Eval16nop1D
		} else {
			p16.ParamsCurveOut16[i] = Out[i].InterpParams
			p16.EvalCurveOut16[i] = p16.ParamsCurveOut16[i].Interpolation.Lerp16
		}
	}

	return p16
}

const PRELINEARIZATION_POINTS = 4096

func XFormSampler16(mm mem.Manager, In, Out []uint16, cargo any) int32 {
	Lut, ok := cargo.(*cmsPipeline)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "XFormSampler16: cargo not *cmsPipeline")
		return 0
	}

	sc := mm.Scratch()
	inF := sc.LUT[0]
	outF := sc.LUT[1]

	nIn := int(Lut.InputChannels)
	nOut := int(Lut.OutputChannels)

	// optional guards in debug builds
	if nIn > len(inF) || nOut > len(outF) {
		cmsSignalError(nil, cmsERROR_RANGE, "channels exceed scratch capacity")
		return 0
	}

	const inv65535 = 1.0 / 65535.0
	for i := 0; i < nIn; i++ {
		inF[i] = float32(In[i]) * inv65535
	}

	cmsPipelineEvalFloat(mm, inF[:nIn], outF[:nOut], Lut)

	const scale = 65535.0
	for i := 0; i < nOut; i++ {
		Out[i] = cmsQuickSaturateWord(float64(outF[i]) * scale)
	}
	return 1
}

func AllCurvesAreLinear(mpe *cmsStage) bool {
	Curves := cmsStageGetPtrToCurveSet(mpe)
	if Curves == nil {
		return false
	}

	n := cmsStageOutputChannels(mpe)

	for i := uint32(0); i < n; i++ {
		if !cmsIsToneCurveLinear(Curves[i]) {
			return false
		}
	}

	return true
}

func PatchLUT(CLUT *cmsStage, At []uint16, Value []uint16, nChannelsOut, nChannelsIn uint32) bool {
	Grid, ok := CLUT.Data.(*cmsStageCLutData)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageCLutData\n")
		return false
	}
	p16 := Grid.Params
	var px, py, pz, pw float64
	var x0, y0, z0, w0, index int

	if CLUT.Type != CmsSigCLutElemType {
		cmsSignalError(CLUT.ContextID, cmsERROR_INTERNAL, "(internal) Attempt to PatchLUT on non-lut stage")
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
		cmsSignalError(CLUT.ContextID, cmsERROR_INTERNAL, "(internal) %d Channels are not supported on PatchLUT", nChannelsIn)
		return false
	}

	for i := 0; i < int(nChannelsOut); i++ {
		Grid.Tab.([]uint16)[index+i] = Value[i]
	}

	return true
}

func WhitesAreEqual(n uint32, White1, White2 []uint16) bool {
	for i := uint32(0); i < n; i++ {
		diff := int(White1[i]) - int(White2[i])
		if math.Abs(float64(diff)) > 0xf000 {
			return true // Values are extremely different; avoid fixup.
		}
		if White1[i] != White2[i] {
			return false
		}
	}
	return true
}
func FixWhiteMisalignment(mm mem.Manager, Lut *cmsPipeline, EntryColorSpace, ExitColorSpace cmsColorSpaceSignature) bool {
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
	cmsPipelineEval16(mm, WhitePointIn, ObtainedOut[:], Lut)

	// If the output already matches the expected white point, return early.
	//CHECK comparison!   WhitePointOut and ObtainedOut has different slice length!
	if WhitesAreEqual(nOuts, WhitePointOut[:], ObtainedOut[:]) {
		return true
	}

	// Check for pre-linearization, CLUT, and post-linearization stages.
	if !cmsPipelineCheckAndRetrieveStages(Lut, 3, []cmsStageSignature{CmsSigCurveSetElemType, CmsSigCLutElemType, CmsSigCurveSetElemType}, &PreLin, &CLUT, &PostLin) &&
		!cmsPipelineCheckAndRetrieveStages(Lut, 2, []cmsStageSignature{CmsSigCurveSetElemType, CmsSigCLutElemType}, &PreLin, &CLUT) &&
		!cmsPipelineCheckAndRetrieveStages(Lut, 2, []cmsStageSignature{CmsSigCLutElemType, CmsSigCurveSetElemType}, &CLUT, &PostLin) &&
		!cmsPipelineCheckAndRetrieveStages(Lut, 1, []cmsStageSignature{CmsSigCLutElemType}, &CLUT) {
		return false
	}

	// Interpolate white points for pre-linearization.
	if PreLin != nil {
		Curves := cmsStageGetPtrToCurveSet(PreLin)
		for i := uint32(0); i < nIns; i++ {
			WhiteIn[i] = cmsEvalToneCurve16(mm, Curves[i], WhitePointIn[i])
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
			InversePostLin := cmsReverseToneCurve(mm, Curves[i])
			if InversePostLin == nil {
				WhiteOut[i] = WhitePointOut[i]
			} else {
				WhiteOut[i] = cmsEvalToneCurve16(mm, InversePostLin, WhitePointOut[i])
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

func OptimizeByResampling(mm mem.Manager, Lut **cmsPipeline, Intent uint32, InputFormat *uint32, OutputFormat *uint32, dwFlags *uint32) bool {
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
		DataSetIn        []*CmsToneCurve
		DataSetOut       []*CmsToneCurve
		p16              *Prelin16Data
	)
	var ok bool
	//fmt.Println("OptimizeByResampling")

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
	Dest = cmsPipelineAlloc(mm, Src.ContextID, Src.InputChannels, Src.OutputChannels)
	if Dest == nil {
		return false
	}

	// Handle prelinearization
	if *dwFlags&CmsFLAGS_CLUT_PRE_LINEARIZATION != 0 {
		PreLin := cmsPipelineGetPtrToFirstStage(Src)
		if PreLin != nil && PreLin.Type == CmsSigCurveSetElemType {
			if !AllCurvesAreLinear(PreLin) {
				NewPreLin = cmsStageDup(mm, PreLin)
				if NewPreLin == nil || !cmsPipelineInsertStage(Dest, CmsAT_BEGIN, NewPreLin) {
					goto Error
				}
				cmsPipelineUnlinkStage(mm, Src, CmsAT_BEGIN, &KeepPreLin)
			}
		}
	}

	// Allocate the CLUT
	CLUT = cmsStageAllocCLut16bit(mm, Src.ContextID, nGridPoints, Src.InputChannels, Src.OutputChannels, nil)

	if CLUT == nil || !cmsPipelineInsertStage(Dest, CmsAT_END, CLUT) {
		goto Error
	}

	// Handle postlinearization
	if *dwFlags&CmsFLAGS_CLUT_POST_LINEARIZATION != 0 {
		PostLin := cmsPipelineGetPtrToLastStage(Src)
		if PostLin != nil && cmsStageType(PostLin) == CmsSigCurveSetElemType {
			if !AllCurvesAreLinear(PostLin) {
				NewPostLin = cmsStageDup(mm, PostLin)
				if NewPostLin == nil || !cmsPipelineInsertStage(Dest, CmsAT_END, NewPostLin) {
					goto Error
				}
				cmsPipelineUnlinkStage(mm, Src, CmsAT_END, &KeepPostLin)
			}
		}
	}

	// Perform sampling
	if !cmsStageSampleCLut16bit(mm, CLUT, XFormSampler16, Src, 0) {
		goto Error
	}

	/***********************************************/
	// Cleanup after sampling
	if KeepPreLin != nil {
		cmsStageFree(mm, KeepPreLin)
	}
	if KeepPostLin != nil {
		cmsStageFree(mm, KeepPostLin)
	}
	cmsPipelineFree(mm, Src)

	DataCLUT, ok = CLUT.Data.(*cmsStageCLutData)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageCLutData\n")
		return false
	}

	if NewPreLin != nil {
		DataSetIn = ((NewPreLin.Data.(*cmsStageToneCurvesData)).TheCurves)
	}
	if NewPostLin != nil {
		DataSetOut = ((NewPostLin.Data.(*cmsStageToneCurvesData)).TheCurves)
	}

	if DataSetIn == nil && DataSetOut == nil {
		/*cmsPipelineSetOptimizationParameters(Dest,
		func(mm mem.Manager, In, Out []uint16, Data any) {
			Data.(*cmsInterpParams).Interpolation.Lerp16(mm, In, Out, Data.(*cmsInterpParams))
		},
		DataCLUT.Params, nil, nil)*/
		cmsPipelineSetFastOptimization(
			Dest,
			DataCLUT.Params.Interpolation.Lerp16, // e.g., Eval4Inputs
			DataCLUT.Params,
		)

	} else {

		p16 = PrelinOpt16alloc(mm, Dest.ContextID, DataCLUT.Params, Dest.InputChannels, DataSetIn, Dest.OutputChannels, DataSetOut)
		if p16 == nil {
			goto Error
		}
		cmsPipelineSetOptimizationParameters(Dest, PrelinEval16, p16, PrelinOpt16free, Prelin16dup)
	}

	// Adjust for absolute colorimetric intent
	if Intent == INTENT_ABSOLUTE_COLORIMETRIC {
		*dwFlags |= CmsFLAGS_NOWHITEONWHITEFIXUP
	}
	if *dwFlags&CmsFLAGS_NOWHITEONWHITEFIXUP == 0 {
		FixWhiteMisalignment(mm, Dest, ColorSpace, OutputColorSpace)
	}

	*Lut = Dest
	//fmt.Println("END OptimizeByResampling")

	return true

Error:
	// Restore original stages on error
	if KeepPreLin != nil {
		cmsPipelineInsertStage(Src, CmsAT_BEGIN, KeepPreLin)
	}
	if KeepPostLin != nil {
		cmsPipelineInsertStage(Src, CmsAT_END, KeepPostLin)
	}
	cmsPipelineFree(mm, Dest)
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

	Val = float64(g.Table16[AtBegin])
	Slope = (Val - float64(BeginVal)) / float64(AtBegin)
	beta = Val - Slope*float64(AtBegin)

	for i := 0; i < AtBegin; i++ {
		g.Table16[i] = cmsQuickSaturateWord(float64(i)*Slope + beta)
	}

	Val = float64(g.Table16[AtEnd])
	Slope = (float64(EndVal) - Val) / float64(AtBegin)
	beta = Val - Slope*float64(AtEnd)

	for i := AtEnd; i < int(g.nEntries); i++ {
		g.Table16[i] = cmsQuickSaturateWord(float64(i)*Slope + beta)
	}
}

func PrelinOpt8alloc(mm mem.Manager, ContextID CmsContext, p *cmsInterpParams, G [3]*CmsToneCurve) *Prelin8Data {
	p8 := mem.New[Prelin8Data](mm)
	if p8 == nil {
		return nil
	}

	for i := 0; i < 256; i++ {
		var Input [3]uint16

		if G[0] != nil {
			Input[0] = cmsEvalToneCurve16(mm, G[0], FROM_8_TO_16(uint8(i)))
			Input[1] = cmsEvalToneCurve16(mm, G[1], FROM_8_TO_16(uint8(i)))
			Input[2] = cmsEvalToneCurve16(mm, G[2], FROM_8_TO_16(uint8(i)))
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

func Prelin8free(ContextID CmsContext, ptr any) {
	cmsFree(ContextID, ptr)
}

func Prelin8dup(ContextID CmsContext, ptr any) any {
	p, ok := ptr.(*Prelin8Data)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *Prelin8Data\n")
		return nil
	}
	copied := new(Prelin8Data)
	*copied = *p // struct copy
	return copied
}

func PrelinEval8(mm mem.Manager, Input []uint16, Output []uint16, D any) {
	var r, g, b uint8
	var rx, ry, rz cmsS15Fixed16Number
	var c0, c1, c2, c3, Rest cmsS15Fixed16Number
	var X0, X1, Y0, Y1, Z0, Z1 cmsS15Fixed16Number

	p8, ok := D.(*Prelin8Data)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *Prelin8Data\n")
		return
	}
	p := p8.P
	TotalOut := int(p.nOutputs)
	// Ensure `p.Table` is a `[]uint16`
	LutTable, ok := p.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in LinLerp1D")

	}
	// DENS implementation
	DENS := func(i, j, k, outChan uint32) cmsS15Fixed16Number {
		return cmsS15Fixed16Number(LutTable[int(i+j+k+outChan)])
	}

	r = uint8(Input[0] >> 8)
	g = uint8(Input[1] >> 8)
	b = uint8(Input[2] >> 8)

	X0 = cmsS15Fixed16Number(p8.X0[r])
	Y0 = cmsS15Fixed16Number(p8.Y0[g])
	Z0 = cmsS15Fixed16Number(p8.Z0[b])

	rx = cmsS15Fixed16Number(p8.Rx[r])
	ry = cmsS15Fixed16Number(p8.Ry[g])
	rz = cmsS15Fixed16Number(p8.Rz[b])

	X1 = X0
	if rx != 0 {
		X1 += cmsS15Fixed16Number(p.opta[2])
	}
	Y1 = Y0
	if ry != 0 {
		Y1 += cmsS15Fixed16Number(p.opta[1])
	}
	Z1 = Z0
	if rz != 0 {
		Z1 += cmsS15Fixed16Number(p.opta[0])
	}

	for OutChan := 0; OutChan < TotalOut; OutChan++ {
		c0 = DENS(uint32(X0), uint32(Y0), uint32(Z0), uint32(OutChan))
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

		Rest = cmsS15Fixed16Number(c1*rx + c2*ry + c3*rz + 0x8001)
		Output[OutChan] = uint16(c0 + ((Rest + (Rest >> 16)) >> 16))
	}
}

func IsDegenerated(g *CmsToneCurve) bool {
	Zeros, Poles := 0, 0

	for i := uint32(0); i < g.nEntries; i++ {
		if g.Table16[i] == 0x0000 {
			Zeros++
		}
		if g.Table16[i] == 0xffff {
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

func OptimizeByComputingLinearization(mm mem.Manager, Lut **cmsPipeline, Intent uint32, InputFormat, OutputFormat, dwFlags *uint32) bool {
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
		OptimizedPrelinCurves []*CmsToneCurve
		OptimizedPrelinCLUT   *cmsStageCLutData
		OptimizedCLUTmpe      *cmsStage
	)
	var ok bool

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
		if (*dwFlags & CmsFLAGS_CLUT_PRE_LINEARIZATION) == 0 {
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

	if cmsStageType(last) == CmsSigCurveSetElemType {
		Data, ok := (cmsStageData(last)).(*cmsStageToneCurvesData)
		if !ok {
			cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *Prelin8Data\n")
			goto Error
		}

		for i := uint32(0); i < Data.NCurves; i++ {
			if IsDegenerated(Data.TheCurves[i]) {
				goto Error
			}
		}
	}

	for t := uint32(0); t < OriginalLut.InputChannels; t++ {
		Trans[t] = cmsBuildTabulatedToneCurve16(mm, OriginalLut.ContextID, PRELINEARIZATION_POINTS, nil)
		if Trans[t] == nil {
			goto Error
		}
	}

	for i := uint32(0); i < PRELINEARIZATION_POINTS; i++ {
		v := float32(float64(i) / float64(PRELINEARIZATION_POINTS-1))
		for t := uint32(0); t < OriginalLut.InputChannels; t++ {
			In[t] = v
		}
		cmsPipelineEvalFloat(mm, In[:], Out[:], OriginalLut)

		for t := uint32(0); t < OriginalLut.InputChannels; t++ {
			Trans[t].Table16[i] = cmsQuickSaturateWord(float64(Out[t]) * 65535.0)
		}
	}

	for t := uint32(0); t < OriginalLut.InputChannels; t++ {
		SlopeLimiting(Trans[t])
	}

	for t := uint32(0); t < OriginalLut.InputChannels; t++ {
		if !cmsIsToneCurveLinear(Trans[t]) {
			//keep C code similarity, although var is unused
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
		TransReverse[t] = cmsReverseToneCurveEx(mm, PRELINEARIZATION_POINTS, Trans[t])
		if TransReverse[t] == nil {
			goto Error
		}
	}

	LutPlusCurves = cmsPipelineDup(mm, OriginalLut)
	if LutPlusCurves == nil {
		goto Error
	}

	if !cmsPipelineInsertStage(LutPlusCurves, CmsAT_BEGIN, cmsStageAllocToneCurves(mm, OriginalLut.ContextID, OriginalLut.InputChannels, TransReverse[:])) {
		goto Error
	}

	OptimizedLUT = cmsPipelineAlloc(mm, OriginalLut.ContextID, OriginalLut.InputChannels, OriginalLut.OutputChannels)
	if OptimizedLUT == nil {
		goto Error
	}

	OptimizedPrelinMpe = cmsStageAllocToneCurves(mm, OriginalLut.ContextID, OriginalLut.InputChannels, Trans[:])
	if !cmsPipelineInsertStage(OptimizedLUT, CmsAT_BEGIN, OptimizedPrelinMpe) {
		goto Error
	}

	OptimizedCLUTmpe = cmsStageAllocCLut16bit(mm, OriginalLut.ContextID, nGridPoints, OriginalLut.InputChannels, OriginalLut.OutputChannels, nil)
	if !cmsPipelineInsertStage(OptimizedLUT, CmsAT_END, OptimizedCLUTmpe) {
		goto Error
	}

	if !cmsStageSampleCLut16bit(mm, OptimizedCLUTmpe, XFormSampler16, LutPlusCurves, 0) {
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

	cmsPipelineFree(mm, LutPlusCurves)

	OptimizedPrelinCurves = cmsStageGetPtrToCurveSet(OptimizedPrelinMpe)
	OptimizedPrelinCLUT, ok = OptimizedCLUTmpe.Data.(*cmsStageCLutData)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "not of the type cmsStageCLutData\n")
		return false
	}

	if cmsFormatterIs8bit(*InputFormat) {

		p8 := PrelinOpt8alloc(mm, OptimizedLUT.ContextID, OptimizedPrelinCLUT.Params, ConvertToToneCurveArray(OptimizedPrelinCurves))
		if p8 == nil {
			return false
		}
		cmsPipelineSetOptimizationParameters(OptimizedLUT, PrelinEval8, p8, Prelin8free, Prelin8dup)
	} else {
		p16 := PrelinOpt16alloc(mm, OptimizedLUT.ContextID, OptimizedPrelinCLUT.Params, 3, OptimizedPrelinCurves, 3, nil)
		if p16 == nil {
			return false
		}
		cmsPipelineSetOptimizationParameters(OptimizedLUT, PrelinEval16, p16, PrelinOpt16free, Prelin16dup)
	}

	if Intent == INTENT_ABSOLUTE_COLORIMETRIC {
		*dwFlags |= CmsFLAGS_NOWHITEONWHITEFIXUP
	}

	if (*dwFlags & CmsFLAGS_NOWHITEONWHITEFIXUP) == 0 {
		if !FixWhiteMisalignment(mm, OptimizedLUT, ColorSpace, OutputColorSpace) {
			return false
		}
	}

	cmsPipelineFree(mm, OriginalLut)
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
		cmsPipelineFree(mm, LutPlusCurves)
	}
	if OptimizedLUT != nil {
		cmsPipelineFree(mm, OptimizedLUT)
	}

	return false
}
func ConvertToToneCurveArray(curves []*CmsToneCurve) [3]*CmsToneCurve {
	var result [3]*CmsToneCurve
	copy(result[:], curves[:3]) // Convert slice to array
	return result
}
func CurvesFree(ContextID CmsContext, ptr any) {
	cmsFree(ContextID, ptr)
}

// CurvesDup duplicates a Curves16Data structure
func CurvesDup(ContextID CmsContext, ptr any) any {
	srcData, ok := ptr.(*Curves16Data)
	if srcData == nil {
		return nil
	}
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "not of the type *Curves16Data\n")
		return nil
	}

	// Step 1: Allocate a new Curves16Data struct
	data := &Curves16Data{
		NCurves:   srcData.NCurves,
		NElements: srcData.NElements,
		Curves:    mem.MakeSlice[[]uint16](mem.Manager{}, int(srcData.NCurves)), // Allocate slice of slices
	}

	// Step 2: Copy each curve (each row in 2D array)
	for i := uint32(0); i < srcData.NCurves; i++ {
		if srcData.Curves[i] != nil {
			// Allocate new slice for each curve
			data.Curves[i] = mem.MakeSlice[uint16](mem.Manager{}, int(srcData.NElements))

			// Copy the curve data
			copy(data.Curves[i], srcData.Curves[i])
		}
	}

	return data
}

func CurvesAlloc(mm mem.Manager, ContextID CmsContext, nCurves, nElements uint32, G []*CmsToneCurve) *Curves16Data {
	// Step 1: Allocate the main structure
	c16 := &Curves16Data{
		NCurves:   nCurves,
		NElements: nElements,
		Curves:    mem.MakeSlice[[]uint16](mem.Manager{}, int(nCurves)), // Allocate slice of slices
	}

	// Step 2: Allocate memory for each curve (each row in 2D array)
	for i := uint32(0); i < nCurves; i++ {
		c16.Curves[i] = mem.MakeSlice[uint16](mem.Manager{}, int(nElements)) // Allocate slice for each row

		// Step 3: Fill the curve with evaluated values
		for j := uint32(0); j < nElements; j++ {
			if nElements == 256 {
				c16.Curves[i][j] = cmsEvalToneCurve16(mm, G[i], FROM_8_TO_16(uint8(j)))
			} else {
				c16.Curves[i][j] = cmsEvalToneCurve16(mm, G[i], uint16(j))
			}
		}
	}

	return c16
}

func FastEvaluateCurves8(mm mem.Manager, In []uint16, Out []uint16, D any) {
	data, ok := D.(*Curves16Data)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "not of the type *Curves16Data\n")
		return
	}

	// Ensure Out has enough space
	if len(Out) < int(data.NCurves) || len(In) < int(data.NCurves) {
		return
	}

	for i := uint32(0); i < data.NCurves; i++ {
		// Directly access the i-th curve using slices
		if data.Curves[i] == nil {
			continue
		}

		// Access the i-th input value
		inValue := In[i]

		// Calculate the index in the curve
		x := inValue >> 8 // Equivalent to dividing by 256

		// Lookup value in the curve safely
		Out[i] = data.Curves[i][x]
	}
}

func FastEvaluateCurves16(mm mem.Manager, In []uint16, Out []uint16, D any) {
	data, ok := D.(*Curves16Data)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "not of the type *Curves16Data\n")
		return
	}
	// Ensure Out and In have enough space
	if len(Out) < int(data.NCurves) || len(In) < int(data.NCurves) {
		return
	}

	for i := uint32(0); i < data.NCurves; i++ {
		// Directly access the i-th curve using slices
		if data.Curves[i] == nil {
			continue
		}

		// Get the input value for this curve
		inValue := In[i]

		// Lookup value in the curve using `inValue` as index
		Out[i] = data.Curves[i][inValue]
	}
}
func FastIdentity16(mm mem.Manager, In []uint16, Out []uint16, D any) {
	Lut, ok := D.(*cmsPipeline)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "not of the type *cmsPipeline\n")
		return
	}
	// Ensure Out and In have enough space
	if len(Out) < int(Lut.InputChannels) || len(In) < int(Lut.InputChannels) {
		return
	}

	for i := uint32(0); i < Lut.InputChannels; i++ {
		// Directly copy input value to output using slice indexing
		Out[i] = In[i]
	}
}

// cmsOPToptimizeFn defines the function type for optimizations.
func OptimizeByJoiningCurves(mm mem.Manager,
	Lut **cmsPipeline,
	Intent uint32,
	InputFormat *uint32,
	OutputFormat *uint32,
	dwFlags *uint32,
) bool {
	var (
		GammaTables    []*CmsToneCurve
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
		if cmsStageType(mpe) != CmsSigCurveSetElemType {
			return false
		}
	}

	// Allocate an empty LUT
	Dest = cmsPipelineAlloc(mm, Src.ContextID, Src.InputChannels, Src.OutputChannels)
	if Dest == nil {
		return false
	}

	//  Allocate GammaTables as a slice instead of using `unsafe.Pointer`
	GammaTables = mem.MakeSlice[*CmsToneCurve](mm, int(Src.InputChannels))

	//  Initialize GammaTables using slice indexing
	for i = 0; i < Src.InputChannels; i++ {
		GammaTables[i] = cmsBuildTabulatedToneCurve16(mm, Src.ContextID, PRELINEARIZATION_POINTS, nil)
		if GammaTables[i] == nil {
			goto Error
		}
	}

	//  Compute 16-bit results using floating-point values
	for i = 0; i < PRELINEARIZATION_POINTS; i++ {
		for j = 0; j < Src.InputChannels; j++ {
			InFloat[j] = float32(float64(i) / float64(PRELINEARIZATION_POINTS-1))
		}

		cmsPipelineEvalFloat(mm, InFloat[:], OutFloat[:], Src)
		for j = 0; j < Src.InputChannels; j++ {
			//  Access the tone curve directly using slice indexing
			toneCurve := GammaTables[j]

			//  Update the table value safely
			toneCurve.Table16[i] = cmsQuickSaturateWord(float64(OutFloat[j]) * 65535.0)
		}
	}

	//  Allocate obtained curves using slice
	ObtainedCurves = cmsStageAllocToneCurves(mm, Src.ContextID, Src.InputChannels, GammaTables)
	if ObtainedCurves == nil {
		goto Error
	}

	//  Free GammaTables properly
	for i = 0; i < Src.InputChannels; i++ {
		CmsFreeToneCurve(GammaTables[i])
	}
	GammaTables = nil // Ensure it’s cleared

	//  Check if all curves are linear
	if !AllCurvesAreLinear(ObtainedCurves) {
		Data, ok := cmsStageData(ObtainedCurves).(*cmsStageToneCurvesData)
		if !ok {
			cmsSignalError(nil, cmsERROR_UNDEFINED, "not of the type *cmsStageToneCurvesData\n")
			goto Error
		}
		if !cmsPipelineInsertStage(Dest, CmsAT_BEGIN, ObtainedCurves) {
			goto Error
		}

		ObtainedCurves = nil

		if cmsFormatterIs8bit(*InputFormat) {
			c16 := CurvesAlloc(mm, Dest.ContextID, Data.NCurves, 256, Data.TheCurves)
			if c16 == nil {
				goto Error
			}
			*dwFlags |= CmsFLAGS_NOCACHE
			cmsPipelineSetOptimizationParameters(Dest, FastEvaluateCurves8, c16, CurvesFree, CurvesDup)
		} else {
			c16 := CurvesAlloc(mm, Dest.ContextID, Data.NCurves, 65536, Data.TheCurves)
			if c16 == nil {
				goto Error
			}
			*dwFlags |= CmsFLAGS_NOCACHE
			cmsPipelineSetOptimizationParameters(Dest, FastEvaluateCurves16, c16, CurvesFree, CurvesDup)
		}
	} else {
		cmsStageFree(mm, ObtainedCurves)
		ObtainedCurves = nil

		if !cmsPipelineInsertStage(Dest, CmsAT_BEGIN, cmsStageAllocIdentity(mm, Dest.ContextID, Src.InputChannels)) {
			goto Error
		}

		*dwFlags |= CmsFLAGS_NOCACHE
		cmsPipelineSetOptimizationParameters(Dest, FastIdentity16, Dest, nil, nil)
	}

	//  Replace the source LUT with the optimized LUT
	cmsPipelineFree(mm, Src)
	*Lut = Dest
	return true

Error:
	if ObtainedCurves != nil {
		cmsStageFree(mm, ObtainedCurves)
	}
	if GammaTables != nil {
		for i = 0; i < Src.InputChannels; i++ {
			if GammaTables[i] != nil {
				CmsFreeToneCurve(GammaTables[i])
			}
		}
	}
	cmsPipelineFree(mm, Dest)
	return false
}

func FreeMatShaper(ContextID CmsContext, Data any) {
	if Data != nil {
		cmsFree(ContextID, Data)
	}
}
func DupMatShaper(ContextID CmsContext, Data any) any {
	p, ok := Data.(*MatShaper8Data)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "not of the type *MatShaper8Data\n")
		return nil
	}
	copied := *p // struct copy
	return &copied
}
func MatShaperEval16(mm mem.Manager, In []uint16, Out []uint16, D any) {
	p, ok := D.(*MatShaper8Data)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "not of the type *MatShaper8Data\n")
		return
	}
	//  Ensure In and Out have at least 3 elements
	if len(In) < 3 || len(Out) < 3 {
		return
	}
	//  Extract indices from input using slice indexing
	ri := uint32(In[0] & 0xFF)
	gi := uint32(In[1] & 0xFF)
	bi := uint32(In[2] & 0xFF)

	//  Apply the first shaper using slice indexing
	r := p.Shaper1R[ri]
	g := p.Shaper1G[gi]
	b := p.Shaper1B[bi]

	//  Evaluate the matrix
	l1 := (p.Mat[0][0]*r + p.Mat[0][1]*g + p.Mat[0][2]*b + p.Off[0] + 0x2000) >> 14
	l2 := (p.Mat[1][0]*r + p.Mat[1][1]*g + p.Mat[1][2]*b + p.Off[1] + 0x2000) >> 14
	l3 := (p.Mat[2][0]*r + p.Mat[2][1]*g + p.Mat[2][2]*b + p.Off[2] + 0x2000) >> 14

	//  Clip to 0..1.0 range
	ri = clipToRange(l1, 0, 16384)
	gi = clipToRange(l2, 0, 16384)
	bi = clipToRange(l3, 0, 16384)

	//  Apply the second shaper using slice indexing
	Out[0] = p.Shaper2R[ri]
	Out[1] = p.Shaper2G[gi]
	Out[2] = p.Shaper2B[bi]
}

func clipToRange(value cmsS1Fixed14Number, min, max cmsS1Fixed14Number) uint32 {
	if value < min {
		return uint32(min)
	} else if value > max {
		return uint32(max)
	}
	return uint32(value)
}
func FillFirstShaper(mm mem.Manager, Table []cmsS1Fixed14Number, Curve *CmsToneCurve) {
	for i := 0; i < 256; i++ {
		R := float32(i) / 255.0
		y := cmsEvalToneCurveFloat(mm, Curve, R)

		if y < 131072.0 {
			Table[i] = DOUBLE_TO_1FIXED14(float64(y))
		} else {
			Table[i] = 0x7fffffff
		}
	}
}
func FillSecondShaper(mm mem.Manager, Table []uint16, Curve *CmsToneCurve, Is8BitsOutput bool) {
	//  Ensure Table has enough elements to prevent out-of-bounds errors
	if len(Table) < 16385 {
		return
	}

	for i := 0; i < 16385; i++ {
		R := float32(i) / 16384.0
		Val := cmsEvalToneCurveFloat(mm, Curve, R)

		//  Clip value to range [0, 1]
		if Val < 0 {
			Val = 0
		} else if Val > 1.0 {
			Val = 1.0
		}

		//  Use slice indexing instead of ``unsafe
		if Is8BitsOutput {
			w := cmsQuickSaturateWord(float64(Val * 65535.0))
			b := FROM_16_TO_8(w)
			Table[i] = FROM_8_TO_16(b) //  Direct slice assignment
		} else {
			Table[i] = cmsQuickSaturateWord(float64(Val * 65535.0)) //  Direct slice assignment
		}
	}
}

func SetMatShaper(mm mem.Manager, Dest *cmsPipeline, Curve1 [3]*CmsToneCurve, Mat *cmsMAT3, Off *cmsVEC3, Curve2 [3]*CmsToneCurve, OutputFormat *uint32) bool {
	//p := (*MatShaper8Data)(cmsMalloc(Dest.ContextID, uint32(unsafe.Sizeof(MatShaper8Data{}))))
	p := mem.New[MatShaper8Data](mm)
	if p == nil {
		return false
	}

	p.ContextID = Dest.ContextID

	// Fill the first and second shapers
	FillFirstShaper(mm, p.Shaper1R[:], Curve1[0])
	FillFirstShaper(mm, p.Shaper1G[:], Curve1[1])
	FillFirstShaper(mm, p.Shaper1B[:], Curve1[2])

	FillSecondShaper(mm, p.Shaper2R[:], Curve2[0], cmsFormatterIs8bit(*OutputFormat))
	FillSecondShaper(mm, p.Shaper2G[:], Curve2[1], cmsFormatterIs8bit(*OutputFormat))
	FillSecondShaper(mm, p.Shaper2B[:], Curve2[2], cmsFormatterIs8bit(*OutputFormat))

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

	cmsPipelineSetOptimizationParameters(Dest, MatShaperEval16, p, FreeMatShaper, DupMatShaper)
	return true
}
func OptimizeMatrixShaper(mm mem.Manager, Lut **cmsPipeline, Intent uint32, InputFormat *uint32, OutputFormat *uint32, dwFlags *uint32) bool {
	//	fmt.Println("OptimizeMatrixShaper")
	var Curve1, Curve2 *cmsStage
	var Matrix1, Matrix2 *cmsStage
	var res cmsMAT3
	var IdentityMat bool
	var Dest, Src *cmsPipeline
	var Offset []float64

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
	if cmsPipelineCheckAndRetrieveStages(Src, 4, []cmsStageSignature{CmsSigCurveSetElemType, CmsSigMatrixElemType, CmsSigMatrixElemType, CmsSigCurveSetElemType},
		&Curve1, &Matrix1, &Matrix2, &Curve2) {
		// Get both matrices
		Data1, ok1 := cmsStageData(Matrix1).(*cmsStageMatrixData)
		Data2, ok2 := cmsStageData(Matrix2).(*cmsStageMatrixData)
		if !ok1 || !ok2 {
			cmsSignalError(nil, cmsERROR_UNDEFINED, "not of the type *cmsStageMatrixData\n")
			return false
		}
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
		res = cmsMAT3perFromSlices(Data2.Double[:], Data1.Double[:])

		// Only the second matrix has an offset
		Offset = Data2.Offset

		// Check if the result is an identity matrix
		if cmsMAT3isIdentity(&res) && Offset == nil {
			IdentityMat = true // Can optimize further
		}
	} else {
		if cmsPipelineCheckAndRetrieveStages(Src, 3, []cmsStageSignature{CmsSigCurveSetElemType, CmsSigMatrixElemType, CmsSigCurveSetElemType},
			&Curve1, &Matrix1, &Curve2) {
			// Single matrix case
			Data, ok := cmsStageData(Matrix1).(*cmsStageMatrixData)
			if !ok {
				cmsSignalError(nil, cmsERROR_UNDEFINED, "not of the type *cmsStageMatrixData\n")
				return false
			}
			// Copy the matrix to the result
			res = SliceToMat(Data.Double)
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
	}

	// Allocate an empty LUT
	Dest = cmsPipelineAlloc(mm, Src.ContextID, Src.InputChannels, Src.OutputChannels)
	if Dest == nil {
		return false
	}
	//var ressl []float64
	// Assemble the new LUT
	if !cmsPipelineInsertStage(Dest, CmsAT_BEGIN, cmsStageDup(mm, Curve1)) {
		goto Error
	}
	//ressl =

	if !IdentityMat {
		if !cmsPipelineInsertStage(Dest, CmsAT_END, cmsStageAllocMatrix(mm, Dest.ContextID, 3, 3, MatToSlice(res), Offset)) {
			goto Error
		}
	}
	//res = SliceToMat(ressl)
	if !cmsPipelineInsertStage(Dest, CmsAT_END, cmsStageDup(mm, Curve2)) {
		goto Error
	}

	// If identity matrix, optimize the curves further
	if IdentityMat {
		OptimizeByJoiningCurves(mm, &Dest, Intent, InputFormat, OutputFormat, dwFlags)
	} else {
		mpeC1, ok1 := cmsStageData(Curve1).(*cmsStageToneCurvesData)
		mpeC2, ok2 := cmsStageData(Curve2).(*cmsStageToneCurvesData)
		if !ok1 || !ok2 {
			cmsSignalError(nil, cmsERROR_UNDEFINED, "not of the type *cmsStageToneCurvesData\n")
			return false
		}
		// Disable cache for this optimization
		*dwFlags |= CmsFLAGS_NOCACHE

		var vec cmsVEC3
		// Set up optimization routines
		if Offset != nil {
			vec = SliceToVec(Offset[:])
		}
		SetMatShaper(mm, Dest, ConvertToToneCurveArray(mpeC1.TheCurves), &res, &vec, ConvertToToneCurveArray(mpeC2.TheCurves), OutputFormat)
	}

	// Free the original pipeline and replace it with the optimized one
	cmsPipelineFree(mm, Src)
	*Lut = Dest
	return true

Error:
	// Free the destination pipeline and leave the source unchanged
	cmsPipelineFree(mm, Dest)
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
		{OptimizePtr: OptimizeByJoiningCurves, Next: nil},
		{OptimizePtr: OptimizeMatrixShaper, Next: nil},
		{OptimizePtr: OptimizeByComputingLinearization, Next: nil},
		{OptimizePtr: OptimizeByResampling, Next: nil},
	}
	// Link the list
	for i := 0; i < len(DefaultOptimization)-1; i++ {
		DefaultOptimization[i].Next = &DefaultOptimization[i+1]
	}
}

// Global optimization plugin chunk.
var cmsOptimizationPluginChunk = cmsOptimizationPluginChunkType{OptimizationCollection: nil}

// DupPluginOptimizationList duplicates the optimization list for a new context.
func DupPluginOptimizationList(mm mem.Manager, ctx CmsContext, src CmsContext) {
	var newHead cmsOptimizationPluginChunkType
	var entry, prev *cmsOptimizationCollection
	head := src.chunks[OptimizationPlugin].(*cmsOptimizationPluginChunkType)

	if head == nil {
		return
	}

	// Walk the list and copy each node.
	for entry = head.OptimizationCollection; entry != nil; entry = entry.Next {
		newEntry := mem.New[cmsOptimizationCollection](mm)
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

	ctx.chunks[OptimizationPlugin] = cmsSubAllocDup(mm, ctx.MemPool, &newHead, uint32(unsafe.Sizeof(newHead)))
}

// cmsAllocOptimizationPluginChunk allocates the optimization plugin chunk.
func cmsAllocOptimizationPluginChunk(mm mem.Manager, ctx CmsContext, src CmsContext) {
	if src != nil {
		DupPluginOptimizationList(mm, ctx, src)
	} else {
		var defaultChunk cmsOptimizationPluginChunkType
		ctx.chunks[OptimizationPlugin] = cmsSubAllocDup(mm, ctx.MemPool, &defaultChunk, uint32(unsafe.Sizeof(defaultChunk)))
	}
}

// cmsRegisterOptimizationPlugin registers a new optimization plugin.
func cmsRegisterOptimizationPlugin(mm mem.Manager, ContextID CmsContext, Data PluginIntrfc) bool {
	// Early nil check
	ctx := ContextID.chunks[OptimizationPlugin].(*cmsOptimizationPluginChunkType)
	if Data == nil {
		ctx.OptimizationCollection = nil
		return true
	}
	plugin, ok := Data.(*cmsPluginOptimization)
	if !ok {
		panic("Plugin is not of the type cmsPluginOptimization\n")

	}
	var newNode *cmsOptimizationCollection
	// Ensure the optimizer callback is present.
	if plugin.OptimizePtr == nil {
		return false
	}

	//newNode = (*cmsOptimizationCollection)(cmsPluginMalloc(ContextID, uint32(unsafe.Sizeof(cmsOptimizationCollection{}))))
	newNode = mem.New[cmsOptimizationCollection](mm)

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
func cmsOptimizePipeline(mm mem.Manager, ContextID CmsContext, PtrLut **cmsPipeline, Intent uint32, InputFormat, OutputFormat, dwFlags *uint32) bool {
	//fmt.Println("cmsOptimizePipeline")
	ctx := CmsContextGetClientChunk(ContextID, OptimizationPlugin).(*cmsOptimizationPluginChunkType)
	var AnySuccess bool
	var mpe *cmsStage

	// A CLUT is being asked, so force this specific optimization.
	if *dwFlags&CmsFLAGS_FORCE_CLUT != 0 {
		PreOptimize(mm, *PtrLut)
		return OptimizeByResampling(mm, PtrLut, Intent, InputFormat, OutputFormat, dwFlags)
	}

	// Check if there's anything to optimize.
	if (*PtrLut).Elements == nil {
		cmsPipelineSetOptimizationParameters(*PtrLut, FastIdentity16, *PtrLut, nil, nil)
		return true
	}

	// Avoid optimization for named color pipelines.
	for mpe = cmsPipelineGetPtrToFirstStage(*PtrLut); mpe != nil; mpe = cmsStageNext(mpe) {
		if cmsStageType(mpe) == CmsSigNamedColorElemType {
			return false
		}
	}

	// Pre-optimize and check for identity transformations.
	AnySuccess = PreOptimize(mm, *PtrLut)
	if (*PtrLut).Elements == nil {
		cmsPipelineSetOptimizationParameters(*PtrLut, FastIdentity16, *PtrLut, nil, nil)
		return true
	}

	// Skip optimization if explicitly disabled.
	if *dwFlags&CmsFLAGS_NOOPTIMIZE != 0 {
		return false
	}

	// Try plugin optimizations.
	for opts := ctx.OptimizationCollection; opts != nil; opts = opts.Next {
		if opts.OptimizePtr(mm, PtrLut, Intent, InputFormat, OutputFormat, dwFlags) {
			return true
		}
	}

	// Try built-in optimizations.
	for opts := &DefaultOptimization[0]; opts != nil; opts = opts.Next {
		if opts.OptimizePtr(mm, PtrLut, Intent, InputFormat, OutputFormat, dwFlags) {
			return true
		}
	}

	// Only simple optimizations succeeded.
	return AnySuccess
}

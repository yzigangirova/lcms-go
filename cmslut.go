package golcms

import (
	//"fmt"
	"math"
	"sync"

	"github.com/yzigangirova/lcms-go/mem"
)

var lutBufferPool = sync.Pool{
	New: func() any {
		// Allocate once
		return new([2][MAX_STAGE_CHANNELS]float32)
	},
}

var in16Pool = sync.Pool{
	New: func() any {
		return new([MAX_STAGE_CHANNELS]uint16)
	},
}

var out16Pool = sync.Pool{
	New: func() any {
		return new([MAX_STAGE_CHANNELS]uint16)
	},
}

func cmsStageAllocPlaceholder(mm mem.Manager,
	ContextID CmsContext,
	Type cmsStageSignature,
	InputChannels, OutputChannels uint32,
	EvalPtr cmsStageEvalFn,
	DupElemPtr cmsStageDupElemFn,
	FreePtr cmsStageFreeElemFn,
	Data any,
) *cmsStage {
	// Allocate memory for cmsStage and initialize to zero
	ph := mem.New[cmsStage](mm)

	if ph == nil {
		return nil
	}

	// Initialize the cmsStage fields
	ph.ContextID = ContextID
	ph.Type = Type
	ph.Implements = Type // Default implementation matches the type
	ph.InputChannels = InputChannels
	ph.OutputChannels = OutputChannels
	ph.EvalPtr = EvalPtr
	ph.DupElemPtr = DupElemPtr
	ph.FreePtr = FreePtr
	ph.Data = Data

	return ph
}

func EvaluateIdentity(mm mem.Manager, In []float32, Out []float32, mpe *cmsStage) {
	MemmoveSlice(Out, In, int(mpe.InputChannels))
}
func cmsStageAllocIdentity(mm mem.Manager, ContextID CmsContext, nChannels uint32) *cmsStage {
	return cmsStageAllocPlaceholder(mm, ContextID,
		CmsSigIdentityElemType,
		nChannels, nChannels,
		EvaluateIdentity,
		nil,
		nil,
		nil)
}

// FromFloatTo16 converts a slice of float32 values to a slice of uint16 values
// FromFloatTo16 converts a slice of float32 values to a slice of uint16 values using unsafe pointer arithmetic.
func FromFloatTo16(In []float32, Out []uint16, n uint32) {
	for i := uint32(0); i < n; i++ {
		// Perform the conversion
		Out[i] = cmsQuickSaturateWord(float64(In[i] * 65535.0))
	}
}

// From16ToFloat converts a slice of uint16 values to a slice of float32 values
// From16ToFloat converts a slice of uint16 values to a slice of float32 values using unsafe pointer arithmetic.
func From16ToFloat(In []uint16, Out []float32, n uint32) {
	for i := uint32(0); i < n; i++ {
		// Perform the conversion
		Out[i] = float32(In[i]) / 65535.0
	}
}

func cmsPipelineCheckAndRetrieveStages(lut *cmsPipeline, n uint32, expectedTypes []cmsStageSignature, retrievedStages ...**cmsStage) bool {
	//	fmt.Println("cmsPipelineCheckAndRetrieveStages")
	// Ensure the number of stages matches
	if cmsPipelineStageCount(lut) != n {
		return false
	}

	// Validate the expected types slice matches the provided number of stages
	if uint32(len(expectedTypes)) != n || uint32(len(retrievedStages)) != n {
		return false
	}

	// Iterate over the stages and match types
	mpe := lut.Elements
	for i := uint32(0); i < n; i++ {
		if mpe == nil || mpe.Type != expectedTypes[i] {
			// Mismatch found; return false
			return false
		}
		mpe = mpe.Next
	}

	// Fill the retrieved stages pointers
	mpe = lut.Elements
	for i := uint32(0); i < n; i++ {
		if retrievedStages[i] != nil {
			*retrievedStages[i] = mpe
		}
		mpe = mpe.Next
	}

	return true
}
func Clipper(mm mem.Manager, In []float32, Out []float32, mpe *cmsStage) {
	for i := uint32(0); i < mpe.InputChannels; i++ {
		// Access In and Out using unsafe.Pointer arithmetic
		inVal := In[i]

		// Perform clipping
		if inVal < 0 {
			Out[i] = 0
		} else {
			Out[i] = inVal
		}
	}
}

func cmsStageClipNegatives(mm mem.Manager, ContextID CmsContext, nChannels uint32) *cmsStage {
	return cmsStageAllocPlaceholder(mm,
		ContextID,
		CmsSigClipNegativesElemType,
		nChannels,
		nChannels,
		Clipper,
		nil,
		nil,
		nil,
	)
}

func cmsStageGetPtrToCurveSet(mpe *cmsStage) []*CmsToneCurve {
	data, ok := mpe.Data.(*cmsStageToneCurvesData)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsToneCurve\n")
		return nil
	}
	return data.TheCurves
}

func EvaluateCurves(mm mem.Manager, In []float32, Out []float32, mpe *cmsStage) {
	//fmt.Println("   START EvaluateCurves In ", In[0], In[1], In[2], In[3])

	data, ok := mpe.Data.(*cmsStageToneCurvesData)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageToneCurvesData\n")
		return
	}
	if data == nil || data.TheCurves == nil {
		return
	}
	//fmt.Printf("start EVALUATE CURVES - data.NCurves %d\n", data.NCurves)
	//	fmt.Println("333 Whole infloat ", In)
	/*const eps = 1e-7
	patchInput := func(v float32) float32 {
		if math.Abs(float64(v)-0.9539864) < eps {
			return 0.954780
		}
		if math.Abs(float64(v)-0.5015101) < eps {
			return 0.501518
		}
		if math.Abs(float64(v)-0.50021994) < eps {
			return 0.500251
		}
		return v
	}*/

	for i := uint32(0); i < data.NCurves; i++ {
		//patchedValue := patchInput(In[i])
		Out[i] = cmsEvalToneCurveFloat(data.TheCurves[i], In[i])
	}

	// Debug: Print final output values
	//fmt.Println("end EVALUATE CURVES")

}

func CurveSetElemTypeFree(mm mem.Manager, mpe *cmsStage) {
	cmsAssert(mpe != nil, "")

	data, ok := mpe.Data.(*cmsStageToneCurvesData)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageToneCurvesData\n")
		return
	}
	if data == nil {
		return
	}

	if data.TheCurves != nil {
		for i := uint32(0); i < data.NCurves; i++ {
			if data.TheCurves[i] != nil {
				CmsFreeToneCurve(data.TheCurves[i])
			}
		}
	}
	cmsFree(mpe.ContextID, data)
}
func CurveSetDup(mm mem.Manager, mpe *cmsStage) any {
	//	fmt.Println("CurveSetDup")
	// Access the data from the input stage
	data, ok := mpe.Data.(*cmsStageToneCurvesData)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageToneCurvesData\n")
		return nil
	}
	// Allocate memory for the new tone curves data structure
	newElem := mem.New[cmsStageToneCurvesData](mm)
	if newElem == nil {
		return nil
	}
	// Set the number of curves
	newElem.NCurves = data.NCurves

	// Allocate memory for the array of tone curve pointers
	newElem.TheCurves = mem.MakeSlice[*CmsToneCurve](mm, int(newElem.NCurves))

	for i := uint32(0); i < newElem.NCurves; i++ {
		// Duplicate each curve. It may fail.
		newElem.TheCurves[i] = cmsDupToneCurve(mm, data.TheCurves[i])
		if newElem.TheCurves[i] == nil {
			goto Error
		}

	}

	return newElem

Error:
	// Cleanup allocated memory in case of an error
	for i := uint32(0); i < newElem.NCurves; i++ {
		if newElem.TheCurves[i] != nil {
			CmsFreeToneCurve(newElem.TheCurves[i])
		}
	}
	cmsFree(mpe.ContextID, newElem)
	return nil
}

func cmsStageAllocToneCurves(mm mem.Manager, ContextID CmsContext, nChannels uint32, Curves []*CmsToneCurve) *cmsStage {
	//fmt.Println("start cmsStageAllocToneCurves")
	// Allocate the placeholder for the stage
	newMPE := cmsStageAllocPlaceholder(mm, ContextID, CmsSigCurveSetElemType, nChannels, nChannels, EvaluateCurves, CurveSetDup, CurveSetElemTypeFree, nil)
	if newMPE == nil {
		return nil
	}

	// Allocate the tone curves data structure
	newElem := mem.New[cmsStageToneCurvesData](mm)
	if newElem == nil {
		cmsStageFree(mm, newMPE)
		return nil
	}

	newMPE.Data = newElem

	// Set the number of curves
	newElem.NCurves = nChannels

	// Allocate memory for the slice of tone curve pointers
	// Allocate memory for the array of tone curve pointers
	newElem.TheCurves = mem.MakeSlice[*CmsToneCurve](mm, int(newElem.NCurves))

	// Handle the input curves, either creating identity curves or duplicating existing ones
	for i := uint32(0); i < nChannels; i++ {
		if Curves == nil {
			// Assign a new tone curve if Curves is nil
			newElem.TheCurves[i] = CmsBuildGamma(mm, ContextID, 1.0)
		} else {
			// Duplicate the tone curve and assign it to NewElem.TheCurves
			newElem.TheCurves[i] = cmsDupToneCurve(mm, Curves[i])
		}

		// Check if the assignment failed
		if newElem.TheCurves[i] == nil {
			cmsStageFree(mm, newMPE)
			return nil
		}
	}
	//fmt.Println("end cmsStageAllocToneCurves")

	return newMPE

}

func cmsStageAllocIdentityCurves(mm mem.Manager, ContextID CmsContext, nChannels uint32) *cmsStage {
	mpe := cmsStageAllocToneCurves(mm, ContextID, nChannels, nil)
	if mpe == nil {
		return nil
	}
	mpe.Implements = CmsSigIdentityElemType
	return mpe
}

func EvaluateMatrix(mm mem.Manager, in []float32, out []float32, mpe *cmsStage) {
	//fmt.Printf("start EvaluateMatrix %.7f  %.7f  %.7f  %.7f \n", in[0], in[1], in[2], in[3])

	data, ok := mpe.Data.(*cmsStageMatrixData)
	if !ok {
		panic("[EvaluateMatrix] ERROR: Data is not *cmsStageMatrixData")
	}

	/*	fmt.Printf("[EvaluateMatrix] data ptr: %p\n", data)
		fmt.Printf("[EvaluateMatrix] data.Double: %v\n", data.Double)
		if data.Offset != nil {
			fmt.Printf("[EvaluateMatrix] data.Offset: %v\n", data.Offset)
		} else {
			fmt.Println("[EvaluateMatrix] data.Offset: nil")
		}

		fmt.Printf("[EvaluateMatrix] Input Channels: %d, Output Channels: %d\n", mpe.InputChannels, mpe.OutputChannels)

		fmt.Print("[EvaluateMatrix] Input values: ")
		for i := uint32(0); i < mpe.InputChannels; i++ {
			fmt.Printf("%.5f ", in[i])
		}
		fmt.Println()*/

	for i := uint32(0); i < mpe.OutputChannels; i++ {
		var tmp float64
		for j := uint32(0); j < mpe.InputChannels; j++ {
			tmp += float64(in[j]) * data.Double[i*mpe.InputChannels+j]
		}
		if data.Offset != nil {
			tmp += data.Offset[i]
		}
		out[i] = float32(tmp)
		//fmt.Printf("[EvaluateMatrix] out[%d] = %.10f\n", i, out[i])
	}

	//fmt.Printf("end EvaluateMatrix %.7f  %.7f  %.7f  %.7f \n", out[0], out[1], out[2], out[3])
}

// MatrixElemDup duplicates the matrix stage data.
func MatrixElemDup(mm mem.Manager, mpe *cmsStage) any {
	//	fmt.Println("MatrixElemDup")

	if mpe == nil || mpe.Data == nil {
		return nil
	}

	Data, ok := mpe.Data.(*cmsStageMatrixData)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageMatrixData\n")
		return nil
	}

	NewElem := mem.New[cmsStageMatrixData](mm)

	NewElem.Double = cmsDupMemSlice(Data.Double)
	if Data.Offset != nil {
		NewElem.Offset = cmsDupMemSlice(Data.Offset)
	}

	return NewElem
}

// MatrixElemTypeFree frees the matrix stage data.
func MatrixElemTypeFree(mm mem.Manager, mpe *cmsStage) {
	if mpe == nil || mpe.Data == nil {
		return
	}

	Data, ok := mpe.Data.(*cmsStageMatrixData)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageMatrixData\n")
		return
	}
	if Data == nil {
		return
	}
	cmsFree(mpe.ContextID, mpe.Data)
}

// USE ARENA!!!
func cmsStageAllocMatrix(mm mem.Manager,
	ContextID CmsContext,
	Rows, Cols uint32,
	Matrix, Offset []float64,
) *cmsStage {
	//	fmt.Println("[cmsStageAllocMatrix] START")
	/*	fmt.Printf("  Input Rows: %d, Cols: %d\n", Rows, Cols)
		fmt.Printf("  Input Matrix: %v\n", Matrix)
		fmt.Printf("  Input Offset: %v\n", Offset)*/

	n := Rows * Cols
	if n == 0 || n >= math.MaxUint32/Cols || n >= math.MaxUint32/Rows || n < Rows || n < Cols {
		//		fmt.Println("[cmsStageAllocMatrix] Invalid matrix size, aborting")
		return nil
	}

	NewMPE := cmsStageAllocPlaceholder(mm,
		ContextID,
		CmsSigMatrixElemType,
		Cols,
		Rows,
		EvaluateMatrix,
		MatrixElemDup,
		MatrixElemTypeFree,
		nil,
	)

	if NewMPE == nil {
		//		fmt.Println("[cmsStageAllocMatrix] Failed to allocate NewMPE")
		return nil
	}

	NewElem := mem.New[cmsStageMatrixData](mm)
	if NewElem == nil {
		//		fmt.Println("[cmsStageAllocMatrix] Failed to allocate NewElem, cleaning up")
		cmsStageFree(mm, NewMPE)
		return nil
	}

	NewElem.Double = mem.MakeSlice[float64](mm, int(n))
	copy(NewElem.Double, Matrix)

	if Offset != nil {
		NewElem.Offset = mem.MakeSlice[float64](mm, int(Rows))
		copy(NewElem.Offset, Offset)
	}

	NewMPE.Data = NewElem

	//	fmt.Println("[cmsStageAllocMatrix] END")
	return NewMPE
}

func EvaluateXYZ2Lab(mm mem.Manager, In []float32, Out []float32, mpe *cmsStage) {
	//fmt.Printf("start EvaluateXYZ2Lab %.7f  %.7f  %.7f  %.7f \n", In[0], In[1], In[2], In[3])
	const XYZadj = MAX_ENCODEABLE_XYZ

	var XYZ cmsCIEXYZ
	var Lab cmsCIELab

	XYZ.X = float64(In[0]) * XYZadj
	XYZ.Y = float64(In[1]) * XYZadj
	XYZ.Z = float64(In[2]) * XYZadj

	// Convert XYZ to Lab
	cmsXYZ2Lab(nil, &Lab, &XYZ)

	// From V4 Lab to 0..1.0
	Out[0] = float32(Lab.L / 100.0)
	Out[1] = float32((Lab.a + 128.0) / 255.0)
	Out[2] = float32((Lab.b + 128.0) / 255.0)
	//fmt.Printf("end EvaluateXYZ2Lab %.7f  %.7f  %.7f  %.7f \n", Out[0], Out[1], Out[2], Out[3])

}

func cmsStageAllocXYZ2Lab(mm mem.Manager, ContextID CmsContext) *cmsStage {
	return cmsStageAllocPlaceholder(mm, ContextID, CmsSigXYZ2LabElemType, 3, 3, EvaluateXYZ2Lab, nil, nil, nil)
}

// This routine does a sweep on whole input space, and calls its callback
// function on knots. returns TRUE if all ok, FALSE otherwise.

func cmsSliceSpace16(mm mem.Manager, nInputs uint32, clutPoints []uint32, Sampler cmsSAMPLER16, cargo any) bool {
	if nInputs >= cmsMAXCHANNELS {
		return false
	}
	var rest int
	var In [cmsMAXCHANNELS]uint16

	nTotalPoints := CubeSize(clutPoints, nInputs)
	if nTotalPoints == 0 {
		return false
	}

	for t := int(nInputs) - 1; t >= 0; t-- {
		Colorant := uint32(rest % int(clutPoints[t]))
		rest /= int(clutPoints[t])

		// Assign quantized value to the input array
		In[t] = cmsQuantizeVal(float64(Colorant), clutPoints[t])
	}

	// Call the sampler with the current input
	if Sampler(mm, In[:], nil, cargo) != 1 {
		return false
	}
	return true
}
func cmsSliceSpaceFloat(mm mem.Manager, nInputs uint32, clutPoints []uint32, Sampler cmsSAMPLERFLOAT, cargo any) int32 {
	if nInputs >= cmsMAXCHANNELS {
		return 0 // FALSE
	}

	nTotalPoints := CubeSize(clutPoints, nInputs)
	if nTotalPoints == 0 {
		return 0 // FALSE
	}
	var In [cmsMAXCHANNELS]float32

	for i := 0; i < int(nTotalPoints); i++ {
		rest := i

		for t := int(nInputs) - 1; t >= 0; t-- {
			Colorant := rest % int(clutPoints[t])
			rest /= int(clutPoints[t])

			// Assign quantized value, scaled to 0.0–1.0 range
			In[t] = float32(cmsQuantizeVal(float64(Colorant), clutPoints[t])) / 65535.0
		}

		// Call the sampler with the current input
		if Sampler(mm, In[:], nil, cargo) != 1 {
			return 0 // FALSE
		}
	}

	return 1 // TRUE
}

// ********************************************************************************
// Type CmsSigLab2XYZElemType
// ********************************************************************************

func EvaluateLab2XYZ(mm mem.Manager, In []float32, Out []float32, mpe *cmsStage) {
	//fmt.Println("start EvaluateLab2XYZ")
	const XYZadj = MAX_ENCODEABLE_XYZ

	var XYZ cmsCIEXYZ
	var Lab cmsCIELab

	// V4 rules
	Lab.L = float64(In[0] * 100.0)
	Lab.a = float64(In[1]*255.0 - 128.0)
	Lab.b = float64(In[2]*255.0 - 128.0)

	cmsLab2XYZ(nil, &XYZ, &Lab)

	// From XYZ, range 0..19997 to 0..1.0, note that 1.99997 comes from 0xffff
	// encoded as 1.15 fixed point, so 1 + (32767.0 / 32768.0)

	Out[0] = float32(XYZ.X / XYZadj)
	Out[1] = float32(XYZ.Y / XYZadj)
	Out[2] = float32(XYZ.Z / XYZadj)
	//fmt.Println("end EvaluateLab2XYZ")

}

// No dup or free routines needed, as the structure has no pointers in it.
func cmsStageAllocLab2XYZ(mm mem.Manager, ContextID CmsContext) *cmsStage {
	return cmsStageAllocPlaceholder(mm, ContextID, CmsSigLab2XYZElemType, 3, 3, EvaluateLab2XYZ, nil, nil, nil)
}

// ********************************************************************************

// v2 L=100 is supposed to be placed on 0xFF00. There is no reasonable
// number of gridpoints that would make exact match. However, a prelinearization
// of 258 entries would map 0xFF00 exactly on entry 257, and this is good to avoid scum dot.
// Almost all what we need, but unfortunately, the rest of entries should be scaled by
// (255*257/256), and this is not exact.

func cmsStageAllocLabV2ToV4curves(mm mem.Manager, ContextID CmsContext) *cmsStage {
	var LabTable [3]*CmsToneCurve
	var mpe *cmsStage
	var i, j int

	// Build 258-entry tone curves for Lab components
	LabTable[0] = cmsBuildTabulatedToneCurve16(mm, ContextID, 258, nil)
	LabTable[1] = cmsBuildTabulatedToneCurve16(mm, ContextID, 258, nil)
	LabTable[2] = cmsBuildTabulatedToneCurve16(mm, ContextID, 258, nil)

	// Ensure all tone curves were created successfully
	for j = 0; j < 3; j++ {
		if LabTable[j] == nil {
			cmsFreeToneCurveTriple(LabTable)
			return nil
		}
		// Populate tone curve entries
		// We need to map * (0xffff / 0xff00), that's same as (257 / 256)
		// So we can use 258-entry tables to do the trick:
		// (i / 257) * (255 * 257) * (257 / 256)
		for i = 0; i < 257; i++ {
			LabTable[j].Table16[i] = uint16((i*0xffff + 0x80) >> 8)
		}

		// Set the last entry to 0xffff
		LabTable[j].Table16[257] = 0xffff
	}

	// Allocate the tone curve stage
	mpe = cmsStageAllocToneCurves(mm, ContextID, 3, LabTable[:])
	cmsFreeToneCurveTriple(LabTable)

	// Check if allocation was successful
	if mpe == nil {
		return nil
	}

	// Set the implementation signature
	mpe.Implements = cmsStageSignature(CmsSigLabV2toV4)
	return mpe
}

// _cmsStageAllocLabV2ToV4 allocates a matrix-based stage for Lab v2 to v4 conversion.
func cmsStageAllocLabV2ToV4(mm mem.Manager, ContextID CmsContext) *cmsStage {
	//fmt.Println("cmsStageAllocLabV2ToV4")
	var v2ToV4 = []float64{
		65535.0 / 65280.0, 0, 0,
		0, 65535.0 / 65280.0, 0,
		0, 0, 65535.0 / 65280.0,
	}

	mpe := cmsStageAllocMatrix(mm, ContextID, 3, 3, v2ToV4, nil)
	if mpe == nil {
		return nil
	}
	mpe.Implements = cmsStageSignature(CmsSigLabV2toV4)
	return mpe
}

// _cmsStageAllocLabV4ToV2 allocates a matrix-based stage for Lab v4 to v2 conversion.
func cmsStageAllocLabV4ToV2(mm mem.Manager, ContextID CmsContext) *cmsStage {
	var v4ToV2 = []float64{
		65280.0 / 65535.0, 0, 0,
		0, 65280.0 / 65535.0, 0,
		0, 0, 65280.0 / 65535.0,
	}

	mpe := cmsStageAllocMatrix(mm, ContextID, 3, 3, v4ToV2, nil)
	if mpe == nil {
		return nil
	}
	mpe.Implements = cmsStageSignature(CmsSigLabV4toV2)
	return mpe
}

// Constants for normalization
const (
	normFactorXYZToFloat = 32768.0 / 65535.0
	normFactorFloatToXYZ = 65535.0 / 32768.0
)

// _cmsStageNormalizeFromLabFloat normalizes Lab values from integer range to floating-point PCS range.
func cmsStageNormalizeFromLabFloat(mm mem.Manager, ContextID CmsContext) *cmsStage {
	a1 := []float64{
		1.0 / 100.0, 0, 0,
		0, 1.0 / 255.0, 0,
		0, 0, 1.0 / 255.0,
	}

	o1 := []float64{
		0,
		128.0 / 255.0,
		128.0 / 255.0,
	}

	mpe := cmsStageAllocMatrix(mm, ContextID, 3, 3, a1, o1)
	if mpe == nil {
		return nil
	}
	mpe.Implements = CmsSigLab2FloatPCS
	return mpe
}

// _cmsStageNormalizeFromXyzFloat normalizes XYZ values from integer range to floating-point PCS range.
func cmsStageNormalizeFromXyzFloat(mm mem.Manager, ContextID CmsContext) *cmsStage {
	a1 := []float64{
		normFactorXYZToFloat, 0, 0,
		0, normFactorXYZToFloat, 0,
		0, 0, normFactorXYZToFloat,
	}

	mpe := cmsStageAllocMatrix(mm, ContextID, 3, 3, a1, nil)
	if mpe == nil {
		return nil
	}
	mpe.Implements = CmsSigXYZ2FloatPCS
	return mpe
}

// _cmsStageNormalizeToLabFloat normalizes Lab values from floating-point PCS range to integer range.
func cmsStageNormalizeToLabFloat(mm mem.Manager, ContextID CmsContext) *cmsStage {
	a1 := []float64{
		100.0, 0, 0,
		0, 255.0, 0,
		0, 0, 255.0,
	}

	o1 := []float64{
		0,
		-128.0,
		-128.0,
	}

	mpe := cmsStageAllocMatrix(mm, ContextID, 3, 3, a1, o1)
	if mpe == nil {
		return nil
	}
	mpe.Implements = CmsSigFloatPCS2Lab
	return mpe
}

// _cmsStageNormalizeToXyzFloat normalizes XYZ values from floating-point PCS range to integer range.
func cmsStageNormalizeToXyzFloat(mm mem.Manager, ContextID CmsContext) *cmsStage {
	a1 := []float64{
		normFactorFloatToXYZ, 0, 0,
		0, normFactorFloatToXYZ, 0,
		0, 0, normFactorFloatToXYZ,
	}

	mpe := cmsStageAllocMatrix(mm, ContextID, 3, 3, a1, nil)
	if mpe == nil {
		return nil
	}
	mpe.Implements = CmsSigFloatPCS2XYZ
	return mpe
}

func cmsStageAllocLabPrelin(mm mem.Manager, ContextID CmsContext) *cmsStage {
	params := []float64{2.4}
	var LabTable [3]*CmsToneCurve

	LabTable[0] = CmsBuildGamma(mm, ContextID, 1.0)
	LabTable[1] = cmsBuildParametricToneCurve(mm, ContextID, 108, params)
	LabTable[2] = cmsBuildParametricToneCurve(mm, ContextID, 108, params)

	return cmsStageAllocToneCurves(mm, ContextID, 3, LabTable[:])
}
func cmsStageFree(mm mem.Manager, mpe *cmsStage) {
	if mpe.FreePtr != nil {
		mpe.FreePtr(mm, mpe)
	}
	cmsFree(mpe.ContextID, mpe)
}
func cmsStageInputChannels(mpe *cmsStage) uint32 {
	return mpe.InputChannels
}
func cmsStageOutputChannels(mpe *cmsStage) uint32 {
	return mpe.OutputChannels
}
func cmsStageType(mpe *cmsStage) cmsStageSignature {
	return mpe.Type
}
func cmsStageData(mpe *cmsStage) any {
	return mpe.Data
}
func cmsGetStageContextID(mpe *cmsStage) CmsContext {
	return mpe.ContextID
}
func cmsStageNext(mpe *cmsStage) *cmsStage {
	return mpe.Next
}
func cmsStageDup(mm mem.Manager, mpe *cmsStage) *cmsStage {
	//	fmt.Println("cmsStageDup")

	if mpe == nil {
		return nil
	}

	NewMPE := cmsStageAllocPlaceholder(mm,
		mpe.ContextID,
		mpe.Type,
		mpe.InputChannels,
		mpe.OutputChannels,
		mpe.EvalPtr,
		mpe.DupElemPtr,
		mpe.FreePtr,
		nil,
	)

	if NewMPE == nil {
		return nil
	}

	NewMPE.Implements = mpe.Implements

	if mpe.DupElemPtr != nil {
		NewMPE.Data = mpe.DupElemPtr(mm, mpe)
		if NewMPE.Data == nil {
			cmsStageFree(mm, NewMPE)
			return nil
		}
	} else {
		NewMPE.Data = nil
	}

	return NewMPE
}

// BlessLUT sets up the channel count and ensures consistency across stages.
func BlessLUT(lut *cmsPipeline) bool {
	//fmt.Println("start BlessLUT")
	// We can set the input/output channels only if we have elements.
	if lut.Elements != nil {
		var prev, next *cmsStage
		var first, last *cmsStage
		first = cmsPipelineGetPtrToFirstStage(lut)
		last = cmsPipelineGetPtrToLastStage(lut)

		if first == nil || last == nil {
			return false
		}

		lut.InputChannels = first.InputChannels
		lut.OutputChannels = last.OutputChannels

		// Check chain consistency
		prev = first
		next = prev.Next

		for next != nil {
			if next.InputChannels != prev.OutputChannels {
				return false
			}

			next = next.Next
			prev = prev.Next
		}
	}
	//fmt.Println("end BlessLUT")

	return true
}

// _LUTeval16 evaluates the LUT on a 16-bit basis
func LUTeval16(mm mem.Manager, In []uint16, Out []uint16, D any) {
	lut, ok := D.(*cmsPipeline)
	if !ok {
		panic(" D  must be of type *cmsPipeline")
	}
	var Storage [2][MAX_STAGE_CHANNELS]float32
	var Phase, NextPhase int

	// Convert input from 16-bit to float
	From16ToFloat(In, Storage[Phase][:], lut.InputChannels)

	// Process each stage in the pipeline
	for mpe := lut.Elements; mpe != nil; mpe = mpe.Next {
		NextPhase = Phase ^ 1
		mpe.EvalPtr(mm, Storage[Phase][:], Storage[NextPhase][:], mpe)
		Phase = NextPhase
	}

	// Convert output from float to 16-bit
	FromFloatTo16(Storage[Phase][:], Out, lut.OutputChannels)
}

func LUTevalFloat(mm mem.Manager, In []float32, Out []float32, D any) {
	lut, ok := D.(*cmsPipeline)
	if !ok {
		panic(" D must be of type *cmsPipeline")
	}

	storagePtr := lutBufferPool.Get().(*[2][MAX_STAGE_CHANNELS]float32)
	defer lutBufferPool.Put(storagePtr) // reuse for next call

	// Work with pointer directly, no copying
	var Phase, NextPhase int
	MemmoveSlice(storagePtr[Phase][:], In, int(lut.InputChannels))

	for mpe := lut.Elements; mpe != nil; mpe = mpe.Next {
		NextPhase = Phase ^ 1
		mpe.EvalPtr(mm, storagePtr[Phase][:], storagePtr[NextPhase][:], mpe)
		Phase = NextPhase
	}

	MemmoveSlice(Out, storagePtr[Phase][:], int(lut.OutputChannels))
}

// cmsPipelineAlloc allocates and initializes a new LUT pipeline
func cmsPipelineAlloc(mm mem.Manager, contextID CmsContext, inputChannels, outputChannels uint32) *cmsPipeline {
	// A value of zero in channels is allowed as a placeholder
	if inputChannels >= cmsMAXCHANNELS || outputChannels >= cmsMAXCHANNELS {
		return nil
	}

	// Allocate memory for the cmsPipeline struct
	newLUT := mem.New[cmsPipeline](mm)

	// Initialize the LUT structure
	newLUT.InputChannels = inputChannels
	newLUT.OutputChannels = outputChannels
	newLUT.Eval16Fn = LUTeval16
	newLUT.EvalFloatFn = LUTevalFloat
	newLUT.DupDataFn = nil
	newLUT.FreeDataFn = nil
	newLUT.Data = newLUT
	newLUT.ContextID = contextID

	// Validate the LUT
	if !BlessLUT(newLUT) {
		return nil
	}

	return newLUT
}

func cmsGetPipelineContextID(lut *cmsPipeline) CmsContext {
	cmsAssert(lut != nil, "lut is nil")
	return lut.ContextID
}
func cmsPipelineInputChannels(lut *cmsPipeline) uint32 {
	cmsAssert(lut != nil, "lut is nil")

	return lut.InputChannels
}
func cmsPipelineOutputChannels(lut *cmsPipeline) uint32 {
	cmsAssert(lut != nil, "lut is nil")

	return lut.OutputChannels
}
func cmsPipelineFree(mm mem.Manager, lut *cmsPipeline) {
	if lut == nil {
		return
	}

	var next *cmsStage
	for mpe := lut.Elements; mpe != nil; mpe = next {
		next = mpe.Next
		cmsStageFree(mm, mpe)
	}

	if lut.FreeDataFn != nil {
		lut.FreeDataFn(lut.ContextID, lut.Data)
	}

	cmsFree(lut.ContextID, lut)
}
func cmsPipelineEval16(mm mem.Manager, In []uint16, Out []uint16, lut *cmsPipeline) {
	cmsAssert(lut != nil, "lut is nil")

	lut.Eval16Fn(mm, In, Out, lut.Data)
}
func cmsPipelineEvalFloat(mm mem.Manager, In []float32, Out []float32, lut *cmsPipeline) {
	cmsAssert(lut != nil, "lut is nil")

	/*fmt.Printf("In[0] %.7f\n", In[0])
	fmt.Printf("In[1] %.7f\n", In[1])
	fmt.Printf("In[2] %.7f\n", In[2])*/

	lut.EvalFloatFn(mm, In, Out, lut)
}
func cmsPipelineDup(mm mem.Manager, lut *cmsPipeline) *cmsPipeline {
	//	fmt.Println("cmsPipelineDup")
	if lut == nil {
		return nil
	}

	NewLUT := cmsPipelineAlloc(mm, lut.ContextID, lut.InputChannels, lut.OutputChannels)
	if NewLUT == nil {
		return nil
	}

	var anterior *cmsStage
	first := true

	for mpe := lut.Elements; mpe != nil; mpe = mpe.Next {
		NewMPE := cmsStageDup(mm, mpe)
		if NewMPE == nil {
			cmsPipelineFree(mm, NewLUT)
			return nil
		}

		if first {
			NewLUT.Elements = NewMPE
			first = false
		} else if anterior != nil {
			anterior.Next = NewMPE
		}
		anterior = NewMPE
	}

	NewLUT.Eval16Fn = lut.Eval16Fn
	NewLUT.EvalFloatFn = lut.EvalFloatFn
	NewLUT.DupDataFn = lut.DupDataFn
	NewLUT.FreeDataFn = lut.FreeDataFn
	NewLUT.SaveAs8Bits = lut.SaveAs8Bits

	if NewLUT.DupDataFn != nil {
		NewLUT.Data = NewLUT.DupDataFn(lut.ContextID, lut.Data)
	}

	if !BlessLUT(NewLUT) {
		cmsFree(lut.ContextID, NewLUT)
		return nil
	}

	return NewLUT
}
func cmsPipelineInsertStage(lut *cmsPipeline, loc cmsStageLoc, mpe *cmsStage) bool {
	//fmt.Println("cmsPipelineInsertStage")
	if lut == nil || mpe == nil {
		return false
	}

	switch loc {
	case CmsAT_BEGIN:
		mpe.Next = lut.Elements
		lut.Elements = mpe

	case CmsAT_END:
		if lut.Elements == nil {
			lut.Elements = mpe
		} else {
			var anterior *cmsStage
			for pt := lut.Elements; pt != nil; pt = pt.Next {
				anterior = pt
			}
			anterior.Next = mpe
			mpe.Next = nil
		}

	default:
		return false
	}

	return BlessLUT(lut)
}
func cmsPipelineUnlinkStage(mm mem.Manager, lut *cmsPipeline, loc cmsStageLoc, mpe **cmsStage) {
	//	fmt.Println("cmsPipelineUnlinkStage")
	if lut.Elements == nil {
		if mpe != nil {
			*mpe = nil
		}
		return
	}

	var unlinked *cmsStage

	switch loc {
	case CmsAT_BEGIN:
		elem := lut.Elements
		lut.Elements = elem.Next
		elem.Next = nil
		unlinked = elem

	case CmsAT_END:
		var anterior, last *cmsStage
		for pt := lut.Elements; pt != nil; pt = pt.Next {
			anterior = last
			last = pt
		}

		unlinked = last
		if anterior != nil {
			anterior.Next = nil
		} else {
			lut.Elements = nil
		}
	}

	if mpe != nil {
		*mpe = unlinked
	} else {
		cmsStageFree(mm, unlinked)
	}

	BlessLUT(lut)
}
func cmsPipelineCat(mm mem.Manager, l1 *cmsPipeline, l2 *cmsPipeline) bool {
	//	fmt.Println("cmsPipelineCat")
	if l1.Elements == nil && l2.Elements == nil {
		l1.InputChannels = l2.InputChannels
		l1.OutputChannels = l2.OutputChannels
	}

	for mpe := l2.Elements; mpe != nil; mpe = mpe.Next {
		if !cmsPipelineInsertStage(l1, CmsAT_END, cmsStageDup(mm, mpe)) {
			return false
		}
	}

	return BlessLUT(l1)
}

// cmsPipelineSetSaveAs8bitsFlag sets the SaveAs8Bits flag and returns its previous value.
func cmsPipelineSetSaveAs8bitsFlag(lut *cmsPipeline, on bool) bool {
	previous := lut.SaveAs8Bits
	lut.SaveAs8Bits = on
	return previous
}

// cmsPipelineGetPtrToFirstStage returns the first stage in the pipeline.
func cmsPipelineGetPtrToFirstStage(lut *cmsPipeline) *cmsStage {
	return lut.Elements
}

// cmsPipelineGetPtrToLastStage returns the last stage in the pipeline.
func cmsPipelineGetPtrToLastStage(lut *cmsPipeline) *cmsStage {
	var prev *cmsStage
	for stage := lut.Elements; stage != nil; stage = stage.Next {
		prev = stage
	}
	return prev
}

// This function may be used to set the optional evaluator and a block of private data. If private data is being used, an optional
// duplicator and free functions should also be specified in order to duplicate the LUT construct. Use nil to inhibit such functionality.
func cmsPipelineSetOptimizationParameters(Lut *cmsPipeline,
	Eval16 cmsPipelineEval16Fn, PrivateData any,
	FreePrivateDataFn cmsFreeUserDataFn, DupPrivateDataFn cmsDupUserDataFn) {
	Lut.Eval16Fn = Eval16
	Lut.DupDataFn = DupPrivateDataFn
	Lut.FreeDataFn = FreePrivateDataFn
	Lut.Data = PrivateData
}

// cmsPipelineStageCount counts the number of stages in the pipeline.
func cmsPipelineStageCount(lut *cmsPipeline) uint32 {
	var count uint32
	for stage := lut.Elements; stage != nil; stage = stage.Next {
		count++
	}
	return count
}

// ----------------------------------------------------------- Reverse interpolation
// Here's how it goes. The derivative Df(x) of the function f is the linear
// transformation that best approximates f near the point x. It can be represented
// by a matrix A whose entries are the partial derivatives of the components of f
// with respect to all the coordinates. This is know as the Jacobian
//
// The best linear approximation to f is given by the matrix equation:
//
// y-y0 = A (x-x0)
//
// So, if x0 is a good "guess" for the zero of f, then solving for the zero of this
// linear approximation will give a "better guess" for the zero of f. Thus let y=0,
// and since y0=f(x0) one can solve the above equation for x. This leads to the
// Newton's method formula:
//
// xn+1 = xn - A-1 f(xn)
//
// where xn+1 denotes the (n+1)-st guess, obtained from the n-th guess xn in the
// fashion described above. Iterating this will give better and better approximations
// if you have a "good enough" initial guess.

const JACOBIAN_EPSILON = 0.001
const INVERSION_MAX_ITERATIONS = 30

// Increment with reflexion on boundary
func IncDelta(Val *float32) {
	if *Val < (1.0 - JACOBIAN_EPSILON) {

		*Val += JACOBIAN_EPSILON

	} else {
		*Val -= JACOBIAN_EPSILON
	}
}

// Euclidean distance between two vectors of n elements each one
func EuclideanDistance(a, b []float32, n int) float32 {
	var sum float32

	for i := 0; i < n; i++ {
		dif := b[i] - a[i]
		sum += dif * dif
	}

	return float32(math.Sqrt(float64(sum)))
}

// cmsPipelineEvalReverseFloat evaluates a LUT in reverse direction using the Newton method.
//
// x1 <- x - [J(x)]^-1 * f(x)
//
// lut: The LUT on where to do the search
// Target: LabK, 3 values of Lab plus destination K which is fixed
// Result: The obtained CMYK
// Hint: Location where to begin the search
func cmsPipelineEvalReverseFloat(mm mem.Manager, Target, Result, Hint []float32, lut *cmsPipeline) bool {
	var (
		i, j           uint32
		error          float64
		LastError      float64 = 1e20
		fx, x, xd, fxd [4]float32
		tmp, tmp2      cmsVEC3
		Jacobian       cmsMAT3
	)

	// Only 3->3 and 4->3 are supported
	if lut.InputChannels != 3 && lut.InputChannels != 4 {
		return false
	}
	if lut.OutputChannels != 3 {
		return false
	}

	// Take the hint as starting point if specified
	if Hint == nil {
		// Begin at any point, we choose 1/3 of CMY axis
		x[0], x[1], x[2] = 0.3, 0.3, 0.3
	} else {
		// Only copy 3 channels from hint...
		for j = 0; j < 3; j++ {
			x[j] = Hint[j]
		}
	}

	// If Lut is 4-dimensions, then grab target[3], which is fixed
	if lut.InputChannels == 4 {
		x[3] = Target[3]
	} else {
		x[3] = 0 // To keep lint happy
	}

	// Iterate
	for i = 0; i < INVERSION_MAX_ITERATIONS; i++ {
		// Get beginning fx
		cmsPipelineEvalFloat(mm, x[:], fx[:], lut)

		// Compute error
		error = float64(EuclideanDistance(fx[:], Target[:], 3))

		// If not convergent, return last safe value
		if error >= LastError {
			break
		}

		// Keep latest values
		LastError = error
		for j = 0; j < lut.InputChannels; j++ {
			Result[j] = x[j]
		}

		// Found an exact match?
		if error <= 0 {
			break
		}

		// Obtain slope (the Jacobian)
		for j = 0; j < 3; j++ {
			xd[0], xd[1], xd[2], xd[3] = x[0], x[1], x[2], x[3] // Copy current guess

			IncDelta(&xd[j]) // Apply a small delta to the j-th dimension

			cmsPipelineEvalFloat(mm, xd[:], fxd[:], lut)

			Jacobian.V[0].N[j] = float64(fxd[0]-fx[0]) / JACOBIAN_EPSILON
			Jacobian.V[1].N[j] = float64(fxd[1]-fx[1]) / JACOBIAN_EPSILON
			Jacobian.V[2].N[j] = float64(fxd[2]-fx[2]) / JACOBIAN_EPSILON
		}

		// Solve system
		tmp2.N[0], tmp2.N[1], tmp2.N[2] = float64(fx[0]-Target[0]), float64(fx[1]-Target[1]), float64(fx[2]-Target[2])

		if !cmsMAT3solve(&tmp, &Jacobian, &tmp2) {
			return false
		}

		// Move our guess
		x[0] -= float32(tmp.N[0])
		x[1] -= float32(tmp.N[1])
		x[2] -= float32(tmp.N[2])

		// Some clipping....
		for j = 0; j < 3; j++ {
			if x[j] < 0 {
				x[j] = 0
			} else if x[j] > 1.0 {
				x[j] = 1.0
			}
		}
	}

	return true
}

// EvaluateCLUTfloat evaluates a CLUT in true floating point.
func EvaluateCLUTfloat(mm mem.Manager, In []float32, Out []float32, mpe *cmsStage) {
	//fmt.Println("start EvaluateCLUTfloat")
	data, ok := mpe.Data.(*cmsStageCLutData)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageClutData\n")
		return
	}
	data.Params.Interpolation.LerpFloat(In, Out, data.Params)
}

func EvaluateCLUTfloatIn16(mm mem.Manager, In []float32, Out []float32, mpe *cmsStage) {
	in16 := in16Pool.Get().(*[MAX_STAGE_CHANNELS]uint16)
	out16 := out16Pool.Get().(*[MAX_STAGE_CHANNELS]uint16)
	defer func() {
		in16Pool.Put(in16)
		out16Pool.Put(out16)
	}()

	data, ok := mpe.Data.(*cmsStageCLutData)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageClutData\n")
		return
	}
	if mpe.InputChannels > MAX_STAGE_CHANNELS || mpe.OutputChannels > MAX_STAGE_CHANNELS {
		panic("Number of channels exceeds MAX_STAGE_CHANNELS")
	}

	FromFloatTo16(In, in16[:], mpe.InputChannels)
	data.Params.Interpolation.Lerp16(in16[:], out16[:], data.Params)
	From16ToFloat(out16[:], Out, mpe.OutputChannels)
}

// CubeSize calculates the total number of nodes in a hypercube.
func CubeSize(Dims []uint32, b uint32) uint32 {
	rv := uint32(1)

	if len(Dims) == 0 {
		return 0
	}

	for b > 0 {
		dim := Dims[b-1]
		if dim <= 1 {
			return 0 // Error
		}
		rv *= dim

		// Check for overflow
		if rv > math.MaxUint32/dim {
			return 0
		}
		b--
	}

	return rv
}

// CLUTElemDup duplicates a CLUT element.
func CLUTElemDup(mm mem.Manager, mpe *cmsStage) any {
	data, ok := mpe.Data.(*cmsStageCLutData)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageClutData\n")
		return nil
	}
	newElem := mem.New[cmsStageCLutData](mm)
	if newElem == nil {
		return nil
	}

	newElem.NEntries = data.NEntries
	newElem.HasFloatValues = data.HasFloatValues

	if data.Tab != nil {
		if data.HasFloatValues {
			newElem.Tab = cmsDupMemSlice(data.Tab.([]float32))
		} else {
			newElem.Tab = cmsDupMemSlice(data.Tab.([]uint16))
		}
	}

	newElem.Params = cmsComputeInterpParamsEx(mm,
		mpe.ContextID,
		data.Params.nSamples[:],
		data.Params.nInputs,
		data.Params.nOutputs,
		newElem.Tab,
		data.Params.dwFlags,
	)

	if newElem.Params != nil {
		return newElem
	} else {
		return nil
	}
}

// CLutElemTypeFree frees the resources of a CLUT element.
func CLutElemTypeFree(mm mem.Manager, mpe *cmsStage) {
	data, ok := mpe.Data.(*cmsStageCLutData)

	// Already empty
	if data == nil {
		return
	}
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageClutData\n")
		return
	}
	// Free interpolation parameters
	cmsFreeInterpParams(data.Params)

	// Free the data structure
	cmsFree(mpe.ContextID, data)
}

// Allocates a 16-bit multidimensional CLUT. This is evaluated at 16-bit precision.
// The table may have different granularity on each dimension.
func cmsStageAllocCLut16bitGranular(mm mem.Manager,
	ContextID CmsContext,
	clutPoints []uint32,
	inputChan, outputChan uint32,
	Table []uint16,
) *cmsStage {
	if clutPoints == nil {
		return nil
	}

	if inputChan > MAX_INPUT_DIMENSIONS {
		cmsSignalError(ContextID, cmsERROR_RANGE, "Too many input channels (%d channels, max=%d)", inputChan, MAX_INPUT_DIMENSIONS)
		return nil
	}

	NewMPE := cmsStageAllocPlaceholder(mm, ContextID, CmsSigCLutElemType, inputChan, outputChan, EvaluateCLUTfloatIn16, CLUTElemDup, CLutElemTypeFree, nil)
	if NewMPE == nil {
		return nil
	}

	NewElem := mem.New[cmsStageCLutData](mm)
	if NewElem == nil {
		cmsStageFree(mm, NewMPE)
		return nil
	}
	NewMPE.Data = NewElem

	NewElem.NEntries = uint32(outputChan) * CubeSize(clutPoints, inputChan)
	NewElem.HasFloatValues = false

	if NewElem.NEntries == 0 {
		cmsStageFree(mm, NewMPE)
		return nil
	}
	NewElem.Tab = mem.MakeSlice[uint16](mm, int(NewElem.NEntries))
	//fmt.Printf("111 mem.MakeSlice[uint16 %d %p %p\n", NewElem.NEntries, &NewElem.Tab.([]uint16)[0], NewElem.Tab)

	if Table != nil {
		for i := 0; i < int(NewElem.NEntries); i++ {
			NewElem.Tab.([]uint16)[i] = Table[i]
		}
	}

	NewElem.Params = cmsComputeInterpParamsEx(mm, ContextID, clutPoints, inputChan, outputChan, NewElem.Tab, CMS_LERP_FLAGS_16BITS)
	if NewElem.Params == nil {
		cmsStageFree(mm, NewMPE)
		return nil
	}

	return NewMPE
}

// Allocates a 16-bit CLUT with the same granularity on all dimensions.
func cmsStageAllocCLut16bit(mm mem.Manager,
	ContextID CmsContext,
	nGridPoints, inputChan, outputChan uint32,
	Table []uint16,
) *cmsStage {
	var Dimensions [MAX_INPUT_DIMENSIONS]uint32
	for i := range Dimensions {
		Dimensions[i] = nGridPoints
	}
	return cmsStageAllocCLut16bitGranular(mm, ContextID, Dimensions[:], inputChan, outputChan, Table)
}

// Allocates a floating-point CLUT with the same granularity on all dimensions.
func cmsStageAllocCLutFloat(mm mem.Manager,
	ContextID CmsContext,
	nGridPoints, inputChan, outputChan uint32,
	Table []float32,
) *cmsStage {
	var Dimensions [MAX_INPUT_DIMENSIONS]uint32
	for i := range Dimensions {
		Dimensions[i] = nGridPoints
	}
	return cmsStageAllocCLutFloatGranular(mm, ContextID, Dimensions[:], inputChan, outputChan, Table)
}

// Allocates a floating-point multidimensional CLUT. Table may have different granularity on each dimension.
func cmsStageAllocCLutFloatGranular(mm mem.Manager,
	ContextID CmsContext,
	clutPoints []uint32,
	inputChan, outputChan uint32,
	Table []float32,
) *cmsStage {
	if clutPoints == nil {
		return nil
	}

	if inputChan > MAX_INPUT_DIMENSIONS {
		cmsSignalError(ContextID, cmsERROR_RANGE, "Too many input channels")
		return nil
	}

	NewMPE := cmsStageAllocPlaceholder(mm, ContextID, CmsSigCLutElemType, inputChan, outputChan, EvaluateCLUTfloat, CLUTElemDup, CLutElemTypeFree, nil)
	if NewMPE == nil {
		return nil
	}

	NewElem := mem.New[cmsStageCLutData](mm)
	NewMPE.Data = NewElem

	NewElem.NEntries = uint32(outputChan) * CubeSize(clutPoints, inputChan)
	NewElem.HasFloatValues = true

	if NewElem.NEntries == 0 {
		cmsStageFree(mm, NewMPE)
		return nil
	}
	NewElem.Tab = mem.MakeSlice[float32](mm, int(NewElem.NEntries))

	if Table != nil {
		for i := 0; i < int(NewElem.NEntries); i++ {
			NewElem.Tab.([]float32)[i] = Table[i]
		}
	}

	NewElem.Params = cmsComputeInterpParamsEx(mm, ContextID, clutPoints, inputChan, outputChan, NewElem.Tab.([]float32), CMS_LERP_FLAGS_FLOAT)
	if NewElem.Params == nil {
		cmsStageFree(mm, NewMPE)
		return nil
	}

	return NewMPE
}

func IdentitySampler(mm mem.Manager, In []uint16, Out []uint16, cargo any) int32 {
	var nChan int

	switch v := cargo.(type) {
	case *int:
		if v == nil {
			cmsSignalError(nil, cmsERROR_RANGE, "Invalid cargo: nil *int")
			return 0
		}
		nChan = *v
	case *int32:
		if v == nil {
			cmsSignalError(nil, cmsERROR_RANGE, "Invalid cargo: nil *int32")
			return 0
		}
		nChan = int(*v)
	case *uint32:
		if v == nil {
			cmsSignalError(nil, cmsERROR_RANGE, "Invalid cargo: nil *uint32")
			return 0
		}
		nChan = int(*v)
	case *uint:
		if v == nil {
			cmsSignalError(nil, cmsERROR_RANGE, "Invalid cargo: nil *uint")
			return 0
		}
		nChan = int(*v)
	default:
		cmsSignalError(nil, cmsERROR_RANGE, "Invalid cargo: expected *int, *int32, *uint, *uint32")
		return 0
	}

	for i := 0; i < nChan && i < len(In) && i < len(Out); i++ {
		Out[i] = In[i]
	}
	return 1
}

func cmsStageAllocIdentityCLut(mm mem.Manager, ContextID CmsContext, nChan uint32) *cmsStage {
	var Dimensions [MAX_INPUT_DIMENSIONS]uint32
	for i := 0; i < MAX_INPUT_DIMENSIONS; i++ {
		Dimensions[i] = 2
	}

	mpe := cmsStageAllocCLut16bitGranular(mm, ContextID, Dimensions[:], nChan, nChan, nil)
	if mpe == nil {
		return nil
	}

	if !cmsStageSampleCLut16bit(mm, mpe, IdentitySampler, &nChan, 0) {
		cmsStageFree(mm, mpe)
		return nil
	}

	mpe.Implements = CmsSigIdentityElemType
	return mpe
}

// Quantizes a value `i` in the range [0, MaxSamples) to a 16-bit value (0..0xffff).
func cmsQuantizeVal(i float64, MaxSamples uint32) uint16 {
	x := (i * 65535.0) / float64(MaxSamples-1)
	return cmsQuickSaturateWord(x)
}

// Performs a sweep over the entire input space and calls the provided callback function on the knots.
// Returns true if all operations succeed, false otherwise.
// здесь происходит ошибка в tab
func cmsStageSampleCLut16bit(mm mem.Manager,
	mpe *cmsStage,
	Sampler cmsSAMPLER16,
	cargo any,
	dwFlags uint32,
) bool {
	//fmt.Println("start cmsStageSampleCLut16bit")
	if mpe == nil {
		return false
	}

	clut, ok := mpe.Data.(*cmsStageCLutData)
	if clut == nil {
		return false
	}
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageClutData\n")
		return false
	}
	nSamples := clut.Params.nSamples
	nInputs := clut.Params.nInputs
	nOutputs := clut.Params.nOutputs

	if nInputs <= 0 || nOutputs <= 0 || nInputs > MAX_INPUT_DIMENSIONS || nOutputs >= MAX_STAGE_CHANNELS {
		return false
	}

	var In [MAX_INPUT_DIMENSIONS + 1]uint16
	var Out [MAX_STAGE_CHANNELS]uint16

	nTotalPoints := CubeSize(nSamples[:], nInputs)
	if nTotalPoints == 0 {
		return false
	}

	index := 0
	for i := 0; i < int(nTotalPoints); i++ {
		rest := i
		for t := int(nInputs) - 1; t >= 0; t-- {
			Colorant := rest % int(nSamples[t])
			rest /= int(nSamples[t])
			In[t] = cmsQuantizeVal(float64(Colorant), nSamples[t])
			//fmt.Printf(" In[t]  %d\n", In[t])
		}

		if clut.Tab != nil {
			for t := 0; t < int(nOutputs); t++ {
				Out[t] = clut.Tab.([]uint16)[index+t]
				//fmt.Printf(" 11 Out[t] %d\n", Out[t])
			}
		}

		if Sampler(mm, In[:], Out[:], cargo) == 0 {
			return false
		}

		if dwFlags&SAMPLER_INSPECT == 0 {
			if clut.Tab != nil {
				for t := 0; t < int(nOutputs); t++ {
					clut.Tab.([]uint16)[index+t] = Out[t]
					//fmt.Printf(" 22 Out[t] %d\n", Out[t])
				}
			}

		}

		index += int(nOutputs)
	}
	//fmt.Println("true END cmsStageSampleCLut16bit")

	return true
}

// Performs a sweep over the entire input space for floating-point CLUTs and calls the provided callback function on the knots.
// Returns true if all operations succeed, false otherwise.
func cmsStageSampleCLutFloat(mm mem.Manager,
	mpe *cmsStage,
	Sampler cmsSAMPLERFLOAT,
	cargo any,
	dwFlags uint32,
) bool {
	if mpe == nil {
		return false
	}

	clut, ok := mpe.Data.(*cmsStageCLutData)
	if clut == nil {
		return false
	}
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsStageClutData\n")
		return false
	}
	nSamples := clut.Params.nSamples
	nInputs := clut.Params.nInputs
	nOutputs := clut.Params.nOutputs

	if nInputs <= 0 || nOutputs <= 0 || nInputs > MAX_INPUT_DIMENSIONS || nOutputs >= MAX_STAGE_CHANNELS {
		return false
	}

	var In [MAX_INPUT_DIMENSIONS + 1]float32
	var Out [MAX_STAGE_CHANNELS]float32

	nTotalPoints := CubeSize(nSamples[:], nInputs)
	if nTotalPoints == 0 {
		return false
	}

	index := 0
	for i := 0; i < int(nTotalPoints); i++ {
		rest := i
		for t := int(nInputs) - 1; t >= 0; t-- {
			Colorant := rest % int(nSamples[t])
			rest /= int(nSamples[t])
			In[t] = float32(cmsQuantizeVal(float64(Colorant), nSamples[t])) / 65535.0
		}

		if clut.Tab != nil {
			for t := 0; t < int(nOutputs); t++ {
				Out[t] = clut.Tab.([]float32)[index+t]
			}
		}

		if Sampler(mm, In[:], Out[:], cargo) != 0 {
			return false
		}

		if dwFlags&SAMPLER_INSPECT == 0 {
			if clut.Tab.([]float32) != nil {
				for t := 0; t < int(nOutputs); t++ {
					clut.Tab.([]float32)[index+t] = Out[t]
				}
			}
		}

		index += int(nOutputs)
	}

	return true
}

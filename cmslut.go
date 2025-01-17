package golcms

import (
	"fmt"
	"math"
	"reflect"
	"unsafe"
)

func cmsStageAllocPlaceholder(
	ContextID cmsContext,
	Type cmsStageSignature,
	InputChannels, OutputChannels uint32,
	EvalPtr cmsStageEvalFn,
	DupElemPtr cmsStageDupElemFn,
	FreePtr cmsStageFreeElemFn,
	Data unsafe.Pointer,
) *cmsStage {
	// Allocate memory for cmsStage and initialize to zero
	ph := (*cmsStage)(cmsMallocZero(ContextID, uint32(unsafe.Sizeof(cmsStage{}))))
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

// FromFloatTo16 converts a slice of float32 values to a slice of uint16 values
func FromFloatTo16(In []float32, Out []uint16, n uint32) {
	for i := uint32(0); i < n; i++ {
		Out[i] = cmsQuickSaturateWord(float64(In[i] * 65535.0))
	}
}

// From16ToFloat converts a slice of uint16 values to a slice of float32 values
func From16ToFloat(In []uint16, Out []float32, n uint32) {
	for i := uint32(0); i < n; i++ {
		Out[i] = float32(In[i]) / 65535.0
	}
}
func cmsPipelineCheckAndRetrieveStages(lut *cmsPipeline, n uint32, expectedTypes []cmsStageSignature, retrievedStages ...**cmsStage) bool {
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

func Clipper(In []float32, Out []float32, mpe *cmsStage) {
	for i := uint32(0); i < mpe.InputChannels; i++ {
		n := In[i]
		if n < 0 {
			Out[i] = 0
		} else {
			Out[i] = n
		}
	}
}
func cmsStageClipNegatives(ContextID cmsContext, nChannels uint32) *cmsStage {
	return cmsStageAllocPlaceholder(
		ContextID,
		cmsSigClipNegativesElemType,
		nChannels,
		nChannels,
		Clipper,
		nil,
		nil,
		nil,
	)
}

func cmsStageGetPtrToCurveSet(mpe *cmsStage) []*cmsToneCurve {
	data := (*cmsStageToneCurvesData)(mpe.Data)
	return data.TheCurves
}

func EvaluateCurves(In []float32, Out []float32, mpe *cmsStage) {
	data := (*cmsStageToneCurvesData)(mpe.Data)
	if data == nil || data.TheCurves == nil {
		return
	}

	for i := uint32(0); i < data.NCurves; i++ {
		Out[i] = cmsEvalToneCurveFloat(data.TheCurves[i], In[i])
	}
}

func CurveSetElemTypeFree(mpe *cmsStage) {
	data := (*cmsStageToneCurvesData)(mpe.Data)
	if data == nil || data.TheCurves == nil {
		return
	}

	for i := uint32(0); i < data.NCurves; i++ {
		for i := uint32(0); i < data.NCurves; i++ {
			if data.TheCurves[i] != nil {
				cmsFreeToneCurve(data.TheCurves[i])
			}
		}
	}
	// I probably do not need freeing of the slice itself after each member is freed
	//cmsFree(mpe.ContextID, unsafe.Pointer(data.TheCurves))
	cmsFree(mpe.ContextID, unsafe.Pointer(data))
}

// Convert unsafe.Pointer to a slice of pointers
func toPointerSlice(ptr unsafe.Pointer, length int) []*cmsToneCurve {
	// Create a slice header pointing to the allocated memory
	sliceHeader := reflect.SliceHeader{
		Data: uintptr(ptr),
		Len:  length,
		Cap:  length,
	}
	return *(*[]*cmsToneCurve)(unsafe.Pointer(&sliceHeader))
}
func CurveSetDup(mpe *cmsStage) unsafe.Pointer {
	// Access the data from the input stage
	data := (*cmsStageToneCurvesData)(mpe.Data)

	// Allocate memory for the new tone curves data structure
	newElem := (*cmsStageToneCurvesData)(cmsMallocZero(mpe.ContextID, uint32(unsafe.Sizeof(cmsStageToneCurvesData{}))))
	if newElem == nil {
		return nil
	}

	// Set the number of curves
	newElem.NCurves = data.NCurves

	// Allocate memory for the array of tone curve pointers
	newElem.TheCurves = make([]*cmsToneCurve, newElem.NCurves)
	newSliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&newElem.TheCurves))
	newBasePtr := unsafe.Pointer(newSliceHeader.Data)

	dataSliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&data.TheCurves))
	dataBasePtr := unsafe.Pointer(dataSliceHeader.Data)

	for i := uint32(0); i < newElem.NCurves; i++ {
		// Access the original curve pointer
		curve := (*cmsToneCurve)(unsafe.Add(dataBasePtr, uintptr(i)*unsafe.Sizeof((*cmsToneCurve)(nil))))

		// Duplicate the curve
		duplicatedCurve := cmsDupToneCurve(curve)
		if duplicatedCurve == nil {
			goto Error
		}

		// Assign the duplicated curve to the new allocated slice
		*(*cmsToneCurve)(unsafe.Add(newBasePtr, uintptr(i)*unsafe.Sizeof((*cmsToneCurve)(nil)))) = *duplicatedCurve
	}

	return unsafe.Pointer(newElem)

Error:
	// Cleanup allocated memory in case of an error
	for i := uint32(0); i < newElem.NCurves; i++ {
		newCurve := (*cmsToneCurve)(unsafe.Add(newBasePtr, uintptr(i)*unsafe.Sizeof((*cmsToneCurve)(nil))))
		if newCurve != nil {
			cmsFreeToneCurve(newCurve)
		}
	}
	cmsFree(mpe.ContextID, unsafe.Pointer(newSliceHeader.Data))
	cmsFree(mpe.ContextID, unsafe.Pointer(newElem))
	return nil
}

func cmsStageAllocToneCurves(ContextID cmsContext, nChannels uint32, Curves **cmsToneCurve) *cmsStage {
	// Allocate the placeholder for the stage
	newMPE := cmsStageAllocPlaceholder(ContextID, cmsSigCurveSetElemType, nChannels, nChannels, EvaluateCurves, CurveSetDup, CurveSetElemTypeFree, nil)
	if newMPE == nil {
		return nil
	}

	// Allocate the tone curves data structure
	newElem := (*cmsStageToneCurvesData)(cmsMallocZero(ContextID, uint32(unsafe.Sizeof(cmsStageToneCurvesData{}))))
	if newElem == nil {
		cmsStageFree(newMPE)
		return nil
	}

	newMPE.Data = unsafe.Pointer(newElem)

	// Set the number of curves
	newElem.NCurves = nChannels

	// Allocate memory for the slice of tone curve pointers
	newElem.TheCurves = make([]*cmsToneCurve, nChannels)
	newSliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&newElem.TheCurves))
	newBasePtr := unsafe.Pointer(newSliceHeader.Data)

	// Handle the input curves, either creating identity curves or duplicating existing ones
	if Curves != nil {
		curvesSliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&Curves))
		curvesBasePtr := unsafe.Pointer(curvesSliceHeader.Data)

		for i := uint32(0); i < nChannels; i++ {
			curve := (*cmsToneCurve)(unsafe.Add(curvesBasePtr, uintptr(i)*unsafe.Sizeof((*cmsToneCurve)(nil))))
			duplicatedCurve := cmsDupToneCurve(curve)
			if duplicatedCurve == nil {
				goto Error
			}
			*(*cmsToneCurve)(unsafe.Add(newBasePtr, uintptr(i)*unsafe.Sizeof((*cmsToneCurve)(nil)))) = *duplicatedCurve
		}
	} else {
		// Create identity curves if no input curves are provided
		for i := uint32(0); i < nChannels; i++ {
			curve := cmsBuildGamma(ContextID, 1.0)
			if curve == nil {
				goto Error
			}
			*(*cmsToneCurve)(unsafe.Add(newBasePtr, uintptr(i)*unsafe.Sizeof((*cmsToneCurve)(nil)))) = *curve
		}
	}

	return newMPE

Error:
	// Cleanup in case of an error
	for i := uint32(0); i < nChannels; i++ {
		curve := (*cmsToneCurve)(unsafe.Add(newBasePtr, uintptr(i)*unsafe.Sizeof((*cmsToneCurve)(nil))))
		if curve != nil {
			cmsFreeToneCurve(curve)
		}
	}
	cmsFree(ContextID, unsafe.Pointer(newSliceHeader.Data))
	cmsFree(ContextID, unsafe.Pointer(newElem))
	cmsStageFree(newMPE)
	return nil
}

func cmsStageAllocIdentityCurves(ContextID cmsContext, nChannels uint32) *cmsStage {
	mpe := cmsStageAllocToneCurves(ContextID, nChannels, nil)
	if mpe == nil {
		return nil
	}
	mpe.Implements = cmsSigIdentityElemType
	return mpe
}

// EvaluateMatrix performs matrix multiplication and applies an optional offset.
func EvaluateMatrix(in []float32, out []float32, mpe *cmsStage) {
	// Cast the unsafe pointer to the original struct type
	data := (*cmsStageMatrixData)(mpe.Data)

	for i := uint32(0); i < mpe.OutputChannels; i++ {
		var tmp float64 = 0
		for j := uint32(0); j < mpe.InputChannels; j++ {
			// Access Double as a slice using unsafe.Pointer arithmetic
			doublePtr := unsafe.Pointer(uintptr(unsafe.Pointer(data.Double)) + uintptr((i*mpe.InputChannels+j)*uint32(unsafe.Sizeof(float64(0)))))
			tmp += float64(in[j]) * *(*float64)(doublePtr)
		}

		if data.Offset != nil {
			// Access Offset as a slice using unsafe.Pointer arithmetic
			offsetPtr := unsafe.Pointer(uintptr(unsafe.Pointer(data.Offset)) + uintptr(i*uint32(unsafe.Sizeof(float64(0)))))
			tmp += *(*float64)(offsetPtr)
		}

		out[i] = float32(tmp)
	}
}

// MatrixElemDup duplicates the matrix stage data.
func MatrixElemDup(mpe *cmsStage) unsafe.Pointer {
	if mpe == nil || mpe.Data == nil {
		return nil
	}

	Data := (*cmsStageMatrixData)(mpe.Data)

	NewElem := (*cmsStageMatrixData)(cmsMallocZero(mpe.ContextID, uint32(unsafe.Sizeof(cmsStageMatrixData{}))))

	sz := mpe.InputChannels * mpe.OutputChannels
	var sizeoffl float64
	NewElem.Double = (*float64)(cmsDupMem(mpe.ContextID, unsafe.Pointer(Data.Double), sz*uint32(unsafe.Sizeof(sizeoffl))))

	if Data.Offset != nil {
		NewElem.Offset = (*float64)(cmsDupMem(mpe.ContextID,
			unsafe.Pointer(Data.Offset), mpe.OutputChannels*uint32(unsafe.Sizeof(sizeoffl))))

	}

	return unsafe.Pointer(NewElem)
}

// MatrixElemTypeFree frees the matrix stage data.
func MatrixElemTypeFree(mpe *cmsStage) {
	if mpe == nil || mpe.Data == nil {
		return
	}

	Data := (*cmsStageMatrixData)(mpe.Data)
	if Data == nil {
		return
	}
	if Data.Double != nil {
		cmsFree(mpe.ContextID, unsafe.Pointer(Data.Double))
	}
	if Data.Offset != nil {
		cmsFree(mpe.ContextID, unsafe.Pointer(Data.Offset))
	}

	cmsFree(mpe.ContextID, mpe.Data)
}

func cmsStageAllocMatrix(
	ContextID cmsContext,
	Rows, Cols uint32,
	Matrix, Offset []float64,
) *cmsStage {
	var i, n uint32
	var NewElem *cmsStageMatrixData
	var NewMPE *cmsStage

	// Calculate the number of elements in the matrix
	n = Rows * Cols

	// Check for overflow and invalid dimensions
	if n == 0 || n >= math.MaxUint32/Cols || n >= math.MaxUint32/Rows || n < Rows || n < Cols {
		return nil
	}

	// Allocate the matrix stage
	NewMPE = cmsStageAllocPlaceholder(
		ContextID,
		cmsSigMatrixElemType,
		Cols,
		Rows,
		EvaluateMatrix,
		MatrixElemDup,
		MatrixElemTypeFree,
		nil,
	)
	if NewMPE == nil {
		return nil
	}

	// Allocate memory for the matrix data
	NewElem = (*cmsStageMatrixData)(cmsMallocZero(ContextID, uint32(unsafe.Sizeof(cmsStageMatrixData{}))))
	if NewElem == nil {
		goto Error
	}
	NewMPE.Data = unsafe.Pointer(NewElem)

	// Allocate memory for the Double array
	NewElem.Double = (*float64)(cmsCalloc(ContextID, n, uint32(unsafe.Sizeof(float64(0)))))
	if NewElem.Double == nil {
		goto Error
	}

	// Copy the matrix elements into the Double array
	for i = 0; i < n; i++ {
		elementPtr := (*float64)(unsafe.Pointer(uintptr(unsafe.Pointer(NewElem.Double)) + uintptr(i)*unsafe.Sizeof(float64(0))))
		*elementPtr = Matrix[i]
	}

	// If an offset is provided, allocate memory for the Offset array and copy its elements
	if Offset != nil {
		NewElem.Offset = (*float64)(cmsCalloc(ContextID, Rows, uint32(unsafe.Sizeof(float64(0)))))
		if NewElem.Offset == nil {
			goto Error
		}

		for i = 0; i < Rows; i++ {
			offsetPtr := (*float64)(unsafe.Pointer(uintptr(unsafe.Pointer(NewElem.Offset)) + uintptr(i)*unsafe.Sizeof(float64(0))))
			*offsetPtr = Offset[i]
		}
	}

	return NewMPE

Error:
	if NewMPE != nil {
		cmsStageFree(NewMPE)
	}
	return nil
}

func EvaluateXYZ2Lab(In []float32, Out []float32, mpe *cmsStage) {
	const XYZadj = MAX_ENCODEABLE_XYZ

	var XYZ cmsCIEXYZ
	var Lab cmsCIELab

	// From 0..1.0 to XYZ
	XYZ.X = float64(In[0]) * XYZadj
	XYZ.Y = float64(In[1]) * XYZadj
	XYZ.Z = float64(In[2]) * XYZadj

	// Convert XYZ to Lab
	cmsXYZ2Lab(nil, &Lab, &XYZ)

	// From V4 Lab to 0..1.0
	Out[0] = float32(Lab.L / 100.0)
	Out[1] = float32((Lab.a + 128.0) / 255.0)
	Out[2] = float32((Lab.b + 128.0) / 255.0)
}

func cmsStageAllocXYZ2Lab(ContextID cmsContext) *cmsStage {
	return cmsStageAllocPlaceholder(ContextID, cmsSigXYZ2LabElemType, 3, 3, EvaluateXYZ2Lab, nil, nil, nil)
}
func EvaluateLab2XYZ(In []float32, Out []float32, mpe *cmsStage) {
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

}

// No dup or free routines needed, as the structure has no pointers in it.
func cmsStageAllocLab2XYZ(ContextID cmsContext) *cmsStage {
	return cmsStageAllocPlaceholder(ContextID, cmsSigLab2XYZElemType, 3, 3, EvaluateLab2XYZ, nil, nil, nil)
}

// ********************************************************************************

// v2 L=100 is supposed to be placed on 0xFF00. There is no reasonable
// number of gridpoints that would make exact match. However, a prelinearization
// of 258 entries would map 0xFF00 exactly on entry 257, and this is good to avoid scum dot.
// Almost all what we need, but unfortunately, the rest of entries should be scaled by
// (255*257/256), and this is not exact.

func cmsStageAllocLabV2ToV4curves(ContextID cmsContext) *cmsStage {
	var LabTable [3]*cmsToneCurve
	var mpe *cmsStage
	var i, j int

	// Build 258-entry tone curves for Lab components
	LabTable[0] = cmsBuildTabulatedToneCurve16(ContextID, 258, nil)
	LabTable[1] = cmsBuildTabulatedToneCurve16(ContextID, 258, nil)
	LabTable[2] = cmsBuildTabulatedToneCurve16(ContextID, 258, nil)

	// Ensure all tone curves were created successfully
	for j = 0; j < 3; j++ {
		if LabTable[j] == nil {
			cmsFreeToneCurveTriple(LabTable)
			return nil
		}
		// Convert Table16 pointer to a slice of uint16
		table16 := unsafe.Slice(LabTable[j].Table16, 258)

		// Populate tone curve entries
		// We need to map * (0xffff / 0xff00), that's same as (257 / 256)
		// So we can use 258-entry tables to do the trick:
		// (i / 257) * (255 * 257) * (257 / 256)
		for i = 0; i < 257; i++ {
			table16[i] = uint16((i*0xffff + 0x80) >> 8)
		}

		// Set the last entry to 0xffff
		table16[257] = 0xffff
	}

	// Allocate the tone curve stage
	mpe = cmsStageAllocToneCurves(ContextID, 3, &LabTable[0])
	cmsFreeToneCurveTriple(LabTable)

	// Check if allocation was successful
	if mpe == nil {
		return nil
	}

	// Set the implementation signature
	mpe.Implements = cmsStageSignature(cmsSigLabV2toV4)
	return mpe
}

// _cmsStageAllocLabV2ToV4 allocates a matrix-based stage for Lab v2 to v4 conversion.
func cmsStageAllocLabV2ToV4(ContextID cmsContext) *cmsStage {
	var v2ToV4 = []float64{
		65535.0 / 65280.0, 0, 0,
		0, 65535.0 / 65280.0, 0,
		0, 0, 65535.0 / 65280.0,
	}

	mpe := cmsStageAllocMatrix(ContextID, 3, 3, v2ToV4, nil)
	if mpe == nil {
		return nil
	}
	mpe.Implements = cmsStageSignature(cmsSigLabV2toV4)
	return mpe
}

// _cmsStageAllocLabV4ToV2 allocates a matrix-based stage for Lab v4 to v2 conversion.
func cmsStageAllocLabV4ToV2(ContextID cmsContext) *cmsStage {
	var v4ToV2 = []float64{
		65280.0 / 65535.0, 0, 0,
		0, 65280.0 / 65535.0, 0,
		0, 0, 65280.0 / 65535.0,
	}

	mpe := cmsStageAllocMatrix(ContextID, 3, 3, v4ToV2, nil)
	if mpe == nil {
		return nil
	}
	mpe.Implements = cmsStageSignature(cmsSigLabV4toV2)
	return mpe
}

// Constants for normalization
const (
	normFactorXYZToFloat = 32768.0 / 65535.0
	normFactorFloatToXYZ = 65535.0 / 32768.0
)

// _cmsStageNormalizeFromLabFloat normalizes Lab values from integer range to floating-point PCS range.
func cmsStageNormalizeFromLabFloat(ContextID cmsContext) *cmsStage {
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

	mpe := cmsStageAllocMatrix(ContextID, 3, 3, a1, o1)
	if mpe == nil {
		return nil
	}
	mpe.Implements = cmsSigLab2FloatPCS
	return mpe
}

// _cmsStageNormalizeFromXyzFloat normalizes XYZ values from integer range to floating-point PCS range.
func cmsStageNormalizeFromXyzFloat(ContextID cmsContext) *cmsStage {
	a1 := []float64{
		normFactorXYZToFloat, 0, 0,
		0, normFactorXYZToFloat, 0,
		0, 0, normFactorXYZToFloat,
	}

	mpe := cmsStageAllocMatrix(ContextID, 3, 3, a1, nil)
	if mpe == nil {
		return nil
	}
	mpe.Implements = cmsSigXYZ2FloatPCS
	return mpe
}

// _cmsStageNormalizeToLabFloat normalizes Lab values from floating-point PCS range to integer range.
func cmsStageNormalizeToLabFloat(ContextID cmsContext) *cmsStage {
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

	mpe := cmsStageAllocMatrix(ContextID, 3, 3, a1, o1)
	if mpe == nil {
		return nil
	}
	mpe.Implements = cmsSigFloatPCS2Lab
	return mpe
}

// _cmsStageNormalizeToXyzFloat normalizes XYZ values from floating-point PCS range to integer range.
func cmsStageNormalizeToXyzFloat(ContextID cmsContext) *cmsStage {
	a1 := []float64{
		normFactorFloatToXYZ, 0, 0,
		0, normFactorFloatToXYZ, 0,
		0, 0, normFactorFloatToXYZ,
	}

	mpe := cmsStageAllocMatrix(ContextID, 3, 3, a1, nil)
	if mpe == nil {
		return nil
	}
	mpe.Implements = cmsSigFloatPCS2XYZ
	return mpe
}

func cmsStageAllocLabPrelin(ContextID cmsContext) *cmsStage {
	params := []float64{2.4}
	var LabTable [3]*cmsToneCurve

	LabTable[0] = cmsBuildGamma(ContextID, 1.0)
	LabTable[1] = cmsBuildParametricToneCurve(ContextID, 108, &params[0])
	LabTable[2] = cmsBuildParametricToneCurve(ContextID, 108, &params[0])

	return cmsStageAllocToneCurves(ContextID, 3, &LabTable[0])
}
func cmsStageFree(mpe *cmsStage) {
	if mpe.FreePtr != nil {
		mpe.FreePtr(mpe)
	}
	cmsFree(mpe.ContextID, unsafe.Pointer(mpe))
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
func cmsStageData(mpe *cmsStage) interface{} {
	return mpe.Data
}
func cmsGetStageContextID(mpe *cmsStage) cmsContext {
	return mpe.ContextID
}
func cmsStageNext(mpe *cmsStage) *cmsStage {
	return mpe.Next
}
func cmsStageDup(mpe *cmsStage) *cmsStage {
	if mpe == nil {
		return nil
	}

	NewMPE := cmsStageAllocPlaceholder(
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
		NewMPE.Data = mpe.DupElemPtr(mpe)
		if NewMPE.Data == nil {
			cmsStageFree(NewMPE)
			return nil
		}
	} else {
		NewMPE.Data = nil
	}

	return NewMPE
}

// BlessLUT sets up the channel count and ensures consistency across stages.
func BlessLUT(lut *cmsPipeline) bool {
	// We can set the input/output channels only if we have elements.
	if lut.Elements != nil {
		first := cmsPipelineGetPtrToFirstStage(lut)
		last := cmsPipelineGetPtrToLastStage(lut)

		if first == nil || last == nil {
			return false
		}

		lut.InputChannels = first.InputChannels
		lut.OutputChannels = last.OutputChannels

		// Check chain consistency
		prev := first
		next := prev.Next

		for next != nil {
			if next.InputChannels != prev.OutputChannels {
				return false
			}

			prev = next
			next = next.Next
		}
	}

	return true
}

// _LUTeval16 evaluates the LUT on a 16-bit basis
func LUTeval16(In []uint16, Out []uint16, D unsafe.Pointer) {
	lut := (*cmsPipeline)(D)
	var Storage [2][MAX_STAGE_CHANNELS]float32
	Phase := 0

	// Convert input from 16-bit to float
	From16ToFloat(In, Storage[Phase][:], lut.InputChannels)

	// Process each stage in the pipeline
	for mpe := lut.Elements; mpe != nil; mpe = mpe.Next {
		NextPhase := Phase ^ 1
		mpe.EvalPtr(Storage[Phase][:], Storage[NextPhase][:], mpe)
		Phase = NextPhase
	}

	// Convert output from float to 16-bit
	FromFloatTo16(Storage[Phase][:], Out, lut.OutputChannels)
}

// _LUTevalFloat evaluates the LUT on a float32 basis
func LUTevalFloat(In []float32, Out []float32, D unsafe.Pointer) {
	lut := (*cmsPipeline)(D)
	var Storage [2][MAX_STAGE_CHANNELS]float32
	Phase := 0

	// Copy input to the first storage buffer
	memmove(unsafe.Pointer(&Storage[Phase][0]), unsafe.Pointer(&In[0]), uintptr(lut.InputChannels)*unsafe.Sizeof(float32(0)))

	// Process each stage in the pipeline
	for mpe := lut.Elements; mpe != nil; mpe = mpe.Next {
		NextPhase := Phase ^ 1
		mpe.EvalPtr(Storage[Phase][:], Storage[NextPhase][:], mpe)
		Phase = NextPhase
	}

	// Copy the result to the output
	memmove(unsafe.Pointer(&Out[0]), unsafe.Pointer(&Storage[Phase][0]), uintptr(lut.OutputChannels)*unsafe.Sizeof(float32(0)))
}

// cmsPipelineAlloc allocates and initializes a new LUT pipeline
func cmsPipelineAlloc(contextID cmsContext, inputChannels, outputChannels uint32) *cmsPipeline {
	// A value of zero in channels is allowed as a placeholder
	if inputChannels >= cmsMAXCHANNELS || outputChannels >= cmsMAXCHANNELS {
		return nil
	}

	// Allocate memory for the cmsPipeline struct
	newLUT := (*cmsPipeline)(cmsMallocZero(contextID, uint32(unsafe.Sizeof(cmsPipeline{}))))
	if newLUT == nil {
		return nil
	}

	// Initialize the LUT structure
	newLUT.InputChannels = inputChannels
	newLUT.OutputChannels = outputChannels
	newLUT.Eval16Fn = LUTeval16
	newLUT.EvalFloatFn = LUTevalFloat
	newLUT.DupDataFn = nil
	newLUT.FreeDataFn = nil
	newLUT.Data = unsafe.Pointer(newLUT)
	newLUT.ContextID = contextID

	// Validate the LUT
	if !BlessLUT(newLUT) {
		cmsFree(contextID, unsafe.Pointer(newLUT))
		return nil
	}

	return newLUT
}
func cmsGetPipelineContextID(lut *cmsPipeline) cmsContext {
	if lut == nil {
		panic("lut is nil")
	}
	return lut.ContextID
}
func cmsPipelineInputChannels(lut *cmsPipeline) uint32 {
	if lut == nil {
		panic("lut is nil")
	}
	return lut.InputChannels
}
func cmsPipelineOutputChannels(lut *cmsPipeline) uint32 {
	if lut == nil {
		panic("lut is nil")
	}
	return lut.OutputChannels
}
func cmsPipelineFree(lut *cmsPipeline) {
	if lut == nil {
		return
	}

	var next *cmsStage
	for mpe := lut.Elements; mpe != nil; mpe = next {
		next = mpe.Next
		cmsStageFree(mpe)
	}

	if lut.FreeDataFn != nil {
		lut.FreeDataFn(lut.ContextID, lut.Data)
	}

	cmsFree(lut.ContextID, unsafe.Pointer(lut))
}
func cmsPipelineEval16(In []uint16, Out []uint16, lut *cmsPipeline) {
	if lut == nil {
		panic("lut is nil")
	}
	lut.Eval16Fn(In, Out, lut.Data)
}
func cmsPipelineEvalFloat(In []float32, Out []float32, lut *cmsPipeline) {
	if lut == nil {
		panic("lut is nil")
	}
	lut.EvalFloatFn(In, Out, unsafe.Pointer(lut))
}
func cmsPipelineDup(lut *cmsPipeline) *cmsPipeline {
	if lut == nil {
		return nil
	}

	NewLUT := cmsPipelineAlloc(lut.ContextID, lut.InputChannels, lut.OutputChannels)
	if NewLUT == nil {
		return nil
	}

	var anterior *cmsStage
	first := true

	for mpe := lut.Elements; mpe != nil; mpe = mpe.Next {
		NewMPE := cmsStageDup(mpe)
		if NewMPE == nil {
			cmsPipelineFree(NewLUT)
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
		cmsFree(lut.ContextID, unsafe.Pointer(NewLUT))
		return nil
	}

	return NewLUT
}
func cmsPipelineInsertStage(lut *cmsPipeline, loc cmsStageLoc, mpe *cmsStage) bool {
	if lut == nil || mpe == nil {
		return false
	}

	switch loc {
	case cmsAT_BEGIN:
		mpe.Next = lut.Elements
		lut.Elements = mpe

	case cmsAT_END:
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

	if !BlessLUT(lut) {
		return false
	}

	return true
}
func cmsPipelineUnlinkStage(lut *cmsPipeline, loc cmsStageLoc, mpe **cmsStage) {
	if lut.Elements == nil {
		if mpe != nil {
			*mpe = nil
		}
		return
	}

	var unlinked *cmsStage

	switch loc {
	case cmsAT_BEGIN:
		elem := lut.Elements
		lut.Elements = elem.Next
		elem.Next = nil
		unlinked = elem

	case cmsAT_END:
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
		cmsStageFree(unlinked)
	}

	BlessLUT(lut)
}
func cmsPipelineCat(l1 *cmsPipeline, l2 *cmsPipeline) bool {
	if l1.Elements == nil && l2.Elements == nil {
		l1.InputChannels = l2.InputChannels
		l1.OutputChannels = l2.OutputChannels
	}

	for mpe := l2.Elements; mpe != nil; mpe = mpe.Next {
		if !cmsPipelineInsertStage(l1, cmsAT_END, cmsStageDup(mpe)) {
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
func cmsPipelineEvalReverseFloat(Target, Result, Hint []float32, lut *cmsPipeline) bool {
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
		cmsPipelineEvalFloat(x[:], fx[:], lut)

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

			cmsPipelineEvalFloat(xd[:], fxd[:], lut)

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
func EvaluateCLUTfloat(In []float32, Out []float32, mpe *cmsStage) {
	data := (*cmsStageCLutData)(mpe.Data)
	data.Params.Interpolation.LerpFloat(&In[0], &Out[0], data.Params)
}

// EvaluateCLUTfloatIn16 converts to 16 bits, evaluates, and back to floating point.
func EvaluateCLUTfloatIn16(In []float32, Out []float32, mpe *cmsStage) {
	var In16 [MAX_STAGE_CHANNELS]uint16
	var Out16 [MAX_STAGE_CHANNELS]uint16

	data := (*cmsStageCLutData)(mpe.Data)

	if len(In) > MAX_STAGE_CHANNELS || len(Out) > MAX_STAGE_CHANNELS {
		panic("Number of channels exceeds MAX_STAGE_CHANNELS")
	}

	FromFloatTo16(In, In16[:len(In)], mpe.InputChannels)
	data.Params.Interpolation.Lerp16(&In16[0], &Out16[0], data.Params)
	From16ToFloat(Out16[:len(Out)], Out[:len(Out)], mpe.OutputChannels)
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
func CLUTElemDup(mpe *cmsStage) unsafe.Pointer {
	data := (*cmsStageCLutData)(mpe.Data)
	newElem := (*cmsStageCLutData)(cmsMallocZero(mpe.ContextID, uint32(unsafe.Sizeof(cmsStageCLutData{}))))
	if newElem == nil {
		return nil
	}

	newElem.NEntries = data.NEntries
	newElem.HasFloatValues = data.HasFloatValues

	if data.Tab.T != nil {
		if data.HasFloatValues {
			newElem.Tab.TFloat = (*float32)(cmsDupMem(mpe.ContextID, unsafe.Pointer(data.Tab.TFloat), data.NEntries*uint32(unsafe.Sizeof(float32(0)))))
			if newElem.Tab.TFloat == nil {
				goto Error
			}
		} else {
			newElem.Tab.T = (*uint16)(cmsDupMem(mpe.ContextID, unsafe.Pointer(data.Tab.T), data.NEntries*uint32(unsafe.Sizeof(uint16(0)))))
			if newElem.Tab.T == nil {
				goto Error
			}
		}
	}

	newElem.Params = cmsComputeInterpParamsEx(
		mpe.ContextID,
		&data.Params.nSamples[0],
		data.Params.nInputs,
		data.Params.nOutputs,
		unsafe.Pointer(newElem.Tab.T),
		data.Params.dwFlags,
	)

	if newElem.Params != nil {
		return unsafe.Pointer(newElem)
	}

Error:
	if newElem.Tab.T != nil {
		cmsFree(mpe.ContextID, unsafe.Pointer(newElem.Tab.T))
	}
	cmsFree(mpe.ContextID, unsafe.Pointer(newElem))
	return nil
}

// CLutElemTypeFree frees the resources of a CLUT element.
func CLutElemTypeFree(mpe *cmsStage) {
	data := (*cmsStageCLutData)(mpe.Data)

	// Already empty
	if data == nil {
		return
	}

	// Free the table
	if data.Tab.T != nil {
		cmsFree(mpe.ContextID, unsafe.Pointer(data.Tab.T))
	}

	// Free interpolation parameters
	cmsFreeInterpParams(data.Params)

	// Free the data structure
	cmsFree(mpe.ContextID, unsafe.Pointer(data))
}

// Allocates a 16-bit multidimensional CLUT. This is evaluated at 16-bit precision.
// The table may have different granularity on each dimension.
func cmsStageAllocCLut16bitGranular(
	ContextID cmsContext,
	clutPoints []uint32,
	inputChan, outputChan uint32,
	Table []uint16,
) *cmsStage {
	if clutPoints == nil {
		return nil
	}

	if inputChan > MAX_INPUT_DIMENSIONS {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_RANGE, fmt.Sprintf("Too many input channels (%d channels, max=%d)", inputChan, MAX_INPUT_DIMENSIONS))
		return nil
	}

	NewMPE := cmsStageAllocPlaceholder(ContextID, cmsSigCLutElemType, inputChan, outputChan, EvaluateCLUTfloatIn16, CLUTElemDup, CLutElemTypeFree, nil)
	if NewMPE == nil {
		return nil
	}

	NewElem := (*cmsStageCLutData)(cmsMallocZero(ContextID, uint32(unsafe.Sizeof(cmsStageCLutData{}))))
	if NewElem == nil {
		cmsStageFree(NewMPE)
		return nil
	}
	NewMPE.Data = unsafe.Pointer(NewElem)

	NewElem.NEntries = uint32(outputChan) * CubeSize(clutPoints, inputChan)
	NewElem.HasFloatValues = false

	if NewElem.NEntries == 0 {
		cmsStageFree(NewMPE)
		return nil
	}

	NewElem.Tab.T = (*uint16)(cmsCalloc(ContextID, NewElem.NEntries, uint32(unsafe.Sizeof(uint16(0)))))

	if Table != nil {
		for i := 0; i < int(NewElem.NEntries); i++ {
			// Calculate the address of the ith element in the allocated memory
			ptr := (*uint16)(unsafe.Add(unsafe.Pointer(NewElem.Tab.T), uintptr(i)*unsafe.Sizeof(uint16(0))))
			// Copy the value from Table to the allocated memory
			*ptr = Table[i]
		}
	}

	NewElem.Params = cmsComputeInterpParamsEx(ContextID, &clutPoints[0], inputChan, outputChan, unsafe.Pointer(NewElem.Tab.T), CMS_LERP_FLAGS_16BITS)
	if NewElem.Params == nil {
		cmsStageFree(NewMPE)
		return nil
	}

	return NewMPE
}

// Allocates a 16-bit CLUT with the same granularity on all dimensions.
func cmsStageAllocCLut16bit(
	ContextID cmsContext,
	nGridPoints, inputChan, outputChan uint32,
	Table []uint16,
) *cmsStage {
	Dimensions := make([]uint32, MAX_INPUT_DIMENSIONS)
	for i := range Dimensions {
		Dimensions[i] = nGridPoints
	}
	return cmsStageAllocCLut16bitGranular(ContextID, Dimensions, inputChan, outputChan, Table)
}

// Allocates a floating-point CLUT with the same granularity on all dimensions.
func cmsStageAllocCLutFloat(
	ContextID cmsContext,
	nGridPoints, inputChan, outputChan uint32,
	Table []float32,
) *cmsStage {
	Dimensions := make([]uint32, MAX_INPUT_DIMENSIONS)
	for i := range Dimensions {
		Dimensions[i] = nGridPoints
	}
	return cmsStageAllocCLutFloatGranular(ContextID, Dimensions, inputChan, outputChan, Table)
}

// Allocates a floating-point multidimensional CLUT. Table may have different granularity on each dimension.
func cmsStageAllocCLutFloatGranular(
	ContextID cmsContext,
	clutPoints []uint32,
	inputChan, outputChan uint32,
	Table []float32,
) *cmsStage {
	if clutPoints == nil {
		return nil
	}

	if inputChan > MAX_INPUT_DIMENSIONS {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_RANGE, fmt.Sprintf("Too many input channels"))
		return nil
	}

	NewMPE := cmsStageAllocPlaceholder(ContextID, cmsSigCLutElemType, inputChan, outputChan, EvaluateCLUTfloat, CLUTElemDup, CLutElemTypeFree, nil)
	if NewMPE == nil {
		return nil
	}

	NewElem := &cmsStageCLutData{}
	NewMPE.Data = unsafe.Pointer(NewElem)

	NewElem.NEntries = uint32(outputChan) * CubeSize(clutPoints, inputChan)
	NewElem.HasFloatValues = true

	if NewElem.NEntries == 0 {
		cmsStageFree(NewMPE)
		return nil
	}
	NewElem.Tab.TFloat = (*float32)(cmsCalloc(ContextID, NewElem.NEntries, uint32(unsafe.Sizeof(float32(0)))))

	if Table != nil {
		for i := 0; i < int(NewElem.NEntries); i++ {
			// Calculate the address of the ith element in the allocated memory
			ptr := (*float32)(unsafe.Add(unsafe.Pointer(NewElem.Tab.TFloat), uintptr(i)*unsafe.Sizeof(float32(0))))
			// Copy the value from Table to the allocated memory
			*ptr = Table[i]
		}
	}

	NewElem.Params = cmsComputeInterpParamsEx(ContextID, &clutPoints[0], inputChan, outputChan, unsafe.Pointer(NewElem.Tab.TFloat), CMS_LERP_FLAGS_FLOAT)
	if NewElem.Params == nil {
		cmsStageFree(NewMPE)
		return nil
	}

	return NewMPE
}

// Quantizes a value `i` in the range [0, MaxSamples) to a 16-bit value (0..0xffff).
func cmsQuantizeVal(i float64, MaxSamples uint32) uint16 {
	x := (i * 65535.0) / float64(MaxSamples-1)
	return cmsQuickSaturateWord(x)
}

// Performs a sweep over the entire input space and calls the provided callback function on the knots.
// Returns true if all operations succeed, false otherwise.
func cmsStageSampleCLut16bit(
	mpe *cmsStage,
	Sampler cmsSAMPLER16,
	Cargo unsafe.Pointer,
	dwFlags uint32,
) bool {
	if mpe == nil {
		return false
	}

	clut := (*cmsStageCLutData)(mpe.Data)
	if clut == nil {
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
		}

		if clut.Tab.T != nil {
			basePtr := unsafe.Pointer(clut.Tab.T) // Get the base pointer
			for t := 0; t < int(nOutputs); t++ {
				// Calculate the address for the element at index + t
				elementPtr := (*uint16)(unsafe.Add(basePtr, uintptr(index+t)*unsafe.Sizeof(uint16(0))))
				Out[t] = *elementPtr // Dereference the pointer to get the value
			}
		}

		if Sampler(In[:], Out[:], Cargo) != 0 {
			return false
		}

		if dwFlags&SAMPLER_INSPECT == 0 {
			if clut.Tab.T != nil {
				basePtr := unsafe.Pointer(clut.Tab.T) // Get the base pointer
				for t := 0; t < int(nOutputs); t++ {
					// Calculate the address for the element at index + t
					elementPtr := (*uint16)(unsafe.Add(basePtr, uintptr(index+t)*unsafe.Sizeof(uint16(0))))
					Out[t] = *elementPtr // Dereference the pointer to get the value
				}
			}

		}

		index += int(nOutputs)
	}

	return true
}

// Performs a sweep over the entire input space for floating-point CLUTs and calls the provided callback function on the knots.
// Returns true if all operations succeed, false otherwise.
func cmsStageSampleCLutFloat(
	mpe *cmsStage,
	Sampler cmsSAMPLERFLOAT,
	Cargo unsafe.Pointer,
	dwFlags uint32,
) bool {
	if mpe == nil {
		return false
	}

	clut := (*cmsStageCLutData)(mpe.Data)
	if clut == nil {
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

		if clut.Tab.TFloat != nil {
			basePtr := unsafe.Pointer(clut.Tab.TFloat) // Get the base pointer
			for t := 0; t < int(nOutputs); t++ {
				// Calculate the address for the element at index + t
				elementPtr := (*float32)(unsafe.Add(basePtr, uintptr(index+t)*unsafe.Sizeof(float32(0))))
				Out[t] = *elementPtr // Dereference the pointer to get the value
			}
		}

		if Sampler(In[:], Out[:], Cargo) != 0 {
			return false
		}

		if dwFlags&SAMPLER_INSPECT == 0 {
			if clut.Tab.TFloat != nil {
				basePtr := unsafe.Pointer(clut.Tab.TFloat) // Get the base pointer
				for t := 0; t < int(nOutputs); t++ {
					// Calculate the address for the element at index + t
					elementPtr := (*float32)(unsafe.Add(basePtr, uintptr(index+t)*unsafe.Sizeof(float32(0))))
					Out[t] = *elementPtr // Dereference the pointer to get the value
				}
			}
		}

		index += int(nOutputs)
	}

	return true
}

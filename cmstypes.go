package golcms

// Tag Serialization  -----------------------------------------------------------------------------
// This file implements every single tag and tag type as described in the ICC spec. Some types
// have been deprecated, like ncl and Data. There is no implementation for those types as there
// are no profiles holding them. The programmer can also extend this list by defining his own types
// by using the appropriate plug-in. There are three types of plug ins regarding that. First type
// allows to define new tags using any existing type. Next plug-in type allows to define new types
// and the third one is very specific: allows to extend the number of elements in the multiprocessing
// elements special type.
//--------------------------------------------------------------------------------------------------

import (
	//"math"

	"unsafe"
)

func cmsWriteWCharArray(io *cmsIOHANDLER, n uint32, Array *uint16) bool {
	// Assert conditions
	if io == nil || (Array == nil && n > 0) {
		return false
	}

	for i := uint32(0); i < n; i++ {
		// Calculate the current element's address using pointer arithmetic
		currentElement := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(Array)) + uintptr(i)*unsafe.Sizeof(*Array)))
		if !cmsWriteUInt16Number(io, currentElement) {
			return false
		}
	}

	return true
}

// Some broken types
const (
	cmsCorbisBrokenXYZtype   cmsTagTypeSignature = 0x17A505B8
	cmsMonacoBrokenCurveType cmsTagTypeSignature = 0x9478EE00
)


// Register a new type handler. This routine is shared between normal types and MPE.
func RegisterTypesPlugin(id cmsContext, Data *cmsPluginBase, pos cmsMemoryClient) bool {
    Plugin := (*cmsPluginTagType)(unsafe.Pointer(Data))
    ctx := (*cmsTagTypePluginChunkType)(cmsContextGetClientChunk(id, pos))

    // If Data is nil, unregister the plug-in.
    if Data == nil {
        // No need to free memory; pool is destroyed as a whole.
        ctx.TagTypes = nil
        return true
    }

    // Allocate memory for the new linked list node.
    pt := (*cmsTagTypeLinkedList)(cmsPluginMalloc(id, uint32(unsafe.Sizeof(cmsTagTypeLinkedList{}))))
    if pt == nil {
        return false
    }

    // Assign handler and link to the current list.
    pt.Handler = Plugin.Handler
    pt.Next = ctx.TagTypes

    // Update the context's tag types to point to the new node.
    ctx.TagTypes = pt

    return true
}

// cmsTagTypeLinkedList represents a linked list of tag type handlers.
type cmsTagTypeLinkedList struct {
	Handler cmsTagTypeHandler
	Next    *cmsTagTypeLinkedList
}

// Infinites
const (
	MINUS_INF = -1e22
	PLUS_INF  = +1e22
)

// Type_XYZ_Read reads XYZ color space data.
func Type_XYZ_Read(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, SizeOfTag uint32) unsafe.Pointer {
	var xyz *cmsCIEXYZ

	*nItems = 0
	xyz = (*cmsCIEXYZ)(cmsMallocZero(self.ContextID, uint32(unsafe.Sizeof(cmsCIEXYZ{}))))
	if xyz == nil {
		return nil
	}

	if !cmsReadXYZNumber(io, xyz) {
		cmsFree(self.ContextID, unsafe.Pointer(xyz))
		return nil
	}

	*nItems = 1
	return unsafe.Pointer(xyz)
}

// Type_XYZ_Write writes XYZ color space data.
func Type_XYZ_Write(self *cmsTagTypeHandler, io *cmsIOHANDLER, Ptr unsafe.Pointer, nItems uint32) bool {
	return cmsWriteXYZNumber(io, (*cmsCIEXYZ)(Ptr))
}

// Type_XYZ_Dup duplicates XYZ color space data.
func Type_XYZ_Dup(self *cmsTagTypeHandler, Ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return cmsDupMem(self.ContextID, Ptr, uint32(unsafe.Sizeof(cmsCIEXYZ{})))
}

// Type_XYZ_Free frees XYZ color space data.
func Type_XYZ_Free(self *cmsTagTypeHandler, Ptr unsafe.Pointer) {
	cmsFree(self.ContextID, Ptr)
}

// DecideXYZtype decides the type of XYZ tag.
func DecideXYZtype(ICCVersion float64, Data unsafe.Pointer) cmsTagTypeSignature {
	return cmsSigXYZType
}

// cmsTagLinkedList represents a linked list of tag definitions.
type cmsTagLinkedList struct {
	Signature  cmsTagSignature
	Descriptor cmsTagDescriptor
	Next       *cmsTagLinkedList
}

// ********************************************************************************
// Type cmsSigLut8Type
// ********************************************************************************

// DecideLUTtypeA2B decides which LUT type to use when writing A2B LUTs.
func DecideLUTtypeA2B(ICCVersion float64, Data unsafe.Pointer) cmsTagTypeSignature {
	Lut := (*cmsPipeline)(Data)

	if ICCVersion < 4.0 {
		if Lut.SaveAs8Bits == cmsBoolTrue {
			return cmsSigLut8Type
		}
		return cmsSigLut16Type
	} else {
		return cmsSigLutAtoBType
	}
}

// DecideLUTtypeB2A decides which LUT type to use when writing B2A LUTs.
func DecideLUTtypeB2A(ICCVersion float64, Data unsafe.Pointer) cmsTagTypeSignature {
	Lut := (*cmsPipeline)(Data)

	if ICCVersion < 4.0 {
		if Lut.SaveAs8Bits == cmsBoolTrue {
			return cmsSigLut8Type
		}
		return cmsSigLut16Type
	} else {
		return cmsSigLutBtoAType
	}
}

// DecideCurveType decides which curve type to use when writing.
func DecideCurveType(ICCVersion float64, Data unsafe.Pointer) cmsTagTypeSignature {
	Curve := (*cmsToneCurve)(Data)

	if ICCVersion < 4.0 {
		return cmsSigCurveType
	}
	if Curve.nSegments != 1 {
		return cmsSigCurveType
	}
	if Curve.Segments[0].Type < 0 {
		return cmsSigCurveType
	}
	if Curve.Segments[0].Type > 5 {
		return cmsSigCurveType
	}

	return cmsSigParametricCurveType
}

// TypeParametricCurveRead reads a parametric curve from the IO handler.
func TypeParametricCurveRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, SizeOfTag uint32) *cmsToneCurve {
	paramsByType := []int{1, 3, 4, 5, 7}
	var params [10]float64
	var curveType uint16
	var newGamma *cmsToneCurve

	if !cmsReadUInt16Number(io, &curveType) {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unknown parametric curve type '%d'")
		return nil
	}
	if !cmsReadUInt16Number(io, nil) { // Reserved
		return nil
	}
	if curveType > 4 {
		return nil
	}

	nParams := paramsByType[curveType]

	for i := 0; i < nParams; i++ {
		if !cmsRead15Fixed16Number(io, &params[i]) {
			return nil
		}
	}

	newGamma = cmsBuildParametricToneCurve(self.ContextID, int32(curveType)+1, params[:nParams])
	*nItems = 1
	return newGamma
}

// TypeParametricCurveWrite writes a parametric curve to the IO handler.
func TypeParametricCurveWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	curve := (*cmsToneCurve)(ptr)
	paramsByType := []int{0, 1, 3, 4, 5, 7}

	typen := curve.Segments[0].Type

	if curve.nSegments > 1 || typen < 1 {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Multisegment or Inverted parametric curves cannot be written")
		return false
	}

	if typen > 5 {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unsupported parametric curve")
		return false
	}

	nParams := paramsByType[typen]

	if !cmsWriteUInt16Number(io, uint16(curve.Segments[0].Type-1)) {
		return false
	}
	if !cmsWriteUInt16Number(io, uint16(0)) {
		return false
	}

	for i := 0; i < nParams; i++ {
		if !cmsWrite15Fixed16Number(io, curve.Segments[0].Params[i]) {
			return false
		}
	}

	return true
}

// TypeParametricCurveDup duplicates a parametric curve.
func TypeParametricCurveDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsDupToneCurve((*cmsToneCurve)(ptr)))
}

// TypeParametricCurveFree frees a parametric curve.
func TypeParametricCurveFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFreeToneCurve((*cmsToneCurve)(ptr))
}

// Type_Text_Read reads a text type structure from the io handler.
func Type_Text_Read(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var text *byte
	var mlu *cmsMLU

	// Create a container
	mlu = cmsMLUalloc(self.ContextID, 1)
	if mlu == nil {
		return nil
	}

	*nItems = 0

	// Ensure valid size
	if sizeOfTag == 0xFFFFFFFF {
		goto Error
	}

	// Allocate memory for the text, with space for null terminator
	text = (*byte)(cmsMalloc(self.ContextID, uint32(uintptr(sizeOfTag)+1)))
	if text == nil {
		goto Error
	}

	// Read text from the IO handler
	if io.Read((*cms_io_handler)(io), unsafe.Pointer(text), 8, sizeOfTag) != 1 {
		goto Error
	}

	// Ensure null termination
	(*(*[1 << 30]byte)(unsafe.Pointer(text)))[sizeOfTag] = 0
	*nItems = 1

	// Store the result in the MLU
	if !cmsMLUsetASCII(mlu, cmsNoLanguage, cmsNoCountry, (*byte)(text), int(sizeOfTag)) {
		goto Error
	}

	cmsFree(self.ContextID, unsafe.Pointer(text))
	return unsafe.Pointer(mlu)

Error:
	if mlu != nil {
		cmsMLUfree(mlu)
	}
	if text != nil {
		cmsFree(self.ContextID, unsafe.Pointer(text))
	}
	return nil
}

// Type_Text_Write writes a text type structure to the io handler.
func Type_Text_Write(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	mlu := (*cmsMLU)(ptr)
	var size uint32
	var rc bool
	var text *byte

	// Get the size of the ASCII representation, including null terminator
	size = cmsMLUgetASCII(mlu, cmsNoLanguage, cmsNoCountry, nil, 0)
	if size == 0 {
		return false
	}

	// Allocate memory for the text
	text = (*byte)(cmsMalloc(self.ContextID, uint32(size)))
	if text == nil {
		return false
	}

	// Retrieve the ASCII text
	cmsMLUgetASCII(mlu, cmsNoLanguage, cmsNoCountry, (*byte)(text), size)

	// Write the text to the IO handler
	rc = io.Write((*cms_io_handler)(io), size, unsafe.Pointer(text))

	cmsFree(self.ContextID, unsafe.Pointer(text))
	return rc
}

// Type_Text_Dup duplicates a text type structure.
func Type_Text_Dup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) *cmsMLU {
	return cmsMLUdup((*cmsMLU)(ptr))
}

// Type_Text_Free frees a text type structure.
func Type_Text_Free(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsMLUfree((*cmsMLU)(ptr))
}

// DecideTextType determines the text type signature based on ICC version.
func DecideTextType(iccVersion float64, data unsafe.Pointer) cmsTagTypeSignature {
	if iccVersion >= 4.0 {
		return cmsSigMultiLocalizedUnicodeType
	}
	return cmsSigTextType
}

// ********************************************************************************
// Type cmsSigTextDescriptionType
// ********************************************************************************

// DecideTextDescType determines the type of text description
func DecideTextDescType(ICCVersion float64, data unsafe.Pointer) cmsTagTypeSignature {
	if ICCVersion >= 4.0 {
		return cmsSigMultiLocalizedUnicodeType
	}
	return cmsSigTextDescriptionType
}

// This tag can come IN UNALIGNED SIZE. In order to prevent issues, we force zeros on description to align it
// Type_Text_Description_Write writes a text description tag
func Type_Text_Description_Write(self *cmsTagTypeHandler, io *cmsIOHANDLER, Ptr unsafe.Pointer, nItems uint32) bool {
	mlu := (*cmsMLU)(Ptr)
	var Text *byte
	var Wide *uint16
	var lenASCII, lenText, lenTagRequirement, lenAligned uint32
	var rc bool = false
	var Filler [68]byte

	// Used below for writing zeroes
	for i := range Filler {
		Filler[i] = 0
	}

	// Get the len of string
	lenASCII = cmsMLUgetASCII(mlu, cmsNoLanguage, cmsNoCountry, nil, 0)
	// Specification ICC.1:2001-04 (v2.4.0): It has been found that textDescriptionType can contain misaligned data
	//(see clause 4.1 for the definition of 'aligned'). Because the Unicode language
	// code and Unicode count immediately follow the ASCII description, their
	// alignment is not correct if the ASCII count is not a multiple of four. The
	// ScriptCode code is misaligned when the ASCII count is odd. Profile reading and
	// writing software must be written carefully in order to handle these alignment
	// problems.
	//
	// The above last sentence suggest to handle alignment issues in the
	// parser. The provided example (Table 69 on Page 60) makes this clear.
	// The padding only in the ASCII count is not sufficient for a aligned tag
	// size, with the same text size in ASCII and Unicode.
	// Null strings
	// Null strings
	if lenASCII <= 0 {
		Text = (*byte)(cmsDupMem(self.ContextID, unsafe.Pointer(&[1]byte{0}), 1))
		Wide = (*uint16)(cmsDupMem(self.ContextID, unsafe.Pointer(&[1]uint16{0}), 1))
	} else {
		// Create independent buffers
		Text = (*byte)(cmsCalloc(self.ContextID, uint32(lenASCII), 1))
		if Text == nil {
			goto Error
		}

		Wide = (*uint16)(cmsCalloc(self.ContextID, uint32(lenASCII), 2))
		if Wide == nil {
			goto Error
		}

		// Get both representations
		cmsMLUgetASCII(mlu, cmsNoLanguage, cmsNoCountry, Text, lenASCII)
		cmsMLUgetWide(mlu, cmsNoLanguage, cmsNoCountry, Wide, lenASCII*2)
	}

	// Tell the real text len including the null terminator and padding
	lenText = uint32(lenASCII) + 1

	// Compute total tag size requirement
	lenTagRequirement = 8 + 4 + lenText + 4 + 4 + 2*lenText + 2 + 1 + 67
	lenAligned = cmsALIGNLONG(lenTagRequirement)

	// * cmsUInt32Number       count;          * Description length
	// * cmsInt8Number         desc[count]     * NULL terminated ascii string
	// * cmsUInt32Number       ucLangCode;     * UniCode language code
	// * cmsUInt32Number       ucCount;        * UniCode description length
	// * cmsInt16Number        ucDesc[ucCount];* The UniCode description
	// * cmsUInt16Number       scCode;         * ScriptCode code
	// * cmsUInt8Number        scCount;        * ScriptCode count
	// * cmsInt8Number         scDesc[67];     * ScriptCode Description
	// Write values
	if !cmsWriteUInt32Number(io, lenText) {
		goto Error
	}
	if !io.Write((*cms_io_handler)(io), lenText, unsafe.Pointer(Text)) {
		goto Error
	}

	if !cmsWriteUInt32Number(io, 0) { // ucLanguageCode
		goto Error
	}

	if !cmsWriteUInt32Number(io, lenText) {
		goto Error
	}

	// Note that in some compilers sizeof(cmsUInt16Number) != sizeof(wchar_t)
	if !cmsWriteWCharArray(io, lenText, Wide) {
		goto Error
	}

	// ScriptCode Code & count (unused)
	if !cmsWriteUInt16Number(io, 0) {
		goto Error
	}
	if !cmsWriteUInt8Number(io, 0) {
		goto Error
	}

	if !io.Write((*cms_io_handler)(io), 67, unsafe.Pointer(&Filler[0])) {
		goto Error
	}

	// Possibly add padding at the end of the tag
	if lenAligned > lenTagRequirement {
		if !io.Write((*cms_io_handler)(io), lenAligned-lenTagRequirement, unsafe.Pointer(&Filler[0])) {
			goto Error
		}
	}

	rc = true

Error:
	if Text != nil {
		cmsFree(self.ContextID, unsafe.Pointer(Text))
	}
	if Wide != nil {
		cmsFree(self.ContextID, unsafe.Pointer(Wide))
	}

	return rc
}

// Type_Text_Description_Dup duplicates a cmsMLU object
func Type_Text_Description_Dup(mlu *cmsMLU) *cmsMLU {
	return cmsMLUdup(mlu)
}

// Type_Text_Description_Free frees a cmsMLU object
func Type_Text_Description_Free(mlu *cmsMLU) {
	cmsMLUfree(mlu)
}

// Both kinds of plug-ins share the same structure
func cmsRegisterTagTypePlugin(id cmsContext, Data *cmsPluginBase) bool {
    return RegisterTypesPlugin(id, Data, TagTypePlugin)
}

func cmsRegisterMultiProcessElementPlugin(id cmsContext, Data *cmsPluginBase) bool {
    return RegisterTypesPlugin(id, Data, MPEPlugin)
}


// This is the list of built-in tags. The data of this list can be modified by plug-ins
func init() {
	var SupportedTags = []cmsTagLinkedList{
		{cmsSigAToB0Tag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigLut16Type, cmsSigLutAtoBType, cmsSigLut8Type}, DecideLUTtypeA2B}, nil},
		{cmsSigAToB1Tag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigLut16Type, cmsSigLutAtoBType, cmsSigLut8Type}, DecideLUTtypeA2B}, nil},
		{cmsSigAToB2Tag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigLut16Type, cmsSigLutAtoBType, cmsSigLut8Type}, DecideLUTtypeA2B}, nil},
		{cmsSigBToA0Tag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigLut16Type, cmsSigLutBtoAType, cmsSigLut8Type}, DecideLUTtypeB2A}, nil},
		{cmsSigBToA1Tag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigLut16Type, cmsSigLutBtoAType, cmsSigLut8Type}, DecideLUTtypeB2A}, nil},
		{cmsSigBToA2Tag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigLut16Type, cmsSigLutBtoAType, cmsSigLut8Type}, DecideLUTtypeB2A}, nil},
		{cmsSigRedColorantTag, cmsTagDescriptor{1, 2, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigXYZType, cmsCorbisBrokenXYZtype}, DecideXYZtype}, nil},
		{cmsSigGreenColorantTag, cmsTagDescriptor{1, 2, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigXYZType, cmsCorbisBrokenXYZtype}, DecideXYZtype}, nil},
		{cmsSigBlueColorantTag, cmsTagDescriptor{1, 2, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigXYZType, cmsCorbisBrokenXYZtype}, DecideXYZtype}, nil},
		{cmsSigRedTRCTag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigCurveType, cmsSigParametricCurveType, cmsMonacoBrokenCurveType}, DecideCurveType}, nil},
		{cmsSigGreenTRCTag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigCurveType, cmsSigParametricCurveType, cmsMonacoBrokenCurveType}, DecideCurveType}, nil},
		{cmsSigBlueTRCTag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigCurveType, cmsSigParametricCurveType, cmsMonacoBrokenCurveType}, DecideCurveType}, nil},
		{cmsSigCalibrationDateTimeTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigDateTimeType}, nil}, nil},
		{cmsSigCharTargetTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigTextType}, nil}, nil},
		{cmsSigChromaticAdaptationTag, cmsTagDescriptor{9, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigS15Fixed16ArrayType}, nil}, nil},
		{cmsSigChromaticityTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigChromaticityType}, nil}, nil},
		{cmsSigColorantOrderTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigColorantOrderType}, nil}, nil},
		{cmsSigColorantTableTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigColorantTableType}, nil}, nil},
		{cmsSigColorantTableOutTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigColorantTableType}, nil}, nil},
		{cmsSigCopyrightTag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigTextType, cmsSigMultiLocalizedUnicodeType, cmsSigTextDescriptionType}, DecideTextType}, nil},
		{cmsSigDateTimeTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigDateTimeType}, nil}, nil},
		{cmsSigDeviceMfgDescTag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigTextDescriptionType, cmsSigMultiLocalizedUnicodeType, cmsSigTextType}, DecideTextDescType}, nil},
		{cmsSigDeviceModelDescTag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigTextDescriptionType, cmsSigMultiLocalizedUnicodeType, cmsSigTextType}, DecideTextDescType}, nil},
		{cmsSigGamutTag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigLut16Type, cmsSigLutBtoAType, cmsSigLut8Type}, DecideLUTtypeB2A}, nil},
		{cmsSigGrayTRCTag, cmsTagDescriptor{1, 2, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigCurveType, cmsSigParametricCurveType}, DecideCurveType}, nil},
		{cmsSigLuminanceTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigXYZType}, nil}, nil},
		{cmsSigMediaBlackPointTag, cmsTagDescriptor{1, 2, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigXYZType, cmsCorbisBrokenXYZtype}, nil}, nil},
		{cmsSigMediaWhitePointTag, cmsTagDescriptor{1, 2, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigXYZType, cmsCorbisBrokenXYZtype}, nil}, nil},
		{cmsSigNamedColor2Tag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigNamedColor2Type}, nil}, nil},
		{cmsSigPreview0Tag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigLut16Type, cmsSigLutBtoAType, cmsSigLut8Type}, DecideLUTtypeB2A}, nil},
		{cmsSigPreview1Tag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigLut16Type, cmsSigLutBtoAType, cmsSigLut8Type}, DecideLUTtypeB2A}, nil},
		{cmsSigPreview2Tag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigLut16Type, cmsSigLutBtoAType, cmsSigLut8Type}, DecideLUTtypeB2A}, nil},
		{cmsSigProfileDescriptionTag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigTextDescriptionType, cmsSigMultiLocalizedUnicodeType, cmsSigTextType}, DecideTextDescType}, nil},
		{cmsSigProfileSequenceDescTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigProfileSequenceDescType}, nil}, nil},
		{cmsSigTechnologyTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigSignatureType}, nil}, nil},
		{cmsSigColorimetricIntentImageStateTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigSignatureType}, nil}, nil},
		{cmsSigPerceptualRenderingIntentGamutTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigSignatureType}, nil}, nil},
		{cmsSigSaturationRenderingIntentGamutTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigSignatureType}, nil}, nil},
		{cmsSigMeasurementTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigMeasurementType}, nil}, nil},
		{cmsSigPs2CRD0Tag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigDataType}, nil}, nil},
		{cmsSigPs2CRD1Tag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigDataType}, nil}, nil},
		{cmsSigPs2CRD2Tag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigDataType}, nil}, nil},
		{cmsSigPs2CRD3Tag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigDataType}, nil}, nil},
		{cmsSigPs2CSATag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigDataType}, nil}, nil},
		{cmsSigPs2RenderingIntentTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigDataType}, nil}, nil},
		{cmsSigViewingCondDescTag, cmsTagDescriptor{1, 3, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigTextDescriptionType, cmsSigMultiLocalizedUnicodeType, cmsSigTextType}, DecideTextDescType}, nil},
		{cmsSigUcrBgTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigUcrBgType}, nil}, nil},
		{cmsSigCrdInfoTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigCrdInfoType}, nil}, nil},
		{cmsSigDToB0Tag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigMultiProcessElementType}, nil}, nil},
		{cmsSigDToB1Tag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigMultiProcessElementType}, nil}, nil},
		{cmsSigDToB2Tag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigMultiProcessElementType}, nil}, nil},
		{cmsSigDToB3Tag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigMultiProcessElementType}, nil}, nil},
		{cmsSigBToD0Tag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigMultiProcessElementType}, nil}, nil},
		{cmsSigBToD1Tag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigMultiProcessElementType}, nil}, nil},
		{cmsSigBToD2Tag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigMultiProcessElementType}, nil}, nil},
		{cmsSigBToD3Tag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigMultiProcessElementType}, nil}, nil},
		{cmsSigScreeningDescTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigTextDescriptionType}, nil}, nil},
		{cmsSigViewingConditionsTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigViewingConditionsType}, nil}, nil},
		{cmsSigScreeningTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigScreeningType}, nil}, nil},
		{cmsSigVcgtTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigVcgtType}, nil}, nil},
		{cmsSigMetaTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigDictType}, nil}, nil},
		{cmsSigProfileSequenceIdTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigProfileSequenceIdType}, nil}, nil},
		{cmsSigProfileDescriptionMLTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigMultiLocalizedUnicodeType}, nil}, nil},
		{cmsSigcicpTag, cmsTagDescriptor{1, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigcicpType}, nil}, nil},
		{cmsSigArgyllArtsTag, cmsTagDescriptor{9, 1, [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature{cmsSigS15Fixed16ArrayType}, nil}, nil},
	}
	// Assign the Next pointers
	for i := 0; i < len(SupportedTags)-1; i++ {
		SupportedTags[i].Next = &SupportedTags[i+1]
	}
}


// _cmsRegisterTagPlugin registers a tag plugin.
func cmsRegisterTagPlugin(id cmsContext, Data *cmsPluginBase) bool {
    Plugin := (*cmsPluginTag)(unsafe.Pointer(Data))
    TagPluginChunk := (*cmsTagPluginChunkType)(cmsContextGetClientChunk(id, TagPlugin))

    // If Data is nil, unregister the plugin.
    if Data == nil {
        TagPluginChunk.Tag = nil
        return true
    }

    // Allocate memory for the new linked list node.
    pt := (*cmsTagLinkedList)(cmsPluginMalloc(id, uint32(unsafe.Sizeof(cmsTagLinkedList{}))))
    if pt == nil {
        return false
    }

    // Set the new node's values.
    pt.Signature = Plugin.Signature
    pt.Descriptor = Plugin.Descriptor
    pt.Next = TagPluginChunk.Tag

    // Update the head of the linked list.
    TagPluginChunk.Tag = pt

    return true
}

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
	"bytes"
	"fmt"
	"math"
	"syscall"
	"time"
	"unsafe"
)

type cmsTagTypeHandler struct {
	Signature  cmsTagTypeSignature
	ReadFn     func(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer
	WriteFn    func(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool
	DupFn      func(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer
	FreeFn     func(self *cmsTagTypeHandler, ptr unsafe.Pointer)
	ContextID  CmsContext
	ICCVersion uint32
}

// cmsTagTypeLinkedList represents a linked list of tag type handlers.
type cmsTagTypeLinkedList struct {
	Handler cmsTagTypeHandler
	Next    *cmsTagTypeLinkedList
}

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
func RegisterTypesPlugin(id CmsContext, Data *cmsPluginBase, pos cmsMemoryClient) bool {
	Plugin := (*cmsPluginTagType)(unsafe.Pointer(Data))
	ctx := (*cmsTagTypePluginChunkType)(CmsContextGetClientChunk(id, pos))

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

// To deal with position tables
type PositionTableEntryFn func(self *cmsTagTypeHandler, io *cmsIOHANDLER, Cargo unsafe.Pointer, n, SizeOfTag uint32) bool

func ReadPositionTable(self *cmsTagTypeHandler, io *cmsIOHANDLER, count, baseOffset uint32, cargo unsafe.Pointer, elementFn PositionTableEntryFn) bool {
	var currentPosition uint32
	currentPosition = uint32(io.Tell((*cms_io_handler)(io)))
	var elementSizes []uint32
	// Verify there is enough space left to read at least two uint32 items for count items
	if ((io.ReportedSize - currentPosition) / (2 * uint32(unsafe.Sizeof(uint32(0))))) < count {
		return false
	}

	// Allocate memory for offsets and sizes
	elementOffsets := (*[1 << 30]uint32)(cmsCalloc(io.ContextID, count, uint32(unsafe.Sizeof(uint32(0)))))[:count:count]
	if elementOffsets == nil {
		goto Error
	}

	elementSizes = (*[1 << 30]uint32)(cmsCalloc(io.ContextID, count, uint32(unsafe.Sizeof(uint32(0)))))[:count:count]
	if elementSizes == nil {
		goto Error
	}

	// Read the offsets and sizes
	for i := uint32(0); i < count; i++ {
		if !cmsReadUInt32Number(io, &elementOffsets[i]) || !cmsReadUInt32Number(io, &elementSizes[i]) {
			goto Error
		}
		elementOffsets[i] += baseOffset
	}

	// Seek to each element and read it
	for i := uint32(0); i < count; i++ {
		if !io.Seek((*cms_io_handler)(io), uint32(elementOffsets[i])) {
			goto Error
		}
		// Call the reader callback
		if !elementFn(self, io, cargo, i, elementSizes[i]) {
			goto Error
		}
	}

	// Success
	cmsFree(io.ContextID, unsafe.Pointer(&elementOffsets[0]))
	cmsFree(io.ContextID, unsafe.Pointer(&elementSizes[0]))
	return true

Error:
	cmsFree(io.ContextID, unsafe.Pointer(&elementOffsets[0]))
	cmsFree(io.ContextID, unsafe.Pointer(&elementSizes[0]))
	return false
}

func WritePositionTable(self *cmsTagTypeHandler, io *cmsIOHANDLER, sizeOfTag, count, baseOffset uint32, cargo unsafe.Pointer, elementFn PositionTableEntryFn) bool {
	// Allocate memory for offsets and sizes
	var currentPos uint32
	var directoryPos uint32
	var elementSizes []uint32

	elementOffsets := (*[1 << 30]uint32)(cmsCalloc(io.ContextID, count, uint32(unsafe.Sizeof(uint32(0)))))[:count:count]
	if elementOffsets == nil {
		goto Error
	}

	elementSizes = (*[1 << 30]uint32)(cmsCalloc(io.ContextID, count, uint32(unsafe.Sizeof(uint32(0)))))[:count:count]
	if elementSizes == nil {
		goto Error
	}

	// Keep starting position of curve offsets
	directoryPos = uint32(io.Tell((*cms_io_handler)(io)))

	// Write a fake directory to be filled later
	for i := uint32(0); i < count; i++ {
		if !cmsWriteUInt32Number(io, 0) || !cmsWriteUInt32Number(io, 0) {
			goto Error
		}
	}

	// Write each element and keep track of size
	for i := uint32(0); i < count; i++ {
		before := uint32(io.Tell((*cms_io_handler)(io)))
		elementOffsets[i] = before - baseOffset

		// Callback to write
		if !elementFn(self, io, cargo, i, sizeOfTag) {
			goto Error
		}

		// Calculate the size
		elementSizes[i] = uint32(io.Tell((*cms_io_handler)(io))) - before
	}

	// Write the directory
	currentPos = uint32(io.Tell((*cms_io_handler)(io)))
	if !io.Seek((*cms_io_handler)(io), directoryPos) {
		goto Error
	}

	for i := uint32(0); i < count; i++ {
		if !cmsWriteUInt32Number(io, elementOffsets[i]) || !cmsWriteUInt32Number(io, elementSizes[i]) {
			goto Error
		}
	}

	if !io.Seek((*cms_io_handler)(io), currentPos) {
		goto Error
	}

	// Success
	cmsFree(io.ContextID, unsafe.Pointer(&elementOffsets[0]))
	cmsFree(io.ContextID, unsafe.Pointer(&elementSizes[0]))
	return true

Error:
	cmsFree(io.ContextID, unsafe.Pointer(&elementOffsets[0]))
	cmsFree(io.ContextID, unsafe.Pointer(&elementSizes[0]))
	return false
}

// Type_XYZ_Read reads XYZ color space data.
func TypeXYZRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, SizeOfTag uint32) unsafe.Pointer {
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
func TypeXYZWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, Ptr unsafe.Pointer, nItems uint32) bool {
	return cmsWriteXYZNumber(io, (*cmsCIEXYZ)(Ptr))
}

// Type_XYZ_Dup duplicates XYZ color space data.
func TypeXYZDup(self *cmsTagTypeHandler, Ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return cmsDupMem(self.ContextID, Ptr, uint32(unsafe.Sizeof(cmsCIEXYZ{})))
}

// Type_XYZ_Free frees XYZ color space data.
func TypeXYZFree(self *cmsTagTypeHandler, Ptr unsafe.Pointer) {
	cmsFree(self.ContextID, Ptr)
}

// DecideXYZtype decides the type of XYZ tag.
func DecideXYZtype(ICCVersion float64, Data unsafe.Pointer) cmsTagTypeSignature {
	return cmsSigXYZType
}

// ********************************************************************************
// Type cmsSigLut8Type
// ********************************************************************************

// DecideLUTtypeA2B decides which LUT type to use when writing A2B LUTs.
func DecideLUTtypeA2B(ICCVersion float64, Data unsafe.Pointer) cmsTagTypeSignature {
	Lut := (*cmsPipeline)(Data)

	if ICCVersion < 4.0 {
		if Lut.SaveAs8Bits {
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
		if Lut.SaveAs8Bits {
			return cmsSigLut8Type
		}
		return cmsSigLut16Type
	} else {
		return cmsSigLutBtoAType
	}
}

/*
firstSegment := (*cmsCurveSegment)(unsafe.Pointer(curve.Segments))
segmentPtr := (*cmsCurveSegment)(unsafe.Addunsafe.Pointer(curve.Segments), uintptr(index) * unsafe.Sizeof(cmsCurveSegment{})))
*/

// DecideCurveType decides which curve type to use when writing.
func DecideCurveType(ICCVersion float64, Data unsafe.Pointer) cmsTagTypeSignature {
	Curve := (*CmsToneCurve)(Data)

	if ICCVersion < 4.0 {
		return cmsSigCurveType
	}
	if Curve.nSegments != 1 {
		return cmsSigCurveType
	}
	if (*cmsCurveSegment)(unsafe.Pointer(Curve.Segments)).Type < 0 {
		return cmsSigCurveType
	}
	if (*cmsCurveSegment)(unsafe.Pointer(Curve.Segments)).Type > 5 {
		return cmsSigCurveType
	}

	return cmsSigParametricCurveType
}

// TypeParametricCurveRead reads a parametric curve from the IO handler.
func TypeParametricCurveRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, SizeOfTag uint32) unsafe.Pointer {
	paramsByType := []int{1, 3, 4, 5, 7}
	var params [10]float64
	var curveType uint16
	var newGamma *CmsToneCurve

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

	newGamma = cmsBuildParametricToneCurve(self.ContextID, int(curveType+1), &params[0])
	*nItems = 1
	return unsafe.Pointer(newGamma)
}

// TypeParametricCurveWrite writes a parametric curve to the IO handler.
func TypeParametricCurveWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	curve := (*CmsToneCurve)(ptr)
	paramsByType := []int{0, 1, 3, 4, 5, 7}

	typen := (*cmsCurveSegment)(unsafe.Pointer(curve.Segments)).Type

	if curve.nSegments > 1 || typen < 1 {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Multisegment or Inverted parametric curves cannot be written")
		return false
	}

	if typen > 5 {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unsupported parametric curve")
		return false
	}

	nParams := paramsByType[typen]

	if !cmsWriteUInt16Number(io, uint16((*cmsCurveSegment)(unsafe.Pointer(curve.Segments)).Type-1)) {
		return false
	}
	if !cmsWriteUInt16Number(io, uint16(0)) {
		return false
	}

	for i := 0; i < nParams; i++ {
		if !cmsWrite15Fixed16Number(io, (*cmsCurveSegment)(unsafe.Pointer(curve.Segments)).Params[i]) {
			return false
		}
	}

	return true
}

// TypeParametricCurveDup duplicates a parametric curve.
func TypeParametricCurveDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsDupToneCurve((*CmsToneCurve)(ptr)))
}

// TypeParametricCurveFree frees a parametric curve.
func TypeParametricCurveFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	CmsFreeToneCurve((*CmsToneCurve)(ptr))
}

// Type_Text_Read reads a text type structure from the io handler.
func TypeTextRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var text *byte

	// Create a container
	mlu := cmsMLUalloc(self.ContextID, 1)
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
	if !cmsMLUsetASCII(mlu, cmsNoLanguage, cmsNoCountry, (*byte)(text)) {
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
func TypeTextWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
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
func TypeTextDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsMLUdup((*cmsMLU)(ptr)))
}

// Type_Text_Free frees a text type structure.
func TypeTextFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
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
func TypeTextDescriptionRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var (
		text            *byte
		textend         *byte
		mlu             *cmsMLU
		asciiCount      uint32
		unicodeCode     uint32
		unicodeCount    uint32
		scriptCodeCode  uint16
		dummy           uint16
		scriptCodeCount uint8
		i               uint32
	)

	*nItems = 0

	// Check if size of tag is at least one DWORD
	if sizeOfTag < 4 {
		return nil
	}

	// Read ASCII count
	if !cmsReadUInt32Number(io, &asciiCount) {
		return nil
	}
	sizeOfTag -= 4

	// Check if tag size is sufficient
	if sizeOfTag < asciiCount {
		return nil
	}

	// Allocate the MLU
	mlu = cmsMLUalloc(self.ContextID, 1)
	if mlu == nil {
		return nil
	}

	// Allocate memory for the text
	text = (*byte)(cmsMalloc(self.ContextID, asciiCount+1))
	if text == nil {
		goto Error
	}

	// Read ASCII text
	if io.Read((*cms_io_handler)(io), unsafe.Pointer(text), 1, asciiCount) != asciiCount {
		goto Error
	}
	sizeOfTag -= asciiCount

	// Ensure null-terminated string
	textend = (*byte)(unsafe.Add(unsafe.Pointer(text), uintptr(asciiCount)*unsafe.Sizeof(byte(0))))
	*textend = 0

	// Set the MLU entry
	if !cmsMLUsetASCII(mlu, cmsNoLanguage, cmsNoCountry, text) {
		goto Error
	}
	cmsFree(self.ContextID, unsafe.Pointer(text))

	text = nil // Text no longer needed

	// Skip Unicode code
	if sizeOfTag < 8 {
		goto Done
	}
	if !cmsReadUInt32Number(io, &unicodeCode) || !cmsReadUInt32Number(io, &unicodeCount) {
		goto Done
	}
	sizeOfTag -= 8

	if sizeOfTag < unicodeCount*2 {
		goto Done
	}

	for i = 0; i < unicodeCount; i++ {
		if io.Read((*cms_io_handler)(io), unsafe.Pointer(&dummy), 2, 1) != 1 {
			goto Done
		}
	}
	sizeOfTag -= unicodeCount * 2

	// Skip ScriptCode code if present. Some buggy profiles does have less
	// data that stricttly required. We need to skip it as this type may come
	// embedded in other types.
	if sizeOfTag >= uint32(unsafe.Sizeof(uint16(0)))+uint32(unsafe.Sizeof(uint8(0)))+67 {
		if !cmsReadUInt16Number(io, &scriptCodeCode) || !cmsReadUInt8Number(io, &scriptCodeCount) {
			goto Done
		}

		for i = 0; i < 67; i++ {
			if io.Read((*cms_io_handler)(io), unsafe.Pointer(&dummy), 1, 1) != 1 {
				goto Error
			}
		}
	}

Done:
	*nItems = 1
	return unsafe.Pointer(mlu)

Error:
	if text != nil {
		cmsFree(self.ContextID, unsafe.Pointer(text))
	}
	if mlu != nil {
		cmsMLUfree(mlu)
	}
	return nil
}

// This tag can come IN UNALIGNED SIZE. In order to prevent issues, we force zeros on description to align it
// Type_Text_Description_Write writes a text description tag
func TypeTextDescriptionWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, Ptr unsafe.Pointer, nItems uint32) bool {
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
	// * uint16       scCode;         * ScriptCode code
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

	// Note that in some compilers sizeof(uint16) != sizeof(wchar_t)
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
func TypeTextDescriptionDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, nItems uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsMLUdup((*cmsMLU)(ptr)))
}

// Type_Text_Description_Free frees a cmsMLU object
func TypeTextDescriptionFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsMLUfree((*cmsMLU)(ptr))
}

// Both kinds of plug-ins share the same structure
func cmsRegisterTagTypePlugin(id CmsContext, Data *cmsPluginBase) bool {
	return RegisterTypesPlugin(id, Data, TagTypePlugin)
}

func cmsRegisterMultiProcessElementPlugin(id CmsContext, Data *cmsPluginBase) bool {
	return RegisterTypesPlugin(id, Data, MPEPlugin)
}

// Return handler for a given type or NULL if not found. Shared between normal types and MPE. It first tries the additons
// made by plug-ins and then the built-in defaults.
// GetHandler returns the handler for a given type signature.
// It first tries the additions made by plug-ins and then the built-in defaults.
func GetHandler(
	sig cmsTagTypeSignature,
	PluginLinkedList *cmsTagTypeLinkedList,
	DefaultLinkedList *cmsTagTypeLinkedList,
) *cmsTagTypeHandler {
	// Check in the plugin-linked list
	for pt := PluginLinkedList; pt != nil; pt = pt.Next {
		if sig == pt.Handler.Signature {
			return &pt.Handler
		}
	}

	// Check in the default-linked list
	for pt := DefaultLinkedList; pt != nil; pt = pt.Next {
		if sig == pt.Handler.Signature {
			return &pt.Handler
		}
	}

	// Return nil if no handler is found
	return nil
}

// Wrapper for tag types
func cmsGetTagTypeHandler(ContextID CmsContext, sig cmsTagTypeSignature) *cmsTagTypeHandler {
	ctx := (*cmsTagTypePluginChunkType)(CmsContextGetClientChunk(ContextID, TagTypePlugin))

	return GetHandler(sig, ctx.TagTypes, (*cmsTagTypeLinkedList)(&SupportedTagTypes[0]))
}

// ********************************************************************************
// Tag support main routines
// ********************************************************************************

// cmsTagLinkedList represents a linked list of tag definitions.
type cmsTagLinkedList struct {
	Signature  cmsTagSignature
	Descriptor cmsTagDescriptor
	Next       *cmsTagLinkedList
}

var SupportedTags []cmsTagLinkedList
var SupportedTagTypes []cmsTagTypeLinkedList
var SupportedMPEtypes []cmsTagTypeLinkedList

var cmsTagTypePluginChunk = cmsTagTypePluginChunkType{TagTypes: nil}

var cmsTagPluginChunk = cmsTagPluginChunkType{Tag: nil}
var cmsMPETypePluginChunk = cmsTagTypePluginChunkType{TagTypes: nil}

// Definition of SupportedMPEtypes using cmsTagTypeHandler and cmsTagTypeLinkedList

// This is the list of built-in tags. The data of this list can be modified by plug-ins
func init() {
	SupportedTags = []cmsTagLinkedList{
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

	// ********************************************************************************
	// Type support main routines
	// ********************************************************************************
	// Definition of SupportedTagTypes using cmsTagTypeHandler
	SupportedTagTypes = []cmsTagTypeLinkedList{
		{cmsTagTypeHandler{Signature: cmsSigChromaticityType, ReadFn: TypeChromaticityRead, WriteFn: TypeChromaticityWrite, DupFn: TypeChromaticityDup, FreeFn: TypeChromaticityFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigColorantOrderType, ReadFn: TypeColorantOrderTypeRead, WriteFn: TypeColorantOrderTypeWrite, DupFn: TypeColorantOrderTypeDup, FreeFn: TypeColorantOrderTypeFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigS15Fixed16ArrayType, ReadFn: TypeS15Fixed16Read, WriteFn: TypeS15Fixed16Write, DupFn: TypeS15Fixed16Dup, FreeFn: TypeS15Fixed16Free}, nil},
		{cmsTagTypeHandler{Signature: cmsSigU16Fixed16ArrayType, ReadFn: TypeU16Fixed16Read, WriteFn: TypeU16Fixed16Write, DupFn: TypeU16Fixed16Dup, FreeFn: TypeU16Fixed16Free}, nil},
		{cmsTagTypeHandler{Signature: cmsSigTextType, ReadFn: TypeTextRead, WriteFn: TypeTextWrite, DupFn: TypeTextDup, FreeFn: TypeTextFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigTextDescriptionType, ReadFn: TypeTextDescriptionRead, WriteFn: TypeTextDescriptionWrite, DupFn: TypeTextDescriptionDup, FreeFn: TypeTextDescriptionFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigCurveType, ReadFn: TypeCurveRead, WriteFn: TypeCurveWrite, DupFn: TypeCurveDup, FreeFn: TypeCurveFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigParametricCurveType, ReadFn: TypeParametricCurveRead, WriteFn: TypeParametricCurveWrite, DupFn: TypeParametricCurveDup, FreeFn: TypeParametricCurveFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigDateTimeType, ReadFn: TypeDateTimeRead, WriteFn: TypeDateTimeWrite, DupFn: TypeDateTimeDup, FreeFn: TypeDateTimeFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigLut8Type, ReadFn: TypeLUT8Read, WriteFn: TypeLUT8Write, DupFn: TypeLUT8Dup, FreeFn: TypeLUT8Free}, nil},
		{cmsTagTypeHandler{Signature: cmsSigLut16Type, ReadFn: TypeLUT16Read, WriteFn: TypeLUT16Write, DupFn: TypeLUT16Dup, FreeFn: TypeLUT16Free}, nil},
		{cmsTagTypeHandler{Signature: cmsSigColorantTableType, ReadFn: TypeColorantTableRead, WriteFn: TypeColorantTableWrite, DupFn: TypeColorantTableDup, FreeFn: TypeColorantTableFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigNamedColor2Type, ReadFn: TypeNamedColorRead, WriteFn: TypeNamedColorWrite, DupFn: TypeNamedColorDup, FreeFn: TypeNamedColorFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigMultiLocalizedUnicodeType, ReadFn: TypeMLURead, WriteFn: TypeMLUWrite, DupFn: TypeMLUDup, FreeFn: TypeMLUFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigProfileSequenceDescType, ReadFn: TypeProfileSequenceDescRead, WriteFn: TypeProfileSequenceDescWrite, DupFn: TypeProfileSequenceDescDup, FreeFn: TypeProfileSequenceDescFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigSignatureType, ReadFn: TypeSignatureRead, WriteFn: TypeSignatureWrite, DupFn: TypeSignatureDup, FreeFn: TypeSignatureFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigMeasurementType, ReadFn: TypeMeasurementRead, WriteFn: TypeMeasurementWrite, DupFn: TypeMeasurementDup, FreeFn: TypeMeasurementFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigDataType, ReadFn: TypeDataRead, WriteFn: TypeDataWrite, DupFn: TypeDataDup, FreeFn: TypeDataFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigLutAtoBType, ReadFn: TypeLUTA2BRead, WriteFn: TypeLUTA2BWrite, DupFn: TypeLUTA2BDup, FreeFn: TypeLUTA2BFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigLutBtoAType, ReadFn: TypeLUTB2ARead, WriteFn: TypeLUTB2AWrite, DupFn: TypeLUTB2ADup, FreeFn: TypeLUTB2AFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigUcrBgType, ReadFn: TypeUcrBgRead, WriteFn: TypeUcrBgWrite, DupFn: TypeUcrBgDup, FreeFn: TypeUcrBgFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigCrdInfoType, ReadFn: TypeCrdInfoRead, WriteFn: TypeCrdInfoWrite, DupFn: TypeCrdInfoDup, FreeFn: TypeCrdInfoFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigMultiProcessElementType, ReadFn: TypeMPERead, WriteFn: TypeMPEWrite, DupFn: TypeMPEDup, FreeFn: TypeMPEFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigScreeningType, ReadFn: TypeScreeningRead, WriteFn: TypeScreeningWrite, DupFn: TypeScreeningDup, FreeFn: TypeScreeningFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigViewingConditionsType, ReadFn: TypeViewingConditionsRead, WriteFn: TypeViewingConditionsWrite, DupFn: TypeViewingConditionsDup, FreeFn: TypeViewingConditionsFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigXYZType, ReadFn: TypeXYZRead, WriteFn: TypeXYZWrite, DupFn: TypeXYZDup, FreeFn: TypeXYZFree}, nil},
		{cmsTagTypeHandler{Signature: cmsCorbisBrokenXYZtype, ReadFn: TypeXYZRead, WriteFn: TypeXYZWrite, DupFn: TypeXYZDup, FreeFn: TypeXYZFree}, nil},
		{cmsTagTypeHandler{Signature: cmsMonacoBrokenCurveType, ReadFn: TypeCurveRead, WriteFn: TypeCurveWrite, DupFn: TypeCurveDup, FreeFn: TypeCurveFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigProfileSequenceIdType, ReadFn: TypeProfileSequenceIdRead, WriteFn: TypeProfileSequenceIdWrite, DupFn: TypeProfileSequenceIdDup, FreeFn: TypeProfileSequenceIdFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigDictType, ReadFn: TypeDictionaryRead, WriteFn: TypeDictionaryWrite, DupFn: TypeDictionaryDup, FreeFn: TypeDictionaryFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigcicpType, ReadFn: TypeVideoSignalRead, WriteFn: TypeVideoSignalWrite, DupFn: TypeVideoSignalDup, FreeFn: TypeVideoSignalFree}, nil},
		{cmsTagTypeHandler{Signature: cmsSigVcgtType, ReadFn: TypeVcgtRead, WriteFn: TypeVcgtWrite, DupFn: TypeVcgtDup, FreeFn: TypeVcgtFree}, nil},
	}

	// Assign the Next pointers
	for i := 0; i < len(SupportedTagTypes)-1; i++ {
		SupportedTagTypes[i].Next = &SupportedTagTypes[i+1]
	}
	/*typempecurveread := TypeMPEcurveRead
	typempecurvewrite := TypeMPEcurveWrite
	genericmpedup := GenericMPEDup
	genericmpefree := GenericMPEFree*/
	SupportedMPEtypes = []cmsTagTypeLinkedList{
		{
			Handler: cmsTagTypeHandler{
				Signature: cmsTagTypeSignature(cmsSigBAcsElemType),
				ReadFn:    nil, // Ignored elements
				WriteFn:   nil, // Ignored elements
				DupFn:     nil,
				FreeFn:    nil,
			},
			Next: nil, // Will be set later
		},
		{
			Handler: cmsTagTypeHandler{
				Signature: cmsTagTypeSignature(cmsSigEAcsElemType),
				ReadFn:    nil, // Ignored elements
				WriteFn:   nil, // Ignored elements
				DupFn:     nil,
				FreeFn:    nil,
			},
			Next: nil, // Will be set later
		},
		{
			Handler: cmsTagTypeHandler{
				Signature: cmsTagTypeSignature(cmsSigCurveSetElemType),
				ReadFn:    TypeMPEcurveRead,  // Specific function for reading MPE curves
				WriteFn:   TypeMPEcurveWrite, // Specific function for writing MPE curves
				DupFn:     GenericMPEDup,
				FreeFn:    GenericMPEFree,
			},
			Next: nil, // Will be set later
		},
		{
			Handler: cmsTagTypeHandler{
				Signature: cmsTagTypeSignature(cmsSigMatrixElemType),
				ReadFn:    TypeMPEmatrixRead,  // Specific function for reading matrices
				WriteFn:   TypeMPEmatrixWrite, // Specific function for writing matrices
				DupFn:     GenericMPEDup,
				FreeFn:    GenericMPEFree,
			},
			Next: nil, // Will be set later
		},
		{
			Handler: cmsTagTypeHandler{
				Signature: cmsTagTypeSignature(cmsSigCLutElemType),
				ReadFn:    TypeMPEclutRead,  // Specific function for reading CLUTs
				WriteFn:   TypeMPEclutWrite, // Specific function for writing CLUTs
				DupFn:     GenericMPEDup,
				FreeFn:    GenericMPEFree,
			},
			Next: nil, // Last element, no next pointer
		},
	}

	// Dynamically set the Next pointers for the linked list
	for i := 0; i < len(SupportedMPEtypes)-1; i++ {
		SupportedMPEtypes[i].Next = &SupportedMPEtypes[i+1]
	}

}

// cmsRegisterTagPlugin registers a tag plugin.
func cmsRegisterTagPlugin(id CmsContext, Data *cmsPluginBase) bool {
	Plugin := (*cmsPluginTag)(unsafe.Pointer(Data))
	TagPluginChunk := (*cmsTagPluginChunkType)(CmsContextGetClientChunk(id, TagPlugin))

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

// cmsGetTagDescriptor returns a descriptor for a given tag or nil.
func cmsGetTagDescriptor(ContextID CmsContext, sig cmsTagSignature) *cmsTagDescriptor {
	// Retrieve the TagPluginChunk from the context.
	TagPluginChunk := (*cmsTagPluginChunkType)(CmsContextGetClientChunk(ContextID, TagPlugin))

	// Check in the linked list of plugins.
	for pt := TagPluginChunk.Tag; pt != nil; pt = pt.Next {
		if sig == pt.Signature {
			return &pt.Descriptor
		}
	}

	// Check in the list of supported tags using the manually assigned Next pointers
	for pt := &SupportedTags[0]; pt != nil; pt = pt.Next {
		if sig == pt.Signature {
			return &pt.Descriptor
		}
	}

	// If not found, return nil.
	return nil
}

// ********************************************************************************
// Type cmsSigScreeningType
// ********************************************************************************
//
// The screeningType describes various screening parameters including screen
// frequency, screening angle, and spot shape.
func TypeScreeningRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	sc := (*cmsScreening)(cmsMallocZero(self.ContextID, uint32(unsafe.Sizeof(cmsScreening{}))))
	if sc == nil {
		return nil
	}
	*nItems = 0

	if !cmsReadUInt32Number(io, &sc.Flag) || !cmsReadUInt32Number(io, &sc.NChannels) {
		goto Error
	}

	if sc.NChannels > cmsMAXCHANNELS-1 {
		sc.NChannels = cmsMAXCHANNELS - 1
	}

	for i := uint32(0); i < sc.NChannels; i++ {
		if !cmsRead15Fixed16Number(io, &sc.Channels[i].Frequency) ||
			!cmsRead15Fixed16Number(io, &sc.Channels[i].ScreenAngle) ||
			!cmsReadUInt32Number(io, &sc.Channels[i].SpotShape) {
			goto Error
		}
	}

	*nItems = 1
	return unsafe.Pointer(sc)

Error:
	if sc != nil {
		cmsFree(self.ContextID, unsafe.Pointer(sc))
	}
	return nil
}

func TypeScreeningWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	sc := (*cmsScreening)(ptr)

	if !cmsWriteUInt32Number(io, sc.Flag) || !cmsWriteUInt32Number(io, sc.NChannels) {
		return false
	}

	for i := uint32(0); i < sc.NChannels; i++ {
		if !cmsWrite15Fixed16Number(io, sc.Channels[i].Frequency) ||
			!cmsWrite15Fixed16Number(io, sc.Channels[i].ScreenAngle) ||
			!cmsWriteUInt32Number(io, sc.Channels[i].SpotShape) {
			return false
		}
	}

	return true
}

func TypeScreeningDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return cmsDupMem(nil, ptr, uint32(unsafe.Sizeof(cmsScreening{})))
}

func TypeScreeningFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFree(nil, ptr)
}
func TypeViewingConditionsRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	vc := (*cmsICCViewingConditions)(cmsMallocZero(self.ContextID, uint32(unsafe.Sizeof(cmsICCViewingConditions{}))))
	*nItems = 0
	if vc == nil {
		return nil
	}

	if !cmsReadXYZNumber(io, &vc.IlluminantXYZ) || !cmsReadXYZNumber(io, &vc.SurroundXYZ) || !cmsReadUInt32Number(io, &vc.IlluminantType) {
		return nil
	}

	*nItems = 1
	return unsafe.Pointer(vc)
}

func TypeViewingConditionsWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	vc := (*cmsICCViewingConditions)(ptr)

	return cmsWriteXYZNumber(io, &vc.IlluminantXYZ) &&
		cmsWriteXYZNumber(io, &vc.SurroundXYZ) &&
		cmsWriteUInt32Number(io, vc.IlluminantType)
}

func TypeViewingConditionsDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return cmsDupMem(nil, ptr, uint32(unsafe.Sizeof(cmsICCViewingConditions{})))
}

func TypeViewingConditionsFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFree(nil, ptr)
}

// Type_Chromaticity_Read reads a Chromaticity type from the IO handler.
func TypeChromaticityRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	*nItems = 0

	// Allocate memory for CmsCIExyYTRIPLE
	chrm := (*CmsCIExyYTRIPLE)(cmsMallocZero(self.ContextID, uint32(unsafe.Sizeof(CmsCIExyYTRIPLE{}))))
	if chrm == nil {
		return nil
	}

	var nChans uint16
	var table uint16

	if !cmsReadUInt16Number(io, &nChans) {
		goto Error
	}

	// Recover from a bug in early versions
	if nChans == 0 && sizeOfTag == 32 {
		if !cmsReadUInt16Number(io, nil) || !cmsReadUInt16Number(io, &nChans) {
			goto Error
		}
	}

	if nChans != 3 {
		goto Error
	}

	if !cmsReadUInt16Number(io, &table) ||
		!cmsRead15Fixed16Number(io, &chrm.Red.x) ||
		!cmsRead15Fixed16Number(io, &chrm.Red.y) {
		goto Error
	}

	chrm.Red.Y = 1.0

	if !cmsRead15Fixed16Number(io, &chrm.Green.x) ||
		!cmsRead15Fixed16Number(io, &chrm.Green.y) {
		goto Error
	}

	chrm.Green.Y = 1.0

	if !cmsRead15Fixed16Number(io, &chrm.Blue.x) ||
		!cmsRead15Fixed16Number(io, &chrm.Blue.y) {
		goto Error
	}

	chrm.Blue.Y = 1.0

	*nItems = 1
	return unsafe.Pointer(chrm)

Error:
	cmsFree(self.ContextID, unsafe.Pointer(chrm))
	return nil
}

// SaveOneChromaticity writes a single chromaticity point.
func SaveOneChromaticity(x, y float64, io *cmsIOHANDLER) bool {
	if !cmsWriteUInt32Number(io, uint32(cmsDoubleTo15Fixed16(x))) ||
		!cmsWriteUInt32Number(io, uint32(cmsDoubleTo15Fixed16(y))) {
		return false
	}
	return true
}

// Type_Chromaticity_Write writes a Chromaticity type to the IO handler.
func TypeChromaticityWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	chrm := (*CmsCIExyYTRIPLE)(ptr)

	if !cmsWriteUInt16Number(io, 3) || // nChannels
		!cmsWriteUInt16Number(io, 0) { // Table
		return false
	}

	if !SaveOneChromaticity(chrm.Red.x, chrm.Red.y, io) ||
		!SaveOneChromaticity(chrm.Green.x, chrm.Green.y, io) ||
		!SaveOneChromaticity(chrm.Blue.x, chrm.Blue.y, io) {
		return false
	}

	return true
}

// Type_Chromaticity_Dup duplicates a Chromaticity structure.
func TypeChromaticityDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return cmsDupMem(self.ContextID, ptr, uint32(unsafe.Sizeof(CmsCIExyYTRIPLE{})))
}

// Type_Chromaticity_Free frees the memory allocated for a Chromaticity structure.
func TypeChromaticityFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFree(self.ContextID, ptr)
}

// ********************************************************************************
// Type cmsSigColorantOrderType
// ********************************************************************************

// This is an optional tag which specifies the laydown order in which colorants will
// be printed on an n-colorant device. The laydown order may be the same as the
// channel generation order listed in the colorantTableTag or the channel order of a
// colour space such as CMYK, in which case this tag is not needed. When this is not
// the case (for example, ink-towers sometimes use the order KCMY), this tag may be
// used to specify the laydown order of the colorants.

func TypeColorantOrderTypeRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	// Allocate memory for ColorantOrder
	colorantOrder := (*[cmsMAXCHANNELS]uint8)(cmsCalloc(self.ContextID, cmsMAXCHANNELS, uint32(unsafe.Sizeof(uint8(0)))))
	if colorantOrder == nil {
		return nil
	}

	*nItems = 0

	var count uint32
	if !cmsReadUInt32Number(io, &count) || count > cmsMAXCHANNELS {
		return nil
	}
	ColorantOrder := (*uint8)(cmsCalloc(self.ContextID, cmsMAXCHANNELS, uint32(unsafe.Sizeof(uint8(0)))))
	if ColorantOrder == nil {
		return nil
	}

	// Set all elements to 0xFF as end marker
	memset(unsafe.Pointer(ColorantOrder), 0xFF, cmsMAXCHANNELS*unsafe.Sizeof(uint8(0)))

	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&colorantOrder[0]), uint32(unsafe.Sizeof(uint8(0))), count) != 0 {
		cmsFree(self.ContextID, unsafe.Pointer(colorantOrder))
		return nil
	}

	*nItems = 1
	return unsafe.Pointer(colorantOrder)
}

func TypeColorantOrderTypeWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	colorantOrder := (*[cmsMAXCHANNELS]uint8)(ptr)

	// Calculate count
	var count uint32
	for i := 0; i < cmsMAXCHANNELS; i++ {
		if colorantOrder[i] != 0xFF {
			count++
		}
	}

	if !cmsWriteUInt32Number(io, count) {
		return false
	}

	sz := count * uint32(unsafe.Sizeof(uint8(0)))
	return io.Write((*cms_io_handler)(io), sz, unsafe.Pointer(&colorantOrder[0]))
}

func TypeColorantOrderTypeDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return cmsDupMem(self.ContextID, ptr, cmsMAXCHANNELS*uint32(unsafe.Sizeof(uint8(0))))
}

func TypeColorantOrderTypeFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFree(self.ContextID, ptr)
}

// ********************************************************************************
// Type cmsSigS15Fixed16ArrayType
// ********************************************************************************
// This type represents an array of generic 4-byte/32-bit fixed point quantity.
// The number of values is determined from the size of the tag.

func TypeS15Fixed16Read(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	*nItems = 0

	// Calculate number of elements
	n := sizeOfTag / uint32(unsafe.Sizeof(uint32(0)))

	// Allocate memory for the array
	arrayDouble := (*[1 << 30]float64)(cmsCalloc(self.ContextID, n, uint32(unsafe.Sizeof(float64(0)))))
	if arrayDouble == nil {
		return nil
	}

	// Read
	for i := uint32(0); i < n; i++ {
		if !cmsRead15Fixed16Number(io, &arrayDouble[i]) {
			cmsFree(self.ContextID, unsafe.Pointer(arrayDouble))
			return nil
		}
	}

	*nItems = n
	return unsafe.Pointer(arrayDouble)
}

func TypeS15Fixed16Write(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	values := (*[1 << 30]float64)(ptr)

	for i := uint32(0); i < nItems; i++ {
		if !cmsWrite15Fixed16Number(io, values[i]) {
			return false
		}
	}

	return true
}

func TypeS15Fixed16Dup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return cmsDupMem(self.ContextID, ptr, n*uint32(unsafe.Sizeof(float64(0))))
}

func TypeS15Fixed16Free(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFree(self.ContextID, ptr)
}

// ********************************************************************************
// Type cmsSigU16Fixed16ArrayType
// ********************************************************************************
// This type represents an array of generic 4-byte/32-bit quantity.
// The number of values is determined from the size of the tag.

func TypeU16Fixed16Read(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	n := sizeOfTag / uint32(unsafe.Sizeof(uint32(0)))
	arrayDouble := (*[1 << 30]float64)(cmsCalloc(self.ContextID, n, uint32(unsafe.Sizeof(float64(0)))))
	if arrayDouble == nil {
		return nil
	}

	for i := uint32(0); i < n; i++ {
		var v uint32
		if !cmsReadUInt32Number(io, &v) {
			cmsFree(self.ContextID, unsafe.Pointer(arrayDouble))
			return nil
		}

		// Convert to cmsFloat64Number
		arrayDouble[i] = float64(v) / 65536.0
	}

	*nItems = n
	return unsafe.Pointer(&arrayDouble[0])
}

func TypeU16Fixed16Write(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	values := (*[1 << 30]float64)(ptr)

	for i := uint32(0); i < nItems; i++ {
		v := uint32(values[i]*65536.0 + 0.5) // Convert back to fixed-point

		if !cmsWriteUInt32Number(io, v) {
			return false
		}
	}

	return true
}

func TypeU16Fixed16Dup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return cmsDupMem(self.ContextID, ptr, n*uint32(unsafe.Sizeof(float64(0))))
}

func TypeU16Fixed16Free(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFree(self.ContextID, ptr)
}

// ********************************************************************************
// Type cmsSigSignatureType
// ********************************************************************************
//
// The signatureType contains a four-byte sequence, Sequences of less than four
// characters are padded at the end with spaces, 20h.
// Typically this type is used for registered tags that can be displayed on many
// development systems as a sequence of four characters.
// TypeSignatureRead reads a cmsSignature from the io handler.
func TypeSignatureRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	sigPtr := (*cmsSignature)(cmsMalloc(self.ContextID, uint32(unsafe.Sizeof(cmsSignature(0)))))
	if sigPtr == nil {
		return nil
	}

	if !cmsReadUInt32Number(io, (*uint32)(unsafe.Pointer(sigPtr))) {
		cmsFree(self.ContextID, unsafe.Pointer(sigPtr))
		return nil
	}

	*nItems = 1
	return unsafe.Pointer(sigPtr)
}

// TypeSignatureWrite writes a cmsSignature to the io handler.
func TypeSignatureWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	sigPtr := (*cmsSignature)(ptr)
	return cmsWriteUInt32Number(io, uint32(*sigPtr))
}

// TypeSignatureDup duplicates a cmsSignature.
func TypeSignatureDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return cmsDupMem(self.ContextID, ptr, n*uint32(unsafe.Sizeof(cmsSignature(0))))
}

func TypeSignatureFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFree(self.ContextID, ptr)
}

// ********************************************************************************
// Type cmsSigCurveType
// ********************************************************************************

func TypeCurveRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var count uint32
	*nItems = 0

	if !cmsReadUInt32Number(io, &count) {
		return nil
	}

	switch count {
	case 0: // Linear
		singleGamma := 1.0
		newGamma := cmsBuildParametricToneCurve(self.ContextID, 1, &singleGamma)
		if newGamma == nil {
			return nil
		}
		*nItems = 1
		return unsafe.Pointer(newGamma)

	case 1: // Single gamma exponent
		var singleGammaFixed uint16
		if !cmsReadUInt16Number(io, &singleGammaFixed) {
			return nil
		}
		singleGamma := cms8Fixed8ToDouble(singleGammaFixed)
		*nItems = 1
		return unsafe.Pointer(cmsBuildParametricToneCurve(self.ContextID, 1, &singleGamma))

	default: // Curve
		if count > 0x7FFF {
			return nil // Prevent malicious behavior
		}

		newGamma := cmsBuildTabulatedToneCurve16(self.ContextID, count, nil)
		if newGamma == nil {
			return nil
		}

		// Convert *uint16 to []uint16
		tableSlice := unsafe.Slice(newGamma.Table16, newGamma.nEntries)

		if !cmsReadUInt16Array(io, count, &tableSlice[0]) {
			CmsFreeToneCurve(newGamma)
			return nil
		}

		*nItems = 1
		return unsafe.Pointer(newGamma)
	}
}
func TypeCurveWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	curve := (*CmsToneCurve)(ptr) // Convert the pointer to a CmsToneCurve struct

	if curve.nSegments == 1 && curve.Segments != nil {
		// Access the first segment using slicing with unsafe.Slice
		segments := unsafe.Slice(curve.Segments, curve.nSegments)

		if segments[0].Type == 1 {
			// Single gamma
			singleGammaFixed := cmsDoubleTo8Fixed8(segments[0].Params[0])
			if !cmsWriteUInt32Number(io, 1) || !cmsWriteUInt16Number(io, singleGammaFixed) {
				return false
			}
			return true
		}
	}

	if !cmsWriteUInt32Number(io, curve.nEntries) {
		return false
	}

	// Convert Table16 to a slice
	tableSlice := unsafe.Slice(curve.Table16, curve.nEntries)
	return cmsWriteUInt16Array(io, curve.nEntries, tableSlice)
}

func TypeCurveDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsDupToneCurve((*CmsToneCurve)(ptr)))
}

func TypeCurveFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	gamma := (*CmsToneCurve)(ptr)
	CmsFreeToneCurve(gamma)
}

// ********************************************************************************
// Type cmsSigParametricCurveType
// ********************************************************************************
/*func DecideCurveType(iccVersion float64, data unsafe.Pointer) cmsTagTypeSignature {
	curve := (*CmsToneCurve)(data)

	if iccVersion < 4.0 {
		return cmsSigCurveType
	}
	if curve.nSegments != 1 {
		return cmsSigCurveType // Only 1-segment curves can be saved as parametric
	}
	if curve.Segments[0].Type < 0 {
		return cmsSigCurveType // Only non-inverted curves
	}
	if curve.Segments[0].Type > 5 {
		return cmsSigCurveType // Only ICC parametric curves
	}

	return cmsSigParametricCurveType
}

func TypeParametricCurveRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	paramsByType := [5]int{1, 3, 4, 5, 7}
	var params [10]float64
	var curveType uint16

	*nItems = 0

	if !cmsReadUInt16Number(io, &curveType) {
		return nil
	}
	if !cmsReadUInt16Number(io, nil) { // Reserved
		return nil
	}

	if curveType > 4 {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, fmt.Sprintf("Unknown parametric curve type '%d'", curveType))
		return nil
	}

	n := paramsByType[curveType]
	for i := 0; i < n; i++ {
		if !cmsRead15Fixed16Number(io, &params[i]) {
			return nil
		}
	}

	newGamma := cmsBuildParametricToneCurve(self.ContextID, int(curveType+1), params[:])
	if newGamma == nil {
		return nil
	}

	*nItems = 1
	return unsafe.Pointer(newGamma)
}

func TypeParametricCurveWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	curve := (*CmsToneCurve)(ptr)
	paramsByType := [6]int{0, 1, 3, 4, 5, 7}
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

	if !cmsWriteUInt16Number(io, uint16(typen-1)) {
		return false
	}
	if !cmsWriteUInt16Number(io, 0) { // Reserved
		return false
	}

	for i := 0; i < nParams; i++ {
		if !cmsWrite15Fixed16Number(io, curve.Segments[0].Params[i]) {
			return false
		}
	}

	return true
}

func TypeParametricCurveDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsDupToneCurve((*CmsToneCurve)(ptr)))
}

func TypeParametricCurveFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	CmsFreeToneCurve((*CmsToneCurve)(ptr))
}*/

// ********************************************************************************
// Type cmsSigDateTimeType
// ********************************************************************************

// A 12-byte value representation of the time and date, where the byte usage is assigned
// as specified in table 1. The actual values are encoded as 16-bit unsigned integers
// (uInt16Number - see 5.1.6).
//
// All the dateTimeNumber values in a profile shall be in Coordinated Universal Time
// (UTC, also known as GMT or ZULU Time). Profile writers are required to convert local
// time to UTC when setting these values. Programs that display these values may show
// the dateTimeNumber as UTC, show the equivalent local time (at current locale), or
// display both UTC and local versions of the dateTimeNumber.
func TypeDateTimeRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var timestamp cmsDateTimeNumber
	newDateTime := (*time.Time)(cmsMalloc(self.ContextID, uint32(unsafe.Sizeof(time.Time{}))))

	*nItems = 0
	if newDateTime == nil {
		return nil
	}

	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&timestamp), uint32(unsafe.Sizeof(timestamp)), 1) != 1 {
		cmsFree(self.ContextID, unsafe.Pointer(newDateTime))
		return nil
	}

	cmsDecodeDateTimeNumber(&timestamp)
	*nItems = 1
	return unsafe.Pointer(newDateTime)
}

func TypeDateTimeWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	dateTime := (*time.Time)(ptr)
	var timestamp cmsDateTimeNumber

	cmsEncodeDateTimeNumber(&timestamp, *dateTime)
	return io.Write((*cms_io_handler)(io), uint32(unsafe.Sizeof(timestamp)), unsafe.Pointer(&timestamp))
}

func TypeDateTimeDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return cmsDupMem(self.ContextID, ptr, uint32(unsafe.Sizeof(time.Time{})))
}

func TypeDateTimeFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFree(self.ContextID, ptr)
}

// ********************************************************************************
// Type icMeasurementType
// ********************************************************************************

/*
The measurementType information refers only to the internal profile data and is
meant to provide profile makers an alternative to the default measurement
specifications.
*/
func TypeMeasurementRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var mc cmsICCMeasurementConditions

	// Zero out the structure
	memset(unsafe.Pointer(&mc), 0, unsafe.Sizeof(mc))

	// Read the data from the IO handler
	if !cmsReadUInt32Number(io, &mc.Observer) ||
		!cmsReadXYZNumber(io, &mc.Backing) ||
		!cmsReadUInt32Number(io, &mc.Geometry) ||
		!cmsRead15Fixed16Number(io, &mc.Flare) ||
		!cmsReadUInt32Number(io, &mc.IlluminantType) {
		return nil
	}

	*nItems = 1
	return cmsDupMem(self.ContextID, unsafe.Pointer(&mc), uint32(unsafe.Sizeof(mc)))
}

func TypeMeasurementWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	mc := (*cmsICCMeasurementConditions)(ptr)

	// Write the data to the IO handler
	return cmsWriteUInt32Number(io, mc.Observer) &&
		cmsWriteXYZNumber(io, &mc.Backing) &&
		cmsWriteUInt32Number(io, mc.Geometry) &&
		cmsWrite15Fixed16Number(io, mc.Flare) &&
		cmsWriteUInt32Number(io, mc.IlluminantType)
}

func TypeMeasurementDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return cmsDupMem(self.ContextID, ptr, uint32(unsafe.Sizeof(cmsICCMeasurementConditions{})))
}

func TypeMeasurementFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFree(self.ContextID, ptr)
}

// ********************************************************************************
// Type cmsSigMultiLocalizedUnicodeType
// ********************************************************************************
//
//	Do NOT trust SizeOfTag as there is an issue on the definition of profileSequenceDescTag. See the TechNote from
//	Max Derhak and Rohit Patil about this: basically the size of the string table should be guessed and cannot be
//	taken from the size of tag if this tag is embedded as part of bigger structures (profileSequenceDescTag, for instance)
func TypeMLURead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var count, recLen, sizeOfHeader, len, offset, largestPosition uint32
	var block *uint16

	*nItems = 0

	if !cmsReadUInt32Number(io, &count) || !cmsReadUInt32Number(io, &recLen) {
		return nil
	}

	if recLen != 12 {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "multiLocalizedUnicodeType of len != 12 is not supported.")
		return nil
	}

	mlu := (*cmsMLU)(cmsMallocZero(self.ContextID, uint32(unsafe.Sizeof(cmsMLU{}))))
	if mlu == nil {
		return nil
	}

	mlu.AllocatedEntries = count
	mlu.UsedEntries = count
	mlu.Entries = (*cmsMLUentry)(cmsCalloc(self.ContextID, count, uint32(unsafe.Sizeof(cmsMLUentry{}))))
	if mlu.Entries == nil {
		cmsFree(self.ContextID, unsafe.Pointer(mlu))
		return nil
	}

	sizeOfHeader = 12*count + uint32(unsafe.Sizeof(cmsTagBase{}))
	largestPosition = 0

	for i := uint32(0); i < count; i++ {
		entry := (*cmsMLUentry)(unsafe.Add(unsafe.Pointer(mlu.Entries), uintptr(i)*unsafe.Sizeof(cmsMLUentry{})))

		if !cmsReadUInt16Number(io, &entry.Language) || !cmsReadUInt16Number(io, &entry.Country) {
			goto Error
		}

		if !cmsReadUInt32Number(io, &len) || !cmsReadUInt32Number(io, &offset) {
			goto Error
		}

		if offset&1 != 0 || offset < sizeOfHeader+8 || (offset+len) < len || (offset+len) > sizeOfTag+8 {
			goto Error
		}

		beginOfString := offset - sizeOfHeader - 8
		entry.Len = (len * uint32(unsafe.Sizeof(rune(0)))) / uint32(unsafe.Sizeof(uint16(0)))
		entry.StrW = (beginOfString * uint32(unsafe.Sizeof(rune(0)))) / uint32(unsafe.Sizeof(uint16(0)))

		endOfString := beginOfString + len
		if endOfString > largestPosition {
			largestPosition = endOfString
		}
	}

	sizeOfTag = (largestPosition * uint32(unsafe.Sizeof(rune(0)))) / uint32(unsafe.Sizeof(uint16(0)))

	if sizeOfTag == 0 {
		block = nil
		mlu.MemPool = nil
		mlu.PoolSize = 0
		mlu.PoolUsed = 0
	} else {
		block = (*uint16)(cmsCalloc(self.ContextID, 1, sizeOfTag))
		//this is a replacement for cmsReadWCharArray in C-code needs additional check
		if block == nil || !cmsReadUInt16Array(io, sizeOfTag/uint32(unsafe.Sizeof(uint16(0))), &(*[1 << 30]uint16)(unsafe.Pointer(block))[:sizeOfTag/uint32(unsafe.Sizeof(uint16(0)))][0] /* slice */) {
			cmsFree(self.ContextID, unsafe.Pointer(block))
			goto Error
		}
		mlu.MemPool = unsafe.Pointer(block)
		mlu.PoolSize = sizeOfTag
		mlu.PoolUsed = sizeOfTag
	}

	*nItems = 1
	return unsafe.Pointer(mlu)

Error:
	if mlu.Entries != nil {
		cmsFree(self.ContextID, unsafe.Pointer(mlu.Entries))
	}
	if mlu != nil {
		cmsFree(self.ContextID, unsafe.Pointer(mlu))
	}
	return nil
}

func TypeMLUWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	mlu := (*cmsMLU)(ptr)
	var headerSize, len, offset uint32

	if ptr == nil {
		return cmsWriteUInt32Number(io, 0) && cmsWriteUInt32Number(io, 12)
	}

	if !cmsWriteUInt32Number(io, mlu.UsedEntries) || !cmsWriteUInt32Number(io, 12) {
		return false
	}

	headerSize = 12*mlu.UsedEntries + uint32(unsafe.Sizeof(cmsTagBase{}))

	for i := uint32(0); i < mlu.UsedEntries; i++ {
		entry := (*cmsMLUentry)(unsafe.Add(unsafe.Pointer(mlu.Entries), uintptr(i)*unsafe.Sizeof(cmsMLUentry{})))

		len = entry.Len * uint32(unsafe.Sizeof(uint16(0)))
		offset = entry.StrW*uint32(unsafe.Sizeof(uint16(0))) + headerSize + 8

		if !cmsWriteUInt16Number(io, entry.Language) ||
			!cmsWriteUInt16Number(io, entry.Country) ||
			!cmsWriteUInt32Number(io, len) ||
			!cmsWriteUInt32Number(io, offset) {
			return false
		}
	}
	// Convert the MemPool pointer to a slice of uint16 for cmsWriteUInt16Array
	poolSize := mlu.PoolUsed / uint32(unsafe.Sizeof(uint16(0)))
	memPoolSlice := unsafe.Slice((*uint16)(mlu.MemPool), poolSize)

	return cmsWriteUInt16Array(io, mlu.PoolUsed/uint32(unsafe.Sizeof(uint16(0))), memPoolSlice)
}

func TypeMLUDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsMLUdup((*cmsMLU)(ptr)))
}

func TypeMLUFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsMLUfree((*cmsMLU)(ptr))
}

// That will create a MPE LUT with Matrix, pre tables, CLUT and post tables.
// 8 bit lut may be scaled easily to v4 PCS, but we need also to properly adjust
// PCS on BToAxx tags and AtoB if abstract. We need to fix input direction.

func TypeLUT8Read(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var inputChannels, outputChannels, clutPoints uint8
	var temp *uint8
	var nTabSize uint32
	var matrix [9]float64
	var newLUT *cmsPipeline

	*nItems = 0

	// Read header
	if !cmsReadUInt8Number(io, &inputChannels) ||
		!cmsReadUInt8Number(io, &outputChannels) ||
		!cmsReadUInt8Number(io, &clutPoints) ||
		!cmsReadUInt8Number(io, nil) { // Padding
		goto Error
	}

	if clutPoints == 1 {
		goto Error // Invalid CLUT points
	}

	// Validate channel counts
	if inputChannels == 0 || inputChannels > cmsMAXCHANNELS ||
		outputChannels == 0 || outputChannels > cmsMAXCHANNELS {
		goto Error
	}

	// Allocate a new pipeline
	newLUT = cmsPipelineAlloc(self.ContextID, uint32(inputChannels), uint32(outputChannels))
	if newLUT == nil {
		goto Error
	}

	// Read the matrix
	for i := 0; i < 9; i++ {
		if !cmsRead15Fixed16Number(io, &matrix[i]) {
			cmsPipelineFree(newLUT)
			goto Error
		}
	}

	// Insert the matrix if it isn't identity
	if inputChannels == 3 && !cmsMAT3isIdentity((*cmsMAT3)(unsafe.Pointer(&matrix))) {
		if !cmsPipelineInsertStage(newLUT, cmsAT_BEGIN, cmsStageAllocMatrix(self.ContextID, 3, 3, &matrix[0], nil)) {

			goto Error
		}
	}

	// Read input tables
	if !Read8bitTables(self.ContextID, io, newLUT, uint32(inputChannels)) {
		goto Error
	}

	// Read 3D CLUT
	nTabSize = uipow(uint32(outputChannels), uint32(clutPoints), uint32(inputChannels))
	if nTabSize == ^uint32(0) {
		goto Error
	}

	if nTabSize > 0 {
		ptrW := (*uint16)(cmsCalloc(self.ContextID, nTabSize, uint32(unsafe.Sizeof(uint16(0)))))
		if ptrW == nil {
			goto Error
		}

		temp = (*uint8)(cmsMalloc(self.ContextID, nTabSize))
		if temp == nil {
			goto Error
		}

		if io.Read((*cms_io_handler)(io), unsafe.Pointer(temp), nTabSize, 1) != 1 {
			cmsFree(self.ContextID, unsafe.Pointer(ptrW))
			cmsFree(self.ContextID, unsafe.Pointer(temp))
			goto Error
		}

		for i := uint32(0); i < nTabSize; i++ {
			*(*uint16)(unsafe.Add(unsafe.Pointer(ptrW), uintptr(i)*unsafe.Sizeof(uint16(0)))) = FROM_8_TO_16(*(*uint8)(unsafe.Add(unsafe.Pointer(temp), uintptr(i))))
		}

		cmsFree(self.ContextID, unsafe.Pointer(temp))
		// Convert `ptrW` to a slice of uint16
		tSlice := unsafe.Slice(ptrW, nTabSize)

		if !cmsPipelineInsertStage(newLUT, cmsAT_END, cmsStageAllocCLut16bit(self.ContextID, uint32(clutPoints), uint32(inputChannels), uint32(outputChannels), &tSlice[0])) {
			cmsFree(self.ContextID, unsafe.Pointer(ptrW))
			goto Error
		}

		cmsFree(self.ContextID, unsafe.Pointer(ptrW))
	}

	// Read output tables
	if !Read8bitTables(self.ContextID, io, newLUT, uint32(outputChannels)) {
		goto Error
	}

	*nItems = 1
	return unsafe.Pointer(newLUT)

Error:
	if newLUT != nil {
		cmsPipelineFree(newLUT)
	}
	return nil

}
func TypeLUT8Write(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	newLUT := (*cmsPipeline)(ptr)
	var (
		mpe            *cmsStage
		preMPE         *cmsStageToneCurvesData
		postMPE        *cmsStageToneCurvesData
		matMPE         *cmsStageMatrixData
		clut           *cmsStageCLutData
		clutPoints     uint32
		val            uint8
		i, j, nTabSize uint32
	)

	// Disassemble the LUT into components
	mpe = newLUT.Elements
	if mpe.Type == cmsSigMatrixElemType {
		if mpe.InputChannels != 3 || mpe.OutputChannels != 3 {
			return false
		}
		matMPE = (*cmsStageMatrixData)(mpe.Data)
		mpe = mpe.Next
	}

	if mpe != nil && mpe.Type == cmsSigCurveSetElemType {
		preMPE = (*cmsStageToneCurvesData)(mpe.Data)
		mpe = mpe.Next
	}

	if mpe != nil && mpe.Type == cmsSigCLutElemType {
		clut = (*cmsStageCLutData)(mpe.Data)
		mpe = mpe.Next
	}

	if mpe != nil && mpe.Type == cmsSigCurveSetElemType {
		postMPE = (*cmsStageToneCurvesData)(mpe.Data)
		mpe = mpe.Next
	}

	// Ensure no extra stages
	if mpe != nil {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "LUT is not suitable to be saved as LUT8")
		return false
	}

	// Check clutPoints
	if clut == nil {
		clutPoints = 0
	} else {
		clutPoints = clut.Params.nSamples[0]
		for i = 1; i < cmsPipelineInputChannels(newLUT); i++ {
			if clut.Params.nSamples[i] != clutPoints {
				cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "LUT with different samples per dimension not suitable to be saved as LUT16")
				return false
			}
		}
	}

	// Write LUT information
	if !cmsWriteUInt8Number(io, uint8(cmsPipelineInputChannels(newLUT))) ||
		!cmsWriteUInt8Number(io, uint8(cmsPipelineOutputChannels(newLUT))) ||
		!cmsWriteUInt8Number(io, uint8(clutPoints)) ||
		!cmsWriteUInt8Number(io, 0) {
		return false
	}

	// Write matrix if exists, else identity matrix
	if matMPE != nil {
		// Convert the Double pointer to a slice
		matrix := unsafe.Slice(matMPE.Double, 9) // Assuming 9 elements in the matrix
		for i := 0; i < 9; i++ {
			if !cmsWrite15Fixed16Number(io, matrix[i]) {
				return false
			}
		}
	} else {
		if !cmsWrite15Fixed16Number(io, 1) || !cmsWrite15Fixed16Number(io, 0) || !cmsWrite15Fixed16Number(io, 0) ||
			!cmsWrite15Fixed16Number(io, 0) || !cmsWrite15Fixed16Number(io, 1) || !cmsWrite15Fixed16Number(io, 0) ||
			!cmsWrite15Fixed16Number(io, 0) || !cmsWrite15Fixed16Number(io, 0) || !cmsWrite15Fixed16Number(io, 1) {
			return false
		}
	}

	// Write prelinearization table
	if !Write8bitTables(self.ContextID, io, newLUT.InputChannels, preMPE) {
		return false
	}

	// Write 3D CLUT
	nTabSize = uipow(newLUT.OutputChannels, clutPoints, newLUT.InputChannels)
	if nTabSize == ^uint32(0) {
		return false
	}
	if nTabSize > 0 && clut != nil {
		// Convert clut.Tab.T to a slice
		tValues := unsafe.Slice(clut.Tab.T, nTabSize) // Assuming `nTabSize` is the length of the array

		for j = 0; j < nTabSize; j++ {
			val = uint8(FROM_16_TO_8(tValues[j]))
			if !cmsWriteUInt8Number(io, val) {
				return false
			}
		}
	}

	// Write postlinearization table
	if !Write8bitTables(self.ContextID, io, newLUT.OutputChannels, postMPE) {
		return false
	}

	return true
}

func TypeLUT8Dup(self *cmsTagTypeHandler, ptr unsafe.Pointer, nItems uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsPipelineDup((*cmsPipeline)(ptr)))
}

func TypeLUT8Free(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsPipelineFree((*cmsPipeline)(ptr))
}

/*
This structure represents a colour transform using tables of 8-bit precision.
This type contains four processing elements: a 3 by 3 matrix (which shall be
the identity matrix unless the input colour space is XYZ), a set of one dimensional
input tables, a multidimensional lookup table, and a set of one dimensional output
tables. Data is processed using these elements via the following sequence:
(matrix) . (1d input tables)  . (multidimensional lookup table - CLUT) . (1d output tables)

Byte Position   Field Length (bytes)  Content Encoded as...
8                  1          Number of Input Channels (i)    uInt8Number
9                  1          Number of Output Channels (o)   uInt8Number
10                 1          Number of CLUT grid points (identical for each side) (g) uInt8Number
11                 1          Reserved for padding (fill with 00h)

12..15             4          Encoded e00 parameter   s15Fixed16Number
*/

func Read8bitTables(ContextID CmsContext, io *cmsIOHANDLER, lut *cmsPipeline, nChannels uint32) bool {
	if nChannels > cmsMAXCHANNELS || nChannels <= 0 {
		return false
	}

	var tables [cmsMAXCHANNELS]*CmsToneCurve
	temp := (*[256]uint8)(cmsMalloc(ContextID, 256))
	if temp == nil {
		return false
	}
	defer cmsFree(ContextID, unsafe.Pointer(temp))

	// Allocate tone curves
	for i := uint32(0); i < nChannels; i++ {
		tables[i] = cmsBuildTabulatedToneCurve16(ContextID, 256, nil)
		if tables[i] == nil {
			goto Error
		}
	}

	// Read and populate tone curve data
	for i := uint32(0); i < nChannels; i++ {
		if io.Read((*cms_io_handler)(io), unsafe.Pointer(&temp[0]), 256, 1) != 0 {
			goto Error
		}
		// Assuming `nEntries` is the length of the Table16 array
		table16Values := unsafe.Slice(tables[i].Table16, 256) // Convert the pointer to a slice

		for j := 0; j < 256; j++ {
			table16Values[j] = uint16(temp[j]) * 257 // Convert 8-bit to 16-bit
		}

	}

	// Insert tone curves into the pipeline
	if !cmsPipelineInsertStage(lut, cmsAT_END, cmsStageAllocToneCurves(ContextID, nChannels, &tables[0])) {
		goto Error
	}

	// Free the tone curves
	for i := uint32(0); i < nChannels; i++ {
		CmsFreeToneCurve(tables[i])
	}

	return true

Error:
	for i := uint32(0); i < nChannels; i++ {
		if tables[i] != nil {
			CmsFreeToneCurve(tables[i])
		}
	}
	return false
}

func Write8bitTables(ContextID CmsContext, io *cmsIOHANDLER, n uint32, tables *cmsStageToneCurvesData) bool {
	if tables != nil {
		for i := uint32(0); i < n; i++ {
			curve := (**CmsToneCurve)(unsafe.Add(unsafe.Pointer(tables.TheCurves), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))
			table16 := unsafe.Slice((*curve).Table16, (*curve).nEntries)
			// Handle identity curves
			if (*curve).nEntries == 2 && table16[0] == 0 && table16[1] == 65535 {
				for j := 0; j < 256; j++ {
					if !cmsWriteUInt8Number(io, uint8(j)) {
						return false
					}
				}
			} else if (*curve).nEntries != 256 {
				cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_RANGE, "LUT8 needs 256 entries on prelinearization")
				return false
			} else {
				for j := 0; j < 256; j++ {
					val := uint8(table16[j] / 257) // Convert 16-bit to 8-bit
					if !cmsWriteUInt8Number(io, val) {
						return false
					}
				}
			}
		}
	}
	return true
}

func uipow(n, a, b uint32) uint32 {
	rv := uint32(1)

	if a == 0 || n == 0 {
		return 0
	}

	for ; b > 0; b-- {
		if rv > (math.MaxUint32 / a) {
			return math.MaxUint32 // Overflow detected
		}
		rv *= a
	}

	rc := rv * n
	if rv != rc/n {
		return math.MaxUint32 // Overflow detected
	}
	return rc
}

// ********************************************************************************
// Type cmsSigLut16Type
// ********************************************************************************
func Read16bitTables(ContextID CmsContext, io *cmsIOHANDLER, lut *cmsPipeline, nChannels, nEntries uint32) bool {
	if nEntries == 0 || nEntries < 2 || nChannels > cmsMAXCHANNELS {
		return false
	}
	var tables [cmsMAXCHANNELS]*CmsToneCurve

	for i := uint32(0); i < nChannels; i++ {
		tables[i] = cmsBuildTabulatedToneCurve16(ContextID, nEntries, nil)
		if tables[i] == nil {
			goto Error
		}

		// Convert the `Table16` pointer to a slice before passing to `cmsReadUInt16Array`
		table16Slice := unsafe.Slice(tables[i].Table16, nEntries)

		if !cmsReadUInt16Array(io, nEntries, &table16Slice[0]) {
			goto Error
		}
	}

	if !cmsPipelineInsertStage(lut, cmsAT_END, cmsStageAllocToneCurves(ContextID, nChannels, &tables[0])) {
		goto Error
	}

	for i := uint32(0); i < nChannels; i++ {
		CmsFreeToneCurve(tables[i])
	}
	return true

Error:
	for i := uint32(0); i < nChannels; i++ {
		if tables[i] != nil {
			CmsFreeToneCurve(tables[i])
		}
	}
	return false
}
func Write16bitTables(ContextID CmsContext, io *cmsIOHANDLER, tables *cmsStageToneCurvesData) bool {
	for i := uint32(0); i < tables.NCurves; i++ {
		curve := (**CmsToneCurve)(unsafe.Add(unsafe.Pointer(tables.TheCurves), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))
		nEntries := (*curve).nEntries

		// Convert the Table16 pointer to a slice
		table16Slice := unsafe.Slice((*curve).Table16, nEntries)

		for j := uint32(0); j < nEntries; j++ {
			val := table16Slice[j]
			if !cmsWriteUInt16Number(io, val) {
				return false
			}
		}
	}
	return true
}

func TypeLUT16Read(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var inputChannels, outputChannels, clutPoints uint8
	var inputEntries, outputEntries uint16
	var matrix [9]float64

	*nItems = 0

	if !cmsReadUInt8Number(io, &inputChannels) || !cmsReadUInt8Number(io, &outputChannels) || !cmsReadUInt8Number(io, &clutPoints) || !cmsReadUInt8Number(io, nil) {
		return nil
	}

	if inputChannels == 0 || inputChannels > cmsMAXCHANNELS || outputChannels == 0 || outputChannels > cmsMAXCHANNELS {
		return nil
	}

	newLUT := cmsPipelineAlloc(self.ContextID, uint32(inputChannels), uint32(outputChannels))
	if newLUT == nil {
		return nil
	}

	for i := 0; i < 9; i++ {
		if !cmsRead15Fixed16Number(io, &matrix[i]) {
			cmsPipelineFree(newLUT)
			return nil
		}
	}

	// Convert the flat matrix to cmsMAT3
	mat3 := cmsMAT3{
		V: [3]cmsVEC3{
			{N: [3]float64{matrix[0], matrix[1], matrix[2]}},
			{N: [3]float64{matrix[3], matrix[4], matrix[5]}},
			{N: [3]float64{matrix[6], matrix[7], matrix[8]}},
		},
	}

	// Only operates on 3 channels
	if inputChannels == 3 && !cmsMAT3isIdentity(&mat3) {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, cmsStageAllocMatrix(self.ContextID, 3, 3, &matrix[0], nil)) {
			cmsPipelineFree(newLUT)
			return nil
		}
	}

	if !cmsReadUInt16Number(io, &inputEntries) || !cmsReadUInt16Number(io, &outputEntries) {
		cmsPipelineFree(newLUT)
		return nil
	}

	if inputEntries > 0x7FFF || outputEntries > 0x7FFF || clutPoints == 1 {
		cmsPipelineFree(newLUT)
		return nil
	}

	if !Read16bitTables(self.ContextID, io, newLUT, uint32(inputChannels), uint32(inputEntries)) {
		cmsPipelineFree(newLUT)
		return nil
	}

	nTabSize := uipow(uint32(outputChannels), uint32(clutPoints), uint32(inputChannels))
	if nTabSize == math.MaxUint32 || nTabSize > 0 {
		t := (*uint16)(cmsCalloc(self.ContextID, nTabSize, uint32(unsafe.Sizeof(uint16(0)))))
		if t == nil {
			cmsPipelineFree(newLUT)
			return nil
		}
		if !cmsReadUInt16Array(io, nTabSize, t) {
			cmsPipelineFree(newLUT)
			return nil
		}
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, cmsStageAllocCLut16bit(self.ContextID, uint32(clutPoints), uint32(inputChannels), uint32(outputChannels), t)) {
			cmsPipelineFree(newLUT)
			return nil
		}
	}

	if !Read16bitTables(self.ContextID, io, newLUT, uint32(outputChannels), uint32(outputEntries)) {
		cmsPipelineFree(newLUT)
		return nil
	}

	*nItems = 1
	return unsafe.Pointer(newLUT)
}

func TypeLUT16Write(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	newLUT := (*cmsPipeline)(ptr)
	var matMPE *cmsStageMatrixData
	var preMPE, postMPE *cmsStageToneCurvesData
	var clut *cmsStageCLutData
	var clutPoints uint32

	mpe := newLUT.Elements

	if mpe != nil && mpe.Type == cmsSigMatrixElemType {
		matMPE = (*cmsStageMatrixData)(mpe.Data)
		if mpe.InputChannels != 3 || mpe.OutputChannels != 3 {
			return false
		}
		mpe = mpe.Next
	}

	if mpe != nil && mpe.Type == cmsSigCurveSetElemType {
		preMPE = (*cmsStageToneCurvesData)(mpe.Data)
		mpe = mpe.Next
	}

	if mpe != nil && mpe.Type == cmsSigCLutElemType {
		clut = (*cmsStageCLutData)(mpe.Data)
		mpe = mpe.Next
	}

	if mpe != nil && mpe.Type == cmsSigCurveSetElemType {
		postMPE = (*cmsStageToneCurvesData)(mpe.Data)
		mpe = mpe.Next
	}

	if mpe != nil {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "LUT is not suitable to be saved as LUT16")
		return false
	}

	inputChannels := cmsPipelineInputChannels(newLUT)
	outputChannels := cmsPipelineOutputChannels(newLUT)

	if clut != nil {
		clutPoints = clut.Params.nSamples[0]
		for i := uint32(1); i < inputChannels; i++ {
			if clut.Params.nSamples[i] != clutPoints {
				cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "LUT with different samples per dimension not suitable to be saved as LUT16")
				return false
			}
		}
	}

	if !cmsWriteUInt8Number(io, uint8(inputChannels)) ||
		!cmsWriteUInt8Number(io, uint8(outputChannels)) ||
		!cmsWriteUInt8Number(io, uint8(clutPoints)) ||
		!cmsWriteUInt8Number(io, 0) {
		return false
	}

	if matMPE != nil {
		// Convert the Double pointer to a slice
		matrix := unsafe.Slice(matMPE.Double, 9) // Assuming 9 elements in the matrix
		for i := 0; i < 9; i++ {
			if !cmsWrite15Fixed16Number(io, matrix[i]) {
				return false
			}
		}
	} else {
		identityMatrix := []float64{1, 0, 0, 0, 1, 0, 0, 0, 1}
		for _, value := range identityMatrix {
			if !cmsWrite15Fixed16Number(io, value) {
				return false
			}
		}
	}

	if preMPE != nil {
		if !cmsWriteUInt16Number(io, uint16((*preMPE.TheCurves).nEntries)) {
			return false
		}
	} else {
		if !cmsWriteUInt16Number(io, 2) {
			return false
		}
	}

	if postMPE != nil {
		if !cmsWriteUInt16Number(io, uint16((*postMPE.TheCurves).nEntries)) {
			return false
		}
	} else {
		if !cmsWriteUInt16Number(io, 2) {
			return false
		}
	}

	if preMPE != nil {
		if !Write16bitTables(self.ContextID, io, preMPE) {
			return false
		}
	} else {
		for i := uint32(0); i < inputChannels; i++ {
			if !cmsWriteUInt16Number(io, 0) || !cmsWriteUInt16Number(io, 0xFFFF) {
				return false
			}
		}
	}

	nTabSize := uipow(outputChannels, clutPoints, inputChannels)
	if nTabSize == math.MaxUint32 {
		return false
	}
	if nTabSize > 0 {
		if clut != nil {
			// Convert clut.Tab.T (*uint16) to a slice
			tabSlice := unsafe.Slice(clut.Tab.T, nTabSize)

			if !cmsWriteUInt16Array(io, nTabSize, tabSlice) {
				return false
			}
		}
	}

	if postMPE != nil {
		if !Write16bitTables(self.ContextID, io, postMPE) {
			return false
		}
	} else {
		for i := uint32(0); i < outputChannels; i++ {
			if !cmsWriteUInt16Number(io, 0) || !cmsWriteUInt16Number(io, 0xFFFF) {
				return false
			}
		}
	}

	return true
}

func TypeLUT16Dup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsPipelineDup((*cmsPipeline)(ptr)))
}

func TypeLUT16Free(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsPipelineFree((*cmsPipeline)(ptr))
}

// ********************************************************************************
// Type cmsSigColorantTableType
// ********************************************************************************
/*
The purpose of this tag is to identify the colorants used in the profile by a
unique name and set of XYZ or L*a*b* values to give the colorant an unambiguous
value. The first colorant listed is the colorant of the first device channel of
a lut tag. The second colorant listed is the colorant of the second device channel
of a lut tag, and so on.
*/

func TypeColorantTableRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var count uint32
	var name [34]byte
	var pcs [3]uint16

	if !cmsReadUInt32Number(io, &count) {
		return nil
	}

	if count > cmsMAXCHANNELS {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_RANGE, "Too many colorants")
		return nil
	}

	list := cmsAllocNamedColorList(self.ContextID, count, 0, "", "")
	if list == nil {
		return nil
	}

	for i := uint32(0); i < count; i++ {
		if io.Read((*cms_io_handler)(io), unsafe.Pointer(&name[0]), 32, 1) != 1 {
			goto Error
		}

		name[32] = 0 // Null-terminate
		if !cmsReadUInt16Array(io, 3, &pcs[0]) {
			goto Error
		}
		nameStr := string(name[:bytes.IndexByte(name[:], 0)])
		if !cmsAppendNamedColor(list, nameStr, &pcs, nil) {
			goto Error
		}
	}

	*nItems = 1
	return unsafe.Pointer(list)

Error:
	*nItems = 0
	cmsFreeNamedColorList(list)
	return nil
}

func TypeColorantTableWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	namedColorList := (*cmsNAMEDCOLORLIST)(ptr)
	nColors := cmsNamedColorCount(namedColorList)

	if !cmsWriteUInt32Number(io, nColors) {
		return false
	}

	for i := uint32(0); i < nColors; i++ {
		var root [cmsMAX_PATH]byte
		var pcs [3]uint16

		if !cmsNamedColorInfo(namedColorList, i, &root[0], nil, nil, &pcs[0], nil) {
			return false
		}

		// Null-terminate root name
		root[32] = 0

		if !io.Write((*cms_io_handler)(io), 32, unsafe.Pointer(&root[0])) {
			return false
		}
		if !cmsWriteUInt16Array(io, 3, pcs[:]) {
			return false
		}
	}

	return true
}

func TypeColorantTableDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	nc := (*cmsNAMEDCOLORLIST)(ptr)
	return unsafe.Pointer(cmsDupNamedColorList(nc))
}

func TypeColorantTableFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFreeNamedColorList((*cmsNAMEDCOLORLIST)(ptr))
}

/*
   NamedColorList := (*cmsNAMEDCOLORLIST) (ptr)
    var prefix[33]byte;     // Prefix for each color name
    var suffix[33]byte;     // Suffix for each color name


    nColors := cmsNamedColorCount(NamedColorList);

    if (!cmsWriteUInt32Number(io, 0)) {return false}
    if (!cmsWriteUInt32Number(io, nColors)) {return false}
    if (!cmsWriteUInt32Number(io, NamedColorList.ColorantCount)) {return false}

    memcpy(prefix, NamedColorList.Prefix, unsafe.Sizeof(prefix))
    memcpy(suffix, NamedColorList.Suffix, unsafe.Sizeof(suffix))

    suffix[32] = 0;
	prefix[32] = 0;

    if (!io.Write(io, 32, prefix)) {return false}
    if (!io.Write(io, 32, suffix)) {return false}

    for i:=0; i < nColors; i++ {

       var PCS[3] uint16
       var  Colorant[cmsMAXCHANNELS]uint16
       var Root[cmsMAX_PATH]byte

        if (!cmsNamedColorInfo(NamedColorList, i, Root, nil, nil, PCS, Colorant)) {return 0}
        Root[32] = 0;
        if (!io.Write(io, 32 , Root)) {return false}
        if (!cmsWriteUInt16Array(io, 3, PCS)) {return false}
        if (!cmsWriteUInt16Array(io, NamedColorList .ColorantCount, Colorant)) {return FALSE;
    }
    return true*/

// ********************************************************************************
// Type cmsSigNamedColor2Type
// ********************************************************************************
//
// The namedColor2Type is a count value and array of structures that provide color
// coordinates for 7-bit ASCII color names. For each named color, a PCS and optional
// device representation of the color are given. Both representations are 16-bit values.
// The device representation corresponds to the header's 'color space of data' field.
// This representation should be consistent with the 'number of device components'
// field in the namedColor2Type. If this field is 0, device coordinates are not provided.
// The PCS representation corresponds to the header's PCS field. The PCS representation
// is always provided. Color names are fixed-length, 32-byte fields including null
// termination. In order to maintain maximum portability, it is strongly recommended
// that special characters of the 7-bit ASCII set not be used.
func TypeNamedColorRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var vendorFlag, count, nDeviceCoords uint32
	var prefix, suffix [32]byte

	*nItems = 0

	if !cmsReadUInt32Number(io, &vendorFlag) ||
		!cmsReadUInt32Number(io, &count) ||
		!cmsReadUInt32Number(io, &nDeviceCoords) {
		return nil
	}

	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&prefix[0]), 32, 1) != 1 ||
		io.Read((*cms_io_handler)(io), unsafe.Pointer(&suffix[0]), 32, 1) != 1 {
		return nil
	}

	prefix[31] = 0 // Null-terminate
	suffix[31] = 0 // Null-terminate
	prefixStr := string(prefix[:bytes.IndexByte(prefix[:], 0)])
	suffixStr := string(suffix[:bytes.IndexByte(suffix[:], 0)])
	namedColorList := cmsAllocNamedColorList(self.ContextID, count, nDeviceCoords, prefixStr, suffixStr)
	if namedColorList == nil {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_RANGE, "Too many named colors")
		return nil
	}

	if nDeviceCoords > cmsMAXCHANNELS {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_RANGE, "Too many device coordinates")
		goto Error
	}

	for i := uint32(0); i < count; i++ {
		var pcs [3]uint16
		var colorant [cmsMAXCHANNELS]uint16
		var root [33]byte

		if io.Read((*cms_io_handler)(io), unsafe.Pointer(&root[0]), 32, 1) != 1 {
			goto Error
		}

		root[32] = 0 // Null-terminate
		if !cmsReadUInt16Array(io, 3, &pcs[0]) || !cmsReadUInt16Array(io, nDeviceCoords, &colorant[0]) {
			goto Error
		}

		rootStr := string(root[:bytes.IndexByte(root[:], 0)])
		if !cmsAppendNamedColor(namedColorList, rootStr, &pcs, &colorant) {
			goto Error
		}
	}

	*nItems = 1
	return unsafe.Pointer(namedColorList)

Error:
	cmsFreeNamedColorList(namedColorList)
	return nil
}

func TypeNamedColorWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	namedColorList := (*cmsNAMEDCOLORLIST)(ptr)
	nColors := cmsNamedColorCount(namedColorList)

	if !cmsWriteUInt32Number(io, 0) ||
		!cmsWriteUInt32Number(io, nColors) ||
		!cmsWriteUInt32Number(io, namedColorList.ColorantCount) {
		return false
	}

	var prefix, suffix [33]byte
	copy(prefix[:32], namedColorList.Prefix[:])
	copy(suffix[:32], namedColorList.Suffix[:])
	prefix[32] = 0 // Null-terminate
	suffix[32] = 0 // Null-terminate

	if !io.Write((*cms_io_handler)(io), 32, unsafe.Pointer(&prefix[0])) ||
		!io.Write((*cms_io_handler)(io), 32, unsafe.Pointer(&suffix[0])) {
		return false
	}

	for i := uint32(0); i < nColors; i++ {
		var root [33]byte
		var pcs [3]uint16
		var colorant [cmsMAXCHANNELS]uint16

		if !cmsNamedColorInfo(namedColorList, i, &root[0], nil, nil, &pcs[0], &colorant[0]) {
			return false
		}

		root[32] = 0 // Null-terminate
		if !io.Write((*cms_io_handler)(io), 32, unsafe.Pointer(&root[0])) ||
			!cmsWriteUInt16Array(io, 3, pcs[:]) ||
			!cmsWriteUInt16Array(io, namedColorList.ColorantCount, colorant[:]) {
			return false
		}
	}

	return true
}

func TypeNamedColorDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	nc := (*cmsNAMEDCOLORLIST)(ptr)
	return unsafe.Pointer(cmsDupNamedColorList(nc))
}

func TypeNamedColorFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFreeNamedColorList((*cmsNAMEDCOLORLIST)(ptr))
}

// ********************************************************************************
// Type cmsSigProfileSequenceDescType
// ********************************************************************************

// This type is an array of structures, each of which contains information from the
// header fields and tags from the original profiles which were combined to create
// the final profile. The order of the structures is the order in which the profiles
// were combined and includes a structure for the final profile. This provides a
// description of the profile sequence from source to destination,
// typically used with the DeviceLink profile.
func ReadEmbeddedText(self *cmsTagTypeHandler, io *cmsIOHANDLER, mlu **cmsMLU, sizeOfTag uint32) bool {
	baseType := cmsReadTypeBase(io)
	switch baseType {
	case cmsSigTextType:
		if *mlu != nil {
			cmsMLUfree(*mlu)
		}
		*mlu = (*cmsMLU)(TypeTextRead(self, io, new(uint32), sizeOfTag))
		return *mlu != nil

	case cmsSigTextDescriptionType:
		if *mlu != nil {
			cmsMLUfree(*mlu)
		}
		*mlu = (*cmsMLU)(TypeTextDescriptionRead(self, io, new(uint32), sizeOfTag))
		return *mlu != nil

	case cmsSigMultiLocalizedUnicodeType:
		if *mlu != nil {
			cmsMLUfree(*mlu)
		}
		*mlu = (*cmsMLU)(TypeMLURead(self, io, new(uint32), sizeOfTag))
		return *mlu != nil

	default:
		return false
	}
}

func TypeProfileSequenceDescRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var count uint32

	*nItems = 0

	if !cmsReadUInt32Number(io, &count) {
		return nil
	}

	if sizeOfTag < uint32(unsafe.Sizeof(count)) {
		return nil
	}
	sizeOfTag -= uint32(unsafe.Sizeof(count))

	outSeq := cmsAllocProfileSequenceDescription(self.ContextID, count)
	if outSeq == nil {
		return nil
	}
	outSeq.n = count
	// Convert `outSeq.seq` to a slice for indexing
	seqSlice := unsafe.Slice(outSeq.seq, count)

	for i := uint32(0); i < count; i++ {
		sec := &seqSlice[i]

		if !cmsReadUInt32Number(io, (*uint32)(&sec.deviceMfg)) ||
			sizeOfTag < uint32(unsafe.Sizeof(sec.deviceMfg)) {
			goto Error
		}
		sizeOfTag -= uint32(unsafe.Sizeof(sec.deviceMfg))

		if !cmsReadUInt32Number(io, (*uint32)(&sec.deviceModel)) ||
			sizeOfTag < uint32(unsafe.Sizeof(sec.deviceModel)) {
			goto Error
		}
		sizeOfTag -= uint32(unsafe.Sizeof(sec.deviceModel))

		if !cmsReadUInt64Number(io, &sec.attributes) ||
			sizeOfTag < uint32(unsafe.Sizeof(sec.attributes)) {
			goto Error
		}
		sizeOfTag -= uint32(unsafe.Sizeof(sec.attributes))

		if !cmsReadUInt32Number(io, (*uint32)(&sec.technology)) ||
			sizeOfTag < uint32(unsafe.Sizeof(sec.technology)) {
			goto Error
		}
		sizeOfTag -= uint32(unsafe.Sizeof(sec.technology))

		if !ReadEmbeddedText(self, io, &sec.Manufacturer, sizeOfTag) ||
			!ReadEmbeddedText(self, io, &sec.Model, sizeOfTag) {
			goto Error
		}
	}

	*nItems = 1
	return unsafe.Pointer(outSeq)

Error:
	cmsFreeProfileSequenceDescription(outSeq)
	return nil
}

func SaveDescription(self *cmsTagTypeHandler, io *cmsIOHANDLER, text *cmsMLU) bool {
	if self.ICCVersion < 0x4000000 {
		if !cmsWriteTypeBase(io, cmsSigTextDescriptionType) {
			return false
		}
		return TypeTextDescriptionWrite(self, io, unsafe.Pointer(text), 1)
	} else {
		if !cmsWriteTypeBase(io, cmsSigMultiLocalizedUnicodeType) {
			return false
		}
		return TypeMLUWrite(self, io, unsafe.Pointer(text), 1)
	}
}

func TypeProfileSequenceDescWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	seq := (*cmsSEQ)(ptr)

	if !cmsWriteUInt32Number(io, seq.n) {
		return false
	}
	// Convert `outSeq.seq` to a slice for indexing
	seqSlice := unsafe.Slice(seq.seq, seq.n)

	for i := uint32(0); i < seq.n; i++ {
		sec := &seqSlice[i]

		if !cmsWriteUInt32Number(io, uint32(sec.deviceMfg)) ||
			!cmsWriteUInt32Number(io, uint32(sec.deviceModel)) ||
			!cmsWriteUInt64Number(io, uint64(sec.attributes)) ||
			!cmsWriteUInt32Number(io, uint32(sec.technology)) ||
			!SaveDescription(self, io, sec.Manufacturer) ||
			!SaveDescription(self, io, sec.Model) {
			return false
		}
	}

	return true
}

func TypeProfileSequenceDescDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsDupProfileSequenceDescription((*cmsSEQ)(ptr)))
}

func TypeProfileSequenceDescFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFreeProfileSequenceDescription((*cmsSEQ)(ptr))
}

// ********************************************************************************
// Type cmsSigProfileSequenceIdType
// ********************************************************************************
/*
In certain workflows using ICC Device Link Profiles, it is necessary to identify the
original profiles that were combined to create the Device Link Profile.
This type is an array of structures, each of which contains information for
identification of a profile used in a sequence
*/
func ReadSeqID(self *cmsTagTypeHandler, io *cmsIOHANDLER, cargo unsafe.Pointer, n, sizeOfTag uint32) bool {
	outSeq := (*cmsSEQ)(cargo)
	seqSlice := unsafe.Slice(outSeq.seq, outSeq.n) // Convert pointer to slice
	seq := &seqSlice[n]

	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&seq.ProfileID.ID8), 16, 1) != 1 {
		return false
	}
	if !ReadEmbeddedText(self, io, &seq.Description, sizeOfTag) {
		return false
	}

	return true
}

func TypeProfileSequenceIdRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var count, baseOffset uint32

	*nItems = 0

	// Get actual position as a basis for element offsets
	baseOffset = uint32(io.Tell((*cms_io_handler)(io))) - uint32(unsafe.Sizeof(cmsTagBase{}))

	// Get table count
	if !cmsReadUInt32Number(io, &count) {
		return nil
	}

	// Allocate an empty structure
	outSeq := cmsAllocProfileSequenceDescription(self.ContextID, count)
	if outSeq == nil {
		return nil
	}

	// Read the position table
	if !ReadPositionTable(self, io, count, baseOffset, unsafe.Pointer(outSeq), ReadSeqID) {
		cmsFreeProfileSequenceDescription(outSeq)
		return nil
	}

	// Success
	*nItems = 1
	return unsafe.Pointer(outSeq)
}

func WriteSeqID(self *cmsTagTypeHandler, io *cmsIOHANDLER, cargo unsafe.Pointer, n, sizeOfTag uint32) bool {
	seq := (*cmsSEQ)(cargo)
	seqSlice := unsafe.Slice(seq.seq, seq.n) // Convert pointer to slice
	currentSeq := &seqSlice[n]

	// Write Profile ID
	if io.Write((*cms_io_handler)(io), 16, unsafe.Pointer(&currentSeq.ProfileID.ID8[0])) {
		return false
	}

	// Store the MLU
	if !SaveDescription(self, io, currentSeq.Description) {
		return false
	}

	return true
}

func TypeProfileSequenceIdWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	seq := (*cmsSEQ)(ptr)
	baseOffset := uint32(io.Tell((*cms_io_handler)(io))) - uint32(unsafe.Sizeof(cmsTagBase{}))

	// Write the table count
	if !cmsWriteUInt32Number(io, seq.n) {
		return false
	}

	// Write the position table and content
	if !WritePositionTable(self, io, 0, seq.n, baseOffset, unsafe.Pointer(seq), WriteSeqID) {
		return false
	}

	return true
}

func TypeProfileSequenceIdDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, nItems uint32) unsafe.Pointer {
	seq := (*cmsSEQ)(ptr)
	return unsafe.Pointer(cmsDupProfileSequenceDescription(seq))
}

func TypeProfileSequenceIdFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFreeProfileSequenceDescription((*cmsSEQ)(ptr))
}

// ********************************************************************************
// Type cmsSigUcrBgType
// ********************************************************************************
/*
This type contains curves representing the under color removal and black
generation and a text string which is a general description of the method used
for the ucr/bg.
*/
func TypeUcrBgRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	n := (*cmsUcrBg)(cmsMallocZero(self.ContextID, uint32(unsafe.Sizeof(cmsUcrBg{}))))
	var asciiString []byte
	var (
		countUcr, countBg uint32
		signedSizeOfTag   int32 = int32(sizeOfTag)
	)

	*nItems = 0
	if n == nil {
		return nil
	}

	// First curve is Under color removal
	if signedSizeOfTag < int32(unsafe.Sizeof(uint32(0))) || !cmsReadUInt32Number(io, &countUcr) {
		goto Error
	}
	signedSizeOfTag -= int32(unsafe.Sizeof(uint32(0)))

	n.Ucr = cmsBuildTabulatedToneCurve16(self.ContextID, countUcr, nil)
	if n.Ucr == nil || signedSizeOfTag < int32(countUcr*uint32(unsafe.Sizeof(uint16(0)))) || !cmsReadUInt16Array(io, countUcr, n.Ucr.Table16) {
		goto Error
	}
	signedSizeOfTag -= int32(countUcr * uint32(unsafe.Sizeof(uint16(0))))

	// Second curve is Black generation
	if signedSizeOfTag < int32(unsafe.Sizeof(uint32(0))) || !cmsReadUInt32Number(io, &countBg) {
		goto Error
	}
	signedSizeOfTag -= int32(unsafe.Sizeof(uint32(0)))

	n.Bg = cmsBuildTabulatedToneCurve16(self.ContextID, countBg, nil)
	if n.Bg == nil || signedSizeOfTag < int32(countBg*uint32(unsafe.Sizeof(uint16(0)))) || !cmsReadUInt16Array(io, countBg, n.Bg.Table16) {
		goto Error
	}
	signedSizeOfTag -= int32(countBg * uint32(unsafe.Sizeof(uint16(0))))

	if signedSizeOfTag < 0 || signedSizeOfTag > 32000 {
		goto Error
	}

	// Now comes the text
	n.Desc = cmsMLUalloc(self.ContextID, 1)
	if n.Desc == nil {
		goto Error
	}

	asciiString = (*[1 << 30]byte)(cmsMalloc(self.ContextID, uint32(signedSizeOfTag)+1))[:signedSizeOfTag+1]
	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&asciiString[0]), 1, uint32(signedSizeOfTag)) != uint32(signedSizeOfTag) {
		cmsFree(self.ContextID, unsafe.Pointer(&asciiString[0]))
		goto Error
	}
	asciiString[signedSizeOfTag] = 0

	cmsMLUsetASCII(n.Desc, cmsNoLanguage, cmsNoCountry, &asciiString[0])
	cmsFree(self.ContextID, unsafe.Pointer(&asciiString[0]))

	*nItems = 1
	return unsafe.Pointer(n)

Error:
	if n.Ucr != nil {
		CmsFreeToneCurve(n.Ucr)
	}
	if n.Bg != nil {
		CmsFreeToneCurve(n.Bg)
	}
	if n.Desc != nil {
		cmsMLUfree(n.Desc)
	}
	cmsFree(self.ContextID, unsafe.Pointer(n))
	*nItems = 0
	return nil
}

func TypeUcrBgWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	value := (*cmsUcrBg)(ptr)
	var textSize uint32

	// First curve is Under color removal
	if !cmsWriteUInt32Number(io, value.Ucr.nEntries) || !cmsWriteUInt16Array(io, value.Ucr.nEntries, unsafe.Slice(value.Ucr.Table16, value.Ucr.nEntries)) {
		return false
	}

	// Then black generation
	if !cmsWriteUInt32Number(io, value.Bg.nEntries) || !cmsWriteUInt16Array(io, value.Bg.nEntries, unsafe.Slice(value.Bg.Table16, value.Bg.nEntries)) {
		return false
	}

	// Now comes the text
	textSize = cmsMLUgetASCII(value.Desc, cmsNoLanguage, cmsNoCountry, nil, 0)
	text := (*[1 << 30]byte)(cmsMalloc(self.ContextID, textSize))[:textSize]
	if cmsMLUgetASCII(value.Desc, cmsNoLanguage, cmsNoCountry, &text[0], textSize) != textSize {
		return false
	}

	if !io.Write((*cms_io_handler)(io), textSize, unsafe.Pointer(&text[0])) {
		return false
	}

	cmsFree(self.ContextID, unsafe.Pointer(&text[0]))
	return true
}

func TypeUcrBgDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	src := (*cmsUcrBg)(ptr)
	newUcrBg := (*cmsUcrBg)(cmsMallocZero(self.ContextID, uint32(unsafe.Sizeof(cmsUcrBg{}))))
	if newUcrBg == nil {
		return nil
	}

	newUcrBg.Bg = cmsDupToneCurve(src.Bg)
	newUcrBg.Ucr = cmsDupToneCurve(src.Ucr)
	newUcrBg.Desc = cmsMLUdup(src.Desc)
	return unsafe.Pointer(newUcrBg)
}

func TypeUcrBgFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	src := (*cmsUcrBg)(ptr)
	if src.Ucr != nil {
		CmsFreeToneCurve(src.Ucr)
	}
	if src.Bg != nil {
		CmsFreeToneCurve(src.Bg)
	}
	if src.Desc != nil {
		cmsMLUfree(src.Desc)
	}
	cmsFree(self.ContextID, ptr)
}

// ********************************************************************************
// Type cmsSigCrdInfoType
// ********************************************************************************

/*
This type contains the PostScript product name to which this profile corresponds
and the names of the companion CRDs. Recall that a single profile can generate
multiple CRDs. It is implemented as a MLU being the language code "PS" and then
country varies for each element:

                nm: PostScript product name
                #0: Rendering intent 0 CRD name
                #1: Rendering intent 1 CRD name
                #2: Rendering intent 2 CRD name
                #3: Rendering intent 3 CRD name
*/

func ReadCountAndString(self *cmsTagTypeHandler, io *cmsIOHANDLER, mlu *cmsMLU, sizeOfTag *uint32, section string) bool {
	var count uint32

	// Check if there is enough space for count
	if *sizeOfTag < uint32(unsafe.Sizeof(count)) {
		return false
	}

	// Read count
	if !cmsReadUInt32Number(io, &count) {
		return false
	}

	// Validate count and remaining tag size
	if count > math.MaxUint32-uint32(unsafe.Sizeof(count)) || *sizeOfTag < count+uint32(unsafe.Sizeof(count)) {
		return false
	}

	// Allocate memory for the string
	text := (*[1 << 30]byte)(cmsMalloc(self.ContextID, count+1))[:count+1]
	if text == nil {
		return false
	}

	// Read string
	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&text[0]), 1, count) != count {
		cmsFree(self.ContextID, unsafe.Pointer(&text[0]))
		return false
	}

	// Null-terminate the string
	text[count] = 0

	// Set the string in the MLU
	cmsMLUsetASCII(mlu, "PS", section, &text[0])

	// Free temporary memory
	cmsFree(self.ContextID, unsafe.Pointer(&text[0]))

	// Update size of tag
	*sizeOfTag -= count + uint32(unsafe.Sizeof(count))
	return true
}

func WriteCountAndString(self *cmsTagTypeHandler, io *cmsIOHANDLER, mlu *cmsMLU, section string) bool {
	textSize := cmsMLUgetASCII(mlu, "PS", section, nil, 0)
	text := (*[1 << 30]byte)(cmsMalloc(self.ContextID, textSize))[:textSize]

	if text == nil {
		return false
	}

	// Write size of string
	if !cmsWriteUInt32Number(io, textSize) {
		return false
	}

	// Get the string
	if cmsMLUgetASCII(mlu, "PS", section, &text[0], textSize) == 0 {
		return false
	}

	// Write the string
	if !io.Write((*cms_io_handler)(io), textSize, unsafe.Pointer(&text[0])) {
		return false
	}

	// Free temporary memory
	cmsFree(self.ContextID, unsafe.Pointer(&text[0]))
	return true
}

func TypeCrdInfoRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	mlu := cmsMLUalloc(self.ContextID, 5)

	*nItems = 0
	if mlu == nil {
		return nil
	}

	// Read strings for each section
	if !ReadCountAndString(self, io, mlu, &sizeOfTag, "nm") ||
		!ReadCountAndString(self, io, mlu, &sizeOfTag, "#0") ||
		!ReadCountAndString(self, io, mlu, &sizeOfTag, "#1") ||
		!ReadCountAndString(self, io, mlu, &sizeOfTag, "#2") ||
		!ReadCountAndString(self, io, mlu, &sizeOfTag, "#3") {
		cmsMLUfree(mlu)
		return nil
	}

	*nItems = 1
	return unsafe.Pointer(mlu)
}

func TypeCrdInfoWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	mlu := (*cmsMLU)(ptr)

	// Write strings for each section
	if !WriteCountAndString(self, io, mlu, "nm") ||
		!WriteCountAndString(self, io, mlu, "#0") ||
		!WriteCountAndString(self, io, mlu, "#1") ||
		!WriteCountAndString(self, io, mlu, "#2") ||
		!WriteCountAndString(self, io, mlu, "#3") {
		return false
	}

	return true
}

func TypeCrdInfoDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, nItems uint32) unsafe.Pointer {
	mlu := (*cmsMLU)(ptr)
	return unsafe.Pointer(cmsMLUdup(mlu))
}

func TypeCrdInfoFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	mlu := (*cmsMLU)(ptr)
	cmsMLUfree(mlu)
}

// ********************************************************************************
// Type cmsSigScreeningType
// ********************************************************************************
//
//The screeningType describes various screening parameters including screen
//frequency, screening angle, and spot shape.

/*func TypeScreeningRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	sc := (*cmsScreening)(cmsMallocZero(self.ContextID, uint32(unsafe.Sizeof(cmsScreening{}))))
	if sc == nil {
		return nil
	}

	*nItems = 0

	// Read the Flag and nChannels
	if !cmsReadUInt32Number(io, &sc.Flag) || !cmsReadUInt32Number(io, &sc.nChannels) {
		cmsFree(self.ContextID, unsafe.Pointer(sc))
		return nil
	}

	// Limit the number of channels to cmsMAXCHANNELS - 1
	if sc.nChannels > cmsMAXCHANNELS-1 {
		sc.nChannels = cmsMAXCHANNELS - 1
	}

	// Read data for each channel
	for i := uint32(0); i < sc.nChannels; i++ {
		channel := &sc.Channels[i]
		if !cmsRead15Fixed16Number(io, &channel.Frequency) ||
			!cmsRead15Fixed16Number(io, &channel.ScreenAngle) ||
			!cmsReadUInt32Number(io, &channel.SpotShape) {
			cmsFree(self.ContextID, unsafe.Pointer(sc))
			return nil
		}
	}

	*nItems = 1
	return unsafe.Pointer(sc)
}

func TypeScreeningWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	sc := (*cmsScreening)(ptr)

	// Write the Flag and nChannels
	if !cmsWriteUInt32Number(io, sc.Flag) || !cmsWriteUInt32Number(io, sc.nChannels) {
		return false
	}

	// Write data for each channel
	for i := uint32(0); i < sc.nChannels; i++ {
		channel := &sc.Channels[i]
		if !cmsWrite15Fixed16Number(io, channel.Frequency) ||
			!cmsWrite15Fixed16Number(io, channel.ScreenAngle) ||
			!cmsWriteUInt32Number(io, channel.SpotShape) {
			return false
		}
	}

	return true
}

func TypeScreeningDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return cmsDupMem(self.ContextID, ptr, uint32(unsafe.Sizeof(cmsScreening{})))
}

func TypeScreeningFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFree(self.ContextID, ptr)
}
*/

// ********************************************************************************
// Type cmsSigDataType
// ********************************************************************************

func TypeDataRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	if sizeOfTag < uint32(unsafe.Sizeof(uint32(0))) {
		return nil
	}

	lenOfData := sizeOfTag - uint32(unsafe.Sizeof(uint32(0)))
	if lenOfData > math.MaxInt32 {
		return nil
	}

	binData := (*cmsICCData)(cmsMalloc(self.ContextID, uint32(unsafe.Sizeof(cmsICCData{}))+lenOfData-1))
	if binData == nil {
		return nil
	}

	binData.len = lenOfData
	if !cmsReadUInt32Number(io, &binData.flag) {
		cmsFree(self.ContextID, unsafe.Pointer(binData))
		return nil
	}

	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&binData.data[0]), 1, lenOfData) != lenOfData {
		cmsFree(self.ContextID, unsafe.Pointer(binData))
		return nil
	}

	*nItems = 1
	return unsafe.Pointer(binData)
}

func TypeDataWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	binData := (*cmsICCData)(ptr)

	if !cmsWriteUInt32Number(io, binData.flag) {
		return false
	}

	return io.Write((*cms_io_handler)(io), binData.len, unsafe.Pointer(&binData.data[0]))
}

func TypeDataDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	binData := (*cmsICCData)(ptr)
	return cmsDupMem(self.ContextID, ptr, uint32(unsafe.Sizeof(cmsICCData{}))+binData.len-1)
}

func TypeDataFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFree(self.ContextID, ptr)
}

// LutAtoB type

// This structure represents a colour transform. The type contains up to five processing
// elements which are stored in the AtoBTag tag in the following order: a set of one
// dimensional curves, a 3 by 3 matrix with offset terms, a set of one dimensional curves,
// a multidimensional lookup table, and a set of one dimensional output curves.
// Data are processed using these elements via the following sequence:
//
//("A" curves) -> (multidimensional lookup table - CLUT) -> ("M" curves) -> (matrix) -> ("B" curves).
//
/*
It is possible to use any or all of these processing elements. At least one processing element
must be included.Only the following combinations are allowed:

B
M - Matrix - B
A - CLUT - B
A - CLUT - M - Matrix - B

*/
func TypeLUTA2BRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var (
		baseOffset = io.Tell((*cms_io_handler)(io)) - uint32(unsafe.Sizeof(cmsTagBase{}))
		inputChan  uint8
		outputChan uint8
		offsetB    uint32
		offsetMat  uint32
		offsetM    uint32
		offsetC    uint32
		offsetA    uint32
	)

	*nItems = 0

	// Read channel counts and offsets
	if !cmsReadUInt8Number(io, &inputChan) || !cmsReadUInt8Number(io, &outputChan) || !cmsReadUInt16Number(io, nil) ||
		!cmsReadUInt32Number(io, &offsetB) || !cmsReadUInt32Number(io, &offsetMat) ||
		!cmsReadUInt32Number(io, &offsetM) || !cmsReadUInt32Number(io, &offsetC) || !cmsReadUInt32Number(io, &offsetA) {
		return nil
	}

	if inputChan == 0 || inputChan >= cmsMAXCHANNELS || outputChan == 0 || outputChan >= cmsMAXCHANNELS {
		return nil
	}

	// Allocate an empty LUT
	newLUT := cmsPipelineAlloc(self.ContextID, uint32(inputChan), uint32(outputChan))
	if newLUT == nil {
		return nil
	}

	// Process each offset and add corresponding stages to the pipeline
	if offsetA != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadSetOfCurves(self, io, baseOffset+offsetA, uint32(inputChan))) {
			goto Error
		}
	}

	if offsetC != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadCLUT(self, io, baseOffset+offsetC, uint32(inputChan), uint32(outputChan))) {
			goto Error
		}
	}

	if offsetM != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadSetOfCurves(self, io, baseOffset+offsetM, uint32(outputChan))) {
			goto Error
		}
	}

	if offsetMat != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadMatrix(self, io, baseOffset+offsetMat)) {
			goto Error
		}
	}

	if offsetB != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadSetOfCurves(self, io, baseOffset+offsetB, uint32(outputChan))) {
			goto Error
		}
	}

	*nItems = 1
	return unsafe.Pointer(newLUT)

Error:
	cmsPipelineFree(newLUT)
	return nil
}

func TypeLUTA2BWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	lut := (*cmsPipeline)(ptr)
	var (
		a, b, m, clut, matrix                         *cmsStage
		offsetB, offsetMat, offsetM, offsetC, offsetA uint32
		baseOffset, directoryPos, currentPos          uint32
	)

	// Get the base for all offsets
	baseOffset = uint32(io.Tell((*cms_io_handler)(io))) - uint32(unsafe.Sizeof(cmsTagBase{}))

	// Check and retrieve stages
	// Check and retrieve stages
	// Check and retrieve stages
	if lut.Elements != nil {
		if !(cmsPipelineCheckAndRetrieveStages(lut, 1, []cmsStageSignature{cmsSigCurveSetElemType}, &b) ||
			cmsPipelineCheckAndRetrieveStages(lut, 3, []cmsStageSignature{cmsSigCurveSetElemType, cmsSigMatrixElemType, cmsSigCurveSetElemType}, &m, &matrix, &b) ||
			cmsPipelineCheckAndRetrieveStages(lut, 3, []cmsStageSignature{cmsSigCurveSetElemType, cmsSigCLutElemType, cmsSigCurveSetElemType}, &a, &clut, &b) ||
			cmsPipelineCheckAndRetrieveStages(lut, 5, []cmsStageSignature{cmsSigCurveSetElemType, cmsSigCLutElemType, cmsSigCurveSetElemType, cmsSigMatrixElemType, cmsSigCurveSetElemType}, &a, &clut, &m, &matrix, &b)) {
			cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_NOT_SUITABLE, "LUT is not suitable to be saved as LutAToB")
			return false
		}
	}

	// Get input and output channels
	inputChan := cmsPipelineInputChannels(lut)
	outputChan := cmsPipelineOutputChannels(lut)

	// Write channel count
	if !cmsWriteUInt8Number(io, uint8(inputChan)) || !cmsWriteUInt8Number(io, uint8(outputChan)) || !cmsWriteUInt16Number(io, 0) {
		return false
	}

	// Keep directory to be filled later
	directoryPos = uint32(io.Tell((*cms_io_handler)(io)))

	// Write the directory
	//this is a  piece from C code with five repeated conditions
	if !cmsWriteUInt32Number(io, 0) {
		return false
	}
	if !cmsWriteUInt32Number(io, 0) {
		return false
	}
	if !cmsWriteUInt32Number(io, 0) {
		return false
	}
	if !cmsWriteUInt32Number(io, 0) {
		return false
	}
	if !cmsWriteUInt32Number(io, 0) {
		return false
	}

	// Write the stages
	if a != nil {
		offsetA = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteSetOfCurves(self, io, cmsSigParametricCurveType, a) {
			return false
		}
	}

	if clut != nil {
		offsetC = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		precision := uint8(2)
		if lut.SaveAs8Bits {
			precision = 1
		}
		if !WriteCLUT(self, io, precision, clut) {
			return false
		}
	}

	if m != nil {
		offsetM = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteSetOfCurves(self, io, cmsSigParametricCurveType, m) {
			return false
		}
	}

	if matrix != nil {
		offsetMat = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteMatrix(self, io, matrix) {
			return false
		}
	}

	if b != nil {
		offsetB = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteSetOfCurves(self, io, cmsSigParametricCurveType, b) {
			return false
		}
	}

	// Fill the directory
	currentPos = uint32(io.Tell((*cms_io_handler)(io)))
	if !io.Seek((*cms_io_handler)(io), directoryPos) {
		return false
	}

	if !cmsWriteUInt32Number(io, offsetB) || !cmsWriteUInt32Number(io, offsetMat) || !cmsWriteUInt32Number(io, offsetM) || !cmsWriteUInt32Number(io, offsetC) || !cmsWriteUInt32Number(io, offsetA) {
		return false
	}

	if !io.Seek((*cms_io_handler)(io), uint32(currentPos)) {
		return false
	}

	return true
}

func TypeLUTA2BDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsPipelineDup((*cmsPipeline)(ptr)))
}

func TypeLUTA2BFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsPipelineFree((*cmsPipeline)(ptr))
}

func WriteMatrix(self *cmsTagTypeHandler, io *cmsIOHANDLER, mpe *cmsStage) bool {
	matrixData := (*cmsStageMatrixData)(mpe.Data)
	n := mpe.InputChannels * mpe.OutputChannels

	// Write the matrix values
	matrix := unsafe.Slice(matrixData.Double, n)
	for i := uint32(0); i < n; i++ {
		if !cmsWrite15Fixed16Number(io, matrix[i]) {
			return false
		}
	}

	// Write the offsets
	if matrixData.Offset != nil {
		offsets := unsafe.Slice(matrixData.Offset, mpe.OutputChannels)
		for i := uint32(0); i < mpe.OutputChannels; i++ {
			if !cmsWrite15Fixed16Number(io, offsets[i]) {
				return false
			}
		}
	} else {
		for i := uint32(0); i < mpe.OutputChannels; i++ {
			if !cmsWrite15Fixed16Number(io, 0) {
				return false
			}
		}
	}

	return true
}
func WriteSetOfCurves(self *cmsTagTypeHandler, io *cmsIOHANDLER, curveType cmsTagTypeSignature, mpe *cmsStage) bool {
	curves := cmsStageGetPtrToCurveSet(mpe)
	outputChannels := cmsStageOutputChannels(mpe)

	for i := uint32(0); i < outputChannels; i++ {
		currentType := curveType
		curve := (**CmsToneCurve)(unsafe.Add(unsafe.Pointer(curves), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))
		if (*curve).Segments != nil {
			segments := unsafe.Slice((*curve).Segments, (*curve).nSegments)

			// Determine the curve type
			if (*curve).nSegments == 0 || ((*curve).nSegments == 2 && segments[1].Type == 0) || segments[0].Type < 0 {
				currentType = cmsSigCurveType
			}
		}

		// Write the curve type
		if !cmsWriteTypeBase(io, currentType) {
			return false
		}

		// Write the curve data
		switch currentType {
		case cmsSigCurveType:
			if !TypeCurveWrite(self, io, unsafe.Pointer(curve), 1) {
				return false
			}
		case cmsSigParametricCurveType:
			if !TypeParametricCurveWrite(self, io, unsafe.Pointer(curve), 1) {
				return false
			}
		default:
			cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unknown curve type")
			return false
		}

		if !cmsWriteAlignment(io) {
			return false
		}
	}

	return true
}
func WriteCLUT(self *cmsTagTypeHandler, io *cmsIOHANDLER, precision uint8, mpe *cmsStage) bool {
	clutData := (*cmsStageCLutData)(mpe.Data)
	var gridPoints [cmsMAXCHANNELS]uint8
	if clutData.HasFloatValues {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_NOT_SUITABLE, "Cannot save floating point data, CLUTs are 8 or 16-bit only")
		return false
	}

	// Write the grid points
	for i := 0; i < int(clutData.Params.nInputs); i++ {
		gridPoints[i] = uint8(clutData.Params.nSamples[i])
	}

	if !io.Write((*cms_io_handler)(io), cmsMAXCHANNELS, unsafe.Pointer(&gridPoints[0])) {
		return false
	}

	// Write precision and padding
	if !cmsWriteUInt8Number(io, precision) || !cmsWriteUInt8Number(io, 0) || !cmsWriteUInt8Number(io, 0) || !cmsWriteUInt8Number(io, 0) {
		return false
	}

	// Write the CLUT data
	switch precision {
	case 1:
		clutEntries := unsafe.Slice(clutData.Tab.T, clutData.NEntries)
		for _, entry := range clutEntries {
			if !cmsWriteUInt8Number(io, uint8(FROM_16_TO_8(entry))) {
				return false
			}
		}
	case 2:
		clutEntries := unsafe.Slice(clutData.Tab.T, clutData.NEntries)
		if !cmsWriteUInt16Array(io, clutData.NEntries, clutEntries) {
			return false
		}
	default:
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unknown precision")
		return false
	}

	return cmsWriteAlignment(io)
}
func ReadMatrix(self *cmsTagTypeHandler, io *cmsIOHANDLER, offset uint32) *cmsStage {
	var dMat [9]float64
	var dOff [3]float64

	// Go to address
	if !io.Seek((*cms_io_handler)(io), offset) {
		return nil
	}

	// Read the matrix
	for i := 0; i < 9; i++ {
		if !cmsRead15Fixed16Number(io, &dMat[i]) {
			return nil
		}
	}

	// Read the offsets
	for i := 0; i < 3; i++ {
		if !cmsRead15Fixed16Number(io, &dOff[i]) {
			return nil
		}
	}

	// Allocate the matrix
	return cmsStageAllocMatrix(self.ContextID, 3, 3, &dMat[0], &dOff[0])
}
func ReadCLUT(self *cmsTagTypeHandler, io *cmsIOHANDLER, offset, inputChannels, outputChannels uint32) *cmsStage {
	var gridPoints8 [cmsMAXCHANNELS]uint8
	var gridPoints [cmsMAXCHANNELS]uint32
	var precision uint8

	// Seek to offset
	if !io.Seek((*cms_io_handler)(io), offset) {
		return nil
	}

	// Read the grid points
	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&gridPoints8[0]), cmsMAXCHANNELS, 1) != 1 {
		return nil
	}

	for i := 0; i < cmsMAXCHANNELS; i++ {
		if gridPoints8[i] == 1 {
			return nil // Impossible value
		}
		gridPoints[i] = uint32(gridPoints8[i])
	}

	// Read the precision and padding
	if !cmsReadUInt8Number(io, &precision) ||
		!cmsReadUInt8Number(io, nil) ||
		!cmsReadUInt8Number(io, nil) ||
		!cmsReadUInt8Number(io, nil) {
		return nil
	}

	// Allocate the CLUT
	clut := cmsStageAllocCLut16bitGranular(self.ContextID, gridPoints[:inputChannels], inputChannels, outputChannels, nil)
	if clut == nil {
		return nil
	}

	data := (*cmsStageCLutData)(clut.Data)

	// Read the CLUT data
	switch precision {
	case 1:
		var value uint8
		// Convert the pointer `data.Tab.T` into a slice of size `data.NEntries`
		tab := unsafe.Slice(data.Tab.T, data.NEntries)

		for i := uint32(0); i < data.NEntries; i++ {
			if io.Read((*cms_io_handler)(io), unsafe.Pointer(&value), 1, 1) != 1 {
				cmsStageFree(clut)
				return nil
			}
			tab[i] = uint16(value) * 257 // Convert 8-bit to 16-bit
		}
	case 2:
		if !cmsReadUInt16Array(io, data.NEntries, data.Tab.T) {
			cmsStageFree(clut)
			return nil
		}
	default:
		cmsStageFree(clut)
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unknown precision")
		return nil
	}

	return clut
}
func ReadEmbeddedCurve(self *cmsTagTypeHandler, io *cmsIOHANDLER) *CmsToneCurve {
	baseType := cmsReadTypeBase(io)

	switch baseType {
	case cmsSigCurveType:
		return (*CmsToneCurve)(TypeCurveRead(self, io, nil, 0))
	case cmsSigParametricCurveType:
		return (*CmsToneCurve)(TypeParametricCurveRead(self, io, nil, 0))
	default:
		var signature [5]byte
		cmsTagSignature2String(signature, cmsTagSignature(baseType))
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unknown curve type ")
		return nil
	}
}
func ReadSetOfCurves(self *cmsTagTypeHandler, io *cmsIOHANDLER, offset, nCurves uint32) *cmsStage {
	if nCurves > cmsMAXCHANNELS {
		return nil
	}

	// Seek to the offset
	if !io.Seek((*cms_io_handler)(io), offset) {
		return nil
	}

	var curves [cmsMAXCHANNELS]*CmsToneCurve
	for i := uint32(0); i < nCurves; i++ {
		curves[i] = ReadEmbeddedCurve(self, io)
		if curves[i] == nil || !cmsReadAlignment(io) {
			for j := uint32(0); j < i; j++ {
				CmsFreeToneCurve(curves[j])
			}
			return nil
		}
	}

	// Allocate the tone curves stage
	stage := cmsStageAllocToneCurves(self.ContextID, nCurves, &curves[0])

	// Free the individual curves
	for i := uint32(0); i < nCurves; i++ {
		CmsFreeToneCurve(curves[i])
	}

	return stage
}

/*
B
B - Matrix - M
B - CLUT - A
B - Matrix - M - CLUT - A
*/

func TypeLUTB2ARead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var (
		inputChan, outputChan                                     uint8
		offsetB, offsetMat, offsetM, offsetC, offsetA, baseOffset uint32
		newLUT                                                    *cmsPipeline
	)

	baseOffset = uint32(io.Tell((*cms_io_handler)(io))) - uint32(unsafe.Sizeof(cmsTagBase{}))

	if !cmsReadUInt8Number(io, &inputChan) || !cmsReadUInt8Number(io, &outputChan) {
		return nil
	}

	if inputChan == 0 || inputChan >= cmsMAXCHANNELS || outputChan == 0 || outputChan >= cmsMAXCHANNELS {
		return nil
	}

	// Padding
	if !cmsReadUInt16Number(io, nil) {
		return nil
	}

	if !cmsReadUInt32Number(io, &offsetB) || !cmsReadUInt32Number(io, &offsetMat) ||
		!cmsReadUInt32Number(io, &offsetM) || !cmsReadUInt32Number(io, &offsetC) ||
		!cmsReadUInt32Number(io, &offsetA) {
		return nil
	}

	// Allocate an empty LUT
	newLUT = cmsPipelineAlloc(self.ContextID, uint32(inputChan), uint32(outputChan))
	if newLUT == nil {
		return nil
	}

	if offsetB != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadSetOfCurves(self, io, baseOffset+offsetB, uint32(inputChan))) {
			goto Error
		}
	}

	if offsetMat != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadMatrix(self, io, baseOffset+offsetMat)) {
			goto Error
		}
	}

	if offsetM != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadSetOfCurves(self, io, baseOffset+offsetM, uint32(inputChan))) {
			goto Error
		}
	}

	if offsetC != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadCLUT(self, io, baseOffset+offsetC, uint32(inputChan), uint32(outputChan))) {
			goto Error
		}
	}

	if offsetA != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadSetOfCurves(self, io, baseOffset+offsetA, uint32(outputChan))) {
			goto Error
		}
	}

	*nItems = 1
	return unsafe.Pointer(newLUT)

Error:
	cmsPipelineFree(newLUT)
	return nil
}

func TypeLUTB2AWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	lut := (*cmsPipeline)(ptr)
	var (
		inputChan, outputChan                                                               uint32
		a, b, m, matrix, clut                                                               *cmsStage
		offsetB, offsetMat, offsetM, offsetC, offsetA, baseOffset, directoryPos, currentPos uint32
	)

	baseOffset = uint32(io.Tell((*cms_io_handler)(io))) - uint32(unsafe.Sizeof(cmsTagBase{}))

	// Check and retrieve stages
	if !cmsPipelineCheckAndRetrieveStages(lut, 1, []cmsStageSignature{cmsSigCurveSetElemType}, &b) &&
		!cmsPipelineCheckAndRetrieveStages(lut, 3, []cmsStageSignature{cmsSigCurveSetElemType, cmsSigMatrixElemType, cmsSigCurveSetElemType}, &b, &matrix, &m) &&
		!cmsPipelineCheckAndRetrieveStages(lut, 3, []cmsStageSignature{cmsSigCurveSetElemType, cmsSigCLutElemType, cmsSigCurveSetElemType}, &b, &clut, &a) &&
		!cmsPipelineCheckAndRetrieveStages(lut, 5, []cmsStageSignature{cmsSigCurveSetElemType, cmsSigMatrixElemType, cmsSigCurveSetElemType, cmsSigCLutElemType, cmsSigCurveSetElemType}, &b, &matrix, &m, &clut, &a) {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_NOT_SUITABLE, "LUT is not suitable to be saved as LutBToA")
		return false
	}

	inputChan = cmsPipelineInputChannels(lut)
	outputChan = cmsPipelineOutputChannels(lut)

	if !cmsWriteUInt8Number(io, uint8(inputChan)) || !cmsWriteUInt8Number(io, uint8(outputChan)) || !cmsWriteUInt16Number(io, 0) {
		return false
	}

	directoryPos = uint32(io.Tell((*cms_io_handler)(io)))

	if !cmsWriteUInt32Number(io, 0) || !cmsWriteUInt32Number(io, 0) || !cmsWriteUInt32Number(io, 0) || !cmsWriteUInt32Number(io, 0) || !cmsWriteUInt32Number(io, 0) {
		return false
	}

	if a != nil {
		offsetA = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteSetOfCurves(self, io, cmsSigParametricCurveType, a) {
			return false
		}
	}

	if clut != nil {
		offsetC = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		precision := uint8(2)
		if lut.SaveAs8Bits {
			precision = 1
		}
		if !WriteCLUT(self, io, precision, clut) {
			return false
		}
	}

	if m != nil {
		offsetM = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteSetOfCurves(self, io, cmsSigParametricCurveType, m) {
			return false
		}
	}

	if matrix != nil {
		offsetMat = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteMatrix(self, io, matrix) {
			return false
		}
	}

	if b != nil {
		offsetB = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteSetOfCurves(self, io, cmsSigParametricCurveType, b) {
			return false
		}
	}

	currentPos = uint32(io.Tell((*cms_io_handler)(io)))

	if !io.Seek((*cms_io_handler)(io), directoryPos) {
		return false
	}

	if !cmsWriteUInt32Number(io, offsetB) || !cmsWriteUInt32Number(io, offsetMat) || !cmsWriteUInt32Number(io, offsetM) || !cmsWriteUInt32Number(io, offsetC) || !cmsWriteUInt32Number(io, offsetA) {
		return false
	}

	if !io.Seek((*cms_io_handler)(io), currentPos) {
		return false
	}

	return true
}

func TypeLUTB2ADup(self *cmsTagTypeHandler, ptr unsafe.Pointer, nItems uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsPipelineDup((*cmsPipeline)(ptr)))
}

func TypeLUTB2AFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsPipelineFree((*cmsPipeline)(ptr))
}

// This is the list of built-in MPE types

// ReadMPEElem reads a single multi-processing element (MPE).
func ReadMPEElem(self *cmsTagTypeHandler, io *cmsIOHANDLER, cargo unsafe.Pointer, n, sizeOfTag uint32) bool {
	var elementSig cmsStageSignature
	var typeHandler *cmsTagTypeHandler
	var nItems uint32
	newLUT := (*cmsPipeline)(cargo)
	mpeTypePluginChunk := (*cmsTagTypePluginChunkType)(CmsContextGetClientChunk(self.ContextID, MPEPlugin))

	// Read the element signature
	if !cmsReadUInt32Number(io, (*uint32)(&elementSig)) {
		return false
	}

	// Skip the reserved placeholder
	if !cmsReadUInt32Number(io, nil) {
		return false
	}

	// Get the handler for the MPE type
	typeHandler = GetHandler(cmsTagTypeSignature(elementSig), mpeTypePluginChunk.TagTypes, &SupportedMPEtypes[0])
	if typeHandler == nil {
		var elementSigStr [5]byte
		cmsTagSignature2String(elementSigStr, cmsTagSignature(elementSig))
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, fmt.Sprintf("Unknown MPE type '%s' found.", string(elementSigStr[:])))
		return false
	}

	// If there's no read method, ignore the element
	if typeHandler.ReadFn != nil {
		// Read the MPE and insert it into the pipeline
		stage := (*cmsStage)(typeHandler.ReadFn(self, io, &nItems, sizeOfTag))
		if stage == nil || !cmsPipelineInsertStage(newLUT, cmsAT_END, stage) {
			return false
		}
	}

	return true
}

// This is the main dispatcher for MPE

func TypeMPERead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var (
		inputChans, outputChans  uint16
		elementCount, baseOffset uint32
		newLUT                   *cmsPipeline
	)

	// Get current file position as base offset
	baseOffset = uint32(io.Tell((*cms_io_handler)(io))) - uint32(unsafe.Sizeof(cmsTagBase{}))

	// Read input and output channel counts
	if !cmsReadUInt16Number(io, &inputChans) || !cmsReadUInt16Number(io, &outputChans) {
		return nil
	}

	// Check channel counts
	if inputChans == 0 || inputChans >= cmsMAXCHANNELS || outputChans == 0 || outputChans >= cmsMAXCHANNELS {
		return nil
	}

	// Allocate an empty LUT
	newLUT = cmsPipelineAlloc(self.ContextID, uint32(inputChans), uint32(outputChans))
	if newLUT == nil {
		return nil
	}

	// Read the element count
	if !cmsReadUInt32Number(io, &elementCount) {
		goto Error
	}

	// Read position table and elements
	if !ReadPositionTable(self, io, elementCount, baseOffset, unsafe.Pointer(newLUT), ReadMPEElem) {
		goto Error
	}

	// Verify channel counts
	if inputChans != uint16(newLUT.InputChannels) || outputChans != uint16(newLUT.OutputChannels) {
		goto Error
	}

	*nItems = 1
	return unsafe.Pointer(newLUT)

Error:
	if newLUT != nil {
		cmsPipelineFree(newLUT)
	}
	*nItems = 0
	return nil
}

// This one is a little bit more complex, so we don't use position tables this time.

func TypeMPEWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	var (
		i, baseOffset, directoryPos, currentPos uint32
		elementOffsets, elementSizes            *uint32
		before, elementCount                    uint32
		elementSig                              cmsStageSignature
		typeHandler                             *cmsTagTypeHandler
		mpeTypePluginChunk                      *cmsTagTypePluginChunkType
		lut                                     *cmsPipeline
		elem                                    *cmsStage
	)

	lut = (*cmsPipeline)(ptr)
	baseOffset = uint32(io.Tell((*cms_io_handler)(io))) - uint32(unsafe.Sizeof(cmsTagBase{}))

	// Retrieve input/output channels and element count
	inputChan := cmsPipelineInputChannels(lut)
	outputChan := cmsPipelineOutputChannels(lut)
	elementCount = cmsPipelineStageCount(lut)
	elem = lut.Elements

	// Allocate space for offsets and sizes
	elementOffsets = (*uint32)(cmsCalloc(self.ContextID, elementCount, uint32(unsafe.Sizeof(uint32(0)))))
	if elementOffsets == nil {
		goto Error
	}

	elementSizes = (*uint32)(cmsCalloc(self.ContextID, elementCount, uint32(unsafe.Sizeof(uint32(0)))))
	if elementSizes == nil {
		goto Error
	}

	// Write header
	if !cmsWriteUInt16Number(io, uint16(inputChan)) || !cmsWriteUInt16Number(io, uint16(outputChan)) || !cmsWriteUInt32Number(io, elementCount) {
		goto Error
	}

	// Write placeholder directory
	directoryPos = uint32(io.Tell((*cms_io_handler)(io)))
	// Write a fake directory to be filled latter on
	for i = 0; i < elementCount; i++ {
		if !cmsWriteUInt32Number(io, 0) {
			goto Error
		} //offset
		if !cmsWriteUInt32Number(io, 0) {
			goto Error
		} //size

	}

	// Retrieve the MPE type plugin chunk
	mpeTypePluginChunk = (*cmsTagTypePluginChunkType)(CmsContextGetClientChunk(self.ContextID, MPEPlugin))

	// Write each element
	for i = 0; i < elementCount; i++ {
		elementOffsetsPtr := (*uint32)(unsafe.Add(unsafe.Pointer(elementOffsets), uintptr(i)*unsafe.Sizeof(uint32(0))))
		*elementOffsetsPtr = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		elementSig = elem.Type

		typeHandler = GetHandler(cmsTagTypeSignature(elementSig), mpeTypePluginChunk.TagTypes, &SupportedMPEtypes[0])
		if typeHandler == nil {
			var signature [5]byte
			cmsTagSignature2String(signature, cmsTagSignature(elementSig))
			cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Found unknown MPE type")
			goto Error
		}

		if !cmsWriteUInt32Number(io, uint32(elementSig)) || !cmsWriteUInt32Number(io, 0) {
			goto Error
		}

		before = uint32(io.Tell((*cms_io_handler)(io)))
		if !typeHandler.WriteFn(self, io, unsafe.Pointer(elem), 1) {
			goto Error
		}

		if !cmsWriteAlignment(io) {
			goto Error
		}
		elementSizesPtr := (*uint32)(unsafe.Add(unsafe.Pointer(elementSizes), uintptr(i)*unsafe.Sizeof(uint32(0))))
		*elementSizesPtr = uint32(io.Tell((*cms_io_handler)(io))) - before
		elem = elem.Next
	}

	// Write directory with actual offsets and sizes
	currentPos = uint32(io.Tell((*cms_io_handler)(io)))
	if !io.Seek((*cms_io_handler)(io), directoryPos) {
		goto Error
	}

	for i = 0; i < elementCount; i++ {
		elementSizesPtr := (*uint32)(unsafe.Add(unsafe.Pointer(elementSizes), uintptr(i)*unsafe.Sizeof(uint32(0))))
		elementOffsetsPtr := (*uint32)(unsafe.Add(unsafe.Pointer(elementOffsets), uintptr(i)*unsafe.Sizeof(uint32(0))))
		if !cmsWriteUInt32Number(io, *elementOffsetsPtr) || !cmsWriteUInt32Number(io, *elementSizesPtr) {
			goto Error
		}
	}

	if !io.Seek((*cms_io_handler)(io), currentPos) {
		goto Error
	}
	if elementOffsets != nil {
		cmsFree(self.ContextID, unsafe.Pointer(elementOffsets))
	}
	if elementSizes != nil {
		cmsFree(self.ContextID, unsafe.Pointer(elementSizes))
	}

	return true

Error:
	if elementOffsets != nil {
		cmsFree(self.ContextID, unsafe.Pointer(elementOffsets))
	}
	if elementSizes != nil {
		cmsFree(self.ContextID, unsafe.Pointer(elementSizes))
	}
	return false
}

func TypeMPEDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, nItems uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsPipelineDup((*cmsPipeline)(ptr)))
}

func TypeMPEFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsPipelineFree((*cmsPipeline)(ptr))
}

// ********************************************************************************
// Type cmsSigDictType
// ********************************************************************************
type cmsDICelem struct {
	ContextID CmsContext
	Offsets   []uint32
	Sizes     []uint32
}

type cmsDICarray struct {
	Name         cmsDICelem
	Value        cmsDICelem
	DisplayName  cmsDICelem
	DisplayValue cmsDICelem
}

// Allocate an empty array element
func AllocElem(contextID CmsContext, e *cmsDICelem, count uint32) bool {
	e.Offsets = make([]uint32, count)
	e.Sizes = make([]uint32, count)

	e.ContextID = contextID
	return e.Offsets != nil && e.Sizes != nil
}

// Free an array element
func FreeElem(e *cmsDICelem) {
	e.Offsets = nil
	e.Sizes = nil
}

// Free the entire array
func FreeArray(a *cmsDICarray) {
	FreeElem(&a.Name)
	FreeElem(&a.Value)
	FreeElem(&a.DisplayName)
	FreeElem(&a.DisplayValue)
}

// Allocate the entire array
func AllocArray(contextID CmsContext, a *cmsDICarray, count uint32, length uint32) bool {
	*a = cmsDICarray{}
	if !AllocElem(contextID, &a.Name, count) || !AllocElem(contextID, &a.Value, count) {
		goto Error
	}

	if length > 16 {
		if !AllocElem(contextID, &a.DisplayName, count) {
			goto Error
		}
	}
	if length > 24 {
		if !AllocElem(contextID, &a.DisplayValue, count) {
			goto Error
		}
	}
	return true

Error:
	FreeArray(a)
	return false
}

// Read a single element
func ReadOneElem(io *cmsIOHANDLER, e *cmsDICelem, i uint32, baseOffset uint32) bool {
	if !cmsReadUInt32Number(io, &e.Offsets[i]) || !cmsReadUInt32Number(io, &e.Sizes[i]) {
		return false
	}

	if e.Offsets[i] > 0 {
		e.Offsets[i] += baseOffset
	}
	return true
}

// Read offset array
func ReadOffsetArray(io *cmsIOHANDLER, a *cmsDICarray, count uint32, length uint32, baseOffset uint32, signedSizeOfTagPtr *int32) bool {
	signedSizeOfTag := *signedSizeOfTagPtr

	for i := uint32(0); i < count; i++ {
		if signedSizeOfTag < 4*int32(unsafe.Sizeof(uint32(0))) {
			return false
		}
		signedSizeOfTag -= 4 * int32(unsafe.Sizeof(uint32(0)))

		if !ReadOneElem(io, &a.Name, i, baseOffset) || !ReadOneElem(io, &a.Value, i, baseOffset) {
			return false
		}

		if length > 16 {
			if signedSizeOfTag < 2*int32(unsafe.Sizeof(uint32(0))) {
				return false
			}
			signedSizeOfTag -= 2 * int32(unsafe.Sizeof(uint32(0)))

			if !ReadOneElem(io, &a.DisplayName, i, baseOffset) {
				return false
			}
		}

		if length > 24 {
			if signedSizeOfTag < 2*int32(unsafe.Sizeof(uint32(0))) {
				return false
			}
			signedSizeOfTag -= 2 * int32(unsafe.Sizeof(uint32(0)))

			if !ReadOneElem(io, &a.DisplayValue, i, baseOffset) {
				return false
			}
		}
	}

	*signedSizeOfTagPtr = signedSizeOfTag
	return true
}

// Write a single element
func WriteOneElem(io *cmsIOHANDLER, e *cmsDICelem, i uint32) bool {
	return cmsWriteUInt32Number(io, e.Offsets[i]) && cmsWriteUInt32Number(io, e.Sizes[i])
}

// Write offset array
func WriteOffsetArray(io *cmsIOHANDLER, a *cmsDICarray, count uint32, length uint32) bool {
	for i := uint32(0); i < count; i++ {
		if !WriteOneElem(io, &a.Name, i) || !WriteOneElem(io, &a.Value, i) {
			return false
		}

		if length > 16 && !WriteOneElem(io, &a.DisplayName, i) {
			return false
		}

		if length > 24 && !WriteOneElem(io, &a.DisplayValue, i) {
			return false
		}
	}
	return true
}
func ReadOneWChar(io *cmsIOHANDLER, e *cmsDICelem, i uint32) (string, bool) {
	if e.Offsets[i] == 0 {
		// Special case for undefined strings
		return "", true
	}

	// Seek to the position of the string
	if !io.Seek((*cms_io_handler)(io), e.Offsets[i]) {
		return "", false
	}

	// Calculate the number of characters (assuming UTF-16)
	nChars := e.Sizes[i] / 2 // 2 bytes per character for UTF-16

	// Read the string data
	rawData := make([]uint16, nChars)
	if !cmsReadUInt16Array(io, nChars, &rawData[0]) {
		return "", false
	}

	// Convert UTF-16 to Go string
	return string(syscall.UTF16ToString(rawData)), true
}

// Read a single wchar string
func WriteOneWChar(io *cmsIOHANDLER, e *cmsDICelem, i uint32, str string, baseOffset uint32) bool {
	// Record the current offset
	before := uint32(io.Tell((*cms_io_handler)(io)))

	if str == "" {
		// Special case for empty strings
		e.Sizes[i] = 0
		e.Offsets[i] = 0
		return true
	}

	// Convert Go string to UTF-16
	utf16Data, _ := syscall.UTF16FromString(str)

	// Write the UTF-16 data
	if !cmsWriteUInt16Array(io, uint32(len(utf16Data)), utf16Data) {
		return false
	}

	// Calculate the size of the written data
	e.Sizes[i] = uint32(io.Tell((*cms_io_handler)(io))) - before
	e.Offsets[i] = before - baseOffset
	return true
}

// Read an MLUC element
func ReadOneMLUC(self *cmsTagTypeHandler, io *cmsIOHANDLER, e *cmsDICelem, i uint32, mlu **cmsMLU) bool {
	if e.Offsets[i] == 0 || e.Sizes[i] == 0 {
		*mlu = nil
		return true
	}

	if !io.Seek((*cms_io_handler)(io), e.Offsets[i]) {
		return false
	}
	var nItems uint32
	*mlu = (*cmsMLU)(TypeMLURead(self, io, &nItems, e.Sizes[i]))
	return *mlu != nil
}

// Write an MLUC element
func WriteOneMLUC(self *cmsTagTypeHandler, io *cmsIOHANDLER, e *cmsDICelem, i uint32, mlu *cmsMLU, baseOffset uint32) bool {
	if mlu == nil {
		e.Sizes[i] = 0
		e.Offsets[i] = 0
		return true
	}

	before := uint32(io.Tell((*cms_io_handler)(io)))
	e.Offsets[i] = before - baseOffset

	if !TypeMLUWrite(self, io, unsafe.Pointer(mlu), 1) {
		return false
	}

	e.Sizes[i] = uint32(io.Tell((*cms_io_handler)(io))) - before
	return true
}

func TypeDictionaryRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var (
		hDict           cmsHANDLE
		count, length   uint32
		baseOffset      uint32
		a               cmsDICarray
		displayNameMLU  *cmsMLU
		displayValueMLU *cmsMLU
		rc              bool
		signedSizeOfTag = int32(sizeOfTag)
	)

	*nItems = 0
	memset(unsafe.Pointer(&a), 0, uintptr(unsafe.Sizeof(cmsDICarray{})))

	// Get current position as base offset
	baseOffset = uint32(io.Tell((*cms_io_handler)(io))) - uint32(unsafe.Sizeof(cmsTagBase{}))

	// Read name-value record count
	signedSizeOfTag -= int32(unsafe.Sizeof(count))
	if signedSizeOfTag < 0 || !cmsReadUInt32Number(io, &count) {
		return nil
	}

	// Read record length
	signedSizeOfTag -= int32(unsafe.Sizeof(length))
	if signedSizeOfTag < 0 || !cmsReadUInt32Number(io, &length) {
		return nil
	}

	// Check valid lengths
	if length != 16 && length != 24 && length != 32 {
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, fmt.Sprintf("Unknown record length in dictionary"))
		return nil
	}

	// Create an empty dictionary
	hDict = cmsDictAlloc(self.ContextID)
	if hDict == nil {
		return nil
	}

	// Allocate column arrays
	if !AllocArray(self.ContextID, &a, count, length) {
		goto Error
	}

	// Read column arrays
	if !ReadOffsetArray(io, &a, count, length, baseOffset, &signedSizeOfTag) {
		goto Error
	}

	// Read each dictionary entry
	for i := uint32(0); i < count; i++ {
		nameWCS, b1 := ReadOneWChar(io, &a.Name, i)
		valueWCS, b2 := ReadOneWChar(io, &a.Value, i)
		if !b1 || !b2 {
			goto Error
		}

		if length > 16 && !ReadOneMLUC(self, io, &a.DisplayName, i, &displayNameMLU) {
			goto Error
		}
		if length > 24 && !ReadOneMLUC(self, io, &a.DisplayValue, i, &displayValueMLU) {
			goto Error
		}

		if nameWCS == "" || valueWCS == "" {
			cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_CORRUPTION_DETECTED, "Bad dictionary Name/Value")
			rc = false
		} else {
			rc = cmsDictAddEntry(hDict, nameWCS, valueWCS, displayNameMLU, displayValueMLU)
		}

		if displayNameMLU != nil {
			cmsMLUfree(displayNameMLU)
		}
		if displayValueMLU != nil {
			cmsMLUfree(displayValueMLU)
		}

		if !rc {
			goto Error
		}
	}

	FreeArray(&a)
	*nItems = 1
	return unsafe.Pointer(hDict)

Error:
	FreeArray(&a)
	if hDict != nil {
		cmsDictFree(hDict)
	}
	return nil
}

func TypeDictionaryWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	var (
		hDict             cmsHANDLE
		count, length     uint32
		directoryPos      uint32
		currentPos        uint32
		baseOffset        uint32
		anyName, anyValue bool
		p                 *cmsDICTentry
		a                 cmsDICarray
	)

	hDict = cmsHANDLE(ptr)
	if hDict == nil {
		return false
	}

	baseOffset = uint32(io.Tell((*cms_io_handler)(io))) - uint32(unsafe.Sizeof(cmsTagBase{}))

	// Analyze the dictionary
	count = 0
	for p = cmsDictGetEntryList(hDict); p != nil; p = cmsDictNextEntry(p) {
		if p.DisplayName != nil {
			anyName = true
		}
		if p.DisplayValue != nil {
			anyValue = true
		}
		count++
	}

	length = 16
	if anyName {
		length += 8
	}
	if anyValue {
		length += 8
	}

	if !cmsWriteUInt32Number(io, count) || !cmsWriteUInt32Number(io, length) {
		return false
	}

	// Allocate and write offsets
	directoryPos = uint32(io.Tell((*cms_io_handler)(io)))
	if !AllocArray(self.ContextID, &a, count, length) || !WriteOffsetArray(io, &a, count, length) {
		goto Error
	}

	// Write each dictionary entry
	p = cmsDictGetEntryList(hDict)
	for i := uint32(0); i < count; i++ {
		if !WriteOneWChar(io, &a.Name, i, p.Name, baseOffset) || !WriteOneWChar(io, &a.Value, i, p.Value, baseOffset) {
			goto Error
		}

		if p.DisplayName != nil && !WriteOneMLUC(self, io, &a.DisplayName, i, p.DisplayName, baseOffset) {
			goto Error
		}
		if p.DisplayValue != nil && !WriteOneMLUC(self, io, &a.DisplayValue, i, p.DisplayValue, baseOffset) {
			goto Error
		}

		p = cmsDictNextEntry(p)
	}

	// Update directory offsets
	currentPos = uint32(io.Tell((*cms_io_handler)(io)))
	if !io.Seek((*cms_io_handler)(io), directoryPos) || !WriteOffsetArray(io, &a, count, length) || !io.Seek((*cms_io_handler)(io), currentPos) {
		goto Error
	}

	FreeArray(&a)
	return true

Error:
	FreeArray(&a)
	return false
}

func TypeDictionaryDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, nItems uint32) unsafe.Pointer {
	return unsafe.Pointer(cmsDictDup(cmsHANDLE(ptr)))
}

func TypeDictionaryFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsDictFree(cmsHANDLE(ptr))
}

// Read a video signal tag
func TypeVideoSignalRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	if sizeOfTag != 8 {
		return nil
	}

	// Skip unused uint32
	if !cmsReadUInt32Number(io, nil) {
		return nil
	}

	// Allocate memory for cmsVideoSignalType
	cicp := (*cmsVideoSignalType)(cmsCalloc(self.ContextID, 1, uint32(unsafe.Sizeof(cmsVideoSignalType{}))))
	if cicp == nil {
		return nil
	}

	// Read fields
	if !cmsReadUInt8Number(io, &cicp.ColourPrimaries) ||
		!cmsReadUInt8Number(io, &cicp.TransferCharacteristics) ||
		!cmsReadUInt8Number(io, &cicp.MatrixCoefficients) ||
		!cmsReadUInt8Number(io, &cicp.VideoFullRangeFlag) {
		goto Error
	}

	// Success
	*nItems = 1
	return unsafe.Pointer(cicp)

Error:
	if cicp != nil {
		cmsFree(self.ContextID, unsafe.Pointer(cicp))
	}
	return nil
}

// Write a video signal tag
func TypeVideoSignalWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	cicp := (*cmsVideoSignalType)(ptr)

	// Write a placeholder uint32
	if !cmsWriteUInt32Number(io, 0) {
		return false
	}

	// Write fields
	if !cmsWriteUInt8Number(io, cicp.ColourPrimaries) ||
		!cmsWriteUInt8Number(io, cicp.TransferCharacteristics) ||
		!cmsWriteUInt8Number(io, cicp.MatrixCoefficients) ||
		!cmsWriteUInt8Number(io, cicp.VideoFullRangeFlag) {
		return false
	}

	return true
}

// Duplicate a video signal tag
func TypeVideoSignalDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return cmsDupMem(self.ContextID, ptr, uint32(unsafe.Sizeof(cmsVideoSignalType{})))
}

// Free a video signal tag
func TypeVideoSignalFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsFree(self.ContextID, ptr)
}

const (
	cmsVideoCardGammaTableType   = 0
	cmsVideoCardGammaFormulaType = 1
)

// Used internally
type cmsVCGTGAMMA struct {
	Gamma float64
	Min   float64
	Max   float64
}

func TypeVcgtRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var tagType uint32
	if !cmsReadUInt32Number(io, &tagType) {
		return nil
	}

	var curves [3]*CmsToneCurve
	switch tagType {
	case cmsVideoCardGammaTableType:
		var nChannels, nElems, nBytes uint16

		if !cmsReadUInt16Number(io, &nChannels) || nChannels != 3 {
			cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unsupported number of channels for VCGT")
			goto Error
		}

		if !cmsReadUInt16Number(io, &nElems) || !cmsReadUInt16Number(io, &nBytes) {
			goto Error
		}

		for i := 0; i < 3; i++ {
			curves[i] = cmsBuildTabulatedToneCurve16(self.ContextID, uint32(nElems), nil)

			// Convert Table16 to a slice
			tableSlice := unsafe.Slice(curves[i].Table16, nElems)

			switch nBytes {
			case 1:
				var v uint8
				for j := 0; j < int(nElems); j++ {
					if !cmsReadUInt8Number(io, &v) {
						goto Error
					}
					tableSlice[j] = uint16(v) * 257
				}
			case 2:
				if !cmsReadUInt16Array(io, uint32(nElems), &tableSlice[0]) {
					goto Error
				}
			default:
				cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unsupported bit depth for VCGT")
				goto Error
			}
		}
	case cmsVideoCardGammaFormulaType:
		for i := 0; i < 3; i++ {
			var gamma, min, max float64
			if !cmsRead15Fixed16Number(io, &gamma) || !cmsRead15Fixed16Number(io, &min) || !cmsRead15Fixed16Number(io, &max) {
				goto Error
			}

			params := []float64{
				gamma,
				math.Pow(max-min, 1.0/gamma),
				0, 0, 0, min, 0,
			}
			curves[i] = cmsBuildParametricToneCurve(self.ContextID, 5, &params[0])
		}
	default:
		cmsSignalError(unsafe.Pointer(self.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unsupported tag type for VCGT")
		goto Error
	}

	*nItems = 1
	return unsafe.Pointer(&curves)

Error:
	cmsFreeToneCurveTriple(curves)
	return nil
}
func TypeVcgtWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	curves := *(*[]*CmsToneCurve)(ptr)

	if len(curves) != 3 {
		return false
	}

	// Handle the parametric tone curve case
	if cmsGetToneCurveParametricType(curves[0]) == 5 &&
		cmsGetToneCurveParametricType(curves[1]) == 5 &&
		cmsGetToneCurveParametricType(curves[2]) == 5 {
		if !cmsWriteUInt32Number(io, cmsVideoCardGammaFormulaType) {
			return false
		}

		for i := 0; i < 3; i++ {
			segments := curves[i].Segments // Dereference the Segments pointer
			if segments == nil {
				return false // Handle cases where Segments is nil
			}

			gamma := segments.Params[0]
			min := segments.Params[5]
			max := math.Pow(segments.Params[1], gamma) + min

			if !cmsWrite15Fixed16Number(io, gamma) ||
				!cmsWrite15Fixed16Number(io, min) ||
				!cmsWrite15Fixed16Number(io, max) {
				return false
			}
		}
	} else {
		// Handle the case of storing a table of 256 words
		if !cmsWriteUInt32Number(io, cmsVideoCardGammaTableType) ||
			!cmsWriteUInt16Number(io, 3) || // 3 channels
			!cmsWriteUInt16Number(io, 256) || // 256 entries per channel
			!cmsWriteUInt16Number(io, 2) { // 2 bytes per entry
			return false
		}

		for i := 0; i < 3; i++ {
			for j := 0; j < 256; j++ {
				value := cmsEvalToneCurveFloat(curves[i], float32(j)/255.0)
				saturated := cmsQuickSaturateWord(float64(value * 65535.0))

				if !cmsWriteUInt16Number(io, saturated) {
					return false
				}
			}
		}
	}
	return true
}

func TypeVcgtDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	oldCurves := *(*[]*CmsToneCurve)(ptr)
	NewCurves := (*[3]*CmsToneCurve)(cmsCalloc(self.ContextID, 3, uint32(unsafe.Sizeof(uintptr(0)))))
	if NewCurves == nil {
		return nil
	}

	NewCurves[0] = cmsDupToneCurve(oldCurves[0])
	NewCurves[1] = cmsDupToneCurve(oldCurves[1])
	NewCurves[2] = cmsDupToneCurve(oldCurves[2])
	return unsafe.Pointer(NewCurves)
}

func TypeVcgtFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	curves := *(*[3]*CmsToneCurve)(ptr)

	if len(curves) != 3 {
		return
	}

	cmsFreeToneCurveTriple(curves)
}

// ********************************************************************************
// Type cmsSigMultiProcessElementType
// ********************************************************************************

func GenericMPEDup(self *cmsTagTypeHandler, ptr unsafe.Pointer, n uint32) unsafe.Pointer {
	return (unsafe.Pointer)(cmsStageDup((*cmsStage)(ptr)))

}

func GenericMPEFree(self *cmsTagTypeHandler, ptr unsafe.Pointer) {
	cmsStageFree((*cmsStage)(ptr))
}

// Each curve is stored in one or more curve segments, with break-points specified between curve segments.
// The first curve segment always starts at -Infinity, and the last curve segment always ends at +Infinity. The
// first and last curve segments shall be specified in terms of a formula, whereas the other segments shall be
// specified either in terms of a formula, or by a sampled curve.

// ReadSegmentedCurve reads an embedded segmented curve
func ReadSegmentedCurve(self *cmsTagTypeHandler, io *cmsIOHANDLER) *CmsToneCurve {
	var elementSig cmsCurveSegSignature
	var nSegments uint16
	var prevBreak float32 = float32(math.Inf(-1)) // -Infinity

	// Read element signature
	if !cmsReadUInt32Number(io, (*uint32)(unsafe.Pointer(&elementSig))) {
		return nil
	}

	// Ensure it's a segmented curve
	if elementSig != cmsSigSegmentedCurve {
		return nil
	}

	// Read the rest of the header
	if !cmsReadUInt32Number(io, nil) || !cmsReadUInt16Number(io, &nSegments) || !cmsReadUInt16Number(io, nil) {
		return nil
	}

	if nSegments < 1 {
		return nil
	}

	segments := make([]cmsCurveSegment, nSegments)

	// Read breakpoints
	for i := uint32(0); i < uint32(nSegments-1); i++ {
		segments[i].X0 = prevBreak
		if !cmsReadFloat32Number(io, &segments[i].X1) {
			return nil
		}
		prevBreak = segments[i].X1

	}
	segments[nSegments-1].X0 = prevBreak
	segments[nSegments-1].X1 = float32(math.Inf(1)) // +Infinity

	// Read each segment
	for i := uint32(0); i < uint32(nSegments); i++ {
		if !cmsReadUInt32Number(io, (*uint32)(unsafe.Pointer(&elementSig))) || !cmsReadUInt32Number(io, nil) {
			return nil
		}

		switch elementSig {
		case cmsSigFormulaCurveSeg:
			var curveType uint16
			paramsByType := []uint32{4, 5, 5}

			if !cmsReadUInt16Number(io, &curveType) || !cmsReadUInt16Number(io, nil) {
				return nil
			}

			segments[i].Type = int32(curveType) + 6
			if curveType > 2 {
				return nil
			}

			for j := uint32(0); j < paramsByType[curveType]; j++ {
				var param float32
				if !cmsReadFloat32Number(io, &param) {
					return nil
				}
				segments[i].Params[j] = float64(param)
			}

		case cmsSigSampledCurveSeg:
			var count uint32
			if !cmsReadUInt32Number(io, &count) {
				return nil
			}

			count++
			segments[i].NGridPoints = count
			segments[i].SampledPoints = (*float32)(cmsCalloc(self.ContextID, count, uint32(unsafe.Sizeof(float32(0)))))

			// Create a slice view of the allocated memory
			sampledPointsSlice := unsafe.Slice(segments[i].SampledPoints, count)

			// Initialize the first point
			sampledPointsSlice[0] = 0

			// Populate the array using the slice
			for j := uint32(1); j < count; j++ {
				if !cmsReadFloat32Number(io, &sampledPointsSlice[j]) {
					return nil
				}
			}

		default:
			return nil
		}
	}

	curve := cmsBuildSegmentedToneCurve(self.ContextID, uint32(nSegments), &segments[0])

	// Fix implicit points
	/*for i := uint32(0); i < uint32(nSegments); i++ {
		if curve.Segments[i].Type == 0 {
			curve.Segments[i].SampledPoints[0] = cmsEvalToneCurveFloat(curve, curve.Segments[i].X0)
		}
	}*/

	// Create a slice view of the curve segments
	curveSegmentsSlice := unsafe.Slice(curve.Segments, nSegments)

	// Fix implicit points
	for i := uint32(0); i < uint32(nSegments); i++ {
		if curveSegmentsSlice[i].Type == 0 {
			// Create a slice view for SampledPoints
			sampledPointsSlice := unsafe.Slice(curveSegmentsSlice[i].SampledPoints, curveSegmentsSlice[i].NGridPoints)
			sampledPointsSlice[0] = cmsEvalToneCurveFloat(curve, curveSegmentsSlice[i].X0)
		}
	}

	return curve
}

// ReadMPECurve reads a single curve for MPE
func ReadMPECurve(self *cmsTagTypeHandler, io *cmsIOHANDLER, cargo unsafe.Pointer, n, sizeOfTag uint32) bool {
	gammaTables := (*[cmsMAXCHANNELS]*CmsToneCurve)(cargo)
	gammaTables[n] = ReadSegmentedCurve(self, io)
	return gammaTables[n] != nil
}

// Type_MPEcurve_Read reads MPE curve type
func TypeMPEcurveRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var inputChans, outputChans uint16
	baseOffset := io.Tell((*cms_io_handler)(io)) - uint32(unsafe.Sizeof(cmsTagBase{}))

	if !cmsReadUInt16Number(io, &inputChans) || !cmsReadUInt16Number(io, &outputChans) {
		return nil
	}

	if inputChans != outputChans {
		return nil
	}

	gammaTables := (**CmsToneCurve)(cmsCalloc(self.ContextID, uint32(inputChans), uint32(unsafe.Sizeof((*CmsToneCurve)(nil)))))
	if gammaTables == nil {
		return nil
	}

	var mpe *cmsStage
	// Read position table and allocate the MPE curve stage
	if ReadPositionTable(self, io, uint32(inputChans), baseOffset, unsafe.Pointer(gammaTables), ReadMPECurve) {
		mpe = cmsStageAllocToneCurves(self.ContextID, uint32(inputChans), gammaTables)

	}

	// Free allocated resources in case of error
	for i := uint32(0); i < uint32(inputChans); i++ {
		curve := (**CmsToneCurve)(unsafe.Add(unsafe.Pointer(gammaTables), uintptr(i)*unsafe.Sizeof((*CmsToneCurve)(nil))))
		if *curve != nil {
			CmsFreeToneCurve(*curve)
		}
	}
	if mpe != nil {
		*nItems = 1
	} else {
		*nItems = 0
	}
	return unsafe.Pointer(mpe)
}

// WriteSegmentedCurve writes a single segmented curve
func WriteSegmentedCurve(io *cmsIOHANDLER, curve *CmsToneCurve) bool {
	nSegments := curve.nSegments
	segments := curve.Segments

	if !cmsWriteUInt32Number(io, uint32(cmsSigSegmentedCurve)) || !cmsWriteUInt32Number(io, 0) ||
		!cmsWriteUInt16Number(io, uint16(nSegments)) || !cmsWriteUInt16Number(io, 0) {
		return false
	}
	// Create a slice view of the segments
	segmentsSlice := unsafe.Slice(segments, nSegments)

	// Write breakpoints
	for i := uint32(0); i < nSegments-1; i++ {
		if !cmsWriteFloat32Number(io, segmentsSlice[i].X1) {
			return false
		}
	}

	// Write each segment
	for i := uint32(0); i < nSegments; i++ {
		actualSeg := segmentsSlice[i]
		switch actualSeg.Type {
		case 0: // Sampled curve
			if !cmsWriteUInt32Number(io, uint32(cmsSigSampledCurveSeg)) || !cmsWriteUInt32Number(io, 0) ||
				!cmsWriteUInt32Number(io, actualSeg.NGridPoints-1) {
				return false
			}
			sampledPointsSlice := unsafe.Slice(actualSeg.SampledPoints, actualSeg.NGridPoints)

			for j := uint32(1); j < actualSeg.NGridPoints; j++ {

				if !cmsWriteFloat32Number(io, sampledPointsSlice[j]) {
					return false
				}
			}
		default: // Formula-based curve
			paramsByType := []uint32{4, 5, 5}
			curveType := actualSeg.Type - 6
			if curveType < 0 || curveType > 2 {
				return false
			}
			if !cmsWriteUInt32Number(io, uint32(cmsSigFormulaCurveSeg)) || !cmsWriteUInt32Number(io, 0) ||
				!cmsWriteUInt16Number(io, uint16(curveType)) || !cmsWriteUInt16Number(io, 0) {
				return false
			}
			for j := uint32(0); j < paramsByType[curveType]; j++ {
				if !cmsWriteFloat32Number(io, float32(actualSeg.Params[j])) {
					return false
				}
			}
		}
	}

	return true
}

// WriteMPECurve writes a curve for MPE
func WriteMPECurve(self *cmsTagTypeHandler, io *cmsIOHANDLER, cargo unsafe.Pointer, n, sizeOfTag uint32) bool {
	curves := (*cmsStageToneCurvesData)(cargo)
	curve := (**CmsToneCurve)(unsafe.Add(unsafe.Pointer(curves.TheCurves), uintptr(n)*unsafe.Sizeof((*CmsToneCurve)(nil))))

	return WriteSegmentedCurve(io, *curve)
}

// Type_MPEcurve_Write writes the MPE curve type
func TypeMPEcurveWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	mpe := (*cmsStage)(ptr)
	curves := (*cmsStageToneCurvesData)(mpe.Data)
	baseOffset := io.Tell((*cms_io_handler)(io)) - uint32(unsafe.Sizeof(cmsTagBase{}))

	// Write header
	if !cmsWriteUInt16Number(io, uint16(mpe.InputChannels)) {
		return false
	}
	if !cmsWriteUInt16Number(io, uint16(mpe.InputChannels)) {
		return false
	}

	// Write position table
	return WritePositionTable(self, io, 0, uint32(mpe.InputChannels), baseOffset, unsafe.Pointer(curves), WriteMPECurve)
}
func TypeMPEmatrixRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var inputChans, outputChans uint16
	if !cmsReadUInt16Number(io, &inputChans) || !cmsReadUInt16Number(io, &outputChans) {
		return nil
	}

	if inputChans >= cmsMAXCHANNELS || outputChans >= cmsMAXCHANNELS {
		return nil
	}

	nElems := uint32(inputChans) * uint32(outputChans)
	matrix := (*float64)(cmsCalloc(self.ContextID, nElems, uint32(unsafe.Sizeof(float64(0)))))
	if matrix == nil {
		return nil
	}

	offsets := (*float64)(cmsCalloc(self.ContextID, uint32(outputChans), uint32(unsafe.Sizeof(float64(0)))))
	if offsets == nil {
		cmsFree(self.ContextID, unsafe.Pointer(matrix))
		return nil
	}
	for i := uint32(0); i < nElems; i++ {
		var v float32
		if !cmsReadFloat32Number(io, &v) {
			cmsFree(self.ContextID, unsafe.Pointer(matrix))
			cmsFree(self.ContextID, unsafe.Pointer(offsets))
			return nil
		}
		matrixPtr := (*float64)(unsafe.Add(unsafe.Pointer(matrix), uintptr(i)*unsafe.Sizeof(float64(0))))
		*matrixPtr = float64(v)
	}

	for i := uint32(0); i < uint32(outputChans); i++ {
		var v float32
		if !cmsReadFloat32Number(io, &v) {
			cmsFree(self.ContextID, unsafe.Pointer(matrix))
			cmsFree(self.ContextID, unsafe.Pointer(offsets))
			return nil
		}
		offsetsPtr := (*float64)(unsafe.Add(unsafe.Pointer(offsets), uintptr(i)*unsafe.Sizeof(float64(0))))
		*offsetsPtr = float64(v)
	}

	mpe := cmsStageAllocMatrix(self.ContextID, uint32(outputChans), uint32(inputChans), matrix, offsets)
	cmsFree(self.ContextID, unsafe.Pointer(matrix))
	cmsFree(self.ContextID, unsafe.Pointer(offsets))
	*nItems = 1
	return unsafe.Pointer(mpe)
}

func TypeMPEmatrixWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	mpe := (*cmsStage)(ptr)
	matrix := (*cmsStageMatrixData)(mpe.Data)

	if !cmsWriteUInt16Number(io, uint16(mpe.InputChannels)) || !cmsWriteUInt16Number(io, uint16(mpe.OutputChannels)) {
		return false
	}

	nElems := mpe.InputChannels * mpe.OutputChannels
	// Convert matrix.Double to a slice
	doubleSlice := unsafe.Slice(matrix.Double, nElems)

	for i := uint32(0); i < nElems; i++ {
		if !cmsWriteFloat32Number(io, float32(doubleSlice[i])) {
			return false
		}
	}

	for i := uint32(0); i < mpe.OutputChannels; i++ {
		if matrix.Offset == nil {
			if !cmsWriteFloat32Number(io, 0) {
				return false
			}
		} else {
			// Convert matrix.Offset to a slice
			offsetSlice := unsafe.Slice(matrix.Offset, mpe.OutputChannels)
			if !cmsWriteFloat32Number(io, float32(offsetSlice[i])) {
				return false
			}
		}
	}
	return true
}

func TypeMPEclutRead(self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) unsafe.Pointer {
	var inputChans, outputChans uint16
	var dimensions8 [16]byte
	var nMaxGrids uint32
	var gridPoints [MAX_INPUT_DIMENSIONS]uint32

	if !cmsReadUInt16Number(io, &inputChans) || !cmsReadUInt16Number(io, &outputChans) || inputChans == 0 || outputChans == 0 {
		return nil
	}

	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&dimensions8[0]), uint32(unsafe.Sizeof(uint8(0))), 16) != 16 {
		return nil
	}
	if inputChans > MAX_INPUT_DIMENSIONS {
		nMaxGrids = MAX_INPUT_DIMENSIONS
	} else {
		nMaxGrids = uint32(inputChans)
	}

	for i := 0; i < int(nMaxGrids); i++ {
		if dimensions8[i] == 1 {
			return nil
		}
		gridPoints[i] = uint32(dimensions8[i])
	}

	mpe := cmsStageAllocCLutFloatGranular(self.ContextID, gridPoints[:], uint32(inputChans), uint32(outputChans), nil)
	if mpe == nil {
		return nil
	}

	clut := (*cmsStageCLutData)(mpe.Data)
	clutSlice := unsafe.Slice(clut.Tab.TFloat, clut.NEntries)

	for i := uint32(0); i < clut.NEntries; i++ {
		if !cmsReadFloat32Number(io, &clutSlice[i]) {
			cmsStageFree(mpe)
			return nil
		}
	}

	*nItems = 1
	return unsafe.Pointer(mpe)
}

func TypeMPEclutWrite(self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr unsafe.Pointer, nItems uint32) bool {
	mpe := (*cmsStage)(ptr)
	clut := (*cmsStageCLutData)(mpe.Data)

	if mpe.InputChannels > MAX_INPUT_DIMENSIONS || !clut.HasFloatValues {
		return false
	}

	if !cmsWriteUInt16Number(io, uint16(mpe.InputChannels)) || !cmsWriteUInt16Number(io, uint16(mpe.OutputChannels)) {
		return false
	}

	var dimensions8 [16]uint8
	for i := uint32(0); i < mpe.InputChannels; i++ {
		dimensions8[i] = uint8(clut.Params.nSamples[i])
	}

	if io.Write((*cms_io_handler)(io), 16, unsafe.Pointer(&dimensions8[0])) {
		return false
	}

	clutSlice := unsafe.Slice(clut.Tab.TFloat, clut.NEntries)

	for i := uint32(0); i < clut.NEntries; i++ {
		if !cmsWriteFloat32Number(io, clutSlice[i]) {
			return false
		}
	}

	return true
}

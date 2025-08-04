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
	"encoding/binary"
	"fmt"
	"math"

	//"syscall"
	"arena"
	"time"
	"unsafe"
)

type wchar uint16
type cmsTagTypeHandler struct {
	Signature  cmsTagTypeSignature
	ReadFn     func(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{}
	WriteFn    func(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool
	DupFn      func(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{}
	FreeFn     func(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{})
	ContextID  CmsContext
	ICCVersion uint32
}

// cmsTagTypeLinkedList represents a linked list of tag type handlers.
type cmsTagTypeLinkedList struct {
	Handler cmsTagTypeHandler
	Next    *cmsTagTypeLinkedList
}

func cmsWriteWCharArray(io *cmsIOHANDLER, n uint32, Array []uint16) bool {
	// Assert conditions
	if io == nil || (Array == nil && n > 0) {
		return false
	}

	for i := uint32(0); i < n; i++ {
		if !cmsWriteUInt16Number(io, uint16(Array[i])) {
			return false
		}
	}

	return true
}
func cmsReadWCharArray(io *cmsIOHANDLER, n uint32, array []uint16) bool {
	if io == nil {
		return false // Equivalent to _cmsAssert(io != NULL)
	}

	//is32 := unsafe.Sizeof(rune(0)) > unsafe.Sizeof(uint16(0))

	// If we need UTF-32, delegate to a conversion function
	//THIS IS NOT MADE YET,uint16 for now
	/*if is32 && array != nil {
	    return convertUTF16ToUTF32(io, n, array)
	}*/

	var tmp uint16

	for i := uint32(0); i < n; i++ {
		if array != nil {
			if !cmsReadUInt16Number(io, &tmp) {
				return false
			}
			array[i] = tmp // Convert uint16 to rune (wchar_t)
		} else {
			if !cmsReadUInt16Number(io, nil) {
				return false
			}
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
func RegisterTypesPlugin(ar *arena.Arena, id CmsContext, Data PluginIntrfc, pos cmsMemoryClient) bool {
	ctx := CmsContextGetClientChunk(id, pos).(*cmsTagTypePluginChunkType)

	// If Data is nil, unregister the plug-in.
	if Data == nil {
		// No need to free memory; pool is destroyed as a whole.
		ctx.TagTypes = nil
		return true
	}
	plugin, ok := Data.(*cmsPluginTagType)
	if !ok {
		fmt.Printf("Error: Plugin is not of the type cmsPluginTagType\n")
		return false
	}
	// Allocate memory for the new linked list node.
	//pt := (*cmsTagTypeLinkedList)(cmsPluginMalloc(id, uint32(unsafe.Sizeof(cmsTagTypeLinkedList{}))))
	pt := allocateStruct[cmsTagTypeLinkedList](ar)

	if pt == nil {
		return false
	}

	// Assign handler and link to the current list.
	pt.Handler = plugin.Handler
	pt.Next = ctx.TagTypes

	// Update the context's tag types to point to the new node.
	ctx.TagTypes = pt

	return true
}

// To deal with position tables
type PositionTableEntryFn func(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, cargo interface{}, n, SizeOfTag uint32) bool

func ReadPositionTable(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, count, baseOffset uint32, cargo interface{}, elementFn PositionTableEntryFn) bool {
	var currentPosition uint32
	currentPosition = uint32(io.Tell((*cms_io_handler)(io)))
	var elementOffsets, elementSizes []uint32
	// Verify there is enough space left to read at least two uint32 items for count items
	if ((io.ReportedSize - currentPosition) / (2 * uint32(unsafe.Sizeof(uint32(0))))) < count {
		return false
	}

	// Allocate memory for offsets and sizes
	elementOffsets = make([]uint32, count)
	elementSizes = make([]uint32, count)

	// Read the offsets and sizes
	for i := uint32(0); i < count; i++ {
		if !cmsReadUInt32Number(io, &elementOffsets[i]) || !cmsReadUInt32Number(io, &elementSizes[i]) {
			return false
		}
		elementOffsets[i] += baseOffset
	}

	// Seek to each element and read it
	for i := uint32(0); i < count; i++ {
		if !io.Seek((*cms_io_handler)(io), uint32(elementOffsets[i])) {
			return false
		}
		// Call the reader callback
		if !elementFn(ar, self, io, cargo, i, elementSizes[i]) {
			return false
		}
	}
	return true
}

func WritePositionTable(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, sizeOfTag, count, baseOffset uint32, cargo interface{}, elementFn PositionTableEntryFn) bool {
	// Allocate memory for offsets and sizes
	var currentPos uint32
	var directoryPos uint32

	// Allocate memory for offsets and sizes
	elementOffsets := make([]uint32, count)
	elementSizes := make([]uint32, count)

	// Keep starting position of curve offsets
	directoryPos = uint32(io.Tell((*cms_io_handler)(io)))

	// Write a fake directory to be filled later
	for i := uint32(0); i < count; i++ {
		if !cmsWriteUInt32Number(io, 0) || !cmsWriteUInt32Number(io, 0) {
			return false
		}
	}

	// Write each element and keep track of size
	for i := uint32(0); i < count; i++ {
		before := uint32(io.Tell((*cms_io_handler)(io)))
		elementOffsets[i] = before - baseOffset

		// Callback to write
		if !elementFn(ar, self, io, cargo, i, sizeOfTag) {
			return false
		}

		// Calculate the size
		elementSizes[i] = uint32(io.Tell((*cms_io_handler)(io))) - before
	}

	// Write the directory
	currentPos = uint32(io.Tell((*cms_io_handler)(io)))
	if !io.Seek((*cms_io_handler)(io), directoryPos) {
		return false
	}

	for i := uint32(0); i < count; i++ {
		if !cmsWriteUInt32Number(io, elementOffsets[i]) || !cmsWriteUInt32Number(io, elementSizes[i]) {
			return false
		}
	}

	if !io.Seek((*cms_io_handler)(io), currentPos) {
		return false
	}
	return true

}

// Type_XYZ_Read reads XYZ color space data.
func TypeXYZRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, SizeOfTag uint32) interface{} {
	var xyz *cmsCIEXYZ

	*nItems = 0
	xyz = allocateStruct[cmsCIEXYZ](ar)
	if xyz == nil {
		return nil
	}

	if !cmsReadXYZNumber(io, xyz) {
		cmsFree(self.ContextID, xyz)
		return nil
	}

	*nItems = 1
	return xyz
}

// Type_XYZ_Write writes XYZ color space data.
func TypeXYZWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	return cmsWriteXYZNumber(io, ptr.(*cmsCIEXYZ))
}

// Type_XYZ_Dup duplicates XYZ color space data.
func TypeXYZDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	pxyz, ok := ptr.(*cmsCIEXYZ)
	xyz := *pxyz //copy values
	if !ok {
		fmt.Printf("Error: not of the type *cmsCIEXYZ\n")
		return false
	}
	return &xyz
}

// Type_XYZ_Free frees XYZ color space data.
func TypeXYZFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsFree(self.ContextID, ptr)
}

// DecideXYZtype decides the type of XYZ tag.
func DecideXYZtype(ICCVersion float64, Data interface{}) cmsTagTypeSignature {
	return cmsSigXYZType
}

// ********************************************************************************
// Type cmsSigLut8Type
// ********************************************************************************

// DecideLUTtypeA2B decides which LUT type to use when writing A2B LUTs.
func DecideLUTtypeA2B(ICCVersion float64, Data interface{}) cmsTagTypeSignature {
	Lut, ok := Data.(*cmsPipeline)
	if !ok {
		fmt.Printf("Error: not of the type *cmsPipeline\n")
		return 0
	}
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
func DecideLUTtypeB2A(ICCVersion float64, Data interface{}) cmsTagTypeSignature {
	Lut, ok := Data.(*cmsPipeline)
	if !ok {
		fmt.Printf("Error: not of the type *cmsPipeline\n")
		return 0
	}
	if ICCVersion < 4.0 {
		if Lut.SaveAs8Bits {
			return cmsSigLut8Type
		}
		return cmsSigLut16Type
	} else {
		return cmsSigLutBtoAType
	}
}

// DecideCurveType decides which curve type to use when writing.
func DecideCurveType(ICCVersion float64, Data interface{}) cmsTagTypeSignature {
	Curve, ok := Data.(*CmsToneCurve)
	if !ok {
		fmt.Printf("Error: not of the type *cmsToneCurve\n")
		return 0
	}
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
func TypeParametricCurveRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, SizeOfTag uint32) interface{} {
	paramsByType := []int{1, 3, 4, 5, 7}
	var params [10]float64
	var curveType uint16
	var newGamma *CmsToneCurve

	if !cmsReadUInt16Number(io, &curveType) {
		cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unknown parametric curve type '%d'")
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

	newGamma = cmsBuildParametricToneCurve(ar, self.ContextID, int(curveType+1), params[:])
	*nItems = 1
	return newGamma
}

// TypeParametricCurveWrite writes a parametric curve to the IO handler.
func TypeParametricCurveWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	curve, ok := ptr.(*CmsToneCurve)
	if !ok {
		fmt.Printf("Error: not of the type *cmsToneCurve\n")
		return false
	}
	paramsByType := []int{0, 1, 3, 4, 5, 7}

	typen := curve.Segments[0].Type

	if curve.nSegments > 1 || typen < 1 {
		cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Multisegment or Inverted parametric curves cannot be written")
		return false
	}

	if typen > 5 {
		cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unsupported parametric curve")
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
func TypeParametricCurveDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	return cmsDupToneCurve(ar, ptr.(*CmsToneCurve))
}

// TypeParametricCurveFree frees a parametric curve.
func TypeParametricCurveFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	CmsFreeToneCurve(ptr.(*CmsToneCurve))
}

// Type_Text_Read reads a text type structure from the io handler.
func TypeTextRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var text []byte

	// Create a container
	mlu := cmsMLUalloc(ar, self.ContextID, 1)
	if mlu == nil {
		return nil
	}

	*nItems = 0

	// Ensure valid size
	if sizeOfTag == 0xFFFFFFFF {
		goto Error
	}

	// Allocate memory for the text, with space for null terminator
	text = make([]byte, sizeOfTag+1)
	if text == nil {
		goto Error
	}

	// Read text from the IO handler
	if io.Read((*cms_io_handler)(io), text, uint32(unsafe.Sizeof(uint8(0))), sizeOfTag) != sizeOfTag {
		goto Error
	}

	// Ensure null termination
	text[sizeOfTag] = 0
	*nItems = 1

	// Store the result in the MLU
	if !cmsMLUsetASCII(mlu, cmsNoLanguage, cmsNoCountry, string(text)) {
		goto Error
	}
	return mlu

Error:
	if mlu != nil {
		cmsMLUfree(mlu)
	}
	return nil
}

// Type_Text_Write writes a text type structure to the io handler.
func TypeTextWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	mlu, ok := ptr.(*cmsMLU)
	if !ok {
		fmt.Printf("Error: not of the type *cmsMLU\n")
		return false
	}
	var size uint32
	var rc bool
	var text []byte

	// Get the size of the ASCII representation, including null terminator
	size = cmsMLUgetASCII(mlu, cmsNoLanguage, cmsNoCountry, nil, 0)
	if size == 0 {
		return false
	}

	// Allocate memory for the text
	text = make([]byte, size)
	if text == nil {
		return false
	}

	// Retrieve the ASCII text
	cmsMLUgetASCII(mlu, cmsNoLanguage, cmsNoCountry, text, size)

	// Write the text to the IO handler
	rc = io.Write((*cms_io_handler)(io), size, text)
	return rc
}

// Type_Text_Dup duplicates a text type structure.
func TypeTextDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	return cmsMLUdup(ar, ptr.(*cmsMLU))
}

// Type_Text_Free frees a text type structure.
func TypeTextFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsMLUfree(ptr.(*cmsMLU))
}

// DecideTextType determines the text type signature based on ICC version.
func DecideTextType(iccVersion float64, Data interface{}) cmsTagTypeSignature {
	if iccVersion >= 4.0 {
		return cmsSigMultiLocalizedUnicodeType
	}
	return cmsSigTextType
}

// ********************************************************************************
// Type cmsSigTextDescriptionType
// ********************************************************************************

// DecideTextDescType determines the type of text description
func DecideTextDescType(ICCVersion float64, Data interface{}) cmsTagTypeSignature {
	if ICCVersion >= 4.0 {
		return cmsSigMultiLocalizedUnicodeType
	}
	return cmsSigTextDescriptionType
}
func TypeTextDescriptionRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var (
		text            []byte
		mlu             *cmsMLU
		asciiCount      uint32
		unicodeCode     uint32
		unicodeCount    uint32
		scriptCodeCode  uint16
		scriptCodeCount uint8
		i               uint32
	)

	*nItems = 0

	//  Check if size of tag is at least one DWORD
	if sizeOfTag < 4 {
		return nil
	}

	//  Read ASCII count
	if !cmsReadUInt32Number(io, &asciiCount) {
		return nil
	}
	sizeOfTag -= 4

	//  Check if tag size is sufficient
	if sizeOfTag < asciiCount {
		return nil
	}

	//  Allocate the MLU
	mlu = cmsMLUalloc(ar, self.ContextID, 1)
	if mlu == nil {
		return nil
	}

	//  Allocate memory for the text slice
	text = make([]byte, asciiCount+1) //  +1 to null-terminate
	if len(text) == 0 {
		goto Error
	}

	//  Read ASCII text into slice
	if io.Read((*cms_io_handler)(io), text, 1, asciiCount) != asciiCount {
		goto Error
	}
	sizeOfTag -= asciiCount

	//  Ensure null-terminated string (Go slices are already zero-initialized)
	text[asciiCount] = 0

	//  Set the MLU entry using the byte slice
	if !cmsMLUsetASCII(mlu, cmsNoLanguage, cmsNoCountry, string(text)) {
		goto Error
	}
	text = nil // Text no longer needed

	//  Skip Unicode code
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

	//  Read and skip Unicode characters
	for i = 0; i < unicodeCount; i++ {
		var tmp [2]byte
		if io.Read((*cms_io_handler)(io), tmp[:], 2, 1) != 1 {
			goto Done
		}
	}
	sizeOfTag -= unicodeCount * 2

	//  Skip ScriptCode code if present
	if sizeOfTag >= uint32(unsafe.Sizeof(uint16(0)))+uint32(unsafe.Sizeof(uint8(0)))+67 {
		if !cmsReadUInt16Number(io, &scriptCodeCode) || !cmsReadUInt8Number(io, &scriptCodeCount) {
			goto Done
		}

		var skipBuffer [67]byte
		if io.Read((*cms_io_handler)(io), skipBuffer[:], 67, 1) != 1 {
			goto Error
		}
	}

Done:
	*nItems = 1
	return mlu

Error:
	if len(text) > 0 {
		text = nil // Let Go's GC handle cleanup
	}
	if mlu != nil {
		cmsMLUfree(mlu)
	}
	return nil
}

// This tag can come IN UNALIGNED SIZE. In order to prevent issues, we force zeros on description to align it
// Type_Text_Description_Write writes a text description tag
func TypeTextDescriptionWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	mlu, ok := ptr.(*cmsMLU)
	if !ok {
		fmt.Printf("Error: not of the type *cmsMLU\n")
		return false
	}
	var Text []byte
	var Wide []uint16
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
		// Allocate single null-terminated elements
		Text = []byte{0}   // Equivalent to calloc(1, sizeof(byte))
		Wide = []uint16{0} // Equivalent to calloc(1, sizeof(uint16))
	} else {
		// Allocate slices instead of manually allocating memory
		Text = make([]byte, lenASCII)   // Allocates a slice of lenASCII bytes
		Wide = make([]uint16, lenASCII) // Allocates a slice of lenASCII uint16s

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
	if !io.Write((*cms_io_handler)(io), lenText, Text) {
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

	if !io.Write((*cms_io_handler)(io), 67, Filler[:]) {
		goto Error
	}

	// Possibly add padding at the end of the tag
	if lenAligned > lenTagRequirement {
		if !io.Write((*cms_io_handler)(io), lenAligned-lenTagRequirement, Filler[:]) {
			goto Error
		}
	}

	rc = true

Error:
	if Text != nil {
		cmsFree(self.ContextID, &Text)
	}
	if Wide != nil {
		cmsFree(self.ContextID, &Wide)
	}

	return rc
}

// Type_Text_Description_Dup duplicates a cmsMLU object
func TypeTextDescriptionDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, nItems uint32) interface{} {
	return cmsMLUdup(ar, ptr.(*cmsMLU))
}

// Type_Text_Description_Free frees a cmsMLU object
func TypeTextDescriptionFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsMLUfree(ptr.(*cmsMLU))
}

// Both kinds of plug-ins share the same structure
func cmsRegisterTagTypePlugin(ar *arena.Arena, id CmsContext, Data PluginIntrfc) bool {
	return RegisterTypesPlugin(ar, id, Data, TagTypePlugin)
}

func cmsRegisterMultiProcessElementPlugin(ar *arena.Arena, id CmsContext, Data PluginIntrfc) bool {
	return RegisterTypesPlugin(ar, id, Data, MPEPlugin)
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
	ctx := CmsContextGetClientChunk(ContextID, TagTypePlugin).(*cmsTagTypePluginChunkType)

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
func cmsRegisterTagPlugin(ar *arena.Arena, id CmsContext, Data PluginIntrfc) bool {
	TagPluginChunk := CmsContextGetClientChunk(id, TagPlugin).(*cmsTagPluginChunkType)

	// If Data is nil, unregister the plugin.
	if Data == nil {
		TagPluginChunk.Tag = nil
		return true
	}
	plugin, ok := Data.(*cmsPluginTag)
	if !ok {
		fmt.Printf("Error: Plugin is not of the type cmsPluginTagType\n")
		return false
	}
	// Allocate memory for the new linked list node.
	//pt := (*cmsTagLinkedList)(cmsPluginMalloc(id, uint32(unsafe.Sizeof(cmsTagLinkedList{}))))
	pt := allocateStruct[cmsTagLinkedList](ar)

	if pt == nil {
		return false
	}

	// Set the new node's values.
	pt.Signature = plugin.Signature
	pt.Descriptor = plugin.Descriptor
	pt.Next = TagPluginChunk.Tag

	// Update the head of the linked list.
	TagPluginChunk.Tag = pt

	return true
}

// cmsGetTagDescriptor returns a descriptor for a given tag or nil.
func cmsGetTagDescriptor(ContextID CmsContext, sig cmsTagSignature) *cmsTagDescriptor {
	// Retrieve the TagPluginChunk from the context.
	TagPluginChunk := CmsContextGetClientChunk(ContextID, TagPlugin).(*cmsTagPluginChunkType)

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
func TypeScreeningRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	sc := allocateStruct[cmsScreening](ar)
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
	return sc

Error:
	if sc != nil {
		cmsFree(self.ContextID, sc)
	}
	return nil
}

func TypeScreeningWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	sc, ok := ptr.(*cmsScreening)
	if !ok {
		fmt.Printf("Error: not of the type *cmsScreening\n")
		return false
	}
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

func TypeScreeningDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	str := *(ptr.(*cmsScreening))
	return &str
}

func TypeScreeningFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsFree(nil, ptr)
}
func TypeViewingConditionsRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	vc := allocateStruct[cmsICCViewingConditions](ar)
	*nItems = 0
	if vc == nil {
		return nil
	}

	if !cmsReadXYZNumber(io, &vc.IlluminantXYZ) || !cmsReadXYZNumber(io, &vc.SurroundXYZ) || !cmsReadUInt32Number(io, &vc.IlluminantType) {
		return nil
	}

	*nItems = 1
	return vc
}

func TypeViewingConditionsWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	vc, ok := ptr.(*cmsICCViewingConditions)
	if !ok {
		fmt.Printf("Error: not of the type *cmsICCViewingConditions\n")
		return false
	}
	return cmsWriteXYZNumber(io, &vc.IlluminantXYZ) &&
		cmsWriteXYZNumber(io, &vc.SurroundXYZ) &&
		cmsWriteUInt32Number(io, vc.IlluminantType)
}

func TypeViewingConditionsDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	str := *(ptr.(*cmsICCViewingConditions))
	return &str
}

func TypeViewingConditionsFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsFree(nil, ptr)
}

// Type_Chromaticity_Read reads a Chromaticity type from the IO handler.
func TypeChromaticityRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	*nItems = 0

	// Allocate memory for CmsCIExyYTRIPLE
	chrm := allocateStruct[CmsCIExyYTRIPLE](ar)
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
		!cmsRead15Fixed16Number(io, &chrm.Red.X_small) ||
		!cmsRead15Fixed16Number(io, &chrm.Red.Y_small) {
		goto Error
	}

	chrm.Red.Y_large = 1.0

	if !cmsRead15Fixed16Number(io, &chrm.Green.X_small) ||
		!cmsRead15Fixed16Number(io, &chrm.Green.Y_small) {
		goto Error
	}

	chrm.Green.Y_large = 1.0

	if !cmsRead15Fixed16Number(io, &chrm.Blue.X_small) ||
		!cmsRead15Fixed16Number(io, &chrm.Blue.Y_small) {
		goto Error
	}

	chrm.Blue.Y_large = 1.0

	*nItems = 1
	return chrm

Error:
	cmsFree(self.ContextID, chrm)
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
func TypeChromaticityWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	chrm, ok := ptr.(*CmsCIExyYTRIPLE)
	if !ok {
		fmt.Printf("Error: not of the type *CmsCIExyYTRIPLE\n")
		return false
	}
	if !cmsWriteUInt16Number(io, 3) || // nChannels
		!cmsWriteUInt16Number(io, 0) { // Table
		return false
	}

	if !SaveOneChromaticity(chrm.Red.X_small, chrm.Red.Y_small, io) ||
		!SaveOneChromaticity(chrm.Green.X_small, chrm.Green.Y_small, io) ||
		!SaveOneChromaticity(chrm.Blue.X_small, chrm.Blue.Y_small, io) {
		return false
	}

	return true
}

// Type_Chromaticity_Dup duplicates a Chromaticity structure.
func TypeChromaticityDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	str := *(ptr.(*CmsCIExyYTRIPLE))
	return &str
}

// Type_Chromaticity_Free frees the memory allocated for a Chromaticity structure.
func TypeChromaticityFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
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

func TypeColorantOrderTypeRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	// Allocate memory for ColorantOrder
	var colorantOrder = make([]uint8, cmsMAXCHANNELS)

	*nItems = 0

	var count uint32
	if !cmsReadUInt32Number(io, &count) || count > cmsMAXCHANNELS {
		return nil
	}
	// Set all elements to 0xFF as end marker
	MemsetSlice(colorantOrder[:], 0xFF, cmsMAXCHANNELS)
	if io.Read((*cms_io_handler)(io), colorantOrder, cmsMAXCHANNELS, count) != count {
		return nil
	}

	*nItems = 1
	return colorantOrder
}

func TypeColorantOrderTypeWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	colorantOrder, ok := ptr.([]uint8)
	if !ok {
		cmsSignalError(nil, cmsERROR_RANGE, "TypeColorantOrderTypeWrite: expected []uint8")
		return false
	}

	var count uint32
	for i := 0; i < cmsMAXCHANNELS && i < len(colorantOrder); i++ {
		if colorantOrder[i] != 0xFF {
			count++
		}
	}

	if !cmsWriteUInt32Number(io, count) {
		return false
	}

	if count == 0 {
		return true // nothing to write
	}

	sz := count * uint32(unsafe.Sizeof(uint8(0)))
	return io.Write((*cms_io_handler)(io), sz, colorantOrder)
}

func TypeColorantOrderTypeDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	original, ok := ptr.([]uint8)
	if !ok {
		return nil
	}
	dup := make([]uint8, len(original))
	copy(dup, original)
	return dup
}

func TypeColorantOrderTypeFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsFree(self.ContextID, ptr)
}

// ********************************************************************************
// Type cmsSigS15Fixed16ArrayType
// ********************************************************************************
// This type represents an array of generic 4-byte/32-bit fixed point quantity.
// The number of values is determined from the size of the tag.

func TypeS15Fixed16Read(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	*nItems = 0

	// Calculate number of elements
	n := sizeOfTag / uint32(unsafe.Sizeof(uint32(0)))

	// Allocate memory for the array
	arrayDouble := make([]float64, n)
	if arrayDouble == nil {
		return nil
	}

	// Read
	for i := uint32(0); i < n; i++ {
		if !cmsRead15Fixed16Number(io, &arrayDouble[i]) {
			return nil
		}
	}

	*nItems = n
	return &arrayDouble[0]
}

func TypeS15Fixed16Write(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	switch v := ptr.(type) {

	case []float64:
		for i := uint32(0); i < nItems && i < uint32(len(v)); i++ {
			if !cmsWrite15Fixed16Number(io, v[i]) {
				return false
			}
		}
		return true

	case *cmsVEC3:
		for i := 0; i < len(v.N); i++ {
			if !cmsWrite15Fixed16Number(io, v.N[i]) {
				return false
			}
		}
		return true

	case *cmsMAT3:
		for i := 0; i < len(v.V); i++ {
			for j := 0; j < len(v.V[i].N); j++ {
				if !cmsWrite15Fixed16Number(io, v.V[i].N[j]) {
					return false
				}
			}
		}
		return true

	default:
		cmsSignalError(nil, cmsERROR_RANGE, "TypeS15Fixed16Write: unsupported data type")
		return false
	}
}

func TypeS15Fixed16Dup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	switch v := ptr.(type) {
	case []float64:
		dup := make([]float64, len(v))
		copy(dup, v)
		return dup

	case *cmsMAT3:
		var dup cmsMAT3
		for i := 0; i < len(v.V); i++ {
			for j := 0; j < len(v.V[i].N); j++ {
				dup.V[i].N[j] = v.V[i].N[j]
			}
		}
		return &dup

	case *cmsVEC3:
		var dup cmsVEC3
		for i := 0; i < len(v.N); i++ {
			dup.N[i] = v.N[i]
		}
		return &dup

	default:
		fmt.Println("unsupported type in TypeS15Fixed16Dup")
		return nil
	}
}

func TypeS15Fixed16Free(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsFree(self.ContextID, ptr)
}

// ********************************************************************************
// Type cmsSigU16Fixed16ArrayType
// ********************************************************************************
// This type represents an array of generic 4-byte/32-bit quantity.
// The number of values is determined from the size of the tag.

func TypeU16Fixed16Read(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	n := sizeOfTag / uint32(unsafe.Sizeof(uint32(0)))
	// Allocate memory for the array
	arrayDouble := make([]float64, n)
	if arrayDouble == nil {
		return nil
	}

	for i := uint32(0); i < n; i++ {
		var v uint32
		if !cmsReadUInt32Number(io, &v) {
			return nil
		}

		// Convert to cmsFloat64Number
		arrayDouble[i] = float64(v) / 65536.0
	}

	*nItems = n
	return &arrayDouble[0]
}

func TypeU16Fixed16Write(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	values, ok := ptr.([]float64)
	if !ok {
		cmsSignalError(nil, cmsERROR_RANGE, "TypeU16Fixed16Write: expected []float64")
		return false
	}

	for i := uint32(0); i < nItems; i++ {
		v := uint32(values[i]*65536.0 + 0.5) // Convert back to fixed-point

		if !cmsWriteUInt32Number(io, v) {
			return false
		}
	}

	return true
}

func TypeU16Fixed16Dup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	original, ok := ptr.([]float64)
	if !ok {
		return nil
	}
	dup := make([]float64, n)
	copy(dup, original)
	return dup
}

func TypeU16Fixed16Free(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
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
func TypeSignatureRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	//sigPtr := (*cmsSignature)(cmsMalloc(self.ContextID, uint32(unsafe.Sizeof(cmsSignature(0)))))
	var sigPtr uint32
	if !cmsReadUInt32Number(io, &sigPtr) {
		cmsFree(self.ContextID, &sigPtr)
		return nil
	}

	*nItems = 1
	return sigPtr
}

// TypeSignatureWrite writes a cmsSignature to the io handler.
func TypeSignatureWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	sigPtr, ok := ptr.(*cmsSignature)
	if !ok {
		fmt.Printf("Error: not of the type *cmsSignature\n")
		return false
	}
	return cmsWriteUInt32Number(io, uint32(*sigPtr))
}

// TypeSignatureDup duplicates a cmsSignature.
func TypeSignatureDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	original, ok := ptr.([]uint32)
	if !ok {
		return nil
	}
	dup := make([]uint32, n)
	copy(dup, original)
	return dup
}

func TypeSignatureFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsFree(self.ContextID, ptr)
}

// ********************************************************************************
// Type cmsSigCurveType
// ********************************************************************************

func TypeCurveRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var count uint32
	*nItems = 0

	if !cmsReadUInt32Number(io, &count) {
		return nil
	}

	switch count {
	case 0: // Linear
		singleGamma := 1.0
		newGamma := cmsBuildParametricToneCurve(ar, self.ContextID, 1, []float64{singleGamma})
		if newGamma == nil {
			return nil
		}
		*nItems = 1
		return newGamma

	case 1: // Single gamma exponent
		var singleGammaFixed uint16
		if !cmsReadUInt16Number(io, &singleGammaFixed) {
			return nil
		}
		singleGamma := cms8Fixed8ToDouble(singleGammaFixed)
		*nItems = 1
		return cmsBuildParametricToneCurve(ar, self.ContextID, 1, []float64{singleGamma})

	default: // Curve
		if count > 0x7FFF {
			return nil // Prevent malicious behavior
		}

		newGamma := cmsBuildTabulatedToneCurve16(ar, self.ContextID, count, nil)
		if newGamma == nil {
			return nil
		}

		// Convert *uint16 to []uint16

		if !cmsReadUInt16Array(io, count, newGamma.Table16) {
			CmsFreeToneCurve(newGamma)
			return nil
		}

		*nItems = 1
		return newGamma
	}
}
func TypeCurveWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	curve, ok := ptr.(*CmsToneCurve) // Convert the pointer to a CmsToneCurve struct
	if !ok {
		fmt.Printf("Error: not of the type *cmsToneCurve\n")
		return false
	}
	if curve.nSegments == 1 && curve.Segments != nil {
		if curve.Segments[0].Type == 1 {
			// Single gamma
			singleGammaFixed := cmsDoubleTo8Fixed8(curve.Segments[0].Params[0])
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
	return cmsWriteUInt16Array(io, curve.nEntries, curve.Table16)
}

func TypeCurveDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	return cmsDupToneCurve(ar, ptr.(*CmsToneCurve))
}

func TypeCurveFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	gamma := ptr.(*CmsToneCurve)
	CmsFreeToneCurve(gamma)
}

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
func TypeDateTimeRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var timestamp cmsDateTimeNumber
	//newDateTime := (*time.Time)(cmsMalloc(self.ContextID, uint32(unsafe.Sizeof(time.Time{}))))
	newDateTime := allocateStruct[time.Time](ar)
	*nItems = 0
	if newDateTime == nil {
		return nil
	}
	timestamp, err := ReadStruct[cmsDateTimeNumber](io, binary.BigEndian, 1)
	if err != nil {
		fmt.Errorf("Failed to read cmsDateTimeNumber: %v", err)
		return nil
	}

	cmsDecodeDateTimeNumber(&timestamp)
	*nItems = 1
	return newDateTime
}

func TypeDateTimeWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	dateTime, ok := ptr.(*time.Time)
	if !ok {
		fmt.Printf("Error: not of the type *dateTime\n")
		return false
	}
	var timestamp cmsDateTimeNumber

	cmsEncodeDateTimeNumber(&timestamp, *dateTime)
	return WriteStruct[cmsDateTimeNumber](io, timestamp, binary.BigEndian)
}

func TypeDateTimeDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	xyz := *(ptr.(*time.Time))
	return &xyz
}

func TypeDateTimeFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
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
func TypeMeasurementRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	mc := cmsICCMeasurementConditions{}

	// Read the data from the IO handler
	if !cmsReadUInt32Number(io, &mc.Observer) ||
		!cmsReadXYZNumber(io, &mc.Backing) ||
		!cmsReadUInt32Number(io, &mc.Geometry) ||
		!cmsRead15Fixed16Number(io, &mc.Flare) ||
		!cmsReadUInt32Number(io, &mc.IlluminantType) {
		return nil
	}

	*nItems = 1
	return mc
}

func TypeMeasurementWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	mc, ok := ptr.(*cmsICCMeasurementConditions)
	if !ok {
		fmt.Printf("Error: not of the type *cmsICCMeasurementConditions\n")
		return false
	}
	// Write the data to the IO handler
	return cmsWriteUInt32Number(io, mc.Observer) &&
		cmsWriteXYZNumber(io, &mc.Backing) &&
		cmsWriteUInt32Number(io, mc.Geometry) &&
		cmsWrite15Fixed16Number(io, mc.Flare) &&
		cmsWriteUInt32Number(io, mc.IlluminantType)
}

func TypeMeasurementDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	orig := ptr.(*cmsICCMeasurementConditions) // Type assertion
	copy := *orig                              // Struct copy by value
	return &copy                               // Return pointer to the new copy
}

func TypeMeasurementFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsFree(self.ContextID, ptr)
}

// ********************************************************************************
// Type cmsSigMultiLocalizedUnicodeType
// ********************************************************************************
//
//	Do NOT trust SizeOfTag as there is an issue on the definition of profileSequenceDescTag. See the TechNote from
//	Max Derhak and Rohit Patil about this: basically the size of the string table should be guessed and cannot be
//	taken from the size of tag if this tag is embedded as part of bigger structures (profileSequenceDescTag, for instance)
func TypeMLURead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var count, recLen, numOfWchar, sizeOfHeader, len, offset, largestPosition uint32
	var block []uint16

	*nItems = 0

	if !cmsReadUInt32Number(io, &count) || !cmsReadUInt32Number(io, &recLen) {
		return nil
	}

	if recLen != 12 {
		cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "multiLocalizedUnicodeType of len != 12 is not supported.")
		return nil
	}

	mlu := (*cmsMLU)(cmsMLUalloc(ar, self.ContextID, count))
	if mlu == nil {
		return nil
	}

	mlu.UsedEntries = count
	sizeOfHeader = 12*count + uint32(unsafe.Sizeof(cmsTagBase{}))
	largestPosition = 0

	for i := uint32(0); i < count; i++ {
		if !cmsReadUInt16Number(io, &(mlu.Entries[i].Language)) || !cmsReadUInt16Number(io, &(mlu.Entries[i].Country)) {
			goto Error
		}

		if !cmsReadUInt32Number(io, &len) || !cmsReadUInt32Number(io, &offset) {
			goto Error
		}

		if offset&1 != 0 || offset < sizeOfHeader+8 || (offset+len) < len || (offset+len) > sizeOfTag+8 {
			goto Error
		}

		beginOfString := offset - sizeOfHeader - 8
		mlu.Entries[i].Len = (len * uint32(unsafe.Sizeof(wchar(0)))) / uint32(unsafe.Sizeof(uint16(0)))
		mlu.Entries[i].StrW = (beginOfString * uint32(unsafe.Sizeof(wchar(0)))) / uint32(unsafe.Sizeof(uint16(0)))

		endOfString := beginOfString + len
		if endOfString > largestPosition {
			largestPosition = endOfString
		}
	}

	sizeOfTag = (largestPosition * uint32(unsafe.Sizeof(wchar(0)))) / uint32(unsafe.Sizeof(uint16(0)))

	if sizeOfTag&1 != 0 {
		goto Error
	}

	block = make([]uint16, sizeOfTag)
	numOfWchar = sizeOfTag / uint32(unsafe.Sizeof(uint16(0)))

	//this is a replacement for cmsReadWCharArray in C-code needs additional check
	if !cmsReadWCharArray(io, numOfWchar, block) {
		goto Error
	}
	mlu.MemPool = block
	mlu.PoolSize = sizeOfTag
	mlu.PoolUsed = sizeOfTag

	*nItems = 1
	return mlu

Error:
	if mlu != nil {
		cmsFree(self.ContextID, mlu)
	}
	return nil
}

func TypeMLUWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	mlu, ok := ptr.(*cmsMLU)
	if !ok {
		fmt.Printf("Error: not of the type *cmsMLU\n")
		return false
	}

	var headerSize, len, offset uint32

	if ptr == nil {
		return cmsWriteUInt32Number(io, 0) && cmsWriteUInt32Number(io, 12)
	}

	if !cmsWriteUInt32Number(io, mlu.UsedEntries) || !cmsWriteUInt32Number(io, 12) {
		return false
	}

	headerSize = 12*mlu.UsedEntries + uint32(unsafe.Sizeof(cmsTagBase{}))

	for i := uint32(0); i < mlu.UsedEntries; i++ {
		len = mlu.Entries[i].Len * uint32(unsafe.Sizeof(uint16(0)))
		offset = mlu.Entries[i].StrW*uint32(unsafe.Sizeof(uint16(0))) + headerSize + 8

		if !cmsWriteUInt16Number(io, mlu.Entries[i].Language) ||
			!cmsWriteUInt16Number(io, mlu.Entries[i].Country) ||
			!cmsWriteUInt32Number(io, len) ||
			!cmsWriteUInt32Number(io, offset) {
			return false
		}
	}
	// Convert the MemPool pointer to a slice of uint16 for cmsWriteUInt16Array
	//poolSize := mlu.PoolUsed / uint32(unsafe.Sizeof(uint16(0)))
	memPoolSlice := mlu.MemPool.([]uint16)

	return cmsWriteUInt16Array(io, mlu.PoolUsed/uint32(unsafe.Sizeof(uint16(0))), memPoolSlice)
}

func TypeMLUDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	return cmsMLUdup(ar, ptr.(*cmsMLU))
}

func TypeMLUFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsMLUfree(ptr.(*cmsMLU))
}

// That will create a MPE LUT with Matrix, pre tables, CLUT and post tables.
// 8 bit lut may be scaled easily to v4 PCS, but we need also to properly adjust
// PCS on BToAxx tags and AtoB if abstract. We need to fix input direction.

func TypeLUT8Read(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var inputChannels, outputChannels, clutPoints uint8
	var nTabSize uint32
	var matrix [9]float64
	var newLUT *cmsPipeline
	var mat cmsMAT3
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
	newLUT = cmsPipelineAlloc(ar, self.ContextID, uint32(inputChannels), uint32(outputChannels))
	if newLUT == nil {
		goto Error
	}

	// Read the matrix
	for i := 0; i < 9; i++ {
		if !cmsRead15Fixed16Number(io, &matrix[i]) {
			cmsPipelineFree(ar, newLUT)
			goto Error
		}
	}

	// Insert the matrix if it isn't identity
	mat = SliceToMat(matrix[:])
	if inputChannels == 3 && !cmsMAT3isIdentity(&mat) {
		if !cmsPipelineInsertStage(newLUT, cmsAT_BEGIN, cmsStageAllocMatrix(ar, self.ContextID, 3, 3, matrix[:], nil)) {

			goto Error
		}
	}

	// Read input tables
	if !Read8bitTables(ar, self.ContextID, io, newLUT, uint32(inputChannels)) {
		goto Error
	}

	// Read 3D CLUT
	nTabSize = uipow(uint32(outputChannels), uint32(clutPoints), uint32(inputChannels))
	if nTabSize == ^uint32(0) {
		goto Error
	}
	if nTabSize > 0 {
		var PtrW, T []uint16 // Equivalent to `cmsUInt16Number *PtrW, *T`
		var Temp []uint8     // Equivalent to `cmsUInt8Number *Temp`

		// Allocate slice instead of raw memory allocation
		T = make([]uint16, nTabSize)
		PtrW = T      // PtrW now references the same slice
		if T == nil { // No need to check nil in Go, but keeping for safety
			goto Error
		}

		// Allocate temporary slice for 8-bit values
		Temp = make([]uint8, nTabSize)
		if Temp == nil {
			T = nil // Free memory reference
			goto Error
		}

		// Read `nTabSize` bytes into Temp
		if io.Read((*cms_io_handler)(io), Temp, nTabSize, 1) != 1 { // `io.Read()` should match the correct signature
			T = nil
			Temp = nil
			goto Error
		}

		// Convert from 8-bit to 16-bit using slice indexing
		for i := 0; i < int(nTabSize); i++ {
			PtrW[i] = FROM_8_TO_16(Temp[i]) // Equivalent to `*PtrW++ = FROM_8_TO_16(Temp[i])`
		}

		// Free Temp since it's no longer needed
		Temp = nil

		// Insert stage into LUT
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, cmsStageAllocCLut16bit(ar, self.ContextID, uint32(clutPoints), uint32(inputChannels), uint32(outputChannels), T)) {
			T = nil
			goto Error
		}

		// No explicit free needed; slices are managed by Go GC
	}

	// Read output tables
	if !Read8bitTables(ar, self.ContextID, io, newLUT, uint32(outputChannels)) {
		goto Error
	}

	*nItems = 1
	return newLUT

Error:
	if newLUT != nil {
		cmsPipelineFree(ar, newLUT)
	}
	return nil

}
func TypeLUT8Write(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	newLUT, ok := ptr.(*cmsPipeline)
	if !ok {
		fmt.Printf("Error: not of the type *cmsPipeline\n")
		return false
	}
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
		matMPE = mpe.Data.(*cmsStageMatrixData)
		mpe = mpe.Next
	}

	if mpe != nil && mpe.Type == cmsSigCurveSetElemType {
		preMPE = mpe.Data.(*cmsStageToneCurvesData)
		mpe = mpe.Next
	}

	if mpe != nil && mpe.Type == cmsSigCLutElemType {
		clut = mpe.Data.(*cmsStageCLutData)
		mpe = mpe.Next
	}

	if mpe != nil && mpe.Type == cmsSigCurveSetElemType {
		postMPE = mpe.Data.(*cmsStageToneCurvesData)
		mpe = mpe.Next
	}

	// Ensure no extra stages
	if mpe != nil {
		cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "LUT is not suitable to be saved as LUT8")
		return false
	}

	// Check clutPoints
	if clut == nil {
		clutPoints = 0
	} else {
		clutPoints = clut.Params.nSamples[0]
		for i = 1; i < cmsPipelineInputChannels(newLUT); i++ {
			if clut.Params.nSamples[i] != clutPoints {
				cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "LUT with different samples per dimension not suitable to be saved as LUT16")
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
		for i := 0; i < 9; i++ {
			if !cmsWrite15Fixed16Number(io, matMPE.Double[i]) {
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
		for j = 0; j < nTabSize; j++ {
			val = uint8(FROM_16_TO_8(clut.Tab.([]uint16)[j]))
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

func TypeLUT8Dup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, nItems uint32) interface{} {
	return cmsPipelineDup(ar, ptr.(*cmsPipeline))
}

func TypeLUT8Free(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsPipelineFree(ar, ptr.(*cmsPipeline))
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

func Read8bitTables(ar *arena.Arena, ContextID CmsContext, io *cmsIOHANDLER, lut *cmsPipeline, nChannels uint32) bool {
	if nChannels > cmsMAXCHANNELS || nChannels <= 0 {
		return false
	}

	var tables [cmsMAXCHANNELS]*CmsToneCurve
	temp := make([]uint8, 256)
	if temp == nil {
		return false
	}
	//defer cmsFree(ContextID, unsafe.Pointer(temp))

	// Allocate tone curves
	for i := uint32(0); i < nChannels; i++ {
		tables[i] = cmsBuildTabulatedToneCurve16(ar, ContextID, 256, nil)
		if tables[i] == nil {
			goto Error
		}
	}

	// Read and populate tone curve data
	for i := uint32(0); i < nChannels; i++ {
		if io.Read((*cms_io_handler)(io), temp, 256, 1) != 1 {
			goto Error
		}
		for j := 0; j < 256; j++ {
			tables[i].Table16[j] = FROM_8_TO_16(temp[j]) // Convert 8-bit to 16-bit
		}

	}

	// Insert tone curves into the pipeline
	if !cmsPipelineInsertStage(lut, cmsAT_END, cmsStageAllocToneCurves(ar, ContextID, nChannels, tables[:])) {
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
			// Handle identity curves
			if tables.TheCurves[i].nEntries == 2 && tables.TheCurves[i].Table16[0] == 0 && tables.TheCurves[i].Table16[1] == 65535 {
				for j := 0; j < 256; j++ {
					if !cmsWriteUInt8Number(io, uint8(j)) {
						return false
					}
				}
			} else if tables.TheCurves[i].nEntries != 256 {
				cmsSignalError(ContextID, cmsERROR_RANGE, "LUT8 needs 256 entries on prelinearization")
				return false
			} else {
				for j := 0; j < 256; j++ {
					val := FROM_16_TO_8(tables.TheCurves[i].Table16[j]) // Convert 16-bit to 8-bit
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
func Read16bitTables(ar *arena.Arena, ContextID CmsContext, io *cmsIOHANDLER, lut *cmsPipeline, nChannels, nEntries uint32) bool {
	if nEntries <= 0 {
		return true
	}
	if nEntries < 2 || nChannels > cmsMAXCHANNELS {
		return false
	}
	var tables [cmsMAXCHANNELS]*CmsToneCurve

	for i := uint32(0); i < nChannels; i++ {
		tables[i] = cmsBuildTabulatedToneCurve16(ar, ContextID, nEntries, nil)
		if tables[i] == nil {
			goto Error
		}

		if !cmsReadUInt16Array(io, nEntries, tables[i].Table16) {
			goto Error
		}
	}

	if !cmsPipelineInsertStage(lut, cmsAT_END, cmsStageAllocToneCurves(ar, ContextID, nChannels, tables[:])) {
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
		nEntries := tables.TheCurves[i].nEntries

		for j := uint32(0); j < nEntries; j++ {
			val := tables.TheCurves[i].Table16[j]
			if !cmsWriteUInt16Number(io, val) {
				return false
			}
		}
	}
	return true
}

func TypeLUT16Read(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
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

	newLUT := cmsPipelineAlloc(ar, self.ContextID, uint32(inputChannels), uint32(outputChannels))
	if newLUT == nil {
		return nil
	}

	for i := 0; i < 9; i++ {
		if !cmsRead15Fixed16Number(io, &matrix[i]) {
			cmsPipelineFree(ar, newLUT)
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
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, cmsStageAllocMatrix(ar, self.ContextID, 3, 3, matrix[:], nil)) {
			cmsPipelineFree(ar, newLUT)
			return nil
		}
	}

	if !cmsReadUInt16Number(io, &inputEntries) || !cmsReadUInt16Number(io, &outputEntries) {
		cmsPipelineFree(ar, newLUT)
		return nil
	}

	if inputEntries > 0x7FFF || outputEntries > 0x7FFF || clutPoints == 1 {
		cmsPipelineFree(ar, newLUT)
		return nil
	}

	if !Read16bitTables(ar, self.ContextID, io, newLUT, uint32(inputChannels), uint32(inputEntries)) {
		cmsPipelineFree(ar, newLUT)
		return nil
	}

	nTabSize := uipow(uint32(outputChannels), uint32(clutPoints), uint32(inputChannels))
	if nTabSize == math.MaxUint32 || nTabSize > 0 {
		t := make([]uint16, nTabSize)
		if !cmsReadUInt16Array(io, nTabSize, t) {
			cmsPipelineFree(ar, newLUT)
			return nil
		}
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, cmsStageAllocCLut16bit(ar, self.ContextID, uint32(clutPoints), uint32(inputChannels), uint32(outputChannels), t)) {
			cmsPipelineFree(ar, newLUT)
			return nil
		}
	}

	if !Read16bitTables(ar, self.ContextID, io, newLUT, uint32(outputChannels), uint32(outputEntries)) {
		cmsPipelineFree(ar, newLUT)
		return nil
	}

	*nItems = 1

	return newLUT
}

func TypeLUT16Write(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	newLUT, ok := ptr.(*cmsPipeline)
	if !ok {
		fmt.Printf("Error: not of the type *cmsPipeline\n")
		return false
	}
	var matMPE *cmsStageMatrixData
	var preMPE, postMPE *cmsStageToneCurvesData
	var clut *cmsStageCLutData
	var clutPoints uint32

	mpe := newLUT.Elements

	if mpe != nil && mpe.Type == cmsSigMatrixElemType {
		matMPE = mpe.Data.(*cmsStageMatrixData)
		if mpe.InputChannels != 3 || mpe.OutputChannels != 3 {
			return false
		}
		mpe = mpe.Next
	}

	if mpe != nil && mpe.Type == cmsSigCurveSetElemType {
		preMPE = mpe.Data.(*cmsStageToneCurvesData)
		mpe = mpe.Next
	}

	if mpe != nil && mpe.Type == cmsSigCLutElemType {
		clut = mpe.Data.(*cmsStageCLutData)
		mpe = mpe.Next
	}

	if mpe != nil && mpe.Type == cmsSigCurveSetElemType {
		postMPE = mpe.Data.(*cmsStageToneCurvesData)
		mpe = mpe.Next
	}

	if mpe != nil {
		cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "LUT is not suitable to be saved as LUT16")
		return false
	}

	inputChannels := cmsPipelineInputChannels(newLUT)
	outputChannels := cmsPipelineOutputChannels(newLUT)

	if clut != nil {
		clutPoints = clut.Params.nSamples[0]
		for i := uint32(1); i < inputChannels; i++ {
			if clut.Params.nSamples[i] != clutPoints {
				cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "LUT with different samples per dimension not suitable to be saved as LUT16")
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
		for i := 0; i < 9; i++ {
			if !cmsWrite15Fixed16Number(io, matMPE.Double[i]) {
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
		if !cmsWriteUInt16Number(io, uint16(preMPE.TheCurves[0].nEntries)) {
			return false
		}
	} else {
		if !cmsWriteUInt16Number(io, 2) {
			return false
		}
	}

	if postMPE != nil {
		if !cmsWriteUInt16Number(io, uint16(postMPE.TheCurves[0].nEntries)) {
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
			if !cmsWriteUInt16Array(io, nTabSize, clut.Tab.([]uint16)) {
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

func TypeLUT16Dup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	return cmsPipelineDup(ar, ptr.(*cmsPipeline))
}

func TypeLUT16Free(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsPipelineFree(ar, ptr.(*cmsPipeline))
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

func TypeColorantTableRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var count uint32
	var name [34]byte
	var pcs [3]uint16

	if !cmsReadUInt32Number(io, &count) {
		return nil
	}

	if count > cmsMAXCHANNELS {
		cmsSignalError(self.ContextID, cmsERROR_RANGE, "Too many colorants")
		return nil
	}

	list := cmsAllocNamedColorList(ar, self.ContextID, count, 0, "", "")
	if list == nil {
		return nil
	}

	for i := uint32(0); i < count; i++ {
		if io.Read((*cms_io_handler)(io), name[:], 32, 1) != 1 {
			goto Error
		}

		name[32] = 0 // Null-terminate
		if !cmsReadUInt16Array(io, 3, pcs[:]) {
			goto Error
		}
		nameStr := string(name[:bytes.IndexByte(name[:], 0)])
		if !cmsAppendNamedColor(list, nameStr, &pcs, nil) {
			goto Error
		}
	}

	*nItems = 1
	return list

Error:
	*nItems = 0
	cmsFreeNamedColorList(list)
	return nil
}

func TypeColorantTableWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	namedColorList, ok := ptr.(*cmsNAMEDCOLORLIST)
	if !ok {
		fmt.Printf("Error: not of the type *NAMEDCOLORLIST\n")
		return false
	}
	nColors := cmsNamedColorCount(namedColorList)

	if !cmsWriteUInt32Number(io, nColors) {
		return false
	}

	for i := uint32(0); i < nColors; i++ {
		var root [cmsMAX_PATH]byte
		var pcs [3]uint16

		if !cmsNamedColorInfo(namedColorList, i, root[:], nil, nil, pcs[:], nil) {
			return false
		}

		// Null-terminate root name
		root[32] = 0

		if !io.Write((*cms_io_handler)(io), 32, root[:]) {
			return false
		}
		if !cmsWriteUInt16Array(io, 3, pcs[:]) {
			return false
		}
	}

	return true
}

func TypeColorantTableDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	nc := ptr.(*cmsNAMEDCOLORLIST)
	return cmsDupNamedColorList(ar, nc)
}

func TypeColorantTableFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsFreeNamedColorList(ptr.(*cmsNAMEDCOLORLIST))
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
func TypeNamedColorRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var vendorFlag, count, nDeviceCoords uint32
	var prefix, suffix [32]byte

	*nItems = 0

	if !cmsReadUInt32Number(io, &vendorFlag) ||
		!cmsReadUInt32Number(io, &count) ||
		!cmsReadUInt32Number(io, &nDeviceCoords) {
		return nil
	}

	if io.Read((*cms_io_handler)(io), prefix, 32, 1) != 1 ||
		io.Read((*cms_io_handler)(io), suffix, 32, 1) != 1 {
		return nil
	}

	prefix[31] = 0 // Null-terminate
	suffix[31] = 0 // Null-terminate
	prefixStr := string(prefix[:bytes.IndexByte(prefix[:], 0)])
	suffixStr := string(suffix[:bytes.IndexByte(suffix[:], 0)])
	namedColorList := cmsAllocNamedColorList(ar, self.ContextID, count, nDeviceCoords, prefixStr, suffixStr)
	if namedColorList == nil {
		cmsSignalError(self.ContextID, cmsERROR_RANGE, "Too many named colors")
		return nil
	}

	if nDeviceCoords > cmsMAXCHANNELS {
		cmsSignalError(self.ContextID, cmsERROR_RANGE, "Too many device coordinates")
		goto Error
	}

	for i := uint32(0); i < count; i++ {
		var pcs [3]uint16
		var colorant [cmsMAXCHANNELS]uint16
		var root [33]byte

		if io.Read((*cms_io_handler)(io), root[:], 32, 1) != 1 {
			goto Error
		}

		root[32] = 0 // Null-terminate
		if !cmsReadUInt16Array(io, 3, pcs[:]) || !cmsReadUInt16Array(io, nDeviceCoords, colorant[:]) {
			goto Error
		}

		rootStr := string(root[:bytes.IndexByte(root[:], 0)])
		if !cmsAppendNamedColor(namedColorList, rootStr, &pcs, &colorant) {
			goto Error
		}
	}

	*nItems = 1
	return namedColorList

Error:
	cmsFreeNamedColorList(namedColorList)
	return nil
}

func TypeNamedColorWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	namedColorList := ptr.(*cmsNAMEDCOLORLIST)
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

	if !io.Write((*cms_io_handler)(io), 32, prefix[:]) ||
		!io.Write((*cms_io_handler)(io), 32, suffix[:]) {
		return false
	}

	for i := uint32(0); i < nColors; i++ {
		var root [33]byte
		var pcs [3]uint16
		var colorant [cmsMAXCHANNELS]uint16

		if !cmsNamedColorInfo(namedColorList, i, root[:], nil, nil, pcs[:], colorant[:]) {
			return false
		}

		root[32] = 0 // Null-terminate
		if !io.Write((*cms_io_handler)(io), 32, root[:]) ||
			!cmsWriteUInt16Array(io, 3, pcs[:]) ||
			!cmsWriteUInt16Array(io, namedColorList.ColorantCount, colorant[:]) {
			return false
		}
	}

	return true
}

func TypeNamedColorDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	nc := ptr.(*cmsNAMEDCOLORLIST)
	return cmsDupNamedColorList(ar, nc)
}

func TypeNamedColorFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsFreeNamedColorList(ptr.(*cmsNAMEDCOLORLIST))
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
func ReadEmbeddedText(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, mlu **cmsMLU, sizeOfTag uint32) bool {
	baseType := cmsReadTypeBase(io)
	switch baseType {
	case cmsSigTextType:
		if *mlu != nil {
			cmsMLUfree(*mlu)
		}
		*mlu = TypeTextRead(ar, self, io, new(uint32), sizeOfTag).(*cmsMLU)
		return *mlu != nil

	case cmsSigTextDescriptionType:
		if *mlu != nil {
			cmsMLUfree(*mlu)
		}
		*mlu = TypeTextDescriptionRead(ar, self, io, new(uint32), sizeOfTag).(*cmsMLU)
		return *mlu != nil

	case cmsSigMultiLocalizedUnicodeType:
		if *mlu != nil {
			cmsMLUfree(*mlu)
		}
		*mlu = TypeMLURead(ar, self, io, new(uint32), sizeOfTag).(*cmsMLU)
		return *mlu != nil

	default:
		return false
	}
}

func TypeProfileSequenceDescRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var count uint32

	*nItems = 0

	if !cmsReadUInt32Number(io, &count) {
		return nil
	}

	if sizeOfTag < uint32(unsafe.Sizeof(count)) {
		return nil
	}
	sizeOfTag -= uint32(unsafe.Sizeof(count))

	outSeq := cmsAllocProfileSequenceDescription(ar, self.ContextID, count)
	if outSeq == nil {
		return nil
	}
	outSeq.n = count
	// Convert `outSeq.seq` to a slice for indexing
	seqSlice := outSeq.seq

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

		if !ReadEmbeddedText(ar, self, io, &sec.Manufacturer, sizeOfTag) ||
			!ReadEmbeddedText(ar, self, io, &sec.Model, sizeOfTag) {
			goto Error
		}
	}

	*nItems = 1
	return outSeq

Error:
	cmsFreeProfileSequenceDescription(outSeq)
	return nil
}

func SaveDescription(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, text *cmsMLU) bool {
	if self.ICCVersion < 0x4000000 {
		if !cmsWriteTypeBase(io, cmsSigTextDescriptionType) {
			return false
		}
		return TypeTextDescriptionWrite(ar, self, io, text, 1)
	} else {
		if !cmsWriteTypeBase(io, cmsSigMultiLocalizedUnicodeType) {
			return false
		}
		return TypeMLUWrite(ar, self, io, text, 1)
	}
}

func TypeProfileSequenceDescWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	seq, ok := ptr.(*cmsSEQ)
	if !ok {
		fmt.Printf("Error: not of the type *cmsSEQ\n")
		return false
	}
	if !cmsWriteUInt32Number(io, seq.n) {
		return false
	}
	// Convert `outSeq.seq` to a slice for indexing
	seqSlice := seq.seq

	for i := uint32(0); i < seq.n; i++ {
		sec := &seqSlice[i]

		if !cmsWriteUInt32Number(io, uint32(sec.deviceMfg)) ||
			!cmsWriteUInt32Number(io, uint32(sec.deviceModel)) ||
			!cmsWriteUInt64Number(io, uint64(sec.attributes)) ||
			!cmsWriteUInt32Number(io, uint32(sec.technology)) ||
			!SaveDescription(ar, self, io, sec.Manufacturer) ||
			!SaveDescription(ar, self, io, sec.Model) {
			return false
		}
	}

	return true
}

func TypeProfileSequenceDescDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	return cmsDupProfileSequenceDescription(ar, ptr.(*cmsSEQ))
}

func TypeProfileSequenceDescFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsFreeProfileSequenceDescription(ptr.(*cmsSEQ))
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
func ReadSeqID(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, cargo interface{}, n, sizeOfTag uint32) bool {
	outSeq, ok := cargo.(*cmsSEQ)
	if !ok {
		fmt.Printf("Error: not of the type *cmsSEQ\n")
		return false
	}
	seqSlice := outSeq.seq // Convert pointer to slice
	seq := &seqSlice[n]

	if io.Read((*cms_io_handler)(io), seq.ProfileID[:], 16, 1) != 1 {
		return false
	}
	if !ReadEmbeddedText(ar, self, io, &seq.Description, sizeOfTag) {
		return false
	}

	return true
}

func TypeProfileSequenceIdRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var count, baseOffset uint32

	*nItems = 0

	// Get actual position as a basis for element offsets
	baseOffset = uint32(io.Tell((*cms_io_handler)(io))) - uint32(unsafe.Sizeof(cmsTagBase{}))

	// Get table count
	if !cmsReadUInt32Number(io, &count) {
		return nil
	}

	// Allocate an empty structure
	outSeq := cmsAllocProfileSequenceDescription(ar, self.ContextID, count)
	if outSeq == nil {
		return nil
	}

	// Read the position table
	if !ReadPositionTable(ar, self, io, count, baseOffset, outSeq, ReadSeqID) {
		cmsFreeProfileSequenceDescription(outSeq)
		return nil
	}

	// Success
	*nItems = 1
	return outSeq
}

func WriteSeqID(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, cargo interface{}, n, sizeOfTag uint32) bool {
	seq, ok := cargo.(*cmsSEQ)
	if !ok {
		fmt.Printf("Error: not of the type *cmsSEQ\n")
		return false
	}
	seqSlice := seq.seq // Convert pointer to slice
	currentSeq := &seqSlice[n]

	// Write Profile ID
	//	if io.Write((*cms_io_handler)(io), 16, unsafe.Pointer(&currentSeq.ProfileID.ID8[0])) {
	if io.Write((*cms_io_handler)(io), 16, currentSeq.ProfileID[:]) {
		return false
	}

	// Store the MLU
	if !SaveDescription(ar,self, io, currentSeq.Description) {
		return false
	}

	return true
}

func TypeProfileSequenceIdWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	seq, ok := ptr.(*cmsSEQ)
	if !ok {
		fmt.Printf("Error: not of the type *cmsSEQ\n")
		return false
	}

	baseOffset := uint32(io.Tell((*cms_io_handler)(io))) - uint32(unsafe.Sizeof(cmsTagBase{}))

	// Write the table count
	if !cmsWriteUInt32Number(io, seq.n) {
		return false
	}

	// Write the position table and content
	if !WritePositionTable(ar, self, io, 0, seq.n, baseOffset, seq, WriteSeqID) {
		return false
	}

	return true
}

func TypeProfileSequenceIdDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, nItems uint32) interface{} {
	seq, ok := ptr.(*cmsSEQ)
	if !ok {
		fmt.Printf("Error: not of the type *cmsSEQ\n")
		return false
	}

	return cmsDupProfileSequenceDescription(ar, seq)
}

func TypeProfileSequenceIdFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsFreeProfileSequenceDescription(ptr.(*cmsSEQ))
}

// ********************************************************************************
// Type cmsSigUcrBgType
// ********************************************************************************
/*
This type contains curves representing the under color removal and black
generation and a text string which is a general description of the method used
for the ucr/bg.
*/
func TypeUcrBgRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	n := allocateStruct[cmsUcrBg](ar)
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

	n.Ucr = cmsBuildTabulatedToneCurve16(ar, self.ContextID, countUcr, nil)
	if n.Ucr == nil || signedSizeOfTag < int32(countUcr*uint32(unsafe.Sizeof(uint16(0)))) || !cmsReadUInt16Array(io, countUcr, n.Ucr.Table16) {
		goto Error
	}
	signedSizeOfTag -= int32(countUcr * uint32(unsafe.Sizeof(uint16(0))))

	// Second curve is Black generation
	if signedSizeOfTag < int32(unsafe.Sizeof(uint32(0))) || !cmsReadUInt32Number(io, &countBg) {
		goto Error
	}
	signedSizeOfTag -= int32(unsafe.Sizeof(uint32(0)))

	n.Bg = cmsBuildTabulatedToneCurve16(ar, self.ContextID, countBg, nil)
	if n.Bg == nil || signedSizeOfTag < int32(countBg*uint32(unsafe.Sizeof(uint16(0)))) || !cmsReadUInt16Array(io, countBg, n.Bg.Table16) {
		goto Error
	}
	signedSizeOfTag -= int32(countBg * uint32(unsafe.Sizeof(uint16(0))))

	if signedSizeOfTag < 0 || signedSizeOfTag > 32000 {
		goto Error
	}

	// Now comes the text
	n.Desc = cmsMLUalloc(ar, self.ContextID, 1)
	if n.Desc == nil {
		goto Error
	}

	asciiString = make([]byte, signedSizeOfTag+1)
	if io.Read((*cms_io_handler)(io), asciiString, 1, uint32(signedSizeOfTag)) != uint32(signedSizeOfTag) {
		cmsFree(self.ContextID, asciiString)
		goto Error
	}
	asciiString[signedSizeOfTag] = 0

	cmsMLUsetASCII(n.Desc, cmsNoLanguage, cmsNoCountry, string(asciiString))

	*nItems = 1
	return n

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
	cmsFree(self.ContextID, n)
	*nItems = 0
	return nil
}

func TypeUcrBgWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	value, ok := ptr.(*cmsUcrBg)
	if !ok {
		fmt.Printf("Error: not of the type *cmsUrcBg\n")
		return false
	}

	var textSize uint32

	// First curve is Under color removal
	if !cmsWriteUInt32Number(io, value.Ucr.nEntries) || !cmsWriteUInt16Array(io, value.Ucr.nEntries, value.Ucr.Table16) {
		return false
	}

	// Then black generation
	if !cmsWriteUInt32Number(io, value.Bg.nEntries) || !cmsWriteUInt16Array(io, value.Bg.nEntries, value.Bg.Table16) {
		return false
	}

	// Now comes the text
	textSize = cmsMLUgetASCII(value.Desc, cmsNoLanguage, cmsNoCountry, nil, 0)
	text := make([]byte, textSize)
	if cmsMLUgetASCII(value.Desc, cmsNoLanguage, cmsNoCountry, text, textSize) != textSize {
		return false
	}

	if !io.Write((*cms_io_handler)(io), textSize, text) {
		return false
	}
	return true
}

func TypeUcrBgDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	src := ptr.(*cmsUcrBg)
	newUcrBg := allocateStruct[cmsUcrBg](ar)
	if newUcrBg == nil {
		return nil
	}

	newUcrBg.Bg = cmsDupToneCurve(ar,src.Bg)
	newUcrBg.Ucr = cmsDupToneCurve(ar,src.Ucr)
	newUcrBg.Desc = cmsMLUdup(ar,src.Desc)
	return newUcrBg
}

func TypeUcrBgFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	src := ptr.(*cmsUcrBg)
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

func ReadCountAndString(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, mlu *cmsMLU, sizeOfTag *uint32, section string) bool {
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
	text := make([]byte, count+1)

	// Read string
	if io.Read((*cms_io_handler)(io), text, 1, count) != count {
		return false
	}

	// Null-terminate the string
	text[count] = 0

	// Set the string in the MLU
	cmsMLUsetASCII(mlu, "PS", section, string(text))

	// Free temporary memory
	cmsFree(self.ContextID, text)

	// Update size of tag
	*sizeOfTag -= count + uint32(unsafe.Sizeof(count))
	return true
}

func WriteCountAndString(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, mlu *cmsMLU, section string) bool {
	textSize := cmsMLUgetASCII(mlu, "PS", section, nil, 0)
	text := make([]byte, textSize)

	if text == nil {
		return false
	}

	// Write size of string
	if !cmsWriteUInt32Number(io, textSize) {
		return false
	}

	// Get the string
	if cmsMLUgetASCII(mlu, "PS", section, text, textSize) == 0 {
		return false
	}

	// Write the string
	if !io.Write((*cms_io_handler)(io), textSize, text) {
		return false
	}

	// Free temporary memory
	cmsFree(self.ContextID, text)
	return true
}

func TypeCrdInfoRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	mlu := cmsMLUalloc(ar,self.ContextID, 5)

	*nItems = 0
	if mlu == nil {
		return nil
	}

	// Read strings for each section
	if !ReadCountAndString(ar,self, io, mlu, &sizeOfTag, "nm") ||
		!ReadCountAndString(ar,self, io, mlu, &sizeOfTag, "#0") ||
		!ReadCountAndString(ar,self, io, mlu, &sizeOfTag, "#1") ||
		!ReadCountAndString(ar,self, io, mlu, &sizeOfTag, "#2") ||
		!ReadCountAndString(ar,self, io, mlu, &sizeOfTag, "#3") {
		cmsMLUfree(mlu)
		return nil
	}

	*nItems = 1
	return mlu
}

func TypeCrdInfoWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	mlu, ok := ptr.(*cmsMLU)
	if !ok {
		fmt.Printf("Error: not of the type *cmsMLU\n")
		return false
	}
	// Write strings for each section
	if !WriteCountAndString(ar,self, io, mlu, "nm") ||
		!WriteCountAndString(ar,self, io, mlu, "#0") ||
		!WriteCountAndString(ar,self, io, mlu, "#1") ||
		!WriteCountAndString(ar,self, io, mlu, "#2") ||
		!WriteCountAndString(ar,self, io, mlu, "#3") {
		return false
	}

	return true
}

func TypeCrdInfoDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, nItems uint32) interface{} {
	mlu := ptr.(*cmsMLU)
	return cmsMLUdup(ar,mlu)
}

func TypeCrdInfoFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	mlu := ptr.(*cmsMLU)
	cmsMLUfree(mlu)
}

// ********************************************************************************
// Type cmsSigScreeningType
// ********************************************************************************
//
//The screeningType describes various screening parameters including screen
//frequency, screening angle, and spot shape.

// ********************************************************************************
// Type cmsSigDataType
// ********************************************************************************
func TypeDataRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	// Minimum size must include the flag (4 bytes)
	if sizeOfTag < 4 {
		return nil
	}

	// Calculate the size of the data after the flag
	lenOfData := sizeOfTag - 4
	if lenOfData > math.MaxInt32 {
		return nil
	}

	// Read the flag first (uint32)
	var flag uint32
	if !cmsReadUInt32Number(io, &flag) {
		return nil
	}

	// Allocate and read the data slice
	data := make([]byte, lenOfData)
	if io.Read((*cms_io_handler)(io), data, 1, lenOfData) != lenOfData {
		return nil
	}

	// Fill the structure
	binData := &cmsICCData{
		Len:  lenOfData,
		Flag: flag,
		Data: data,
	}

	*nItems = 1
	return binData
}
func TypeDataWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	binData, ok := ptr.(*cmsICCData)
	if !ok {
		fmt.Printf("Error: not of the type *cmsICCData\n")
		return false
	}
	// Validate that Len matches the actual length of Data
	if uint32(len(binData.Data)) != binData.Len {
		return false // or panic, depending on how strict you want to be
	}

	// Write the flag field
	if !cmsWriteUInt32Number(io, binData.Flag) {
		return false
	}

	// Write the data
	if !io.Write((*cms_io_handler)(io), binData.Len, binData.Data) {
		return false
	}

	return true
}

func TypeDataDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	orig := ptr.(*cmsICCData)

	// Deep copy the binary data
	dataCopy := make([]uint8, len(orig.Data))
	copy(dataCopy, orig.Data)

	return &cmsICCData{
		Len:  orig.Len,
		Flag: orig.Flag,
		Data: dataCopy,
	}
}

func TypeDataFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
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
func TypeLUTA2BRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
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
	newLUT := cmsPipelineAlloc(ar,self.ContextID, uint32(inputChan), uint32(outputChan))
	if newLUT == nil {
		return nil
	}

	// Process each offset and add corresponding stages to the pipeline
	if offsetA != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadSetOfCurves(ar,self, io, baseOffset+offsetA, uint32(inputChan))) {
			goto Error
		}
	}

	if offsetC != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadCLUT(ar,self, io, baseOffset+offsetC, uint32(inputChan), uint32(outputChan))) {
			goto Error
		}
	}

	if offsetM != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadSetOfCurves(ar,self, io, baseOffset+offsetM, uint32(outputChan))) {
			goto Error
		}
	}

	if offsetMat != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadMatrix(ar,self, io, baseOffset+offsetMat)) {
			goto Error
		}
	}

	if offsetB != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadSetOfCurves(ar,self, io, baseOffset+offsetB, uint32(outputChan))) {
			goto Error
		}
	}

	*nItems = 1
	return newLUT

Error:
	cmsPipelineFree(ar,newLUT)
	return nil
}

func TypeLUTA2BWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	lut, ok := ptr.(*cmsPipeline)
	if !ok {
		fmt.Printf("Error: not of the type *cmsPipeline\n")
		return false
	}
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
			cmsSignalError(self.ContextID, cmsERROR_NOT_SUITABLE, "LUT is not suitable to be saved as LutAToB")
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
		if !WriteSetOfCurves(ar,self, io, cmsSigParametricCurveType, a) {
			return false
		}
	}

	if clut != nil {
		offsetC = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		precision := uint8(2)
		if lut.SaveAs8Bits {
			precision = 1
		}
		if !WriteCLUT(ar,self, io, precision, clut) {
			return false
		}
	}

	if m != nil {
		offsetM = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteSetOfCurves(ar,self, io, cmsSigParametricCurveType, m) {
			return false
		}
	}

	if matrix != nil {
		offsetMat = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteMatrix(ar,self, io, matrix) {
			return false
		}
	}

	if b != nil {
		offsetB = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteSetOfCurves(ar,self, io, cmsSigParametricCurveType, b) {
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

func TypeLUTA2BDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	return cmsPipelineDup(ar,ptr.(*cmsPipeline))
}

func TypeLUTA2BFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsPipelineFree(ar,ptr.(*cmsPipeline))
}

func WriteMatrix(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, mpe *cmsStage) bool {
	//fmt.Println("START WriteMatrix")

	matrixData, ok := mpe.Data.(*cmsStageMatrixData)
	if !ok {
		fmt.Printf("Error: not of the type *cmsStageMatrixData\n")
		return false
	}

	n := mpe.InputChannels * mpe.OutputChannels

	// Write the matrix values
	for i := uint32(0); i < n; i++ {
		if !cmsWrite15Fixed16Number(io, matrixData.Double[i]) {
			return false
		}
	}

	// Write the offsets
	if matrixData.Offset != nil {
		for i := uint32(0); i < mpe.OutputChannels; i++ {
			if !cmsWrite15Fixed16Number(io, matrixData.Offset[i]) {
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
	//fmt.Println("END WriteMatrix")

	return true
}
func WriteSetOfCurves(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, curveType cmsTagTypeSignature, mpe *cmsStage) bool {
	curves := cmsStageGetPtrToCurveSet(mpe)
	outputChannels := cmsStageOutputChannels(mpe)

	for i := uint32(0); i < outputChannels; i++ {
		currentType := curveType
		if curves[i].Segments != nil {
			// Determine the curve type
			if curves[i].nSegments == 0 || (curves[i].nSegments == 2 && curves[i].Segments[1].Type == 0) || curves[i].Segments[0].Type < 0 {
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
			if !TypeCurveWrite(ar,self, io, curves[i], 1) {
				return false
			}
		case cmsSigParametricCurveType:
			if !TypeParametricCurveWrite(ar,self, io, curves[i], 1) {
				return false
			}
		default:
			cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unknown curve type")
			return false
		}

		if !cmsWriteAlignment(io) {
			return false
		}
	}

	return true
}
func WriteCLUT(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, precision uint8, mpe *cmsStage) bool {
	clutData, ok := mpe.Data.(*cmsStageCLutData)
	if !ok {
		fmt.Printf("Error: not of the type *cmsStageCLutData\n")
		return false
	}

	var gridPoints [cmsMAXCHANNELS]uint8
	if clutData.HasFloatValues {
		cmsSignalError(self.ContextID, cmsERROR_NOT_SUITABLE, "Cannot save floating point data, CLUTs are 8 or 16-bit only")
		return false
	}

	// Write the grid points
	for i := 0; i < int(clutData.Params.nInputs); i++ {
		gridPoints[i] = uint8(clutData.Params.nSamples[i])
	}

	if !io.Write((*cms_io_handler)(io), cmsMAXCHANNELS, gridPoints[:]) {
		return false
	}

	// Write precision and padding
	if !cmsWriteUInt8Number(io, precision) || !cmsWriteUInt8Number(io, 0) || !cmsWriteUInt8Number(io, 0) || !cmsWriteUInt8Number(io, 0) {
		return false
	}

	// Write the CLUT data
	switch precision {
	case 1:
		for _, entry := range clutData.Tab.([]uint16) {
			if !cmsWriteUInt8Number(io, uint8(FROM_16_TO_8(entry))) {
				return false
			}
		}
	case 2:
		if !cmsWriteUInt16Array(io, clutData.NEntries, clutData.Tab.([]uint16)) {
			return false
		}
	default:
		cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unknown precision")
		return false
	}

	return cmsWriteAlignment(io)
}
func ReadMatrix(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, offset uint32) *cmsStage {
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
	return cmsStageAllocMatrix(ar,self.ContextID, 3, 3, dMat[:], dOff[:])
}
func ReadCLUT(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, offset, inputChannels, outputChannels uint32) *cmsStage {
	var gridPoints8 [cmsMAXCHANNELS]uint8
	var gridPoints [cmsMAXCHANNELS]uint32
	var precision uint8

	// Seek to offset
	if !io.Seek((*cms_io_handler)(io), offset) {
		return nil
	}

	// Read the grid points
	if io.Read((*cms_io_handler)(io), gridPoints8[:], cmsMAXCHANNELS, 1) != 1 {
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
	clut := cmsStageAllocCLut16bitGranular(ar,self.ContextID, gridPoints[:inputChannels], inputChannels, outputChannels, nil)
	if clut == nil {
		return nil
	}

	data, ok := clut.Data.(*cmsStageCLutData)
	if !ok {
		fmt.Printf("Error: not of the type *cmsStageCLutData\n")
		return nil
	}

	// Read the CLUT data
	switch precision {
	case 1:
		for i := uint32(0); i < data.NEntries; i++ {
			value, err := ReadStruct[uint8](io, binary.BigEndian, 1)
			if err != nil {
				cmsStageFree(ar,clut)
				fmt.Errorf("Failed to read uint8: %v", err)
				return nil
			}
			data.Tab.([]uint16)[i] = FROM_8_TO_16(value)
		}
	case 2:
		if !cmsReadUInt16Array(io, data.NEntries, data.Tab.([]uint16)) {
			cmsStageFree(ar,clut)
			return nil
		}
	default:
		cmsStageFree(ar,clut)
		cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unknown precision")
		return nil
	}

	return clut
}
func ReadEmbeddedCurve(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER) *CmsToneCurve {
	baseType := cmsReadTypeBase(io)

	switch baseType {
	case cmsSigCurveType:
		return TypeCurveRead(ar,self, io, nil, 0).(*CmsToneCurve)
	case cmsSigParametricCurveType:
		return TypeParametricCurveRead(ar,self, io, nil, 0).(*CmsToneCurve)
	default:
		//	str := cmsTagSignature2String(sig)
		cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unknown curve type ")
		return nil
	}
}
func ReadSetOfCurves(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, offset, nCurves uint32) *cmsStage {
	if nCurves > cmsMAXCHANNELS {
		return nil
	}

	// Seek to the offset
	if !io.Seek((*cms_io_handler)(io), offset) {
		return nil
	}

	var curves [cmsMAXCHANNELS]*CmsToneCurve
	for i := uint32(0); i < nCurves; i++ {
		curves[i] = ReadEmbeddedCurve(ar,self, io)
		if curves[i] == nil || !cmsReadAlignment(io) {
			for j := uint32(0); j < i; j++ {
				CmsFreeToneCurve(curves[j])
			}
			return nil
		}
	}

	// Allocate the tone curves stage
	stage := cmsStageAllocToneCurves(ar,self.ContextID, nCurves, curves[:])

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

func TypeLUTB2ARead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
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
	newLUT = cmsPipelineAlloc(ar,self.ContextID, uint32(inputChan), uint32(outputChan))
	if newLUT == nil {
		return nil
	}

	if offsetB != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadSetOfCurves(ar,self, io, baseOffset+offsetB, uint32(inputChan))) {
			goto Error
		}
	}

	if offsetMat != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadMatrix(ar,self, io, baseOffset+offsetMat)) {
			goto Error
		}
	}

	if offsetM != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadSetOfCurves(ar,self, io, baseOffset+offsetM, uint32(inputChan))) {
			goto Error
		}
	}

	if offsetC != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadCLUT(ar,self, io, baseOffset+offsetC, uint32(inputChan), uint32(outputChan))) {
			goto Error
		}
	}

	if offsetA != 0 {
		if !cmsPipelineInsertStage(newLUT, cmsAT_END, ReadSetOfCurves(ar,self, io, baseOffset+offsetA, uint32(outputChan))) {
			goto Error
		}
	}

	*nItems = 1
	return newLUT

Error:
	cmsPipelineFree(ar,newLUT)
	return nil
}

func TypeLUTB2AWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	lut, ok := ptr.(*cmsPipeline)
	if !ok {
		fmt.Printf("Error: not of the type *cmsPipeline\n")
		return false
	}

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
		cmsSignalError(self.ContextID, cmsERROR_NOT_SUITABLE, "LUT is not suitable to be saved as LutBToA")
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
		if !WriteSetOfCurves(ar,self, io, cmsSigParametricCurveType, a) {
			return false
		}
	}

	if clut != nil {
		offsetC = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		precision := uint8(2)
		if lut.SaveAs8Bits {
			precision = 1
		}
		if !WriteCLUT(ar,self, io, precision, clut) {
			return false
		}
	}

	if m != nil {
		offsetM = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteSetOfCurves(ar,self, io, cmsSigParametricCurveType, m) {
			return false
		}
	}

	if matrix != nil {
		offsetMat = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteMatrix(ar,self, io, matrix) {
			return false
		}
	}

	if b != nil {
		offsetB = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		if !WriteSetOfCurves(ar,self, io, cmsSigParametricCurveType, b) {
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

func TypeLUTB2ADup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, nItems uint32) interface{} {
	return cmsPipelineDup(ar,ptr.(*cmsPipeline))
}

func TypeLUTB2AFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsPipelineFree(ar,ptr.(*cmsPipeline))
}

// This is the list of built-in MPE types

// ReadMPEElem reads a single multi-processing element (MPE).
func ReadMPEElem(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, cargo interface{}, n, sizeOfTag uint32) bool {
	var elementSig cmsStageSignature
	var typeHandler *cmsTagTypeHandler
	var nItems uint32
	newLUT, ok := cargo.(*cmsPipeline)
	if !ok {
		fmt.Printf("Error: not of the type *cmsPipeline\n")
		return false
	}

	mpeTypePluginChunk := CmsContextGetClientChunk(self.ContextID, MPEPlugin).(*cmsTagTypePluginChunkType)

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
		str := cmsTagSignature2String(cmsTagSignature(elementSig))
		cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, fmt.Sprintf("Unknown MPE type '%s' found.", str))
		return false
	}

	// If there's no read method, ignore the element
	if typeHandler.ReadFn != nil {
		// Read the MPE and insert it into the pipeline
		stage := typeHandler.ReadFn(ar,self, io, &nItems, sizeOfTag).(*cmsStage)
		if stage == nil || !cmsPipelineInsertStage(newLUT, cmsAT_END, stage) {
			return false
		}
	}

	return true
}

// This is the main dispatcher for MPE

func TypeMPERead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
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
	newLUT = cmsPipelineAlloc(ar,self.ContextID, uint32(inputChans), uint32(outputChans))
	if newLUT == nil {
		return nil
	}

	// Read the element count
	if !cmsReadUInt32Number(io, &elementCount) {
		goto Error
	}

	// Read position table and elements
	if !ReadPositionTable(ar,self, io, elementCount, baseOffset, newLUT, ReadMPEElem) {
		goto Error
	}

	// Verify channel counts
	if inputChans != uint16(newLUT.InputChannels) || outputChans != uint16(newLUT.OutputChannels) {
		goto Error
	}

	*nItems = 1
	return newLUT

Error:
	if newLUT != nil {
		cmsPipelineFree(ar,newLUT)
	}
	*nItems = 0
	return nil
}

// This one is a little bit more complex, so we don't use position tables this time.

func TypeMPEWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	var (
		i, baseOffset, directoryPos, currentPos uint32
		elementOffsets, elementSizes            []uint32
		before, elementCount                    uint32
		elementSig                              cmsStageSignature
		typeHandler                             *cmsTagTypeHandler
		mpeTypePluginChunk                      *cmsTagTypePluginChunkType
		lut                                     *cmsPipeline
		elem                                    *cmsStage
	)

	lut, ok := ptr.(*cmsPipeline)
	if !ok {
		fmt.Printf("Error: not of the type *cmsPipeline\n")
		return false
	}

	baseOffset = uint32(io.Tell((*cms_io_handler)(io))) - uint32(unsafe.Sizeof(cmsTagBase{}))

	// Retrieve input/output channels and element count
	inputChan := cmsPipelineInputChannels(lut)
	outputChan := cmsPipelineOutputChannels(lut)
	elementCount = cmsPipelineStageCount(lut)
	elem = lut.Elements

	// Allocate space for offsets and sizes
	elementOffsets = make([]uint32, elementCount)
	elementSizes = make([]uint32, elementCount)

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
	mpeTypePluginChunk = CmsContextGetClientChunk(self.ContextID, MPEPlugin).(*cmsTagTypePluginChunkType)

	// Write each element
	for i = 0; i < elementCount; i++ {
		elementOffsets[i] = uint32(io.Tell((*cms_io_handler)(io))) - baseOffset
		elementSig = elem.Type

		typeHandler = GetHandler(cmsTagTypeSignature(elementSig), mpeTypePluginChunk.TagTypes, &SupportedMPEtypes[0])
		if typeHandler == nil {
			//cmsTagSignature2String(cmsTagSignature(elementSig))
			cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Found unknown MPE type")
			goto Error
		}

		if !cmsWriteUInt32Number(io, uint32(elementSig)) || !cmsWriteUInt32Number(io, 0) {
			goto Error
		}

		before = uint32(io.Tell((*cms_io_handler)(io)))
		if !typeHandler.WriteFn(ar,self, io, elem, 1) {
			goto Error
		}

		if !cmsWriteAlignment(io) {
			goto Error
		}
		elementSizes[i] = uint32(io.Tell((*cms_io_handler)(io))) - before
		elem = elem.Next
	}

	// Write directory with actual offsets and sizes
	currentPos = uint32(io.Tell((*cms_io_handler)(io)))
	if !io.Seek((*cms_io_handler)(io), directoryPos) {
		goto Error
	}

	for i = 0; i < elementCount; i++ {
		if !cmsWriteUInt32Number(io, elementOffsets[i]) || !cmsWriteUInt32Number(io, elementSizes[i]) {
			goto Error
		}
	}

	if !io.Seek((*cms_io_handler)(io), currentPos) {
		goto Error
	}
	return true

Error:
	return false
}

func TypeMPEDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, nItems uint32) interface{} {
	return cmsPipelineDup(ar,ptr.(*cmsPipeline))
}

func TypeMPEFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsPipelineFree(ar,ptr.(*cmsPipeline))
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

// Read an MLUC element
func ReadOneMLUC(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, e *cmsDICelem, i uint32, mlu **cmsMLU) bool {
	if e.Offsets[i] == 0 || e.Sizes[i] == 0 {
		*mlu = nil
		return true
	}

	if !io.Seek((*cms_io_handler)(io), e.Offsets[i]) {
		return false
	}
	var nItems uint32
	*mlu = TypeMLURead(ar,self, io, &nItems, e.Sizes[i]).(*cmsMLU)
	return *mlu != nil
}

// Write an MLUC element
func WriteOneMLUC(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, e *cmsDICelem, i uint32, mlu *cmsMLU, baseOffset uint32) bool {
	if mlu == nil {
		e.Sizes[i] = 0
		e.Offsets[i] = 0
		return true
	}

	before := uint32(io.Tell((*cms_io_handler)(io)))
	e.Offsets[i] = before - baseOffset

	if !TypeMLUWrite(ar,self, io, mlu, 1) {
		return false
	}

	e.Sizes[i] = uint32(io.Tell((*cms_io_handler)(io))) - before
	return true
}

func TypeDictionaryRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var (
		hDict           CmsHANDLE
		count, length   uint32
		baseOffset      uint32
		a               cmsDICarray
		displayNameMLU  *cmsMLU
		displayValueMLU *cmsMLU
		rc              bool
		signedSizeOfTag = int32(sizeOfTag)
	)

	*nItems = 0
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
		cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, fmt.Sprintf("Unknown record length in dictionary"))
		return nil
	}

	// Create an empty dictionary
	hDict = cmsDictAlloc(ar,self.ContextID)
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

		if length > 16 && !ReadOneMLUC(ar,self, io, &a.DisplayName, i, &displayNameMLU) {
			goto Error
		}
		if length > 24 && !ReadOneMLUC(ar,self, io, &a.DisplayValue, i, &displayValueMLU) {
			goto Error
		}

		if nameWCS == "" || valueWCS == "" {
			cmsSignalError(self.ContextID, cmsERROR_CORRUPTION_DETECTED, "Bad dictionary Name/Value")
			rc = false
		} else {
			rc = cmsDictAddEntry(ar,hDict, nameWCS, valueWCS, displayNameMLU, displayValueMLU)
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
	return hDict

Error:
	FreeArray(&a)
	if hDict != nil {
		cmsDictFree(hDict)
	}
	return nil
}

func TypeDictionaryWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	var (
		hDict             CmsHANDLE
		count, length     uint32
		directoryPos      uint32
		currentPos        uint32
		baseOffset        uint32
		anyName, anyValue bool
		p                 *cmsDICTentry
		a                 cmsDICarray
	)

	hDict = CmsHANDLE(ptr)
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

		if p.DisplayName != nil && !WriteOneMLUC(ar,self, io, &a.DisplayName, i, p.DisplayName, baseOffset) {
			goto Error
		}
		if p.DisplayValue != nil && !WriteOneMLUC(ar,self, io, &a.DisplayValue, i, p.DisplayValue, baseOffset) {
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

func TypeDictionaryDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, nItems uint32) interface{} {
	return cmsDictDup(ar,CmsHANDLE(ptr))
}

func TypeDictionaryFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsDictFree(CmsHANDLE(ptr))
}

// Read a video signal tag
func TypeVideoSignalRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	if sizeOfTag != 8 {
		return nil
	}

	// Skip unused uint32
	if !cmsReadUInt32Number(io, nil) {
		return nil
	}

	// Allocate memory for cmsVideoSignalType
	cicp := allocateStruct[cmsVideoSignalType](ar)
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
	return cicp

Error:
	if cicp != nil {
		cmsFree(self.ContextID, cicp)
	}
	return nil
}

// Write a video signal tag
func TypeVideoSignalWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	cicp, ok := ptr.(*cmsVideoSignalType)
	if !ok {
		fmt.Printf("Error: not of the type *cmsVideoSignal\n")
		return false
	}

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
func TypeVideoSignalDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	orig := ptr.(*cmsVideoSignalType) // Type assertion
	copy := *orig                     // Struct copy by value
	return &copy                      // Return pointer to the new copy
}

// Free a video signal tag
func TypeVideoSignalFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
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

func TypeVcgtRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var tagType uint32
	if !cmsReadUInt32Number(io, &tagType) {
		return nil
	}

	var curves [3]*CmsToneCurve
	switch tagType {
	case cmsVideoCardGammaTableType:
		var nChannels, nElems, nBytes uint16

		if !cmsReadUInt16Number(io, &nChannels) || nChannels != 3 {
			cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unsupported number of channels for VCGT")
			goto Error
		}

		if !cmsReadUInt16Number(io, &nElems) || !cmsReadUInt16Number(io, &nBytes) {
			goto Error
		}

		for i := 0; i < 3; i++ {
			curves[i] = cmsBuildTabulatedToneCurve16(ar,self.ContextID, uint32(nElems), nil)
			switch nBytes {
			case 1:
				var v uint8
				for j := 0; j < int(nElems); j++ {
					if !cmsReadUInt8Number(io, &v) {
						goto Error
					}
					curves[i].Table16[j] = uint16(v) * 257
				}
			case 2:
				if !cmsReadUInt16Array(io, uint32(nElems), curves[i].Table16) {
					goto Error
				}
			default:
				cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unsupported bit depth for VCGT")
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
			curves[i] = cmsBuildParametricToneCurve(ar,self.ContextID, 5, params)
		}
	default:
		cmsSignalError(self.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unsupported tag type for VCGT")
		goto Error
	}

	*nItems = 1
	return &curves

Error:
	cmsFreeToneCurveTriple(curves)
	return nil
}
func TypeVcgtWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	curves, ok := ptr.([]*CmsToneCurve)
	if !ok {
		cmsSignalError(nil, cmsERROR_RANGE, "Ptr is not of type []*CmsToneCurve")
		return false // or nil, or 0, depending on context
	}

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

			gamma := segments[0].Params[0]
			min := segments[0].Params[5]
			max := math.Pow(segments[0].Params[1], gamma) + min

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

func TypeVcgtDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	oldCurves := ptr.([]*CmsToneCurve)
	NewCurves := make([]*CmsToneCurve, 3)
	if NewCurves == nil {
		return nil
	}

	NewCurves[0] = cmsDupToneCurve(ar,oldCurves[0])
	NewCurves[1] = cmsDupToneCurve(ar,oldCurves[1])
	NewCurves[2] = cmsDupToneCurve(ar,oldCurves[2])
	return &NewCurves[0]
}

func TypeVcgtFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	curvesSlice, ok := ptr.([]*CmsToneCurve)
	if !ok || len(curvesSlice) != 3 {
		return
	}

	// Convert slice to fixed array
	var curvesArray [3]*CmsToneCurve
	copy(curvesArray[:], curvesSlice)

	cmsFreeToneCurveTriple(curvesArray)
}

// ********************************************************************************
// Type cmsSigMultiProcessElementType
// ********************************************************************************

func GenericMPEDup(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}, n uint32) interface{} {
	//fmt.Println("GenericMPEDup")
	return cmsStageDup(ar,ptr.(*cmsStage))

}

func GenericMPEFree(ar *arena.Arena, self *cmsTagTypeHandler, ptr interface{}) {
	cmsStageFree(ar,ptr.(*cmsStage))
}

// Each curve is stored in one or more curve segments, with break-points specified between curve segments.
// The first curve segment always starts at -Infinity, and the last curve segment always ends at +Infinity. The
// first and last curve segments shall be specified in terms of a formula, whereas the other segments shall be
// specified either in terms of a formula, or by a sampled curve.

// ReadSegmentedCurve reads an embedded segmented curve
func ReadSegmentedCurve(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER) *CmsToneCurve {
	var elementSig cmsCurveSegSignature
	var nSegments uint16
	var prevBreak float32 = float32(math.Inf(-1)) // -Infinity

	// Read element signature
	if !cmsReadUInt32Number(io, (*uint32)(&elementSig)) {
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
		if !cmsReadUInt32Number(io, (*uint32)(&elementSig)) || !cmsReadUInt32Number(io, nil) {
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
			segments[i].SampledPoints = make([]float32, count)

			// Initialize the first point
			segments[i].SampledPoints[0] = 0

			// Populate the array using the slice
			for j := uint32(1); j < count; j++ {
				if !cmsReadFloat32Number(io, &segments[i].SampledPoints[j]) {
					return nil
				}
			}

		default:
			return nil
		}
	}

	curve := cmsBuildSegmentedToneCurve(ar,self.ContextID, uint32(nSegments), segments)

	// Fix implicit points
	for i := uint32(0); i < uint32(nSegments); i++ {
		if curve.Segments[i].Type == 0 {
			curve.Segments[i].SampledPoints[0] = cmsEvalToneCurveFloat(curve, curve.Segments[i].X0)
		}
	}

	return curve
}

// ReadMPECurve reads a single curve for MPE
func ReadMPECurve(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, cargo interface{}, n, sizeOfTag uint32) bool {
	gammaTables := cargo.([]*CmsToneCurve)
	gammaTables[n] = ReadSegmentedCurve(ar,self, io)
	return gammaTables[n] != nil
}

// Type_MPEcurve_Read reads MPE curve type
func TypeMPEcurveRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var inputChans, outputChans uint16
	baseOffset := io.Tell((*cms_io_handler)(io)) - uint32(unsafe.Sizeof(cmsTagBase{}))

	if !cmsReadUInt16Number(io, &inputChans) || !cmsReadUInt16Number(io, &outputChans) {
		return nil
	}

	if inputChans != outputChans {
		return nil
	}

	gammaTables := make([]*CmsToneCurve, inputChans)

	var mpe *cmsStage
	// Read position table and allocate the MPE curve stage
	if ReadPositionTable(ar,self, io, uint32(inputChans), baseOffset, gammaTables, ReadMPECurve) {
		mpe = cmsStageAllocToneCurves(ar,self.ContextID, uint32(inputChans), gammaTables)

	}

	// Free allocated resources in case of error
	for i := uint32(0); i < uint32(inputChans); i++ {
		if gammaTables[i] != nil {
			CmsFreeToneCurve(gammaTables[i])
		}
	}

	if mpe != nil {
		*nItems = 1
	} else {
		*nItems = 0
	}
	return mpe
}

// WriteSegmentedCurve writes a single segmented curve
func WriteSegmentedCurve(io *cmsIOHANDLER, curve *CmsToneCurve) bool {
	nSegments := curve.nSegments
	segments := curve.Segments

	if !cmsWriteUInt32Number(io, uint32(cmsSigSegmentedCurve)) || !cmsWriteUInt32Number(io, 0) ||
		!cmsWriteUInt16Number(io, uint16(nSegments)) || !cmsWriteUInt16Number(io, 0) {
		return false
	}
	// Write breakpoints
	for i := uint32(0); i < nSegments-1; i++ {
		if !cmsWriteFloat32Number(io, segments[i].X1) {
			return false
		}
	}

	// Write each segment
	for i := uint32(0); i < nSegments; i++ {
		actualSeg := segments[i]
		switch actualSeg.Type {
		case 0: // Sampled curve
			if !cmsWriteUInt32Number(io, uint32(cmsSigSampledCurveSeg)) || !cmsWriteUInt32Number(io, 0) ||
				!cmsWriteUInt32Number(io, actualSeg.NGridPoints-1) {
				return false
			}

			for j := uint32(1); j < actualSeg.NGridPoints; j++ {

				if !cmsWriteFloat32Number(io, actualSeg.SampledPoints[j]) {
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
func WriteMPECurve(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, cargo interface{}, n, sizeOfTag uint32) bool {
	curves, ok := cargo.(*cmsStageToneCurvesData)
	if !ok {
		fmt.Printf("Error: not of the type *cmsStageToneCurvesData\n")
		return false
	}

	return WriteSegmentedCurve(io, curves.TheCurves[n])
}

// Type_MPEcurve_Write writes the MPE curve type
func TypeMPEcurveWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	mpe, ok := ptr.(*cmsStage)
	curves, ok := mpe.Data.(*cmsStageToneCurvesData)
	if !ok {
		fmt.Printf("Error: not of the type *cmsStageToneCurvesData\n")
		return false
	}

	baseOffset := io.Tell((*cms_io_handler)(io)) - uint32(unsafe.Sizeof(cmsTagBase{}))

	// Write header
	if !cmsWriteUInt16Number(io, uint16(mpe.InputChannels)) {
		return false
	}
	if !cmsWriteUInt16Number(io, uint16(mpe.InputChannels)) {
		return false
	}

	// Write position table
	return WritePositionTable(ar,self, io, 0, uint32(mpe.InputChannels), baseOffset, curves, WriteMPECurve)
}
func TypeMPEmatrixRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var inputChans, outputChans uint16
	if !cmsReadUInt16Number(io, &inputChans) || !cmsReadUInt16Number(io, &outputChans) {
		return nil
	}

	if inputChans >= cmsMAXCHANNELS || outputChans >= cmsMAXCHANNELS {
		return nil
	}

	nElems := uint32(inputChans) * uint32(outputChans)
	matrix := make([]float64, nElems)
	offsets := make([]float64, outputChans)
	for i := uint32(0); i < nElems; i++ {
		var v float32
		if !cmsReadFloat32Number(io, &v) {
			return nil
		}
		matrix[i] = float64(v)
	}

	for i := uint32(0); i < uint32(outputChans); i++ {
		var v float32
		if !cmsReadFloat32Number(io, &v) {
			return nil
		}
		offsets[i] = float64(v)
	}

	mpe := cmsStageAllocMatrix(ar,self.ContextID, uint32(outputChans), uint32(inputChans), matrix, offsets)
	*nItems = 1
	return mpe
}

func TypeMPEmatrixWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	mpe, ok := ptr.(*cmsStage)
	matrix, ok := mpe.Data.(*cmsStageMatrixData)
	if !ok {
		fmt.Printf("Error: not of the type *cmsStageMatrixData\n")
		return false
	}

	if !cmsWriteUInt16Number(io, uint16(mpe.InputChannels)) || !cmsWriteUInt16Number(io, uint16(mpe.OutputChannels)) {
		return false
	}

	nElems := mpe.InputChannels * mpe.OutputChannels

	for i := uint32(0); i < nElems; i++ {
		if !cmsWriteFloat32Number(io, float32(matrix.Double[i])) {
			return false
		}
	}

	for i := uint32(0); i < mpe.OutputChannels; i++ {
		if matrix.Offset == nil {
			if !cmsWriteFloat32Number(io, 0) {
				return false
			}
		} else {
			if !cmsWriteFloat32Number(io, float32(matrix.Offset[i])) {
				return false
			}
		}
	}
	return true
}

func TypeMPEclutRead(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, nItems *uint32, sizeOfTag uint32) interface{} {
	var inputChans, outputChans uint16
	var dimensions8 [16]byte
	var nMaxGrids uint32
	var gridPoints [MAX_INPUT_DIMENSIONS]uint32

	if !cmsReadUInt16Number(io, &inputChans) || !cmsReadUInt16Number(io, &outputChans) || inputChans == 0 || outputChans == 0 {
		return nil
	}

	if io.Read((*cms_io_handler)(io), dimensions8[:], uint32(unsafe.Sizeof(uint8(0))), 16) != 16 {
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

	mpe := cmsStageAllocCLutFloatGranular(ar,self.ContextID, gridPoints[:], uint32(inputChans), uint32(outputChans), nil)
	if mpe == nil {
		return nil
	}

	clut := mpe.Data.(*cmsStageCLutData)
	for i := uint32(0); i < clut.NEntries; i++ {
		if !cmsReadFloat32Number(io, &clut.Tab.([]float32)[i]) {
			cmsStageFree(ar,mpe)
			return nil
		}
	}

	*nItems = 1
	return mpe
}

func TypeMPEclutWrite(ar *arena.Arena, self *cmsTagTypeHandler, io *cmsIOHANDLER, ptr interface{}, nItems uint32) bool {
	mpe := ptr.(*cmsStage)
	clut := mpe.Data.(*cmsStageCLutData)

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

	if io.Write((*cms_io_handler)(io), 16, dimensions8[:]) {
		return false
	}
	for i := uint32(0); i < clut.NEntries; i++ {
		if !cmsWriteFloat32Number(io, clut.Tab.([]float32)[i]) {
			return false
		}
	}

	return true
}

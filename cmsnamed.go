package golcms

import (
	//"encoding/binary"
	//"unsafe"
	"fmt"
)

// cmsMLUalloc allocates an empty multi-localized unicode object.
func cmsMLUalloc(ContextID CmsContext, nItems uint32) *cmsMLU {
	if nItems <= 0 {
		nItems = 2
	}

	mlu := allocateStruct[cmsMLU]()
	if mlu == nil {
		return nil
	}

	mlu.ContextID = ContextID

	mlu.Entries = make([]cmsMLUentry, nItems)
	mlu.AllocatedEntries = nItems
	mlu.UsedEntries = 0

	return mlu
}

// GrowMLUpool grows the memory pool for an MLU. Pool size is doubled on each call.
func GrowMLUpool(mlu *cmsMLU) bool {
	if mlu == nil {
		return false
	}

	size := uint32(256)
	if mlu.PoolSize > 0 {
		size = mlu.PoolSize * 2
	}

	if size < mlu.PoolSize {
		return false
	}

	newPtr := cmsRealloc(mlu.ContextID, mlu.MemPool, size)
	if newPtr == nil {
		return false
	}

	mlu.MemPool = newPtr
	mlu.PoolSize = size

	return true
}

// GrowMLUtable grows the entry table for an MLU. Table size is doubled on each call.
func GrowMLUtable(mlu *cmsMLU) bool {
	if mlu == nil {
		return false
	}

	// Calculate the new size
	allocatedEntries := mlu.AllocatedEntries * 2
	if allocatedEntries/2 != mlu.AllocatedEntries {
		return false // Overflow check
	}

	// Create a new slice with the increased capacity
	newEntries := make([]cmsMLUentry, allocatedEntries)

	// Copy old entries into the new slice
	copy(newEntries, mlu.Entries)

	// Assign the new slice back
	mlu.Entries = newEntries
	mlu.AllocatedEntries = allocatedEntries

	return true
}

// SearchMLUEntry searches for a specific entry in the MLU based on language and country codes.
func SearchMLUEntry(mlu *cmsMLU, LanguageCode, CountryCode uint16) int {
	if mlu == nil {
		return -1
	}
	for i := uint32(0); i < mlu.UsedEntries; i++ {
		// Compare the fields
		if mlu.Entries[i].Country == CountryCode && mlu.Entries[i].Language == LanguageCode {
			return int(i)
		}
	}
	return -1
}

// AddMLUBlock adds a block of characters to the MLU for a specific language and country.
func AddMLUBlock(mlu *cmsMLU, block []uint16, LanguageCode, CountryCode uint16) bool {
	size := uint32(len(block) * 2)

	if mlu == nil {
		return false
	}
	if mlu.UsedEntries >= mlu.AllocatedEntries {
		if !GrowMLUtable(mlu) {
			return false
		}
	}
	if SearchMLUEntry(mlu, LanguageCode, CountryCode) >= 0 {
		return false
	}
	for (mlu.PoolSize - mlu.PoolUsed) < size {
		if !GrowMLUpool(mlu) {
			return false
		}
	}

	offset := mlu.PoolUsed
	ptr := mlu.MemPool.([]uint8)
	if ptr == nil {
		return false
	}

	// Encode uint16 slice as little-endian []byte
	for i, v := range block {
		ptr[offset+2*uint32(i)] = byte(v)
		ptr[offset+2*uint32(i)+1] = byte(v >> 8)
	}

	mlu.PoolUsed += size

	// Record entry
	entry := &mlu.Entries[mlu.UsedEntries]
	entry.StrW = offset
	entry.Len = size
	entry.Country = CountryCode
	entry.Language = LanguageCode
	mlu.UsedEntries++

	return true
}

// strTo16 converts a 3-char code to a cmsUInt16Number.
func strTo16(str string) uint16 {
	if len(str) < 2 {
		return 0
	}
	return uint16(str[0])<<8 | uint16(str[1])
}

// strFrom16 converts a cmsUInt16Number to a 3-char string.
func strFrom16(n uint16) string {
	return string([]byte{byte(n >> 8), byte(n), 0})
}

// cmsMLUsetASCII adds an ASCII entry to an MLU.
func cmsMLUsetASCII(mlu *cmsMLU, languageCode, countryCode string, asciiStr string) bool {
	if mlu == nil {
		return false
	}

	lang := strTo16(languageCode)
	country := strTo16(countryCode)

	if asciiStr == "" {
		asciiStr = "\x00"
	}

	// Convert ASCII string to UTF-16LE []uint16
	wStr := make([]uint16, len(asciiStr))
	for i, b := range []byte(asciiStr) {
		wStr[i] = uint16(b) // zero-extend ASCII into UTF-16
	}

	return AddMLUBlock(mlu, wStr, lang, country)
}

// cmsMLUsetWide adds a wide string entry to an MLU.
func cmsMLUsetWide(mlu *cmsMLU, Language, Country string, WideString []uint16) bool {
	if mlu == nil || WideString == nil {
		return false
	}

	lang := strTo16(Language)
	country := strTo16(Country)
	lenWide := uint32(len(WideString) * 2)

	if lenWide == 0 {
		lenWide = 2
	}

	return AddMLUBlock(mlu, WideString, lang, country)
}

// cmsMLUdup duplicates an MLU.
func cmsMLUdup(mlu *cmsMLU) *cmsMLU {
	if mlu == nil {
		return nil
	}

	newMLU := cmsMLUalloc(mlu.ContextID, mlu.UsedEntries)
	if newMLU == nil {
		return nil
	}

	if newMLU.AllocatedEntries < mlu.UsedEntries ||
		newMLU.Entries == nil || mlu.Entries == nil {
		cmsMLUfree(newMLU)
		return nil
	}

	copy(newMLU.Entries[:mlu.UsedEntries], mlu.Entries[:mlu.UsedEntries])
	newMLU.UsedEntries = mlu.UsedEntries

	if mlu.PoolUsed == 0 {
		newMLU.MemPool = nil
	} else {
		pool := make([]byte, mlu.PoolUsed)
		src, ok := mlu.MemPool.([]byte)
		if !ok || len(src) < int(mlu.PoolUsed) {
			cmsMLUfree(newMLU)
			return nil
		}
		copy(pool, src[:mlu.PoolUsed])
		newMLU.MemPool = pool
	}

	newMLU.PoolSize = mlu.PoolUsed
	newMLU.PoolUsed = mlu.PoolUsed

	return newMLU
}

// cmsMLUfree frees all memory used by an MLU.
func cmsMLUfree(mlu *cmsMLU) {
	if mlu != nil {
		if mlu.MemPool != nil {
			cmsFree(mlu.ContextID, mlu.MemPool)
		}
		cmsFree(mlu.ContextID, mlu)
	}
}

// cmsMLUgetWide searches for an entry in the MLU object and retrieves the wide string.
func _cmsMLUgetWide(
	mlu *cmsMLU,
	length *uint32,
	LanguageCode, CountryCode uint16,
	UsedLanguageCode, UsedCountryCode *uint16,
) []uint16 {
	if mlu == nil || mlu.AllocatedEntries == 0 || mlu.MemPool == nil {
		return nil
	}

	memPool, ok := mlu.MemPool.([]uint8)
	if !ok {
		return nil
	}

	bestMatch := -1
	for i := uint32(0); i < mlu.UsedEntries; i++ {
		entry := mlu.Entries[i]

		if entry.Language == LanguageCode {
			if bestMatch == -1 {
				bestMatch = int(i)
			}
			if entry.Country == CountryCode {
				if UsedLanguageCode != nil {
					*UsedLanguageCode = entry.Language
				}
				if UsedCountryCode != nil {
					*UsedCountryCode = entry.Country
				}
				if length != nil {
					*length = entry.Len
				}
				if entry.StrW+entry.Len > mlu.PoolSize {
					return nil
				}
				return bytesToUint16Slice(memPool[entry.StrW : entry.StrW+entry.Len])
			}
		}
	}

	// No string found. Return First one
	if bestMatch == -1 {
		bestMatch = 0
	}

	entry := mlu.Entries[bestMatch]
	if UsedLanguageCode != nil {
		*UsedLanguageCode = entry.Language
	}
	if UsedCountryCode != nil {
		*UsedCountryCode = entry.Country
	}
	if length != nil {
		*length = entry.Len
	}
	if entry.StrW+entry.Len > mlu.PoolSize {
		return nil
	}
	return bytesToUint16Slice(memPool[entry.StrW : entry.StrW+entry.Len])
}
func cmsMLUgetWide(
	mlu *cmsMLU,
	LanguageCode string,
	CountryCode string,
	Buffer []uint16,
	BufferSize uint32,
) uint32 {
	var strLen uint32

	lang := strTo16(LanguageCode)
	country := strTo16(CountryCode)

	if mlu == nil {
		return 0
	}

	wStr := _cmsMLUgetWide(mlu, &strLen, lang, country, nil, nil)
	if wStr == nil {
		return 0
	}

	if Buffer == nil {
		return strLen + 2
	}

	if BufferSize == 0 {
		return 0
	}

	n := strLen / 2
	if BufferSize < strLen+2 {
		n = (BufferSize - 2) / 2
	}

	for i := uint32(0); i < n; i++ {
		Buffer[i] = wStr[i]
	}
	Buffer[n] = 0

	return strLen + 2
}

// cmsMLUgetASCII retrieves the ASCII string for a specific language and country.
func cmsMLUgetASCII(
	mlu *cmsMLU,
	LanguageCode, CountryCode string,
	buffer []byte,
	bufferSize uint32,
) uint32 {
	lang := strTo16(LanguageCode)
	country := strTo16(CountryCode)

	var strLen uint32
	wide := _cmsMLUgetWide(mlu, &strLen, lang, country, nil, nil)
	if wide == nil {
		return 0
	}

	asciiLen := strLen / 2

	// If buffer is nil, just return required size (including null terminator)
	if buffer == nil {
		return asciiLen + 1
	}

	if bufferSize == 0 {
		return 0
	}

	// Adjust length to fit in buffer including null terminator
	if bufferSize < asciiLen+1 {
		asciiLen = bufferSize - 1
	}

	for i := uint32(0); i < asciiLen; i++ {
		if wide[i] == 0 {
			buffer[i] = 0
		} else {
			buffer[i] = byte(wide[i])
		}
	}

	// Null-terminate
	buffer[asciiLen] = 0

	return asciiLen + 1
}

// cmsMLUgetTranslation retrieves the language and country used for the translation.
func cmsMLUgetTranslation(mlu *cmsMLU, LanguageCode, CountryCode string, ObtainedLanguage, ObtainedCountry *string) bool {
	lang := strTo16(LanguageCode)
	country := strTo16(CountryCode)

	var usedLang, usedCountry uint16
	wide := _cmsMLUgetWide(mlu, nil, lang, country, &usedLang, &usedCountry)
	if wide == nil {
		return false
	}

	if ObtainedLanguage != nil {
		*ObtainedLanguage = strFrom16(usedLang)
	}
	if ObtainedCountry != nil {
		*ObtainedCountry = strFrom16(usedCountry)
	}
	return true
}

// cmsMLUtranslationsCount returns the number of translations in the MLU object.
func cmsMLUtranslationsCount(mlu *cmsMLU) uint32 {
	if mlu == nil {
		return 0
	}
	return mlu.UsedEntries
}

// cmsMLUtranslationsCodes retrieves the language and country codes for a specific index in the MLU.
func cmsMLUtranslationsCodes(mlu *cmsMLU, idx uint32, LanguageCode, CountryCode *string) bool {
	if mlu == nil || idx >= mlu.UsedEntries {
		return false
	}
	// Calculate the address of the current entry
	entry := mlu.Entries[idx]
	// Cast the calculated address back to a *cmsMLUentry
	if LanguageCode != nil {
		*LanguageCode = strFrom16(entry.Language)
	}
	if CountryCode != nil {
		*CountryCode = strFrom16(entry.Country)
	}
	return true
}

// GrowNamedColorList grows the list to accommodate at least NumElements.
// GrowNamedColorList expands the capacity of the named color list dynamically.
func GrowNamedColorList(v *cmsNAMEDCOLORLIST) bool {
	if v == nil {
		return false
	}

	var newSize uint32
	if v.Allocated == 0 {
		newSize = 64 // Initial guess
	} else {
		newSize = v.Allocated * 2
	}

	// Limit the maximum size of the list
	if newSize > 1024*100 {
		v.List = nil
		return false
	}

	// Extend the slice to the new size
	if len(v.List) < int(newSize) {
		newList := make([]cmsNAMEDCOLOR, newSize)
		copy(newList, v.List) // Preserve existing elements
		v.List = newList
	}

	v.Allocated = newSize
	return true
}

// cmsAllocNamedColorList allocates a list for n elements.
func cmsAllocNamedColorList(ContextID CmsContext, n, ColorantCount uint32, Prefix, Suffix string) *cmsNAMEDCOLORLIST {
	if ColorantCount > cmsMAXCHANNELS {
		return nil
	}

	v := allocateStruct[cmsNAMEDCOLORLIST]()
	if v == nil {
		return nil
	}

	v.List = nil
	v.nColors = 0
	v.ContextID = ContextID

	// Grow the list to fit the required number of elements
	for v.Allocated < n {
		if !GrowNamedColorList(v) {
			cmsFreeNamedColorList(v)
			return nil
		}
	}

	// Set Prefix and Suffix
	copy(v.Prefix[:], Prefix)
	copy(v.Suffix[:], Suffix)
	v.Prefix[len(v.Prefix)-1] = 0
	v.Suffix[len(v.Suffix)-1] = 0

	v.ColorantCount = ColorantCount

	return v
}

// cmsFreeNamedColorList frees a named color list.
func cmsFreeNamedColorList(v *cmsNAMEDCOLORLIST) {
	if v == nil {
		return
	}
	cmsFree(v.ContextID, v)
}

// cmsDupNamedColorList duplicates a named color list.
func cmsDupNamedColorList(v *cmsNAMEDCOLORLIST) *cmsNAMEDCOLORLIST {
	if v == nil {
		return nil
	}

	newNC := cmsAllocNamedColorList(v.ContextID, v.nColors, v.ColorantCount, string(v.Prefix[:]), string(v.Suffix[:]))
	if newNC == nil {
		return nil
	}

	// Ensure the allocated size matches
	for newNC.Allocated < v.Allocated {
		if !GrowNamedColorList(newNC) {
			cmsFreeNamedColorList(newNC)
			return nil
		}
	}

	// Copy data
	//this is already copied in cmsAllocNamedColorList
	//copy(newNC.Prefix[:], v.Prefix[:])
	//copy(newNC.Suffix[:], v.Suffix[:])
	newNC.ColorantCount = v.ColorantCount

	MemcpySlice(newNC.List, v.List, int(v.nColors))
	newNC.nColors = v.nColors

	return newNC
}

// FreeNamedColorList releases the resources for the named color list.
func FreeNamedColorList(mpe *cmsStage) {
	list := mpe.Data.(*cmsNAMEDCOLORLIST)
	cmsFreeNamedColorList(list)
}

// DupNamedColorList duplicates the named color list.
func DupNamedColorList(mpe *cmsStage) interface{} {
	list, ok := mpe.Data.(*cmsNAMEDCOLORLIST)
	if !ok {
		fmt.Printf("Error: Interface data assertion error, not *cmsNAMEDCOLORLIST\n")
		return nil
	}
	return cmsDupNamedColorList(list)
}

// EvalNamedColorPCS evaluates the named color in PCS (Profile Connection Space).

// EvalNamedColorPCS evaluates named color in PCS (Lab) space.
func EvalNamedColorPCS(in []float32, out []float32, mpe *cmsStage) {
	NamedColorList, ok := mpe.Data.(*cmsNAMEDCOLORLIST)
	if !ok {
		fmt.Printf("Error: Interface data assertion error, not *cmsNAMEDCOLORLIST\n")
		return
	}
	index := uint16(cmsQuickSaturateWord(float64(in[0]) * 65535.0))
	// Interpret the `List` pointer as a slice of cmsNAMEDCOLOR.

	if uint32(index) >= NamedColorList.nColors {
		cmsSignalError(NamedColorList.ContextID, cmsERROR_RANGE, "Color %d out of range")
		out[0] = 0.0
		out[1] = 0.0
		out[2] = 0.0
	} else {

		// Named color always uses Lab
		out[0] = float32(NamedColorList.List[index].PCS[0] / 65535.0)
		out[1] = float32(NamedColorList.List[index].PCS[1] / 65535.0)
		out[2] = float32(NamedColorList.List[index].PCS[2] / 65535.0)
	}
}

// EvalNamedColor evaluates named color in device colorant space.
func EvalNamedColor(in []float32, out []float32, mpe *cmsStage) {
	namedColorList, ok := mpe.Data.(*cmsNAMEDCOLORLIST)
	if !ok {
		fmt.Printf("Error: Interface data assertion error, not *cmsNAMEDCOLORLIST\n")
		return
	}
	index := uint16(cmsQuickSaturateWord(float64(in[0]) * 65535.0))

	if uint32(index) >= namedColorList.nColors {
		cmsSignalError(namedColorList.ContextID, cmsERROR_RANGE, "Color out of range")

		// Zero-out the output for all colorants.
		for j := uint32(0); j < namedColorList.ColorantCount; j++ {
			out[j] = 0.0
		}
	} else {
		// Access DeviceColorant values for the selected color.
		for j := uint32(0); j < namedColorList.ColorantCount; j++ {
			out[j] = float32(namedColorList.List[index].DeviceColorant[j] / 65535.0)
		}
	}
}

// Named color lookup element
// _cmsStageAllocNamedColor allocates a named color lookup element.
func cmsStageAllocNamedColor(namedColorList *cmsNAMEDCOLORLIST, usePCS bool) *cmsStage {
	// Determine the output channel count based on the `usePCS` condition.
	outputChannels := uint32(1)
	if usePCS {
		outputChannels = 3
	} else {
		outputChannels = namedColorList.ColorantCount
	}

	// Select the evaluation function based on the `usePCS` condition.
	var evalFunc cmsStageEvalFn
	if usePCS {
		evalFunc = EvalNamedColorPCS
	} else {
		evalFunc = EvalNamedColor
	}

	// Allocate the placeholder stage.
	return cmsStageAllocPlaceholder(
		namedColorList.ContextID,
		cmsSigNamedColorElemType,
		1,                                    // Input channels are always 1.
		outputChannels,                       // Output channels depend on `usePCS`.
		evalFunc,                             // Evaluation function depends on `usePCS`.
		DupNamedColorList,                    // Duplication function.
		FreeNamedColorList,                   // Freeing function.
		cmsDupNamedColorList(namedColorList), // Duplicate the named color list.
	)
}

func cmsAppendNamedColor(namedColorList *cmsNAMEDCOLORLIST, name string, PCS *[3]uint16, Colorant *[cmsMAXCHANNELS]uint16) bool {
	if namedColorList == nil {
		return false
	}

	// Grow the list if necessary
	if namedColorList.nColors+1 > namedColorList.Allocated {
		if !GrowNamedColorList(namedColorList) {
			return false
		}
	}

	// Calculate the index for the new color
	index := namedColorList.nColors

	// Access the list element
	entry := namedColorList.List[index]

	// Copy Colorant data
	for i := uint32(0); i < namedColorList.ColorantCount; i++ {
		if Colorant != nil {
			entry.DeviceColorant[i] = Colorant[i]
		} else {
			entry.DeviceColorant[i] = 0
		}
	}

	// Copy PCS data
	for i := 0; i < 3; i++ {
		if PCS != nil {
			entry.PCS[i] = PCS[i]
		} else {
			entry.PCS[i] = 0
		}
	}

	// Copy the name
	if name != "" {
		copy(entry.Name[:], name)
		entry.Name[len(entry.Name)-1] = 0
	} else {
		entry.Name[0] = 0
	}

	// Increment the number of colors
	namedColorList.nColors++
	return true
}

func cmsNamedColorCount(namedColorList *cmsNAMEDCOLORLIST) uint32 {
	if namedColorList == nil {
		return 0
	}
	return namedColorList.nColors
}

func cmsNamedColorInfo(namedColorList *cmsNAMEDCOLORLIST, nColor uint32, name, prefix, suffix []byte, pcs, colorant []uint16) bool {
	if namedColorList == nil || nColor >= cmsNamedColorCount(namedColorList) {
		return false
	}

	// Access the specific color entry
	colorEntry := namedColorList.List[nColor]

	// Copy Name, Prefix, and Suffix safely
	copy(name, colorEntry.Name[:])
	copy(prefix, namedColorList.Prefix[:])
	copy(suffix, namedColorList.Suffix[:])

	// Copy PCS (3 elements)
	if len(pcs) >= 3 {
		copy(pcs[:3], colorEntry.PCS[:3])
	}

	// Copy Colorant values
	colorantCount := int(namedColorList.ColorantCount)
	if len(colorant) >= colorantCount {
		copy(colorant[:colorantCount], colorEntry.DeviceColorant[:colorantCount])
	}

	return true
}

func cmsNamedColorIndex(namedColorList *cmsNAMEDCOLORLIST, name *byte) int32 {
	if namedColorList == nil {
		return -1
	}

	n := cmsNamedColorCount(namedColorList)

	for i := uint32(0); i < n; i++ {
		// Access the Name field of each cmsNAMEDCOLOR in the List
		if cmsstrcasecmp(name, &namedColorList.List[i].Name[0]) == 0 {
			return int32(i)
		}
	}

	return -1
}

// cmsAllocProfileSequenceDescription allocates memory for a profile sequence description.
func cmsAllocProfileSequenceDescription(ContextID CmsContext, n uint32) *cmsSEQ {
	if n == 0 || n > 255 {
		return nil // Invalid input
	}

	seq := allocateStruct[cmsSEQ]()
	if seq == nil {
		return nil
	}

	seq.ContextID = ContextID
	seq.seq = make([]cmsPSEQDESC, n)
	seq.n = n

	if seq.seq == nil {
		cmsFree(ContextID, seq)
		return nil
	}

	// Initialize each entry in the sequence
	for i := uint32(0); i < n; i++ {
		entry := seq.seq[i]
		entry.Manufacturer = nil
		entry.Model = nil
		entry.Description = nil
	}

	return seq
}

// cmsFreeProfileSequenceDescription frees the memory allocated for a profile sequence description.
func cmsFreeProfileSequenceDescription(pseq *cmsSEQ) {
	if pseq == nil {
		return
	}

	for i := uint32(0); i < pseq.n; i++ {
		entry := pseq.seq[i]

		if entry.Manufacturer != nil {
			cmsMLUfree(entry.Manufacturer)
		}
		if entry.Model != nil {
			cmsMLUfree(entry.Model)
		}
		if entry.Description != nil {
			cmsMLUfree(entry.Description)
		}
	}

	cmsFree(pseq.ContextID, pseq)
}

// cmsDupProfileSequenceDescription duplicates a profile sequence description.
func cmsDupProfileSequenceDescription(pseq *cmsSEQ) *cmsSEQ {
	if pseq == nil {
		return nil
	}

	newSeq := allocateStruct[cmsSEQ]()

	if newSeq == nil {
		return nil
	}

	newSeq.seq = make([]cmsPSEQDESC, pseq.n)
	if newSeq.seq == nil {
		cmsFreeProfileSequenceDescription(newSeq)
		return nil
	}

	newSeq.ContextID = pseq.ContextID
	newSeq.n = pseq.n

	for i := uint32(0); i < pseq.n; i++ {
		srcEntry := pseq.seq[i]
		dstEntry := newSeq.seq[i]

		// Copy basic fields
		dstEntry.deviceMfg = srcEntry.deviceMfg
		dstEntry.deviceModel = srcEntry.deviceModel
		dstEntry.attributes = srcEntry.attributes
		dstEntry.ProfileID = srcEntry.ProfileID
		dstEntry.technology = srcEntry.technology

		// Duplicate MLU fields
		dstEntry.Manufacturer = cmsMLUdup(srcEntry.Manufacturer)
		dstEntry.Model = cmsMLUdup(srcEntry.Model)
		dstEntry.Description = cmsMLUdup(srcEntry.Description)
	}

	return newSeq
}

// Dictionary structure
type cmsDICT struct {
	head      *cmsDICTentry
	ContextID CmsContext
}

// Allocate an empty dictionary
func cmsDictAlloc(contextID CmsContext) CmsHANDLE {
	dict := allocateStruct[cmsDICT]()
	return CmsHANDLE(dict)
}

// Dispose resources
func cmsDictFree(hDict CmsHANDLE) {
	dict, ok := hDict.(*cmsDICT)
	if dict == nil {
		return
	}
	if !ok {
		fmt.Printf("Error: Interface data assertion error, not *cmsDICT\n")
		return
	}
	entry := dict.head
	for entry != nil {
		if entry.DisplayName != nil {
			cmsMLUfree(entry.DisplayName)
		}
		if entry.DisplayValue != nil {
			cmsMLUfree(entry.DisplayValue)
		}

		next := entry.Next
		cmsFree(dict.ContextID, entry)
		entry = next
	}

	cmsFree(dict.ContextID, dict)
}

// Duplicate a wide character string
func DupWcs(contextID CmsContext, ptr []uint16) []uint16 {
	if ptr == nil {
		return nil
	}
	duplicate := cmsDupMemSlice(ptr)
	return duplicate
}

// Add a new entry to the linked list
func cmsDictAddEntry(hDict CmsHANDLE, name string, value string, displayName *cmsMLU, displayValue *cmsMLU) bool {
	dict, ok := hDict.(*cmsDICT)
	if !ok {
		fmt.Printf("Error: Interface data assertion error, not *cmsDICT\n")
		return false
	}
	if dict == nil || name == "" {
		return false
	}

	entry := allocateStruct[cmsDICTentry]()
	if entry == nil {
		return false
	}

	entry.DisplayName = cmsMLUdup(displayName)
	entry.DisplayValue = cmsMLUdup(displayValue)
	entry.Name = name
	entry.Value = value
	entry.Next = dict.head
	dict.head = entry

	return true
}

// Duplicate an existing dictionary
func cmsDictDup(hDict CmsHANDLE) CmsHANDLE {
	oldDict, ok := hDict.(*cmsDICT)
	if !ok {
		fmt.Printf("Error: Interface data assertion error, not *cmsDICT\n")
		return nil
	}
	if oldDict == nil {
		return nil
	}

	newDict := cmsDictAlloc(oldDict.ContextID)
	if newDict == nil {
		return nil
	}

	entry := oldDict.head
	for entry != nil {
		if !cmsDictAddEntry(newDict, entry.Name, entry.Value, entry.DisplayName, entry.DisplayValue) {
			cmsDictFree(newDict)
			return nil
		}
		entry = entry.Next
	}

	return newDict
}

// Get a pointer to the linked list
func cmsDictGetEntryList(hDict CmsHANDLE) *cmsDICTentry {
	dict, ok := hDict.(*cmsDICT)
	if !ok {
		fmt.Printf("Error: Interface data assertion error, not *cmsDICT\n")
		return nil
	}
	if dict == nil {
		return nil
	}
	return dict.head
}

// Helper for external languages
func cmsDictNextEntry(e *cmsDICTentry) *cmsDICTentry {
	if e == nil {
		return nil
	}
	return e.Next
}

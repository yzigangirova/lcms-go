package golcms

import (
	"encoding/binary"
	"unicode/utf16"
	"unsafe"
)

// cmsMLUalloc allocates an empty multi-localized unicode object.
func cmsMLUalloc(ContextID cmsContext, nItems uint32) *cmsMLU {
	if nItems <= 0 {
		nItems = 2
	}

	mlu := (*cmsMLU)(cmsMallocZero(ContextID, uint32(unsafe.Sizeof(cmsMLU{}))))
	if mlu == nil {
		return nil
	}

	mlu.ContextID = ContextID

	mlu.Entries = (*cmsMLUentry)(cmsCalloc(ContextID, nItems, uint32(unsafe.Sizeof(cmsMLUentry{}))))
	if mlu.Entries == nil {
		cmsFree(ContextID, unsafe.Pointer(mlu))
		return nil
	}

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

	allocatedEntries := mlu.AllocatedEntries * 2
	if allocatedEntries/2 != mlu.AllocatedEntries {
		return false
	}

	newPtr := (*cmsMLUentry)(cmsRealloc(mlu.ContextID, unsafe.Pointer(mlu.Entries), allocatedEntries*uint32(unsafe.Sizeof(cmsMLUentry{}))))
	if newPtr == nil {
		return false
	}

	mlu.Entries = newPtr
	mlu.AllocatedEntries = allocatedEntries

	return true
}

// SearchMLUEntry searches for a specific entry in the MLU based on language and country codes.
func SearchMLUEntry(mlu *cmsMLU, LanguageCode, CountryCode uint16) int {
	if mlu == nil {
		return -1
	}
	for i := uint32(0); i < mlu.UsedEntries; i++ {
		// Convert the base pointer to uintptr for arithmetic
		basePtr := uintptr(unsafe.Pointer(mlu.Entries))
		
		// Calculate the address of the current entry
		entryPtr := unsafe.Pointer(basePtr + uintptr(i)*unsafe.Sizeof(cmsMLUentry{}))
		
		// Cast the calculated address back to a *cmsMLUentry
		entry := (*cmsMLUentry)(entryPtr)
		
		// Compare the fields
		if entry.Country == CountryCode && entry.Language == LanguageCode {
			return int(i)
		}
	}
	//old piece
	/*for i := uint32(0); i < mlu.UsedEntries; i++ {
		entry := (*cmsMLUentry)(unsafe.Pointer(uintptr(mlu.Entries) + uintptr(i)*unsafe.Sizeof(cmsMLUentry{})))
		if entry.Country == CountryCode && entry.Language == LanguageCode {
			return int(i)
		}
	}*/

	return -1
}

// AddMLUBlock adds a block of characters to the MLU for a specific language and country.
func AddMLUBlock(mlu *cmsMLU, size uint32, block *uint16, LanguageCode, CountryCode uint16) bool {
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
	ptr := (*uint8)(mlu.MemPool)
	if ptr == nil {
		return false
	}

	memmove(unsafe.Add(unsafe.Pointer(ptr), offset), unsafe.Pointer(block), uintptr(size))
	mlu.PoolUsed += size

	//entry := mlu.Entries[mlu.UsedEntries]
	// Convert the base pointer to uintptr for arithmetic
	basePtr := uintptr(unsafe.Pointer(mlu.Entries))
	// Calculate the address of the current entry
	entryPtr := unsafe.Pointer(basePtr + uintptr(mlu.UsedEntries)*unsafe.Sizeof(cmsMLUentry{}))
	// Cast the calculated address back to a *cmsMLUentry
	entry := (*cmsMLUentry)(entryPtr)
	
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
func cmsMLUsetASCII(mlu *cmsMLU, LanguageCode, CountryCode string, ASCIIString *byte, lenASCII int) bool {
	if mlu == nil {
		return false
	}

	lang := strTo16(LanguageCode)
	country := strTo16(CountryCode)

	if lenASCII == 0 {
		lenASCII = 1
	}

	wStr := make([]uint16, lenASCII)
	for i := range wStr {
		if i < lenASCII {
			currentPtr := (*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(ASCIIString)) + uintptr(i)))
			val := binary.LittleEndian.Uint16([]byte{
				*currentPtr,
				*(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(currentPtr)) + 1)),
			})
			wStr[i] = uint16(val)
		}
	}

	rc := AddMLUBlock(mlu, uint32(len(wStr)*2), (*uint16)(unsafe.Pointer(&wStr[0])), lang, country)
	return rc
}

// mywcslen calculates the length of a wide string.
func mywcslen(s *uint16) uint32 {
	if s == nil {
		return 0
	}
	var length uint32
	for ptr := uintptr(unsafe.Pointer(s)); *(*uint16)(unsafe.Pointer(ptr)) != 0; ptr += 2 {
		length++
	}
	return length
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

	return AddMLUBlock(mlu, lenWide, (*uint16)(unsafe.Pointer(&WideString[0])), lang, country)
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

	if newMLU.AllocatedEntries < mlu.UsedEntries {
		cmsMLUfree(newMLU)
		return nil
	}

	if newMLU.Entries == nil || mlu.Entries == nil {
		cmsMLUfree(newMLU)
		return nil
	}

    memmove(unsafe.Pointer(newMLU.Entries), unsafe.Pointer(mlu.Entries), uintptr(mlu.UsedEntries) * unsafe.Sizeof(cmsMLUentry{}))
	newMLU.UsedEntries = mlu.UsedEntries

	if mlu.PoolUsed == 0 {
		newMLU.MemPool = nil
	} else {
		newMLU.MemPool = cmsMalloc(mlu.ContextID, mlu.PoolUsed)
		if newMLU.MemPool == nil {
			cmsMLUfree(newMLU)
			return nil
		}
	}

	newMLU.PoolSize = mlu.PoolUsed
	if newMLU.MemPool == nil || mlu.MemPool == nil {
		cmsMLUfree(newMLU)
		return nil
	}

	memmove(newMLU.MemPool, mlu.MemPool, uintptr(mlu.PoolUsed))
	newMLU.PoolUsed = mlu.PoolUsed

	return newMLU
}

// cmsMLUfree frees all memory used by an MLU.
func cmsMLUfree(mlu *cmsMLU) {
	if mlu != nil {
		if mlu.Entries != nil {
			cmsFree(mlu.ContextID, unsafe.Pointer(mlu.Entries))
		}
		if mlu.MemPool != nil {
			cmsFree(mlu.ContextID, mlu.MemPool)
		}
		cmsFree(mlu.ContextID, unsafe.Pointer(mlu))
	}
}

// cmsMLUgetWide searches for an entry in the MLU object and retrieves the wide string.
func _cmsMLUgetWide(mlu *cmsMLU, length *uint32, LanguageCode, CountryCode uint16, UsedLanguageCode, UsedCountryCode *uint16) *uint16 {
	if mlu == nil || mlu.AllocatedEntries == 0 {
		return nil
	}

	// Convert the base pointer to uintptr for arithmetic
	basePtr := uintptr(unsafe.Pointer(mlu.Entries))
	var bestMatch int = -1
	for i := uint32(0); i < mlu.UsedEntries; i++ {
		// Calculate the address of the current entry
		entryPtr := unsafe.Pointer(basePtr + uintptr(i)*unsafe.Sizeof(cmsMLUentry{}))
		// Cast the calculated address back to a *cmsMLUentry
		entry := (*cmsMLUentry)(entryPtr)
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

				return (*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(mlu.MemPool)) + uintptr(entry.StrW)))
			    }
		    }
	    }
	}
	if bestMatch == -1 {
		bestMatch = 0
	}
	entryPtr := unsafe.Pointer(basePtr + uintptr(bestMatch)*unsafe.Sizeof(cmsMLUentry{}))
	// Cast the calculated address back to a *cmsMLUentry
	entry := (*cmsMLUentry)(entryPtr)
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
	return (*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(mlu.MemPool)) + uintptr(entry.StrW)))
}


// cmsMLUgetASCII retrieves the ASCII string for a specific language and country.
func cmsMLUgetASCII(mlu *cmsMLU, LanguageCode, CountryCode string, Buffer *byte, BufferSize uint32) uint32 {
	lang := strTo16(LanguageCode)
	country := strTo16(CountryCode)

	var strLen uint32
	wide := _cmsMLUgetWide(mlu, &strLen, lang, country, nil, nil)
	if wide == nil {
		return 0
	}

	asciiLen := strLen / 2
	if Buffer == nil {
		return asciiLen + 1
	}
	if BufferSize <= 0 {
		return 0
	}
	if BufferSize < asciiLen+1 {
		asciiLen = BufferSize - 1
	}

	for i := uint32(0); i < asciiLen; i++ {
		//char := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(wide)) + uintptr(i*2)))
		// Calculate the address of the current entry
		// Cast the calculated address back to a *cmsMLUentry
		//buff_i := *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(Buffer)) + uintptr(i)))
		wide_i := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(wide)) + uintptr(i*2)))
		if wide_i == 0 {
			*(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(Buffer)) + uintptr(i))) = 0
		} else {
			*(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(Buffer)) + uintptr(i))) = byte(wide_i)
		}
	}
	*(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(Buffer)) + uintptr(asciiLen))) = 0
	return asciiLen + 1
}
func cmsMLUgetWide(
    mlu *cmsMLU,
    LanguageCode string,
    CountryCode string,
    Buffer *uint16,
    BufferSize uint32,
) uint32 {
    var Wide *uint16
    var StrLen uint32 = 0

    // Convert language and country codes to uint16
    Lang := strTo16(LanguageCode[:])
    Cntry := strTo16(CountryCode[:])

    // Sanity check
    if mlu == nil {
        return 0
    }

    // Get the wide string representation
    Wide = _cmsMLUgetWide(mlu, &StrLen, Lang, Cntry, nil, nil)
    if Wide == nil {
        return 0
    }

    // If the buffer is null, just return the length
    if Buffer == nil {
        return StrLen + uint32(unsafe.Sizeof(uint16(0)))
    }

    // If the buffer size is zero, no data can be written
    if BufferSize == 0 {
        return 0
    }

    // Adjust the length if the buffer size is too small
    if BufferSize < StrLen+uint32(unsafe.Sizeof(uint16(0))) {
        StrLen = BufferSize - uint32(unsafe.Sizeof(uint16(0)))
    }

    // Copy the data from Wide to the buffer
    src := unsafe.Pointer(Wide)
    dst := unsafe.Pointer(Buffer)
    memmove(dst, src, uintptr(StrLen))

    // Null-terminate the buffer
    bufferSlice := (*[1 << 30]uint16)(unsafe.Pointer(Buffer))[:StrLen/2+1]
    bufferSlice[StrLen/2] = 0

    return StrLen + uint32(unsafe.Sizeof(uint16(0)))
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
	basePtr := uintptr(unsafe.Pointer(mlu.Entries))
	// Calculate the address of the current entry
	entryPtr := unsafe.Pointer(basePtr + uintptr(idx)*unsafe.Sizeof(cmsMLUentry{}))
	// Cast the calculated address back to a *cmsMLUentry
	entry := (*cmsMLUentry)(entryPtr)

	if LanguageCode != nil {
		*LanguageCode = strFrom16(entry.Language)
	}
	if CountryCode != nil {
		*CountryCode = strFrom16(entry.Country)
	}
	return true
}

package golcms

import (
	"unicode/utf16"
)

// Converts a UTF-16 encoded []uint16 to a Go string
func toGoString(raw []uint16) string {
	for i, v := range raw {
		if v == 0 {
			return string(utf16.Decode(raw[:i]))
		}
	}
	return string(utf16.Decode(raw))
}

// Converts a Go string to UTF-16 with a null terminator
func fromGoString(s string) ([]uint16, error) {
	u := utf16.Encode([]rune(s))
	return append(u, 0), nil
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
	utf16Data, _ := fromGoString(str)

	// Write the UTF-16 data
	if !cmsWriteUInt16Array(io, uint32(len(utf16Data)), utf16Data) {
		return false
	}

	// Calculate the size of the written data
	e.Sizes[i] = uint32(io.Tell((*cms_io_handler)(io))) - before
	e.Offsets[i] = before - baseOffset
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
	rawData := mem.MakeSlice[uint16, nChars)
	if !cmsReadUInt16Array(io, nChars, rawData) {
		return "", false
	}

	// Convert UTF-16 to Go string
	return string(toGoString(rawData)), true
}

package golcms

import (
	//"unsafe"
)

// StringToUTF16Slice converts a Go string to a slice of uint16 for wide characters.
// StringToUTF16Slice converts a Go string to a UTF-16 encoded []uint16 (with null terminator)
func StringToUTF16Slice(s string) []uint16 {
	u, _ := fromGoString(s)
	return u
}
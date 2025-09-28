package golcms

import (
	"syscall"
	//"unsafe"
)

// StringToUTF16Slice converts a Go string to a slice of uint16 for wide characters.
func StringToUTF16Slice(s string) []uint16 {
	// Convert the string to a slice of runes (Unicode code points)
	// Encode the runes into UTF-16
	utf16, _ := syscall.UTF16FromString(s)
	return utf16[:len(utf16)-1]
	//return utf16
}
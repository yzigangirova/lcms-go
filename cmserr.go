package golcms

import (
	//"math"
	"fmt"
)

// cmsSignalError simulates error signaling
func cmsSignalError(id interface{}, code int, message string) {
	fmt.Printf("Error: %s (code %d)\n", message, code)
}

package golcms

import (
	"math"
	"reflect"
	"unsafe"
)

// Base types
type (
	cmsUInt8Number   uint8
	cmsInt8Number    int8
	cmsFloat32Number float32
	cmsFloat64Number float64
)

// 16-bit base types
type (
	cmsUInt16Number uint16
	cmsInt16Number  int16
)

// 32-bit base types
type (
	cmsUInt32Number uint32
	cmsInt32Number  int32
)

// 64-bit base types
// These are defined only if Go's types natively support 64-bit integers.
type (
	cmsUInt64Number uint64
	cmsInt64Number  int64
)

// //////////////////LCMS placeholders////////////////////////
// ICC Intents
const (
	INTENT_PERCEPTUAL = iota
	INTENT_RELATIVE_COLORIMETRIC
	INTENT_SATURATION
	INTENT_ABSOLUTE_COLORIMETRIC
)

// Some common definitions
const cmsMAX_PATH = 256

// Little CMS specific typedefs

type cmsInfoType int

// Define cmsHPROFILE as unsafe.Pointer to represent a void pointer
type cmsHPROFILE unsafe.Pointer
type cmsHANDLE unsafe.Pointer // Generic handle
type cmsHTRANSFORM unsafe.Pointer
type cmsToneCurve unsafe.Pointer

// cmsCreateContext creates a new context with the given plugin and user data.
func cmsCreateContext(plugin unsafe.Pointer, userData unsafe.Pointer) cmsContext

// cmsDeleteContext deletes a given context.
func cmsDeleteContext(contextID cmsContext)

// cmsDupContext duplicates a given context, optionally setting new user data.
func cmsDupContext(contextID cmsContext, newUserData unsafe.Pointer) cmsContext

// cmsGetContextUserData retrieves the user data associated with the given context.
func cmsGetContextUserData(contextID cmsContext) unsafe.Pointer

// Plug-In Registering Functions

// cmsPlugin registers a global plugin.
func cmsPlugin(plugin unsafe.Pointer) bool

// cmsPluginTHR registers a plugin for a specific context.
func cmsPluginTHR(contextID cmsContext, plugin unsafe.Pointer) bool

// cmsUnregisterPlugins unregisters all global plugins.
func cmsUnregisterPlugins()

// cmsUnregisterPluginsTHR unregisters plugins for a specific context.
func cmsUnregisterPluginsTHR(contextID cmsContext)

// Error Codes
const (
	cmsERROR_UNDEFINED           = 0  // Undefined error
	cmsERROR_FILE                = 1  // File-related error
	cmsERROR_RANGE               = 2  // Range error
	cmsERROR_INTERNAL            = 3  // Internal error
	cmsERROR_NULL                = 4  // Null pointer error
	cmsERROR_READ                = 5  // Read error
	cmsERROR_SEEK                = 6  // Seek error
	cmsERROR_WRITE               = 7  // Write error
	cmsERROR_UNKNOWN_EXTENSION   = 8  // Unknown extension
	cmsERROR_COLORSPACE_CHECK    = 9  // Colorspace check failed
	cmsERROR_ALREADY_DEFINED     = 10 // Already defined
	cmsERROR_BAD_SIGNATURE       = 11 // Bad signature
	cmsERROR_CORRUPTION_DETECTED = 12 // Corruption detected
	cmsERROR_NOT_SUITABLE        = 13 // Not suitable
)

// Error logging function type
// Error logger is called with the ContextID when a message is raised. This gives the
// chance to know which thread is responsible for the warning and any environment associated
// with it. Non-multithreading applications may safely ignore this parameter.
// Note that under certain special circumstances, ContextID may be NULL.
type cmsLogErrorHandlerFunction func(ContextID cmsContext, ErrorCode cmsUInt32Number, Text string)

// Allows user to set any specific logger
func cmsSetLogErrorHandler(fn cmsLogErrorHandlerFunction) {
	// Implementation of setting global error handler would go here
}

// Allows user to set any specific logger in a thread-safe manner
func cmsSetLogErrorHandlerTHR(contextID cmsContext, fn cmsLogErrorHandlerFunction) {
	// Implementation of thread-specific error handler would go here
}

// Define cmsInfoType as int, which should match the type in the C library
// type cmsInfoType C.int
// Constants representing the info type in the CMS library
const (
	cmsInfoDescription cmsInfoType = iota
	cmsInfoManufacturer
	cmsInfoModel
	cmsInfoCopyright
)

// Bit-shifting helpers and constants

// Pixel type constants

// Pixel types
// Pixel type constants
const (
	PT_ANY = 0 // Don't check colorspace
	// 1 & 2 are reserved
	PT_GRAY  = 3
	PT_RGB   = 4
	PT_CMY   = 5
	PT_CMYK  = 6
	PT_YCbCr = 7
	PT_YUV   = 8 // Lu'v'
	PT_XYZ   = 9
	PT_Lab   = 10
	PT_YUVK  = 11 // Lu'v'K
	PT_HSV   = 12
	PT_HLS   = 13
	PT_Yxy   = 14
	PT_MCH1  = 15
	PT_MCH2  = 16
	PT_MCH3  = 17
	PT_MCH4  = 18
	PT_MCH5  = 19
	PT_MCH6  = 20
	PT_MCH7  = 21
	PT_MCH8  = 22
	PT_MCH9  = 23
	PT_MCH10 = 24
	PT_MCH11 = 25
	PT_MCH12 = 26
	PT_MCH13 = 27
	PT_MCH14 = 28
	PT_MCH15 = 29
	PT_LabV2 = 30 // Identical to PT_Lab, but using the V2 old encoding
)

// Pixel type definitions using helper functions

// Define constants using the shift macros
var (
	TYPE_GRAY_8          = COLORSPACE_SH(PT_GRAY) | CHANNELS_SH(1) | BYTES_SH(1)
	TYPE_GRAY_8_REV      = COLORSPACE_SH(PT_GRAY) | CHANNELS_SH(1) | BYTES_SH(1) | FLAVOR_SH(1)
	TYPE_GRAY_16         = COLORSPACE_SH(PT_GRAY) | CHANNELS_SH(1) | BYTES_SH(2)
	TYPE_GRAY_16_REV     = COLORSPACE_SH(PT_GRAY) | CHANNELS_SH(1) | BYTES_SH(2) | FLAVOR_SH(1)
	TYPE_GRAY_16_SE      = COLORSPACE_SH(PT_GRAY) | CHANNELS_SH(1) | BYTES_SH(2) | ENDIAN16_SH(1)
	TYPE_GRAYA_8         = COLORSPACE_SH(PT_GRAY) | EXTRA_SH(1) | CHANNELS_SH(1) | BYTES_SH(1)
	TYPE_GRAYA_8_PREMUL  = COLORSPACE_SH(PT_GRAY) | EXTRA_SH(1) | CHANNELS_SH(1) | BYTES_SH(1) | PREMUL_SH(1)
	TYPE_GRAYA_16        = COLORSPACE_SH(PT_GRAY) | EXTRA_SH(1) | CHANNELS_SH(1) | BYTES_SH(2)
	TYPE_GRAYA_16_PREMUL = COLORSPACE_SH(PT_GRAY) | EXTRA_SH(1) | CHANNELS_SH(1) | BYTES_SH(2) | PREMUL_SH(1)
	TYPE_GRAYA_16_SE     = COLORSPACE_SH(PT_GRAY) | EXTRA_SH(1) | CHANNELS_SH(1) | BYTES_SH(2) | ENDIAN16_SH(1)
	TYPE_GRAYA_8_PLANAR  = COLORSPACE_SH(PT_GRAY) | EXTRA_SH(1) | CHANNELS_SH(1) | BYTES_SH(1) | PLANAR_SH(1)
	TYPE_GRAYA_16_PLANAR = COLORSPACE_SH(PT_GRAY) | EXTRA_SH(1) | CHANNELS_SH(1) | BYTES_SH(2) | PLANAR_SH(1)

	TYPE_RGB_8          = COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(1)
	TYPE_RGB_8_PLANAR   = COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(1) | PLANAR_SH(1)
	TYPE_BGR_8          = COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(1) | DOSWAP_SH(1)
	TYPE_BGR_8_PLANAR   = COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(1) | DOSWAP_SH(1) | PLANAR_SH(1)
	TYPE_RGB_16         = COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(2)
	TYPE_RGB_16_PLANAR  = COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(2) | PLANAR_SH(1)
	TYPE_RGB_16_SE      = COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(2) | ENDIAN16_SH(1)
	TYPE_BGR_16         = COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(2) | DOSWAP_SH(1)
	TYPE_BGR_16_PLANAR  = COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(2) | DOSWAP_SH(1) | PLANAR_SH(1)
	TYPE_BGR_16_SE      = COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(2) | DOSWAP_SH(1) | ENDIAN16_SH(1)
	TYPE_RGBA_8         = COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(1)
	TYPE_RGBA_8_PREMUL  = COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(1) | PREMUL_SH(1)
	TYPE_RGBA_8_PLANAR  = COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(1) | PLANAR_SH(1)
	TYPE_RGBA_16        = COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(2)
	TYPE_RGBA_16_PREMUL = COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(2) | PREMUL_SH(1)
	TYPE_RGBA_16_PLANAR = COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(2) | PLANAR_SH(1)
	TYPE_RGBA_16_SE     = COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(2) | ENDIAN16_SH(1)

	TYPE_ARGB_8         = COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(1) | SWAPFIRST_SH(1)
	TYPE_ARGB_8_PREMUL  = COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(1) | SWAPFIRST_SH(1) | PREMUL_SH(1)
	TYPE_ARGB_16        = COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(2) | SWAPFIRST_SH(1)
	TYPE_ARGB_16_PREMUL = COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(2) | SWAPFIRST_SH(1) | PREMUL_SH(1)

	TYPE_CMY_8         = COLORSPACE_SH(PT_CMY) | CHANNELS_SH(3) | BYTES_SH(1)
	TYPE_CMY_8_PLANAR  = COLORSPACE_SH(PT_CMY) | CHANNELS_SH(3) | BYTES_SH(1) | PLANAR_SH(1)
	TYPE_CMY_16        = COLORSPACE_SH(PT_CMY) | CHANNELS_SH(3) | BYTES_SH(2)
	TYPE_CMY_16_PLANAR = COLORSPACE_SH(PT_CMY) | CHANNELS_SH(3) | BYTES_SH(2) | PLANAR_SH(1)
	TYPE_CMY_16_SE     = COLORSPACE_SH(PT_CMY) | CHANNELS_SH(3) | BYTES_SH(2) | ENDIAN16_SH(1)

	TYPE_CMYK_8         = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(1)
	TYPE_CMYKA_8        = (PT_CMYK) | EXTRA_SH(1) | CHANNELS_SH(4) | BYTES_SH(1)
	TYPE_CMYK_8_REV     = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(1) | FLAVOR_SH(1)
	TYPE_YUVK_8         = TYPE_CMYK_8_REV
	TYPE_CMYK_8_PLANAR  = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(1) | PLANAR_SH(1)
	TYPE_CMYK_16        = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(2)
	TYPE_CMYK_16_REV    = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(2) | FLAVOR_SH(1)
	TYPE_YUVK_16        = TYPE_CMYK_16_REV
	TYPE_CMYK_16_PLANAR = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(2) | PLANAR_SH(1)
	TYPE_CMYK_16_SE     = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(2) | ENDIAN16_SH(1)

	TYPE_KYMC_8     = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(1) | DOSWAP_SH(1)
	TYPE_KYMC_16    = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(2) | DOSWAP_SH(1)
	TYPE_KYMC_16_SE = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(2) | DOSWAP_SH(1) | ENDIAN16_SH(1)

	TYPE_KCMY_8      = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(1) | SWAPFIRST_SH(1)
	TYPE_KCMY_8_REV  = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(1) | FLAVOR_SH(1) | SWAPFIRST_SH(1)
	TYPE_KCMY_16     = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(2) | SWAPFIRST_SH(1)
	TYPE_KCMY_16_REV = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(2) | FLAVOR_SH(1) | SWAPFIRST_SH(1)
	TYPE_KCMY_16_SE  = COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(2) | ENDIAN16_SH(1) | SWAPFIRST_SH(1)

	TYPE_CMYK5_8         = COLORSPACE_SH(PT_MCH5) | CHANNELS_SH(5) | BYTES_SH(1)
	TYPE_CMYK5_16        = COLORSPACE_SH(PT_MCH5) | CHANNELS_SH(5) | BYTES_SH(2)
	TYPE_CMYK5_16_SE     = COLORSPACE_SH(PT_MCH5) | CHANNELS_SH(5) | BYTES_SH(2) | ENDIAN16_SH(1)
	TYPE_KYMC5_8         = COLORSPACE_SH(PT_MCH5) | CHANNELS_SH(5) | BYTES_SH(1) | DOSWAP_SH(1)
	TYPE_KYMC5_16        = COLORSPACE_SH(PT_MCH5) | CHANNELS_SH(5) | BYTES_SH(2) | DOSWAP_SH(1)
	TYPE_KYMC5_16_SE     = COLORSPACE_SH(PT_MCH5) | CHANNELS_SH(5) | BYTES_SH(2) | DOSWAP_SH(1) | ENDIAN16_SH(1)
	TYPE_CMYK6_8         = COLORSPACE_SH(PT_MCH6) | CHANNELS_SH(6) | BYTES_SH(1)
	TYPE_CMYK6_8_PLANAR  = COLORSPACE_SH(PT_MCH6) | CHANNELS_SH(6) | BYTES_SH(1) | PLANAR_SH(1)
	TYPE_CMYK6_16        = COLORSPACE_SH(PT_MCH6) | CHANNELS_SH(6) | BYTES_SH(2)
	TYPE_CMYK6_16_PLANAR = COLORSPACE_SH(PT_MCH6) | CHANNELS_SH(6) | BYTES_SH(2) | PLANAR_SH(1)
	TYPE_CMYK6_16_SE     = COLORSPACE_SH(PT_MCH6) | CHANNELS_SH(6) | BYTES_SH(2) | ENDIAN16_SH(1)
	TYPE_CMYK7_8         = COLORSPACE_SH(PT_MCH7) | CHANNELS_SH(7) | BYTES_SH(1)
	TYPE_CMYK7_16        = COLORSPACE_SH(PT_MCH7) | CHANNELS_SH(7) | BYTES_SH(2)
	TYPE_CMYK7_16_SE     = COLORSPACE_SH(PT_MCH7) | CHANNELS_SH(7) | BYTES_SH(2) | ENDIAN16_SH(1)
	TYPE_KYMC7_8         = COLORSPACE_SH(PT_MCH7) | CHANNELS_SH(7) | BYTES_SH(1) | DOSWAP_SH(1)
	TYPE_KYMC7_16        = COLORSPACE_SH(PT_MCH7) | CHANNELS_SH(7) | BYTES_SH(2) | DOSWAP_SH(1)
	TYPE_KYMC7_16_SE     = COLORSPACE_SH(PT_MCH7) | CHANNELS_SH(7) | BYTES_SH(2) | DOSWAP_SH(1) | ENDIAN16_SH(1)
	TYPE_CMYK8_8         = COLORSPACE_SH(PT_MCH8) | CHANNELS_SH(8) | BYTES_SH(1)
	TYPE_CMYK8_16        = COLORSPACE_SH(PT_MCH8) | CHANNELS_SH(8) | BYTES_SH(2)
	TYPE_CMYK8_16_SE     = COLORSPACE_SH(PT_MCH8) | CHANNELS_SH(8) | BYTES_SH(2) | ENDIAN16_SH(1)
	TYPE_KYMC8_8         = COLORSPACE_SH(PT_MCH8) | CHANNELS_SH(8) | BYTES_SH(1) | DOSWAP_SH(1)
	TYPE_KYMC8_16        = COLORSPACE_SH(PT_MCH8) | CHANNELS_SH(8) | BYTES_SH(2) | DOSWAP_SH(1)
	TYPE_KYMC8_16_SE     = COLORSPACE_SH(PT_MCH8) | CHANNELS_SH(8) | BYTES_SH(2) | DOSWAP_SH(1) | ENDIAN16_SH(1)
	TYPE_CMYK9_8         = COLORSPACE_SH(PT_MCH9) | CHANNELS_SH(9) | BYTES_SH(1)
	TYPE_CMYK9_16        = COLORSPACE_SH(PT_MCH9) | CHANNELS_SH(9) | BYTES_SH(2)
	TYPE_CMYK9_16_SE     = COLORSPACE_SH(PT_MCH9) | CHANNELS_SH(9) | BYTES_SH(2) | ENDIAN16_SH(1)
	TYPE_KYMC9_8         = COLORSPACE_SH(PT_MCH9) | CHANNELS_SH(9) | BYTES_SH(1) | DOSWAP_SH(1)
	TYPE_KYMC9_16        = COLORSPACE_SH(PT_MCH9) | CHANNELS_SH(9) | BYTES_SH(2) | DOSWAP_SH(1)
	TYPE_KYMC9_16_SE     = COLORSPACE_SH(PT_MCH9) | CHANNELS_SH(9) | BYTES_SH(2) | DOSWAP_SH(1) | ENDIAN16_SH(1)
	TYPE_CMYK10_8        = COLORSPACE_SH(PT_MCH10) | CHANNELS_SH(10) | BYTES_SH(1)
	TYPE_CMYK10_16       = COLORSPACE_SH(PT_MCH10) | CHANNELS_SH(10) | BYTES_SH(2)
	TYPE_CMYK10_16_SE    = COLORSPACE_SH(PT_MCH10) | CHANNELS_SH(10) | BYTES_SH(2) | ENDIAN16_SH(1)
	TYPE_KYMC10_8        = COLORSPACE_SH(PT_MCH10) | CHANNELS_SH(10) | BYTES_SH(1) | DOSWAP_SH(1)
	TYPE_KYMC10_16       = COLORSPACE_SH(PT_MCH10) | CHANNELS_SH(10) | BYTES_SH(2) | DOSWAP_SH(1)
	TYPE_KYMC10_16_SE    = COLORSPACE_SH(PT_MCH10) | CHANNELS_SH(10) | BYTES_SH(2) | DOSWAP_SH(1) | ENDIAN16_SH(1)
	TYPE_CMYK11_8        = COLORSPACE_SH(PT_MCH11) | CHANNELS_SH(11) | BYTES_SH(1)
	TYPE_CMYK11_16       = COLORSPACE_SH(PT_MCH11) | CHANNELS_SH(11) | BYTES_SH(2)
	TYPE_CMYK11_16_SE    = COLORSPACE_SH(PT_MCH11) | CHANNELS_SH(11) | BYTES_SH(2) | ENDIAN16_SH(1)
	TYPE_KYMC11_8        = COLORSPACE_SH(PT_MCH11) | CHANNELS_SH(11) | BYTES_SH(1) | DOSWAP_SH(1)
	TYPE_KYMC11_16       = COLORSPACE_SH(PT_MCH11) | CHANNELS_SH(11) | BYTES_SH(2) | DOSWAP_SH(1)
	TYPE_KYMC11_16_SE    = COLORSPACE_SH(PT_MCH11) | CHANNELS_SH(11) | BYTES_SH(2) | DOSWAP_SH(1) | ENDIAN16_SH(1)
	TYPE_CMYK12_8        = COLORSPACE_SH(PT_MCH12) | CHANNELS_SH(12) | BYTES_SH(1)
	TYPE_CMYK12_16       = COLORSPACE_SH(PT_MCH12) | CHANNELS_SH(12) | BYTES_SH(2)
	TYPE_CMYK12_16_SE    = COLORSPACE_SH(PT_MCH12) | CHANNELS_SH(12) | BYTES_SH(2) | ENDIAN16_SH(1)
	TYPE_KYMC12_8        = COLORSPACE_SH(PT_MCH12) | CHANNELS_SH(12) | BYTES_SH(1) | DOSWAP_SH(1)
	TYPE_KYMC12_16       = COLORSPACE_SH(PT_MCH12) | CHANNELS_SH(12) | BYTES_SH(2) | DOSWAP_SH(1)
	TYPE_KYMC12_16_SE    = COLORSPACE_SH(PT_MCH12) | CHANNELS_SH(12) | BYTES_SH(2) | DOSWAP_SH(1) | ENDIAN16_SH(1)

	// Colorimetric
	TYPE_XYZ_16  = COLORSPACE_SH(PT_XYZ) | CHANNELS_SH(3) | BYTES_SH(2)
	TYPE_Lab_8   = COLORSPACE_SH(PT_Lab) | CHANNELS_SH(3) | BYTES_SH(1)
	TYPE_LabV2_8 = COLORSPACE_SH(PT_LabV2) | CHANNELS_SH(3) | BYTES_SH(1)

	TYPE_ALab_8   = COLORSPACE_SH(PT_Lab) | CHANNELS_SH(3) | BYTES_SH(1) | EXTRA_SH(1) | SWAPFIRST_SH(1)
	TYPE_ALabV2_8 = COLORSPACE_SH(PT_LabV2) | CHANNELS_SH(3) | BYTES_SH(1) | EXTRA_SH(1) | SWAPFIRST_SH(1)
	TYPE_Lab_16   = COLORSPACE_SH(PT_Lab) | CHANNELS_SH(3) | BYTES_SH(2)
	TYPE_LabV2_16 = COLORSPACE_SH(PT_LabV2) | CHANNELS_SH(3) | BYTES_SH(2)
	TYPE_Yxy_16   = COLORSPACE_SH(PT_Yxy) | CHANNELS_SH(3) | BYTES_SH(2)

	// YCbCr
	TYPE_YCbCr_8         = COLORSPACE_SH(PT_YCbCr) | CHANNELS_SH(3) | BYTES_SH(1)
	TYPE_YCbCr_8_PLANAR  = COLORSPACE_SH(PT_YCbCr) | CHANNELS_SH(3) | BYTES_SH(1) | PLANAR_SH(1)
	TYPE_YCbCr_16        = COLORSPACE_SH(PT_YCbCr) | CHANNELS_SH(3) | BYTES_SH(2)
	TYPE_YCbCr_16_PLANAR = COLORSPACE_SH(PT_YCbCr) | CHANNELS_SH(3) | BYTES_SH(2) | PLANAR_SH(1)
	TYPE_YCbCr_16_SE     = COLORSPACE_SH(PT_YCbCr) | CHANNELS_SH(3) | BYTES_SH(2) | ENDIAN16_SH(1)

	// YUV
	TYPE_YUV_8         = COLORSPACE_SH(PT_YUV) | CHANNELS_SH(3) | BYTES_SH(1)
	TYPE_YUV_8_PLANAR  = COLORSPACE_SH(PT_YUV) | CHANNELS_SH(3) | BYTES_SH(1) | PLANAR_SH(1)
	TYPE_YUV_16        = COLORSPACE_SH(PT_YUV) | CHANNELS_SH(3) | BYTES_SH(2)
	TYPE_YUV_16_PLANAR = COLORSPACE_SH(PT_YUV) | CHANNELS_SH(3) | BYTES_SH(2) | PLANAR_SH(1)
	TYPE_YUV_16_SE     = COLORSPACE_SH(PT_YUV) | CHANNELS_SH(3) | BYTES_SH(2) | ENDIAN16_SH(1)

	// HLS
	TYPE_HLS_8         = COLORSPACE_SH(PT_HLS) | CHANNELS_SH(3) | BYTES_SH(1)
	TYPE_HLS_8_PLANAR  = COLORSPACE_SH(PT_HLS) | CHANNELS_SH(3) | BYTES_SH(1) | PLANAR_SH(1)
	TYPE_HLS_16        = COLORSPACE_SH(PT_HLS) | CHANNELS_SH(3) | BYTES_SH(2)
	TYPE_HLS_16_PLANAR = COLORSPACE_SH(PT_HLS) | CHANNELS_SH(3) | BYTES_SH(2) | PLANAR_SH(1)
	TYPE_HLS_16_SE     = COLORSPACE_SH(PT_HLS) | CHANNELS_SH(3) | BYTES_SH(2) | ENDIAN16_SH(1)

	// HSV
	TYPE_HSV_8         = COLORSPACE_SH(PT_HSV) | CHANNELS_SH(3) | BYTES_SH(1)
	TYPE_HSV_8_PLANAR  = COLORSPACE_SH(PT_HSV) | CHANNELS_SH(3) | BYTES_SH(1) | PLANAR_SH(1)
	TYPE_HSV_16        = COLORSPACE_SH(PT_HSV) | CHANNELS_SH(3) | BYTES_SH(2)
	TYPE_HSV_16_PLANAR = COLORSPACE_SH(PT_HSV) | CHANNELS_SH(3) | BYTES_SH(2) | PLANAR_SH(1)
	TYPE_HSV_16_SE     = COLORSPACE_SH(PT_HSV) | CHANNELS_SH(3) | BYTES_SH(2) | ENDIAN16_SH(1)
)

// Float formatters
var (
	TYPE_XYZ_FLT          = FLOAT_SH(1) | COLORSPACE_SH(PT_XYZ) | CHANNELS_SH(3) | BYTES_SH(4)
	TYPE_Lab_FLT          = FLOAT_SH(1) | COLORSPACE_SH(PT_Lab) | CHANNELS_SH(3) | BYTES_SH(4)
	TYPE_LabA_FLT         = FLOAT_SH(1) | COLORSPACE_SH(PT_Lab) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(4)
	TYPE_GRAY_FLT         = FLOAT_SH(1) | COLORSPACE_SH(PT_GRAY) | CHANNELS_SH(1) | BYTES_SH(4)
	TYPE_GRAYA_FLT        = FLOAT_SH(1) | COLORSPACE_SH(PT_GRAY) | CHANNELS_SH(1) | BYTES_SH(4) | EXTRA_SH(1)
	TYPE_GRAYA_FLT_PREMUL = FLOAT_SH(1) | COLORSPACE_SH(PT_GRAY) | CHANNELS_SH(1) | BYTES_SH(4) | EXTRA_SH(1) | PREMUL_SH(1)
	TYPE_RGB_FLT          = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(4)

	TYPE_RGBA_FLT        = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(4)
	TYPE_RGBA_FLT_PREMUL = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(4) | PREMUL_SH(1)
	TYPE_ARGB_FLT        = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(4) | SWAPFIRST_SH(1)
	TYPE_ARGB_FLT_PREMUL = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(4) | SWAPFIRST_SH(1) | PREMUL_SH(1)
	TYPE_BGR_FLT         = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(4) | DOSWAP_SH(1)
	TYPE_BGRA_FLT        = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(4) | DOSWAP_SH(1) | SWAPFIRST_SH(1)
	TYPE_BGRA_FLT_PREMUL = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(4) | DOSWAP_SH(1) | SWAPFIRST_SH(1) | PREMUL_SH(1)
	TYPE_ABGR_FLT        = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(4) | DOSWAP_SH(1)
	TYPE_ABGR_FLT_PREMUL = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(4) | DOSWAP_SH(1) | PREMUL_SH(1)

	TYPE_CMYK_FLT = FLOAT_SH(1) | COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(4)
)

// Floating point formatters
var (
	TYPE_XYZ_DBL  = FLOAT_SH(1) | COLORSPACE_SH(PT_XYZ) | CHANNELS_SH(3) | BYTES_SH(0)
	TYPE_Lab_DBL  = FLOAT_SH(1) | COLORSPACE_SH(PT_Lab) | CHANNELS_SH(3) | BYTES_SH(0)
	TYPE_GRAY_DBL = FLOAT_SH(1) | COLORSPACE_SH(PT_GRAY) | CHANNELS_SH(1) | BYTES_SH(0)
	TYPE_RGB_DBL  = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(0)
	TYPE_BGR_DBL  = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(0) | DOSWAP_SH(1)
	TYPE_CMYK_DBL = FLOAT_SH(1) | COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(0)
)

// IEEE 754-2008 "half"
var (
	TYPE_GRAY_HALF_FLT = FLOAT_SH(1) | COLORSPACE_SH(PT_GRAY) | CHANNELS_SH(1) | BYTES_SH(2)
	TYPE_RGB_HALF_FLT  = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(2)
	TYPE_RGBA_HALF_FLT = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(2)
	TYPE_CMYK_HALF_FLT = FLOAT_SH(1) | COLORSPACE_SH(PT_CMYK) | CHANNELS_SH(4) | BYTES_SH(2)

	TYPE_ARGB_HALF_FLT = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(2) | SWAPFIRST_SH(1)
	TYPE_BGR_HALF_FLT  = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(2) | DOSWAP_SH(1)
	TYPE_BGRA_HALF_FLT = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | EXTRA_SH(1) | CHANNELS_SH(3) | BYTES_SH(2) | DOSWAP_SH(1) | SWAPFIRST_SH(1)
	TYPE_ABGR_HALF_FLT = FLOAT_SH(1) | COLORSPACE_SH(PT_RGB) | CHANNELS_SH(3) | BYTES_SH(2) | DOSWAP_SH(1)
)

// Flags
const (
	cmsFLAGS_NOCACHE       = 0x0040 // Inhibit 1-pixel cache
	cmsFLAGS_NOOPTIMIZE    = 0x0100 // Inhibit optimizations
	cmsFLAGS_NULLTRANSFORM = 0x0200 // Don't transform anyway

	// Proofing flags
	cmsFLAGS_GAMUTCHECK   = 0x1000 // Out of Gamut alarm
	cmsFLAGS_SOFTPROOFING = 0x4000 // Do softproofing

	// Misc
	cmsFLAGS_BLACKPOINTCOMPENSATION = 0x2000 // Black point compensation
	cmsFLAGS_NOWHITEONWHITEFIXUP    = 0x0004 // Don't fix scum dot
	cmsFLAGS_HIGHRESPRECALC         = 0x0400 // Use more memory for better accuracy
	cmsFLAGS_LOWRESPRECALC          = 0x0800 // Use less memory to minimize resources

	// For devicelink creation
	cmsFLAGS_8BITS_DEVICELINK = 0x0008 // Create 8-bit devicelinks
	cmsFLAGS_GUESSDEVICECLASS = 0x0020 // Guess device class for transform2devicelink
	cmsFLAGS_KEEP_SEQUENCE    = 0x0080 // Keep profile sequence for devicelink creation

	// Specific to particular optimizations
	cmsFLAGS_FORCE_CLUT              = 0x0002 // Force CLUT optimization
	cmsFLAGS_CLUT_POST_LINEARIZATION = 0x0001 // Create postlinearization tables if possible
	cmsFLAGS_CLUT_PRE_LINEARIZATION  = 0x0010 // Create prelinearization tables if possible

	// Specific to unbounded mode
	cmsFLAGS_NONEGATIVES = 0x8000 // Prevent negative numbers in floating-point transforms

	// Copy alpha channels when transforming
	cmsFLAGS_COPY_ALPHA = 0x04000000 // Alpha channels are copied on cmsDoTransform()

	// Fine-tune control over number of gridpoints
	cmsFLAGS_GRIDPOINTS_MASK  = 0xFF
	cmsFLAGS_GRIDPOINTS_SHIFT = 16

	// CRD special
	cmsFLAGS_NODEFAULTRESOURCEDEF = 0x01000000 // No default resource definitions
)

type cmsColorSpaceSignature uint32

const (
	cmsSigXYZData   cmsColorSpaceSignature = 0x58595A20 // 'XYZ '
	cmsSigLabData   cmsColorSpaceSignature = 0x4C616220 // 'Lab '
	cmsSigLuvData   cmsColorSpaceSignature = 0x4C757620 // 'Luv '
	cmsSigYCbCrData cmsColorSpaceSignature = 0x59436272 // 'YCbr'
	cmsSigYxyData   cmsColorSpaceSignature = 0x59787920 // 'Yxy '
	cmsSigRgbData   cmsColorSpaceSignature = 0x52474220 // 'RGB '
	cmsSigGrayData  cmsColorSpaceSignature = 0x47524159 // 'GRAY'
	cmsSigHsvData   cmsColorSpaceSignature = 0x48535620 // 'HSV '
	cmsSigHlsData   cmsColorSpaceSignature = 0x484C5320 // 'HLS '
	cmsSigCmykData  cmsColorSpaceSignature = 0x434D594B // 'CMYK'
	cmsSigCmyData   cmsColorSpaceSignature = 0x434D5920 // 'CMY '

	cmsSigMCH1Data cmsColorSpaceSignature = 0x4D434831 // 'MCH1'
	cmsSigMCH2Data cmsColorSpaceSignature = 0x4D434832 // 'MCH2'
	cmsSigMCH3Data cmsColorSpaceSignature = 0x4D434833 // 'MCH3'
	cmsSigMCH4Data cmsColorSpaceSignature = 0x4D434834 // 'MCH4'
	cmsSigMCH5Data cmsColorSpaceSignature = 0x4D434835 // 'MCH5'
	cmsSigMCH6Data cmsColorSpaceSignature = 0x4D434836 // 'MCH6'
	cmsSigMCH7Data cmsColorSpaceSignature = 0x4D434837 // 'MCH7'
	cmsSigMCH8Data cmsColorSpaceSignature = 0x4D434838 // 'MCH8'
	cmsSigMCH9Data cmsColorSpaceSignature = 0x4D434839 // 'MCH9'
	cmsSigMCHAData cmsColorSpaceSignature = 0x4D434841 // 'MCHA'
	cmsSigMCHBData cmsColorSpaceSignature = 0x4D434842 // 'MCHB'
	cmsSigMCHCData cmsColorSpaceSignature = 0x4D434843 // 'MCHC'
	cmsSigMCHDData cmsColorSpaceSignature = 0x4D434844 // 'MCHD'
	cmsSigMCHEData cmsColorSpaceSignature = 0x4D434845 // 'MCHE'
	cmsSigMCHFData cmsColorSpaceSignature = 0x4D434846 // 'MCHF'

	cmsSigNamedData  cmsColorSpaceSignature = 0x6E6D636C // 'nmcl'
	cmsSig1colorData cmsColorSpaceSignature = 0x31434C52 // '1CLR'
	cmsSig2colorData cmsColorSpaceSignature = 0x32434C52 // '2CLR'
	cmsSig3colorData cmsColorSpaceSignature = 0x33434C52 // '3CLR'
	cmsSig4colorData cmsColorSpaceSignature = 0x34434C52 // '4CLR'
	cmsSig5colorData cmsColorSpaceSignature = 0x35434C52 // '5CLR'
	cmsSig6colorData cmsColorSpaceSignature = 0x36434C52 // '6CLR'
	cmsSig7colorData cmsColorSpaceSignature = 0x37434C52 // '7CLR'
	cmsSig8colorData cmsColorSpaceSignature = 0x38434C52 // '8CLR'
	cmsSig9colorData cmsColorSpaceSignature = 0x39434C52 // '9CLR'

	cmsSig10colorData cmsColorSpaceSignature = 0x41434C52 // 'ACLR'
	cmsSig11colorData cmsColorSpaceSignature = 0x42434C52 // 'BCLR'
	cmsSig12colorData cmsColorSpaceSignature = 0x43434C52 // 'CCLR'
	cmsSig13colorData cmsColorSpaceSignature = 0x44434C52 // 'DCLR'
	cmsSig14colorData cmsColorSpaceSignature = 0x45434C52 // 'ECLR'
	cmsSig15colorData cmsColorSpaceSignature = 0x46434C52 // 'FCLR'

	cmsSigLuvKData cmsColorSpaceSignature = 0x4C75764B // 'LuvK'
)

type cmsTechnologySignature uint32

const (
	cmsSigDigitalCamera              cmsTechnologySignature = 0x6463616D // 'dcam'
	cmsSigFilmScanner                cmsTechnologySignature = 0x6673636E // 'fscn'
	cmsSigReflectiveScanner          cmsTechnologySignature = 0x7273636E // 'rscn'
	cmsSigInkJetPrinter              cmsTechnologySignature = 0x696A6574 // 'ijet'
	cmsSigThermalWaxPrinter          cmsTechnologySignature = 0x74776178 // 'twax'
	cmsSigElectrophotographicPrinter cmsTechnologySignature = 0x6570686F // 'epho'
	cmsSigElectrostaticPrinter       cmsTechnologySignature = 0x65737461 // 'esta'
	cmsSigDyeSublimationPrinter      cmsTechnologySignature = 0x64737562 // 'dsub'
	cmsSigPhotographicPaperPrinter   cmsTechnologySignature = 0x7270686F // 'rpho'
	cmsSigFilmWriter                 cmsTechnologySignature = 0x6670726E // 'fprn'
	cmsSigVideoMonitor               cmsTechnologySignature = 0x7669646D // 'vidm'
	cmsSigVideoCamera                cmsTechnologySignature = 0x76696463 // 'vidc'
	cmsSigProjectionTelevision       cmsTechnologySignature = 0x706A7476 // 'pjtv'
	cmsSigCRTDisplay                 cmsTechnologySignature = 0x43525420 // 'CRT '
	cmsSigPMDisplay                  cmsTechnologySignature = 0x504D4420 // 'PMD '
	cmsSigAMDisplay                  cmsTechnologySignature = 0x414D4420 // 'AMD '
	cmsSigPhotoCD                    cmsTechnologySignature = 0x4B504344 // 'KPCD'
	cmsSigPhotoImageSetter           cmsTechnologySignature = 0x696D6773 // 'imgs'
	cmsSigGravure                    cmsTechnologySignature = 0x67726176 // 'grav'
	cmsSigOffsetLithography          cmsTechnologySignature = 0x6F666673 // 'offs'
	cmsSigSilkscreen                 cmsTechnologySignature = 0x73696C6B // 'silk'
	cmsSigFlexography                cmsTechnologySignature = 0x666C6578 // 'flex'
	cmsSigMotionPictureFilmScanner   cmsTechnologySignature = 0x6D706673 // 'mpfs'
	cmsSigMotionPictureFilmRecorder  cmsTechnologySignature = 0x6D706672 // 'mpfr'
	cmsSigDigitalMotionPictureCamera cmsTechnologySignature = 0x646D7063 // 'dmpc'
	cmsSigDigitalCinemaProjector     cmsTechnologySignature = 0x64636A70 // 'dcpj'
)

type cmsStageSignature uint32

const (
	cmsSigCurveSetElemType cmsStageSignature = 0x63767374 // 'cvst'
	cmsSigMatrixElemType   cmsStageSignature = 0x6D617466 // 'matf'
	cmsSigCLutElemType     cmsStageSignature = 0x636C7574 // 'clut'

	cmsSigBAcsElemType cmsStageSignature = 0x62414353 // 'bACS'
	cmsSigEAcsElemType cmsStageSignature = 0x65414353 // 'eACS'

	// Custom from here, not in the ICC Spec
	cmsSigXYZ2LabElemType    cmsStageSignature = 0x6C327820 // 'l2x '
	cmsSigLab2XYZElemType    cmsStageSignature = 0x78326C20 // 'x2l '
	cmsSigNamedColorElemType cmsStageSignature = 0x6E636C20 // 'ncl '
	cmsSigLabV2toV4          cmsStageSignature = 0x32203420 // '2 4 '
	cmsSigLabV4toV2          cmsStageSignature = 0x34203220 // '4 2 '

	// Identities
	cmsSigIdentityElemType cmsStageSignature = 0x69646E20 // 'idn '

	// Float to floatPCS
	cmsSigLab2FloatPCS          cmsStageSignature = 0x64326C20 // 'd2l '
	cmsSigFloatPCS2Lab          cmsStageSignature = 0x6C326420 // 'l2d '
	cmsSigXYZ2FloatPCS          cmsStageSignature = 0x64327820 // 'd2x '
	cmsSigFloatPCS2XYZ          cmsStageSignature = 0x78326420 // 'x2d '
	cmsSigClipNegativesElemType cmsStageSignature = 0x636c7020 // 'clp '
)

// Helper to calculate grid points
func cmsFLAGS_GRIDPOINTS(n int) int {
	return (n & cmsFLAGS_GRIDPOINTS_MASK) << cmsFLAGS_GRIDPOINTS_SHIFT
}

// cmsCIEXYZ represents a color in the CIE XYZ color space
type cmsCIEXYZ struct {
	X cmsFloat64Number
	Y cmsFloat64Number
	Z cmsFloat64Number
}

// cmsCIExyY represents a color in the CIE xyY color space

type cmsCIExyY struct {
	x cmsFloat64Number
	y cmsFloat64Number
	Y cmsFloat64Number //
}

// cmsCIELab represents a color in the CIE Lab color space
type cmsCIELab struct {
	L cmsFloat64Number
	a cmsFloat64Number
	b cmsFloat64Number
}

// cmsCIELCh represents a color in the CIE LCh color space
type cmsCIELCh struct {
	L cmsFloat64Number
	C cmsFloat64Number
	h cmsFloat64Number
}

// cmsJCh represents a color in the JCh color space
type cmsJCh struct {
	J cmsFloat64Number
	C cmsFloat64Number
	h cmsFloat64Number
}

// cmsCIEXYZTRIPLE represents a set of primary colors (Red, Green, Blue) in the CIE XYZ color space
type cmsCIEXYZTRIPLE struct {
	Red   cmsCIEXYZ
	Green cmsCIEXYZ
	Blue  cmsCIEXYZ
}

// cmsCIExyYTRIPLE represents a set of primary colors (Red, Green, Blue) in the CIE xyY color space
type cmsCIExyYTRIPLE struct {
	Red   cmsCIExyY
	Green cmsCIExyY
	Blue  cmsCIExyY
}

type cmsSEQ struct {
	n         cmsUInt32Number
	ContextID cmsContext
	seq       *cmsPSEQDESC
}

type cmsPSEQDESC struct {
	deviceMfg    cmsSignature
	deviceModel  cmsSignature
	attributes   cmsUInt64Number
	technology   cmsTechnologySignature
	ProfileID    cmsProfileID
	Manufacturer *cmsMLU
	Model        *cmsMLU
	Description  *cmsMLU
}

type cmsProfileID struct {
	ID8  [16]cmsUInt8Number
	ID16 [8]cmsUInt16Number
	ID32 [4]cmsUInt32Number
}

const cmsMAXCHANNELS = 16

// Fallback for 64-bit types if not supported (Go inherently supports 64-bit integers, so this is rarely needed).
type (
	cmsUInt64Array [2]cmsUInt32Number
	cmsInt64Array  [2]cmsInt32Number
)

// Derivative types
type (
	cmsSignature        cmsUInt32Number
	cmsU8Fixed8Number   cmsUInt16Number
	cmsS15Fixed16Number cmsInt32Number
	cmsU16Fixed16Number cmsUInt32Number
)

// Boolean type
type cmsBool int

// Utility function for constants like limits (Go has math package for max values).
const (
	cmsBoolTrue  cmsBool = 1
	cmsBoolFalse cmsBool = 0
)

// Ensure proper type sizes at compile-time (if desired, otherwise not necessary in Go due to well-defined type sizes).
const (
	cmsCheckUInt8Size  = uint8(math.MaxUint8) == 255
	cmsCheckInt8Size   = int8(math.MaxInt8) == 127
	cmsCheckUInt16Size = uint16(math.MaxUint16) == 65535
	cmsCheckInt16Size  = int16(math.MaxInt16) == 32767
	cmsCheckUInt32Size = uint32(math.MaxUint32) == 4294967295
	cmsCheckInt32Size  = int32(math.MaxInt32) == 2147483647
	cmsCheckUInt64Size = uint64(math.MaxUint64) == 18446744073709551615
	cmsCheckInt64Size  = int64(math.MaxInt64) == 9223372036854775807
)

// Pixel format description:
// Bit fields for defining the format of a pixel are defined as follows:
//
//   M: Premultiplied alpha (only works when extra samples is 1)
//   A: Floating point -- With this flag we can differentiate 16 bits as float and as int
//   O: Optimized -- previous optimization already returns the final 8-bit value
//   T: Pixeltype
//   F: Flavor (0 = MinIsBlack, 1 = MinIsWhite)
//   P: Planar (0 = Chunky, 1 = Planar)
//   X: Swap 16-bit endianness
//   S: Do swap? (e.g., BGR, KYMC)
//   E: Extra samples
//   C: Channels (Samples per pixel)
//   B: Bytes per sample

// Constants for pixel format bit-field manipulation

// Bit-shift macros for pixel format
func PREMUL_SH(m uint32) uint32     { return m << 23 }
func FLOAT_SH(a uint32) uint32      { return a << 22 }
func OPTIMIZED_SH(s uint32) uint32  { return s << 21 }
func COLORSPACE_SH(s uint32) uint32 { return s << 16 }
func SWAPFIRST_SH(s uint32) uint32  { return s << 14 }
func FLAVOR_SH(s uint32) uint32     { return s << 13 }
func PLANAR_SH(p uint32) uint32     { return p << 12 }
func ENDIAN16_SH(e uint32) uint32   { return e << 11 }
func DOSWAP_SH(e uint32) uint32     { return e << 10 }
func EXTRA_SH(e uint32) uint32      { return e << 7 }
func CHANNELS_SH(c uint32) uint32   { return c << 3 }
func BYTES_SH(b uint32) uint32      { return b }

func T_PREMUL(m uint32) uint32 {
	return (m >> 23) & 1
}

func T_FLOAT(a uint32) uint32 {
	return (a >> 22) & 1
}

func T_OPTIMIZED(o uint32) uint32 {
	return (o >> 21) & 1
}

func T_COLORSPACE(s uint32) uint32 {
	return (s >> 16) & 31
}

func T_SWAPFIRST(s uint32) uint32 {
	return (s >> 14) & 1
}

func T_FLAVOR(s uint32) uint32 {
	return (s >> 13) & 1
}

func T_PLANAR(p uint32) uint32 {
	return (p >> 12) & 1
}

func T_ENDIAN16(e uint32) uint32 {
	return (e >> 11) & 1
}

func T_DOSWAP(e uint32) uint32 {
	return (e >> 10) & 1
}

func T_EXTRA(e uint32) uint32 {
	return (e >> 7) & 7
}

func T_CHANNELS(c uint32) uint32 {
	return (c >> 3) & 15
}

func T_BYTES(b uint32) uint32 {
	return b & 7
}

// FROM_8_TO_16 converts an 8-bit value to a 16-bit value
func FROM_8_TO_16(rgb cmsUInt8Number) cmsUInt16Number {
	return cmsUInt16Number(rgb)<<8 | cmsUInt16Number(rgb)
}

// FROM_16_TO_8 converts a 16-bit value to an 8-bit value
func FROM_16_TO_8(rgb cmsUInt16Number) cmsUInt8Number {
	return cmsUInt8Number(((cmsUInt32Number(rgb)*65281 + 8388608) >> 24) & 0xFF)
}

// memmove copies `n` bytes from `src` to `dst`.
// It works like C's memmove, supporting overlapping memory regions.
func memmove(dst, src unsafe.Pointer, n uintptr) {
	// Create byte slices from the pointers
	dstSlice := *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(dst),
		Len:  int(n),
		Cap:  int(n),
	}))

	srcSlice := *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(src),
		Len:  int(n),
		Cap:  int(n),
	}))

	// Use Go's copy function which handles overlapping memory safely
	copy(dstSlice, srcSlice)
}

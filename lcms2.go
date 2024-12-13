package golcms

import (
	"math"
	//"reflect"
	"unsafe"
)

// Base types
/*type (
	uint8   uint8
	int8    int8
	float32 float32
	float64 float64
)

// 16-bit base types
type (
	uint16 uint16
	int16  int16
)

// 32-bit base types
type (
	uint32 uint32
	int32  int32
)

// 64-bit base types
// These are defined only if Go's types natively support 64-bit integers.
type (
	uint64 uint64
	int64  int64
)*/
const LCMS_VERSION = 2150

// //////////////////LCMS placeholders////////////////////////
// ICC Intents
const (
	INTENT_PERCEPTUAL = iota
	INTENT_RELATIVE_COLORIMETRIC
	INTENT_SATURATION
	INTENT_ABSOLUTE_COLORIMETRIC
)

// Non-ICC intents
const (
	INTENT_PRESERVE_K_ONLY_PERCEPTUAL             = 10
	INTENT_PRESERVE_K_ONLY_RELATIVE_COLORIMETRIC  = 11
	INTENT_PRESERVE_K_ONLY_SATURATION             = 12
	INTENT_PRESERVE_K_PLANE_PERCEPTUAL            = 13
	INTENT_PRESERVE_K_PLANE_RELATIVE_COLORIMETRIC = 14
	INTENT_PRESERVE_K_PLANE_SATURATION            = 15
)

// Some common definitions
const cmsMAX_PATH = 256

// Little CMS specific typedefs

type cmsInfoType int

// Define cmsHPROFILE as unsafe.Pointer to represent a void pointer
type cmsHPROFILE unsafe.Pointer
type cmsHANDLE unsafe.Pointer // Generic handle
type cmsHTRANSFORM unsafe.Pointer
type cmsToneCurve cms_curve_struct

// cmsCreateContext creates a new context with the given plugin and user data.
func cmsCreateContext(plugin unsafe.Pointer, userData unsafe.Pointer) cmsContext

// cmsDeleteContext deletes a given context.
func cmsDeleteContext(contextID cmsContext)

// cmsDupContext duplicates a given context, optionally setting new user data.
func cmsDupContext(contextID cmsContext, newUserData unsafe.Pointer) cmsContext

// cmsGetContextUserData retrieves the user data associated with the given context.
func cmsGetContextUserData(contextID cmsContext) unsafe.Pointer

// Plug-In Registering Functions - see cmsplugin

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
type cmsLogErrorHandlerFunction func(ContextID cmsContext, ErrorCode uint32, Text string)

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

// Common structures in ICC tags
type cmsICCData struct {
	len  uint32
	flag uint32
	data [1]uint8
}

// ICC date time
type cmsDateTimeNumber struct {
	year    uint16
	month   uint16
	day     uint16
	hours   uint16
	minutes uint16
	seconds uint16
}

// ICC XYZ
type cmsEncodedXYZNumber struct {
	X cmsS15Fixed16Number
	Y cmsS15Fixed16Number
	Z cmsS15Fixed16Number
}

// Profile ID as computed by MD5 algorithm
type cmsProfileID struct {
	ID8  [16]uint8
	ID16 [8]uint16
	ID32 [4]uint32
}

// cmsTagTypeSignature represents the base ICC type definitions.
// Base ICC type definitions
const (
	cmsSigChromaticityType          cmsTagTypeSignature = 0x6368726D // 'chrm'
	cmsSigcicpType                  cmsTagTypeSignature = 0x63696370 // 'cicp'
	cmsSigColorantOrderType         cmsTagTypeSignature = 0x636C726F // 'clro'
	cmsSigColorantTableType         cmsTagTypeSignature = 0x636C7274 // 'clrt'
	cmsSigCrdInfoType               cmsTagTypeSignature = 0x63726469 // 'crdi'
	cmsSigCurveType                 cmsTagTypeSignature = 0x63757276 // 'curv'
	cmsSigDataType                  cmsTagTypeSignature = 0x64617461 // 'data'
	cmsSigDictType                  cmsTagTypeSignature = 0x64696374 // 'dict'
	cmsSigDateTimeType              cmsTagTypeSignature = 0x6474696D // 'dtim'
	cmsSigDeviceSettingsType        cmsTagTypeSignature = 0x64657673 // 'devs'
	cmsSigLut16Type                 cmsTagTypeSignature = 0x6d667432 // 'mft2'
	cmsSigLut8Type                  cmsTagTypeSignature = 0x6d667431 // 'mft1'
	cmsSigLutAtoBType               cmsTagTypeSignature = 0x6d414220 // 'mAB '
	cmsSigLutBtoAType               cmsTagTypeSignature = 0x6d424120 // 'mBA '
	cmsSigMeasurementType           cmsTagTypeSignature = 0x6D656173 // 'meas'
	cmsSigMultiLocalizedUnicodeType cmsTagTypeSignature = 0x6D6C7563 // 'mluc'
	cmsSigMultiProcessElementType   cmsTagTypeSignature = 0x6D706574 // 'mpet'
	cmsSigNamedColorType            cmsTagTypeSignature = 0x6E636f6C // 'ncol' -- DEPRECATED!
	cmsSigNamedColor2Type           cmsTagTypeSignature = 0x6E636C32 // 'ncl2'
	cmsSigParametricCurveType       cmsTagTypeSignature = 0x70617261 // 'para'
	cmsSigProfileSequenceDescType   cmsTagTypeSignature = 0x70736571 // 'pseq'
	cmsSigProfileSequenceIdType     cmsTagTypeSignature = 0x70736964 // 'psid'
	cmsSigResponseCurveSet16Type    cmsTagTypeSignature = 0x72637332 // 'rcs2'
	cmsSigS15Fixed16ArrayType       cmsTagTypeSignature = 0x73663332 // 'sf32'
	cmsSigScreeningType             cmsTagTypeSignature = 0x7363726E // 'scrn'
	cmsSigSignatureType             cmsTagTypeSignature = 0x73696720 // 'sig '
	cmsSigTextType                  cmsTagTypeSignature = 0x74657874 // 'text'
	cmsSigTextDescriptionType       cmsTagTypeSignature = 0x64657363 // 'desc'
	cmsSigU16Fixed16ArrayType       cmsTagTypeSignature = 0x75663332 // 'uf32'
	cmsSigUcrBgType                 cmsTagTypeSignature = 0x62666420 // 'bfd '
	cmsSigUInt16ArrayType           cmsTagTypeSignature = 0x75693136 // 'ui16'
	cmsSigUInt32ArrayType           cmsTagTypeSignature = 0x75693332 // 'ui32'
	cmsSigUInt64ArrayType           cmsTagTypeSignature = 0x75693634 // 'ui64'
	cmsSigUInt8ArrayType            cmsTagTypeSignature = 0x75693038 // 'ui08'
	cmsSigVcgtType                  cmsTagTypeSignature = 0x76636774 // 'vcgt'
	cmsSigViewingConditionsType     cmsTagTypeSignature = 0x76696577 // 'view'
	cmsSigXYZType                   cmsTagTypeSignature = 0x58595A20 // 'XYZ '
)

// Base ICC tag definitions
const (
	cmsSigAToB0Tag                          cmsTagSignature = 0x41324230 // 'A2B0'
	cmsSigAToB1Tag                          cmsTagSignature = 0x41324231 // 'A2B1'
	cmsSigAToB2Tag                          cmsTagSignature = 0x41324232 // 'A2B2'
	cmsSigBlueColorantTag                   cmsTagSignature = 0x6258595A // 'bXYZ'
	cmsSigBlueMatrixColumnTag               cmsTagSignature = 0x6258595A // 'bXYZ'
	cmsSigBlueTRCTag                        cmsTagSignature = 0x62545243 // 'bTRC'
	cmsSigBToA0Tag                          cmsTagSignature = 0x42324130 // 'B2A0'
	cmsSigBToA1Tag                          cmsTagSignature = 0x42324131 // 'B2A1'
	cmsSigBToA2Tag                          cmsTagSignature = 0x42324132 // 'B2A2'
	cmsSigCalibrationDateTimeTag            cmsTagSignature = 0x63616C74 // 'calt'
	cmsSigCharTargetTag                     cmsTagSignature = 0x74617267 // 'targ'
	cmsSigChromaticAdaptationTag            cmsTagSignature = 0x63686164 // 'chad'
	cmsSigChromaticityTag                   cmsTagSignature = 0x6368726D // 'chrm'
	cmsSigColorantOrderTag                  cmsTagSignature = 0x636C726F // 'clro'
	cmsSigColorantTableTag                  cmsTagSignature = 0x636C7274 // 'clrt'
	cmsSigColorantTableOutTag               cmsTagSignature = 0x636C6F74 // 'clot'
	cmsSigColorimetricIntentImageStateTag   cmsTagSignature = 0x63696973 // 'ciis'
	cmsSigCopyrightTag                      cmsTagSignature = 0x63707274 // 'cprt'
	cmsSigCrdInfoTag                        cmsTagSignature = 0x63726469 // 'crdi'
	cmsSigDataTag                           cmsTagSignature = 0x64617461 // 'data'
	cmsSigDateTimeTag                       cmsTagSignature = 0x6474696D // 'dtim'
	cmsSigDeviceMfgDescTag                  cmsTagSignature = 0x646D6E64 // 'dmnd'
	cmsSigDeviceModelDescTag                cmsTagSignature = 0x646D6464 // 'dmdd'
	cmsSigDeviceSettingsTag                 cmsTagSignature = 0x64657673 // 'devs'
	cmsSigDToB0Tag                          cmsTagSignature = 0x44324230 // 'D2B0'
	cmsSigDToB1Tag                          cmsTagSignature = 0x44324231 // 'D2B1'
	cmsSigDToB2Tag                          cmsTagSignature = 0x44324232 // 'D2B2'
	cmsSigDToB3Tag                          cmsTagSignature = 0x44324233 // 'D2B3'
	cmsSigBToD0Tag                          cmsTagSignature = 0x42324430 // 'B2D0'
	cmsSigBToD1Tag                          cmsTagSignature = 0x42324431 // 'B2D1'
	cmsSigBToD2Tag                          cmsTagSignature = 0x42324432 // 'B2D2'
	cmsSigBToD3Tag                          cmsTagSignature = 0x42324433 // 'B2D3'
	cmsSigGamutTag                          cmsTagSignature = 0x67616D74 // 'gamt'
	cmsSigGrayTRCTag                        cmsTagSignature = 0x6b545243 // 'kTRC'
	cmsSigGreenColorantTag                  cmsTagSignature = 0x6758595A // 'gXYZ'
	cmsSigGreenMatrixColumnTag              cmsTagSignature = 0x6758595A // 'gXYZ'
	cmsSigGreenTRCTag                       cmsTagSignature = 0x67545243 // 'gTRC'
	cmsSigLuminanceTag                      cmsTagSignature = 0x6C756D69 // 'lumi'
	cmsSigMeasurementTag                    cmsTagSignature = 0x6D656173 // 'meas'
	cmsSigMediaBlackPointTag                cmsTagSignature = 0x626B7074 // 'bkpt'
	cmsSigMediaWhitePointTag                cmsTagSignature = 0x77747074 // 'wtpt'
	cmsSigNamedColorTag                     cmsTagSignature = 0x6E636F6C // 'ncol' // Deprecated by the ICC
	cmsSigNamedColor2Tag                    cmsTagSignature = 0x6E636C32 // 'ncl2'
	cmsSigOutputResponseTag                 cmsTagSignature = 0x72657370 // 'resp'
	cmsSigPerceptualRenderingIntentGamutTag cmsTagSignature = 0x72696730 // 'rig0'
	cmsSigPreview0Tag                       cmsTagSignature = 0x70726530 // 'pre0'
	cmsSigPreview1Tag                       cmsTagSignature = 0x70726531 // 'pre1'
	cmsSigPreview2Tag                       cmsTagSignature = 0x70726532 // 'pre2'
	cmsSigProfileDescriptionTag             cmsTagSignature = 0x64657363 // 'desc'
	cmsSigProfileDescriptionMLTag           cmsTagSignature = 0x6473636D // 'dscm'
	cmsSigProfileSequenceDescTag            cmsTagSignature = 0x70736571 // 'pseq'
	cmsSigProfileSequenceIdTag              cmsTagSignature = 0x70736964 // 'psid'
	cmsSigPs2CRD0Tag                        cmsTagSignature = 0x70736430 // 'psd0'
	cmsSigPs2CRD1Tag                        cmsTagSignature = 0x70736431 // 'psd1'
	cmsSigPs2CRD2Tag                        cmsTagSignature = 0x70736432 // 'psd2'
	cmsSigPs2CRD3Tag                        cmsTagSignature = 0x70736433 // 'psd3'
	cmsSigPs2CSATag                         cmsTagSignature = 0x70733273 // 'ps2s'
	cmsSigPs2RenderingIntentTag             cmsTagSignature = 0x70733269 // 'ps2i'
	cmsSigRedColorantTag                    cmsTagSignature = 0x7258595A // 'rXYZ'
	cmsSigRedMatrixColumnTag                cmsTagSignature = 0x7258595A // 'rXYZ'
	cmsSigRedTRCTag                         cmsTagSignature = 0x72545243 // 'rTRC'
	cmsSigSaturationRenderingIntentGamutTag cmsTagSignature = 0x72696732 // 'rig2'
	cmsSigScreeningDescTag                  cmsTagSignature = 0x73637264 // 'scrd'
	cmsSigScreeningTag                      cmsTagSignature = 0x7363726E // 'scrn'
	cmsSigTechnologyTag                     cmsTagSignature = 0x74656368 // 'tech'
	cmsSigUcrBgTag                          cmsTagSignature = 0x62666420 // 'bfd '
	cmsSigViewingCondDescTag                cmsTagSignature = 0x76756564 // 'vued'
	cmsSigViewingConditionsTag              cmsTagSignature = 0x76696577 // 'view'
	cmsSigVcgtTag                           cmsTagSignature = 0x76636774 // 'vcgt'
	cmsSigMetaTag                           cmsTagSignature = 0x6D657461 // 'meta'
	cmsSigcicpTag                           cmsTagSignature = 0x63696370 // 'cicp'
	cmsSigArgyllArtsTag                     cmsTagSignature = 0x61727473 // 'arts'
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

// ICC Technology tag
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

// cmsProfileClassSignature represents ICC Profile Classes
type cmsProfileClassSignature uint32

const (
	cmsSigInputClass      cmsProfileClassSignature = 0x73636E72 // 'scnr'
	cmsSigDisplayClass    cmsProfileClassSignature = 0x6D6E7472 // 'mntr'
	cmsSigOutputClass     cmsProfileClassSignature = 0x70727472 // 'prtr'
	cmsSigLinkClass       cmsProfileClassSignature = 0x6C696E6B // 'link'
	cmsSigAbstractClass   cmsProfileClassSignature = 0x61627374 // 'abst'
	cmsSigColorSpaceClass cmsProfileClassSignature = 0x73706163 // 'spac'
	cmsSigNamedColorClass cmsProfileClassSignature = 0x6E6D636C // 'nmcl'
)

// Helper to calculate grid points
func cmsFLAGS_GRIDPOINTS(n int) int {
	return (n & cmsFLAGS_GRIDPOINTS_MASK) << cmsFLAGS_GRIDPOINTS_SHIFT
}

// cmsCIEXYZ represents a color in the CIE XYZ color space
type cmsCIEXYZ struct {
	X float64
	Y float64
	Z float64
}

// cmsCIExyY represents a color in the CIE xyY color space

type cmsCIExyY struct {
	x float64
	y float64
	Y float64 //
}

// cmsCIELab represents a color in the CIE Lab color space
type cmsCIELab struct {
	L float64
	a float64
	b float64
}

// cmsCIELCh represents a color in the CIE LCh color space
type cmsCIELCh struct {
	L float64
	C float64
	h float64
}

// cmsJCh represents a color in the JCh color space
type cmsJCh struct {
	J float64
	C float64
	h float64
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
	n         uint32
	ContextID cmsContext
	seq       *cmsPSEQDESC
}

type cmsPSEQDESC struct {
	deviceMfg    cmsSignature
	deviceModel  cmsSignature
	attributes   uint64
	technology   cmsTechnologySignature
	ProfileID    cmsProfileID
	Manufacturer *cmsMLU
	Model        *cmsMLU
	Description  *cmsMLU
}

const cmsMAXCHANNELS = 16

// Fallback for 64-bit types if not supported (Go inherently supports 64-bit integers, so this is rarely needed).
type (
	cmsUInt64Array [2]uint32
	cmsInt64Array  [2]int32
)

// Derivative types
type (
	cmsSignature        uint32
	cmsU8Fixed8Number   uint16
	cmsS15Fixed16Number int32
	cmsU16Fixed16Number uint32
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
func FROM_8_TO_16(rgb uint8) uint16 {
	return uint16(rgb)<<8 | uint16(rgb)
}

// FROM_16_TO_8 converts a 16-bit value to an 8-bit value
func FROM_16_TO_8(rgb uint16) uint8 {
	return uint8(((uint32(rgb)*65281 + 8388608) >> 24) & 0xFF)
}

// cmsIOHANDLER is an alias for _cmsIOHandler.
type cmsIOHANDLER cms_io_handler
type cmsContext *cmsContextStruct

// cmsCurveSegment represents the curve segment structure.
type cmsCurveSegment struct {
	X0            float32
	X1            float32
	Type          int32
	Params        [10]float64
	NGridPoints   uint32
	SampledPoints []float32
}

package golcms

import "github.com/yzigangirova/lcms-go/mem"

//"math"
//"reflect"
//"unsafe"

// yuliana
// this is used instead of pointer to void
// anySlice is a constraint that enforces a type parameter to be a slice of any type.
type anySlice[T any] interface {
	~[]T // Ensures the type is a slice of some type `T`
}

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

// How profiles may be used
const (
	LCMS_USED_AS_INPUT  = 0
	LCMS_USED_AS_OUTPUT = 1
	LCMS_USED_AS_PROOF  = 2
)

type CmsInfoType int

type cmsDICTentry struct {
	Next *cmsDICTentry

	DisplayName  *cmsMLU
	DisplayValue *cmsMLU
	Name         string
	Value        string
}

// ----------------------------------------------------------------------------------------------
// ICC profile internal base types. Strictly, shouldn't be declared in this header, but maybe
// somebody want to use this info for accessing profile header directly, so here it is.

// Profile header -- it is 32-bit aligned, so no issues are expected on alignment
type CmsICCHeader struct {
	Size            uint32                   // Profile size in bytes
	CmmId           cmsSignature             // CMM for this profile
	Version         uint32                   // Format version number
	DeviceClass     cmsProfileClassSignature // Type of profile
	ColorSpace      cmsColorSpaceSignature   // Color space of data
	PCS             cmsColorSpaceSignature   // PCS, XYZ or Lab only
	Date            cmsDateTimeNumber        // Date profile was created
	Magic           cmsSignature             // Magic Number to identify an ICC profile
	Platform        cmsPlatformSignature     // Primary Platform
	Flags           uint32                   // Various bit settings
	Manufacturer    cmsSignature             // Device manufacturer
	Model           uint32                   // Device model number
	Attributes      uint64                   // Device attributes
	RenderingIntent uint32                   // Rendering intent
	Illuminant      cmsEncodedXYZNumber      // Profile illuminant
	Creator         cmsSignature             // Profile creator
	ProfileID       cmsProfileID             // Profile ID using MD5
	Reserved        [28]uint8                // Reserved for future use

}

// ICC base tag
type CmsTagBase struct {
	Sig      cmsTagTypeSignature
	Reserved [4]int8
}

// A tag entry in directory
type CmsTagEntry struct {
	Sig    cmsTagSignature // The tag signature
	Offset uint32          // Start of tag
	Size   uint32          // Size in bytes

}

type CmsHPROFILE any
type CmsHANDLE any // Generic handle
type CmsHTRANSFORM any
type CmsToneCurve cms_curve_struct

// Where to place/locate the stages in the pipeline chain
type cmsStageLoc int

const (
	CmsAT_BEGIN cmsStageLoc = iota
	CmsAT_END
)

// V4 perceptual black
const (
	CmsPERCEPTUAL_BLACK_X = 0.00336
	CmsPERCEPTUAL_BLACK_Y = 0.0034731
	CmsPERCEPTUAL_BLACK_Z = 0.00287
)

// Definitions in ICC spec
const CmsMagicNumber = 0x61637370 // 'acsp'
const lCmsSignature = 0x6c636d73  // 'lcms'

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
type cmsLogErrorHandlerFunction func(ContextID CmsContext, ErrorCode uint32, Text string)

// Define cmsInfoType as int, which should match the type in the C library
// type cmsInfoType C.int
// Constants representing the info type in the CMS library
const (
	cmsInfoDescription CmsInfoType = iota
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
	CmsFLAGS_NOCACHE       = 0x0040 // Inhibit 1-pixel cache
	CmsFLAGS_NOOPTIMIZE    = 0x0100 // Inhibit optimizations
	CmsFLAGS_NULLTRANSFORM = 0x0200 // Don't transform anyway

	// Proofing flags
	CmsFLAGS_GAMUTCHECK   = 0x1000 // Out of Gamut alarm
	CmsFLAGS_SOFTPROOFING = 0x4000 // Do softproofing

	// Misc
	CmsFLAGS_BLACKPOINTCOMPENSATION = 0x2000 // Black point compensation
	CmsFLAGS_NOWHITEONWHITEFIXUP    = 0x0004 // Don't fix scum dot
	CmsFLAGS_HIGHRESPRECALC         = 0x0400 // Use more memory for better accuracy
	CmsFLAGS_LOWRESPRECALC          = 0x0800 // Use less memory to minimize resources

	// For devicelink creation
	CmsFLAGS_8BITS_DEVICELINK = 0x0008 // Create 8-bit devicelinks
	CmsFLAGS_GUESSDEVICECLASS = 0x0020 // Guess device class for transform2devicelink
	CmsFLAGS_KEEP_SEQUENCE    = 0x0080 // Keep profile sequence for devicelink creation

	// Specific to particular optimizations
	CmsFLAGS_FORCE_CLUT              = 0x0002 // Force CLUT optimization
	CmsFLAGS_CLUT_POST_LINEARIZATION = 0x0001 // Create postlinearization tables if possible
	CmsFLAGS_CLUT_PRE_LINEARIZATION  = 0x0010 // Create prelinearization tables if possible

	// Specific to unbounded mode
	CmsFLAGS_NONEGATIVES = 0x8000 // Prevent negative numbers in floating-point transforms

	// Copy alpha channels when transforming
	CmsFLAGS_COPY_ALPHA = 0x04000000 // Alpha channels are copied on CmsDoTransform()

	// Fine-tune control over number of gridpoints
	CmsFLAGS_GRIDPOINTS_MASK  = 0xFF
	CmsFLAGS_GRIDPOINTS_SHIFT = 16

	// CRD special
	CmsFLAGS_NODEFAULTRESOURCEDEF = 0x01000000 // No default resource definitions
)

// Common structures in ICC tags
type cmsICCData struct {
	Len  uint32
	Flag uint32
	Data []uint8
}

// ICC date time
type cmsDateTimeNumber struct {
	Year    uint16
	Month   uint16
	Day     uint16
	Hours   uint16
	Minutes uint16
	Seconds uint16
}

// ICC XYZ
type cmsEncodedXYZNumber struct {
	X cmsS15Fixed16Number
	Y cmsS15Fixed16Number
	Z cmsS15Fixed16Number
}

// Profile ID as computed by MD5 algorithm
type cmsProfileID [16]byte

// cmsTagTypeSignature represents the base ICC type definitions.
// Base ICC type definitions
const (
	CmsSigChromaticityType          cmsTagTypeSignature = 0x6368726D // 'chrm'
	CmsSigcicpType                  cmsTagTypeSignature = 0x63696370 // 'cicp'
	CmsSigColorantOrderType         cmsTagTypeSignature = 0x636C726F // 'clro'
	CmsSigColorantTableType         cmsTagTypeSignature = 0x636C7274 // 'clrt'
	CmsSigCrdInfoType               cmsTagTypeSignature = 0x63726469 // 'crdi'
	CmsSigCurveType                 cmsTagTypeSignature = 0x63757276 // 'curv'
	CmsSigDataType                  cmsTagTypeSignature = 0x64617461 // 'data'
	CmsSigDictType                  cmsTagTypeSignature = 0x64696374 // 'dict'
	CmsSigDateTimeType              cmsTagTypeSignature = 0x6474696D // 'dtim'
	CmsSigDeviceSettingsType        cmsTagTypeSignature = 0x64657673 // 'devs'
	CmsSigLut16Type                 cmsTagTypeSignature = 0x6d667432 // 'mft2'
	CmsSigLut8Type                  cmsTagTypeSignature = 0x6d667431 // 'mft1'
	CmsSigLutAtoBType               cmsTagTypeSignature = 0x6d414220 // 'mAB '
	CmsSigLutBtoAType               cmsTagTypeSignature = 0x6d424120 // 'mBA '
	CmsSigMeasurementType           cmsTagTypeSignature = 0x6D656173 // 'meas'
	CmsSigMultiLocalizedUnicodeType cmsTagTypeSignature = 0x6D6C7563 // 'mluc'
	CmsSigMultiProcessElementType   cmsTagTypeSignature = 0x6D706574 // 'mpet'
	CmsSigNamedColorType            cmsTagTypeSignature = 0x6E636f6C // 'ncol' -- DEPRECATED!
	CmsSigNamedColor2Type           cmsTagTypeSignature = 0x6E636C32 // 'ncl2'
	CmsSigParametricCurveType       cmsTagTypeSignature = 0x70617261 // 'para'
	CmsSigProfileSequenceDescType   cmsTagTypeSignature = 0x70736571 // 'pseq'
	CmsSigProfileSequenceIdType     cmsTagTypeSignature = 0x70736964 // 'psid'
	CmsSigResponseCurveSet16Type    cmsTagTypeSignature = 0x72637332 // 'rcs2'
	CmsSigS15Fixed16ArrayType       cmsTagTypeSignature = 0x73663332 // 'sf32'
	CmsSigScreeningType             cmsTagTypeSignature = 0x7363726E // 'scrn'
	CmsSigSignatureType             cmsTagTypeSignature = 0x73696720 // 'sig '
	CmsSigTextType                  cmsTagTypeSignature = 0x74657874 // 'text'
	CmsSigTextDescriptionType       cmsTagTypeSignature = 0x64657363 // 'desc'
	CmsSigU16Fixed16ArrayType       cmsTagTypeSignature = 0x75663332 // 'uf32'
	CmsSigUcrBgType                 cmsTagTypeSignature = 0x62666420 // 'bfd '
	CmsSigUInt16ArrayType           cmsTagTypeSignature = 0x75693136 // 'ui16'
	CmsSigUInt32ArrayType           cmsTagTypeSignature = 0x75693332 // 'ui32'
	CmsSigUInt64ArrayType           cmsTagTypeSignature = 0x75693634 // 'ui64'
	CmsSigUInt8ArrayType            cmsTagTypeSignature = 0x75693038 // 'ui08'
	CmsSigVcgtType                  cmsTagTypeSignature = 0x76636774 // 'vcgt'
	CmsSigViewingConditionsType     cmsTagTypeSignature = 0x76696577 // 'view'
	CmsSigXYZType                   cmsTagTypeSignature = 0x58595A20 // 'XYZ '
)

// Base ICC tag definitions
const (
	CmsSigAToB0Tag                          cmsTagSignature = 0x41324230 // 'A2B0'
	CmsSigAToB1Tag                          cmsTagSignature = 0x41324231 // 'A2B1'
	CmsSigAToB2Tag                          cmsTagSignature = 0x41324232 // 'A2B2'
	CmsSigBlueColorantTag                   cmsTagSignature = 0x6258595A // 'bXYZ'
	CmsSigBlueMatrixColumnTag               cmsTagSignature = 0x6258595A // 'bXYZ'
	CmsSigBlueTRCTag                        cmsTagSignature = 0x62545243 // 'bTRC'
	CmsSigBToA0Tag                          cmsTagSignature = 0x42324130 // 'B2A0'
	CmsSigBToA1Tag                          cmsTagSignature = 0x42324131 // 'B2A1'
	CmsSigBToA2Tag                          cmsTagSignature = 0x42324132 // 'B2A2'
	CmsSigCalibrationDateTimeTag            cmsTagSignature = 0x63616C74 // 'calt'
	CmsSigCharTargetTag                     cmsTagSignature = 0x74617267 // 'targ'
	CmsSigChromaticAdaptationTag            cmsTagSignature = 0x63686164 // 'chad'
	CmsSigChromaticityTag                   cmsTagSignature = 0x6368726D // 'chrm'
	CmsSigColorantOrderTag                  cmsTagSignature = 0x636C726F // 'clro'
	CmsSigColorantTableTag                  cmsTagSignature = 0x636C7274 // 'clrt'
	CmsSigColorantTableOutTag               cmsTagSignature = 0x636C6F74 // 'clot'
	CmsSigColorimetricIntentImageStateTag   cmsTagSignature = 0x63696973 // 'ciis'
	CmsSigCopyrightTag                      cmsTagSignature = 0x63707274 // 'cprt'
	CmsSigCrdInfoTag                        cmsTagSignature = 0x63726469 // 'crdi'
	CmsSigDataTag                           cmsTagSignature = 0x64617461 // 'data'
	CmsSigDateTimeTag                       cmsTagSignature = 0x6474696D // 'dtim'
	CmsSigDeviceMfgDescTag                  cmsTagSignature = 0x646D6E64 // 'dmnd'
	CmsSigDeviceModelDescTag                cmsTagSignature = 0x646D6464 // 'dmdd'
	CmsSigDeviceSettingsTag                 cmsTagSignature = 0x64657673 // 'devs'
	CmsSigDToB0Tag                          cmsTagSignature = 0x44324230 // 'D2B0'
	CmsSigDToB1Tag                          cmsTagSignature = 0x44324231 // 'D2B1'
	CmsSigDToB2Tag                          cmsTagSignature = 0x44324232 // 'D2B2'
	CmsSigDToB3Tag                          cmsTagSignature = 0x44324233 // 'D2B3'
	CmsSigBToD0Tag                          cmsTagSignature = 0x42324430 // 'B2D0'
	CmsSigBToD1Tag                          cmsTagSignature = 0x42324431 // 'B2D1'
	CmsSigBToD2Tag                          cmsTagSignature = 0x42324432 // 'B2D2'
	CmsSigBToD3Tag                          cmsTagSignature = 0x42324433 // 'B2D3'
	CmsSigGamutTag                          cmsTagSignature = 0x67616D74 // 'gamt'
	CmsSigGrayTRCTag                        cmsTagSignature = 0x6b545243 // 'kTRC'
	CmsSigGreenColorantTag                  cmsTagSignature = 0x6758595A // 'gXYZ'
	CmsSigGreenMatrixColumnTag              cmsTagSignature = 0x6758595A // 'gXYZ'
	CmsSigGreenTRCTag                       cmsTagSignature = 0x67545243 // 'gTRC'
	CmsSigLuminanceTag                      cmsTagSignature = 0x6C756D69 // 'lumi'
	CmsSigMeasurementTag                    cmsTagSignature = 0x6D656173 // 'meas'
	CmsSigMediaBlackPointTag                cmsTagSignature = 0x626B7074 // 'bkpt'
	CmsSigMediaWhitePointTag                cmsTagSignature = 0x77747074 // 'wtpt'
	CmsSigNamedColorTag                     cmsTagSignature = 0x6E636F6C // 'ncol' // Deprecated by the ICC
	CmsSigNamedColor2Tag                    cmsTagSignature = 0x6E636C32 // 'ncl2'
	CmsSigOutputResponseTag                 cmsTagSignature = 0x72657370 // 'resp'
	CmsSigPerceptualRenderingIntentGamutTag cmsTagSignature = 0x72696730 // 'rig0'
	CmsSigPreview0Tag                       cmsTagSignature = 0x70726530 // 'pre0'
	CmsSigPreview1Tag                       cmsTagSignature = 0x70726531 // 'pre1'
	CmsSigPreview2Tag                       cmsTagSignature = 0x70726532 // 'pre2'
	CmsSigProfileDescriptionTag             cmsTagSignature = 0x64657363 // 'desc'
	CmsSigProfileDescriptionMLTag           cmsTagSignature = 0x6473636D // 'dscm'
	CmsSigProfileSequenceDescTag            cmsTagSignature = 0x70736571 // 'pseq'
	CmsSigProfileSequenceIdTag              cmsTagSignature = 0x70736964 // 'psid'
	CmsSigPs2CRD0Tag                        cmsTagSignature = 0x70736430 // 'psd0'
	CmsSigPs2CRD1Tag                        cmsTagSignature = 0x70736431 // 'psd1'
	CmsSigPs2CRD2Tag                        cmsTagSignature = 0x70736432 // 'psd2'
	CmsSigPs2CRD3Tag                        cmsTagSignature = 0x70736433 // 'psd3'
	CmsSigPs2CSATag                         cmsTagSignature = 0x70733273 // 'ps2s'
	CmsSigPs2RenderingIntentTag             cmsTagSignature = 0x70733269 // 'ps2i'
	CmsSigRedColorantTag                    cmsTagSignature = 0x7258595A // 'rXYZ'
	CmsSigRedMatrixColumnTag                cmsTagSignature = 0x7258595A // 'rXYZ'
	CmsSigRedTRCTag                         cmsTagSignature = 0x72545243 // 'rTRC'
	CmsSigSaturationRenderingIntentGamutTag cmsTagSignature = 0x72696732 // 'rig2'
	CmsSigScreeningDescTag                  cmsTagSignature = 0x73637264 // 'scrd'
	CmsSigScreeningTag                      cmsTagSignature = 0x7363726E // 'scrn'
	CmsSigTechnologyTag                     cmsTagSignature = 0x74656368 // 'tech'
	CmsSigUcrBgTag                          cmsTagSignature = 0x62666420 // 'bfd '
	CmsSigViewingCondDescTag                cmsTagSignature = 0x76756564 // 'vued'
	CmsSigViewingConditionsTag              cmsTagSignature = 0x76696577 // 'view'
	CmsSigVcgtTag                           cmsTagSignature = 0x76636774 // 'vcgt'
	CmsSigMetaTag                           cmsTagSignature = 0x6D657461 // 'meta'
	CmsSigcicpTag                           cmsTagSignature = 0x63696370 // 'cicp'
	CmsSigArgyllArtsTag                     cmsTagSignature = 0x61727473 // 'arts'
)

type cmsColorSpaceSignature uint32

const (
	CmsSigXYZData   cmsColorSpaceSignature = 0x58595A20 // 'XYZ '
	CmsSigLabData   cmsColorSpaceSignature = 0x4C616220 // 'Lab '
	CmsSigLuvData   cmsColorSpaceSignature = 0x4C757620 // 'Luv '
	CmsSigYCbCrData cmsColorSpaceSignature = 0x59436272 // 'YCbr'
	CmsSigYxyData   cmsColorSpaceSignature = 0x59787920 // 'Yxy '
	CmsSigRgbData   cmsColorSpaceSignature = 0x52474220 // 'RGB '
	CmsSigGrayData  cmsColorSpaceSignature = 0x47524159 // 'GRAY'
	CmsSigHsvData   cmsColorSpaceSignature = 0x48535620 // 'HSV '
	CmsSigHlsData   cmsColorSpaceSignature = 0x484C5320 // 'HLS '
	CmsSigCmykData  cmsColorSpaceSignature = 0x434D594B // 'CMYK'
	CmsSigCmyData   cmsColorSpaceSignature = 0x434D5920 // 'CMY '

	CmsSigMCH1Data cmsColorSpaceSignature = 0x4D434831 // 'MCH1'
	CmsSigMCH2Data cmsColorSpaceSignature = 0x4D434832 // 'MCH2'
	CmsSigMCH3Data cmsColorSpaceSignature = 0x4D434833 // 'MCH3'
	CmsSigMCH4Data cmsColorSpaceSignature = 0x4D434834 // 'MCH4'
	CmsSigMCH5Data cmsColorSpaceSignature = 0x4D434835 // 'MCH5'
	CmsSigMCH6Data cmsColorSpaceSignature = 0x4D434836 // 'MCH6'
	CmsSigMCH7Data cmsColorSpaceSignature = 0x4D434837 // 'MCH7'
	CmsSigMCH8Data cmsColorSpaceSignature = 0x4D434838 // 'MCH8'
	CmsSigMCH9Data cmsColorSpaceSignature = 0x4D434839 // 'MCH9'
	CmsSigMCHAData cmsColorSpaceSignature = 0x4D434841 // 'MCHA'
	CmsSigMCHBData cmsColorSpaceSignature = 0x4D434842 // 'MCHB'
	CmsSigMCHCData cmsColorSpaceSignature = 0x4D434843 // 'MCHC'
	CmsSigMCHDData cmsColorSpaceSignature = 0x4D434844 // 'MCHD'
	CmsSigMCHEData cmsColorSpaceSignature = 0x4D434845 // 'MCHE'
	CmsSigMCHFData cmsColorSpaceSignature = 0x4D434846 // 'MCHF'

	CmsSigNamedData  cmsColorSpaceSignature = 0x6E6D636C // 'nmcl'
	CmsSig1colorData cmsColorSpaceSignature = 0x31434C52 // '1CLR'
	CmsSig2colorData cmsColorSpaceSignature = 0x32434C52 // '2CLR'
	CmsSig3colorData cmsColorSpaceSignature = 0x33434C52 // '3CLR'
	CmsSig4colorData cmsColorSpaceSignature = 0x34434C52 // '4CLR'
	CmsSig5colorData cmsColorSpaceSignature = 0x35434C52 // '5CLR'
	CmsSig6colorData cmsColorSpaceSignature = 0x36434C52 // '6CLR'
	CmsSig7colorData cmsColorSpaceSignature = 0x37434C52 // '7CLR'
	CmsSig8colorData cmsColorSpaceSignature = 0x38434C52 // '8CLR'
	CmsSig9colorData cmsColorSpaceSignature = 0x39434C52 // '9CLR'

	CmsSig10colorData cmsColorSpaceSignature = 0x41434C52 // 'ACLR'
	CmsSig11colorData cmsColorSpaceSignature = 0x42434C52 // 'BCLR'
	CmsSig12colorData cmsColorSpaceSignature = 0x43434C52 // 'CCLR'
	CmsSig13colorData cmsColorSpaceSignature = 0x44434C52 // 'DCLR'
	CmsSig14colorData cmsColorSpaceSignature = 0x45434C52 // 'ECLR'
	CmsSig15colorData cmsColorSpaceSignature = 0x46434C52 // 'FCLR'

	CmsSigLuvKData cmsColorSpaceSignature = 0x4C75764B // 'LuvK'
)

type cmsTechnologySignature uint32

// ICC Technology tag
const (
	CmsSigDigitalCamera              cmsTechnologySignature = 0x6463616D // 'dcam'
	CmsSigFilmScanner                cmsTechnologySignature = 0x6673636E // 'fscn'
	CmsSigReflectiveScanner          cmsTechnologySignature = 0x7273636E // 'rscn'
	CmsSigInkJetPrinter              cmsTechnologySignature = 0x696A6574 // 'ijet'
	CmsSigThermalWaxPrinter          cmsTechnologySignature = 0x74776178 // 'twax'
	CmsSigElectrophotographicPrinter cmsTechnologySignature = 0x6570686F // 'epho'
	CmsSigElectrostaticPrinter       cmsTechnologySignature = 0x65737461 // 'esta'
	CmsSigDyeSublimationPrinter      cmsTechnologySignature = 0x64737562 // 'dsub'
	CmsSigPhotographicPaperPrinter   cmsTechnologySignature = 0x7270686F // 'rpho'
	CmsSigFilmWriter                 cmsTechnologySignature = 0x6670726E // 'fprn'
	CmsSigVideoMonitor               cmsTechnologySignature = 0x7669646D // 'vidm'
	CmsSigVideoCamera                cmsTechnologySignature = 0x76696463 // 'vidc'
	CmsSigProjectionTelevision       cmsTechnologySignature = 0x706A7476 // 'pjtv'
	CmsSigCRTDisplay                 cmsTechnologySignature = 0x43525420 // 'CRT '
	CmsSigPMDisplay                  cmsTechnologySignature = 0x504D4420 // 'PMD '
	CmsSigAMDisplay                  cmsTechnologySignature = 0x414D4420 // 'AMD '
	CmsSigPhotoCD                    cmsTechnologySignature = 0x4B504344 // 'KPCD'
	CmsSigPhotoImageSetter           cmsTechnologySignature = 0x696D6773 // 'imgs'
	CmsSigGravure                    cmsTechnologySignature = 0x67726176 // 'grav'
	CmsSigOffsetLithography          cmsTechnologySignature = 0x6F666673 // 'offs'
	CmsSigSilkscreen                 cmsTechnologySignature = 0x73696C6B // 'silk'
	CmsSigFlexography                cmsTechnologySignature = 0x666C6578 // 'flex'
	CmsSigMotionPictureFilmScanner   cmsTechnologySignature = 0x6D706673 // 'mpfs'
	CmsSigMotionPictureFilmRecorder  cmsTechnologySignature = 0x6D706672 // 'mpfr'
	CmsSigDigitalMotionPictureCamera cmsTechnologySignature = 0x646D7063 // 'dmpc'
	CmsSigDigitalCinemaProjector     cmsTechnologySignature = 0x64636A70 // 'dcpj'
)

type cmsStageSignature uint32

const (
	CmsSigCurveSetElemType cmsStageSignature = 0x63767374 // 'cvst'
	CmsSigMatrixElemType   cmsStageSignature = 0x6D617466 // 'matf'
	CmsSigCLutElemType     cmsStageSignature = 0x636C7574 // 'clut'

	CmsSigBAcsElemType cmsStageSignature = 0x62414353 // 'bACS'
	CmsSigEAcsElemType cmsStageSignature = 0x65414353 // 'eACS'

	// Custom from here, not in the ICC Spec
	CmsSigXYZ2LabElemType    cmsStageSignature = 0x6C327820 // 'l2x '
	CmsSigLab2XYZElemType    cmsStageSignature = 0x78326C20 // 'x2l '
	CmsSigNamedColorElemType cmsStageSignature = 0x6E636C20 // 'ncl '
	CmsSigLabV2toV4          cmsStageSignature = 0x32203420 // '2 4 '
	CmsSigLabV4toV2          cmsStageSignature = 0x34203220 // '4 2 '

	// Identities
	CmsSigIdentityElemType cmsStageSignature = 0x69646E20 // 'idn '

	// Float to floatPCS
	CmsSigLab2FloatPCS          cmsStageSignature = 0x64326C20 // 'd2l '
	CmsSigFloatPCS2Lab          cmsStageSignature = 0x6C326420 // 'l2d '
	CmsSigXYZ2FloatPCS          cmsStageSignature = 0x64327820 // 'd2x '
	CmsSigFloatPCS2XYZ          cmsStageSignature = 0x78326420 // 'x2d '
	CmsSigClipNegativesElemType cmsStageSignature = 0x636c7020 // 'clp '
)

// Types of CurveElements
type cmsCurveSegSignature uint32

const (
	CmsSigFormulaCurveSeg cmsCurveSegSignature = 0x70617266 // 'parf'
	CmsSigSampledCurveSeg cmsCurveSegSignature = 0x73616D66 // 'samf'
	CmsSigSegmentedCurve  cmsCurveSegSignature = 0x63757266 // 'curf'
)

// Used in ResponseCurveType
const (
	CmsSigStatusA uint32 = 0x53746141 // 'StaA'
	CmsSigStatusE uint32 = 0x53746145 // 'StaE'
	CmsSigStatusI uint32 = 0x53746149 // 'StaI'
	CmsSigStatusT uint32 = 0x53746154 // 'StaT'
	CmsSigStatusM uint32 = 0x5374614D // 'StaM'
	CmsSigDN      uint32 = 0x444E2020 // 'DN  '
	CmsSigDNP     uint32 = 0x444E2050 // 'DN P'
	CmsSigDNN     uint32 = 0x444E4E20 // 'DNN '
	CmsSigDNNP    uint32 = 0x444E4E50 // 'DNNP'
)

// Device attributes, currently defined values correspond to the low 4 bytes
// of the 8-byte attribute quantity
const (
	cmsReflective   uint32 = 0
	cmsTransparency uint32 = 1
	cmsGlossy       uint32 = 0
	cmsMatte        uint32 = 2
)

type cmsPlatformSignature uint32

const (
	CmsSigMacintosh cmsPlatformSignature = 0x4150504C // 'APPL'
	CmsSigMicrosoft cmsPlatformSignature = 0x4D534654 // 'MSFT'
	CmsSigSolaris   cmsPlatformSignature = 0x53554E57 // 'SUNW'
	CmsSigSGI       cmsPlatformSignature = 0x53474920 // 'SGI '
	CmsSigTaligent  cmsPlatformSignature = 0x54474E54 // 'TGNT'
	CmsSigUnices    cmsPlatformSignature = 0x2A6E6978 // '*nix'   // From argyll -- Not official

)

// cmsProfileClassSignature represents ICC Profile Classes
type cmsProfileClassSignature uint32

const (
	CmsSigInputClass      cmsProfileClassSignature = 0x73636E72 // 'scnr'
	CmsSigDisplayClass    cmsProfileClassSignature = 0x6D6E7472 // 'mntr'
	CmsSigOutputClass     cmsProfileClassSignature = 0x70727472 // 'prtr'
	CmsSigLinkClass       cmsProfileClassSignature = 0x6C696E6B // 'link'
	CmsSigAbstractClass   cmsProfileClassSignature = 0x61627374 // 'abst'
	CmsSigColorSpaceClass cmsProfileClassSignature = 0x73706163 // 'spac'
	CmsSigNamedColorClass cmsProfileClassSignature = 0x6E6D636C // 'nmcl'
)

// Helper to calculate grid points
func CmsFLAGS_GRIDPOINTS(n int) int {
	return (n & CmsFLAGS_GRIDPOINTS_MASK) << CmsFLAGS_GRIDPOINTS_SHIFT
}

// cmsCIEXYZ represents a color in the CIE XYZ color space
type cmsCIEXYZ struct {
	X float64
	Y float64
	Z float64
}

// CmsCIExyY represents a color in the CIE xyY color space

type CmsCIExyY struct {
	X_small float64
	Y_small float64
	Y_large float64 //
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

// CmsCIExyYTRIPLE represents a set of primary colors (Red, Green, Blue) in the CIE xyY color space
type CmsCIExyYTRIPLE struct {
	Red   CmsCIExyY
	Green CmsCIExyY
	Blue  CmsCIExyY
}

type cmsSEQ struct {
	n         uint32
	ContextID CmsContext
	seq       []cmsPSEQDESC
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

// Constants for Illuminant types
const (
	cmsILLUMINANT_TYPE_UNKNOWN = 0x0000000
	cmsILLUMINANT_TYPE_D50     = 0x0000001
	cmsILLUMINANT_TYPE_D65     = 0x0000002
	cmsILLUMINANT_TYPE_D93     = 0x0000003
	cmsILLUMINANT_TYPE_F2      = 0x0000004
	cmsILLUMINANT_TYPE_D55     = 0x0000005
	cmsILLUMINANT_TYPE_A       = 0x0000006
	cmsILLUMINANT_TYPE_E       = 0x0000007
	cmsILLUMINANT_TYPE_F8      = 0x0000008
)

// cmsICCMeasurementConditions represents measurement conditions in ICC profiles.
type cmsICCMeasurementConditions struct {
	Observer       uint32    // 0 = unknown, 1 = CIE 1931, 2 = CIE 1964
	Backing        cmsCIEXYZ // Value of backing
	Geometry       uint32    // 0 = unknown, 1 = 45/0, 0/45, 2 = 0d, d/0
	Flare          float64   // 0..1.0
	IlluminantType uint32    // Illuminant type
}

// cmsICCViewingConditions represents viewing conditions in ICC profiles.
type cmsICCViewingConditions struct {
	IlluminantXYZ  cmsCIEXYZ // Not the same struct as CAM02
	SurroundXYZ    cmsCIEXYZ // For storing the tag
	IlluminantType uint32    // Viewing condition
}

// cmsVideoSignalType represents video signal characteristics.
type cmsVideoSignalType struct {
	ColourPrimaries         uint8 // Recommendation ITU-T H.273
	TransferCharacteristics uint8 // (ISO/IEC 23091-2)
	MatrixCoefficients      uint8
	VideoFullRangeFlag      uint8
}

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
type CmsContext *CmsContextStruct

// cmsCurveSegment represents the curve segment structure.
type cmsCurveSegment struct {
	X0            float32
	X1            float32
	Type          int32
	Params        [10]float64
	NGridPoints   uint32
	SampledPoints []float32
}

type cmsSAMPLER16 func(mm mem.Manager, In []uint16, Out []uint16, cargo any) int32

//lint:ignore U1000 kept for parity with lcms; used in future ports
type cmsSAMPLERFLOAT func(mm mem.Manager, In []float32, Out []float32, cargo any) int32

// Use this flag to prevent changes being written to destination
const SAMPLER_INSPECT = 0x01000000

type cmsScreeningChannel struct {
	Frequency   float64
	ScreenAngle float64
	SpotShape   uint32
}

type cmsScreening struct {
	Flag      uint32
	NChannels uint32
	Channels  [cmsMAXCHANNELS]cmsScreeningChannel
}

// Undercolorremoval & black generation -------------------------------------------------------------------------------------

type cmsUcrBg struct {
	Ucr  *CmsToneCurve
	Bg   *CmsToneCurve
	Desc *cmsMLU
}

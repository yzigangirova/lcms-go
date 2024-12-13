package golcms

import (
	"time"
	"unsafe"
)

// cmsGetHeaderRenderingIntent retrieves the rendering intent from the profile
func cmsGetHeaderRenderingIntent(hProfile unsafe.Pointer) uint32 {
	icc := (*cmsICCPROFILE)(hProfile)
	return icc.RenderingIntent
}

// cmsSetHeaderRenderingIntent sets the rendering intent in the profile
func cmsSetHeaderRenderingIntent(hProfile unsafe.Pointer, RenderingIntent uint32) {
	icc := (*cmsICCPROFILE)(hProfile)
	icc.RenderingIntent = RenderingIntent
}

// cmsGetHeaderFlags retrieves the flags from the profile
func cmsGetHeaderFlags(hProfile unsafe.Pointer) uint32 {
	icc := (*cmsICCPROFILE)(hProfile)
	return icc.Flags
}

// cmsSetHeaderFlags sets the flags in the profile
func cmsSetHeaderFlags(hProfile unsafe.Pointer, Flags uint32) {
	icc := (*cmsICCPROFILE)(hProfile)
	icc.Flags = Flags
}

// cmsGetHeaderManufacturer retrieves the manufacturer from the profile
func cmsGetHeaderManufacturer(hProfile unsafe.Pointer) uint32 {
	icc := (*cmsICCPROFILE)(hProfile)
	return icc.Manufacturer

}

// cmsSetHeaderManufacturer sets the manufacturer in the profile
func cmsSetHeaderManufacturer(hProfile unsafe.Pointer, Manufacturer uint32) {
	icc := (*cmsICCPROFILE)(hProfile)
	icc.Manufacturer = Manufacturer
}

// cmsGetHeaderCreator retrieves the creator from the profile
func cmsGetHeaderCreator(hProfile unsafe.Pointer) uint32 {
	icc := (*cmsICCPROFILE)(hProfile)
	return icc.Creator
}

// cmsGetHeaderModel retrieves the model from the profile
func cmsGetHeaderModel(hProfile unsafe.Pointer) uint32 {
	icc := (*cmsICCPROFILE)(hProfile)
	return icc.Model
}

// cmsSetHeaderModel sets the model in the profile
func cmsSetHeaderModel(hProfile unsafe.Pointer, Model uint32) {
	icc := (*cmsICCPROFILE)(hProfile)
	icc.Model = Model
}

// cmsGetHeaderAttributes retrieves the attributes from the profile
func cmsGetHeaderAttributes(hProfile unsafe.Pointer, Flags *uint64) {
	icc := (*cmsICCPROFILE)(hProfile)
	memmove(unsafe.Pointer(Flags), unsafe.Pointer(&icc.Attributes), unsafe.Sizeof(icc.Attributes))
}

// cmsSetHeaderAttributes sets the attributes in the profile
func cmsSetHeaderAttributes(hProfile unsafe.Pointer, Flags uint64) {
	icc := (*cmsICCPROFILE)(hProfile)
	memmove(unsafe.Pointer(&icc.Attributes), unsafe.Pointer(&Flags), unsafe.Sizeof(icc.Attributes))
}

// cmsGetHeaderProfileID retrieves the profile ID from the profile
func cmsGetHeaderProfileID(hProfile unsafe.Pointer, ProfileID *[16]byte) {
	icc := (*cmsICCPROFILE)(hProfile)
	memmove(unsafe.Pointer(ProfileID), unsafe.Pointer(&icc.ProfileID), unsafe.Sizeof(icc.ProfileID))
}

// cmsSetHeaderProfileID sets the profile ID in the profile
func cmsSetHeaderProfileID(hProfile unsafe.Pointer, ProfileID *[16]byte) {
	icc := (*cmsICCPROFILE)(hProfile)
	memmove(unsafe.Pointer(&icc.ProfileID), unsafe.Pointer(ProfileID), unsafe.Sizeof(icc.ProfileID))
}

// cmsGetHeaderCreationDateTime retrieves the creation date and time from the profile
func cmsGetHeaderCreationDateTime(hProfile unsafe.Pointer) time.Time {
	icc := (*cmsICCPROFILE)(hProfile)
	//memmove(unsafe.Pointer(t), unsafe.Pointer(&icc.Created), unsafe.Sizeof(icc.Created))
	return icc.Created
}

// cmsGetPCS retrieves the PCS from the profile
func cmsGetPCS(hProfile unsafe.Pointer) cmsColorSpaceSignature {
	icc := (*cmsICCPROFILE)(hProfile)
	return icc.PCS
}

// cmsSetPCS sets the PCS in the profile
func cmsSetPCS(hProfile unsafe.Pointer, pcs cmsColorSpaceSignature) {
	icc := (*cmsICCPROFILE)(hProfile)
	icc.PCS = pcs
}

// cmsGetColorSpace retrieves the color space from the profile
func cmsGetColorSpace(hProfile unsafe.Pointer) cmsColorSpaceSignature {
	icc := (*cmsICCPROFILE)(hProfile)
	return icc.ColorSpace
}

// cmsSetColorSpace sets the color space in the profile
func cmsSetColorSpace(hProfile unsafe.Pointer, sig cmsColorSpaceSignature) {
	icc := (*cmsICCPROFILE)(hProfile)
	icc.ColorSpace = sig
}

// cmsGetDeviceClass retrieves the device class from the profile
func cmsGetDeviceClass(hProfile unsafe.Pointer) cmsProfileClassSignature {
	icc := (*cmsICCPROFILE)(hProfile)
	return icc.DeviceClass
}

// cmsSetDeviceClass sets the device class in the profile
func cmsSetDeviceClass(hProfile unsafe.Pointer, sig cmsProfileClassSignature) {
	icc := (*cmsICCPROFILE)(hProfile)
	icc.DeviceClass = sig
}

// cmsGetEncodedICCversion retrieves the ICC version from the profile
func cmsGetEncodedICCversion(hProfile unsafe.Pointer) uint32 {
	icc := (*cmsICCPROFILE)(hProfile)
	return icc.Version
}

// cmsSetEncodedICCversion sets the ICC version in the profile
func cmsSetEncodedICCversion(hProfile unsafe.Pointer, Version uint32) {
	icc := (*cmsICCPROFILE)(hProfile)
	icc.Version = Version
}

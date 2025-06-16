package golcms

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"time"
	"unsafe"

	//"io"
	"bytes"
	"sync"
)

// Generic I/O, tag dictionary management, profile struct

// IOhandlers are abstractions used by littleCMS to read from whatever file, stream,
// memory block or any storage. Each IOhandler provides implementations for read,
// write, seek and tell functions. LittleCMS code deals with IO across those objects.
// In this way, is easier to add support for new storage media.

// NULL stream, for taking care of used space -------------------------------------

// NULL IOhandler basically does nothing but keep track on how many bytes have been
// written. This is handy when creating profiles, where the file size is needed in the
// header. Then, whole profile is serialized across NULL IOhandler and a second pass
// writes the bytes to the pertinent IOhandler.

// FILENULL represents the null file structure for tracking byte usage.
type FILENULL struct {
	Pointer uint32 // Points to current location
}

// NULLRead simulates reading from a null IOHandler.
// func NULLRead(iohandler *cms_io_handler, buffer []byte, size, count uint32) uint32 {
func NULLRead(iohandler *cms_io_handler, buffer interface{}, size, count uint32) uint32 {
	resData, ok := iohandler.Stream.(*FILENULL)
	if !ok || resData == nil {
		// If Stream is not a *FILENULL, do nothing
		return 0
	}
	length := size * count
	resData.Pointer += length
	return count
}

// NULLSeek simulates seeking in a null IOHandler.
func NULLSeek(iohandler *cms_io_handler, offset uint32) bool {
	resData, ok := iohandler.Stream.(*FILENULL)
	if !ok || resData == nil {
		// If Stream is not a *FILENULL, do nothing
		return false
	}
	resData.Pointer = offset
	return true
}

// NULLTell retrieves the current pointer position in the null IOHandler.
func NULLTell(iohandler *cms_io_handler) uint32 {
	resData, ok := iohandler.Stream.(*FILENULL)
	if !ok || resData == nil {
		// If Stream is not a *FILENULL, do nothing
		return 0
	}
	return resData.Pointer
}

// NULLWrite simulates writing to a null IOHandler.
// func NULLWrite(iohandler *cms_io_handler, size uint32, ptr []byte) bool {
func NULLWrite(iohandler *cms_io_handler, size uint32, ptr []byte) bool {
	resData, ok := iohandler.Stream.(*FILENULL)
	if !ok || resData == nil {
		// If Stream is not a *FILENULL, do nothing
		return false
	}
	resData.Pointer += size
	if resData.Pointer > iohandler.UsedSpace {
		iohandler.UsedSpace = resData.Pointer
	}

	return true
}

// NULLClose closes the null IOHandler and releases associated memory.
func NULLClose(iohandler *cms_io_handler) bool {
	resData, ok := iohandler.Stream.(*FILENULL)
	if !ok || resData == nil {
		// If Stream is not a *FILENULL, do nothing
		return false
	}
	cmsFree(iohandler.ContextID, resData)
	cmsFree(iohandler.ContextID, iohandler)
	return true
}

// cmsOpenIOhandlerFromNULL creates a null IOHandler for tracking space usage.
func cmsOpenIOhandlerFromNULL(ContextID CmsContext) *cmsIOHANDLER {

	// Allocate memory for the IOHandler
	iohandler := allocateStruct[cmsIOHANDLER]()
	if iohandler == nil {
		return nil
	}

	// Allocate memory for the FILENULL structure
	fm := allocateStruct[FILENULL]()
	if fm == nil {
		cmsFree(ContextID, iohandler)
		return nil
	}

	// Initialize the FILENULL structure
	fm.Pointer = 0

	// Initialize the IOHandler structure
	iohandler.ContextID = ContextID
	iohandler.Stream = fm
	iohandler.UsedSpace = 0
	iohandler.ReportedSize = 0
	//iohandler.PhysicalFile[0] = 0

	iohandler.Read = NULLRead
	iohandler.Seek = NULLSeek
	iohandler.Close = NULLClose
	iohandler.Tell = NULLTell
	iohandler.Write = NULLWrite

	return iohandler
}

func cmsOpenIOhandlerFromStream(ContextID CmsContext, stream *os.File) *cmsIOHANDLER {
	if stream == nil {
		cmsSignalError(ContextID, cmsERROR_FILE, "Stream cannot be nil")
		return nil
	}

	// Determine the size of the stream
	fileInfo, err := stream.Stat()
	if err != nil {
		cmsSignalError(ContextID, cmsERROR_FILE, "Cannot get size of stream")
		return nil
	}
	fileSize := fileInfo.Size()

	if fileSize < 0 {
		cmsSignalError(ContextID, cmsERROR_FILE, "Cannot get size of stream")
		return nil
	}

	// Allocate memory for cmsIOHANDLER
	iohandler := allocateStruct[cmsIOHANDLER]()
	if iohandler == nil {
		return nil
	}

	// Initialize the IOHANDLER fields
	iohandler.ContextID = ContextID
	iohandler.Stream = stream
	iohandler.UsedSpace = 0
	iohandler.ReportedSize = uint32(fileSize)
	//iohandler.PhysicalFile //init to zeros

	// Assign function pointers
	iohandler.Read = FileRead
	iohandler.Seek = FileSeek
	iohandler.Close = FileClose
	iohandler.Tell = FileTell
	iohandler.Write = FileWrite

	return iohandler
}

// Close an open IO handler
func cmsCloseIOhandler(io *cmsIOHANDLER) bool {
	return io.Close((*cms_io_handler)(io))
}

// cmsGetHeaderRenderingIntent retrieves the rendering intent from the profile
func cmsGetHeaderRenderingIntent(hProfile CmsHPROFILE) uint32 {
	icc := hProfile.(*cmsICCPROFILE)
	return icc.RenderingIntent
}

// cmsSetHeaderRenderingIntent sets the rendering intent in the profile
func cmsSetHeaderRenderingIntent(hProfile CmsHPROFILE, RenderingIntent uint32) {
	icc := hProfile.(*cmsICCPROFILE)
	icc.RenderingIntent = RenderingIntent
}

// cmsGetHeaderFlags retrieves the flags from the profile
func cmsGetHeaderFlags(hProfile CmsHPROFILE) uint32 {
	icc := hProfile.(*cmsICCPROFILE)
	return icc.Flags
}

// cmsSetHeaderFlags sets the flags in the profile
func cmsSetHeaderFlags(hProfile CmsHPROFILE, Flags uint32) {
	icc := hProfile.(*cmsICCPROFILE)
	icc.Flags = Flags
}

// cmsGetHeaderManufacturer retrieves the manufacturer from the profile
func cmsGetHeaderManufacturer(hProfile CmsHPROFILE) uint32 {
	icc := hProfile.(*cmsICCPROFILE)
	return icc.Manufacturer

}

// cmsSetHeaderManufacturer sets the manufacturer in the profile
func cmsSetHeaderManufacturer(hProfile CmsHPROFILE, Manufacturer uint32) {
	icc := hProfile.(*cmsICCPROFILE)
	icc.Manufacturer = Manufacturer
}

// cmsGetHeaderCreator retrieves the creator from the profile
func cmsGetHeaderCreator(hProfile CmsHPROFILE) uint32 {
	icc := hProfile.(*cmsICCPROFILE)
	return icc.Creator
}

// cmsGetHeaderModel retrieves the model from the profile
func cmsGetHeaderModel(hProfile CmsHPROFILE) uint32 {
	icc := hProfile.(*cmsICCPROFILE)
	return icc.Model
}

// cmsSetHeaderModel sets the model in the profile
func cmsSetHeaderModel(hProfile CmsHPROFILE, Model uint32) {
	icc := hProfile.(*cmsICCPROFILE)
	icc.Model = Model
}

// cmsGetHeaderAttributes retrieves the attributes from the profile
func cmsGetHeaderAttributes(hProfile CmsHPROFILE, Flags *uint64) {
	icc := hProfile.(*cmsICCPROFILE)
	*Flags = icc.Attributes
}

// cmsSetHeaderAttributes sets the attributes in the profile
func cmsSetHeaderAttributes(hProfile CmsHPROFILE, Flags uint64) {
	icc := hProfile.(*cmsICCPROFILE)
	icc.Attributes = Flags
}

// cmsGetHeaderProfileID retrieves the profile ID from the profile
func cmsGetHeaderProfileID(hProfile CmsHPROFILE, ProfileID []byte) {
	icc := hProfile.(*cmsICCPROFILE)
	copy(ProfileID, icc.ProfileID[:])
}

// cmsSetHeaderProfileID sets the profile ID in the profile
func cmsSetHeaderProfileID(hProfile CmsHPROFILE, ProfileID []byte) {
	icc := hProfile.(*cmsICCPROFILE)
	copy(ProfileID, icc.ProfileID[:])
}

// cmsGetHeaderCreationDateTime retrieves the creation date and time from the profile
func cmsGetHeaderCreationDateTime(hProfile CmsHPROFILE) time.Time {
	icc := hProfile.(*cmsICCPROFILE)
	return icc.Created
}

// cmsGetPCS retrieves the PCS from the profile
func cmsGetPCS(hProfile CmsHPROFILE) cmsColorSpaceSignature {
	icc := hProfile.(*cmsICCPROFILE)
	return icc.PCS
}

// cmsSetPCS sets the PCS in the profile
func cmsSetPCS(hProfile CmsHPROFILE, pcs cmsColorSpaceSignature) {
	icc := hProfile.(*cmsICCPROFILE)
	icc.PCS = pcs
}

// cmsGetColorSpace retrieves the color space from the profile
func CmsGetColorSpace(hProfile CmsHPROFILE) cmsColorSpaceSignature {
	icc := hProfile.(*cmsICCPROFILE)
	return icc.ColorSpace
}

// cmsSetColorSpace sets the color space in the profile
func cmsSetColorSpace(hProfile CmsHPROFILE, sig cmsColorSpaceSignature) {
	icc := hProfile.(*cmsICCPROFILE)
	icc.ColorSpace = sig
}

// cmsGetDeviceClass retrieves the device class from the profile
func cmsGetDeviceClass(hProfile CmsHPROFILE) cmsProfileClassSignature {
	icc := hProfile.(*cmsICCPROFILE)
	return icc.DeviceClass
}

// cmsSetDeviceClass sets the device class in the profile
func cmsSetDeviceClass(hProfile CmsHPROFILE, sig cmsProfileClassSignature) {
	icc := hProfile.(*cmsICCPROFILE)
	icc.DeviceClass = sig
}

// cmsGetEncodedICCversion retrieves the ICC version from the profile
func cmsGetEncodedICCversion(hProfile CmsHPROFILE) uint32 {
	icc := hProfile.(*cmsICCPROFILE)
	return icc.Version
}

// cmsSetEncodedICCversion sets the ICC version in the profile
func cmsSetEncodedICCversion(hProfile CmsHPROFILE, Version uint32) {
	icc := hProfile.(*cmsICCPROFILE)
	icc.Version = Version
}

// BaseToBase converts a number from one base to another.
func BaseToBase(input uint32, baseIn, baseOut int) uint32 {
	var buff []int
	for input > 0 {
		buff = append(buff, int(input%uint32(baseIn)))
		input /= uint32(baseIn)
	}

	output := uint32(0)
	for i := len(buff) - 1; i >= 0; i-- {
		output = output*uint32(baseOut) + uint32(buff[i])
	}

	return output
}

// cmsSetProfileVersion sets the profile version in the ICC profile.
func cmsSetProfileVersion(hProfile CmsHPROFILE, version float64) {
	icc := hProfile.(*cmsICCPROFILE)

	// Convert version (e.g., 4.2) to 0x42000000 format.
	icc.Version = BaseToBase(uint32(math.Floor(version*100.0+0.5)), 10, 16) << 16
}

// cmsGetProfileVersion retrieves the profile version from the ICC profile.
func cmsGetProfileVersion(hProfile CmsHPROFILE) float64 {
	icc := hProfile.(*cmsICCPROFILE)

	// Extract version from the 16 most significant bits.
	versionPart := icc.Version >> 16
	return float64(BaseToBase(versionPart, 16, 10)) / 100.0
}
func cmsOpenProfileFromFileTHR(ContextID CmsContext, lpFileName string, sAccess string) CmsHPROFILE {
	var NewIcc *cmsICCPROFILE
	hEmpty := cmsCreateProfilePlaceholder(ContextID)

	if hEmpty == nil {
		return nil
	}

	NewIcc = hEmpty.(*cmsICCPROFILE)

	NewIcc.IOhandler = cmsOpenIOhandlerFromFile(ContextID, lpFileName, sAccess)
	if NewIcc.IOhandler == nil {
		goto Error
	}

	// Check if access mode is write
	if sAccess == "W" || sAccess == "w" {
		NewIcc.IsWrite = true
		return hEmpty
	}

	// Read profile header
	if !cmsReadHeader(NewIcc) {
		goto Error
	}

	return hEmpty

Error:
	CmsCloseProfile(hEmpty)
	return nil
}

func CmsOpenProfileFromFile(ICCProfile string, sAccess string) CmsHPROFILE {
	return cmsOpenProfileFromFileTHR(nil, ICCProfile, sAccess)
}

func cmsOpenProfileFromMemTHR(ContextID CmsContext, MemPtr interface{}, dwSize uint32) CmsHPROFILE {
	var NewIcc *cmsICCPROFILE
	hEmpty := cmsCreateProfilePlaceholder(ContextID)

	if hEmpty == nil {
		return nil
	}

	NewIcc = hEmpty.(*cmsICCPROFILE)

	// Open the IO handler from memory
	NewIcc.IOhandler = cmsOpenIOhandlerFromMem(ContextID, MemPtr, dwSize, "r")
	if NewIcc.IOhandler == nil {
		goto Error
	}

	// Read the profile header
	if !cmsReadHeader(NewIcc) {
		goto Error
	}

	return hEmpty

Error:
	CmsCloseProfile(hEmpty)
	return nil
}

func CmsOpenProfileFromMem(MemPtr interface{}, dwSize uint32) CmsHPROFILE {
	return cmsOpenProfileFromMemTHR(nil, MemPtr, dwSize)
}

func cmsSaveProfileToIOhandler(hProfile CmsHPROFILE, io *cmsIOHANDLER) uint32 {
	Icc := hProfile.(*cmsICCPROFILE)
	var Keep cmsICCPROFILE
	var PrevIO *cmsIOHANDLER
	var UsedSpace uint32
	ContextID := Icc.ContextID
	mm := &Icc.UsrMutex
	if !cmsLockMutex(ContextID, (*cmsMutex)(mm)) {
		return 0
	}
	Keep = *Icc
	ContextID = cmsGetProfileContextID(hProfile)
	Icc.IOhandler = cmsOpenIOhandlerFromNULL(ContextID)
	PrevIO = Icc.IOhandler
	if PrevIO == nil {
		cmsUnlockMutex(ContextID, (*cmsMutex)(mm))
		return 0
	}

	if !cmsWriteHeader(Icc, 0) {
		goto Error
	}
	if !SaveTags(Icc, &Keep) {
		goto Error
	}

	UsedSpace = PrevIO.UsedSpace

	if io != nil {
		Icc.IOhandler = io
		if !SetLinks(Icc) {
			goto Error
		}
		if !cmsWriteHeader(Icc, UsedSpace) {
			goto Error
		}
		if !SaveTags(Icc, &Keep) {
			goto Error
		}
	}
	*Icc = Keep
	if !cmsCloseIOhandler(PrevIO) {
		UsedSpace = 0
	}
	cmsUnlockMutex(ContextID, (*cmsMutex)(mm))
	return UsedSpace

Error:
	cmsCloseIOhandler(PrevIO)
	*Icc = Keep
	cmsUnlockMutex(ContextID, (*cmsMutex)(mm))
	return 0
}

func cmsSaveProfileToFile(hProfile CmsHPROFILE, FileName string) bool {
	ContextID := cmsGetProfileContextID(hProfile)
	io := cmsOpenIOhandlerFromFile(ContextID, FileName, "w")
	if io == nil {
		return false
	}

	rc := cmsSaveProfileToIOhandler(hProfile, io) != 0
	rc = rc && cmsCloseIOhandler(io)

	if !rc {
		// Replace C++'s `remove` with Go's `os.Remove`
		if err := os.Remove(FileName); err != nil {
			// Optionally, handle the error (e.g., log it)
			cmsSignalError(ContextID, cmsERROR_FILE, "Failed to remove file")
		}
	}
	return rc
}

func cmsSaveProfileToStream(hProfile CmsHPROFILE, stream *os.File) bool {
	ContextID := cmsGetProfileContextID(hProfile)
	io := cmsOpenIOhandlerFromStream(ContextID, stream)
	if io == nil {
		return false
	}

	rc := cmsSaveProfileToIOhandler(hProfile, io) != 0
	rc = rc && cmsCloseIOhandler(io)
	return rc
}

func cmsSaveProfileToMem(hProfile CmsHPROFILE, MemPtr unsafe.Pointer, BytesNeeded *uint32) bool {
	ContextID := cmsGetProfileContextID(hProfile)

	if MemPtr == nil {
		*BytesNeeded = cmsSaveProfileToIOhandler(hProfile, nil)
		return *BytesNeeded != 0
	}

	io := cmsOpenIOhandlerFromMem(ContextID, MemPtr, *BytesNeeded, "w")
	if io == nil {
		return false
	}

	rc := cmsSaveProfileToIOhandler(hProfile, io) != 0
	rc = rc && cmsCloseIOhandler(io)
	return rc
}

func freeOneTag(Icc *cmsICCPROFILE, i uint32) {
	if Icc.TagPtrs[i] != nil {
		TypeHandler := Icc.TagTypeHandlers[i]
		if TypeHandler != nil {
			LocalTypeHandler := *TypeHandler
			LocalTypeHandler.ContextID = Icc.ContextID
			LocalTypeHandler.ICCVersion = Icc.Version
			LocalTypeHandler.FreeFn(&LocalTypeHandler, Icc.TagPtrs[i])
		} else {
			//cmsFree(Icc.ContextID, Icc.TagPtrs[i])
		}
	}
}

func CmsCloseProfile(hProfile CmsHPROFILE) bool {
	Icc := hProfile.(*cmsICCPROFILE)
	var rc bool = true
	mm := &Icc.UsrMutex
	if Icc == nil {
		return false
	}

	if Icc.IsWrite {
		Icc.IsWrite = false
		rc = rc && cmsSaveProfileToFile(hProfile, Icc.IOhandler.PhysicalFile)
	}

	for i := uint32(0); i < Icc.TagCount; i++ {
		freeOneTag(Icc, i)
	}

	if Icc.IOhandler != nil {
		rc = rc && cmsCloseIOhandler(Icc.IOhandler)
	}

	cmsDestroyMutex(Icc.ContextID, (*cmsMutex)(mm))
	cmsFree(Icc.ContextID, Icc)
	return rc
}

// Returns TRUE if a given tag is supported by a plug-in
func IsTypeSupported(TagDescriptor *cmsTagDescriptor, Type cmsTagTypeSignature) bool {
	var nMaxTypes uint32

	nMaxTypes = TagDescriptor.NSupportedTypes
	if nMaxTypes >= MAX_TYPES_IN_LCMS_PLUGIN {
		nMaxTypes = MAX_TYPES_IN_LCMS_PLUGIN
	}
	for i := 0; i < int(nMaxTypes); i++ {
		if Type == TagDescriptor.SupportedTypes[i] {
			return true
		}
	}

	return false
}

func cmsReadTag(hProfile CmsHPROFILE, sig cmsTagSignature) interface{} {
	Icc := hProfile.(*cmsICCPROFILE)
	var io *cmsIOHANDLER
	var TypeHandler *cmsTagTypeHandler
	var LocalTypeHandler cmsTagTypeHandler
	var TagDescriptor *cmsTagDescriptor
	var BaseType cmsTagTypeSignature
	var Offset, TagSize, ElemCount uint32
	var n int
	//fmt.Println("start cmsReadTag")
	mm := &Icc.UsrMutex
	// Lock the mutex
	if !cmsLockMutex(Icc.ContextID, (*cmsMutex)(mm)) {
		return nil
	}

	// Search for the tag
	n = cmsSearchTag(Icc, sig, true)
	if n < 0 {
		// Tag not found
		cmsUnlockMutex(Icc.ContextID, (*cmsMutex)(mm))
		return nil
	}

	// If the tag is already in memory, return it
	if Icc.TagPtrs[n] != nil {
		if Icc.TagTypeHandlers[n] == nil {
			goto Error
		}

		// Sanity check
		BaseType = Icc.TagTypeHandlers[n].Signature
		if BaseType == 0 {
			goto Error
		}

		TagDescriptor = cmsGetTagDescriptor(Icc.ContextID, sig)
		if TagDescriptor == nil {
			goto Error
		}

		if !IsTypeSupported(TagDescriptor, BaseType) {
			goto Error
		}

		if Icc.TagSaveAsRaw[n] {
			goto Error // Reading raw tags as cooked is not supported
		}

		cmsUnlockMutex(Icc.ContextID, (*cmsMutex)(mm))
		return Icc.TagPtrs[n]
	}

	// Read tag from the file
	Offset = Icc.TagOffsets[n]
	TagSize = Icc.TagSizes[n]

	if TagSize < 8 {
		goto Error
	}

	io = Icc.IOhandler
	if io == nil {
		// Built-in profile manipulated
		cmsSignalError(Icc.ContextID, cmsERROR_CORRUPTION_DETECTED, "Corrupted built-in profile.")
		goto Error
	}

	// Seek to the tag location
	if !io.Seek((*cms_io_handler)(io), Offset) {
		goto Error
	}

	// Get the tag descriptor
	TagDescriptor = cmsGetTagDescriptor(Icc.ContextID, sig)
	if TagDescriptor == nil {
		//	str := cmsTagSignature2String(sig)
		cmsSignalError(Icc.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unknown tag type found.")
		goto Error
	}

	// Read the base type of the tag
	BaseType = cmsReadTypeBase(io)
	if BaseType == 0 {
		goto Error
	}

	if !IsTypeSupported(TagDescriptor, BaseType) {
		goto Error
	}

	TagSize -= 8 // Adjust for base type size

	// Get the tag type handler
	TypeHandler = cmsGetTagTypeHandler(Icc.ContextID, BaseType)
	if TypeHandler == nil {
		goto Error
	}

	// Set up local handler
	LocalTypeHandler = *TypeHandler
	Icc.TagTypeHandlers[n] = TypeHandler
	LocalTypeHandler.ContextID = Icc.ContextID
	LocalTypeHandler.ICCVersion = Icc.Version
	// Read the tag
	Icc.TagPtrs[n] = LocalTypeHandler.ReadFn(&LocalTypeHandler, io, &ElemCount, TagSize)
	// The tag type is supported, but something wrong happened and we cannot read the tag.
	// let know the user about this (although it is just a warning)
	if Icc.TagPtrs[n] == nil {
		//	str := cmsTagSignature2String(sig)
		cmsSignalError(Icc.ContextID, cmsERROR_CORRUPTION_DETECTED, "Corrupted tag")
		goto Error
	}

	// Check element count consistency
	// This is a weird error that may be a symptom of something more serious, the number of
	// stored item is actually less than the number of required elements.
	if ElemCount < TagDescriptor.ElemCount {
		//	str := cmsTagSignature2String(sig)
		cmsSignalError(Icc.ContextID, cmsERROR_CORRUPTION_DETECTED,
			"Inconsistent number of items")
		goto Error
	}

	// Unlock and return
	cmsUnlockMutex(Icc.ContextID, (*cmsMutex)(mm))
	//	fmt.Println("end cmsReadTag")
	return Icc.TagPtrs[n]

Error:
	freeOneTag(Icc, uint32(n))
	Icc.TagPtrs[n] = nil
	cmsUnlockMutex(Icc.ContextID, (*cmsMutex)(mm))
	return nil
}

// Creates an empty structure holding all required parameters
func cmsCreateProfilePlaceholder(ContextID CmsContext) CmsHPROFILE {
	Icc := allocateStruct[cmsICCPROFILE]()
	var cm *cmsMutex
	if Icc == nil {
		return nil
	}

	Icc.ContextID = ContextID

	// Set it to empty
	Icc.TagCount = 0

	// Set default version
	Icc.Version = 0x02100000

	// Set default device class
	Icc.DeviceClass = cmsSigDisplayClass

	// Set creation date/time
	if !cmsGetTime(&Icc.Created) {
		goto Error
	}

	// Create a mutex if the user provided a proper plugin, NULL otherwise
	cm = cmsCreateMutex(ContextID)
	Icc.UsrMutex = (*sync.Mutex)(*cm)

	// Return the handle
	return Icc

Error:
	cmsFree(ContextID, Icc)
	return nil
}

// cmsGetTagTrueType translates to Go
func cmsGetTagTrueType(hProfile CmsHPROFILE, sig cmsTagSignature) cmsTagTypeSignature {
	Icc := hProfile.(*cmsICCPROFILE) // Cast hProfile to *cmsICCPROFILE

	// Search for the given tag in ICC profile directory
	n := cmsSearchTag(Icc, sig, true)
	if n < 0 {
		return cmsTagTypeSignature(0) // Not found, return 0
	}

	// Get the handler. The true type is there
	TypeHandler := Icc.TagTypeHandlers[n]
	return TypeHandler.Signature
}

// cmsWriteTag translates the given function
func cmsWriteTag(hProfile CmsHPROFILE, sig cmsTagSignature, data interface{}) bool {
	//	fmt.Println("WriteTag")
	Icc := hProfile.(*cmsICCPROFILE)
	var TypeHandler *cmsTagTypeHandler
	var LocalTypeHandler cmsTagTypeHandler
	var TagDescriptor *cmsTagDescriptor
	var Type cmsTagTypeSignature
	var i int
	var Version float64
	var TypeString int
	mm := &Icc.UsrMutex
	if !cmsLockMutex(Icc.ContextID, (*cmsMutex)(mm)) {
		return false
	}

	// Handle deletion of the tag
	if data == nil {
		i = cmsSearchTag(Icc, sig, false)
		if i >= 0 {
			// Mark the tag as deleted
			cmsDeleteTagByPos(Icc, i)
			Icc.TagNames[i] = 0
			cmsUnlockMutex(Icc.ContextID, (*cmsMutex)(mm))
			return true
		}
		goto Error
	}

	// Add a new tag or get the position of an existing one
	if !cmsNewTag(Icc, sig, &i) {
		goto Error
	}

	// Initialize the new tag
	Icc.TagSaveAsRaw[i] = false
	Icc.TagLinked[i] = 0

	// Retrieve information about the tag
	TagDescriptor = cmsGetTagDescriptor(Icc.ContextID, sig)
	if TagDescriptor == nil {
		cmsSignalError(Icc.ContextID, cmsERROR_UNKNOWN_EXTENSION, fmt.Sprintf("Unsupported tag '%x'", sig))
		goto Error
	}

	// Determine the type based on the version and data
	Version = cmsGetProfileVersion(hProfile)
	if TagDescriptor.DecideType != nil {
		Type = TagDescriptor.DecideType(Version, data)
	} else {
		Type = TagDescriptor.SupportedTypes[0]
	}

	// Check if the type is supported
	if !IsTypeSupported(TagDescriptor, Type) {
		str := cmsTagSignature2String(sig)
		cmsSignalError(Icc.ContextID, cmsERROR_UNKNOWN_EXTENSION, fmt.Sprintf("Unsupported type '%s' for tag '%s'", TypeString, str))
		goto Error
	}

	// Get the handler for the type
	TypeHandler = cmsGetTagTypeHandler(Icc.ContextID, Type)
	if TypeHandler == nil {
		str := cmsTagSignature2String(sig)
		cmsSignalError(Icc.ContextID, cmsERROR_UNKNOWN_EXTENSION, fmt.Sprintf("Unsupported type '%s' for tag '%s'", TypeString, str))
		goto Error
	}

	// Set up the tag fields in the profile structure
	Icc.TagTypeHandlers[i] = TypeHandler
	Icc.TagNames[i] = sig
	Icc.TagSizes[i] = 0
	Icc.TagOffsets[i] = 0

	// Duplicate the pointer for the tag data
	LocalTypeHandler = *TypeHandler
	LocalTypeHandler.ContextID = Icc.ContextID
	LocalTypeHandler.ICCVersion = Icc.Version
	Icc.TagPtrs[i] = LocalTypeHandler.DupFn(&LocalTypeHandler, data, TagDescriptor.ElemCount)

	if Icc.TagPtrs[i] == nil {
		str := cmsTagSignature2String(sig)
		cmsSignalError(Icc.ContextID, cmsERROR_CORRUPTION_DETECTED, fmt.Sprintf("Malformed struct in type '%s' for tag '%s'", str))
		goto Error
	}

	cmsUnlockMutex(Icc.ContextID, (*cmsMutex)(mm))
	return true

Error:
	cmsUnlockMutex(Icc.ContextID, (*cmsMutex)(mm))
	return false
}

// Retrieve the context ID from a profile
func cmsGetProfileContextID(hProfile CmsHPROFILE) CmsContext {
	Icc, ok := hProfile.(*cmsICCPROFILE)

	if !ok || Icc == nil {
		return nil
	}

	return Icc.ContextID
}

// Return the number of tags
func cmsGetTagCount(hProfile CmsHPROFILE) int32 {
	Icc := hProfile.(*cmsICCPROFILE)
	if Icc == nil {
		return -1
	}
	return int32(Icc.TagCount)
}

// Return the tag signature of a given tag number
func cmsGetTagSignature(hProfile CmsHPROFILE, n uint32) cmsTagSignature {
	Icc := hProfile.(*cmsICCPROFILE)

	if n >= uint32(Icc.TagCount) || n >= MAX_TABLE_TAG {
		return 0 // Mark as not available
	}

	return Icc.TagNames[n]
}

// Search for a specific tag in the tag dictionary
func SearchOneTag(Profile *cmsICCPROFILE, sig cmsTagSignature) int {
	for i := 0; i < int(Profile.TagCount); i++ {
		if sig == Profile.TagNames[i] {
			return i
		}
	}
	return -1
}

// Search for a specific tag in the tag dictionary.
// If `followLinks` is true, the position of the linked tag is returned.
func cmsSearchTag(Icc *cmsICCPROFILE, sig cmsTagSignature, followLinks bool) int {
	var n int
	var LinkedSig cmsTagSignature

	for {
		// Search for the given tag in ICC profile directory
		n = SearchOneTag(Icc, sig)
		if n < 0 {
			return -1 // Not found
		}

		if !followLinks {
			return n // Found, don't follow links
		}

		// Is this a linked tag?
		LinkedSig = Icc.TagLinked[n]

		if LinkedSig == 0 {
			break
		}

		// Yes, follow the link
		sig = LinkedSig
	}

	return n
}

// Deletes a tag entry
func cmsDeleteTagByPos(Icc *cmsICCPROFILE, i int) {
	cmsAssert(Icc != nil, "")
	cmsAssert(i >= 0, "")

	if Icc.TagPtrs[i] != nil {
		// Free previous version
		if Icc.TagSaveAsRaw[i] {
			//cmsFree(Icc.ContextID, Icc.TagPtrs[i])
		} else {
			TypeHandler := Icc.TagTypeHandlers[i]
			if TypeHandler != nil {
				LocalTypeHandler := *TypeHandler
				LocalTypeHandler.ContextID = Icc.ContextID
				LocalTypeHandler.ICCVersion = Icc.Version
				LocalTypeHandler.FreeFn(&LocalTypeHandler, Icc.TagPtrs[i])
				Icc.TagPtrs[i] = nil
			}
		}
	}
}

// Creates a new tag entry
func cmsNewTag(Icc *cmsICCPROFILE, sig cmsTagSignature, NewPos *int) bool {
	// Search for the tag
	i := cmsSearchTag(Icc, sig, false)
	if i >= 0 {
		// Already exists? delete it
		cmsDeleteTagByPos(Icc, i)
		*NewPos = i
	} else {
		// No, make a new one
		if Icc.TagCount >= MAX_TABLE_TAG {
			cmsSignalError(Icc.ContextID, cmsERROR_RANGE, fmt.Sprintf("Too many tags (%d)", MAX_TABLE_TAG))
			return false
		}

		*NewPos = int(Icc.TagCount)
		Icc.TagCount++
	}

	return true
}

// Check existence
func cmsIsTag(hProfile CmsHPROFILE, sig cmsTagSignature) bool {
	Icc := hProfile.(*cmsICCPROFILE)
	return cmsSearchTag(Icc, sig, false) >= 0
}

// Checks for link compatibility
func CompatibleTypes(desc1 *cmsTagDescriptor, desc2 *cmsTagDescriptor) bool {

	if desc1 == nil || desc2 == nil {
		return false
	}

	if desc1.NSupportedTypes != desc2.NSupportedTypes {
		return false
	}
	if desc1.ElemCount != desc2.ElemCount {
		return false
	}

	for i := uint32(0); i < desc1.NSupportedTypes; i++ {
		if desc1.SupportedTypes[i] != desc2.SupportedTypes[i] {
			return false
		}
	}

	return true
}

// _validatedVersion enforces that the profile version adheres to the specification.
func validatedVersion(dWord uint32) uint32 {
	pBytes := []byte{
		byte(dWord >> 24),
		byte((dWord >> 16) & 0xFF),
		byte((dWord >> 8) & 0xFF),
		byte(dWord & 0xFF),
	}

	if pBytes[0] > 0x09 {
		pBytes[0] = 0x09
	}

	temp1 := pBytes[1] & 0xF0
	temp2 := pBytes[1] & 0x0F

	if temp1 > 0x90 {
		temp1 = 0x90
	}
	if temp2 > 0x09 {
		temp2 = 0x09
	}

	pBytes[1] = temp1 | temp2
	pBytes[2] = 0x00
	pBytes[3] = 0x00

	return uint32(pBytes[0])<<24 | uint32(pBytes[1])<<16 | uint32(pBytes[2])<<8 | uint32(pBytes[3])
}

// validDeviceClass checks if the given device class is valid.
func validDeviceClass(cl cmsProfileClassSignature) bool {
	if cl == 0 {
		return true // Allow zero for older compatibility.
	}

	switch cl {
	case cmsSigInputClass:
		return true
	case cmsSigDisplayClass:
		return true
	case cmsSigOutputClass:
		return true
	case cmsSigLinkClass:
		return true
	case cmsSigAbstractClass:
		return true
	case cmsSigColorSpaceClass:
		return true
	case cmsSigNamedColorClass:
		return true
	default:
		return false
	}
}
func ReadStruct[T any](io *cmsIOHANDLER, endian binary.ByteOrder, count uint32) (T, error) {
	var value T

	size := binary.Size(value)
	if size <= 0 {
		return value, fmt.Errorf("invalid struct size: %d", size)
	}

	buf := make([]byte, size)
	// Read the header
	if io.Read((*cms_io_handler)(io), buf, uint32(size), count) != count {
		return value, fmt.Errorf("FileRead failed")
	}

	err := binary.Read(bytes.NewReader(buf), endian, &value)
	if err != nil {
		return value, fmt.Errorf("binary.Read failed: %w", err)
	}

	return value, nil
}

// cmsReadHeader reads and validates the profile header.
func cmsReadHeader(Icc *cmsICCPROFILE) bool {
	var Tag cmsTagEntry
	var Header cmsICCHeader
	var TagCount uint32
	io := Icc.IOhandler

	Header, err := ReadStruct[cmsICCHeader](io, binary.BigEndian, 1)
	if err != nil {
		fmt.Errorf("Failed to read ICC header: %v", err)
	}

	// Validate file as an ICC profile
	if cmsAdjustEndianess32(uint32(Header.Magic)) != cmsMagicNumber {
		cmsSignalError(Icc.ContextID, cmsERROR_BAD_SIGNATURE, "not an ICC profile, invalid signature")
		return false
	}

	// Adjust endianness of the used parameters
	Icc.DeviceClass = cmsProfileClassSignature(cmsAdjustEndianess32(uint32(Header.DeviceClass)))
	Icc.ColorSpace = cmsColorSpaceSignature(cmsAdjustEndianess32(uint32(Header.ColorSpace)))
	Icc.PCS = cmsColorSpaceSignature(cmsAdjustEndianess32(uint32(Header.PCS)))
	Icc.RenderingIntent = cmsAdjustEndianess32(Header.RenderingIntent)
	Icc.Flags = cmsAdjustEndianess32(Header.Flags)
	Icc.Manufacturer = cmsAdjustEndianess32(uint32(Header.Manufacturer))
	Icc.Model = cmsAdjustEndianess32(Header.Model)
	Icc.Creator = cmsAdjustEndianess32(uint32(Header.Creator))
	Icc.Attributes = cmsAdjustEndianess64(Header.Attributes)
	Icc.Version = cmsAdjustEndianess32(validatedVersion(Header.Version))

	if Icc.Version > 0x5000000 {
		cmsSignalError(Icc.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unsupported profile version")
		return false
	}

	if !validDeviceClass(Icc.DeviceClass) {
		cmsSignalError(Icc.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unsupported device class")
		return false
	}

	// Get size as reported in header
	HeaderSize := cmsAdjustEndianess32(Header.Size)
	if HeaderSize >= Icc.IOhandler.ReportedSize {
		HeaderSize = Icc.IOhandler.ReportedSize
	}

	// Get creation date/time
	Icc.Created = cmsDecodeDateTimeNumber(&Header.Date)

	// The profile ID are 32 raw bytes
	copy(Icc.ProfileID[:], Header.ProfileID[:])

	// Read tag directory
	if !cmsReadUInt32Number(io, &TagCount) {
		return false
	}
	if TagCount > MAX_TABLE_TAG {
		cmsSignalError(Icc.ContextID, cmsERROR_RANGE, "Too many tags")
		return false
	}

	// Initialize tag directory
	Icc.TagCount = 0
	for i := uint32(0); i < TagCount; i++ {
		if !cmsReadUInt32Number(io, &Tag.Sig) ||
			!cmsReadUInt32Number(io, &Tag.Offset) ||
			!cmsReadUInt32Number(io, &Tag.Size) {
			return false
		}

		// Perform sanity checks
		if Tag.Size == 0 || Tag.Offset == 0 || Tag.Offset+Tag.Size > HeaderSize || Tag.Offset+Tag.Size < Tag.Offset {
			continue
		}

		Icc.TagNames[Icc.TagCount] = Tag.Sig
		Icc.TagOffsets[Icc.TagCount] = Tag.Offset
		Icc.TagSizes[Icc.TagCount] = Tag.Size

		// Search for links
		for j := uint32(0); j < Icc.TagCount; j++ {
			if Icc.TagOffsets[j] == Tag.Offset && Icc.TagSizes[j] == Tag.Size {
				if CompatibleTypes(
					cmsGetTagDescriptor(Icc.ContextID, Icc.TagNames[j]),
					cmsGetTagDescriptor(Icc.ContextID, Tag.Sig),
				) {
					Icc.TagLinked[Icc.TagCount] = Icc.TagNames[j]
				}
			}
		}

		Icc.TagCount++
	}

	// Check for duplicate tags
	for i := uint32(0); i < Icc.TagCount; i++ {
		for j := uint32(0); j < Icc.TagCount; j++ {
			if i != j && Icc.TagNames[i] == Icc.TagNames[j] {
				cmsSignalError(Icc.ContextID, cmsERROR_RANGE, "Duplicate tag found")
				return false
			}
		}
	}

	return true
}

func WriteStruct[T any](io *cmsIOHANDLER, value T, endian binary.ByteOrder) bool {
	var buf bytes.Buffer
	if err := binary.Write(&buf, endian, value); err != nil {
		fmt.Println("binary.Write failed: %w", err)
		return false
	}

	size := buf.Len()
	if !io.Write((*cms_io_handler)(io), uint32(size), buf.Bytes()) {
		fmt.Println("FileWrite failed: wrote %d element(s), expected 1")
		return false
	}

	return true
}

// cmsWriteHeader saves the profile header.
func cmsWriteHeader(Icc *cmsICCPROFILE, UsedSpace uint32) bool {
	var Header cmsICCHeader
	var Tag cmsTagEntry
	var Count uint32

	Header.Size = cmsAdjustEndianess32(UsedSpace)
	Header.CmmId = cmsSignature(cmsAdjustEndianess32(lcmsSignature))
	Header.Version = cmsAdjustEndianess32(Icc.Version)
	Header.DeviceClass = cmsProfileClassSignature(cmsAdjustEndianess32(uint32(Icc.DeviceClass)))
	Header.ColorSpace = cmsColorSpaceSignature(cmsAdjustEndianess32(uint32(Icc.ColorSpace)))
	Header.PCS = cmsColorSpaceSignature(cmsAdjustEndianess32(uint32(Icc.PCS)))
	cmsEncodeDateTimeNumber(&Header.Date, Icc.Created)
	Header.Magic = cmsSignature(cmsAdjustEndianess32(cmsMagicNumber))

	Header.Platform = cmsPlatformSignature(cmsAdjustEndianess32(uint32(cmsSigMicrosoft)))
	Header.Flags = cmsAdjustEndianess32(Icc.Flags)
	Header.Manufacturer = cmsSignature(cmsAdjustEndianess32(Icc.Manufacturer))
	Header.Model = cmsAdjustEndianess32(Icc.Model)
	Icc.Attributes = cmsAdjustEndianess64(Header.Attributes)
	Header.RenderingIntent = cmsAdjustEndianess32(Icc.RenderingIntent)
	Header.Illuminant.X = cmsS15Fixed16Number(cmsAdjustEndianess32(uint32(cmsDoubleTo15Fixed16(cmsD50_XYZ().X))))
	Header.Illuminant.Y = cmsS15Fixed16Number(cmsAdjustEndianess32(uint32(cmsDoubleTo15Fixed16(cmsD50_XYZ().Y))))
	Header.Illuminant.Z = cmsS15Fixed16Number(cmsAdjustEndianess32(uint32(cmsDoubleTo15Fixed16(cmsD50_XYZ().Z))))
	Header.Creator = cmsSignature(cmsAdjustEndianess32(lcmsSignature))

	// Set profile ID. Endianness is always big endian
	copy(Header.ProfileID[:], Icc.ProfileID[:])

	// Write header
	if !WriteStruct[cmsICCHeader](Icc.IOhandler, Header, binary.BigEndian) {
		return false
	}

	// Save tag directory
	Count = 0
	for i := uint32(0); i < Icc.TagCount; i++ {
		if Icc.TagNames[i] != 0 {
			Count++
		}
	}

	if !cmsWriteUInt32Number(Icc.IOhandler, Count) {
		return false
	}

	for i := uint32(0); i < Icc.TagCount; i++ {
		if Icc.TagNames[i] == 0 {
			continue
		}

		Tag.Sig = cmsTagSignature(cmsAdjustEndianess32(uint32(Icc.TagNames[i])))
		Tag.Offset = cmsAdjustEndianess32(Icc.TagOffsets[i])
		Tag.Size = cmsAdjustEndianess32(Icc.TagSizes[i])

		if !WriteStruct[cmsTagEntry](Icc.IOhandler, Tag, binary.BigEndian) {
			return false
		}
	}

	return true
}

// SaveTags dumps tag contents. If the profile is being modified, untouched tags are copied from FileOrig.
func SaveTags(Icc *cmsICCPROFILE, FileOrig *cmsICCPROFILE) bool {
	io := Icc.IOhandler
	Version := cmsGetProfileVersion(CmsHPROFILE(Icc))

	for i := uint32(0); i < Icc.TagCount; i++ {
		if Icc.TagNames[i] == 0 {
			continue
		}

		// Linked tags are not written
		if Icc.TagLinked[i] != 0 {
			continue
		}

		Icc.TagOffsets[i] = io.UsedSpace
		begin := io.UsedSpace

		data := Icc.TagPtrs[i].([]uint8)
		if data == nil {
			// Handle blind copy of unmodified disk-based ICC profile tags
			if FileOrig != nil && Icc.TagOffsets[i] != 0 {
				if FileOrig.IOhandler != nil {
					tagSize := FileOrig.TagSizes[i]
					tagOffset := FileOrig.TagOffsets[i]
					mem := cmsMalloc(Icc.ContextID, tagSize)
					if mem == nil {
						return false
					}

					if !FileOrig.IOhandler.Seek((*cms_io_handler)(FileOrig.IOhandler), tagOffset) ||
						FileOrig.IOhandler.Read((*cms_io_handler)(FileOrig.IOhandler), mem, tagSize, 1) != 1 ||
						!io.Write((*cms_io_handler)(io), tagSize, mem) {
						cmsFree(Icc.ContextID, mem)
						return false
					}

					cmsFree(Icc.ContextID, mem)
					Icc.TagSizes[i] = io.UsedSpace - begin

					// Align to 32-bit boundary
					if !cmsWriteAlignment(io) {
						return false
					}
				}
			}
			continue
		}

		// Save tag as RAW if specified
		if Icc.TagSaveAsRaw[i] {
			if !io.Write((*cms_io_handler)(io), Icc.TagSizes[i], data) {
				return false
			}
		} else {
			// Search for tag support
			tagDescriptor := cmsGetTagDescriptor(Icc.ContextID, Icc.TagNames[i])
			if tagDescriptor == nil {
				continue
			}

			var tagType cmsTagTypeSignature
			if tagDescriptor.DecideType != nil {
				tagType = tagDescriptor.DecideType(Version, data)
			} else {
				tagType = tagDescriptor.SupportedTypes[0]
			}

			typeHandler := cmsGetTagTypeHandler(Icc.ContextID, tagType)
			if typeHandler == nil {
				cmsSignalError(Icc.ContextID, cmsERROR_INTERNAL, "(Internal) no handler for tag")
				continue
			}

			typeBase := typeHandler.Signature
			if !cmsWriteTypeBase(io, typeBase) {
				return false
			}

			localTypeHandler := *typeHandler
			localTypeHandler.ContextID = Icc.ContextID
			localTypeHandler.ICCVersion = Icc.Version
			if !localTypeHandler.WriteFn(&localTypeHandler, io, data, tagDescriptor.ElemCount) {
				cmsSignalError(Icc.ContextID, cmsERROR_WRITE, "Couldn't write type")
				return false
			}
		}

		Icc.TagSizes[i] = io.UsedSpace - begin

		// Align to 32-bit boundary
		if !cmsWriteAlignment(io) {
			return false
		}
	}

	return true
}

// SetLinks fills the offset and size fields for all linked tags.
func SetLinks(Icc *cmsICCPROFILE) bool {
	for i := uint32(0); i < Icc.TagCount; i++ {
		lnk := Icc.TagLinked[i]
		if lnk != 0 {
			j := cmsSearchTag(Icc, lnk, false)
			if j >= 0 {
				Icc.TagOffsets[i] = Icc.TagOffsets[j]
				Icc.TagSizes[i] = Icc.TagSizes[j]
			}
		}
	}
	return true
}

// FILEMEM represents the memory-based stream structure.
type FILEMEM struct {
	Block            []byte // Points to allocated memory
	Size             uint32 // Size of allocated memory
	Pointer          uint32 // Points to current location
	FreeBlockOnClose bool   // Indicates if the block should be freed on close
}

// func MemoryRead(iohandler *cms_io_handler, buffer []byte, size, count uint32) uint32 {
func MemoryRead(iohandler *cms_io_handler, buffer interface{}, size, count uint32) uint32 {
	resData, ok := iohandler.Stream.(*FILEMEM)
	if !ok || resData == nil {
		// If Stream is not a *FILENULL, do nothing
		return 0
	}
	length := size * count

	if resData.Pointer+length > resData.Size {
		length = resData.Size - resData.Pointer
		cmsSignalError(nil, cmsERROR_READ,
			fmt.Sprintf("Read from memory error. Got %d bytes, block should be of %d bytes", length, count*size))
		return 0
	}

	// Reconstruct slice from pointer
	src := resData.Block[resData.Pointer : resData.Pointer+length]

	// Assert interface and copy into destination
	switch dst := buffer.(type) {
	case []byte:
		copy(dst, src)
	case *[]byte:
		copy(*dst, src)
	default:
		cmsSignalError(nil, cmsERROR_READ, "Unsupported buffer type for MemoryRead")
		return 0
	}

	resData.Pointer += length
	return count
}

// MemorySeek sets the current position in the memory block.
func MemorySeek(iohandler *cms_io_handler, offset uint32) bool {
	resData, ok := iohandler.Stream.(*FILEMEM)
	if !ok || resData == nil {
		// If Stream is not a *FILENULL, do nothing
		return false
	}

	if offset > resData.Size {
		cmsSignalError(iohandler.ContextID, cmsERROR_SEEK, "Too few data; probably corrupted profile")
		return false
	}

	resData.Pointer = offset
	return true
}

// MemoryTell returns the current position in the memory block.
func MemoryTell(iohandler *cms_io_handler) uint32 {
	resData, ok := iohandler.Stream.(*FILEMEM)
	if !ok || resData == nil {
		// If Stream is not a *FILENULL, do nothing
		return 0
	}
	return resData.Pointer
}

// MemoryWrite writes data to the memory block and updates the used space.
func MemoryWrite(iohandler *cms_io_handler, size uint32, ptr []byte) bool {
	resData, ok := iohandler.Stream.(*FILEMEM)
	if !ok || resData == nil {
		// If Stream is not a *FILENULL, do nothing
		return false
	}
	if resData == nil {
		return false
	}

	// Check for available space
	if resData.Pointer+size > resData.Size {
		size = resData.Size - resData.Pointer
	}

	if size == 0 {
		return true // Writing zero bytes is okay but does nothing
	}

	// Convert *byte block to []byte
	//	dest := unsafe.Slice(resData.Block, resData.Size)
	destSlice := resData.Block[resData.Pointer : resData.Pointer+size]
	copy(destSlice, ptr)
	resData.Pointer += size

	if resData.Pointer > iohandler.UsedSpace {
		iohandler.UsedSpace = resData.Pointer
	}

	return true
}

// MemoryClose closes the memory-based stream and frees resources if necessary.
func MemoryClose(iohandler *cms_io_handler) bool {
	resData, ok := iohandler.Stream.(*FILEMEM)
	if !ok || resData == nil {
		// If Stream is not a *FILENULL, do nothing
		return false
	}

	if resData.FreeBlockOnClose {
		if resData.Block != nil {
			cmsFree(iohandler.ContextID, resData.Block)
		}
	}

	cmsFree(iohandler.ContextID, resData)
	cmsFree(iohandler.ContextID, iohandler)

	return true
}
func cmsOpenIOhandlerFromMem(ContextID CmsContext, Buffer interface{}, size uint32, AccessMode string) *cmsIOHANDLER {
	if AccessMode == "" {
		cmsSignalError(nil, cmsERROR_READ, "Access mode cannot be empty")
		return nil
	}

	iohandler := allocateStruct[cmsIOHANDLER]()
	if iohandler == nil {
		return nil
	}

	var fm *FILEMEM

	switch AccessMode[0] {
	case 'r': // Read mode
		fm = allocateStruct[FILEMEM]()
		if fm == nil {
			goto Error
		}

		if Buffer == nil {
			cmsSignalError(nil, cmsERROR_READ, "Couldn't read profile from nil buffer")
			goto Error
		}

		// Allocate internal block
		fm.Block = cmsMalloc(ContextID, size)
		if fm.Block == nil {
			goto Error
		}

		// Convert Buffer to []byte and copy into internal block
		src, ok := Buffer.([]byte)
		if !ok {
			cmsSignalError(nil, cmsERROR_READ, "Expected []byte as buffer for reading")
			goto Error
		}
		if uint32(len(src)) < size {
			cmsSignalError(nil, cmsERROR_READ, "Provided buffer smaller than requested size")
			goto Error
		}
		copy(fm.Block, src)

		fm.FreeBlockOnClose = true
		fm.Size = size
		fm.Pointer = 0
		iohandler.ReportedSize = size

	case 'w': // Write mode
		fm = allocateStruct[FILEMEM]()
		if fm == nil {
			goto Error
		}

		if Buffer == nil {
			cmsSignalError(nil, cmsERROR_READ, "Buffer cannot be nil for write mode")
			goto Error
		}

		dst, ok := Buffer.([]byte)
		if !ok {
			cmsSignalError(nil, cmsERROR_READ, "Expected []byte as buffer for writing")
			goto Error
		}
		if uint32(len(dst)) < size {
			cmsSignalError(nil, cmsERROR_READ, "Provided write buffer smaller than requested size")
			goto Error
		}

		fm.Block = dst // pointer to start of external buffer
		fm.FreeBlockOnClose = false
		fm.Size = size
		fm.Pointer = 0
		iohandler.ReportedSize = 0

	default:
		cmsSignalError(nil, cmsERROR_UNKNOWN_EXTENSION, "Unknown access mode")
		goto Error
	}

	// Setup IO handler
	iohandler.ContextID = ContextID
	iohandler.Stream = fm
	iohandler.UsedSpace = 0
	iohandler.PhysicalFile = ""

	// Assign handler functions
	iohandler.Read = MemoryRead
	iohandler.Seek = MemorySeek
	iohandler.Close = MemoryClose
	iohandler.Tell = MemoryTell
	iohandler.Write = MemoryWrite

	return iohandler

Error:
	if fm != nil {
		if fm.Block != nil && fm.FreeBlockOnClose {
			cmsFree(ContextID, fm.Block)
		}
		cmsFree(ContextID, fm)
	}
	if iohandler != nil {
		cmsFree(ContextID, iohandler)
	}
	return nil
}

func cmsOpenIOhandlerFromFile(ContextID CmsContext, FileName string, AccessMode string) *cmsIOHANDLER {
	if FileName == "" || AccessMode == "" {
		cmsSignalError(ContextID, cmsERROR_FILE, "Invalid file name or access mode")
		return nil
	}

	iohandler := allocateStruct[cmsIOHANDLER]()
	if iohandler == nil {
		return nil
	}

	var file *os.File
	var err error

	mode := ""
	for _, ch := range AccessMode {
		switch ch {
		case 'r', 'w':
			if mode != "" {
				cmsFree(ContextID, iohandler)
				cmsSignalError(ContextID, cmsERROR_FILE, "Access mode already specified")
				return nil
			}
			mode = string(ch)
		case 'e': // Ignored in Go, no direct equivalent for "close-on-exec"
			continue
		default:
			cmsFree(ContextID, iohandler)
			cmsSignalError(ContextID, cmsERROR_FILE, "Wrong access mode")
			return nil
		}
	}

	switch mode {
	case "r":
		file, err = os.Open(FileName)
		if err != nil {
			cmsFree(ContextID, iohandler)
			cmsSignalError(ContextID, cmsERROR_FILE, "File  not found")
			return nil
		}
		info, err := file.Stat()
		if err != nil {
			file.Close()
			cmsFree(ContextID, iohandler)
			cmsSignalError(ContextID, cmsERROR_FILE, "Cannot get size of file ")
			return nil
		}
		iohandler.ReportedSize = uint32(info.Size())

	case "w":
		file, err = os.Create(FileName)
		if err != nil {
			cmsFree(ContextID, iohandler)
			cmsSignalError(ContextID, cmsERROR_FILE, "Couldn't create ")
			return nil
		}
		iohandler.ReportedSize = 0

	default:
		cmsFree(ContextID, iohandler)
		return nil
	}

	iohandler.ContextID = ContextID
	iohandler.Stream = file
	iohandler.UsedSpace = 0
	iohandler.PhysicalFile = FileName

	iohandler.Read = FileRead
	iohandler.Seek = FileSeek
	iohandler.Close = FileClose
	iohandler.Tell = FileTell
	iohandler.Write = FileWrite

	return iohandler
}

// FileRead reads count elements of size bytes each from the file stream. Returns the number of elements read.
/*func FileRead(iohandler *cms_io_handler, buffer []byte, size, count uint32) uint32 {
	file := (*os.File)(iohandler.Stream)
	totalBytes := int(size * count)
	readBuffer := make([]byte, totalBytes)

	nRead, err := file.Read(readBuffer)
	if err != nil {
		cmsSignalError(unsafe.Pointer(iohandler.ContextID), cmsERROR_FILE, "Read error. Got  bytes, block should be of  bytes")
		return 0
	}

	// Copy the read data into the provided buffer
	MemmoveSlice(buffer, readBuffer, nRead)

	if nRead < totalBytes {
		cmsSignalError(unsafe.Pointer(iohandler.ContextID), cmsERROR_FILE, "Read error. Got  bytes, block should be of  bytes")
		return 0
	}

	return uint32(nRead / int(size))
}*/

// FileRead reads count elements of size bytes each from the file stream. Returns the number of elements read.
func FileRead(iohandler *cms_io_handler, buffer interface{}, size, count uint32) uint32 {
	fmt.Println("fileread")
	file, ok := iohandler.Stream.(*os.File)
	if !ok || file == nil {
		// If Stream is not a *FILENULL, do nothing
		return 0
	}
	totalBytes := int(size * count)

	readBuffer := make([]byte, totalBytes)
	nRead, err := file.Read(readBuffer)
	if err != nil {
		//		cmsSignalError(nil, cmsERROR_FILE, "Read error: %v", err)
		cmsSignalError(nil, cmsERROR_FILE, "Read error: %v")
		return 0
	}

	// Type-assert the output buffer
	switch dst := buffer.(type) {
	case []byte:
		copy(dst, readBuffer[:nRead])
	case *[]byte:
		copy(*dst, readBuffer[:nRead])
	default:
		cmsSignalError(nil, cmsERROR_FILE, "Unsupported buffer type in FileRead")
		return 0
	}

	if nRead < totalBytes {
		//	cmsSignalError(nil, cmsERROR_FILE, "Read error: got %d bytes, expected %d", nRead, totalBytes)
		cmsSignalError(nil, cmsERROR_FILE, "Read error: got %d bytes, expected %d")
		return 0
	}

	return uint32(nRead / int(size))
}

// FileSeek repositions the file pointer within the file. Returns true on success, false otherwise.
func FileSeek(iohandler *cms_io_handler, offset uint32) bool {
	file, ok := iohandler.Stream.(*os.File)
	if !ok || file == nil {
		// If Stream is not a *FILENULL, do nothing
		cmsSignalError(nil, cmsERROR_FILE, "Unsupported stream type in FileSeek")
		return false
	}

	_, err := file.Seek(int64(offset), 0) // Equivalent to SEEK_SET
	if err != nil {
		cmsSignalError(iohandler.ContextID, cmsERROR_FILE, "Seek error; probably corrupted file")
		return false
	}

	return true
}

// FileTell returns the current position of the file pointer within the file. Returns 0 on error, which is also a valid position.
func FileTell(iohandler *cms_io_handler) uint32 {
	file, ok := iohandler.Stream.(*os.File)
	if !ok || file == nil {
		// If Stream is not a *FILENULL, do nothing
		cmsSignalError(nil, cmsERROR_FILE, "Unsupported stream type in FileTell")
		return 0
	}

	pos, err := file.Seek(0, 1) // Equivalent to SEEK_CUR
	if err != nil {
		cmsSignalError(iohandler.ContextID, cmsERROR_FILE, "Tell error; probably corrupted file")
		return 0
	}

	return uint32(pos)
}

// FileWrite writes data to the stream. Returns true on success, false otherwise.
/*func FileWrite(iohandler *cms_io_handler, size uint32, buffer []byte) bool {
	if size == 0 {
		return true // We allow writing 0 bytes, but nothing is written
	}

	file := (*os.File)(iohandler.Stream)
	nWritten, err := file.Write(buffer)
	if err != nil || uint32(nWritten) != size {
		cmsSignalError(unsafe.Pointer(iohandler.ContextID), cmsERROR_FILE, "Write error; expected to write  bytes")
		return false
	}

	iohandler.UsedSpace += size
	return true
}*/

// FileWrite writes data to the stream. Returns true on success, false otherwise.
func FileWrite(iohandler *cms_io_handler, size uint32, buffer []byte) bool {
	if size == 0 {
		return true // We allow writing 0 bytes, but nothing is written
	}

	file, ok := iohandler.Stream.(*os.File)
	if !ok || file == nil {
		// If Stream is not a *FILENULL, do nothing
		cmsSignalError(nil, cmsERROR_FILE, "Unsupported stream type in FileRead")
		return false
	}
	nWritten, err := file.Write(buffer)
	if err != nil || uint32(nWritten) != size {
		cmsSignalError(iohandler.ContextID, cmsERROR_FILE, "Write error; expected to write  bytes")
		return false
	}

	iohandler.UsedSpace += size
	return true
}

// FileClose closes the file stream. Returns true on success, false otherwise.
func FileClose(iohandler *cms_io_handler) bool {
	file, ok := iohandler.Stream.(*os.File)
	if !ok || file == nil {
		// If Stream is not a *FILENULL, do nothing
		return false
	}

	if err := file.Close(); err != nil {
		cmsSignalError(iohandler.ContextID, cmsERROR_FILE, "Close error; unable to close the file")
		return false
	}

	cmsFree(iohandler.ContextID, iohandler)
	return true
}
func cmsWriteRawTag(hProfile CmsHPROFILE, sig cmsTagSignature, data interface{}, size uint32) bool {
	Icc := hProfile.(*cmsICCPROFILE)
	mm := &Icc.UsrMutex
	var i int

	if !cmsLockMutex(Icc.ContextID, (*cmsMutex)(mm)) {
		return false
	}

	if !cmsNewTag(Icc, sig, &i) {
		cmsUnlockMutex(Icc.ContextID, (*cmsMutex)(mm))
		return false
	}

	Icc.TagSaveAsRaw[i] = true
	Icc.TagNames[i] = sig
	Icc.TagLinked[i] = 0

	Icc.TagPtrs[i] = cmsDupMem(Icc.ContextID, data, size)
	Icc.TagSizes[i] = size

	cmsUnlockMutex(Icc.ContextID, (*cmsMutex)(mm))

	if Icc.TagPtrs[i] == nil {
		Icc.TagNames[i] = 0
		return false
	}
	return true
}
func cmsLinkTag(hProfile CmsHPROFILE, sig cmsTagSignature, dest cmsTagSignature) bool {
	Icc := hProfile.(*cmsICCPROFILE)
	var i int
	mm := &Icc.UsrMutex
	if !cmsLockMutex(Icc.ContextID, (*cmsMutex)(mm)) {
		return false
	}

	if !cmsNewTag(Icc, sig, &i) {
		cmsUnlockMutex(Icc.ContextID, (*cmsMutex)(mm))
		return false
	}

	Icc.TagSaveAsRaw[i] = false
	Icc.TagNames[i] = sig
	Icc.TagLinked[i] = dest
	Icc.TagPtrs[i] = nil
	Icc.TagSizes[i] = 0
	Icc.TagOffsets[i] = 0

	cmsUnlockMutex(Icc.ContextID, (*cmsMutex)(mm))
	return true
}
func cmsTagLinkedTo(hProfile CmsHPROFILE, sig cmsTagSignature) cmsTagSignature {
	Icc := hProfile.(*cmsICCPROFILE)
	i := cmsSearchTag(Icc, sig, false)

	if i < 0 {
		return 0 // Not found
	}

	return Icc.TagLinked[i]
}

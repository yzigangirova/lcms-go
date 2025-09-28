package golcms

import (
	//"bytes"
	"encoding/binary"
	"sync"
	"time"

	//"fmt"
	"math"
	"unsafe"

	"github.com/yzigangirova/lcms-go/mem"
)

// Check if the platform is little-endian

// Platform endianess determined at runtime
/* var platformEndian binary.ByteOrder

func init() {
	if isBigEndian() {
		platformEndian = binary.BigEndian
	} else {
		platformEndian = binary.LittleEndian
	}
}*/

/*// Adjust a 16-bit value for the platform endianess
func cmsAdjustEndianess16(word uint16) uint16 {
	if platformEndian == binary.BigEndian {
		return word
	}
	return (word << 8) | (word >> 8)
}

// Adjust a 32-bit value for the platform endianess
func cmsAdjustEndianess32(dword uint32) uint32 {
	if platformEndian == binary.BigEndian {
		return dword
	}
	return (dword&0xFF)<<24 |
		(dword&0xFF00)<<8 |
		(dword&0xFF0000)>>8 |
		(dword>>24)&0xFF
}

// Adjust a 64-bit value for the platform endianess
func cmsAdjustEndianess64(qword uint64) uint64 {
	if platformEndian == binary.BigEndian {
		return qword
	}
	return (qword&0xFF)<<56 |
		(qword&0xFF00)<<40 |
		(qword&0xFF0000)<<24 |
		(qword&0xFF000000)<<8 |
		(qword&0xFF00000000)>>8 |
		(qword&0xFF0000000000)>>24 |
		(qword&0xFF000000000000)>>40 |
		(qword>>56)&0xFF
}*/

// Auxiliary -- read 8, 16 and 32-bit numbers
// cmsReadUInt8Number reads a single uint8 number.

func cmsReadUInt8Number(io *cmsIOHANDLER, n *uint8) bool {
	var tmp uint8

	tmp, err := ReadStruct[uint8](io, binary.BigEndian, 1)
	if err != nil {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Failed to read uint8: %v", err)
		return false
	}

	if n != nil {
		*n = tmp
	}
	return true
}

// cmsReadUInt16Number reads a single uint16 number.
func cmsReadUInt16Number(io *cmsIOHANDLER, n *uint16) bool {
	var tmp uint16

	tmp, err := ReadStruct[uint16](io, binary.BigEndian, 1)
	if err != nil {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Failed to read uint16: %v", err)
		return false
	}

	if n != nil {
		*n = tmp
	}
	return true
}

// cmsReadUInt16Array reads an array of uint16 numbers.
func cmsReadUInt16Array(io *cmsIOHANDLER, n uint32, array []uint16) bool {
	for i := uint32(0); i < n; i++ {
		if array != nil {
			if !cmsReadUInt16Number(io, &array[i]) {
				return false
			}
		} else {
			if !cmsReadUInt16Number(io, nil) {
				return false
			}
		}
	}
	return true
}

// cmsReadUInt32Number reads a single uint32 number.
func cmsReadUInt32Number(io *cmsIOHANDLER, n *uint32) bool {
	var tmp uint32

	tmp, err := ReadStruct[uint32](io, binary.BigEndian, 1)
	if err != nil {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Failed to read uint32: %v", err)
		return false
	}
	if n != nil {
		*n = tmp
	}
	return true
}

// cmsReadFloat32Number reads a single float32 number.
func cmsReadFloat32Number(io *cmsIOHANDLER, n *float32) bool {
	var tmp struct {
		Integer uint32
	}
	var err error
	tmp.Integer, err = ReadStruct[uint32](io, binary.BigEndian, 1)
	if err != nil {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Failed to read uint32: %v", err)
		return false
	}

	if n != nil {
		*n = math.Float32frombits(tmp.Integer)

		// Safeguard against absurd values
		if *n > 1e20 || *n < -1e20 {
			return false
		}

		// Additional C99 fpclassify handling
		if math.IsNaN(float64(*n)) || math.IsInf(float64(*n), 0) {
			return false
		}
	}

	return true
}

// cmsReadUInt64Number reads a single uint64 number.
func cmsReadUInt64Number(io *cmsIOHANDLER, n *uint64) bool {
	var tmp uint64

	tmp, err := ReadStruct[uint64](io, binary.BigEndian, 1)
	if err != nil {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Failed to read uint64: %v", err)
		return false
	}
	if n != nil {
		*n = tmp
	}

	return true
}

// cmsRead15Fixed16Number reads a 15.16 fixed-point number as a float64.
func cmsRead15Fixed16Number(io *cmsIOHANDLER, n *float64) bool {
	var tmp uint32

	tmp, err := ReadStruct[uint32](io, binary.BigEndian, 1)
	if err != nil {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Failed to read uint32: %v", err)
		return false
	}

	if n != nil {
		*n = cms15Fixed16ToDouble(int32(tmp))
	}

	return true
}

// cmsReadXYZNumber reads an XYZ color space number.
func cmsReadXYZNumber(io *cmsIOHANDLER, XYZ *cmsCIEXYZ) bool {
	var xyz cmsEncodedXYZNumber

	xyz, err := ReadStruct[cmsEncodedXYZNumber](io, binary.BigEndian, 1)
	if err != nil {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Failed to read uint32: %v", err)
		return false
	}

	if XYZ != nil {
		XYZ.X = cms15Fixed16ToDouble(int32(xyz.X))
		XYZ.Y = cms15Fixed16ToDouble(int32(xyz.Y))
		XYZ.Z = cms15Fixed16ToDouble(int32(xyz.Z))
	}
	return true
}

// Writing Functions

func cmsWriteUInt8Number(io *cmsIOHANDLER, n uint8) bool {
	cmsAssert(io != nil, "nil pointer in cmsWriteUInt8Number")
	return WriteStruct[uint8](io, n, binary.BigEndian)
}

func cmsWriteUInt16Number(io *cmsIOHANDLER, n uint16) bool {
	cmsAssert(io != nil, "nil pointer in cmsWriteUInt16Number")
	return WriteStruct[uint16](io, n, binary.BigEndian)
}

func cmsWriteUInt16Array(io *cmsIOHANDLER, n uint32, array []uint16) bool {
	cmsAssert(io != nil, "nil pointer in cmsWriteUInt16Array")
	for i := uint32(0); i < n; i++ {
		if !WriteStruct[uint16](io, array[i], binary.BigEndian) {
			return false
		}
	}
	return true
}
func cmsWriteUInt32Number(io *cmsIOHANDLER, n uint32) bool {
	cmsAssert(io != nil, "nil pointer in cmsWriteUInt32Number")

	return WriteStruct[uint32](io, n, binary.BigEndian)
}

func cmsWriteFloat32Number(io *cmsIOHANDLER, n float32) bool {
	cmsAssert(io != nil, "nil pointer in cmsWriteFloat32Number")

	return WriteStruct[float32](io, n, binary.BigEndian)
}
func cmsWriteUInt64Number(io *cmsIOHANDLER, n uint64) bool {
	cmsAssert(io != nil, "nil pointer in cmsWriteUInt64Number")
	return WriteStruct[uint64](io, n, binary.BigEndian)
}

func cmsWrite15Fixed16Number(io *cmsIOHANDLER, n float64) bool {
	cmsAssert(io != nil, "nil pointer in cmsWrite15Fixed16Number")

	fixed := uint32(cmsDoubleTo15Fixed16(n))
	return WriteStruct[uint32](io, fixed, binary.BigEndian)

}

func cmsWriteXYZNumber(io *cmsIOHANDLER, xyz *cmsCIEXYZ) bool {
	cmsAssert(io != nil, "nil pointer in cmsWriteXYZNumber")
	cmsAssert(xyz != nil, "nil pointer in cmsWriteXYZNumber")

	encodedXYZ := cmsEncodedXYZNumber{
		X: cmsS15Fixed16Number(cmsDoubleTo15Fixed16(xyz.X)),
		Y: cmsS15Fixed16Number(cmsDoubleTo15Fixed16(xyz.Y)),
		Z: cmsS15Fixed16Number(cmsDoubleTo15Fixed16(xyz.Z)),
	}

	return WriteStruct[cmsEncodedXYZNumber](io, encodedXYZ, binary.BigEndian)
}

// Fixed Point Conversions

func cms8Fixed8ToDouble(fixed8 uint16) float64 {
	lsb := uint8(fixed8 & 0xff)
	msb := uint8((fixed8 >> 8) & 0xff)
	return float64(msb) + float64(lsb)/256.0
}

func cmsDoubleTo8Fixed8(val float64) uint16 {
	gammaFixed32 := cmsDoubleTo15Fixed16(val)
	return uint16((gammaFixed32 >> 8) & 0xFFFF)
}

func cms15Fixed16ToDouble(fix32 int32) float64 {
	sign := 1.0
	if fix32 < 0 {
		sign = -1.0
		fix32 = -fix32
	}

	whole := uint16((fix32 >> 16) & 0xffff)
	fracPart := uint16(fix32 & 0xffff)

	mid := float64(fracPart) / 65536.0
	floater := float64(whole) + mid

	return sign * floater
}

// from double to Fixed point 15.16
func cmsDoubleTo15Fixed16(v float64) cmsS15Fixed16Number {
	return cmsS15Fixed16Number(math.Floor((v)*65536.0 + 0.5))
}

// Date/Time Functions

func cmsDecodeDateTimeNumber(source *cmsDateTimeNumber) time.Time {
	return time.Date(
		int(source.Year),
		time.Month(source.Month),
		int(source.Day),
		int(source.Hours),
		int(source.Minutes),
		int(source.Seconds),
		0,
		time.UTC,
	)
}

func cmsEncodeDateTimeNumber(dest *cmsDateTimeNumber, t time.Time) {
	dest.Seconds = uint16(t.Second())
	dest.Minutes = uint16(t.Minute())
	dest.Hours = uint16(t.Hour())
	dest.Day = uint16(t.Day())
	dest.Month = uint16(t.Month())
	dest.Year = uint16(t.Year())
}

// Read/Write Base Tag

func cmsReadTypeBase(io *cmsIOHANDLER) cmsTagTypeSignature {
	var base CmsTagBase
	base, err := ReadStruct[CmsTagBase](io, binary.BigEndian, 1)
	if err != nil {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Failed to read uint32: %v", err)
		return 0
	}
	return base.Sig
}

func cmsWriteTypeBase(io *cmsIOHANDLER, sig cmsTagTypeSignature) bool {
	var base CmsTagBase
	base.Sig = sig
	for i := range base.Reserved {
		base.Reserved[i] = 0
	}
	return WriteStruct[CmsTagBase](io, base, binary.BigEndian)
}

// Alignment Functions

func cmsReadAlignment(io *cmsIOHANDLER) bool {
	At := io.Tell((*cms_io_handler)(io))
	nextAligned := cmsALIGNLONG(At)
	bytesToNextAlignedPos := nextAligned - At
	var buffer [4]uint8

	if bytesToNextAlignedPos == 0 {
		return true
	}
	if bytesToNextAlignedPos > 4 {
		return false
	}

	return io.Read((*cms_io_handler)(io), &buffer, uint32(unsafe.Sizeof(buffer)), 1) == 1
}

func cmsWriteAlignment(io *cmsIOHANDLER) bool {
	At := io.Tell((*cms_io_handler)(io))
	nextAligned := cmsALIGNLONG(At)
	bytesToNextAlignedPos := nextAligned - At
	var buffer [4]uint8

	if bytesToNextAlignedPos == 0 {
		return true
	}
	if bytesToNextAlignedPos > 4 {
		return false
	}
	return io.Write((*cms_io_handler)(io), uint32(len(buffer)), buffer[:])
}

// Plugin memory management -------------------------------------------------------------------------------------------------

// Specialized malloc for plugins, freed upon exit
/*func cmsPluginMalloc(contextID CmsContext, size uint32) unsafe.Pointer {
	ctx := cmsGetContext(contextID)

	if ctx.MemPool == nil {
		if contextID == nil {
			ctx.MemPool = cmsCreateSubAlloc(nil, 2*1024)
			if ctx.MemPool == nil {
				return nil
			}
		} else {
			cmsSignalError(ContextID, cmsERROR_CORRUPTION_DETECTED, "nil memory pool on context")
			return nil
		}
	}

	return cmsSubAlloc(ctx.MemPool, size)
}*/

// Main plugin dispatcher
func cmsPlugin(mm mem.Manager, plugin PluginIntrfc) bool {
	return cmsPluginTHR(mm, nil, plugin)
}

// Plugin dispatcher for a specific thread
//
//lint:ignore U1000 kept for parity with lcms; used in future ports
func cmsPluginTHR(mm mem.Manager, contextID CmsContext, plugin PluginIntrfc) bool {
	currentPlugin, ok := plugin.(*cmsPluginBase)
	if !ok {
		panic("Plugin is not of the type cmsPluginBase\n")

	}
	for currentPlugin != nil {
		if currentPlugin.Magic != cmsPluginMagicNumber {
			cmsSignalError(contextID, cmsERROR_UNKNOWN_EXTENSION, "Unrecognized plugin")
			return false
		}

		if currentPlugin.ExpectedVersion > LCMS_VERSION {
			cmsSignalError(contextID, cmsERROR_UNKNOWN_EXTENSION, "Unrecognized plugin")
			return false
		}

		switch currentPlugin.Type {
		case cmsPluginMemHandlerSig:
			if cmsRegisterMemHandlerPlugin(contextID, currentPlugin) {
				return false
			}
		case cmsPluginInterpolationSig:
			if !cmsRegisterInterpPlugin(contextID, currentPlugin) {
				return false
			}
		case cmsPluginTagTypeSig:
			if !cmsRegisterTagTypePlugin(mm, contextID, currentPlugin) {
				return false
			}
		case cmsPluginTagSig:
			if !cmsRegisterTagPlugin(mm, contextID, currentPlugin) {
				return false
			}
		case cmsPluginFormattersSig:
			if !cmsRegisterFormattersPlugin(mm, contextID, currentPlugin) {
				return false
			}
		case cmsPluginRenderingIntentSig:
			if !cmsRegisterRenderingIntentPlugin(mm, contextID, currentPlugin) {
				return false
			}
		case cmsPluginParametricCurveSig:
			if !cmsRegisterParametricCurvesPlugin(mm, contextID, currentPlugin) {
				return false
			}
		case cmsPluginMultiProcessElementSig:
			if !cmsRegisterMultiProcessElementPlugin(mm, contextID, currentPlugin) {
				return false
			}
		case cmsPluginOptimizationSig:
			if !cmsRegisterOptimizationPlugin(mm, contextID, currentPlugin) {
				return false
			}
		case cmsPluginTransformSig:
			if !cmsRegisterTransformPlugin(mm, contextID, currentPlugin) {
				return false
			}
		case cmsPluginMutexSig:
			if !cmsRegisterMutexPlugin(contextID, currentPlugin) {
				return false
			}
		case cmsPluginParallelizationSig:
			if !cmsRegisterParallelizationPlugin(contextID, currentPlugin) {
				return false
			}
		default:
			cmsSignalError(contextID, cmsERROR_UNKNOWN_EXTENSION, "Unrecognized plugin type")
			return false
		}

		currentPlugin = currentPlugin.Next
	}

	// Plugins registered successfully
	return true
}

// Revert all plugins to default
func cmsUnregisterPlugins(mm mem.Manager) {
	cmsUnregisterPluginsTHR(mm, nil)
}

/* C-code The context pool (linked list head)  NOT IMPEMENTED NEEDS FURTHER CONSIDERATION
static _cmsMutex _cmsContextPoolHeadMutex = CMS_MUTEX_INITIALIZER;
static struct _cmsContext_struct* _cmsContextPoolHead = NULL;
*/
// Mutex for context pool head
var (
	CmsContextPoolHeadMutex sync.Mutex
	CmsContextPoolHead      CmsContext
	initializedMutex        sync.Once
)

// Initialize the context mutex
func InitContextMutex() bool {
	/*	var initializationSuccessful bool

		initializedMutex.Do(func() {
			defer func() {
				// Recover from any unexpected panic during initialization
				if r := recover(); r != nil {
					initializationSuccessful = false
					cmsSignalError(nil, 1, "Context mutex initialization failed")
				}
			}()

			// Lock the global mutex to simulate initialization
			contextMutex.Lock()
			defer contextMutex.Unlock()

			// Simulate some initialization logic
			initializationSuccessful = true
		})*/

	return true
}

type cmsContextChunk any

// Global storage for system context
var globalContext = CmsContextStruct{
	Next:    nil, // Not in the linked list
	MemPool: nil, // No suballocator
	chunks: [MemoryClientMax]cmsContextChunk{
		nil,                            // UserPtr
		&cmsLogErrorChunk,              // Logger
		&cmsAlarmCodesChunk,            // AlarmCodes
		&cmsAdaptationStateChunk,       // AdaptationState
		&cmsMemPluginChunk,             // MemPlugin
		&cmsInterpPluginChunk,          // InterpPlugin
		&cmsCurvesPluginChunk,          // CurvesPlugin
		&cmsFormattersPluginChunk,      // FormattersPlugin
		&cmsTagTypePluginChunk,         // TagTypePlugin
		&cmsTagPluginChunk,             // TagPlugin
		&cmsIntentsPluginChunk,         // IntentPlugin
		&cmsMPETypePluginChunk,         // MPEPlugin
		&cmsOptimizationPluginChunk,    // OptimizationPlugin
		&cmsTransformPluginChunk,       // TransformPlugin
		&cmsMutexPluginChunk,           // MutexPlugin
		&cmsParallelizationPluginChunk, // ParallelizationPlugin
	}, // The default memory allocator is not used for context 0
}

// cmsGetContext retrieves the associated context pointer, with guessing. Never returns nil.
func cmsGetContext(ContextID CmsContext) CmsContext {
	id := (CmsContext)(ContextID)

	// Use global settings if ContextID is nil
	if id == nil {
		return &globalContext
	}

	InitContextMutex()

	// Enter critical section
	mm1 := &CmsContextPoolHeadMutex
	cm1 := cmsMutex(mm1)
	cmsEnterCriticalSectionPrimitive(&cm1)

	// Search through the context pool
	for ctx := CmsContextPoolHead; ctx != nil; ctx = ctx.Next {
		if id == ctx {
			// Leave critical section and return the context
			mm2 := &CmsContextPoolHeadMutex
			cm2 := cmsMutex(mm2)
			cmsLeaveCriticalSectionPrimitive(&cm2)
			return ctx
		}
	}

	// Leave critical section if not found
	mm3 := &CmsContextPoolHeadMutex
	cm3 := cmsMutex(mm3)
	cmsLeaveCriticalSectionPrimitive(&cm3)
	return &globalContext
}

// This function returns the given context its default pristine state,
// as no plug-ins were declared. There is no way to unregister a single
// plug-in, as a single call to cmsPluginTHR() function may register
// many different plug-ins simultaneously, then there is no way to
// identify which plug-in to unregister.
//
//lint:ignore U1000 kept for parity with lcms; used in future ports
func cmsUnregisterPluginsTHR(mm mem.Manager, ContextID CmsContext) {
	cmsRegisterMemHandlerPlugin(ContextID, nil)
	cmsRegisterInterpPlugin(ContextID, nil)
	cmsRegisterTagTypePlugin(mm, ContextID, nil)
	cmsRegisterTagPlugin(mm, ContextID, nil)
	cmsRegisterFormattersPlugin(mm, ContextID, nil)
	cmsRegisterRenderingIntentPlugin(mm, ContextID, nil)
	cmsRegisterParametricCurvesPlugin(mm, ContextID, nil)
	cmsRegisterMultiProcessElementPlugin(mm, ContextID, nil)
	cmsRegisterOptimizationPlugin(mm, ContextID, nil)
	cmsRegisterTransformPlugin(mm, ContextID, nil)
	cmsRegisterMutexPlugin(ContextID, nil)
	cmsRegisterParallelizationPlugin(ContextID, nil)

}

// CmsContextGetClientChunk retrieves the memory area associated with each context client
// Internal: get the memory area associanted with each context client
// Returns the block assigned to the specific zone. Never return nil.
func CmsContextGetClientChunk(ContextID CmsContext, mc cmsMemoryClient) any {
	if mc < 0 || mc >= MemoryClientMax {
		cmsSignalError(ContextID, cmsERROR_INTERNAL, "Bad context client -- possible corruption")

		// This is catastrophic. Should never reach here
		cmsAssert(false, "Bad context client -- possible corruption")

		// Reverts to global context
		return globalContext.chunks[UserPtr]
	}

	ctx := cmsGetContext(ContextID)
	ptr := ctx.chunks[mc]

	if ptr != nil {
		return ptr
	}

	// A nil ptr means no special settings for that context, and this reverts to globalContext globals
	return globalContext.chunks[mc]
}

// cmsGetTime provides thread-safe time retrieval and populates the given *time.Time with UTC time.
func cmsGetTime(ptrTime *time.Time) bool {
	// Get the current time
	now := time.Now()

	// Ensure thread safety with a mutex
	if !InitContextMutex() {
		return false
	}
	mm := &CmsContextPoolHeadMutex
	cm := (cmsMutex(mm))
	cmsEnterCriticalSectionPrimitive(&cm)

	// Convert to UTC
	utcTime := now.UTC()
	cmsLeaveCriticalSectionPrimitive(&cm)

	if ptrTime == nil {
		return false
	}

	*ptrTime = utcTime
	return true
}

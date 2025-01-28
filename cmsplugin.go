package golcms

import (
	"bytes"
	"encoding/binary"
	"sync"
	"time"

	//"fmt"
	"math"
	"unsafe"
)

// Check if the platform is little-endian
func IsLittleEndian() bool {
	var test uint16 = 0x1
	return (*[2]byte)(unsafe.Pointer(&test))[0] == 0x1
}

// Platform endianess determined at runtime
var platformEndian binary.ByteOrder

func init() {
	if IsLittleEndian() {
		platformEndian = binary.LittleEndian
	} else {
		platformEndian = binary.BigEndian
	}
}

// Adjust a 16-bit value for the platform endianess
func cmsAdjustEndianess16(word uint16) uint16 {
	if platformEndian == binary.BigEndian {
		return word // No adjustment needed
	}

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, word)
	var adjusted uint16
	binary.Read(&buf, binary.BigEndian, &adjusted)
	return adjusted
}

// Adjust a 32-bit value for the platform endianess
func cmsAdjustEndianess32(dword uint32) uint32 {
	if platformEndian == binary.BigEndian {
		return dword // No adjustment needed
	}

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, dword)
	var adjusted uint32
	binary.Read(&buf, binary.BigEndian, &adjusted)
	return adjusted
}

// Adjust a 64-bit value for the platform endianess
func cmsAdjustEndianess64(qword uint64) uint64 {
	if platformEndian == binary.BigEndian {
		return qword // No adjustment needed
	}

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, qword)
	var adjusted uint64
	binary.Read(&buf, binary.BigEndian, &adjusted)
	return adjusted
}

// Auxiliary -- read 8, 16 and 32-bit numbers
// cmsReadUInt8Number reads a single uint8 number.

func cmsReadUInt8Number(io *cmsIOHANDLER, n *uint8) bool {
	var tmp uint8

	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&tmp), uint32(unsafe.Sizeof(tmp)), 1) != 1 {
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

	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&tmp), uint32(unsafe.Sizeof(tmp)), 1) != 1 {
		return false
	}

	if n != nil {
		*n = cmsAdjustEndianess16(tmp)
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

	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&tmp), uint32(unsafe.Sizeof(tmp)), 1) != 1 {
		return false
	}

	if n != nil {
		*n = cmsAdjustEndianess32(tmp)
	}
	return true
}

// cmsReadFloat32Number reads a single float32 number.
func cmsReadFloat32Number(io *cmsIOHANDLER, n *float32) bool {
	var tmp struct {
		Integer uint32
	}

	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&tmp.Integer), uint32(unsafe.Sizeof(tmp.Integer)), 1) != 1 {
		return false
	}

	if n != nil {
		tmp.Integer = cmsAdjustEndianess32(tmp.Integer)
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

	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&tmp), uint32(unsafe.Sizeof(tmp)), 1) != 1 {
		return false
	}

	if n != nil {
		*n = cmsAdjustEndianess64(tmp)
	}

	return true
}

// cmsRead15Fixed16Number reads a 15.16 fixed-point number as a float64.
func cmsRead15Fixed16Number(io *cmsIOHANDLER, n *float64) bool {
	var tmp uint32

	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&tmp), uint32(unsafe.Sizeof(tmp)), 1) != 1 {
		return false
	}

	if n != nil {
		*n = cms15Fixed16ToDouble(int32(cmsAdjustEndianess32(tmp)))
	}

	return true
}

// cmsReadXYZNumber reads an XYZ color space number.
func cmsReadXYZNumber(io *cmsIOHANDLER, XYZ *cmsCIEXYZ) bool {
	var xyz cmsEncodedXYZNumber

	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&xyz), uint32(unsafe.Sizeof(xyz)), 1) != 1 {
		return false
	}

	if XYZ != nil {
		XYZ.X = cms15Fixed16ToDouble(int32(cmsAdjustEndianess32(uint32(xyz.X))))
		XYZ.Y = cms15Fixed16ToDouble(int32(cmsAdjustEndianess32(uint32(xyz.Y))))
		XYZ.Z = cms15Fixed16ToDouble(int32(cmsAdjustEndianess32(uint32(xyz.Z))))
	}
	return true
}

// Writing Functions

func cmsWriteUInt8Number(io *cmsIOHANDLER, n uint8) bool {
	if io == nil {
		panic("nil pointer in cmsWriteUInt8Number")
	}

	if io.Write((*cms_io_handler)(io), 1, unsafe.Pointer(&n)) != true {
		return false
	}
	return true
}

func cmsWriteUInt16Number(io *cmsIOHANDLER, n uint16) bool {
	if io == nil {
		panic("nil pointer in cmsWriteUInt16Number")
	}

	tmp := cmsAdjustEndianess16(n)
	if io.Write((*cms_io_handler)(io), 2, unsafe.Pointer(&tmp)) != true {
		return false
	}
	return true
}

func cmsWriteUInt16Array(io *cmsIOHANDLER, n uint32, array []uint16) bool {
	if io == nil || array == nil {
		panic("nil pointer in cmsWriteUInt16Array")
	}

	for i := uint32(0); i < n; i++ {
		if !cmsWriteUInt16Number(io, array[i]) {
			return false
		}
	}
	return true
}

func cmsWriteUInt32Number(io *cmsIOHANDLER, n uint32) bool {
	if io == nil {
		panic("nil pointer in cmsWriteUInt32Number")
	}

	tmp := cmsAdjustEndianess32(n)
	if io.Write((*cms_io_handler)(io), 4, unsafe.Pointer(&tmp)) != true {
		return false
	}
	return true
}

func cmsWriteFloat32Number(io *cmsIOHANDLER, n float32) bool {
	if io == nil {
		panic("nil pointer in cmsWriteFloat32Number")
	}

	tmp := math.Float32bits(n)
	tmp = cmsAdjustEndianess32(tmp)
	if !io.Write((*cms_io_handler)(io), 4, unsafe.Pointer(&tmp)) {
		return false
	}
	return true
}

func cmsWriteUInt64Number(io *cmsIOHANDLER, n uint64) bool {
	if io == nil {
		panic("nil pointer in cmsWriteUInt64Number")
	}

	tmp := cmsAdjustEndianess64(n)
	if !io.Write((*cms_io_handler)(io), 8, unsafe.Pointer(&tmp)) {
		return false
	}
	return true
}

func cmsWrite15Fixed16Number(io *cmsIOHANDLER, n float64) bool {
	if io == nil {
		panic("nil pointer in cmsWrite15Fixed16Number")
	}

	tmp := cmsAdjustEndianess32(uint32(cmsDoubleTo15Fixed16(n)))
	if !io.Write((*cms_io_handler)(io), 4, unsafe.Pointer(&tmp)) {
		return false
	}
	return true
}

func cmsWriteXYZNumber(io *cmsIOHANDLER, xyz *cmsCIEXYZ) bool {
	if io == nil || xyz == nil {
		panic("nil pointer in cmsWriteXYZNumber")
	}

	var encodedXYZ cmsEncodedXYZNumber
	encodedXYZ.X = cmsS15Fixed16Number(cmsAdjustEndianess32(uint32(cmsDoubleTo15Fixed16(xyz.X))))
	encodedXYZ.Y = cmsS15Fixed16Number(cmsAdjustEndianess32(uint32(cmsDoubleTo15Fixed16(xyz.Y))))
	encodedXYZ.Z = cmsS15Fixed16Number(cmsAdjustEndianess32(uint32(cmsDoubleTo15Fixed16(xyz.Z))))

	if !io.Write((*cms_io_handler)(io), uint32(binary.Size(encodedXYZ)), unsafe.Pointer(&encodedXYZ)) {
		return false
	}
	return true
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
		int(cmsAdjustEndianess16(source.year)),
		time.Month(cmsAdjustEndianess16(source.month)),
		int(cmsAdjustEndianess16(source.day)),
		int(cmsAdjustEndianess16(source.hours)),
		int(cmsAdjustEndianess16(source.minutes)),
		int(cmsAdjustEndianess16(source.seconds)),
		0,
		time.UTC,
	)
}

func cmsEncodeDateTimeNumber(dest *cmsDateTimeNumber, t time.Time) {
	dest.seconds = cmsAdjustEndianess16(uint16(t.Second()))
	dest.minutes = cmsAdjustEndianess16(uint16(t.Minute()))
	dest.hours = cmsAdjustEndianess16(uint16(t.Hour()))
	dest.day = cmsAdjustEndianess16(uint16(t.Day()))
	dest.month = cmsAdjustEndianess16(uint16(t.Month()))
	dest.year = cmsAdjustEndianess16(uint16(t.Year()))
}

// Read/Write Base Tag

func cmsReadTypeBase(io *cmsIOHANDLER) cmsTagTypeSignature {
	var base cmsTagBase
	if io.Read((*cms_io_handler)(io), unsafe.Pointer(&base), uint32(unsafe.Sizeof(base)), 1) != 1 {
		return 0
	}
	return cmsTagTypeSignature(cmsAdjustEndianess32(uint32(base.Sig)))
}

func cmsWriteTypeBase(io *cmsIOHANDLER, sig cmsTagTypeSignature) bool {
	var base cmsTagBase
	base.Sig = cmsTagTypeSignature(cmsAdjustEndianess32(uint32(sig)))
	for i := range base.Reserved {
		base.Reserved[i] = 0
	}
	return io.Write((*cms_io_handler)(io), uint32(unsafe.Sizeof(base)), unsafe.Pointer(&base))
}

// Alignment Functions

func cmsReadAlignment(io *cmsIOHANDLER) bool {
	currentPos := io.Tell((*cms_io_handler)(io))
	nextAligned := cmsALIGNLONG(currentPos)
	bytesToNextAlignedPos := nextAligned - currentPos

	if bytesToNextAlignedPos == 0 {
		return true
	}
	if bytesToNextAlignedPos > 4 {
		return false
	}

	buffer := make([]byte, bytesToNextAlignedPos)
	return io.Read((*cms_io_handler)(io), unsafe.Pointer(&buffer), uint32(unsafe.Sizeof(buffer)), 1) == 1
}

func cmsWriteAlignment(io *cmsIOHANDLER) bool {
	currentPos := io.Tell((*cms_io_handler)(io))
	nextAligned := cmsALIGNLONG(currentPos)
	bytesToNextAlignedPos := nextAligned - currentPos

	if bytesToNextAlignedPos == 0 {
		return true
	}
	if bytesToNextAlignedPos > 4 {
		return false
	}

	buffer := make([]byte, bytesToNextAlignedPos)
	for i := range buffer {
		buffer[i] = 0
	}
	return io.Write((*cms_io_handler)(io), uint32(len(buffer)), unsafe.Pointer(&buffer))
}

// Plugin memory management -------------------------------------------------------------------------------------------------

// Specialized malloc for plugins, freed upon exit
func cmsPluginMalloc(contextID CmsContext, size uint32) unsafe.Pointer {
	ctx := cmsGetContext(contextID)

	if ctx.MemPool == nil {
		if contextID == nil {
			ctx.MemPool = cmsCreateSubAlloc(nil, 2*1024)
			if ctx.MemPool == nil {
				return nil
			}
		} else {
			cmsSignalError(unsafe.Pointer(contextID), cmsERROR_CORRUPTION_DETECTED, "nil memory pool on context")
			return nil
		}
	}

	return cmsSubAlloc(ctx.MemPool, size)
}

// Main plugin dispatcher
func cmsPlugin(plugin unsafe.Pointer) bool {
	return cmsPluginTHR(nil, plugin)
}

// Plugin dispatcher for a specific thread
func cmsPluginTHR(contextID CmsContext, plugin unsafe.Pointer) bool {
	currentPlugin := (*cmsPluginBase)(plugin)

	for currentPlugin != nil {
		if currentPlugin.Magic != cmsPluginMagicNumber {
			cmsSignalError(unsafe.Pointer(contextID), cmsERROR_UNKNOWN_EXTENSION, "Unrecognized plugin")
			return false
		}

		if currentPlugin.ExpectedVersion > LCMS_VERSION {
			cmsSignalError(unsafe.Pointer(contextID), cmsERROR_UNKNOWN_EXTENSION, "Unrecognized plugin")
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
			if !cmsRegisterTagTypePlugin(contextID, currentPlugin) {
				return false
			}
		case cmsPluginTagSig:
			if !cmsRegisterTagPlugin(contextID, currentPlugin) {
				return false
			}
		case cmsPluginFormattersSig:
			if !cmsRegisterFormattersPlugin(contextID, currentPlugin) {
				return false
			}
		case cmsPluginRenderingIntentSig:
			if !cmsRegisterRenderingIntentPlugin(contextID, currentPlugin) {
				return false
			}
		case cmsPluginParametricCurveSig:
			if !cmsRegisterParametricCurvesPlugin(contextID, currentPlugin) {
				return false
			}
		case cmsPluginMultiProcessElementSig:
			if !cmsRegisterMultiProcessElementPlugin(contextID, currentPlugin) {
				return false
			}
		case cmsPluginOptimizationSig:
			if !cmsRegisterOptimizationPlugin(contextID, currentPlugin) {
				return false
			}
		case cmsPluginTransformSig:
			if !cmsRegisterTransformPlugin(contextID, currentPlugin) {
				return false
			}
		case cmsPluginMutexSig:
			if !cmsRegisterMutexPlugin(contextID, currentPlugin) {
				return false
			}
		case cmsPluginParallelizationSig:
			if !cmsRegisterParallelizationPlugin(contextID, unsafe.Pointer(currentPlugin)) {
				return false
			}
		default:
			cmsSignalError(unsafe.Pointer(contextID), cmsERROR_UNKNOWN_EXTENSION, "Unrecognized plugin type")
			return false
		}

		currentPlugin = currentPlugin.Next
	}

	// Plugins registered successfully
	return true
}

// Revert all plugins to default
func cmsUnregisterPlugins() {
	cmsUnregisterPluginsTHR(nil)
}

// Mutex for context pool head
var (
	CmsContextPoolHeadMutex sync.Mutex
	CmsContextPoolHead      CmsContext
	initializedMutex        sync.Once
)

// Global mutex to ensure thread safety
var contextMutex sync.Mutex

// Use sync.Once to ensure initialization happens exactly once
var initOnce sync.Once

// Initialize the context mutex
func InitContextMutex() bool {
	var initializationSuccessful bool

	initOnce.Do(func() {
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
	})

	return initializationSuccessful
}

// Global storage for system context
var globalContext = CmsContextStruct{
	Next:    nil, // Not in the linked list
	MemPool: nil, // No suballocator
	chunks: [MemoryClientMax]unsafe.Pointer{
		nil,                                            // UserPtr
		unsafe.Pointer(&cmsLogErrorChunk),              // Logger
		unsafe.Pointer(&cmsAlarmCodesChunk),            // AlarmCodes
		unsafe.Pointer(&cmsAdaptationStateChunk),       // AdaptationState
		unsafe.Pointer(&cmsMemPluginChunk),             // MemPlugin
		unsafe.Pointer(&cmsInterpPluginChunk),          // InterpPlugin
		unsafe.Pointer(&cmsCurvesPluginChunk),          // CurvesPlugin
		unsafe.Pointer(&cmsFormattersPluginChunk),      // FormattersPlugin
		unsafe.Pointer(&cmsTagTypePluginChunk),         // TagTypePlugin
		unsafe.Pointer(&cmsTagPluginChunk),             // TagPlugin
		unsafe.Pointer(&cmsIntentsPluginChunk),         // IntentPlugin
		unsafe.Pointer(&cmsMPETypePluginChunk),         // MPEPlugin
		unsafe.Pointer(&cmsOptimizationPluginChunk),    // OptimizationPlugin
		unsafe.Pointer(&cmsTransformPluginChunk),       // TransformPlugin
		unsafe.Pointer(&cmsMutexPluginChunk),           // MutexPlugin
		unsafe.Pointer(&cmsParallelizationPluginChunk), // ParallelizationPlugin
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
	cmsEnterCriticalSectionPrimitive(&cmsMutex{mutex: CmsContextPoolHeadMutex})

	// Search through the context pool
	for ctx := CmsContextPoolHead; ctx != nil; ctx = ctx.Next {
		if id == ctx {
			// Leave critical section and return the context
			cmsLeaveCriticalSectionPrimitive(&cmsMutex{mutex: CmsContextPoolHeadMutex})
			return ctx
		}
	}

	// Leave critical section if not found
	cmsLeaveCriticalSectionPrimitive(&cmsMutex{mutex: CmsContextPoolHeadMutex})
	return &globalContext
}

// This function returns the given context its default pristine state,
// as no plug-ins were declared. There is no way to unregister a single
// plug-in, as a single call to cmsPluginTHR() function may register
// many different plug-ins simultaneously, then there is no way to
// identify which plug-in to unregister.
func cmsUnregisterPluginsTHR(ContextID CmsContext) {
	cmsRegisterMemHandlerPlugin(ContextID, nil)
	cmsRegisterInterpPlugin(ContextID, nil)
	cmsRegisterTagTypePlugin(ContextID, nil)
	cmsRegisterTagPlugin(ContextID, nil)
	cmsRegisterFormattersPlugin(ContextID, nil)
	cmsRegisterRenderingIntentPlugin(ContextID, nil)
	cmsRegisterParametricCurvesPlugin(ContextID, nil)
	cmsRegisterMultiProcessElementPlugin(ContextID, nil)
	cmsRegisterOptimizationPlugin(ContextID, nil)
	cmsRegisterTransformPlugin(ContextID, nil)
	cmsRegisterMutexPlugin(ContextID, nil)
	cmsRegisterParallelizationPlugin(ContextID, nil)

}

// CmsContextGetClientChunk retrieves the memory area associated with each context client
// Internal: get the memory area associanted with each context client
// Returns the block assigned to the specific zone. Never return nil.
func CmsContextGetClientChunk(ContextID CmsContext, mc cmsMemoryClient) unsafe.Pointer {
	if mc < 0 || mc >= MemoryClientMax {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_INTERNAL, "Bad context client -- possible corruption")

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
	contextMutex.Lock()
	defer contextMutex.Unlock()

	// Convert to UTC
	utcTime := now.UTC()

	if ptrTime == nil {
		return false
	}

	*ptrTime = utcTime
	return true
}

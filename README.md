# lcms-go

This is a port of [LittleCMS](https://www.littlecms.com/) (lcms2) to the Go programming language.

LittleCMS is a small, open-source color management engine that implements ICC color management. 
It uses ICC profiles to convert colors between device-dependent spaces (such as RGB and CMYK) 
and device-independent spaces (such as Lab and XYZ), providing consistent color reproduction 
across imaging and printing workflows.

The Go port is **incomplete** and still evolving.

## Endianness

Big-endian platforms are not currently supported.  
The code is structured so big-endian support can be added later, but it is non-functional for now.

## Scope

This Go port focuses on ICC-based color interpretation for common spaces — Gray, RGB, CMYK, Lab, XYZ 
— using existing profiles and standard PCS conversions.

To keep it lean, it omits features not required to *use* profiles, including:

- Profile-building and measurement I/O (e.g. CGATS/IT8, spectral data)
- Advanced appearance models (e.g. CIECAM02)
- Profile authoring utilities (profile writing / ID calculation)
- Advanced gamut analysis / mapping tools

## Multithreading / concurrency

Some multithreading-related elements from the original C code (flags, hooks, and structs) 
have been kept in the API for legacy compatibility and possible future use, but they are 
not active in the current Go port.

Actual parallelism is implemented using Go’s own goroutines and synchronization primitives, 
not the old C threading infrastructure.

## Memory management (work in progress)

I am exploring Go arena usage for better memory behavior; some parameters and hooks exist, 
but this is not developed yet.

Some C-style free functions are intentionally left in place 
for possible future use — they are no-ops for now.

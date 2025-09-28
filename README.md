This is a port of LittleCMS to the Go programming language.

The port is incomplete.

    Endianness
    Big-endian platforms are not currently supported. The code is structured so big-endian support can be added later, 
    but it is non-functional for now.

    Scope
    This Go port focuses on ICC-based color interpretation for common spaces—Gray, RGB, CMYK, Lab, XYZ 
    using existing profiles and standard PCS conversions. To keep it lean, it omits features not required 
    to use profiles: profile-building and measurement I/O (e.g., CGATS/IT8, spectral data), 
    advanced appearance models (e.g., CIECAM02), profile authoring utilities (profile writing/ID calculation), 
    and advanced gamut analysis/mapping tools.

    Memory management (work in progress)
    I’m exploring Go arena usage for better memory behavior; some parameters/hooks exist,
    but this is not developed yet. Some C-style free functions are intentionally left in place 
    for possible future use—they are no-ops for now.

I’ve tried to stay close to the LittleCMS style (both out of respect and for easier debugging), so some code looks “ungoish” (e.g., more goto than typical Go). That may change over time.

The port is still early; testing and issue reports are very welcome.

## Installation

Requires Go 1.22+.

go get github.com/yzigangirova/lcms-go



License & attribution

    License: MIT. See LICENSE.

    Copyright (this port): © 2025 Yuliana Zigangirova

    Upstream attribution: Portions derived from LittleCMS (lcms).
    © 1998–2023 Marti Maria Saguer — used under the MIT License.


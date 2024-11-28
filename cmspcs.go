package golcms

import (
	"math"
)

// ---------------------------------------------------------------------------------
//  Little Color Management System
//  Copyright (c) 1998-2023 Marti Maria Saguer
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the Software
// is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS
// OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
// WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
// ---------------------------------------------------------------------------------
//      inter PCS conversions XYZ <-> CIE L* a* b*
/*


       CIE 15:2004 CIELab is defined as:

       L* = 116*f(Y/Yn) - 16                     0 <= L* <= 100
       a* = 500*[f(X/Xn) - f(Y/Yn)]
       b* = 200*[f(Y/Yn) - f(Z/Zn)]

       and

              f(t) = t^(1/3)                     1 >= t >  (24/116)^3
                     (841/108)*t + (16/116)      0 <= t <= (24/116)^3


       Reverse transform is:

       X = Xn*[a* / 500 + (L* + 16) / 116] ^ 3   if (X/Xn) > (24/116)
         = Xn*(a* / 500 + L* / 116) / 7.787      if (X/Xn) <= (24/116)



       PCS in Lab2 is encoded as:

              8 bit Lab PCS:

                     L*      0..100 into a 0..ff byte.
                     a*      t + 128 range is -128.0  +127.0
                     b*

             16 bit Lab PCS:

                     L*     0..100  into a 0..ff00 word.
                     a*     t + 128  range is  -128.0  +127.9961
                     b*



Interchange Space   Component     Actual Range        Encoded Range
CIE XYZ             X             0 -> 1.99997        0x0000 -> 0xffff
CIE XYZ             Y             0 -> 1.99997        0x0000 -> 0xffff
CIE XYZ             Z             0 -> 1.99997        0x0000 -> 0xffff

Version 2,3
-----------

CIELAB (16 bit)     L*            0 -> 100.0          0x0000 -> 0xff00
CIELAB (16 bit)     a*            -128.0 -> +127.996  0x0000 -> 0x8000 -> 0xffff
CIELAB (16 bit)     b*            -128.0 -> +127.996  0x0000 -> 0x8000 -> 0xffff


Version 4
---------

CIELAB (16 bit)     L*            0 -> 100.0          0x0000 -> 0xffff
CIELAB (16 bit)     a*            -128.0 -> +127      0x0000 -> 0x8080 -> 0xffff
CIELAB (16 bit)     b*            -128.0 -> +127      0x0000 -> 0x8080 -> 0xffff

*/

// Conversions
func cmsXYZ2xyY(dest *cmsCIExyY, source *cmsCIEXYZ) {
	sum := 1.0 / (source.X + source.Y + source.Z)
	dest.x = source.X * sum
	dest.y = source.Y * sum
	dest.Y = source.Z
}

func xyY2XYZ(dest *cmsCIEXYZ, source *cmsCIExyY) {
	dest.X = (source.x / source.y) * source.Y
	dest.Y = source.Y
	dest.Z = ((1 - source.x - source.y) / source.y) * source.Y
}

/*
   The break point (24/116)^3 = (6/29)^3 is a very small amount of tristimulus
   primary (0.008856).  Generally, this only happens for
   nearly ideal blacks and for some orange / amber colors in transmission mode.
   For example, the Z value of the orange turn indicator lamp lens on an
   automobile will often be below this value.  But the Z does not
   contribute to the perceived color directly.
*/
// f(t) function used in Lab/XYZ conversions
func f(t cmsFloat64Number) cmsFloat64Number {
	limit := cmsFloat64Number(math.Pow(24.0/116.0, 3))
	if t <= limit {
		return (841.0/108.0)*t + (16.0 / 116.0)
	}
	return cmsFloat64Number(math.Cbrt(float64(t)))
}

// Inverse of f(t)
func f_1(t cmsFloat64Number) cmsFloat64Number {
	limit := cmsFloat64Number(24.0 / 116.0)
	if t <= limit {
		return (108.0 / 841.0) * (t - (16.0 / 116.0))
	}
	return cmsFloat64Number(math.Pow(float64(t), 3))
}

// Standard XYZ to Lab. it can handle negative XZY numbers in some cases
func cmsXYZ2Lab(whitePoint *cmsCIEXYZ, lab *cmsCIELab, xyz *cmsCIEXYZ) {
	if whitePoint == nil {
		whitePoint = &cmsCIEXYZ{X: 0.95047, Y: 1.00000, Z: 1.08883} // D50 white point
	}

	fx := f(xyz.X / whitePoint.X)
	fy := f(xyz.Y / whitePoint.Y)
	fz := f(xyz.Z / whitePoint.Z)

	lab.L = 116.0*fy - 16.0
	lab.a = 500.0 * (fx - fy)
	lab.b = 200.0 * (fy - fz)
}

// Lab to XYZ conversion
func cmsLab2XYZ(whitePoint *cmsCIEXYZ, xyz *cmsCIEXYZ, lab *cmsCIELab) {
	if whitePoint == nil {
		whitePoint = &cmsCIEXYZ{X: 0.95047, Y: 1.00000, Z: 1.08883} // D50 white point
	}

	y := (lab.L + 16.0) / 116.0
	x := y + 0.002*lab.a
	z := y - 0.005*lab.b

	xyz.X = f_1(x) * whitePoint.X
	xyz.Y = f_1(y) * whitePoint.Y
	xyz.Z = f_1(z) * whitePoint.Z
}

// Helper functions to convert Lab values to float and back
func L2float2(v cmsUInt16Number) cmsFloat64Number {
	return cmsFloat64Number(v) / 652.800
}

func ab2float2(v cmsUInt16Number) cmsFloat64Number {
	return (cmsFloat64Number(v) / 256.0) - 128.0
}

// Lab value to fixed-point encoding (Version 2)
func L2Fix2(L cmsFloat64Number) cmsUInt16Number {
	return cmsQuickSaturateWord(L * 652.8)
}

func ab2Fix2(ab cmsFloat64Number) cmsUInt16Number {
	return cmsQuickSaturateWord((ab + 128.0) * 256.0)
}

// Lab value to float decoding (Version 4)
func L2float4(v cmsUInt16Number) cmsFloat64Number {
	return cmsFloat64Number(v) / 655.35
}

func ab2float4(v cmsUInt16Number) cmsFloat64Number {
	return (cmsFloat64Number(v) / 257.0) - 128.0
}

func cmsLabEncoded2FloatV2(Lab *cmsCIELab, wLab [3]cmsUInt16Number) {
	Lab.L = L2float2(wLab[0])
	Lab.a = ab2float2(wLab[1])
	Lab.b = ab2float2(wLab[2])
}

func cmsLabEncoded2Float(Lab *cmsCIELab, wLab [3]cmsUInt16Number) {
	Lab.L = L2float4(wLab[0])
	Lab.a = ab2float4(wLab[1])
	Lab.b = ab2float4(wLab[2])
}

// Lab Encoding and Decoding Utilities

// Clamp function for Lab L values (Version 2)
func Clamp_L_doubleV2(L cmsFloat64Number) cmsFloat64Number {
	LMax := (cmsFloat64Number(0xFFFF) * 100.0) / 0xFF00
	if L < 0 {
		return 0
	}
	if L > LMax {
		return LMax
	}
	return L
}

// Clamp function for Lab a/b values (Version 2)
func Clamp_ab_doubleV2(ab cmsFloat64Number) cmsFloat64Number {
	if ab < MIN_ENCODEABLE_ab2 {
		return MIN_ENCODEABLE_ab2
	}
	if ab > MAX_ENCODEABLE_ab2 {
		return MAX_ENCODEABLE_ab2
	}
	return ab
}

func cmsFloat2LabEncodedV2(wLab [3]cmsUInt16Number, fLab *cmsCIELab) {
	var Lab cmsCIELab

	Lab.L = Clamp_L_doubleV2(fLab.L)
	Lab.a = Clamp_ab_doubleV2(fLab.a)
	Lab.b = Clamp_ab_doubleV2(fLab.b)

	wLab[0] = L2Fix2(Lab.L)
	wLab[1] = ab2Fix2(Lab.a)
	wLab[2] = ab2Fix2(Lab.b)
}

// Lab encoding (Version 4)
func Clamp_L_doubleV4(L cmsFloat64Number) cmsFloat64Number {
	if L < 0 {
		return 0
	}
	if L > 100.0 {
		return 100.0
	}
	return L
}

func Clamp_ab_doubleV4(ab cmsFloat64Number) cmsFloat64Number {
	if ab < MIN_ENCODEABLE_ab4 {
		return MIN_ENCODEABLE_ab4
	}
	if ab > MAX_ENCODEABLE_ab4 {
		return MAX_ENCODEABLE_ab4
	}
	return ab
}

func L2Fix4(L cmsFloat64Number) cmsUInt16Number {
	return cmsQuickSaturateWord(L * 655.35)
}

func ab2Fix4(ab cmsFloat64Number) cmsUInt16Number {
	return cmsQuickSaturateWord((ab + 128.0) * 257.0)
}

func cmsFloat2LabEncoded(wLab [3]cmsUInt16Number, fLab *cmsCIELab) {
	var Lab cmsCIELab

	Lab.L = Clamp_L_doubleV4(fLab.L)
	Lab.a = Clamp_ab_doubleV4(fLab.a)
	Lab.b = Clamp_ab_doubleV4(fLab.b)

	wLab[0] = L2Fix4(Lab.L)
	wLab[1] = ab2Fix4(Lab.a)
	wLab[2] = ab2Fix4(Lab.b)
}

// Utility Functions
func RADIANS(deg cmsFloat64Number) cmsFloat64Number {
	return (deg * math.Pi) / 180.0
}

func atan2deg(a, b cmsFloat64Number) cmsFloat64Number {
	if a == 0 && b == 0 {
		return 0
	}
	h := cmsFloat64Number(math.Atan2(float64(a), float64(b)) * (180.0 / math.Pi))
	for h > 360.0 {
		h -= 360.0
	}
	for h < 0 {
		h += 360.0
	}
	return h
}

func Sqr(v cmsFloat64Number) cmsFloat64Number {
	return v * v
}

// Lab to LCh Conversion
func cmsLab2LCh(LCh *cmsCIELCh, Lab *cmsCIELab) {
	LCh.L = Lab.L
	LCh.C = cmsFloat64Number(math.Sqrt(float64(Sqr(Lab.a) + Sqr(Lab.b))))
	LCh.h = atan2deg(Lab.b, Lab.a)
}

// LCh to Lab Conversion
func cmsLCh2Lab(Lab *cmsCIELab, LCh *cmsCIELCh) {
	hRadians := RADIANS(LCh.h)
	Lab.L = LCh.L
	Lab.a = LCh.C * cmsFloat64Number(math.Cos(float64(hRadians)))
	Lab.b = LCh.C * cmsFloat64Number(math.Sin(float64(hRadians)))
}

// XYZ Encoding and Decoding
func XYZ2Fix(d cmsFloat64Number) cmsUInt16Number {
	return cmsQuickSaturateWord(d * 32768.0)
}

func cmsFloat2XYZEncoded(XYZ [3]cmsUInt16Number, fXYZ *cmsCIEXYZ) {
	var xyz cmsCIEXYZ
	xyz.X, xyz.Y, xyz.Z = fXYZ.X, fXYZ.Y, fXYZ.Z

	// Clamp to encodable values
	if xyz.Y <= 0 {
		xyz.X, xyz.Y, xyz.Z = 0, 0, 0
	}
	xyz.X = cmsFloat64Number(math.Min(math.Max(0, float64(xyz.X)), MAX_ENCODEABLE_XYZ))
	xyz.Y = cmsFloat64Number(math.Min(math.Max(0, float64(xyz.Y)), MAX_ENCODEABLE_XYZ))
	xyz.Z = cmsFloat64Number(math.Min(math.Max(0, float64(xyz.Z)), MAX_ENCODEABLE_XYZ))

	XYZ[0] = XYZ2Fix(xyz.X)
	XYZ[1] = XYZ2Fix(xyz.Y)
	XYZ[2] = XYZ2Fix(xyz.Z)
}

func XYZ2Float(v cmsUInt16Number) cmsFloat64Number {
	return cmsFloat64Number(v) / 32768.0
}

func cmsXYZEncoded2Float(fXYZ *cmsCIEXYZ, XYZ [3]cmsUInt16Number) {
	fXYZ.X = XYZ2Float(XYZ[0])
	fXYZ.Y = XYZ2Float(XYZ[1])
	fXYZ.Z = XYZ2Float(XYZ[2])
}

// Delta-E Calculations

// Standard Delta-E
func cmsDeltaE(Lab1, Lab2 *cmsCIELab) cmsFloat64Number {
	dL := Lab1.L - Lab2.L
	da := Lab1.a - Lab2.a
	db := Lab1.b - Lab2.b
	return cmsFloat64Number(math.Sqrt(float64(Sqr(dL) + Sqr(da) + Sqr(db))))
}

// CIE94 Delta-E
func cmsCIE94DeltaE(Lab1, Lab2 *cmsCIELab) cmsFloat64Number {
	var LCh1, LCh2 cmsCIELCh

	dL := cmsFloat64Number(math.Abs(float64(Lab1.L - Lab2.L)))
	dC := cmsFloat64Number(math.Abs(float64(LCh1.C - LCh2.C)))
	cmsLab2LCh(&LCh1, Lab1)
	cmsLab2LCh(&LCh2, Lab2)
	dE := cmsDeltaE(Lab1, Lab2)

	dhsq := Sqr(dE) - Sqr(dL) - Sqr(dC)
	var dh cmsFloat64Number
	if dhsq > 0 {
		dh = cmsFloat64Number(math.Sqrt(float64(dhsq)))
	}

	c12 := math.Sqrt(float64(LCh1.C * LCh2.C))
	sc := cmsFloat64Number(1.0 + (0.048 * c12))
	sh := cmsFloat64Number(1.0 + (0.014 * c12))

	return cmsFloat64Number(math.Sqrt(float64(Sqr(dL) + Sqr(dC)/Sqr(sc) + Sqr(dh)/Sqr(sh))))
}

// CMC Delta-E
func CMCdeltaE(Lab1, Lab2 *cmsCIELab, l, c cmsFloat64Number) cmsFloat64Number {
	if Lab1.L == 0 && Lab2.L == 0 {
		return 0
	}

	var LCh1, LCh2 cmsCIELCh
	cmsLab2LCh(&LCh1, Lab1)
	cmsLab2LCh(&LCh2, Lab2)

	dL := Lab2.L - Lab1.L
	dC := LCh2.C - LCh1.C
	dE := cmsDeltaE(Lab1, Lab2)

	var dh float64
	if Sqr(dE) > (Sqr(dL) + Sqr(dC)) {
		dh = math.Sqrt(float64(Sqr(dE) - Sqr(dL) - Sqr(dC)))
	} else {
		dh = 0
	}

	var t cmsFloat64Number
	if LCh1.h > 164 && LCh1.h < 345 {
		t = cmsFloat64Number(0.56 + math.Abs(0.2*math.Cos(float64(RADIANS(LCh1.h+168)))))
	} else {
		t = cmsFloat64Number(0.36 + math.Abs(0.4*math.Cos(float64(RADIANS(LCh1.h+35)))))
	}

	sc := cmsFloat64Number(0.0638*LCh1.C/(1+0.0131*LCh1.C) + 0.638)
	sl := cmsFloat64Number(0.040975 * Lab1.L / (1 + 0.01765*Lab1.L))

	if Lab1.L < 16 {
		sl = 0.511
	}

	f := cmsFloat64Number(math.Sqrt(float64(Sqr(LCh1.C) * Sqr(LCh1.C) / (Sqr(LCh1.C)*Sqr(LCh1.C) + 1900))))
	sh := cmsFloat64Number(sc * (t*f + 1 - f))

	return cmsFloat64Number(math.Sqrt(Sqr(dL/(l*sl)) + Sqr(dC/(c*sc)) + Sqr(dh/sh)))
}

// CIE2000 Delta-E
func CIE2000DeltaE(Lab1, Lab2 *CIELab, Kl, Kc, Kh float64) float64 {
	L1, a1, b1 := Lab1.L, Lab1.a, Lab1.b
	C1 := math.Sqrt(sqr(a1) + sqr(b1))

	L2, a2, b2 := Lab2.L, Lab2.a, Lab2.b
	C2 := math.Sqrt(sqr(a2) + sqr(b2))

	meanC := (C1 + C2) / 2.0
	G := 0.5 * (1 - math.Sqrt(math.Pow(meanC, 7)/(math.Pow(meanC, 7)+math.Pow(25.0, 7))))

	a1Prime := (1 + G) * a1
	C1Prime := math.Sqrt(sqr(a1Prime) + sqr(b1))
	h1Prime := atan2deg(b1, a1Prime)

	a2Prime := (1 + G) * a2
	C2Prime := math.Sqrt(sqr(a2Prime) + sqr(b2))
	h2Prime := atan2deg(b2, a2Prime)

	meanCPrime := (C1Prime + C2Prime) / 2.0
	meanHPrime := 0.0
	if math.Abs(h1Prime-h2Prime) > 180.0 {
		if h1Prime+h2Prime < 360.0 {
			meanHPrime = (h1Prime + h2Prime + 360.0) / 2.0
		} else {
			meanHPrime = (h1Prime + h2Prime - 360.0) / 2.0
		}
	} else {
		meanHPrime = (h1Prime + h2Prime) / 2.0
	}

	deltaLPrime := L2 - L1
	deltaCPrime := C2Prime - C1Prime

	deltaHPrime := 0.0
	if math.Abs(h2Prime-h1Prime) > 180.0 {
		if h2Prime > h1Prime {
			deltaHPrime = (h2Prime - h1Prime - 360.0)
		} else {
			deltaHPrime = (h2Prime - h1Prime + 360.0)
		}
	} else {
		deltaHPrime = h2Prime - h1Prime
	}

	deltaH := 2.0 * math.Sqrt(C1Prime*C2Prime) * math.Sin(radians(deltaHPrime/2.0))
	Sl := 1 + (0.015*sqr((L1+L2)/2.0-50.0))/math.Sqrt(20.0+sqr((L1+L2)/2.0-50.0))
	Sc := 1 + 0.045*meanCPrime
	T := 1 - 0.17*math.Cos(radians(meanHPrime-30.0)) +
		0.24*math.Cos(radians(2.0*meanHPrime)) +
		0.32*math.Cos(radians(3.0*meanHPrime+6.0)) -
		0.20*math.Cos(radians(4.0*meanHPrime-63.0))
	Sh := 1 + 0.015*meanCPrime*T
	deltaTheta := 30.0 * math.Exp(-sqr((meanHPrime-275.0)/25.0))
	Rc := 2.0 * math.Sqrt(math.Pow(meanCPrime, 7.0)/(math.Pow(meanCPrime, 7.0)+math.Pow(25.0, 7.0)))
	Rt := -math.Sin(radians(2.0*deltaTheta)) * Rc

	return math.Sqrt(
		sqr(deltaLPrime/(Sl*Kl)) +
			sqr(deltaCPrime/(Sc*Kc)) +
			sqr(deltaH/(Sh*Kh)) +
			Rt*(deltaCPrime/(Sc*Kc))*(deltaH/(Sh*Kh)),
	)
}

// Gridpoints calculation based on color space
func ReasonableGridpointsByColorspace(Colorspace cmsColorSpaceSignature, Flags uint32) uint32 {
	if Flags&0x00FF0000 != 0 {
		return (Flags >> 16) & 0xFF
	}

	nChannels := ChannelsOf(Colorspace)

	if Flags&cmsFLAGS_HIGHRESPRECALC != 0 {
		if nChannels > 4 {
			return 7
		}
		if nChannels == 4 {
			return 23
		}
		return 49
	}

	if Flags&cmsFLAGS_LOWRESPRECALC != 0 {
		if nChannels > 4 {
			return 6
		}
		if nChannels == 1 {
			return 33
		}
		return 17
	}

	if nChannels > 4 {
		return 7
	}
	if nChannels == 4 {
		return 17
	}
	return 33
}

// Translate colorspace signature to ICC representation
func ICCcolorSpace(OurNotation int) cmsColorSpaceSignature {
	switch OurNotation {
	case PT_GRAY:
		return cmsSigGrayData
	case PT_RGB:
		return cmsSigRgbData
	case PT_CMY:
		return cmsSigCmyData
	case PT_CMYK:
		return cmsSigCmykData
	case PT_XYZ:
		return cmsSigXYZData
	case PT_Lab:
		return cmsSigLabData
	default:
		return cmsColorSpaceSignature(0)
	}
}

// Endpoints by color space
func EndPointsBySpace(Space cmsColorSpaceSignature) (White, Black []uint16, nOutputs uint32, ok bool) {
	var (
		RGBblack  = []uint16{0, 0, 0}
		RGBwhite  = []uint16{0xffff, 0xffff, 0xffff}
		CMYKblack = []uint16{0xffff, 0xffff, 0xffff, 0xffff}
		CMYKwhite = []uint16{0, 0, 0, 0}
		LABblack  = []uint16{0, 0x8080, 0x8080} // V4 Lab encoding
		LABwhite  = []uint16{0xffff, 0x8080, 0x8080}
		CMYblack  = []uint16{0xffff, 0xffff, 0xffff}
		CMYwhite  = []uint16{0, 0, 0}
		Grayblack = []uint16{0}
		Graywhite = []uint16{0xffff}
	)

	switch Space {
	case cmsSigGrayData:
		return Graywhite, Grayblack, 1, true
	case cmsSigRgbData:
		return RGBwhite, RGBblack, 3, true
	case cmsSigLabData:
		return LABwhite, LABblack, 3, true
	case cmsSigCmykData:
		return CMYKwhite, CMYKblack, 4, true
	case cmsSigCmyData:
		return CMYwhite, CMYblack, 3, true
	default:
		return nil, nil, 0, false
	}
}

// Translate from internal color space to ICC representation
func ICCcolorSpaceFromInternal(OurNotation int) cmsColorSpaceSignature {
	switch OurNotation {
	case PT_GRAY:
		return cmsSigGrayData
	case PT_RGB:
		return cmsSigRgbData
	case PT_CMY:
		return cmsSigCmyData
	case PT_CMYK:
		return cmsSigCmykData
	case PT_XYZ:
		return cmsSigXYZData
	case PT_Lab:
		return cmsSigLabData
	case PT_YCbCr:
		return cmsSigYCbCrData
	case PT_HSV:
		return cmsSigHsvData
	case PT_HLS:
		return cmsSigHlsData
	case PT_Yxy:
		return cmsSigYxyData
	default:
		return cmsColorSpaceSignature(0)
	}
}

// Translate from ICC representation to internal color space
func LCMSColorSpace(ProfileSpace cmsColorSpaceSignature) int {
	switch ProfileSpace {
	case cmsSigGrayData:
		return PT_GRAY
	case cmsSigRgbData:
		return PT_RGB
	case cmsSigCmyData:
		return PT_CMY
	case cmsSigCmykData:
		return PT_CMYK
	case cmsSigXYZData:
		return PT_XYZ
	case cmsSigLabData:
		return PT_Lab
	case cmsSigYCbCrData:
		return PT_YCbCr
	case cmsSigHsvData:
		return PT_HSV
	case cmsSigHlsData:
		return PT_HLS
	case cmsSigYxyData:
		return PT_Yxy
	default:
		return 0
	}
}

// Get the number of channels in a color space
func ChannelsOfColorSpace(ColorSpace cmsColorSpaceSignature) int {
	switch ColorSpace {
	case cmsSigGrayData, cmsSig1colorData, cmsSigMCH1Data:
		return 1
	case cmsSig2colorData, cmsSigMCH2Data:
		return 2
	case cmsSigRgbData, cmsSigLabData, cmsSigXYZData, cmsSigYCbCrData, cmsSigYxyData, cmsSigHsvData, cmsSigHlsData, cmsSigCmyData, cmsSig3colorData, cmsSigMCH3Data:
		return 3
	case cmsSigCmykData, cmsSig4colorData, cmsSigMCH4Data:
		return 4
	case cmsSig5colorData, cmsSigMCH5Data:
		return 5
	case cmsSig6colorData, cmsSigMCH6Data:
		return 6
	case cmsSig7colorData, cmsSigMCH7Data:
		return 7
	case cmsSig8colorData, cmsSigMCH8Data:
		return 8
	case cmsSig9colorData, cmsSigMCH9Data:
		return 9
	case cmsSig10colorData, cmsSigMCHAData:
		return 10
	case cmsSig11colorData, cmsSigMCHBData:
		return 11
	case cmsSig12colorData, cmsSigMCHCData:
		return 12
	case cmsSig13colorData, cmsSigMCHDData:
		return 13
	case cmsSig14colorData, cmsSigMCHEData:
		return 14
	case cmsSig15colorData, cmsSigMCHFData:
		return 15
	default:
		return -1
	}
}

// Deprecated function for getting the number of channels
func ChannelsOf(ColorSpace cmsColorSpaceSignature) uint32 {
	n := ChannelsOfColorSpace(ColorSpace)
	if n < 0 {
		return 3
	}
	return uint32(n)
}

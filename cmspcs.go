package golcms

import (
	"math"
)

//      inter PCS conversions XYZ <. CIE L* a* b*
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
CIE XYZ             X             0 . 1.99997        0x0000 . 0xffff
CIE XYZ             Y             0 . 1.99997        0x0000 . 0xffff
CIE XYZ             Z             0 . 1.99997        0x0000 . 0xffff

Version 2,3
-----------

CIELAB (16 bit)     L*            0 . 100.0          0x0000 . 0xff00
CIELAB (16 bit)     a*            -128.0 . +127.996  0x0000 . 0x8000 . 0xffff
CIELAB (16 bit)     b*            -128.0 . +127.996  0x0000 . 0x8000 . 0xffff


Version 4
---------

CIELAB (16 bit)     L*            0 . 100.0          0x0000 . 0xffff
CIELAB (16 bit)     a*            -128.0 . +127      0x0000 . 0x8080 . 0xffff
CIELAB (16 bit)     b*            -128.0 . +127      0x0000 . 0x8080 . 0xffff

*/

// Conversions
func cmsXYZ2xyY(dest *CmsCIExyY, source *cmsCIEXYZ) {
	sum := 1.0 / (source.X + source.Y + source.Z)
	dest.X_small = source.X * sum
	dest.Y_small = source.Y * sum
	dest.Y_large = source.Z
}

func cmsxyY2XYZ(dest *cmsCIEXYZ, source *CmsCIExyY) {
	dest.X = (source.X_small / source.Y_small) * source.Y_large
	dest.Y = source.Y_large
	dest.Z = ((1 - source.X_small - source.Y_small) / source.Y_small) * source.Y_large
}

/*
   The break point (24/116)^3 = (6/29)^3 is a very small amount of tristimulus
   primary (0.008856).  Generally, this only happens for
   nearly ideal blacks and for some orange / amber colors in transmission mode.
   For example, the Z value of the orange turn indicator lamp lens on an
   automobile will often be below this value.  But the Z does not
   contribute to the perceived color directly.
*/
// Precomputed constants for Lab/XYZ conversion.
// 6/29 = 24/116
const (
	labBreak      = (24.0 / 116.0) * (24.0 / 116.0) * (24.0 / 116.0) // (6/29)^3
	labInvBreak   = 24.0 / 116.0                                     // 6/29
	labLinearK    = 841.0 / 108.0
	labLinearBias = 16.0 / 116.0
)

// f(t) used in XYZ -> Lab
func f(t float64) float64 {
	if t <= labBreak {
		return labLinearK*t + labLinearBias
	}
	return math.Cbrt(t)
}

// f⁻¹(t) used in Lab -> XYZ
func f_1(t float64) float64 {
	if t <= labInvBreak {
		return (108.0 / 841.0) * (t - labLinearBias)
	}
	// t^3 is cheaper than math.Pow(t, 3)
	return t * t * t
}

/*// f(t) function used in Lab/XYZ conversions
func f(t float64) float64 {
	limit := math.Pow(24.0/116.0, 3)
	if t <= limit {
		return (841.0/108.0)*t + (16.0 / 116.0)
	}
	return math.Cbrt(t)
}

// Inverse of f(t)
func f_1(t float64) float64 {
	limit := 24.0 / 116.0
	if t <= limit {
		return (108.0 / 841.0) * (t - (16.0 / 116.0))
	}
	return math.Pow(t, 3)
}*/

// Standard XYZ to Lab. it can handle negative XZY numbers in some cases
func cmsXYZ2Lab(whitePoint *cmsCIEXYZ, lab *cmsCIELab, xyz *cmsCIEXYZ) {
	if whitePoint == nil {
		whitePoint = cmsD50_XYZ()
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
		whitePoint = cmsD50_XYZ()
	}

	y := (lab.L + 16.0) / 116.0
	x := y + 0.002*lab.a
	z := y - 0.005*lab.b

	xyz.X = f_1(x) * whitePoint.X
	xyz.Y = f_1(y) * whitePoint.Y
	xyz.Z = f_1(z) * whitePoint.Z
}

// Helper functions to convert Lab values to float and back
//
//lint:ignore U1000 kept for parity with lcms; used in future ports
func L2float2(v uint16) float64 {
	return float64(v) / 652.800
}

//lint:ignore U1000 kept for parity with lcms; used in future ports
func ab2float2(v uint16) float64 {
	return float64(v)/256.0 - 128.0
}

// Lab value to fixed-point encoding (Version 2)
func L2Fix2(L float64) uint16 {
	return cmsQuickSaturateWord(L * 652.8)
}

//lint:ignore U1000 kept for parity with lcms; used in future ports
func ab2Fix2(ab float64) uint16 {
	return cmsQuickSaturateWord((ab + 128.0) * 256.0)
}

// Lab value to float decoding (Version 4)
func L2float4(v uint16) float64 {
	return float64(v) / 655.35
}

func ab2float4(v uint16) float64 {
	return (float64(v) / 257.0) - 128.0
}

func cmsLabEncoded2FloatV2(Lab *cmsCIELab, wLab *[3]uint16) {
	Lab.L = L2float2(wLab[0])
	Lab.a = ab2float2(wLab[1])
	Lab.b = ab2float2(wLab[2])
}

func cmsLabEncoded2Float(Lab *cmsCIELab, wLab *[3]uint16) {
	Lab.L = L2float4(wLab[0])
	Lab.a = ab2float4(wLab[1])
	Lab.b = ab2float4(wLab[2])
}

// Lab Encoding and Decoding Utilities

// Clamp function for Lab L values (Version 2)
func Clamp_L_doubleV2(L float64) float64 {
	LMax := (float64(0xFFFF) * 100.0) / 0xFF00
	if L < 0 {
		return 0
	}
	if L > LMax {
		return LMax
	}
	return L
}

// Clamp function for Lab a/b values (Version 2)
func Clamp_ab_doubleV2(ab float64) float64 {
	if ab < MIN_ENCODEABLE_ab2 {
		return MIN_ENCODEABLE_ab2
	}
	if ab > MAX_ENCODEABLE_ab2 {
		return MAX_ENCODEABLE_ab2
	}
	return ab
}

func cmsFloat2LabEncodedV2(wLab *[3]uint16, fLab *cmsCIELab) {
	var Lab cmsCIELab

	Lab.L = Clamp_L_doubleV2(fLab.L)
	Lab.a = Clamp_ab_doubleV2(fLab.a)
	Lab.b = Clamp_ab_doubleV2(fLab.b)

	wLab[0] = L2Fix2(Lab.L)
	wLab[1] = ab2Fix2(Lab.a)
	wLab[2] = ab2Fix2(Lab.b)
}

// Lab encoding (Version 4)
func Clamp_L_doubleV4(L float64) float64 {
	if L < 0 {
		return 0
	}
	if L > 100.0 {
		return 100.0
	}
	return L
}

func Clamp_ab_doubleV4(ab float64) float64 {
	if ab < MIN_ENCODEABLE_ab4 {
		return MIN_ENCODEABLE_ab4
	}
	if ab > MAX_ENCODEABLE_ab4 {
		return MAX_ENCODEABLE_ab4
	}
	return ab
}

func L2Fix4(L float64) uint16 {
	return cmsQuickSaturateWord(L * 655.35)
}

func ab2Fix4(ab float64) uint16 {
	return cmsQuickSaturateWord((ab + 128.0) * 257.0)
}

func cmsFloat2LabEncoded(wLab []uint16, fLab *cmsCIELab) {
	var Lab cmsCIELab

	Lab.L = Clamp_L_doubleV4(fLab.L)
	Lab.a = Clamp_ab_doubleV4(fLab.a)
	Lab.b = Clamp_ab_doubleV4(fLab.b)

	wLab[0] = L2Fix4(Lab.L)
	wLab[1] = ab2Fix4(Lab.a)
	wLab[2] = ab2Fix4(Lab.b)
}

// Utility Functions
func RADIANS(deg float64) float64 {
	return (deg * math.Pi) / 180.0
}

func atan2deg(a, b float64) float64 {
	if a == 0 && b == 0 {
		return 0
	}
	h := math.Atan2(a, b) * (180.0 / math.Pi)
	for h > 360.0 {
		h -= 360.0
	}
	for h < 0 {
		h += 360.0
	}
	return h
}

func Sqr(v float64) float64 {
	return v * v
}

// Lab to LCh Conversion
func cmsLab2LCh(LCh *cmsCIELCh, Lab *cmsCIELab) {
	LCh.L = Lab.L
	LCh.C = math.Sqrt(Sqr(Lab.a) + Sqr(Lab.b))
	LCh.h = atan2deg(Lab.b, Lab.a)
}

// LCh to Lab Conversion
func cmsLCh2Lab(Lab *cmsCIELab, LCh *cmsCIELCh) {
	hRadians := RADIANS(LCh.h)
	Lab.L = LCh.L
	Lab.a = LCh.C * math.Cos(hRadians)
	Lab.b = LCh.C * math.Sin(hRadians)
}

// XYZ Encoding and Decoding
func XYZ2Fix(d float64) uint16 {
	return cmsQuickSaturateWord(d * 32768.0)
}

func cmsFloat2XYZEncoded(XYZ *[3]uint16, fXYZ *cmsCIEXYZ) {
	var xyz cmsCIEXYZ
	xyz.X, xyz.Y, xyz.Z = fXYZ.X, fXYZ.Y, fXYZ.Z

	// Clamp to encodable values
	if xyz.Y <= 0 {
		xyz.X, xyz.Y, xyz.Z = 0, 0, 0
	}
	xyz.X = math.Min(math.Max(0, xyz.X), MAX_ENCODEABLE_XYZ)
	xyz.Y = math.Min(math.Max(0, xyz.Y), MAX_ENCODEABLE_XYZ)
	xyz.Z = math.Min(math.Max(0, xyz.Z), MAX_ENCODEABLE_XYZ)

	XYZ[0] = XYZ2Fix(xyz.X)
	XYZ[1] = XYZ2Fix(xyz.Y)
	XYZ[2] = XYZ2Fix(xyz.Z)
}

func XYZ2Float(v uint16) float64 {
	return float64(v) / 32768.0
}

func cmsXYZEncoded2Float(fXYZ *cmsCIEXYZ, XYZ *[3]uint16) {
	fXYZ.X = XYZ2Float(XYZ[0])
	fXYZ.Y = XYZ2Float(XYZ[1])
	fXYZ.Z = XYZ2Float(XYZ[2])
}

// Delta-E Calculations

// Standard Delta-E
func cmsDeltaE(Lab1, Lab2 *cmsCIELab) float64 {
	dL := Lab1.L - Lab2.L
	da := Lab1.a - Lab2.a
	db := Lab1.b - Lab2.b
	return math.Sqrt(Sqr(dL) + Sqr(da) + Sqr(db))
}

// CIE94 Delta-E
func cmsCIE94DeltaE(Lab1, Lab2 *cmsCIELab) float64 {
	var LCh1, LCh2 cmsCIELCh

	dL := math.Abs(Lab1.L - Lab2.L)
	dC := math.Abs(LCh1.C - LCh2.C)
	cmsLab2LCh(&LCh1, Lab1)
	cmsLab2LCh(&LCh2, Lab2)
	dE := cmsDeltaE(Lab1, Lab2)

	dhsq := Sqr(dE) - Sqr(dL) - Sqr(dC)
	var dh float64
	if dhsq > 0 {
		dh = math.Sqrt(dhsq)
	}

	c12 := math.Sqrt(LCh1.C * LCh2.C)
	sc := 1.0 + (0.048 * c12)
	sh := 1.0 + (0.014 * c12)

	return math.Sqrt(Sqr(dL) + Sqr(dC)/Sqr(sc) + Sqr(dh)/Sqr(sh))
}

// Auxiliary
func ComputeLBFD(Lab *cmsCIELab) float64 {
	var yt float64

	if Lab.L > 7.996969 {
		yt = (Sqr((Lab.L+16)/116) * ((Lab.L + 16) / 116)) * 100
	} else {
		yt = 100 * (Lab.L / 903.3)
	}
	return 54.6*(math.Log10E*math.Log(yt+1.5)) - 9.6
}

// bfd - gets BFD(1:1) difference between Lab1, Lab2
func cmsBFDdeltaE(Lab1 *cmsCIELab, Lab2 *cmsCIELab) float64 {
	var lbfd1, lbfd2, AveC, Aveh, dE, deltaL, deltaC, deltah, dc, t, g, dh, rh, rc, rt, bfd float64
	var LCh1, LCh2 cmsCIELCh

	lbfd1 = ComputeLBFD(Lab1)
	lbfd2 = ComputeLBFD(Lab2)
	deltaL = lbfd2 - lbfd1

	cmsLab2LCh(&LCh1, Lab1)
	cmsLab2LCh(&LCh2, Lab2)

	deltaC = LCh2.C - LCh1.C
	AveC = (LCh1.C + LCh2.C) / 2
	Aveh = (LCh1.h + LCh2.h) / 2

	dE = cmsDeltaE(Lab1, Lab2)

	if Sqr(dE) > (Sqr(Lab2.L-Lab1.L) + Sqr(deltaC)) {
		deltah = math.Sqrt(Sqr(dE) - Sqr(Lab2.L-Lab1.L) - Sqr(deltaC))
	} else {
		deltah = 0
	}

	dc = 0.035*AveC/(1+0.00365*AveC) + 0.521
	g = math.Sqrt(Sqr(Sqr(AveC)) / (Sqr(Sqr(AveC)) + 14000))
	t = 0.627 + (0.055*math.Cos((Aveh-254)/(180/math.Pi)) -
		0.040*math.Cos((2*Aveh-136)/(180/math.Pi)) +
		0.070*math.Cos((3*Aveh-31)/(180/math.Pi)) +
		0.049*math.Cos((4*Aveh+114)/(180/math.Pi)) -
		0.015*math.Cos((5*Aveh-103)/(180/math.Pi)))

	dh = dc * (g*t + 1 - g)
	rh = -0.260*math.Cos((Aveh-308)/(180/math.Pi)) -
		0.379*math.Cos((2*Aveh-160)/(180/math.Pi)) -
		0.636*math.Cos((3*Aveh+254)/(180/math.Pi)) +
		0.226*math.Cos((4*Aveh+140)/(180/math.Pi)) -
		0.194*math.Cos((5*Aveh+280)/(180/math.Pi))

	rc = math.Sqrt((AveC * AveC * AveC * AveC * AveC * AveC) / ((AveC * AveC * AveC * AveC * AveC * AveC) + 70000000))
	rt = rh * rc

	bfd = math.Sqrt(Sqr(deltaL) + Sqr(deltaC/dc) + Sqr(deltah/dh) + (rt * (deltaC / dc) * (deltah / dh)))

	return bfd
}

// cmc - CMC(l:c) difference between Lab1, Lab2
func cmsCMCdeltaE(Lab1, Lab2 *cmsCIELab, l, c float64) float64 {
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
		dh = math.Sqrt(Sqr(dE) - Sqr(dL) - Sqr(dC))
	} else {
		dh = 0
	}

	var t float64
	if LCh1.h > 164 && LCh1.h < 345 {
		t = 0.56 + math.Abs(0.2*math.Cos(RADIANS(LCh1.h+168)))
	} else {
		t = 0.36 + math.Abs(0.4*math.Cos(RADIANS(LCh1.h+35)))
	}

	sc := 0.0638*LCh1.C/(1+0.0131*LCh1.C) + 0.638
	sl := 0.040975 * Lab1.L / (1 + 0.01765*Lab1.L)

	if Lab1.L < 16 {
		sl = 0.511
	}

	f := math.Sqrt(Sqr(LCh1.C) * Sqr(LCh1.C) / (Sqr(LCh1.C)*Sqr(LCh1.C) + 1900))
	sh := sc * (t*f + 1 - f)

	return math.Sqrt(Sqr(dL/(l*sl)) + Sqr(dC/(c*sc)) + Sqr(dh/sh))
}

// CIE2000 Delta-E
func CIE2000DeltaE(Lab1, Lab2 *cmsCIELab, Kl, Kc, Kh float64) float64 {
	L1, a1, b1 := Lab1.L, Lab1.a, Lab1.b
	C1 := math.Sqrt(Sqr(a1) + Sqr(b1))

	L2, a2, b2 := Lab2.L, Lab2.a, Lab2.b
	C2 := math.Sqrt(Sqr(a2) + Sqr(b2))

	meanC := (C1 + C2) / 2.0
	G := 0.5 * (1 - math.Sqrt(math.Pow(meanC, 7)/(math.Pow(meanC, 7)+math.Pow(25.0, 7))))

	a1Prime := (1 + G) * a1
	C1Prime := math.Sqrt(Sqr(a1Prime) + Sqr(b1))
	h1Prime := atan2deg(b1, a1Prime)

	a2Prime := (1 + G) * a2
	C2Prime := math.Sqrt(Sqr(a2Prime) + Sqr(b2))
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

	deltaH := 2.0 * math.Sqrt(C1Prime*C2Prime) * math.Sin(RADIANS(deltaHPrime/2.0))
	Sl := 1 + (0.015*Sqr((L1+L2)/2.0-50.0))/math.Sqrt(20.0+Sqr((L1+L2)/2.0-50.0))
	Sc := 1 + 0.045*meanCPrime
	T := 1 - 0.17*math.Cos(RADIANS(meanHPrime-30.0)) +
		0.24*math.Cos(RADIANS(2.0*meanHPrime)) +
		0.32*math.Cos(RADIANS(3.0*meanHPrime+6.0)) -
		0.20*math.Cos(RADIANS(4.0*meanHPrime-63.0))
	Sh := 1 + 0.015*meanCPrime*T
	deltaTheta := 30.0 * math.Exp(-Sqr((meanHPrime-275.0)/25.0))
	Rc := 2.0 * math.Sqrt(math.Pow(meanCPrime, 7.0)/(math.Pow(meanCPrime, 7.0)+math.Pow(25.0, 7.0)))
	Rt := -math.Sin(RADIANS(2.0*deltaTheta)) * Rc

	return math.Sqrt(
		Sqr(deltaLPrime/(Sl*Kl)) +
			Sqr(deltaCPrime/(Sc*Kc)) +
			Sqr(deltaH/(Sh*Kh)) +
			Rt*(deltaCPrime/(Sc*Kc))*(deltaH/(Sh*Kh)),
	)
}

// Gridpoints calculation based on color space
func cmsReasonableGridpointsByColorspace(Colorspace cmsColorSpaceSignature, Flags uint32) uint32 {
	if Flags&0x00FF0000 != 0 {
		return (Flags >> 16) & 0xFF
	}

	nChannels := cmsChannelsOf(Colorspace)

	if Flags&CmsFLAGS_HIGHRESPRECALC != 0 {
		if nChannels > 4 {
			return 7
		}
		if nChannels == 4 {
			return 23
		}
		return 49
	}

	if Flags&CmsFLAGS_LOWRESPRECALC != 0 {
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

// Predefined arrays for common spaces

// _cmsEndPointsBySpace retrieves endpoints by color space
/*func cmsEndPointsBySpace(
	space cmsColorSpaceSignature,
	white **uint16,
	black **uint16,
	nOutputs *uint32,
) bool {
	var (
		RGBblack  = []uint16{0, 0, 0}
		RGBwhite  = []uint16{0xffff, 0xffff, 0xffff}
		CMYKblack = []uint16{0xffff, 0xffff, 0xffff, 0xffff} // 400% of ink
		CMYKwhite = []uint16{0, 0, 0, 0}
		LABblack  = []uint16{0, 0x8080, 0x8080} // V4 Lab encoding
		LABwhite  = []uint16{0xffff, 0x8080, 0x8080}
		CMYblack  = []uint16{0xffff, 0xffff, 0xffff}
		CMYwhite  = []uint16{0, 0, 0}
		Grayblack = []uint16{0}
		GrayWhite = []uint16{0xffff}
	)

	switch space {
	case CmsSigGrayData:
		if white != nil {
			*white = &GrayWhite[0]
		}
		if black != nil {
			*black = &Grayblack[0]
		}
		if nOutputs != nil {
			*nOutputs = 1
		}
		return true

	case CmsSigRgbData:
		if white != nil {
			*white = &RGBwhite[0]
		}
		if black != nil {
			*black = &RGBblack[0]
		}
		if nOutputs != nil {
			*nOutputs = 3
		}
		return true

	case CmsSigLabData:
		if white != nil {
			*white = &LABwhite[0]
		}
		if black != nil {
			*black = &LABblack[0]
		}
		if nOutputs != nil {
			*nOutputs = 3
		}
		return true

	case CmsSigCmykData:
		if white != nil {
			*white = &CMYKwhite[0]
		}
		if black != nil {
			*black = &CMYKblack[0]
		}
		if nOutputs != nil {
			*nOutputs = 4
		}
		return true

	case CmsSigCmyData:
		if white != nil {
			*white = &CMYwhite[0]
		}
		if black != nil {
			*black = &CMYblack[0]
		}
		if nOutputs != nil {
			*nOutputs = 3
		}
		return true

	default:
		return false
	}
}*/

func cmsEndPointsBySpace(
	space cmsColorSpaceSignature,
	white *[]uint16,
	black *[]uint16,
	nOutputs *uint32,
) bool {
	var (
		RGBblack  = []uint16{0, 0, 0}
		RGBwhite  = []uint16{0xffff, 0xffff, 0xffff}
		CMYKblack = []uint16{0xffff, 0xffff, 0xffff, 0xffff} // 400% of ink
		CMYKwhite = []uint16{0, 0, 0, 0}
		LABblack  = []uint16{0, 0x8080, 0x8080} // V4 Lab encoding
		LABwhite  = []uint16{0xffff, 0x8080, 0x8080}
		CMYblack  = []uint16{0xffff, 0xffff, 0xffff}
		CMYwhite  = []uint16{0, 0, 0}
		Grayblack = []uint16{0}
		GrayWhite = []uint16{0xffff}
	)

	switch space {
	case CmsSigGrayData:
		if white != nil {
			*white = GrayWhite
		}
		if black != nil {
			*black = Grayblack
		}
		if nOutputs != nil {
			*nOutputs = 1
		}
		return true

	case CmsSigRgbData:
		if white != nil {
			*white = RGBwhite
		}
		if black != nil {
			*black = RGBblack
		}
		if nOutputs != nil {
			*nOutputs = 3
		}
		return true

	case CmsSigLabData:
		if white != nil {
			*white = LABwhite
		}
		if black != nil {
			*black = LABblack
		}
		if nOutputs != nil {
			*nOutputs = 3
		}
		return true

	case CmsSigCmykData:
		if white != nil {
			*white = CMYKwhite
		}
		if black != nil {
			*black = CMYKblack
		}
		if nOutputs != nil {
			*nOutputs = 4
		}
		return true

	case CmsSigCmyData:
		if white != nil {
			*white = CMYwhite
		}
		if black != nil {
			*black = CMYblack
		}
		if nOutputs != nil {
			*nOutputs = 3
		}
		return true

	default:
		return false
	}
}

// Translate from our colorspace to ICC representation.
func cmsICCcolorSpace(ourNotation int) cmsColorSpaceSignature {
	switch ourNotation {
	case 1, PT_GRAY:
		return CmsSigGrayData
	case 2, PT_RGB:
		return CmsSigRgbData
	case PT_CMY:
		return CmsSigCmyData
	case PT_CMYK:
		return CmsSigCmykData
	case PT_YCbCr:
		return CmsSigYCbCrData
	case PT_YUV:
		return CmsSigLuvData
	case PT_XYZ:
		return CmsSigXYZData
	case PT_LabV2, PT_Lab:
		return CmsSigLabData
	case PT_YUVK:
		return CmsSigLuvKData
	case PT_HSV:
		return CmsSigHsvData
	case PT_HLS:
		return CmsSigHlsData
	case PT_Yxy:
		return CmsSigYxyData
	case PT_MCH1:
		return CmsSigMCH1Data
	case PT_MCH2:
		return CmsSigMCH2Data
	case PT_MCH3:
		return CmsSigMCH3Data
	case PT_MCH4:
		return CmsSigMCH4Data
	case PT_MCH5:
		return CmsSigMCH5Data
	case PT_MCH6:
		return CmsSigMCH6Data
	case PT_MCH7:
		return CmsSigMCH7Data
	case PT_MCH8:
		return CmsSigMCH8Data
	case PT_MCH9:
		return CmsSigMCH9Data
	case PT_MCH10:
		return CmsSigMCHAData
	case PT_MCH11:
		return CmsSigMCHBData
	case PT_MCH12:
		return CmsSigMCHCData
	case PT_MCH13:
		return CmsSigMCHDData
	case PT_MCH14:
		return CmsSigMCHEData
	case PT_MCH15:
		return CmsSigMCHFData
	default:
		return cmsColorSpaceSignature(0)
	}
}

// Translate from ICC representation to our colorspace.
func cmsLCMScolorSpace(profileSpace cmsColorSpaceSignature) int {
	switch profileSpace {
	case CmsSigGrayData:
		return PT_GRAY
	case CmsSigRgbData:
		return PT_RGB
	case CmsSigCmyData:
		return PT_CMY
	case CmsSigCmykData:
		return PT_CMYK
	case CmsSigYCbCrData:
		return PT_YCbCr
	case CmsSigLuvData:
		return PT_YUV
	case CmsSigXYZData:
		return PT_XYZ
	case CmsSigLabData:
		return PT_Lab
	case CmsSigLuvKData:
		return PT_YUVK
	case CmsSigHsvData:
		return PT_HSV
	case CmsSigHlsData:
		return PT_HLS
	case CmsSigYxyData:
		return PT_Yxy
	case CmsSigMCH1Data, CmsSig1colorData:
		return PT_MCH1
	case CmsSigMCH2Data, CmsSig2colorData:
		return PT_MCH2
	case CmsSigMCH3Data, CmsSig3colorData:
		return PT_MCH3
	case CmsSigMCH4Data, CmsSig4colorData:
		return PT_MCH4
	case CmsSigMCH5Data, CmsSig5colorData:
		return PT_MCH5
	case CmsSigMCH6Data, CmsSig6colorData:
		return PT_MCH6
	case CmsSigMCH7Data, CmsSig7colorData:
		return PT_MCH7
	case CmsSigMCH8Data, CmsSig8colorData:
		return PT_MCH8
	case CmsSigMCH9Data, CmsSig9colorData:
		return PT_MCH9
	case CmsSigMCHAData, CmsSig10colorData:
		return PT_MCH10
	case CmsSigMCHBData, CmsSig11colorData:
		return PT_MCH11
	case CmsSigMCHCData, CmsSig12colorData:
		return PT_MCH12
	case CmsSigMCHDData, CmsSig13colorData:
		return PT_MCH13
	case CmsSigMCHEData, CmsSig14colorData:
		return PT_MCH14
	case CmsSigMCHFData, CmsSig15colorData:
		return PT_MCH15
	default:
		return 0
	}
}

// Get the number of channels in a color space
func cmsChannelsOfColorSpace(ColorSpace cmsColorSpaceSignature) int32 {
	switch ColorSpace {
	case CmsSigGrayData, CmsSig1colorData, CmsSigMCH1Data:
		return 1
	case CmsSig2colorData, CmsSigMCH2Data:
		return 2
	case CmsSigRgbData, CmsSigLabData, CmsSigXYZData, CmsSigYCbCrData, CmsSigYxyData, CmsSigHsvData, CmsSigHlsData, CmsSigCmyData, CmsSig3colorData, CmsSigMCH3Data:
		return 3
	case CmsSigCmykData, CmsSig4colorData, CmsSigMCH4Data:
		return 4
	case CmsSig5colorData, CmsSigMCH5Data:
		return 5
	case CmsSig6colorData, CmsSigMCH6Data:
		return 6
	case CmsSig7colorData, CmsSigMCH7Data:
		return 7
	case CmsSig8colorData, CmsSigMCH8Data:
		return 8
	case CmsSig9colorData, CmsSigMCH9Data:
		return 9
	case CmsSig10colorData, CmsSigMCHAData:
		return 10
	case CmsSig11colorData, CmsSigMCHBData:
		return 11
	case CmsSig12colorData, CmsSigMCHCData:
		return 12
	case CmsSig13colorData, CmsSigMCHDData:
		return 13
	case CmsSig14colorData, CmsSigMCHEData:
		return 14
	case CmsSig15colorData, CmsSigMCHFData:
		return 15
	default:
		return -1
	}
}

// Deprecated function for getting the number of channels
func cmsChannelsOf(ColorSpace cmsColorSpaceSignature) uint32 {
	n := cmsChannelsOfColorSpace(ColorSpace)
	if n < 0 {
		return 3
	}
	return uint32(n)
}

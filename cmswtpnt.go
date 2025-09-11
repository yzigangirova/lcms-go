package golcms

import (
	//"errors"
	"math"
)

// Constants and definitions
const (
	cmsD50X = 0.9642
	cmsD50Y = 1.0000
	cmsD50Z = 0.8249
)

// D50 - Widely used
func cmsD50_XYZ() *cmsCIEXYZ {
	return &cmsCIEXYZ{X: cmsD50X, Y: cmsD50Y, Z: cmsD50Z}
}

func cmsD50_xyY() *CmsCIExyY {
	d50xyY := &CmsCIExyY{}
	cmsXYZ2xyY(d50xyY, cmsD50_XYZ())
	return d50xyY
}

// Obtains WhitePoint from Temperature
func cmsWhitePointFromTemp(WhitePoint *CmsCIExyY, TempK float64) bool {
	var x, y, T, T2, T3 float64

	if WhitePoint == nil {
		return false
	}

	T = TempK
	T2 = T * T
	T3 = T2 * T

	if T >= 4000. && T <= 7000. {
		x = -4.6070*(1e9/T3) + 2.9678*(1e6/T2) + 0.09911*(1e3/T) + 0.244063
	} else if T > 7000.0 && T <= 25000.0 {
		x = -2.0064*(1e9/T3) + 1.9018*(1e6/T2) + 0.24748*(1e3/T) + 0.237040
	} else {
		return false
	}

	y = -3.000*(x*x) + 2.870*x - 0.275

	WhitePoint.X_small = x
	WhitePoint.Y_small = y
	WhitePoint.Y_large = 1.0

	return true
}

// ISOTEMPERATURE represents isotemperature data used for white point conversions.
type ISOTEMPERATURE struct {
	Mirek float64 // Temperature in microreciprocal kelvin
	Ut    float64 // u coordinate of intersection with blackbody locus
	Vt    float64 // v coordinate of intersection with blackbody locus
	Tt    float64 // Slope of ISOTEMPERATURE line
}

// Isotemperature data
var isotempdata = []ISOTEMPERATURE{
	{0, 0.18006, 0.26352, -0.24341},
	{10, 0.18066, 0.26589, -0.25479},
	{20, 0.18133, 0.26846, -0.26876},
	{30, 0.18208, 0.27119, -0.28539},
	{40, 0.18293, 0.27407, -0.30470},
	{50, 0.18388, 0.27709, -0.32675},
	{60, 0.18494, 0.28021, -0.35156},
	{70, 0.18611, 0.28342, -0.37915},
	{80, 0.18740, 0.28668, -0.40955},
	{90, 0.18880, 0.28997, -0.44278},
	{100, 0.19032, 0.29326, -0.47888},
	{125, 0.19462, 0.30141, -0.58204},
	{150, 0.19962, 0.30921, -0.70471},
	{175, 0.20525, 0.31647, -0.84901},
	{200, 0.21142, 0.32312, -1.0182},
	{225, 0.21807, 0.32909, -1.2168},
	{250, 0.22511, 0.33439, -1.4512},
	{275, 0.23247, 0.33904, -1.7298},
	{300, 0.24010, 0.34308, -2.0637},
	{325, 0.24702, 0.34655, -2.4681},
	{350, 0.25591, 0.34951, -2.9641},
	{375, 0.26400, 0.35200, -3.5814},
	{400, 0.27218, 0.35407, -4.3633},
	{425, 0.28039, 0.35577, -5.3762},
	{450, 0.28863, 0.35714, -6.7262},
	{475, 0.29685, 0.35823, -8.5955},
	{500, 0.30505, 0.35907, -11.324},
	{525, 0.31320, 0.35968, -15.628},
	{550, 0.32129, 0.36011, -23.325},
	{575, 0.32931, 0.36038, -40.770},
	{600, 0.33724, 0.36051, -116.45},
}

// Constants
var NISO = len(isotempdata)

// cmsTempFromWhitePoint calculates the correlated color temperature (CCT) from a given white point.

// Robertson's method
func cmsTempFromWhitePoint(TempK *float64, WhitePoint *CmsCIExyY) bool {
	if WhitePoint == nil || TempK == nil {
		return false
	}

	var us, vs, uj, vj, tj, di, dj, mi, mj float64
	xs, ys := WhitePoint.X_small, WhitePoint.Y_small

	us = (2 * xs) / (-xs + 6*ys + 1.5)
	vs = (3 * ys) / (-xs + 6*ys + 1.5)

	for j := 0; j < len(isotempdata); j++ {
		uj = isotempdata[j].Ut
		vj = isotempdata[j].Vt
		tj = isotempdata[j].Tt
		mj = isotempdata[j].Mirek

		dj = ((vs - vj) - tj*(us-uj)) / math.Sqrt(1.0+tj*tj)

		if j != 0 && di/dj < 0.0 {
			*TempK = 1000000.0 / (mi + (di/(di-dj))*(mj-mi))
			return true
		}

		di = dj
		mi = mj
	}

	return false
}

// Compute chromatic adaptation matrix using Chad as cone matrix
func ComputeChromaticAdaptation(Conversion *cmsMAT3, SourceWhitePoint, DestWhitePoint *cmsCIEXYZ, Chad *cmsMAT3) bool {
	var ChadInv cmsMAT3
	var ConeSourceXYZ, ConeDestXYZ, ConeSourceRGB, ConeDestRGB cmsVEC3
	var Cone, Tmp cmsMAT3

	Tmp = *Chad
	if !cmsMAT3inverse(&Tmp, &ChadInv) {
		return false
	}

	cmsVEC3init(&ConeSourceXYZ, SourceWhitePoint.X, SourceWhitePoint.Y, SourceWhitePoint.Z)
	cmsVEC3init(&ConeDestXYZ, DestWhitePoint.X, DestWhitePoint.Y, DestWhitePoint.Z)

	cmsMAT3eval(&ConeSourceRGB, Chad, &ConeSourceXYZ)
	cmsMAT3eval(&ConeDestRGB, Chad, &ConeDestXYZ)

	if math.Abs(ConeSourceRGB.N[0]) < MATRIX_DET_TOLERANCE ||
		math.Abs(ConeSourceRGB.N[1]) < MATRIX_DET_TOLERANCE ||
		math.Abs(ConeSourceRGB.N[2]) < MATRIX_DET_TOLERANCE {
		return false
	}

	cmsVEC3init(&Cone.V[0], ConeDestRGB.N[0]/ConeSourceRGB.N[0], 0.0, 0.0)
	cmsVEC3init(&Cone.V[1], 0.0, ConeDestRGB.N[1]/ConeSourceRGB.N[1], 0.0)
	cmsVEC3init(&Cone.V[2], 0.0, 0.0, ConeDestRGB.N[2]/ConeSourceRGB.N[2])

	Tmp = cmsMAT3per(&Cone, Chad)
	*Conversion = cmsMAT3per(&ChadInv, &Tmp)

	return true
}

// Returns the final chromatic adaptation matrix from illuminant FromIll to ToIll
func cmsAdaptationMatrix(r *cmsMAT3, ConeMatrix *cmsMAT3, FromIll, ToIll *cmsCIEXYZ) bool {
	var LamRigg = cmsMAT3{
		V: [3]cmsVEC3{
			{N: [3]float64{0.8951, 0.2664, -0.1614}},
			{N: [3]float64{-0.7502, 1.7135, 0.0367}},
			{N: [3]float64{0.0389, -0.0685, 1.0296}},
		},
	}

	if ConeMatrix == nil {
		ConeMatrix = &LamRigg
	}

	return ComputeChromaticAdaptation(r, FromIll, ToIll, ConeMatrix)
}

// cmsAdaptMatrixToD50 computes the adaptation matrix to the D50 white point.
// The source white point is provided in the xyY representation.

func cmsAdaptMatrixToD50(r *cmsMAT3, SourceWhitePt *CmsCIExyY) bool {
	var (
		Dn       cmsCIEXYZ
		Bradford cmsMAT3
		Tmp      cmsMAT3
	)

	// Convert the xyY source white point to XYZ
	cmsxyY2XYZ(&Dn, SourceWhitePt)

	// Compute the adaptation matrix to D50
	if !cmsAdaptationMatrix(&Bradford, nil, &Dn, cmsD50_XYZ()) {
		return false
	}

	// Save the current matrix in Tmp
	Tmp = *r

	// Apply the adaptation matrix
	*r = cmsMAT3per(&Bradford, &Tmp)

	return true
}

// Build a White point, primary chromas transfer matrix from RGB to CIE XYZ
// This is just an approximation, I am not handling all the non-linear
// aspects of the RGB to XYZ process, and assuming that the gamma correction
// has transitive property in the transformation chain.
//
// the algorithm:
//
//   - First I build the absolute conversion matrix using
//     primaries in XYZ. This matrix is next inverted
//   - Then I eval the source white point across this matrix
//     obtaining the coefficients of the transformation
//   - Then, I apply these coefficients to the original matrix

// _cmsBuildRGB2XYZtransferMatrix builds a transformation matrix from RGB to CIE XYZ.
// This is an approximation that assumes gamma correction has a transitive property
// and handles linear transformations only.
//
// Algorithm:
//  1. Build an absolute conversion matrix using primaries in XYZ. This matrix is then inverted.
//  2. Evaluate the source white point across this matrix to obtain transformation coefficients.
//  3. Apply these coefficients to the original matrix.

func cmsBuildRGB2XYZtransferMatrix(r *cmsMAT3, WhitePt *CmsCIExyY, Primrs *CmsCIExyYTRIPLE) bool {
	var (
		WhitePoint, Coef  cmsVEC3
		Result, Primaries cmsMAT3
		xn, yn            float64
		xr, yr            float64
		xg, yg            float64
		xb, yb            float64
	)

	xn = WhitePt.X_small
	yn = WhitePt.Y_small
	xr = Primrs.Red.X_small
	yr = Primrs.Red.Y_small
	xg = Primrs.Green.X_small
	yg = Primrs.Green.Y_small
	xb = Primrs.Blue.X_small
	yb = Primrs.Blue.Y_small

	// Build Primaries matrix
	cmsVEC3init(&Primaries.V[0], xr, xg, xb)
	cmsVEC3init(&Primaries.V[1], yr, yg, yb)
	cmsVEC3init(&Primaries.V[2], (1 - xr - yr), (1 - xg - yg), (1 - xb - yb))

	// Result = Primaries ^ (-1) inverse matrix
	if !cmsMAT3inverse(&Primaries, &Result) {
		return false
	}

	cmsVEC3init(&WhitePoint, xn/yn, 1.0, (1.0-xn-yn)/yn)

	// Across inverse primaries ...
	cmsMAT3eval(&Coef, &Result, &WhitePoint)

	// Build the transformation matrix using Coefs
	cmsVEC3init(&r.V[0], Coef.N[VX]*xr, Coef.N[VY]*xg, Coef.N[VZ]*xb)
	cmsVEC3init(&r.V[1], Coef.N[VX]*yr, Coef.N[VY]*yg, Coef.N[VZ]*yb)
	cmsVEC3init(&r.V[2], Coef.N[VX]*(1.0-xr-yr), Coef.N[VY]*(1.0-xg-yg), Coef.N[VZ]*(1.0-xb-yb))

	return cmsAdaptMatrixToD50(r, WhitePt)
}

// Adapts a color to a given illuminant
func cmsAdaptToIlluminant(Result, SourceWhitePt, Illuminant, Value *cmsCIEXYZ) bool {
	var Bradford cmsMAT3
	var In, Out cmsVEC3

	if Result == nil || SourceWhitePt == nil || Illuminant == nil || Value == nil {
		return false
	}

	if !cmsAdaptationMatrix(&Bradford, nil, SourceWhitePt, Illuminant) {
		return false
	}

	cmsVEC3init(&In, Value.X, Value.Y, Value.Z)
	cmsMAT3eval(&Out, &Bradford, &In)

	Result.X = Out.N[0]
	Result.Y = Out.N[1]
	Result.Z = Out.N[2]

	return true
}

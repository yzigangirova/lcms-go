package golcms

import (
	"math"
	//"unsafe"
)

func DSWAP(x, y *float64) {
	tmp := (*x)
	(*x) = (*y)
	(*y) = tmp
}

// Initiate a vector
func cmsVEC3init(r *cmsVEC3, x float64, y float64, z float64) {
	r.N[VX] = x
	r.N[VY] = y
	r.N[VZ] = z
}

// Vector subtraction
func cmsVEC3minus(r *cmsVEC3, a *cmsVEC3, b *cmsVEC3) {
	r.N[VX] = a.N[VX] - b.N[VX]
	r.N[VY] = a.N[VY] - b.N[VY]
	r.N[VZ] = a.N[VZ] - b.N[VZ]
}

// Vector cross product
func cmsVEC3cross(r, u, v *cmsVEC3) {
	r.N[VX] = u.N[VY]*v.N[VZ] - v.N[VY]*u.N[VZ]
	r.N[VY] = u.N[VZ]*v.N[VX] - v.N[VZ]*u.N[VX]
	r.N[VZ] = u.N[VX]*v.N[VY] - v.N[VX]*u.N[VY]
}

// Vector dot product
func cmsVEC3dot(u, v *cmsVEC3) float64 {
	return u.N[VX]*v.N[VX] + u.N[VY]*v.N[VY] + u.N[VZ]*v.N[VZ]
}

// Euclidean length
func cmsVEC3length(a *cmsVEC3) float64 {
	return math.Sqrt(a.N[VX]*a.N[VX] +
		a.N[VY]*a.N[VY] +
		a.N[VZ]*a.N[VZ])
}

// Euclidean distance
func cmsVEC3distance(a, b *cmsVEC3) float64 {
	d1 := a.N[VX] - b.N[VX]
	d2 := a.N[VY] - b.N[VY]
	d3 := a.N[VZ] - b.N[VZ]

	return math.Sqrt(d1*d1 + d2*d2 + d3*d3)
}

// 3x3 Identity
func cmsMAT3identity(a *cmsMAT3) {
	cmsVEC3init(&a.V[0], 1.0, 0.0, 0.0)
	cmsVEC3init(&a.V[1], 0.0, 1.0, 0.0)
	cmsVEC3init(&a.V[2], 0.0, 0.0, 1.0)
}

func CloseEnough(a, b float64) bool {
	return math.Abs(b-a) < (1.0 / 65535.0)
}

func cmsMAT3isIdentity(a *cmsMAT3) bool {
	var Identity cmsMAT3

	cmsMAT3identity(&Identity)

	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if !CloseEnough(a.V[i].N[j], Identity.V[i].N[j]) {
				return false
			}
		}
	}
	return true
}

// Multiply two matrices
/*func cmsMAT3per(r, a, b *cmsMAT3) {
	// Helper function to compute the dot product for a specific row and column
	rowCol := func(i, j int) float64 {
		return a.V[i].N[0]*b.V[0].N[j] + a.V[i].N[1]*b.V[1].N[j] + a.V[i].N[2]*b.V[2].N[j]
	}

	// Initialize the result matrix
	cmsVEC3init(&r.V[0], rowCol(0, 0), rowCol(0, 1), rowCol(0, 2))
	cmsVEC3init(&r.V[1], rowCol(1, 0), rowCol(1, 1), rowCol(1, 2))
	cmsVEC3init(&r.V[2], rowCol(2, 0), rowCol(2, 1), rowCol(2, 2))
}*/

func cmsMAT3per(a, b *cmsMAT3) cmsMAT3 {
	rowCol := func(i, j int) float64 {
		return a.V[i].N[0]*b.V[0].N[j] + a.V[i].N[1]*b.V[1].N[j] + a.V[i].N[2]*b.V[2].N[j]
	}

	return cmsMAT3{
		V: [3]cmsVEC3{
			{N: [3]float64{rowCol(0, 0), rowCol(0, 1), rowCol(0, 2)}},
			{N: [3]float64{rowCol(1, 0), rowCol(1, 1), rowCol(1, 2)}},
			{N: [3]float64{rowCol(2, 0), rowCol(2, 1), rowCol(2, 2)}},
		},
	}
}
func cmsMAT3perFromSlices(a, b []float64) cmsMAT3 {
	if len(a) != 9 || len(b) != 9 {
		panic("cmsMAT3mulFromSlices: input slices must have 9 elements each")
	}

	rowCol := func(i, j int) float64 {
		return a[i*3+0]*b[0*3+j] + a[i*3+1]*b[1*3+j] + a[i*3+2]*b[2*3+j]
	}

	return cmsMAT3{
		V: [3]cmsVEC3{
			{N: [3]float64{rowCol(0, 0), rowCol(0, 1), rowCol(0, 2)}},
			{N: [3]float64{rowCol(1, 0), rowCol(1, 1), rowCol(1, 2)}},
			{N: [3]float64{rowCol(2, 0), rowCol(2, 1), rowCol(2, 2)}},
		},
	}
}

// Inverse of a matrix b = a^(-1)
func cmsMAT3inverse(a, b *cmsMAT3) bool {
	var det, c0, c1, c2 float64

	c0 = a.V[1].N[1]*a.V[2].N[2] - a.V[1].N[2]*a.V[2].N[1]
	c1 = -a.V[1].N[0]*a.V[2].N[2] + a.V[1].N[2]*a.V[2].N[0]
	c2 = a.V[1].N[0]*a.V[2].N[1] - a.V[1].N[1]*a.V[2].N[0]

	det = a.V[0].N[0]*c0 + a.V[0].N[1]*c1 + a.V[0].N[2]*c2

	if math.Abs(det) < MATRIX_DET_TOLERANCE {
		return false // singular matrix; can't invert
	}
	b.V[0].N[0] = c0 / det
	b.V[0].N[1] = (a.V[0].N[2]*a.V[2].N[1] - a.V[0].N[1]*a.V[2].N[2]) / det
	b.V[0].N[2] = (a.V[0].N[1]*a.V[1].N[2] - a.V[0].N[2]*a.V[1].N[1]) / det
	b.V[1].N[0] = c1 / det
	b.V[1].N[1] = (a.V[0].N[0]*a.V[2].N[2] - a.V[0].N[2]*a.V[2].N[0]) / det
	b.V[1].N[2] = (a.V[0].N[2]*a.V[1].N[0] - a.V[0].N[0]*a.V[1].N[2]) / det
	b.V[2].N[0] = c2 / det
	b.V[2].N[1] = (a.V[0].N[1]*a.V[2].N[0] - a.V[0].N[0]*a.V[2].N[1]) / det
	b.V[2].N[2] = (a.V[0].N[0]*a.V[1].N[1] - a.V[0].N[1]*a.V[1].N[0]) / det

	return true
}

// Solve a system in the form Ax = b
func cmsMAT3solve(x *cmsVEC3, a *cmsMAT3, b *cmsVEC3) bool {
	var m, a_1 cmsMAT3

	m = *a // Struct copy – safe, idiomatic, efficient

	if !cmsMAT3inverse(&m, &a_1) {
		return false // Singular matrix
	}
	cmsMAT3eval(x, &a_1, b)
	return true
}

// Evaluate a vector across a matrix
func cmsMAT3eval(r *cmsVEC3, a *cmsMAT3, v *cmsVEC3) {
	r.N[VX] = a.V[0].N[VX]*v.N[VX] + a.V[0].N[VY]*v.N[VY] + a.V[0].N[VZ]*v.N[VZ]
	r.N[VY] = a.V[1].N[VX]*v.N[VX] + a.V[1].N[VY]*v.N[VY] + a.V[1].N[VZ]*v.N[VZ]
	r.N[VZ] = a.V[2].N[VX]*v.N[VX] + a.V[2].N[VY]*v.N[VY] + a.V[2].N[VZ]*v.N[VZ]
}

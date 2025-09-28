package golcms

import (
	"math"
	"testing"
)

func TestDSWAP(t *testing.T) {
	x, y := 1.0, 2.0
	DSWAP(&x, &y)
	if x != 2.0 || y != 1.0 {
		t.Errorf("DSWAP failed: x=%f, y=%f", x, y)
	}
}

func TestCmsVEC3init(t *testing.T) {
	var v cmsVEC3
	cmsVEC3init(&v, 1.1, 2.2, 3.3)
	if v.N[0] != 1.1 || v.N[1] != 2.2 || v.N[2] != 3.3 {
		t.Errorf("cmsVEC3init failed: got %v", v.N)
	}
}

func TestCmsVEC3minus(t *testing.T) {
	var a = cmsVEC3{N: [3]float64{3, 2, 1}}
	var b = cmsVEC3{N: [3]float64{1, 1, 1}}
	var r cmsVEC3
	cmsVEC3minus(&r, &a, &b)
	if r.N != [3]float64{2, 1, 0} {
		t.Errorf("cmsVEC3minus failed: got %v", r.N)
	}
}

func TestCmsVEC3dot(t *testing.T) {
	a := cmsVEC3{N: [3]float64{1, 2, 3}}
	b := cmsVEC3{N: [3]float64{4, 5, 6}}
	dot := cmsVEC3dot(&a, &b)
	if math.Abs(dot-32.0) > 1e-6 {
		t.Errorf("cmsVEC3dot expected 32, got %f", dot)
	}
}

func TestCmsVEC3length(t *testing.T) {
	a := cmsVEC3{N: [3]float64{3, 4, 0}}
	l := cmsVEC3length(&a)
	if math.Abs(l-5.0) > 1e-6 {
		t.Errorf("cmsVEC3length expected 5.0, got %f", l)
	}
}
func TestCmsVEC3cross(t *testing.T) {
	a := cmsVEC3{N: [3]float64{1, 0, 0}}
	b := cmsVEC3{N: [3]float64{0, 1, 0}}
	var result cmsVEC3
	cmsVEC3cross(&result, &a, &b)
	expected := [3]float64{0, 0, 1}
	for i := 0; i < 3; i++ {
		if math.Abs(result.N[i]-expected[i]) > 1e-6 {
			t.Errorf("cmsVEC3cross failed at %d: got %f, want %f", i, result.N[i], expected[i])
		}
	}
}

func TestCmsVEC3distance(t *testing.T) {
	a := cmsVEC3{N: [3]float64{0, 0, 0}}
	b := cmsVEC3{N: [3]float64{3, 4, 0}}
	d := cmsVEC3distance(&a, &b)
	if math.Abs(d-5.0) > 1e-6 {
		t.Errorf("cmsVEC3distance expected 5.0, got %f", d)
	}
}

func TestCmsMAT3identity(t *testing.T) {
	var m cmsMAT3
	cmsMAT3identity(&m)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			expected := 0.0
			if i == j {
				expected = 1.0
			}
			if math.Abs(m.V[i].N[j]-expected) > 1e-6 {
				t.Errorf("identity matrix wrong at [%d][%d]: got %f", i, j, m.V[i].N[j])
			}
		}
	}
}

func TestCmsMAT3isIdentity(t *testing.T) {
	var m cmsMAT3
	cmsMAT3identity(&m)
	if !cmsMAT3isIdentity(&m) {
		t.Errorf("cmsMAT3isIdentity failed for identity matrix")
	}
}

/*func TestCmsMAT3per(t *testing.T) {
	m := cmsMAT3{
		V: [3]cmsVEC3{
			{N: [3]float64{1, 2, 3}},
			{N: [3]float64{4, 5, 6}},
			{N: [3]float64{7, 8, 9}},
		},
	}
	cmsMAT3per(&m, 0, 1)
	if m.V[0].N != [3]float64{4, 5, 6} || m.V[1].N != [3]float64{1, 2, 3} {
		t.Errorf("cmsMAT3per failed: row swap incorrect")
	}
}*/

func TestCmsMAT3eval(t *testing.T) {
	var m cmsMAT3
	cmsMAT3identity(&m)
	a := cmsVEC3{N: [3]float64{1, 2, 3}}
	var result cmsVEC3
	cmsMAT3eval(&result, &m, &a)
	for i := 0; i < 3; i++ {
		if math.Abs(result.N[i]-a.N[i]) > 1e-6 {
			t.Errorf("cmsMAT3eval failed at %d: got %f, want %f", i, result.N[i], a.N[i])
		}
	}
}

/*func TestCmsMAT3inverse(t *testing.T) {
	m := cmsMAT3{
		V: [3]cmsVEC3{
			{N: [3]float64{1, 0, 0}},
			{N: [3]float64{0, 1, 0}},
			{N: [3]float64{0, 0, 1}},
		},
	}
	var r cmsMAT3
	ok := cmsMAT3inverse(&r, &m)
	if !ok || !cmsMAT3isIdentity(&r) {
		t.Errorf("cmsMAT3inverse failed to produce identity matrix")
	}
}*/

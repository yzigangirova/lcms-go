package golcms

import (
	"testing"
)

func TestStrTo16(t *testing.T) {
	if strTo16("en") != 0x656E {
		t.Errorf("strTo16 failed: expected 0x656E, got 0x%04X", strTo16("en"))
	}
}

func TestStrFrom16(t *testing.T) {
	str := strFrom16(0x656E)
	if str[:2] != "en" {
		t.Errorf("strFrom16 failed: expected 'en', got %s", str)
	}
}

func TestCmsMLUalloc_Basic(t *testing.T) {
	mlu := cmsMLUalloc(nil,nil, 0)
	if mlu == nil {
		t.Fatal("cmsMLUalloc returned nil")
	}
	if mlu.AllocatedEntries == 0 || mlu.Entries == nil {
		t.Errorf("cmsMLUalloc did not initialize entries properly")
	}
}

func TestCmsMLUtranslationsCount(t *testing.T) {
	mlu := cmsMLUalloc(nil,nil, 1)
	mlu.UsedEntries = 3
	if cmsMLUtranslationsCount(mlu) != 3 {
		t.Errorf("cmsMLUtranslationsCount expected 3, got %d", cmsMLUtranslationsCount(mlu))
	}
}

func TestSearchMLUEntry_NotFound(t *testing.T) {
	mlu := cmsMLUalloc(nil,nil, 2)
	idx := SearchMLUEntry(mlu, 0x656E, 0x5553)
	if idx != -1 {
		t.Errorf("Expected -1 for missing entry, got %d", idx)
	}
}

func TestGrowMLUtable_DoubleSize(t *testing.T) {
	mlu := cmsMLUalloc(nil,nil, 2)
	ok := GrowMLUtable(mlu)
	if !ok || mlu.AllocatedEntries != 4 {
		t.Errorf("GrowMLUtable failed: AllocatedEntries=%d", mlu.AllocatedEntries)
	}
}
/*func TestCmsMLUsetASCII_SetAndGet(t *testing.T) {
	mlu := cmsMLUalloc(nil, 1)
	ok := cmsMLUsetASCII(mlu, "en", "US", "Hello World")
	if !ok {
		t.Fatal("cmsMLUsetASCII failed")
	}
	str := cmsMLUgetASCII(mlu, "en", "US")
	if str != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", str)
	}
}

func TestCmsMLUdup(t *testing.T) {
	mlu := cmsMLUalloc(nil, 1)
	cmsMLUsetASCII(mlu, "en", "US", "Hi")
	dup := cmsMLUdup(mlu)
	if dup == nil {
		t.Fatal("cmsMLUdup returned nil")
	}
	if cmsMLUgetASCII(dup, "en", "US") != "Hi" {
		t.Errorf("cmsMLUdup did not copy content correctly")
	}
}

func TestCmsMLUtranslationsCodes(t *testing.T) {
	mlu := cmsMLUalloc(nil, 1)
	cmsMLUsetASCII(mlu, "en", "US", "Color")

	langs, countries := cmsMLUtranslationsCodes(mlu)
	if len(langs) != 1 || langs[0] != "en" || countries[0] != "US" {
		t.Errorf("cmsMLUtranslationsCodes mismatch: got %v/%v", langs, countries)
	}
}

func TestCmsAllocNamedColorList(t *testing.T) {
	list := cmsAllocNamedColorList(nil, 1, 3, "prefix", "suffix")
	if list == nil || list.Prefix != "prefix" || list.Suffix != "suffix" {
		t.Errorf("cmsAllocNamedColorList failed to allocate properly")
	}
}

func TestCmsAppendNamedColor(t *testing.T) {
	list := cmsAllocNamedColorList(nil, 1, 3, "", "")
	pcs := []uint16{100, 200, 300}
	device := []uint16{1, 2, 3}
	ok := cmsAppendNamedColor(list, "MyColor", pcs, device)
	if !ok {
		t.Fatal("cmsAppendNamedColor failed")
	}
	if list.nColors != 1 {
		t.Errorf("Expected 1 color, got %d", list.nColors)
	}
}

func TestCmsNamedColorInfo(t *testing.T) {
	list := cmsAllocNamedColorList(nil, 1, 3, "", "")
	pcs := []uint16{101, 202, 303}
	device := []uint16{3, 2, 1}
	cmsAppendNamedColor(list, "MyNamed", pcs, device)

	var name string
	var outPCS, outDevice [cmsMAXCHANNELS]uint16

	ok := cmsNamedColorInfo(list, 0, &name, outPCS[:], outDevice[:])
	if !ok || name != "MyNamed" || outPCS[0] != 101 || outDevice[2] != 1 {
		t.Errorf("cmsNamedColorInfo returned incorrect info")
	}
}

func TestCmsNamedColorIndex(t *testing.T) {
	list := cmsAllocNamedColorList(nil, 1, 3, "", "")
	cmsAppendNamedColor(list, "SampleColor", []uint16{1, 1, 1}, []uint16{2, 2, 2})

	idx := cmsNamedColorIndex(list, "SampleColor")
	if idx != 0 {
		t.Errorf("cmsNamedColorIndex expected 0, got %d", idx)
	}
}
func TestCmsEvalNamedColor(t *testing.T) {
	list := cmsAllocNamedColorList(nil, 1, 3, "", "")
	cmsAppendNamedColor(list, "Color1", []uint16{10, 20, 30}, []uint16{1, 2, 3})

	output := make([]uint16, 3)
	ok := cmsEvalNamedColor(list, "Color1", output)
	if !ok || output[0] != 1 || output[2] != 3 {
		t.Errorf("cmsEvalNamedColor failed, got %v", output)
	}
}

func TestCmsEvalNamedColorPCS(t *testing.T) {
	list := cmsAllocNamedColorList(nil, 1, 3, "", "")
	cmsAppendNamedColor(list, "PCS1", []uint16{42, 24, 12}, []uint16{7, 8, 9})

	output := make([]uint16, 3)
	ok := cmsEvalNamedColorPCS(list, "PCS1", output)
	if !ok || output[0] != 42 || output[2] != 12 {
		t.Errorf("cmsEvalNamedColorPCS failed, got %v", output)
	}
}

func TestCmsAllocProfileSequenceDescription(t *testing.T) {
	desc := cmsAllocProfileSequenceDescription(3)
	if desc == nil || desc.n != 3 {
		t.Errorf("cmsAllocProfileSequenceDescription failed: n=%d", desc.n)
	}
}

func TestCmsAllocProfileSequenceDescriptionCopy(t *testing.T) {
	orig := cmsAllocProfileSequenceDescription(2)
	copy := cmsAllocProfileSequenceDescriptionCopy(orig)
	if copy == nil || copy.n != 2 {
		t.Errorf("cmsAllocProfileSequenceDescriptionCopy failed")
	}
}

func TestCmsAllocDict(t *testing.T) {
	d := cmsAllocDict()
	if d == nil || d.Entries == nil {
		t.Fatal("cmsAllocDict returned nil or uninitialized")
	}
}

func TestCmsDictAddEntryAndDup(t *testing.T) {
	d := cmsAllocDict()
	ok := cmsDictAddEntry(d, "key", "value", "display", "lang", "country")
	if !ok {
		t.Fatal("cmsDictAddEntry failed")
	}
	copy := cmsDictDup(d)
	if copy == nil || copy.n != 1 {
		t.Fatal("cmsDictDup failed to duplicate entries")
	}
	if copy.Entries[0].DisplayName != "display" {
		t.Errorf("cmsDictDup copied wrong display name: %s", copy.Entries[0].DisplayName)
	}
}*/

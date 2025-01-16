package golcms

import (
	"unsafe"
)

// LUT tags
var (
    Device2PCS16 = []cmsTagSignature{
        cmsSigAToB0Tag, // Perceptual
        cmsSigAToB1Tag, // Relative colorimetric
        cmsSigAToB2Tag, // Saturation
        cmsSigAToB1Tag, // Absolute colorimetric
    }

    Device2PCSFloat = []cmsTagSignature{
        cmsSigDToB0Tag, // Perceptual
        cmsSigDToB1Tag, // Relative colorimetric
        cmsSigDToB2Tag, // Saturation
        cmsSigDToB3Tag, // Absolute colorimetric
    }

    PCS2Device16 = []cmsTagSignature{
        cmsSigBToA0Tag, // Perceptual
        cmsSigBToA1Tag, // Relative colorimetric
        cmsSigBToA2Tag, // Saturation
        cmsSigBToA1Tag, // Absolute colorimetric
    }

    PCS2DeviceFloat = []cmsTagSignature{
        cmsSigBToD0Tag, // Perceptual
        cmsSigBToD1Tag, // Relative colorimetric
        cmsSigBToD2Tag, // Saturation
        cmsSigBToD3Tag, // Absolute colorimetric
    }
)

// Factors to convert from 1.15 fixed point to 0..1.0 range and vice-versa
const (
    InpAdj = 1.0 / MAX_ENCODEABLE_XYZ // (65536.0 / (65535.0 * 2.0))
    OutpAdj = MAX_ENCODEABLE_XYZ      // ((2.0 * 65535.0) / 65536.0)
)

// Several resources for gray conversions
var (
    GrayInputMatrix = []float64{
        InpAdj * cmsD50X,
        InpAdj * cmsD50Y,
        InpAdj * cmsD50Z,
    }

    OneToThreeInputMatrix = []float64{
        1, 1, 1,
    }

    PickYMatrix = []float64{
        0,
        OutpAdj * cmsD50Y,
        0,
    }

    PickLstarMatrix = []float64{
        1, 0, 0,
    }
)

// cmsReadMediaWhitePoint retrieves the media white point and addresses issues in old profiles.
func cmsReadMediaWhitePoint(Dest *cmsCIEXYZ, hProfile cmsHPROFILE) bool {
    // Ensure Dest is not nil
    if Dest == nil {
        return false
    }

    // Read the media white point tag
    Tag := (*cmsCIEXYZ)(cmsReadTag(hProfile, cmsSigMediaWhitePointTag))

    // If no white point, use D50 as default
    if Tag == nil {
        *Dest = *cmsD50_XYZ()
        return true
    }

    // For V2 display profiles, return D50 as the white point
    if cmsGetEncodedICCversion(unsafe.Pointer(hProfile)) < 0x4000000 {
        if cmsGetDeviceClass(unsafe.Pointer(hProfile)) == cmsSigDisplayClass {
            *Dest = *cmsD50_XYZ()
            return true
        }
    }

    // Assign the retrieved tag to Dest
    *Dest = *Tag
    return true
}
func cmsReadCHAD(Dest *cmsMAT3, hProfile cmsHPROFILE) bool {
    if Dest == nil {
        panic("Destination matrix cannot be nil") // Replace cmsAssert
    }

    // Attempt to read the Chromatic Adaptation Tag
    Tag := cmsReadTag(hProfile, cmsSigChromaticAdaptationTag).(*cmsMAT3)
    if Tag != nil {
        *Dest = *Tag
        return true
    }

    // No CHAD available, default it to identity
    cmsMAT3identity(Dest)

    // For V2 display profiles, ensure D50 as the white point
    if cmsGetEncodedICCversion(unsafe.Pointer(hProfile)) < 0x4000000 {
        if cmsGetDeviceClass(unsafe.Pointer(hProfile)) == cmsSigDisplayClass {
            White := cmsReadTag(hProfile, cmsSigMediaWhitePointTag).(*cmsCIEXYZ)
            if White == nil {
                cmsMAT3identity(Dest)
                return true
            }

            return cmsAdaptationMatrix(Dest, nil, White, cmsD50_XYZ())
        }
    }

    return true
}


func cmsReadDevicelinkLUT(hProfile cmsHPROFILE, Intent uint32) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)

	if Intent > INTENT_ABSOLUTE_COLORIMETRIC {
		return nil
	}

	tag16 := Device2PCS16[Intent]
	tagFloat := Device2PCSFloat[Intent]

	// Handle named color profiles
	if cmsGetDeviceClass(unsafe.Pointer(hProfile)) == cmsSigNamedColorClass {
		nc := (*cmsNAMEDCOLORLIST)(cmsReadTag(hProfile, cmsSigNamedColor2Tag))
		if nc == nil {
			return nil
		}

		Lut := cmsPipelineAlloc(ContextID, 0, 0)
		if Lut == nil {
			goto Error
		}

		if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageAllocNamedColor(nc, false)) {
			goto Error
		}

		if cmsGetColorSpace(unsafe.Pointer(hProfile)) == cmsSigLabData {
			if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocLabV2ToV4(ContextID)) {
				goto Error
			}
		}

		return Lut

	Error:
		cmsPipelineFree(Lut)
		return nil
	}

	// Handle floating point LUTs
	if cmsIsTag(hProfile, tagFloat) {
		return cmsReadFloatDevicelinkTag(hProfile, tagFloat)
	}

	tagFloat = Device2PCSFloat[0]
	if cmsIsTag(hProfile, tagFloat) {
		return cmsPipelineDup((*cmsPipeline)(cmsReadTag(hProfile, tagFloat)))
	}

	// Check for 16-bit LUTs
	if !cmsIsTag(hProfile, tag16) {
		tag16 = Device2PCS16[0]
		if !cmsIsTag(hProfile, tag16) {
			return nil
		}
	}

	// Read the tag
	Lut := (*cmsPipeline)(cmsReadTag(hProfile, tag16))
	if Lut == nil {
		return nil
	}

	// Duplicate the pipeline as the profile owns the original
	Lut = cmsPipelineDup(Lut)
	if Lut == nil {
		return nil
	}

	// Adjust interpolation for Lab PCS
	if cmsGetPCS(unsafe.Pointer(hProfile)) == cmsSigLabData {
		ChangeInterpolationToTrilinear(Lut)
	}

	// Check original tag type
	OriginalType := cmsGetTagTrueType(hProfile, tag16)

	// Adjust for Lab16 output
	if OriginalType != cmsSigLut16Type {
		return Lut
	}

	if cmsGetColorSpace(unsafe.Pointer(hProfile)) == cmsSigLabData {
		if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageAllocLabV4ToV2(ContextID)) {
			goto Error2
		}
	}

	if cmsGetPCS(unsafe.Pointer(hProfile)) == cmsSigLabData {
		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocLabV2ToV4(ContextID)) {
			goto Error2
		}
	}

	return Lut

Error2:
	cmsPipelineFree(Lut)
	return nil
}

func cmsReadInputLUT(hProfile cmsHPROFILE, Intent uint32) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)

	if cmsGetDeviceClass(unsafe.Pointer(hProfile)) == cmsSigNamedColorClass {
		nc := cmsReadTag(hProfile, cmsSigNamedColor2Tag).(*cmsNAMEDCOLORLIST)
		if nc == nil {
			return nil
		}

		Lut := cmsPipelineAlloc(ContextID, 0, 0)
		if Lut == nil {
			return nil
		}

		if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageAllocNamedColor(nc, true)) ||
			!cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocLabV2ToV4(ContextID)) {
			cmsPipelineFree(Lut)
			return nil
		}
		return Lut
	}

	if Intent <= INTENT_ABSOLUTE_COLORIMETRIC {
		tag16 := Device2PCS16[Intent]
		tagFloat := Device2PCSFloat[Intent]

		if cmsIsTag(hProfile, tagFloat) {
			return cmsReadFloatInputTag(hProfile, tagFloat)
		}

		if !cmsIsTag(hProfile, tag16) {
			tag16 = Device2PCS16[0]
		}

		if cmsIsTag(hProfile, tag16) {
			Lut := cmsReadTag(hProfile, tag16).(*cmsPipeline)
			if Lut == nil {
				return nil
			}

			OriginalType := cmsGetTagTrueType(hProfile, tag16)
			Lut = cmsPipelineDup(Lut)

			if OriginalType != cmsSigLut16Type || cmsGetPCS(unsafe.Pointer(hProfile)) != cmsSigLabData {
				return Lut
			}

			if cmsGetColorSpace(hProfile) == cmsSigLabData &&
				!cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageAllocLabV4ToV2(ContextID)) {
				cmsPipelineFree(Lut)
				return nil
			}

			if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocLabV2ToV4(ContextID)) {
				cmsPipelineFree(Lut)
				return nil
			}
			return Lut
		}
	}

	if cmsGetColorSpace(unsafe.Pointer(hProfile)) == cmsSigGrayData {
		return BuildGrayInputMatrixPipeline(hProfile)
	}

	return BuildRGBInputMatrixShaper(hProfile)
}
func BuildGrayOutputPipeline(hProfile cmsHPROFILE) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)
	GrayTRC := cmsReadTag(hProfile, cmsSigGrayTRCTag).(*cmsToneCurve)
	if GrayTRC == nil {
		return nil
	}

	RevGrayTRC := cmsReverseToneCurve(GrayTRC)
	if RevGrayTRC == nil {
		return nil
	}

	Lut := cmsPipelineAlloc(ContextID, 3, 1)
	if Lut == nil {
		cmsFreeToneCurve(RevGrayTRC)
		return nil
	}

	if cmsGetPCS(unsafe.Pointer(hProfile)) == cmsSigLabData {
		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocMatrix(ContextID, 1, 3, PickLstarMatrix, nil)) {
			cmsFreeToneCurve(RevGrayTRC)
			cmsPipelineFree(Lut)
			return nil
		}
	} else {
		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocMatrix(ContextID, 1, 3, PickYMatrix, nil)) {
			cmsFreeToneCurve(RevGrayTRC)
			cmsPipelineFree(Lut)
			return nil
		}
	}

	if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocToneCurves(ContextID, 1, []*cmsToneCurve{RevGrayTRC})) {
		cmsFreeToneCurve(RevGrayTRC)
		cmsPipelineFree(Lut)
		return nil
	}

	cmsFreeToneCurve(RevGrayTRC)
	return Lut
}
func ChangeInterpolationToTrilinear(Lut *cmsPipeline) {
	for Stage := cmsPipelineGetPtrToFirstStage(Lut); Stage != nil; Stage = cmsStageNext(Stage) {
		if cmsStageType(Stage) == cmsSigCLutElemType {
			CLUT := Stage.Data.(*cmsStageCLutData)
			CLUT.Params.DwFlags |= CMS_LERP_FLAGS_TRILINEAR
			cmsSetInterpolationRoutine(Lut.ContextID, CLUT.Params)
		}
	}
}

func cmsReadOutputLUT(hProfile cmsHPROFILE, Intent uint32) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)

	if Intent <= INTENT_ABSOLUTE_COLORIMETRIC {
		tag16 := PCS2Device16[Intent]
		tagFloat := PCS2DeviceFloat[Intent]

		if cmsIsTag(hProfile, tagFloat) {
			return cmsReadFloatOutputTag(hProfile, tagFloat)
		}

		if !cmsIsTag(hProfile, tag16) {
			tag16 = PCS2Device16[0]
		}

		if cmsIsTag(hProfile, tag16) {
			Lut := cmsReadTag(hProfile, tag16).(*cmsPipeline)
			if Lut == nil {
				return nil
			}

			OriginalType := cmsGetTagTrueType(hProfile, tag16)
			Lut = cmsPipelineDup(Lut)

			if cmsGetPCS(unsafe.Pointer(hProfile)) == cmsSigLabData {
				ChangeInterpolationToTrilinear(Lut)
			}

			if OriginalType != cmsSigLut16Type || cmsGetPCS(unsafe.Pointer(hProfile)) != cmsSigLabData {
				return Lut
			}

			if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageAllocLabV4ToV2(ContextID)) {
				cmsPipelineFree(Lut)
				return nil
			}

			if cmsGetColorSpace(unsafe.Pointer(hProfile)) == cmsSigLabData &&
				!cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocLabV2ToV4(ContextID)) {
				cmsPipelineFree(Lut)
				return nil
			}

			return Lut
		}
	}

	if cmsGetColorSpace(unsafe.Pointer(hProfile)) == cmsSigGrayData {
		return BuildGrayOutputPipeline(hProfile)
	}

	return BuildRGBOutputMatrixShaper(hProfile)
}
// cmsReadProfileSequence reads both profile sequence description and profile sequence ID if present,
// then combines them into a unique structure holding both.
func cmsReadProfileSequence(hProfile *cmsHPROFILE) *cmsSEQ {
	var ProfileSeq, ProfileId, NewSeq *cmsSEQ

	// Take profile sequence description first
	ProfileSeq = (*cmsSEQ)(cmsReadTag(hProfile, cmsSigProfileSequenceDescTag))

	// Take profile sequence ID
	ProfileId = (*cmsSEQ)(cmsReadTag(hProfile, cmsSigProfileSequenceIdTag))

	// Handle cases where either or both are NULL
	if ProfileSeq == nil && ProfileId == nil {
		return nil
	}
	if ProfileSeq == nil {
		return cmsDupProfileSequenceDescription(ProfileId)
	}
	if ProfileId == nil {
		return cmsDupProfileSequenceDescription(ProfileSeq)
	}

	// Check if sequence lengths match; otherwise, duplicate the description sequence
	if ProfileSeq.n != ProfileId.n {
		return cmsDupProfileSequenceDescription(ProfileSeq)
	}

	// Duplicate the profile sequence description
	NewSeq = cmsDupProfileSequenceDescription(ProfileSeq)

	// Mix profile sequence ID into the new sequence
	if NewSeq != nil {
		for i := uint32(0); i < ProfileSeq.n; i++ {
			copy(NewSeq.seq[i].ProfileID[:], ProfileId.seq[i].ProfileID[:])
			NewSeq.seq[i].Description = cmsMLUdup(ProfileId.seq[i].Description)
		}
	}

	return NewSeq
}

// cmsWriteProfileSequence dumps the contents of the profile sequence in both tags (if v4 is available).
func cmsWriteProfileSequence(hProfile *cmsHPROFILE, seq *cmsSEQ) bool {
	// Write the profile sequence description tag
	if !cmsWriteTag(hProfile, cmsSigProfileSequenceDescTag, seq) {
		return false
	}

	// If the profile is version 4 or later, write the profile sequence ID tag
	if cmsGetEncodedICCversion(hProfile) >= 0x4000000 {
		if !cmsWriteTag(hProfile, cmsSigProfileSequenceIdTag, seq) {
			return false
		}
	}

	return true
}

// GetMLUFromProfile reads and duplicates an MLU tag from the profile if found.
func GetMLUFromProfile(h *cmsHPROFILE, sig cmsTagSignature) *cmsMLU {
	mlu := (*cmsMLU)(cmsReadTag(h, sig))
	if mlu == nil {
		return nil
	}

	return cmsMLUdup(mlu)
}

func cmsCompileProfileSequence(ContextID cmsContext, nProfiles uint32, hProfiles []cmsHPROFILE) *cmsSEQ {
	// Allocate a profile sequence description
	seq := cmsAllocProfileSequenceDescription(ContextID, nProfiles)
	if seq == nil {
		return nil
	}

	// Iterate through profiles and populate the sequence
	for i := uint32(0); i < nProfiles; i++ {
		ps := &seq.seq[i] // Reference to the current profile sequence descriptor
		h := hProfiles[i] // Current profile

		// Extract header attributes
		cmsGetHeaderAttributes(h, &ps.attributes)
		cmsGetHeaderProfileID(h, &ps.ProfileID.ID8[0])
		ps.deviceMfg = cmsGetHeaderManufacturer(h)
		ps.deviceModel = cmsGetHeaderModel(h)

		// Retrieve technology tag
		techpt := (*cmsTechnologySignature)(cmsReadTag(h, cmsSigTechnologyTag))
		if techpt == nil {
			ps.technology = cmsTechnologySignature(0)
		} else {
			ps.technology = *techpt
		}

		// Retrieve MLU tags
		ps.Manufacturer = GetMLUFromProfile(h, cmsSigDeviceMfgDescTag)
		ps.Model = GetMLUFromProfile(h, cmsSigDeviceModelDescTag)
		ps.Description = GetMLUFromProfile(h, cmsSigProfileDescriptionTag)
	}

	return seq
}

/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package cups

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/cups-connector/cdd"
)

func translationTest(t *testing.T, ppd string, expected *cdd.PrinterDescriptionSection) {
	description, _, _ := translatePPD(ppd)
	if !reflect.DeepEqual(expected, description) {
		e, _ := json.Marshal(expected)
		d, _ := json.Marshal(description)
		t.Logf("expected\n %s\ngot\n %s", e, d)
		t.Fail()
	}
}

func TestTrPrintingSpeed(t *testing.T) {
	ppd := `*PPD-Adobe: "4.3"
*Throughput: "30"`
	expected := &cdd.PrinterDescriptionSection{
		PrintingSpeed: &cdd.PrintingSpeed{
			[]cdd.PrintingSpeedOption{
				cdd.PrintingSpeedOption{
					SpeedPPM: 30.0,
				},
			},
		},
	}
	translationTest(t, ppd, expected)
}

func TestTrMediaSize(t *testing.T) {
	ppd := `*PPD-Adobe: "4.3"
*OpenUI *PageSize: PickOne
*DefaultPageSize: Letter
*PageSize A3/A3: ""
*PageSize ISOB5/B5 - ISO: ""
*PageSize B5/B5 - JIS: ""
*PageSize Letter/Letter: ""
*PageSize HalfLetter/5.5x8.5: ""
*CloseUI: *PageSize`
	expected := &cdd.PrinterDescriptionSection{
		MediaSize: &cdd.MediaSize{
			Option: []cdd.MediaSizeOption{
				cdd.MediaSizeOption{cdd.MediaSizeISOA3, mmToMicrons(297), mmToMicrons(420), false, false, "", "A3", cdd.NewLocalizedString("A3")},
				cdd.MediaSizeOption{cdd.MediaSizeISOB5, mmToMicrons(176), mmToMicrons(250), false, false, "", "ISOB5", cdd.NewLocalizedString("B5 (ISO)")},
				cdd.MediaSizeOption{cdd.MediaSizeJISB5, mmToMicrons(182), mmToMicrons(257), false, false, "", "B5", cdd.NewLocalizedString("B5 (JIS)")},
				cdd.MediaSizeOption{cdd.MediaSizeNALetter, inchesToMicrons(8.5), inchesToMicrons(11), false, true, "", "Letter", cdd.NewLocalizedString("Letter")},
				cdd.MediaSizeOption{cdd.MediaSizeCustom, inchesToMicrons(5.5), inchesToMicrons(8.5), false, false, "", "HalfLetter", cdd.NewLocalizedString("5.5x8.5")},
			},
		},
	}
	translationTest(t, ppd, expected)
}

func TestTrColor(t *testing.T) {
	ppd := `*PPD-Adobe: "4.3"
*OpenUI *ColorModel/Color Mode: PickOne
*DefaultColorModel: Gray
*ColorModel CMYK/Color: "(cmyk) RCsetdevicecolor"
*ColorModel Gray/Black and White: "(gray) RCsetdevicecolor"
*CloseUI: *ColorModel`
	expected := &cdd.PrinterDescriptionSection{
		Color: &cdd.Color{
			Option: []cdd.ColorOption{
				cdd.ColorOption{"CMYK", cdd.ColorTypeStandardColor, "", false, cdd.NewLocalizedString("Color")},
				cdd.ColorOption{"Gray", cdd.ColorTypeStandardMonochrome, "", true, cdd.NewLocalizedString("Black and White")},
			},
			VendorKey: "ColorModel",
		},
	}
	translationTest(t, ppd, expected)

	ppd = `*PPD-Adobe: "4.3"
*OpenUI *CMAndResolution/Print Color as Gray: PickOne
*OrderDependency: 20 AnySetup *CMAndResolution
*DefaultCMAndResolution: CMYKImageRET3600
*CMAndResolution CMYKImageRET3600/Off: "
  <</ProcessColorModel /DeviceCMYK /HWResolution [600 600] /PreRenderingEnhance false >> setpagedevice"
*End
*CMAndResolution Gray600x600dpi/On: "
  <</ProcessColorModel /DeviceGray /HWResolution [600 600] >> setpagedevice"
*End
*CloseUI: *CMAndResolution
`
	expected = &cdd.PrinterDescriptionSection{
		Color: &cdd.Color{
			Option: []cdd.ColorOption{
				cdd.ColorOption{"CMYKImageRET3600", cdd.ColorTypeStandardColor, "", true, cdd.NewLocalizedString("Color")},
				cdd.ColorOption{"Gray600x600dpi", cdd.ColorTypeStandardMonochrome, "", false, cdd.NewLocalizedString("Gray")},
			},
			VendorKey: "CMAndResolution",
		},
	}
	translationTest(t, ppd, expected)

	ppd = `*PPD-Adobe: "4.3"
*OpenUI *CMAndResolution/Print Color as Gray: PickOne
*OrderDependency: 20 AnySetup *CMAndResolution
*DefaultCMAndResolution: CMYKImageRET2400
*CMAndResolution CMYKImageRET2400/Off - ImageRET 2400: "<< /ProcessColorModel /DeviceCMYK /HWResolution [600 600]  >> setpagedevice"
*CMAndResolution Gray1200x1200dpi/On - ProRes 1200: "<</ProcessColorModel /DeviceGray /HWResolution [1200 1200] /PreRenderingEnhance false>> setpagedevice"
*CMAndResolution Gray600x600dpi/On - 600 dpi: "<</ProcessColorModel /DeviceGray /HWResolution [600 600] /PreRenderingEnhance false>> setpagedevice"
*CloseUI: *CMAndResolution
`
	expected = &cdd.PrinterDescriptionSection{
		Color: &cdd.Color{
			Option: []cdd.ColorOption{
				cdd.ColorOption{"CMYKImageRET2400", cdd.ColorTypeStandardColor, "", true, cdd.NewLocalizedString("Color, ImageRET 2400")},
				cdd.ColorOption{"Gray1200x1200dpi", cdd.ColorTypeCustomMonochrome, "", false, cdd.NewLocalizedString("Gray, ProRes 1200")},
				cdd.ColorOption{"Gray600x600dpi", cdd.ColorTypeCustomMonochrome, "", false, cdd.NewLocalizedString("Gray, 600 dpi")},
			},
			VendorKey: "CMAndResolution",
		},
	}
	translationTest(t, ppd, expected)

	ppd = `*PPD-Adobe: "4.3"
*OpenUI  *SelectColor/Select Color: PickOne
*OrderDependency: 10 AnySetup *SelectColor
*DefaultSelectColor: Color
*SelectColor Color/Color:  "<</ProcessColorModel /DeviceCMYK>> setpagedevice"
*SelectColor Grayscale/Grayscale:  "<</ProcessColorModel /DeviceGray>> setpagedevice"
*CloseUI: *SelectColor
`
	expected = &cdd.PrinterDescriptionSection{
		Color: &cdd.Color{
			Option: []cdd.ColorOption{
				cdd.ColorOption{"Color", cdd.ColorTypeStandardColor, "", true, cdd.NewLocalizedString("Color")},
				cdd.ColorOption{"Grayscale", cdd.ColorTypeStandardMonochrome, "", false, cdd.NewLocalizedString("Grayscale")},
			},
			VendorKey: "SelectColor",
		},
	}
	translationTest(t, ppd, expected)
}

func TestTrDuplex(t *testing.T) {
	ppd := `*PPD-Adobe: "4.3"
*OpenUI *Duplex/Duplex: PickOne
*DefaultDuplex: None
*Duplex None/Off: ""
*Duplex DuplexNoTumble/Long Edge: ""
*CloseUI: *Duplex`
	expected := &cdd.PrinterDescriptionSection{
		Duplex: &cdd.Duplex{
			Option: []cdd.DuplexOption{
				cdd.DuplexOption{cdd.DuplexNoDuplex, true, "None"},
				cdd.DuplexOption{cdd.DuplexLongEdge, false, "DuplexNoTumble"},
			},
			VendorKey: "Duplex",
		},
	}
	translationTest(t, ppd, expected)
}

func TestTrKMDuplex(t *testing.T) {
	ppd := `*PPD-Adobe: "4.3"
*OpenUI  *KMDuplex/Print Type: PickOne
*OrderDependency: 5 AnySetup *KMDuplex
*DefaultKMDuplex: Double
*KMDuplex Single/1-Sided:  "<< /Duplex false >> setpagedevice
 << /Layout 0 >> /KMOptions /ProcSet findresource /setKMoptions get exec"
*End
*KMDuplex Double/2-Sided:  "<< /Duplex true >> setpagedevice
 << /Layout 0 >> /KMOptions /ProcSet findresource /setKMoptions get exec"
*End
*KMDuplex Booklet/Booklet:  "<< /Duplex true >> setpagedevice
 << /Layout 1 >> /KMOptions /ProcSet findresource /setKMoptions get exec"
*End
*CloseUI: *KMDuplex
`
	expected := &cdd.PrinterDescriptionSection{
		Duplex: &cdd.Duplex{
			Option: []cdd.DuplexOption{
				cdd.DuplexOption{cdd.DuplexNoDuplex, false, "Single"},
				cdd.DuplexOption{cdd.DuplexLongEdge, true, "Double"},
				cdd.DuplexOption{cdd.DuplexShortEdge, false, "Booklet"},
			},
			VendorKey: "KMDuplex",
		},
	}
	translationTest(t, ppd, expected)

	ppd = `*PPD-Adobe: "4.3"
*OpenUI  *KMDuplex/Duplex: Boolean
*OrderDependency: 15 AnySetup *KMDuplex
*DefaultKMDuplex: False
*KMDuplex False/Off:  "<< /Duplex false >> setpagedevice"
*KMDuplex True/On:  "<< /Duplex true >> setpagedevice"
*CloseUI: *KMDuplex
`
	expected = &cdd.PrinterDescriptionSection{
		Duplex: &cdd.Duplex{
			Option: []cdd.DuplexOption{
				cdd.DuplexOption{cdd.DuplexNoDuplex, true, "False"},
				cdd.DuplexOption{cdd.DuplexLongEdge, false, "True"},
			},
			VendorKey: "KMDuplex",
		},
	}
	translationTest(t, ppd, expected)
}

func TestTrDPI(t *testing.T) {
	ppd := `*PPD-Adobe: "4.3"
*OpenUI *Resolution/Resolution: PickOne
*DefaultResolution: 600dpi
*Resolution 600dpi/600 dpi: ""
*Resolution 1200x600dpi/1200x600 dpi: ""
*Resolution 1200x1200dpi/1200 dpi: ""
*CloseUI: *Resolution`
	expected := &cdd.PrinterDescriptionSection{
		DPI: &cdd.DPI{
			Option: []cdd.DPIOption{
				cdd.DPIOption{600, 600, true, "", "600dpi", cdd.NewLocalizedString("600 dpi")},
				cdd.DPIOption{1200, 600, false, "", "1200x600dpi", cdd.NewLocalizedString("1200x600 dpi")},
				cdd.DPIOption{1200, 1200, false, "", "1200x1200dpi", cdd.NewLocalizedString("1200 dpi")},
			},
		},
	}
	translationTest(t, ppd, expected)
}

func TestTrInputSlot(t *testing.T) {
	ppd := `*PPD-Adobe: "4.3"
*OpenUI *OutputBin/Destination: PickOne
*OrderDependency: 210 AnySetup *OutputBin
*DefaultOutputBin: FinProof
*OutputBin Standard/Internal Tray 1: ""
*OutputBin Bin1/Internal Tray 2: ""
*OutputBin External/External Tray: ""
*CloseUI: *OutputBin`
	expected := &cdd.PrinterDescriptionSection{
		VendorCapability: &[]cdd.VendorCapability{
			cdd.VendorCapability{
				ID:                   "OutputBin",
				Type:                 cdd.VendorCapabilitySelect,
				DisplayNameLocalized: cdd.NewLocalizedString("Destination"),
				SelectCap: &cdd.SelectCapability{
					Option: []cdd.SelectCapabilityOption{
						cdd.SelectCapabilityOption{"Standard", "", true, cdd.NewLocalizedString("Internal Tray 1")},
						cdd.SelectCapabilityOption{"Bin1", "", false, cdd.NewLocalizedString("Internal Tray 2")},
						cdd.SelectCapabilityOption{"External", "", false, cdd.NewLocalizedString("External Tray")},
					},
				},
			},
		},
	}
	translationTest(t, ppd, expected)
}

func TestTrPrintQuality(t *testing.T) {
	ppd := `*PPD-Adobe: "4.3"
*OpenUI *HPPrintQuality/Print Quality: PickOne
*DefaultHPPrintQuality: FastRes1200
*HPPrintQuality FastRes1200/FastRes 1200: ""
*HPPrintQuality 600dpi/600 dpi: ""
*HPPrintQuality ProRes1200/ProRes 1200: ""
*CloseUI: *HPPrintQuality`
	expected := &cdd.PrinterDescriptionSection{
		VendorCapability: &[]cdd.VendorCapability{
			cdd.VendorCapability{
				ID:                   "HPPrintQuality",
				Type:                 cdd.VendorCapabilitySelect,
				DisplayNameLocalized: cdd.NewLocalizedString("Print Quality"),
				SelectCap: &cdd.SelectCapability{
					Option: []cdd.SelectCapabilityOption{
						cdd.SelectCapabilityOption{"FastRes1200", "", true, cdd.NewLocalizedString("FastRes 1200")},
						cdd.SelectCapabilityOption{"600dpi", "", false, cdd.NewLocalizedString("600 dpi")},
						cdd.SelectCapabilityOption{"ProRes1200", "", false, cdd.NewLocalizedString("ProRes 1200")},
					},
				},
			},
		},
	}
	translationTest(t, ppd, expected)
}

func TestRicohLockedPrint(t *testing.T) {
	ppd := `*PPD-Adobe: "4.3"
*OpenUI *JobType/JobType: PickOne
*FoomaticRIPOption JobType: enum CmdLine B
*OrderDependency: 255 AnySetup *JobType
*DefaultJobType: Normal
*JobType Normal/Normal: "%% FoomaticRIPOptionSetting: JobType=Normal"
*JobType SamplePrint/Sample Print: "%% FoomaticRIPOptionSetting: JobType=SamplePrint"
*JobType LockedPrint/Locked Print: ""
*JobType DocServer/Document Server: ""
*CloseUI: *JobType

*OpenUI *LockedPrintPassword/Locked Print Password (4-8 digits): PickOne
*FoomaticRIPOption LockedPrintPassword: password CmdLine C
*FoomaticRIPOptionMaxLength LockedPrintPassword:8
*FoomaticRIPOptionAllowedChars LockedPrintPassword: "0-9"
*OrderDependency: 255 AnySetup *LockedPrintPassword
*DefaultLockedPrintPassword: None
*LockedPrintPassword None/None: ""
*LockedPrintPassword 4001/4001: "%% FoomaticRIPOptionSetting: LockedPrintPassword=4001"
*LockedPrintPassword 4002/4002: "%% FoomaticRIPOptionSetting: LockedPrintPassword=4002"
*LockedPrintPassword 4003/4003: "%% FoomaticRIPOptionSetting: LockedPrintPassword=4003"
*CloseUI: *LockedPrintPassword

*CustomLockedPrintPassword True/Custom Password: ""
*ParamCustomLockedPrintPassword Password: 1 passcode 4 8
`
	expected := &cdd.PrinterDescriptionSection{
		VendorCapability: &[]cdd.VendorCapability{
			cdd.VendorCapability{
				ID:                   "JobType:LockedPrint/LockedPrintPassword",
				Type:                 cdd.VendorCapabilityTypedValue,
				DisplayNameLocalized: cdd.NewLocalizedString("Password (4 numbers)"),
				TypedValueCap: &cdd.TypedValueCapability{
					ValueType: cdd.TypedValueCapabilityTypeString,
				},
			},
		},
	}
	translationTest(t, ppd, expected)
}

func easyModelTest(t *testing.T, input, expected string) {
	got := cleanupModel(input)
	if expected != got {
		t.Logf("expected %s got %s", expected, got)
		t.Fail()
	}
}

func TestCleanupModel(t *testing.T) {
	easyModelTest(t, "C451 PS(P)", "C451")
	easyModelTest(t, "MD-1000 Foomatic/md2k", "MD-1000")
	easyModelTest(t, "M24 Foomatic/epson (recommended)", "M24")
	easyModelTest(t, "LaserJet 2 w/PS Foomatic/Postscript (recommended)", "LaserJet 2")
	easyModelTest(t, "8445 PS2", "8445")
	easyModelTest(t, "AL-2600 PS3 v3016.103", "AL-2600")
	easyModelTest(t, "AR-163FG PS, 1.1", "AR-163FG")
	easyModelTest(t, "3212 PXL", "3212")
	easyModelTest(t, "Aficio SP C431DN PDF cups-team recommended", "Aficio SP C431DN")
	easyModelTest(t, "PIXMA Pro9000 - CUPS+Gutenprint v5.2.8-pre1", "PIXMA Pro9000")
	easyModelTest(t, "LaserJet M401dne PS A4 cups-team recommended", "LaserJet M401dne")
	easyModelTest(t, "LaserJet 4250 PS v3010.107 cups-team Letter+Duplex", "LaserJet 4250")
	easyModelTest(t, "Designjet Z5200 PostScript - PS", "Designjet Z5200")
	easyModelTest(t, "DCP-7025 BR-Script3", "DCP-7025")
	easyModelTest(t, "HL-5070DN BR-Script3J", "HL-5070DN")
	easyModelTest(t, "HL-1450 BR-Script2", "HL-1450")
	easyModelTest(t, "FS-600 (KPDL-2) Foomatic/Postscript (recommended)", "FS-600")
	easyModelTest(t, "XP-750 Series, Epson Inkjet Printer Driver (ESC/P-R) for Linux", "XP-750 Series")
	easyModelTest(t, "C5700(PS)", "C5700")
	easyModelTest(t, "OfficeJet 7400 Foomatic/hpijs (recommended) - HPLIP 0.9.7", "OfficeJet 7400")
	easyModelTest(t, "LaserJet p4015n, hpcups 3.13.9", "LaserJet p4015n")
	easyModelTest(t, "Color LaserJet 3600 hpijs, 3.13.9, requires proprietary plugin", "Color LaserJet 3600")
	easyModelTest(t, "LaserJet 4250 pcl3, hpcups 3.13.9", "LaserJet 4250")
	easyModelTest(t, "DesignJet T790 pcl, 1.0", "DesignJet T790")
}

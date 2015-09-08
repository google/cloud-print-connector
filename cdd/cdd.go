/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

// package cdd represents the Cloud Device Description format described here:
// https://developers.google.com/cloud-print/docs/cdd
//
// Not-required fields are marked with the omitempty JSON attribute.
package cdd

import (
	"strconv"
	"strings"
)

type CloudDeviceDescription struct {
	Version string                     `json:"version"`
	Printer *PrinterDescriptionSection `json:"printer"`
}

type PrinterDescriptionSection struct {
	SupportedContentType *[]SupportedContentType `json:"supported_content_type,omitempty"`
	PrintingSpeed        *PrintingSpeed          `json:"printing_speed,omitempty"`
	PWGRasterConfig      *PWGRasterConfig        `json:"pwg_raster_config,omitempty"`
	InputTrayUnit        *[]InputTrayUnit        `json:"input_tray_unit,omitempty"`
	OutputBinUnit        *[]OutputBinUnit        `json:"output_bin_unit,omitempty"`
	Marker               *[]Marker               `json:"marker,omitempty"`
	Cover                *[]Cover                `json:"cover,omitempty"`
	MediaPath            *[]MediaPath            `json:"media_path,omitempty"`
	VendorCapability     *[]VendorCapability     `json:"vendor_capability,omitempty"`
	Color                *Color                  `json:"color,omitempty"`
	Duplex               *Duplex                 `json:"duplex,omitempty"`
	PageOrientation      *PageOrientation        `json:"page_orientation,omitempty"`
	Copies               *Copies                 `json:"copies,omitempty"`
	Margins              *Margins                `json:"margins,omitempty"`
	DPI                  *DPI                    `json:"dpi,omitempty"`
	FitToPage            *FitToPage              `json:"fit_to_page,omitempty"`
	PageRange            *PageRange              `json:"page_range,omitempty"`
	MediaSize            *MediaSize              `json:"media_size,omitempty"`
	Collate              *Collate                `json:"collate,omitempty"`
	ReverseOrder         *ReverseOrder           `json:"reverse_order,omitempty"`
}

// Absorb copies all non-nil fields from the passed-in description.
func (a *PrinterDescriptionSection) Absorb(b *PrinterDescriptionSection) {
	if b.SupportedContentType != nil {
		a.SupportedContentType = b.SupportedContentType
	}
	if b.PrintingSpeed != nil {
		a.PrintingSpeed = b.PrintingSpeed
	}
	if b.PWGRasterConfig != nil {
		a.PWGRasterConfig = b.PWGRasterConfig
	}
	if b.InputTrayUnit != nil {
		a.InputTrayUnit = b.InputTrayUnit
	}
	if b.OutputBinUnit != nil {
		a.OutputBinUnit = b.OutputBinUnit
	}
	if b.Marker != nil {
		a.Marker = b.Marker
	}
	if b.Cover != nil {
		a.Cover = b.Cover
	}
	if b.MediaPath != nil {
		a.MediaPath = b.MediaPath
	}
	if b.VendorCapability != nil {
		if a.VendorCapability == nil || len(*a.VendorCapability) == 0 {
			a.VendorCapability = b.VendorCapability
		} else { // Preserve vendor capabilities that already exist in a.
			aKeys := make(map[string]struct{}, len(*a.VendorCapability))
			for _, v := range *a.VendorCapability {
				aKeys[v.ID] = struct{}{}
			}
			for _, v := range *b.VendorCapability {
				if _, exists := aKeys[v.ID]; !exists {
					*a.VendorCapability = append(*a.VendorCapability, v)
				}
			}
		}
	}
	if b.Color != nil {
		a.Color = b.Color
	}
	if b.Duplex != nil {
		a.Duplex = b.Duplex
	}
	if b.PageOrientation != nil {
		a.PageOrientation = b.PageOrientation
	}
	if b.Copies != nil {
		a.Copies = b.Copies
	}
	if b.Margins != nil {
		a.Margins = b.Margins
	}
	if b.DPI != nil {
		a.DPI = b.DPI
	}
	if b.FitToPage != nil {
		a.FitToPage = b.FitToPage
	}
	if b.PageRange != nil {
		a.PageRange = b.PageRange
	}
	if b.MediaSize != nil {
		a.MediaSize = b.MediaSize
	}
	if b.Collate != nil {
		a.Collate = b.Collate
	}
	if b.ReverseOrder != nil {
		a.ReverseOrder = b.ReverseOrder
	}
}

type SupportedContentType struct {
	ContentType string `json:"content_type"`
	MinVersion  string `json:"min_version,omitempty"`
	MaxVersion  string `json:"max_version,omitempty"`
}

func NewSupportedContentType(contentType string) *[]SupportedContentType {
	return &[]SupportedContentType{SupportedContentType{ContentType: contentType}}
}

type PrintingSpeed struct {
	Option []PrintingSpeedOption `json:"option,omitempty"`
}

type PrintingSpeedOption struct {
	SpeedPPM      float32          `json:"speed_ppm"`
	ColorType     *[]ColorType     `json:"color_type,omitempty"`
	MediaSizeName *[]MediaSizeName `json:"media_size_name,omitempty"`
}

type PWGRasterConfig struct {
	DocumentResolutionSupported *[]PWGRasterConfigResolution `json:"document_resolution_supported,omitempty"`
	DocumentTypeSupported       *[]string                    `json:"document_type_supported,omitempty"` // enum
	DocumentSheetBack           string                       `json:"document_sheet_back,omitempty"`     // enum; default = "ROTATED"
	ReverseOrderStreaming       *bool                        `json:"reverse_order_streaming,omitempty"`
	RotateAllPages              *bool                        `json:"rotate_all_pages,omitempty"`
}

type PWGRasterConfigResolution struct {
	CrossFeedDir int32 `json:"cross_feed_dir"`
	FeedDir      int32 `json:"feed_dir"`
}

type InputTrayUnitType string

const (
	InputTrayUnitCustom         InputTrayUnitType = "CUSTOM"
	InputTrayUnitInputTray      InputTrayUnitType = "INPUT_TRAY"
	InputTrayUnitBypassTray     InputTrayUnitType = "BYPASS_TRAY"
	InputTrayUnitManualFeedTray InputTrayUnitType = "MANUAL_FEED_TRAY"
	InputTrayUnitLCT            InputTrayUnitType = "LCT" // Large capacity tray.
	InputTrayUnitEnvelopeTray   InputTrayUnitType = "ENVELOPE_TRAY"
	InputTrayUnitRoll           InputTrayUnitType = "ROLL"
)

type InputTrayUnit struct {
	VendorID                   string              `json:"vendor_id"`
	Type                       InputTrayUnitType   `json:"type"`
	Index                      *SchizophrenicInt64 `json:"index,omitempty"`
	CustomDisplayName          string              `json:"custom_display_name,omitempty"`
	CustomDisplayNameLocalized *[]LocalizedString  `json:"custom_display_name_localized,omitempty"`
}

type OutputBinUnitType string

const (
	OutputBinUnitCustom    OutputBinUnitType = "CUSTOM"
	OutputBinUnitOutputBin OutputBinUnitType = "OUTPUT_BIN"
	OutputBinUnitMailbox   OutputBinUnitType = "MAILBOX"
	OutputBinUnitStacker   OutputBinUnitType = "STACKER"
)

type OutputBinUnit struct {
	VendorID                   string              `json:"vendor_id"`
	Type                       OutputBinUnitType   `json:"type"`
	Index                      *SchizophrenicInt64 `json:"index,omitempty"`
	CustomDisplayName          string              `json:"custom_display_name,omitempty"`
	CustomDisplayNameLocalized *[]LocalizedString  `json:"custom_display_name_localized,omitempty"`
}

type MarkerType string

const (
	MarkerCustom  MarkerType = "CUSTOM"
	MarkerToner   MarkerType = "TONER"
	MarkerInk     MarkerType = "INK"
	MarkerStaples MarkerType = "STAPLES"
)

type MarkerColorType string

const (
	MarkerColorCustom       MarkerColorType = "CUSTOM"
	MarkerColorBlack        MarkerColorType = "BLACK"
	MarkerColorColor        MarkerColorType = "COLOR"
	MarkerColorCyan         MarkerColorType = "CYAN"
	MarkerColorMagenta      MarkerColorType = "MAGENTA"
	MarkerColorYellow       MarkerColorType = "YELLOW"
	MarkerColorLightCyan    MarkerColorType = "LIGHT_CYAN"
	MarkerColorLightMagenta MarkerColorType = "LIGHT_MAGENTA"
	MarkerColorGray         MarkerColorType = "GRAY"
	MarkerColorLightGray    MarkerColorType = "LIGHT_GRAY"
	MarkerColorPigmentBlack MarkerColorType = "PIGMENT_BLACK"
	MarkerColorMatteBlack   MarkerColorType = "MATTE_BLACK"
	MarkerColorPhotoCyan    MarkerColorType = "PHOTO_CYAN"
	MarkerColorPhotoMagenta MarkerColorType = "PHOTO_MAGENTA"
	MarkerColorPhotoYellow  MarkerColorType = "PHOTO_YELLOW"
	MarkerColorPhotoGray    MarkerColorType = "PHOTO_GRAY"
	MarkerColorRed          MarkerColorType = "RED"
	MarkerColorGreen        MarkerColorType = "GREEN"
	MarkerColorBlue         MarkerColorType = "BLUE"
)

type MarkerColor struct {
	Type                       MarkerColorType    `json:"type"`
	CustomDisplayName          string             `json:"custom_display_name,omitempty"`
	CustomDisplayNameLocalized *[]LocalizedString `json:"custom_display_name_localized,omitempty"`
}

type Marker struct {
	VendorID                   string             `json:"vendor_id"`
	Type                       MarkerType         `json:"type"`
	Color                      *MarkerColor       `json:"color,omitempty"`
	CustomDisplayName          string             `json:"custom_display_name,omitempty"`
	CustomDisplayNameLocalized *[]LocalizedString `json:"custom_display_name_localized,omitempty"`
}

type CoverType string

const (
	CoverTypeCustom CoverType = "CUSTOM"
	CoverTypeDoor   CoverType = "DOOR"
	CoverTypeCover  CoverType = "COVER"
)

type Cover struct {
	VendorID                   string              `json:"vendor_id"`
	Type                       CoverType           `json:"type"`
	Index                      *SchizophrenicInt64 `json:"index,omitempty"`
	CustomDisplayName          string              `json:"custom_display_name,omitempty"`
	CustomDisplayNameLocalized *[]LocalizedString  `json:"custom_display_name_localized,omitempty"`
}

type MediaPath struct {
	VendorID string `json:"vendor_id"`
}

type VendorCapabilityType string

const (
	VendorCapabilityRange      VendorCapabilityType = "RANGE"
	VendorCapabilitySelect     VendorCapabilityType = "SELECT"
	VendorCapabilityTypedValue VendorCapabilityType = "TYPED_VALUE"
)

type VendorCapability struct {
	ID                   string                `json:"id"`
	DisplayName          string                `json:"display_name,omitempty"`
	Type                 VendorCapabilityType  `json:"type"`
	RangeCap             *RangeCapability      `json:"range_cap,omitempty"`
	SelectCap            *SelectCapability     `json:"select_cap,omitempty"`
	TypedValueCap        *TypedValueCapability `json:"typed_value_cap,omitempty"`
	DisplayNameLocalized *[]LocalizedString    `json:"display_name_localized,omitempty"`
}

type RangeCapabilityValueType string

const (
	RangeCapabilityValueFloat   RangeCapabilityValueType = "FLOAT"
	RangeCapabilityValueInteger RangeCapabilityValueType = "INTEGER"
)

type RangeCapability struct {
	ValueType RangeCapabilityValueType `json:"value_type"`
	Default   string                   `json:"default,omitempty"`
	Min       string                   `json:"min,omitempty"`
	Max       string                   `json:"max,omitempty"`
}

type SelectCapability struct {
	Option []SelectCapabilityOption `json:"option"`
}

type SelectCapabilityOption struct {
	Value                string             `json:"value"`
	DisplayName          string             `json:"display_name,omitempty"`
	IsDefault            bool               `json:"is_default"` // default = false
	DisplayNameLocalized *[]LocalizedString `json:"display_name_localized,omitempty"`
}

type TypedValueCapabilityValueType string

const (
	TypedValueCapabilityTypeBoolean TypedValueCapabilityValueType = "BOOLEAN"
	TypedValueCapabilityTypeFloat   TypedValueCapabilityValueType = "FLOAT"
	TypedValueCapabilityTypeInteger TypedValueCapabilityValueType = "INTEGER"
	TypedValueCapabilityTypeString  TypedValueCapabilityValueType = "STRING"
)

type TypedValueCapability struct {
	ValueType TypedValueCapabilityValueType `json:"value_type"`
	Default   string                        `json:"default,omitempty"`
}

type Color struct {
	Option []ColorOption `json:"option"`
}

type ColorType string

const (
	ColorTypeStandardColor      ColorType = "STANDARD_COLOR"
	ColorTypeStandardMonochrome ColorType = "STANDARD_MONOCHROME"
	ColorTypeCustomColor        ColorType = "CUSTOM_COLOR"
	ColorTypeCustomMonochrome   ColorType = "CUSTOM_MONOCHROME"
	ColorTypeAuto               ColorType = "AUTO"
)

type ColorOption struct {
	VendorID                   string             `json:"vendor_id"`
	Type                       ColorType          `json:"type"`
	CustomDisplayName          string             `json:"custom_display_name,omitempty"`
	IsDefault                  bool               `json:"is_default"` // default = false
	CustomDisplayNameLocalized *[]LocalizedString `json:"custom_display_name_localized,omitempty"`
}

type Duplex struct {
	Option []DuplexOption `json:"option"`
}

type DuplexType string

const (
	DuplexNoDuplex  DuplexType = "NO_DUPLEX"
	DuplexLongEdge  DuplexType = "LONG_EDGE"
	DuplexShortEdge DuplexType = "SHORT_EDGE"
)

type DuplexOption struct {
	Type      DuplexType `json:"type"`       // default = "NO_DUPLEX"
	IsDefault bool       `json:"is_default"` // default = false
}

type PageOrientation struct {
	Option []PageOrientationOption `json:"option"`
}

type PageOrientationType string

const (
	PageOrientationPortrait  PageOrientationType = "PORTRAIT"
	PageOrientationLandscape PageOrientationType = "LANDSCAPE"
	PageOrientationAuto      PageOrientationType = "AUTO"
)

type PageOrientationOption struct {
	Type      PageOrientationType `json:"type"`
	IsDefault bool                `json:"is_default"` // default = false
}

type Copies struct {
	Default int32 `json:"default"`
	Max     int32 `json:"max"`
}

type Margins struct {
	Option []MarginsOption `json:"option"`
}

type MarginsType string

const (
	MarginsBorderless MarginsType = "BORDERLESS"
	MarginsStandard   MarginsType = "STANDARD"
	MarginsCustom     MarginsType = "CUSTOM"
)

type MarginsOption struct {
	Type          MarginsType `json:"type"`
	TopMicrons    int32       `json:"top_microns"`
	RightMicrons  int32       `json:"right_microns"`
	BottomMicrons int32       `json:"bottom_microns"`
	LeftMicrons   int32       `json:"left_microns"`
	IsDefault     bool        `json:"is_default"` // default = false
}

type DPI struct {
	Option           []DPIOption `json:"option"`
	MinHorizontalDPI int32       `json:"min_horizontal_dpi,omitempty"`
	MaxHorizontalDPI int32       `json:"max_horizontal_dpi,omitempty"`
	MinVerticalDPI   int32       `json:"min_vertical_dpi,omitempty"`
	MaxVerticalDPI   int32       `json:"max_vertical_dpi,omitempty"`
}

type DPIOption struct {
	HorizontalDPI              int32              `json:"horizontal_dpi"`
	VerticalDPI                int32              `json:"vertical_dpi"`
	IsDefault                  bool               `json:"is_default"` // default = false
	CustomDisplayName          string             `json:"custom_display_name,omitempty"`
	VendorID                   string             `json:"vendor_id,omitempty"`
	CustomDisplayNameLocalized *[]LocalizedString `json:"custom_display_name_localized,omitempty"`
}

type FitToPage struct {
	Option []FitToPageOption `json:"option"`
}

type FitToPageType string

const (
	FitToPageNoFitting    FitToPageType = "NO_FITTING"
	FitToPageFitToPage    FitToPageType = "FIT_TO_PAGE"
	FitToPageGrowToPage   FitToPageType = "GROW_TO_PAGE"
	FitToPageShrinkToPage FitToPageType = "SHRINK_TO_PAGE"
	FitToPageFillPage     FitToPageType = "FILL_PAGE"
)

type FitToPageOption struct {
	Type      FitToPageType `json:"type"`
	IsDefault bool          `json:"is_default"` // default = false
}

type PageRange struct {
	Interval []PageRangeInterval `json:"interval"`
}

type PageRangeInterval struct {
	Start int32 `json:"start"`
	End   int32 `json:"end,omitempty"`
}

type MediaSize struct {
	Option           []MediaSizeOption `json:"option"`
	MaxWidthMicrons  int32             `json:"max_width_microns,omitempty"`
	MaxHeightMicrons int32             `json:"max_height_microns,omitempty"`
	MinWidthMicrons  int32             `json:"min_width_microns,omitempty"`
	MinHeightMicrons int32             `json:"min_height_microns,omitempty"`
}

type MediaSizeOption struct {
	Name                       MediaSizeName      `json:"name"` // default = "CUSTOM"
	WidthMicrons               int32              `json:"width_microns,omitempty"`
	HeightMicrons              int32              `json:"height_microns,omitempty"`
	IsContinuousFeed           bool               `json:"is_continuous_feed"` // default = false
	IsDefault                  bool               `json:"is_default"`         // default = false
	CustomDisplayName          string             `json:"custom_display_name,omitempty"`
	VendorID                   string             `json:"vendor_id,omitempty"`
	CustomDisplayNameLocalized *[]LocalizedString `json:"custom_display_name_localized,omitempty"`
}

type MediaSizeName string

const (
	MediaSizeCustom        MediaSizeName = "CUSTOM"
	MediaSizeNAIndex3x5    MediaSizeName = "NA_INDEX_3X5"
	MediaSizeNAPersonal    MediaSizeName = "NA_PERSONAL"
	MediaSizeNAMonarch     MediaSizeName = "NA_MONARCH"
	MediaSizeNANumber9     MediaSizeName = "NA_NUMBER_9"
	MediaSizeNAIndex4x6    MediaSizeName = "NA_INDEX_4X6"
	MediaSizeNANumber10    MediaSizeName = "NA_NUMBER_10"
	MediaSizeNAA2          MediaSizeName = "NA_A2"
	MediaSizeNANumber11    MediaSizeName = "NA_NUMBER_11"
	MediaSizeNANumber12    MediaSizeName = "NA_NUMBER_12"
	MediaSizeNA5x7         MediaSizeName = "NA_5X7"
	MediaSizeNAIndex5x8    MediaSizeName = "NA_INDEX_5X8"
	MediaSizeNANumber14    MediaSizeName = "NA_NUMBER_14"
	MediaSizeNAInvoice     MediaSizeName = "NA_INVOICE"
	MediaSizeNAIndex4x6Ext MediaSizeName = "NA_INDEX_4X6_EXT"
	MediaSizeNA6x9         MediaSizeName = "NA_6X9"
	MediaSizeNAC5          MediaSizeName = "NA_C5"
	MediaSizeNA7x9         MediaSizeName = "NA_7X9"
	MediaSizeNAExecutive   MediaSizeName = "NA_EXECUTIVE"
	MediaSizeNAGovtLetter  MediaSizeName = "NA_GOVT_LETTER"
	MediaSizeNAGovtLegal   MediaSizeName = "NA_GOVT_LEGAL"
	MediaSizeNAQuarto      MediaSizeName = "NA_QUARTO"
	MediaSizeNALetter      MediaSizeName = "NA_LETTER"
	MediaSizeNAFanfoldEur  MediaSizeName = "NA_FANFOLD_EUR"
	MediaSizeNALetterPlus  MediaSizeName = "NA_LETTER_PLUS"
	MediaSizeNAFoolscap    MediaSizeName = "NA_FOOLSCAP"
	MediaSizeNALegal       MediaSizeName = "NA_LEGAL"
	MediaSizeNASuperA      MediaSizeName = "NA_SUPER_A"
	MediaSizeNA9x11        MediaSizeName = "NA_9X11"
	MediaSizeNAArchA       MediaSizeName = "NA_ARCH_A"
	MediaSizeNALetterExtra MediaSizeName = "NA_LETTER_EXTRA"
	MediaSizeNALegalExtra  MediaSizeName = "NA_LEGAL_EXTRA"
	MediaSizeNA10x11       MediaSizeName = "NA_10X11"
	MediaSizeNA10x13       MediaSizeName = "NA_10X13"
	MediaSizeNA10x14       MediaSizeName = "NA_10X14"
	MediaSizeNA10x15       MediaSizeName = "NA_10X15"
	MediaSizeNA11x12       MediaSizeName = "NA_11X12"
	MediaSizeNAEDP         MediaSizeName = "NA_EDP"
	MediaSizeNAFanfoldUS   MediaSizeName = "NA_FANFOLD_US"
	MediaSizeNA11x15       MediaSizeName = "NA_11X15"
	MediaSizeNALedger      MediaSizeName = "NA_LEDGER"
	MediaSizeNAEurEDP      MediaSizeName = "NA_EUR_EDP"
	MediaSizeNAArchB       MediaSizeName = "NA_ARCH_B"
	MediaSizeNA12x19       MediaSizeName = "NA_12X19"
	MediaSizeNABPlus       MediaSizeName = "NA_B_PLUS"
	MediaSizeNASuperB      MediaSizeName = "NA_SUPER_B"
	MediaSizeNAC           MediaSizeName = "NA_C"
	MediaSizeNAArchC       MediaSizeName = "NA_ARCH_C"
	MediaSizeNAD           MediaSizeName = "NA_D"
	MediaSizeNAArchD       MediaSizeName = "NA_ARCH_D"
	MediaSizeNAAsmeF       MediaSizeName = "NA_ASME_F"
	MediaSizeNAWideFormat  MediaSizeName = "NA_WIDE_FORMAT"
	MediaSizeNAE           MediaSizeName = "NA_E"
	MediaSizeNAArchE       MediaSizeName = "NA_ARCH_E"
	MediaSizeNAF           MediaSizeName = "NA_F"
	MediaSizeROC16k        MediaSizeName = "ROC_16K"
	MediaSizeROC8k         MediaSizeName = "ROC_8K"
	MediaSizePRC32k        MediaSizeName = "PRC_32K"
	MediaSizePRC1          MediaSizeName = "PRC_1"
	MediaSizePRC2          MediaSizeName = "PRC_2"
	MediaSizePRC4          MediaSizeName = "PRC_4"
	MediaSizePRC5          MediaSizeName = "PRC_5"
	MediaSizePRC8          MediaSizeName = "PRC_8"
	MediaSizePRC6          MediaSizeName = "PRC_6"
	MediaSizePRC3          MediaSizeName = "PRC_3"
	MediaSizePRC16k        MediaSizeName = "PRC_16K"
	MediaSizePRC7          MediaSizeName = "PRC_7"
	MediaSizeOMJuuroKuKai  MediaSizeName = "OM_JUURO_KU_KAI"
	MediaSizeOMPaKai       MediaSizeName = "OM_PA_KAI"
	MediaSizeOMDaiPaKai    MediaSizeName = "OM_DAI_PA_KAI"
	MediaSizePRC10         MediaSizeName = "PRC_10"
	MediaSizeISOA10        MediaSizeName = "ISO_A10"
	MediaSizeISOA9         MediaSizeName = "ISO_A9"
	MediaSizeISOA8         MediaSizeName = "ISO_A8"
	MediaSizeISOA7         MediaSizeName = "ISO_A7"
	MediaSizeISOA6         MediaSizeName = "ISO_A6"
	MediaSizeISOA5         MediaSizeName = "ISO_A5"
	MediaSizeISOA5Extra    MediaSizeName = "ISO_A5_EXTRA"
	MediaSizeISOA4         MediaSizeName = "ISO_A4"
	MediaSizeISOA4Tab      MediaSizeName = "ISO_A4_TAB"
	MediaSizeISOA4Extra    MediaSizeName = "ISO_A4_EXTRA"
	MediaSizeISOA3         MediaSizeName = "ISO_A3"
	MediaSizeISOA4x3       MediaSizeName = "ISO_A4X3"
	MediaSizeISOA4x4       MediaSizeName = "ISO_A4X4"
	MediaSizeISOA4x5       MediaSizeName = "ISO_A4X5"
	MediaSizeISOA4x6       MediaSizeName = "ISO_A4X6"
	MediaSizeISOA4x7       MediaSizeName = "ISO_A4X7"
	MediaSizeISOA4x8       MediaSizeName = "ISO_A4X8"
	MediaSizeISOA4x9       MediaSizeName = "ISO_A4X9"
	MediaSizeISOA3Extra    MediaSizeName = "ISO_A3_EXTRA"
	MediaSizeISOA2         MediaSizeName = "ISO_A2"
	MediaSizeISOA3x3       MediaSizeName = "ISO_A3X3"
	MediaSizeISOA3x4       MediaSizeName = "ISO_A3X4"
	MediaSizeISOA3x5       MediaSizeName = "ISO_A3X5"
	MediaSizeISOA3x6       MediaSizeName = "ISO_A3X6"
	MediaSizeISOA3x7       MediaSizeName = "ISO_A3X7"
	MediaSizeISOA1         MediaSizeName = "ISO_A1"
	MediaSizeISOA2x3       MediaSizeName = "ISO_A2X3"
	MediaSizeISOA2x4       MediaSizeName = "ISO_A2X4"
	MediaSizeISOA2x5       MediaSizeName = "ISO_A2X5"
	MediaSizeISOA0         MediaSizeName = "ISO_A0"
	MediaSizeISOA1x3       MediaSizeName = "ISO_A1X3"
	MediaSizeISOA1x4       MediaSizeName = "ISO_A1X4"
	MediaSizeISO2A0        MediaSizeName = "ISO_2A0"
	MediaSizeISOA0x3       MediaSizeName = "ISO_A0X3"
	MediaSizeISOB10        MediaSizeName = "ISO_B10"
	MediaSizeISOB9         MediaSizeName = "ISO_B9"
	MediaSizeISOB8         MediaSizeName = "ISO_B8"
	MediaSizeISOB7         MediaSizeName = "ISO_B7"
	MediaSizeISOB6         MediaSizeName = "ISO_B6"
	MediaSizeISOB6C4       MediaSizeName = "ISO_B6C4"
	MediaSizeISOB5         MediaSizeName = "ISO_B5"
	MediaSizeISOB5Extra    MediaSizeName = "ISO_B5_EXTRA"
	MediaSizeISOB4         MediaSizeName = "ISO_B4"
	MediaSizeISOB3         MediaSizeName = "ISO_B3"
	MediaSizeISOB2         MediaSizeName = "ISO_B2"
	MediaSizeISOB1         MediaSizeName = "ISO_B1"
	MediaSizeISOB0         MediaSizeName = "ISO_B0"
	MediaSizeISOC10        MediaSizeName = "ISO_C10"
	MediaSizeISOC9         MediaSizeName = "ISO_C9"
	MediaSizeISOC8         MediaSizeName = "ISO_C8"
	MediaSizeISOC7         MediaSizeName = "ISO_C7"
	MediaSizeISOC7c6       MediaSizeName = "ISO_C7C6"
	MediaSizeISOC6         MediaSizeName = "ISO_C6"
	MediaSizeISOC6c5       MediaSizeName = "ISO_C6C5"
	MediaSizeISOC5         MediaSizeName = "ISO_C5"
	MediaSizeISOC4         MediaSizeName = "ISO_C4"
	MediaSizeISOC3         MediaSizeName = "ISO_C3"
	MediaSizeISOC2         MediaSizeName = "ISO_C2"
	MediaSizeISOC1         MediaSizeName = "ISO_C1"
	MediaSizeISOC0         MediaSizeName = "ISO_C0"
	MediaSizeISODL         MediaSizeName = "ISO_DL"
	MediaSizeISORA2        MediaSizeName = "ISO_RA2"
	MediaSizeISOSRA2       MediaSizeName = "ISO_SRA2"
	MediaSizeISORA1        MediaSizeName = "ISO_RA1"
	MediaSizeISOSRA1       MediaSizeName = "ISO_SRA1"
	MediaSizeISORA0        MediaSizeName = "ISO_RA0"
	MediaSizeISOSRA0       MediaSizeName = "ISO_SRA0"
	MediaSizeJISB10        MediaSizeName = "JIS_B10"
	MediaSizeJISB9         MediaSizeName = "JIS_B9"
	MediaSizeJISB8         MediaSizeName = "JIS_B8"
	MediaSizeJISB7         MediaSizeName = "JIS_B7"
	MediaSizeJISB6         MediaSizeName = "JIS_B6"
	MediaSizeJISB5         MediaSizeName = "JIS_B5"
	MediaSizeJISB4         MediaSizeName = "JIS_B4"
	MediaSizeJISB3         MediaSizeName = "JIS_B3"
	MediaSizeJISB2         MediaSizeName = "JIS_B2"
	MediaSizeJISB1         MediaSizeName = "JIS_B1"
	MediaSizeJISB0         MediaSizeName = "JIS_B0"
	MediaSizeJISExec       MediaSizeName = "JIS_EXEC"
	MediaSizeJPNChou4      MediaSizeName = "JPN_CHOU4"
	MediaSizeJPNHagaki     MediaSizeName = "JPN_HAGAKI"
	MediaSizeJPNYou4       MediaSizeName = "JPN_YOU4"
	MediaSizeJPNChou2      MediaSizeName = "JPN_CHOU2"
	MediaSizeJPNChou3      MediaSizeName = "JPN_CHOU3"
	MediaSizeJPNOufuku     MediaSizeName = "JPN_OUFUKU"
	MediaSizeJPNKahu       MediaSizeName = "JPN_KAHU"
	MediaSizeJPNKaku2      MediaSizeName = "JPN_KAKU2"
	MediaSizeOMSmallPhoto  MediaSizeName = "OM_SMALL_PHOTO"
	MediaSizeOMItalian     MediaSizeName = "OM_ITALIAN"
	MediaSizeOMPostfix     MediaSizeName = "OM_POSTFIX"
	MediaSizeOMLargePhoto  MediaSizeName = "OM_LARGE_PHOTO"
	MediaSizeOMFolio       MediaSizeName = "OM_FOLIO"
	MediaSizeOMFolioSP     MediaSizeName = "OM_FOLIO_SP"
	MediaSizeOMInvite      MediaSizeName = "OM_INVITE"
)

type Collate struct {
	Default bool `json:"default"` // default = true
}

type ReverseOrder struct {
	Default bool `json:"default"` // default = false
}

type LocalizedString struct {
	Locale string `json:"locale"` // enum; use "EN"
	Value  string `json:"value"`
}

func NewLocalizedString(value string) *[]LocalizedString {
	return &[]LocalizedString{LocalizedString{"EN", value}}
}

// SchizophrenicInt64 is an int64 value that encodes to JSON without quotes,
// but decodes with-or-without quotes. GCP requires this for int64 values.
type SchizophrenicInt64 int64

func NewSchizophrenicInt64(i uint) *SchizophrenicInt64 {
	x := SchizophrenicInt64(i)
	return &x
}

// MarshalJSON marshals without quotes.
func (i SchizophrenicInt64) MarshalJSON() ([]byte, error) {
	return []byte(i.String()), nil
}

// UnmarshalJSON unmarshals with or without quotes.
func (i *SchizophrenicInt64) UnmarshalJSON(data []byte) error {
	s := string(data)
	if len(s) >= 2 &&
		strings.HasPrefix(s, "\"") &&
		strings.HasSuffix(s, "\"") {
		s = s[1 : len(s)-1]
	}

	j, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}

	*i = SchizophrenicInt64(j)
	return nil
}

func (i *SchizophrenicInt64) String() string {
	return strconv.FormatInt(int64(*i), 10)
}

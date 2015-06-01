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
		a.VendorCapability = b.VendorCapability
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
	Option *PrintingSpeedOption `json:"option,omitempty"`
}

type PrintingSpeedOption struct {
	SpeedPPM      float32   `json:"speed_ppm"`
	ColorType     *[]string `json:"color_type,omitempty"`      // enum
	MediaSizeName *[]string `json:"media_size_name,omitempty"` // enum
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
	InputTrayUnitInputTray                        = "INPUT_TRAY"
	InputTrayUnitBypassTray                       = "BYPASS_TRAY"
	InputTrayUnitManualFeedTray                   = "MANUAL_FEED_TRAY"
	InputTrayUnitLCT                              = "LCT" // Large capacity tray.
	InputTrayUnitEnvelopeTray                     = "ENVELOPE_TRAY"
	InputTrayUnitRoll                             = "ROLL"
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
	OutputBinUnitOutputBin                   = "OUTPUT_BIN"
	OutputBinUnitMailbox                     = "MAILBOX"
	OutputBinUnitStacker                     = "STACKER"
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
	MarkerToner              = "TONER"
	MarkerInk                = "INK"
	MarkerStaples            = "STAPLES"
)

type MarkerColorType string

const (
	MarkerColorCustom       MarkerColorType = "CUSTOM"
	MarkerColorBlack                        = "BLACK"
	MarkerColorColor                        = "COLOR"
	MarkerColorCyan                         = "CYAN"
	MarkerColorMagenta                      = "MAGENTA"
	MarkerColorYellow                       = "YELLOW"
	MarkerColorLightCyan                    = "LIGHT_CYAN"
	MarkerColorLightMagenta                 = "LIGHT_MAGENTA"
	MarkerColorGray                         = "GRAY"
	MarkerColorLightGray                    = "LIGHT_GRAY"
	MarkerColorPigmentBlack                 = "PIGMENT_BLACK"
	MarkerColorMatteBlack                   = "MATTE_BLACK"
	MarkerColorPhotoCyan                    = "PHOTO_CYAN"
	MarkerColorPhotoMagenta                 = "PHOTO_MAGENTA"
	MarkerColorPhotoYellow                  = "PHOTO_YELLOW"
	MarkerColorPhotoGray                    = "PHOTO_GRAY"
	MarkerColorRed                          = "RED"
	MarkerColorGreen                        = "GREEN"
	MarkerColorBlue                         = "BLUE"
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
	CoverTypeDoor             = "DOOR"
	CoverTypeCover            = "COVER"
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
	VendorCapabilitySelect                          = "SELECT"
	VendorCapabilityTypedValue                      = "TYPED_VALUE"
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
	RangeCapabilityValueInteger                          = "INTEGER"
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
	TypedValueCapabilityValueBoolean TypedValueCapabilityValueType = "BOOLEAN"
	TypedValueCapabilityValueFloat                                 = "FLOAT"
	TypedValueCapabilityValueInteger                               = "INTEGER"
	TypedValueCapabilityValueString                                = "STRING"
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
	ColorTypeStandardMonochrome           = "STANDARD_MONOCHROME"
	ColorTypeCustomColor                  = "CUSTOM_COLOR"
	ColorTypeCustomMonochrome             = "CUSTOM_MONOCHROME"
	ColorTypeAuto                         = "AUTO"
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
	DuplexLongEdge             = "LONG_EDGE"
	DuplexShortEdge            = "SHORT_EDGE"
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
	PageOrientationLandscape                     = "LANDSCAPE"
	PageOrientationAuto                          = "AUTO"
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
	MarginsStandard               = "STANDARD"
	MarginsCustom                 = "CUSTOM"
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
	VendorID                   string             `json:"vendor_id"`
	CustomDisplayNameLocalized *[]LocalizedString `json:"custom_display_name_localized,omitempty"`
}

type FitToPage struct {
	Option []FitToPageOption `json:"option"`
}

type FitToPageType string

const (
	FitToPageNoFitting    FitToPageType = "NO_FITTING"
	FitToPageFitToPage                  = "FIT_TO_PAGE"
	FitToPageGrowToPage                 = "GROW_TO_PAGE"
	FitToPageShrinkToPage               = "SHRINK_TO_PAGE"
	FitToPageFillPage                   = "FILL_PAGE"
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
	Name                       string             `json:"name"` // enum; default = "CUSTOM"
	WidthMicrons               int32              `json:"width_microns,omitempty"`
	HeightMicrons              int32              `json:"height_microns,omitempty"`
	IsContinuousFeed           bool               `json:"is_continuous_feed"` // default = false
	IsDefault                  bool               `json:"is_default"`         // default = false
	CustomDisplayName          string             `json:"custom_display_name,omitempty"`
	VendorID                   string             `json:"vendor_id,omitempty"`
	CustomDisplayNameLocalized *[]LocalizedString `json:"custom_display_name_localized,omitempty"`
}

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
// but decodes with quotes. GCP requires this for int64 values.
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

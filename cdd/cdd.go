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

type CloudDeviceDescription struct {
	Version string                    `json:"version"`
	Printer PrinterDescriptionSection `json:"printer"`
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

type InputTrayUnit struct {
	VendorID                   string             `json:"vendor_id"`
	Type                       string             `json:"type"` // enum
	Index                      int64              `json:"index,omitempty"`
	CustomDisplayName          string             `json:"custom_display_name,omitempty"`
	CustomDisplayNameLocalized *[]LocalizedString `json:"custom_display_name_localized,omitempty"`
}

type OutputBinUnit struct {
	VendorID                   string             `json:"vendor_id"`
	Type                       string             `json:"type"` // enum
	Index                      int64              `json:"index,omitempty"`
	CustomDisplayName          string             `json:"custom_display_name,omitempty"`
	CustomDisplayNameLocalized *[]LocalizedString `json:"custom_display_name_localized,omitempty"`
}

type Marker struct {
	VendorID                   string             `json:"vendor_id"`
	Type                       string             `json:"type"` // enum
	Color                      *MarkerColor       `json:"color,omitempty"`
	CustomDisplayName          string             `json:"custom_display_name,omitempty"`
	CustomDisplayNameLocalized *[]LocalizedString `json:"custom_display_name_localized,omitempty"`
}

type MarkerColor struct {
	Type                       string             `json:"type"`
	CustomDisplayName          string             `json:"custom_display_name,omitempty"`
	CustomDisplayNameLocalized *[]LocalizedString `json:"custom_display_name_localized,omitempty"`
}

type Cover struct {
	VendorID                   string             `json:"vendor_id"`
	Type                       string             `json:"type"` // enum
	Index                      int64              `json:"index,omitempty"`
	CustomDisplayName          string             `json:"custom_display_name,omitempty"`
	CustomDisplayNameLocalized *[]LocalizedString `json:"custom_display_name_localized,omitempty"`
}

type MediaPath struct {
	VendorID string `json:"vendor_id"`
}

type VendorCapability struct {
	ID                   string                `json:"id"`
	DisplayName          string                `json:"display_name,omitempty"`
	Type                 string                `json:"type"` // enum
	RangeCap             *RangeCapability      `json:"range_cap,omitempty"`
	SelectCap            *SelectCapability     `json:"select_cap,omitempty"`
	TypedValueCap        *TypedValueCapability `json:"typed_value_cap,omitempty"`
	DisplayNameLocalized *[]LocalizedString    `json:"display_name_localized,omitempty"`
}

type RangeCapability struct {
	ValueType string `json:"value_type"`
	Default   string `json:"default,omitempty"`
	Min       string `json:"min,omitempty"`
	Max       string `json:"max,omitempty"`
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

type TypedValueCapability struct {
	ValueType string `json:"value_type"` // enum
	Default   string `json:"default,omitempty"`
}

type Color struct {
	Option []ColorOption `json:"option"`
}

type ColorOption struct {
	VendorID                   string             `json:"vendor_id"`
	Type                       string             `json:"type"` // enum
	CustomDisplayName          string             `json:"custom_display_name,omitempty"`
	IsDefault                  bool               `json:"is_default"` // default = false
	CustomDisplayNameLocalized *[]LocalizedString `json:"custom_display_name_localized,omitempty"`
}

type Duplex struct {
	Option []DuplexOption `json:"option"`
}

type DuplexOption struct {
	Type      string `json:"type"`       // enum; default = "NO_DUPLEX"
	IsDefault bool   `json:"is_default"` // default = false
}

type PageOrientation struct {
	Option []PageOrientationOption `json:"option"`
}

type PageOrientationOption struct {
	Type      string `json:"type"`       // enum
	IsDefault bool   `json:"is_default"` // default = false
}

type Copies struct {
	Default int32 `json:"default"`
	Max     int32 `json:"max"`
}

type Margins struct {
	Option []MarginsOption `json:"option"`
}

type MarginsOption struct {
	Type          string `json:"type"` // enum
	TopMicrons    int32  `json:"top_microns"`
	RightMicrons  int32  `json:"right_microns"`
	BottomMicrons int32  `json:"bottom_microns"`
	LeftMicrons   int32  `json:"left_microns"`
	IsDefault     bool   `json:"is_default"` // default = false
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

type FitToPageOption struct {
	Type      string `json:"type"`       // enum
	IsDefault bool   `json:"is_default"` // default = false
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

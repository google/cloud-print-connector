/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package cdd

type CloudJobTicket struct {
	Version string             `json:"version"`
	Print   PrintTicketSection `json:"print"`
}

type PrintTicketSection struct {
	VendorTicketItem []VendorTicketItem         `json:"vendor_ticket_item"`
	Color            *ColorTicketItem           `json:"color"`
	Duplex           *DuplexTicketItem          `json:"duplex"`
	PageOrientation  *PageOrientationTicketItem `json:"page_orientation"`
	Copies           *CopiesTicketItem          `json:"copies"`
	Margins          *MarginsTicketItem         `json:"margins"`
	DPI              *DPITicketItem             `json:"dpi"`
	FitToPage        *FitToPageTicketItem       `json:"fit_to_page"`
	PageRange        *PageRangeTicketItem       `json:"page_range"`
	MediaSize        *MediaSizeTicketItem       `json:"media_size"`
	Collate          *CollateTicketItem         `json:"collate"`
	ReverseOrder     *ReverseOrderTicketItem    `json:"reverse_order"`
}

type VendorTicketItem struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type ColorTicketItem struct {
	VendorID string    `json:"vendor_id"`
	Type     ColorType `json:"type"`
}

type DuplexTicketItem struct {
	Type DuplexType `json:"type"`
}

type PageOrientationTicketItem struct {
	Type PageOrientationType `json:"type"`
}

type CopiesTicketItem struct {
	Copies int32 `json:"copies"`
}

type MarginsTicketItem struct {
	TopMicrons    int32 `json:"top_microns"`
	RightMicrons  int32 `json:"right_microns"`
	BottomMicrons int32 `json:"bottom_microns"`
	LeftMicrons   int32 `json:"left_microns"`
}

type DPITicketItem struct {
	HorizontalDPI int32  `json:"horizontal_dpi"`
	VerticalDPI   int32  `json:"vertical_dpi"`
	VendorID      string `json:"vendor_id"`
}

type FitToPageTicketItem struct {
	Type FitToPageType `json:"type"`
}

type PageRangeTicketItem struct {
	Interval []PageRangeInterval `json:"interval"`
}

type MediaSizeTicketItem struct {
	WidthMicrons     int32  `json:"width_microns"`
	HeightMicrons    int32  `json:"height_microns"`
	IsContinuousFeed bool   `json:"is_continuous_feed"` // default = false
	VendorID         string `json:"vendor_id"`
}

type CollateTicketItem struct {
	Collate bool `json:"collate"`
}

type ReverseOrderTicketItem struct {
	ReverseOrder bool `json:"reverse_order"`
}

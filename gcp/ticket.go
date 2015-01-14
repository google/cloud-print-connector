/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package gcp is the Google Cloud Print API client.
package gcp

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"strings"
)

type cloudJobTicket struct {
	Version string             `json:"version"`
	Print   printTicketSection `json:"print"`
}

type printTicketSection struct {
	VendorTicketItems []vendorTicketItem        `json:"vendor_ticket_item"`
	Color             colorTicketItem           `json:"color"`
	Duplex            duplexTicketItem          `json:"duplex"`
	PageOrientation   pageOrientationTicketItem `json:"page_orientation"`
	Copies            copiesTicketItem          `json:"copies"`
	Margins           marginsTicketItem         `json:"margins"`
	DPI               dpiTicketItem             `json:"dpi"`
	FitToPage         fitToPageTicketItem       `json:"fit_to_page"`
	PageRange         pageRangeTicketItem       `json:"page_range"`
	MediaSize         mediaSizeTicketItem       `json:"media_size"`
	Collate           collateTicketItem         `json:"collate"`
	ReverseOrder      reverseOrderTicketItem    `json:"reverse_order"`
}

type vendorTicketItem struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type colorTicketItem struct {
	VendorID string `json:"vendor_id"`
	Type     string `json:"type"`
}

type duplexTicketItem struct {
	Type string `json:"type"`
}

type pageOrientationTicketItem struct {
	Type string `json:"type"`
}

type copiesTicketItem struct {
	Copies int32 `json:"copies"`
}

type marginsTicketItem struct {
	TopMicrons    int32 `json:"top_microns"`
	RightMicrons  int32 `json:"right_microns"`
	BottomMicrons int32 `json:"bottom_microns"`
	LeftMicrons   int32 `json:"left_microns"`
}

type dpiTicketItem struct {
	HorizontalDPI int32  `json:"horizontal_dpi"`
	VerticalDPI   int32  `json:"vertical_dpi"`
	VendorID      string `json:"vendor_id"`
}

type fitToPageTicketItem struct {
	Type string `json:"type"`
}

type pageRangeTicketItem struct {
	Intervals []pageRangeInterval `json:"interval"`
}

type pageRangeInterval struct {
	Start int32 `json:"start"`
	End   int32 `json:"end"`
}

type mediaSizeTicketItem struct {
	WidthMicrons     int32  `json:"width_microns"`
	HeightMicrons    int32  `json:"height_microns"`
	IsContinuousFeed bool   `json:"is_continuous_feed"`
	VendorID         string `json:"vendor_id"`
}

type collateTicketItem struct {
	Collate bool `json:"collate"`
}

type reverseOrderTicketItem struct {
	ReverseOrder bool `json:"reverse_order"`
}

// Ticket gets a ticket, aka print job options.
func (gcp *GoogleCloudPrint) Ticket(ticketURL string) (map[string]string, error) {
	response, err := getWithRetry(gcp.robotTransport, ticketURL+"&use_cjt=true")
	if err != nil {
		return nil, err
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var ticket cloudJobTicket
	ticket.Print.Margins.TopMicrons = math.MinInt32
	ticket.Print.Margins.RightMicrons = math.MinInt32
	ticket.Print.Margins.BottomMicrons = math.MinInt32
	ticket.Print.Margins.LeftMicrons = math.MinInt32
	err = json.Unmarshal(responseBody, &ticket)
	if err != nil {
		return nil, err
	}

	return translateTicketToOptions(&ticket.Print)
}

// translateTicketToOptions converts a GCP Ticket object to a map of CUPS-compatible options.
func translateTicketToOptions(ticket *printTicketSection) (map[string]string, error) {
	m := make(map[string]string)

	for _, vti := range ticket.VendorTicketItems {
		m[vti.ID] = vti.Value
	}
	if ticket.Color.VendorID != "" {
		m["ColorModel"] = ticket.Color.VendorID
	}
	switch strings.ToUpper(ticket.Duplex.Type) {
	case "LONG_EDGE":
		m["Duplex"] = "DuplexNoTumble"
	case "SHORT_EDGE":
		m["Duplex"] = "DuplexTumble"
	}
	switch strings.ToUpper(ticket.PageOrientation.Type) {
	case "PORTRAIT":
		m["orientation-requested"] = "3"
	case "LANDSCAPE":
		m["orientation-requested"] = "4"
	}
	if ticket.Copies.Copies != 0 {
		m["copies"] = fmt.Sprintf("%d", ticket.Copies.Copies)
	}
	if ticket.Margins.TopMicrons != math.MinInt32 {
		m["page-top"] = fmt.Sprintf("%d", micronsToPoints(ticket.Margins.TopMicrons))
	}
	if ticket.Margins.RightMicrons != math.MinInt32 {
		m["page-right"] = fmt.Sprintf("%d", micronsToPoints(ticket.Margins.RightMicrons))
	}
	if ticket.Margins.BottomMicrons != math.MinInt32 {
		m["page-bottom"] = fmt.Sprintf("%d", micronsToPoints(ticket.Margins.BottomMicrons))
	}
	if ticket.Margins.LeftMicrons != math.MinInt32 {
		m["page-left"] = fmt.Sprintf("%d", micronsToPoints(ticket.Margins.LeftMicrons))
	}
	if ticket.DPI.VendorID != "" {
		m["Resolution"] = ticket.DPI.VendorID
	}
	switch ticket.FitToPage.Type {
	case "FIT_TO_PAGE":
		m["fit-to-page"] = "true"
	case "NO_FITTING":
		m["fit-to-page"] = "false"
	}
	if len(ticket.PageRange.Intervals) > 0 {
		pageRanges := make([]string, 0, len(ticket.PageRange.Intervals))
		for _, interval := range ticket.PageRange.Intervals {
			if interval.Start == interval.End {
				pageRanges = append(pageRanges, string(interval.Start))
			} else {
				pageRanges = append(pageRanges, fmt.Sprintf("%d-%d", interval.Start, interval.End))
			}
		}
		m["page-ranges"] = strings.Join(pageRanges, ",")
	}
	if ticket.MediaSize.VendorID != "" {
		m["media"] = ticket.MediaSize.VendorID
	}
	if ticket.Collate.Collate {
		m["Collate"] = "true"
	} else {
		m["Collate"] = "false"
	}
	if ticket.ReverseOrder.ReverseOrder {
		m["outputorder"] = "reverse"
	} else {
		m["outputorder"] = "normal"
	}

	return m, nil
}

func micronsToPoints(microns int32) int32 {
	return int32(float32(microns)*72/25400 + 0.5)
}

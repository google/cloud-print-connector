/*
Copyright 2015 Google Inc. All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file or at
https://developers.google.com/open-source/licenses/bsd
*/

package cups

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/cups-connector/cdd"
	"github.com/google/cups-connector/log"
)

const (
	ppdBoolean                 = "Boolean"
	ppdCMAndResolution         = "CMAndResolution"
	ppdCloseGroup              = "CloseGroup"
	ppdCloseSubGroup           = "CloseSubGroup"
	ppdCloseUI                 = "CloseUI"
	ppdColorModel              = "ColorModel"
	ppdDefault                 = "Default"
	ppdDuplex                  = "Duplex"
	ppdDuplexNoTumble          = "DuplexNoTumble"
	ppdDuplexTumble            = "DuplexTumble"
	ppdEnd                     = "End"
	ppdFalse                   = "False"
	ppdHWMargins               = "HWMargins"
	ppdInstallableOptions      = "InstallableOptions"
	ppdJCLCloseUI              = "JCLCloseUI"
	ppdJCLOpenUI               = "JCLOpenUI"
	ppdJobType                 = "JobType"
	ppdKMDuplex                = "KMDuplex"
	ppdKMDuplexBooklet         = "Booklet"
	ppdKMDuplexDouble          = "Double"
	ppdKMDuplexSingle          = "Single"
	ppdLockedPrint             = "LockedPrint"
	ppdLockedPrintPassword     = "LockedPrintPassword"
	ppdManufacturer            = "Manufacturer"
	ppdMediaType               = "MediaType"
	ppdNickName                = "NickName"
	ppdNone                    = "None"
	ppdOpenGroup               = "OpenGroup"
	ppdOpenSubGroup            = "OpenSubGroup"
	ppdOpenUI                  = "OpenUI"
	ppdOutputBin               = "OutputBin"
	ppdPageSize                = "PageSize"
	ppdPickMany                = "PickMany"
	ppdPickOne                 = "PickOne"
	ppdPrintQualityTranslation = "Print Quality"
	ppdResolution              = "Resolution"
	ppdSelectColor             = "SelectColor"
	ppdThroughput              = "Throughput"
	ppdTrue                    = "True"
	ppdUIConstraints           = "UIConstraints"

	// These characters are not allowed in PPD main keywords or option keywords,
	// so they are safe to use as separators in CDD strings.
	// A:B/C is interpreted as 2 CUPS/IPP options: A=B and C=[VendorTicketItem.Value].
	internalKeySeparator   = ":"
	internalValueSeparator = "/"
)

var (
	rLineSplit = regexp.MustCompile(`(?:\r\n|\r|\n)\*`)
	rStatement = regexp.MustCompile(`` +
		`^([^\s:/]+)` + // Main keyword; not optional.
		`(?:\s+([^/:]+))?` + // Option keyword.
		`(?:/([^:]*))?` + // Translation string.
		`(?::\s*(?:"([^"]*)"|(.*)))?\s*$`) // Value.
	rConstraint = regexp.MustCompile(`^\*([^\s\*]+)\s+(\S+)\s+\*([^\s\*]+)\s+(\S+)$`)
	// rModel is for removing superfluous information from *ModelName or *NickName.
	rModel *regexp.Regexp = regexp.MustCompile(`\s+(` +
		`(w/)?PS2?3?(\(P\))?(,\s+[0-9\.]+)?|` +
		`pcl3?(,\s+\d+(\.\d+))*|` +
		`-|PXL|PDF|cups-team|CUPS\+Gutenprint\s+v\S+|\(?recommended\)?|` +
		`(A4|Letter)(\+Duplex)?|` +
		`Post[Ss]cript|BR-Script2?3?J?|` +
		`v[0-9\.]+|` +
		`\(?KPDL(-2)?\)?|` +
		`Foomatic/\S+|Epson Inkjet Printer Driver \(ESC/P-R\) for \S+|` +
		`(hpcups|hpijs|HPLIP),?\s+\d+(\.\d+)*|requires proprietary plugin` +
		`)\s*$`)
	rPageSize              = regexp.MustCompile(`([\d.]+)(?:mm|in)?x([\d.]+)(mm|in)?`)
	rColor                 = regexp.MustCompile(`(?i)^(?:cmy|rgb|color)`)
	rGray                  = regexp.MustCompile(`(?i)^(?:gray|black|mono)`)
	rCMAndResolutionPrefix = regexp.MustCompile(`(?i)^(?:on|off)\s*-?\s*`)
	rResolution            = regexp.MustCompile(`^(\d+)(?:x(\d+))?dpi$`)
	rHWMargins             = regexp.MustCompile(`^(\d+)\s+(\d+)\s+(\d+)\s+(\d+)$`)

	ricohPasswordVendorID = fmt.Sprintf("%s%s%s%s%s",
		ppdJobType, internalKeySeparator, ppdLockedPrint, internalValueSeparator, ppdLockedPrintPassword)
	rRicohPasswordFormat = regexp.MustCompile(`^\d{4}$`)
)

// statement represents a PPD statement.
type statement struct {
	mainKeyword   string
	optionKeyword string
	translation   string
	value         string
}

type entryType uint8

const (
	entryTypePickOne entryType = iota
	entryTypeBoolean entryType = iota
)

// entry represents a PPD OpenUI entry.
type entry struct {
	mainKeyword  string
	translation  string
	entryType    entryType
	defaultValue string
	options      []statement
}

// translatePPD extracts a PrinterDescriptionSection, manufacturer string, and model string
// from a PPD string.
func translatePPD(ppd string) (*cdd.PrinterDescriptionSection, string, string) {
	statements := ppdToStatements(ppd)
	openUIStatements, installables, uiConstraints, standAlones := groupStatements(statements)
	openUIStatements = filterConstraints(openUIStatements, installables, uiConstraints)
	entriesByMainKeyword, entriesByTranslation := openUIStatementsToEntries(openUIStatements)

	pds := cdd.PrinterDescriptionSection{
		VendorCapability: &[]cdd.VendorCapability{},
	}
	if e, exists := entriesByMainKeyword[ppdPageSize]; exists {
		pds.MediaSize = convertMediaSize(e)
	}
	if e, exists := entriesByMainKeyword[ppdColorModel]; exists {
		pds.Color = convertColorPPD(e)
	} else if e, exists := entriesByMainKeyword[ppdCMAndResolution]; exists {
		pds.Color = convertColorPPD(e)
	} else if e, exists := entriesByMainKeyword[ppdSelectColor]; exists {
		pds.Color = convertColorPPD(e)
	}
	if e, exists := entriesByMainKeyword[ppdDuplex]; exists {
		pds.Duplex = convertDuplex(e)
	} else if e, exists := entriesByMainKeyword[ppdKMDuplex]; exists {
		pds.Duplex = convertDuplex(e)
	}
	if e, exists := entriesByMainKeyword[ppdResolution]; exists {
		pds.DPI = convertDPI(e)
	}
	if e, exists := entriesByMainKeyword[ppdOutputBin]; exists {
		*pds.VendorCapability = append(*pds.VendorCapability, *convertVendorCapability(e))
	}
	if jobType, exists := entriesByMainKeyword[ppdJobType]; exists {
		if lockedPrintPassword, exists := entriesByMainKeyword[ppdLockedPrintPassword]; exists {
			vc := convertRicohLockedPrintPassword(jobType, lockedPrintPassword)
			if vc != nil {
				*pds.VendorCapability = append(*pds.VendorCapability, *vc)
			}
		}
	}
	if e, exists := entriesByTranslation[ppdPrintQualityTranslation]; exists {
		*pds.VendorCapability = append(*pds.VendorCapability, *convertVendorCapability(e))
	}
	if len(*pds.VendorCapability) == 0 {
		// Don't generate invalid CDD JSON.
		pds.VendorCapability = nil
	}

	var manufacturer, model string
	for _, s := range standAlones {
		switch s.mainKeyword {
		case ppdManufacturer:
			manufacturer = s.value
		case ppdNickName:
			model = cleanupModel(s.value)
		case ppdHWMargins:
			pds.Margins = convertMargins(s.value)
		case ppdThroughput:
			pds.PrintingSpeed = convertPrintingSpeed(s.value, pds.Color)
		}
	}
	model = strings.TrimLeft(strings.TrimPrefix(model, manufacturer), " ")

	return &pds, manufacturer, model
}

// ppdToStatements converts a PPD file to a slice of statements.
func ppdToStatements(ppd string) []statement {
	var statements []statement
	for _, line := range rLineSplit.Split(ppd, -1) {
		if strings.HasPrefix(line, "%") || strings.HasPrefix(line, "?") {
			// Ignore comments and query statements.
			continue
		}
		found := rStatement.FindStringSubmatch(line)
		if found == nil {
			continue
		}

		mainKeyword, optionKeyword, translation := found[1], found[2], found[3]
		if mainKeyword == ppdEnd {
			// Ignore End statements.
			continue
		}

		found[4] = strings.TrimSpace(found[4])
		found[5] = strings.TrimSpace(found[5])
		var value string
		if found[4] != "" {
			value = found[4]
		} else {
			value = found[5]
		}
		statements = append(statements, statement{mainKeyword, optionKeyword, translation, value})
	}

	return statements
}

// groupStatements groups statements into:
//  1) OpenUI entries
//  2) InstallableOptions OpenUI entries
//  3) UIConstraints statements
//  4) other stand-alone statements
// Other Groups and SubGroups structures are thrown away.
func groupStatements(statements []statement) ([][]statement, [][]statement, []statement, []statement) {
	var openUIs, installables [][]statement
	var uiConstraints, standAlones []statement
	var insideOpenUI, insideInstallable bool

	for _, s := range statements {
		switch s.mainKeyword {
		case ppdOpenUI, ppdJCLOpenUI:
			insideOpenUI = true
			if insideInstallable {
				installables = append(installables, []statement{s})
			} else {
				openUIs = append(openUIs, []statement{s})
			}
		case ppdCloseUI, ppdJCLCloseUI:
			insideOpenUI = false
		case ppdOpenGroup:
			if strings.HasPrefix(s.value, ppdInstallableOptions) {
				insideInstallable = true
			}
		case ppdCloseGroup:
			if strings.HasPrefix(s.value, ppdInstallableOptions) {
				insideInstallable = false
			}
		case ppdOpenSubGroup:
		case ppdCloseSubGroup:
		case ppdUIConstraints:
			uiConstraints = append(uiConstraints, s)
		default:
			if insideInstallable {
				if len(installables) > 0 {
					installables[len(installables)-1] = append(installables[len(installables)-1], s)
				}
			} else if insideOpenUI {
				if len(openUIs) > 0 {
					openUIs[len(openUIs)-1] = append(openUIs[len(openUIs)-1], s)
				}
			} else {
				standAlones = append(standAlones, s)
			}
		}
	}

	return openUIs, installables, uiConstraints, standAlones
}

type pair struct {
	first  string
	second string
}

func filterConstraints(openUIs, installables [][]statement, uiConstraints []statement) [][]statement {
	installedDefaults := make(map[pair]struct{}, len(installables))
	for _, installable := range installables {
		for _, s := range installable {
			if strings.HasPrefix(s.mainKeyword, ppdDefault) {
				uiKeyword := strings.TrimPrefix(s.mainKeyword, ppdDefault)
				installedDefaults[pair{uiKeyword, s.value}] = struct{}{}
			}
		}
	}

	constraints := map[pair]struct{}{}
	for _, s := range uiConstraints {
		constraint := rConstraint.FindStringSubmatch(s.value)
		if constraint == nil || len(constraint) != 5 {
			continue
		}
		if _, exists := installedDefaults[pair{constraint[1], constraint[2]}]; exists {
			constraints[pair{constraint[3], constraint[4]}] = struct{}{}
		}
	}

	var newOpenUIs [][]statement
	for _, openUI := range openUIs {
		newOpenUI := make([]statement, 0, len(openUI))
		for _, s := range openUI {
			if _, exists := constraints[pair{s.mainKeyword, s.optionKeyword}]; !exists {
				newOpenUI = append(newOpenUI, s)
			}
		}
		newOpenUIs = append(newOpenUIs, newOpenUI)
	}

	return newOpenUIs
}

func openUIStatementsToEntries(statements [][]statement) (map[string]entry, map[string]entry) {
	byMainKeyword, byTranslation := make(map[string]entry), make(map[string]entry)

	for _, openUI := range statements {
		var e entry
		e.mainKeyword = strings.TrimPrefix(openUI[0].optionKeyword, "*")
		e.translation = openUI[0].translation
		if len(e.translation) < 1 {
			e.translation = e.mainKeyword
		}
		switch openUI[0].value {
		case ppdPickOne:
			e.entryType = entryTypePickOne
		case ppdBoolean:
			e.entryType = entryTypeBoolean
		case ppdPickMany:
			log.Warning("This PPD file contains a PickMany OpenUI entry, which is not supported")
			continue
		default:
			continue
		}

		optionValues := map[string]struct{}{}
		for _, s := range openUI {
			if strings.HasPrefix(s.mainKeyword, ppdDefault) {
				e.defaultValue = s.value
			} else if strings.HasPrefix(s.mainKeyword, e.mainKeyword) {
				e.options = append(e.options, s)
				optionValues[s.optionKeyword] = struct{}{}
			}
		}
		if len(e.options) < 1 {
			continue
		}
		if _, exists := optionValues[e.defaultValue]; !exists {
			e.defaultValue = e.options[0].value
		}

		byMainKeyword[e.mainKeyword] = e
		byTranslation[e.translation] = e
	}

	return byMainKeyword, byTranslation
}

func cleanupModel(model string) string {
	for {
		newModel := rModel.ReplaceAllString(model, "")
		// rModel looks for whitespace in the prefix. These don't have whitespace prefixes.
		newModel = strings.TrimSuffix(newModel, ",")
		newModel = strings.TrimSuffix(newModel, "(PS)")
		if model == newModel {
			break
		}
		model = newModel
	}
	return model
}

func convertMargins(hwMargins string) *cdd.Margins {
	found := rHWMargins.FindStringSubmatch(hwMargins)
	if found == nil {
		return nil
	}

	marginsType := cdd.MarginsBorderless

	marginsMicrons := make([]int32, 4)
	for i := 1; i < len(found); i++ {
		intValue, err := strconv.ParseInt(found[i], 10, 32)
		if err != nil {
			return nil
		}
		if intValue > 0 {
			marginsType = cdd.MarginsStandard
		}
		marginsMicrons[i-1] = pointsToMicrons(intValue)
	}

	// HWResolution format: left, bottom, right, top.
	return &cdd.Margins{
		[]cdd.MarginsOption{
			cdd.MarginsOption{
				Type:          marginsType,
				LeftMicrons:   marginsMicrons[0],
				BottomMicrons: marginsMicrons[1],
				RightMicrons:  marginsMicrons[2],
				TopMicrons:    marginsMicrons[3],
				IsDefault:     true,
			},
		},
	}
}

func convertPrintingSpeed(throughput string, color *cdd.Color) *cdd.PrintingSpeed {
	speedPPM, err := strconv.ParseInt(throughput, 10, 32)
	if err != nil {
		return nil
	}
	var colorTypes *[]cdd.ColorType
	if color != nil {
		mColorTypes := make(map[cdd.ColorType]struct{}, len(color.Option))
		for _, co := range color.Option {
			mColorTypes[co.Type] = struct{}{}
		}
		ct := make([]cdd.ColorType, 0, len(mColorTypes))
		for colorType := range mColorTypes {
			ct = append(ct, colorType)
		}
		colorTypes = &ct
	}
	return &cdd.PrintingSpeed{
		[]cdd.PrintingSpeedOption{
			cdd.PrintingSpeedOption{
				SpeedPPM:  float32(speedPPM),
				ColorType: colorTypes,
			},
		},
	}
}

func cleanupColorName(colorValue, colorName string) string {
	newColorName := rCMAndResolutionPrefix.ReplaceAllString(colorName, "")
	if colorName == newColorName {
		return colorName
	}

	if rGray.MatchString(colorValue) || rGray.MatchString(newColorName) {
		if len(newColorName) > 0 {
			return "Gray, " + newColorName
		} else {
			return "Gray"
		}
	}
	if rColor.MatchString(colorValue) || rColor.MatchString(newColorName) {
		if len(newColorName) > 0 {
			return "Color, " + newColorName
		} else {
			return "Color"
		}
	}
	return newColorName
}

func convertColorPPD(e entry) *cdd.Color {
	var colorOptions, grayOptions, otherOptions []statement

	for _, o := range e.options {
		if rGray.MatchString(o.optionKeyword) {
			grayOptions = append(grayOptions, o)
		} else if rColor.MatchString(o.optionKeyword) {
			colorOptions = append(colorOptions, o)
		} else {
			otherOptions = append(otherOptions, o)
		}
	}

	c := cdd.Color{VendorKey: e.mainKeyword}
	if len(colorOptions) == 1 {
		colorName := cleanupColorName(colorOptions[0].optionKeyword, colorOptions[0].translation)
		co := cdd.ColorOption{
			VendorID: colorOptions[0].optionKeyword,
			Type:     cdd.ColorTypeStandardColor,
			CustomDisplayNameLocalized: cdd.NewLocalizedString(colorName),
		}
		c.Option = append(c.Option, co)
	} else {
		for _, o := range colorOptions {
			colorName := cleanupColorName(o.optionKeyword, o.translation)
			co := cdd.ColorOption{
				VendorID: o.optionKeyword,
				Type:     cdd.ColorTypeCustomColor,
				CustomDisplayNameLocalized: cdd.NewLocalizedString(colorName),
			}
			c.Option = append(c.Option, co)
		}
	}

	if len(grayOptions) == 1 {
		colorName := cleanupColorName(grayOptions[0].optionKeyword, grayOptions[0].translation)
		co := cdd.ColorOption{
			VendorID: grayOptions[0].optionKeyword,
			Type:     cdd.ColorTypeStandardMonochrome,
			CustomDisplayNameLocalized: cdd.NewLocalizedString(colorName),
		}
		c.Option = append(c.Option, co)
	} else {
		for _, o := range grayOptions {
			colorName := cleanupColorName(o.optionKeyword, o.translation)
			co := cdd.ColorOption{
				VendorID: o.optionKeyword,
				Type:     cdd.ColorTypeCustomMonochrome,
				CustomDisplayNameLocalized: cdd.NewLocalizedString(colorName),
			}
			c.Option = append(c.Option, co)
		}
	}

	for _, o := range otherOptions {
		colorName := cleanupColorName(o.optionKeyword, o.translation)
		co := cdd.ColorOption{
			VendorID: o.optionKeyword,
			Type:     cdd.ColorTypeCustomMonochrome,
			CustomDisplayNameLocalized: cdd.NewLocalizedString(colorName),
		}
		c.Option = append(c.Option, co)
	}

	if len(c.Option) == 0 {
		return nil
	}

	for i := range c.Option {
		if c.Option[i].VendorID == e.defaultValue {
			c.Option[i].IsDefault = true
			return &c
		}
	}

	c.Option[0].IsDefault = true
	return &c
}

func convertDuplex(e entry) *cdd.Duplex {
	d := cdd.Duplex{VendorKey: e.mainKeyword}

	var foundDefault bool
	for _, o := range e.options {
		def := o.optionKeyword == e.defaultValue
		switch o.optionKeyword {
		case ppdNone, ppdFalse, ppdKMDuplexSingle:
			d.Option = append(d.Option, cdd.DuplexOption{cdd.DuplexNoDuplex, def, o.optionKeyword})
			foundDefault = foundDefault || def
		case ppdDuplexNoTumble, ppdTrue, ppdKMDuplexDouble:
			d.Option = append(d.Option, cdd.DuplexOption{cdd.DuplexLongEdge, def, o.optionKeyword})
			foundDefault = foundDefault || def
		case ppdDuplexTumble, ppdKMDuplexBooklet:
			d.Option = append(d.Option, cdd.DuplexOption{cdd.DuplexShortEdge, def, o.optionKeyword})
			foundDefault = foundDefault || def
		default:
			if strings.HasPrefix(o.optionKeyword, "1") {
				d.Option = append(d.Option, cdd.DuplexOption{cdd.DuplexNoDuplex, def, o.optionKeyword})
				foundDefault = foundDefault || def
			} else if strings.HasPrefix(o.optionKeyword, "2") {
				d.Option = append(d.Option, cdd.DuplexOption{cdd.DuplexLongEdge, def, o.optionKeyword})
				foundDefault = foundDefault || def
			}
		}
	}

	if len(d.Option) == 0 {
		return nil
	}

	if !foundDefault {
		d.Option[0].IsDefault = true
	}
	return &d
}

func convertDPI(e entry) *cdd.DPI {
	d := cdd.DPI{}
	for _, o := range e.options {
		found := rResolution.FindStringSubmatch(o.optionKeyword)
		if found == nil {
			continue
		}
		h, err := strconv.ParseInt(found[1], 10, 32)
		if err != nil {
			continue
		}
		v, err := strconv.ParseInt(found[2], 10, 32)
		if err != nil {
			v = h
		}
		do := cdd.DPIOption{
			HorizontalDPI:              int32(h),
			VerticalDPI:                int32(v),
			VendorID:                   o.optionKeyword,
			CustomDisplayNameLocalized: cdd.NewLocalizedString(o.translation),
		}
		d.Option = append(d.Option, do)
	}

	if len(d.Option) == 0 {
		return nil
	}

	for i := range d.Option {
		if d.Option[i].VendorID == e.defaultValue {
			d.Option[i].IsDefault = true
			return &d
		}
	}

	d.Option[0].IsDefault = true
	return &d
}

// Convert 2 entries, JobType and LockedPrintPassword, to one CDD VendorCapability.
// http://www.linuxfoundation.org/collaborate/workgroups/openprinting/databasericohfaq#What_is_JobType_.22Locked_Print.22.3F_How_do_I_use_it.3F
func convertRicohLockedPrintPassword(jobType, lockedPrintPassword entry) *cdd.VendorCapability {
	var foundLockedPrint bool
	for _, s := range jobType.options {
		if s.optionKeyword == ppdLockedPrint {
			foundLockedPrint = true
			break
		}
	}
	if !foundLockedPrint {
		return nil
	}

	return &cdd.VendorCapability{
		ID:   ricohPasswordVendorID,
		Type: cdd.VendorCapabilityTypedValue,
		TypedValueCap: &cdd.TypedValueCapability{
			ValueType: cdd.TypedValueCapabilityTypeString,
		},
		DisplayNameLocalized: cdd.NewLocalizedString("Password (4 numbers)"),
	}
}

func convertVendorCapability(e entry) *cdd.VendorCapability {
	vc := cdd.VendorCapability{
		ID:                   e.mainKeyword,
		DisplayNameLocalized: cdd.NewLocalizedString(e.translation),
	}

	if e.entryType == entryTypePickOne {
		vc.Type = cdd.VendorCapabilitySelect
		vc.SelectCap = &cdd.SelectCapability{}

		var foundDefault bool
		for _, o := range e.options {
			sco := cdd.SelectCapabilityOption{
				Value:                o.optionKeyword,
				DisplayNameLocalized: cdd.NewLocalizedString(o.translation),
			}
			if e.defaultValue == o.optionKeyword {
				foundDefault = true
				sco.IsDefault = true
			}
			vc.SelectCap.Option = append(vc.SelectCap.Option, sco)
		}

		if !foundDefault {
			vc.SelectCap.Option[0].IsDefault = true
		}

	} else {
		vc.Type = cdd.VendorCapabilityTypedValue
		vc.TypedValueCap = &cdd.TypedValueCapability{
			ValueType: cdd.TypedValueCapabilityTypeBoolean,
			Default:   e.defaultValue,
		}
	}

	return &vc
}

func convertMediaSize(e entry) *cdd.MediaSize {
	foundDefault := false
	ms := cdd.MediaSize{}
	for _, option := range e.options {
		if strings.HasSuffix(option.optionKeyword, ".FullBleed") {
			continue
		}

		var o cdd.MediaSizeOption
		var exists bool
		if o, exists = ppdMediaSizes[option.optionKeyword]; !exists {
			op := getCustomMediaSizeOption(option.optionKeyword, option.translation)
			if op == nil {
				// Failed to make a custom media size.
				continue
			}
			o = *op
		}

		if e.defaultValue == option.optionKeyword {
			o.IsDefault = true
			foundDefault = true
		}
		ms.Option = append(ms.Option, o)
	}

	if len(ms.Option) == 0 {
		return nil
	}

	if !foundDefault {
		ms.Option[0].IsDefault = true
	}
	return &ms
}

func getCustomMediaSizeOption(optionKeyword, translation string) *cdd.MediaSizeOption {
	found := rPageSize.FindStringSubmatch(optionKeyword)
	if found == nil {
		found = rPageSize.FindStringSubmatch(translation)
	}
	if found == nil {
		return nil
	}

	width, err := strconv.ParseFloat(found[1], 32)
	if err != nil {
		return nil
	}
	height, err := strconv.ParseFloat(found[2], 32)
	if err != nil {
		return nil
	}

	if found[3] == "mm" {
		return &cdd.MediaSizeOption{
			Name:                       cdd.MediaSizeCustom,
			WidthMicrons:               mmToMicrons(float32(width)),
			HeightMicrons:              mmToMicrons(float32(height)),
			VendorID:                   optionKeyword,
			CustomDisplayNameLocalized: cdd.NewLocalizedString(translation),
		}
	} else {
		return &cdd.MediaSizeOption{
			Name:                       cdd.MediaSizeCustom,
			WidthMicrons:               inchesToMicrons(float32(width)),
			HeightMicrons:              inchesToMicrons(float32(height)),
			VendorID:                   optionKeyword,
			CustomDisplayNameLocalized: cdd.NewLocalizedString(translation),
		}
	}
}

func inchesToMicrons(inches float32) int32 {
	return int32(inches*25400 + 0.5)
}

func mmToMicrons(mm float32) int32 {
	return int32(mm*1000 + 0.5)
}

func pointsToMicrons(points int64) int32 {
	return int32(float32(points*25400)/72 + 0.5)
}

var ppdMediaSizes = map[string]cdd.MediaSizeOption{
	"3x5":                   {Name: cdd.MediaSizeNAIndex3x5, WidthMicrons: inchesToMicrons(3), HeightMicrons: inchesToMicrons(5), VendorID: "3x5", CustomDisplayNameLocalized: cdd.NewLocalizedString("3x5")},
	"4x6":                   {Name: cdd.MediaSizeNAIndex4x6, WidthMicrons: inchesToMicrons(4), HeightMicrons: inchesToMicrons(6), VendorID: "4x6", CustomDisplayNameLocalized: cdd.NewLocalizedString("4x6")},
	"5x7":                   {Name: cdd.MediaSizeNA5x7, WidthMicrons: inchesToMicrons(5), HeightMicrons: inchesToMicrons(7), VendorID: "5x7", CustomDisplayNameLocalized: cdd.NewLocalizedString("5x7")},
	"5x8":                   {Name: cdd.MediaSizeNAIndex5x8, WidthMicrons: inchesToMicrons(5), HeightMicrons: inchesToMicrons(8), VendorID: "5x8", CustomDisplayNameLocalized: cdd.NewLocalizedString("5x8")},
	"6x9":                   {Name: cdd.MediaSizeNA6x9, WidthMicrons: inchesToMicrons(6), HeightMicrons: inchesToMicrons(9), VendorID: "6x9", CustomDisplayNameLocalized: cdd.NewLocalizedString("6x9")},
	"6.5x9.5":               {Name: cdd.MediaSizeNAC5, WidthMicrons: inchesToMicrons(6.5), HeightMicrons: inchesToMicrons(9.5), VendorID: "6.5x9.5", CustomDisplayNameLocalized: cdd.NewLocalizedString("6.5x9.5")},
	"7x9":                   {Name: cdd.MediaSizeNA7x9, WidthMicrons: inchesToMicrons(7), HeightMicrons: inchesToMicrons(9), VendorID: "7x9", CustomDisplayNameLocalized: cdd.NewLocalizedString("7x9")},
	"8x10":                  {Name: cdd.MediaSizeNAGovtLetter, WidthMicrons: inchesToMicrons(8), HeightMicrons: inchesToMicrons(10), VendorID: "8x10", CustomDisplayNameLocalized: cdd.NewLocalizedString("8x10")},
	"8x13":                  {Name: cdd.MediaSizeNAGovtLegal, WidthMicrons: inchesToMicrons(8), HeightMicrons: inchesToMicrons(13), VendorID: "8x13", CustomDisplayNameLocalized: cdd.NewLocalizedString("8x13")},
	"9x11":                  {Name: cdd.MediaSizeNA9x11, WidthMicrons: inchesToMicrons(9), HeightMicrons: inchesToMicrons(11), VendorID: "9x11", CustomDisplayNameLocalized: cdd.NewLocalizedString("9x11")},
	"10x11":                 {Name: cdd.MediaSizeNA10x11, WidthMicrons: inchesToMicrons(10), HeightMicrons: inchesToMicrons(11), VendorID: "10x11", CustomDisplayNameLocalized: cdd.NewLocalizedString("10x11")},
	"10x13":                 {Name: cdd.MediaSizeNA10x13, WidthMicrons: inchesToMicrons(10), HeightMicrons: inchesToMicrons(13), VendorID: "10x13", CustomDisplayNameLocalized: cdd.NewLocalizedString("10x13")},
	"10x14":                 {Name: cdd.MediaSizeNA10x14, WidthMicrons: inchesToMicrons(10), HeightMicrons: inchesToMicrons(14), VendorID: "10x14", CustomDisplayNameLocalized: cdd.NewLocalizedString("10x14")},
	"10x15":                 {Name: cdd.MediaSizeNA10x15, WidthMicrons: inchesToMicrons(10), HeightMicrons: inchesToMicrons(15), VendorID: "10x15", CustomDisplayNameLocalized: cdd.NewLocalizedString("10x15")},
	"11x12":                 {Name: cdd.MediaSizeNA11x12, WidthMicrons: inchesToMicrons(11), HeightMicrons: inchesToMicrons(12), VendorID: "11x12", CustomDisplayNameLocalized: cdd.NewLocalizedString("11x12")},
	"11x14":                 {Name: cdd.MediaSizeNAEDP, WidthMicrons: inchesToMicrons(11), HeightMicrons: inchesToMicrons(14), VendorID: "11x14", CustomDisplayNameLocalized: cdd.NewLocalizedString("11x14")},
	"11x15":                 {Name: cdd.MediaSizeNA11x15, WidthMicrons: inchesToMicrons(11), HeightMicrons: inchesToMicrons(15), VendorID: "11x15", CustomDisplayNameLocalized: cdd.NewLocalizedString("11x15")},
	"11x17":                 {Name: cdd.MediaSizeNALedger, WidthMicrons: inchesToMicrons(11), HeightMicrons: inchesToMicrons(17), VendorID: "11x17", CustomDisplayNameLocalized: cdd.NewLocalizedString("11x17")},
	"12x18":                 {Name: cdd.MediaSizeNAArchB, WidthMicrons: inchesToMicrons(12), HeightMicrons: inchesToMicrons(18), VendorID: "12x18", CustomDisplayNameLocalized: cdd.NewLocalizedString("12x18")},
	"12x19":                 {Name: cdd.MediaSizeNA12x19, WidthMicrons: inchesToMicrons(12), HeightMicrons: inchesToMicrons(19), VendorID: "12x19", CustomDisplayNameLocalized: cdd.NewLocalizedString("12x19")},
	"13x19":                 {Name: cdd.MediaSizeNASuperB, WidthMicrons: inchesToMicrons(13), HeightMicrons: inchesToMicrons(19), VendorID: "13x19", CustomDisplayNameLocalized: cdd.NewLocalizedString("13x19")},
	"EnvPersonal":           {Name: cdd.MediaSizeNAPersonal, WidthMicrons: inchesToMicrons(3.625), HeightMicrons: inchesToMicrons(6.5), VendorID: "EnvPersonal", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvPersonal")},
	"Monarch":               {Name: cdd.MediaSizeNAMonarch, WidthMicrons: inchesToMicrons(3.875), HeightMicrons: inchesToMicrons(7.5), VendorID: "Monarch", CustomDisplayNameLocalized: cdd.NewLocalizedString("Monarch")},
	"EnvMonarch":            {Name: cdd.MediaSizeNAMonarch, WidthMicrons: inchesToMicrons(3.875), HeightMicrons: inchesToMicrons(7.5), VendorID: "EnvMonarch", CustomDisplayNameLocalized: cdd.NewLocalizedString("Monarch")},
	"Comm10":                {Name: cdd.MediaSizeNANumber10, WidthMicrons: inchesToMicrons(4.125), HeightMicrons: inchesToMicrons(9.5), VendorID: "Comm10", CustomDisplayNameLocalized: cdd.NewLocalizedString("Comm10")},
	"EnvA2":                 {Name: cdd.MediaSizeNAA2, WidthMicrons: inchesToMicrons(4.375), HeightMicrons: inchesToMicrons(5.75), VendorID: "EnvA2", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvA2")},
	"Env9":                  {Name: cdd.MediaSizeNANumber9, WidthMicrons: inchesToMicrons(3.875), HeightMicrons: inchesToMicrons(8.875), VendorID: "Env9", CustomDisplayNameLocalized: cdd.NewLocalizedString("Env9")},
	"Env10":                 {Name: cdd.MediaSizeNANumber10, WidthMicrons: inchesToMicrons(4.125), HeightMicrons: inchesToMicrons(9.5), VendorID: "Env10", CustomDisplayNameLocalized: cdd.NewLocalizedString("Env10")},
	"Env11":                 {Name: cdd.MediaSizeNANumber11, WidthMicrons: inchesToMicrons(4.5), HeightMicrons: inchesToMicrons(10.375), VendorID: "Env11", CustomDisplayNameLocalized: cdd.NewLocalizedString("Env11")},
	"Env12":                 {Name: cdd.MediaSizeNANumber12, WidthMicrons: inchesToMicrons(4.75), HeightMicrons: inchesToMicrons(11), VendorID: "Env12", CustomDisplayNameLocalized: cdd.NewLocalizedString("Env12")},
	"Env14":                 {Name: cdd.MediaSizeNANumber14, WidthMicrons: inchesToMicrons(5), HeightMicrons: inchesToMicrons(11.5), VendorID: "Env14", CustomDisplayNameLocalized: cdd.NewLocalizedString("Env14")},
	"Statement":             {Name: cdd.MediaSizeNAInvoice, WidthMicrons: inchesToMicrons(5.5), HeightMicrons: inchesToMicrons(8.5), VendorID: "Statement", CustomDisplayNameLocalized: cdd.NewLocalizedString("Statement")},
	"Executive":             {Name: cdd.MediaSizeNAExecutive, WidthMicrons: inchesToMicrons(7.25), HeightMicrons: inchesToMicrons(10.5), VendorID: "Executive", CustomDisplayNameLocalized: cdd.NewLocalizedString("Executive")},
	"Quarto":                {Name: cdd.MediaSizeNAQuarto, WidthMicrons: inchesToMicrons(8.5), HeightMicrons: inchesToMicrons(10.83), VendorID: "Quarto", CustomDisplayNameLocalized: cdd.NewLocalizedString("Quarto")},
	"EngQuatro":             {Name: cdd.MediaSizeCustom, WidthMicrons: inchesToMicrons(8), HeightMicrons: inchesToMicrons(10), VendorID: "EngQuatro", CustomDisplayNameLocalized: cdd.NewLocalizedString("English Quatro 8x10")},
	"Letter":                {Name: cdd.MediaSizeNALetter, WidthMicrons: inchesToMicrons(8.5), HeightMicrons: inchesToMicrons(11), VendorID: "Letter", CustomDisplayNameLocalized: cdd.NewLocalizedString("Letter")},
	"LetterExtra":           {Name: cdd.MediaSizeNALetterExtra, WidthMicrons: inchesToMicrons(9.5), HeightMicrons: inchesToMicrons(12), VendorID: "LetterExtra", CustomDisplayNameLocalized: cdd.NewLocalizedString("Letter Extra")},
	"LetterPlus":            {Name: cdd.MediaSizeNALetterPlus, WidthMicrons: inchesToMicrons(8.5), HeightMicrons: inchesToMicrons(12.69), VendorID: "LetterPlus", CustomDisplayNameLocalized: cdd.NewLocalizedString("Letter Plus")},
	"Legal":                 {Name: cdd.MediaSizeNALegal, WidthMicrons: inchesToMicrons(8.5), HeightMicrons: inchesToMicrons(14), VendorID: "Legal", CustomDisplayNameLocalized: cdd.NewLocalizedString("Legal")},
	"LegalExtra":            {Name: cdd.MediaSizeNALegalExtra, WidthMicrons: inchesToMicrons(9.5), HeightMicrons: inchesToMicrons(15), VendorID: "LegalExtra", CustomDisplayNameLocalized: cdd.NewLocalizedString("Legal Extra")},
	"FanFoldGerman":         {Name: cdd.MediaSizeNAFanfoldEur, WidthMicrons: inchesToMicrons(8.5), HeightMicrons: inchesToMicrons(12), VendorID: "FanFoldGerman", CustomDisplayNameLocalized: cdd.NewLocalizedString("FanFoldGerman")},
	"Foolscap":              {Name: cdd.MediaSizeNAFoolscap, WidthMicrons: inchesToMicrons(8.5), HeightMicrons: inchesToMicrons(13), VendorID: "Foolscap", CustomDisplayNameLocalized: cdd.NewLocalizedString("Foolscap")},
	"FanFoldGermanLegal":    {Name: cdd.MediaSizeNAFoolscap, WidthMicrons: inchesToMicrons(8.5), HeightMicrons: inchesToMicrons(13), VendorID: "FanFoldGermanLegal", CustomDisplayNameLocalized: cdd.NewLocalizedString("Fan Fold German Legal")},
	"GovernmentLG":          {Name: cdd.MediaSizeNAFoolscap, WidthMicrons: inchesToMicrons(8.5), HeightMicrons: inchesToMicrons(13), VendorID: "GovernmentLG", CustomDisplayNameLocalized: cdd.NewLocalizedString("GovernmentLG")},
	"SuperA":                {Name: cdd.MediaSizeNASuperA, WidthMicrons: inchesToMicrons(8.94), HeightMicrons: inchesToMicrons(14), VendorID: "SuperA", CustomDisplayNameLocalized: cdd.NewLocalizedString("Super A")},
	"SuperB":                {Name: cdd.MediaSizeNABPlus, WidthMicrons: inchesToMicrons(12), HeightMicrons: inchesToMicrons(19.17), VendorID: "SuperB", CustomDisplayNameLocalized: cdd.NewLocalizedString("Super B")},
	"Tabloid":               {Name: cdd.MediaSizeNALedger, WidthMicrons: inchesToMicrons(11), HeightMicrons: inchesToMicrons(17), VendorID: "Tabloid", CustomDisplayNameLocalized: cdd.NewLocalizedString("Tabloid")},
	"Ledger":                {Name: cdd.MediaSizeNALedger, WidthMicrons: inchesToMicrons(11), HeightMicrons: inchesToMicrons(17), VendorID: "Ledger", CustomDisplayNameLocalized: cdd.NewLocalizedString("Ledger")},
	"ARCHA":                 {Name: cdd.MediaSizeNAArchA, WidthMicrons: inchesToMicrons(9), HeightMicrons: inchesToMicrons(12), VendorID: "ARCHA", CustomDisplayNameLocalized: cdd.NewLocalizedString("Arch A")},
	"ARCHB":                 {Name: cdd.MediaSizeNAArchB, WidthMicrons: inchesToMicrons(12), HeightMicrons: inchesToMicrons(18), VendorID: "ARCHB", CustomDisplayNameLocalized: cdd.NewLocalizedString("Arch B")},
	"ARCHC":                 {Name: cdd.MediaSizeNAArchC, WidthMicrons: inchesToMicrons(18), HeightMicrons: inchesToMicrons(24), VendorID: "ARCHC", CustomDisplayNameLocalized: cdd.NewLocalizedString("Arch C")},
	"ARCHD":                 {Name: cdd.MediaSizeNAArchD, WidthMicrons: inchesToMicrons(24), HeightMicrons: inchesToMicrons(36), VendorID: "ARCHD", CustomDisplayNameLocalized: cdd.NewLocalizedString("Arch D")},
	"ARCHE":                 {Name: cdd.MediaSizeNAArchE, WidthMicrons: inchesToMicrons(36), HeightMicrons: inchesToMicrons(48), VendorID: "ARCHE", CustomDisplayNameLocalized: cdd.NewLocalizedString("Arch E")},
	"AnsiC":                 {Name: cdd.MediaSizeNAC, WidthMicrons: inchesToMicrons(17), HeightMicrons: inchesToMicrons(22), VendorID: "AnsiC", CustomDisplayNameLocalized: cdd.NewLocalizedString("ANSI C")},
	"AnsiD":                 {Name: cdd.MediaSizeNAD, WidthMicrons: inchesToMicrons(22), HeightMicrons: inchesToMicrons(34), VendorID: "AnsiD", CustomDisplayNameLocalized: cdd.NewLocalizedString("ANSI D")},
	"AnsiE":                 {Name: cdd.MediaSizeNAE, WidthMicrons: inchesToMicrons(34), HeightMicrons: inchesToMicrons(44), VendorID: "AnsiE", CustomDisplayNameLocalized: cdd.NewLocalizedString("ANSI E")},
	"AnsiF":                 {Name: cdd.MediaSizeNAF, WidthMicrons: inchesToMicrons(44), HeightMicrons: inchesToMicrons(68), VendorID: "AnsiF", CustomDisplayNameLocalized: cdd.NewLocalizedString("ANSI F")},
	"F":                     {Name: cdd.MediaSizeNAF, WidthMicrons: inchesToMicrons(44), HeightMicrons: inchesToMicrons(68), VendorID: "F", CustomDisplayNameLocalized: cdd.NewLocalizedString("ANSI F")},
	"roc16k":                {Name: cdd.MediaSizeROC16k, WidthMicrons: inchesToMicrons(7.75), HeightMicrons: inchesToMicrons(10.75), VendorID: "roc16k", CustomDisplayNameLocalized: cdd.NewLocalizedString("16K (ROC)")},
	"roc8k":                 {Name: cdd.MediaSizeROC8k, WidthMicrons: inchesToMicrons(10.75), HeightMicrons: inchesToMicrons(15.5), VendorID: "roc8k", CustomDisplayNameLocalized: cdd.NewLocalizedString("8K (ROC)")},
	"PRC32K":                {Name: cdd.MediaSizePRC32k, WidthMicrons: mmToMicrons(97), HeightMicrons: mmToMicrons(151), VendorID: "PRC32K", CustomDisplayNameLocalized: cdd.NewLocalizedString("32K (PRC)")},
	"EnvPRC1":               {Name: cdd.MediaSizePRC1, WidthMicrons: mmToMicrons(102), HeightMicrons: mmToMicrons(165), VendorID: "EnvPRC1", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvPRC1")},
	"EnvPRC2":               {Name: cdd.MediaSizePRC2, WidthMicrons: mmToMicrons(102), HeightMicrons: mmToMicrons(176), VendorID: "EnvPRC2", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvPRC2")},
	"EnvPRC4":               {Name: cdd.MediaSizePRC4, WidthMicrons: mmToMicrons(110), HeightMicrons: mmToMicrons(208), VendorID: "EnvPRC4", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvPRC4")},
	"EnvPRC5":               {Name: cdd.MediaSizePRC5, WidthMicrons: mmToMicrons(110), HeightMicrons: mmToMicrons(220), VendorID: "EnvPRC5", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvPRC5")},
	"EnvPRC8":               {Name: cdd.MediaSizePRC8, WidthMicrons: mmToMicrons(120), HeightMicrons: mmToMicrons(309), VendorID: "EnvPRC8", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvPRC8")},
	"EnvPRC6":               {Name: cdd.MediaSizePRC6, WidthMicrons: mmToMicrons(120), HeightMicrons: mmToMicrons(230), VendorID: "EnvPRC6", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvPRC6")},
	"EnvPRC3":               {Name: cdd.MediaSizePRC3, WidthMicrons: mmToMicrons(125), HeightMicrons: mmToMicrons(176), VendorID: "EnvPRC3", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvPRC3")},
	"PRC16K":                {Name: cdd.MediaSizePRC16k, WidthMicrons: mmToMicrons(146), HeightMicrons: mmToMicrons(215), VendorID: "PRC16K", CustomDisplayNameLocalized: cdd.NewLocalizedString("PRC16K")},
	"EnvPRC7":               {Name: cdd.MediaSizePRC7, WidthMicrons: mmToMicrons(160), HeightMicrons: mmToMicrons(230), VendorID: "EnvPRC7", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvPRC7")},
	"A0":                    {Name: cdd.MediaSizeISOA0, WidthMicrons: mmToMicrons(841), HeightMicrons: mmToMicrons(1189), VendorID: "A0", CustomDisplayNameLocalized: cdd.NewLocalizedString("A0")},
	"A1":                    {Name: cdd.MediaSizeISOA1, WidthMicrons: mmToMicrons(594), HeightMicrons: mmToMicrons(841), VendorID: "A1", CustomDisplayNameLocalized: cdd.NewLocalizedString("A1")},
	"A2":                    {Name: cdd.MediaSizeISOA2, WidthMicrons: mmToMicrons(420), HeightMicrons: mmToMicrons(594), VendorID: "A2", CustomDisplayNameLocalized: cdd.NewLocalizedString("A2")},
	"A3":                    {Name: cdd.MediaSizeISOA3, WidthMicrons: mmToMicrons(297), HeightMicrons: mmToMicrons(420), VendorID: "A3", CustomDisplayNameLocalized: cdd.NewLocalizedString("A3")},
	"A3Extra":               {Name: cdd.MediaSizeISOA3Extra, WidthMicrons: mmToMicrons(322), HeightMicrons: mmToMicrons(445), VendorID: "A3Extra", CustomDisplayNameLocalized: cdd.NewLocalizedString("A3 Extra")},
	"A4":                    {Name: cdd.MediaSizeISOA4, WidthMicrons: mmToMicrons(210), HeightMicrons: mmToMicrons(297), VendorID: "A4", CustomDisplayNameLocalized: cdd.NewLocalizedString("A4")},
	"A4Extra":               {Name: cdd.MediaSizeISOA4Extra, WidthMicrons: mmToMicrons(235.5), HeightMicrons: mmToMicrons(322.3), VendorID: "A4Extra", CustomDisplayNameLocalized: cdd.NewLocalizedString("A4 Extra")},
	"A4Tab":                 {Name: cdd.MediaSizeISOA4Tab, WidthMicrons: mmToMicrons(225), HeightMicrons: mmToMicrons(297), VendorID: "A4Tab", CustomDisplayNameLocalized: cdd.NewLocalizedString("A4 Tab")},
	"A5":                    {Name: cdd.MediaSizeISOA5, WidthMicrons: mmToMicrons(148), HeightMicrons: mmToMicrons(210), VendorID: "A5", CustomDisplayNameLocalized: cdd.NewLocalizedString("A5")},
	"A5Extra":               {Name: cdd.MediaSizeISOA5Extra, WidthMicrons: mmToMicrons(174), HeightMicrons: mmToMicrons(235), VendorID: "A5Extra", CustomDisplayNameLocalized: cdd.NewLocalizedString("A5 Extra")},
	"A6":                    {Name: cdd.MediaSizeISOA6, WidthMicrons: mmToMicrons(105), HeightMicrons: mmToMicrons(148), VendorID: "A6", CustomDisplayNameLocalized: cdd.NewLocalizedString("A6")},
	"A7":                    {Name: cdd.MediaSizeISOA7, WidthMicrons: mmToMicrons(74), HeightMicrons: mmToMicrons(105), VendorID: "A7", CustomDisplayNameLocalized: cdd.NewLocalizedString("A7")},
	"A8":                    {Name: cdd.MediaSizeISOA8, WidthMicrons: mmToMicrons(52), HeightMicrons: mmToMicrons(74), VendorID: "A8", CustomDisplayNameLocalized: cdd.NewLocalizedString("A8")},
	"A9":                    {Name: cdd.MediaSizeISOA9, WidthMicrons: mmToMicrons(37), HeightMicrons: mmToMicrons(52), VendorID: "A9", CustomDisplayNameLocalized: cdd.NewLocalizedString("A9")},
	"A10":                   {Name: cdd.MediaSizeISOA10, WidthMicrons: mmToMicrons(26), HeightMicrons: mmToMicrons(37), VendorID: "A10", CustomDisplayNameLocalized: cdd.NewLocalizedString("A10")},
	"ISOB0":                 {Name: cdd.MediaSizeISOB0, WidthMicrons: mmToMicrons(1000), HeightMicrons: mmToMicrons(1414), VendorID: "ISOB0", CustomDisplayNameLocalized: cdd.NewLocalizedString("B0 (ISO)")},
	"ISOB1":                 {Name: cdd.MediaSizeISOB1, WidthMicrons: mmToMicrons(707), HeightMicrons: mmToMicrons(1000), VendorID: "ISOB1", CustomDisplayNameLocalized: cdd.NewLocalizedString("B1 (ISO)")},
	"ISOB2":                 {Name: cdd.MediaSizeISOB2, WidthMicrons: mmToMicrons(500), HeightMicrons: mmToMicrons(707), VendorID: "ISOB2", CustomDisplayNameLocalized: cdd.NewLocalizedString("B2 (ISO)")},
	"ISOB3":                 {Name: cdd.MediaSizeISOB3, WidthMicrons: mmToMicrons(353), HeightMicrons: mmToMicrons(500), VendorID: "ISOB3", CustomDisplayNameLocalized: cdd.NewLocalizedString("B3 (ISO)")},
	"ISOB4":                 {Name: cdd.MediaSizeISOB4, WidthMicrons: mmToMicrons(250), HeightMicrons: mmToMicrons(353), VendorID: "ISOB4", CustomDisplayNameLocalized: cdd.NewLocalizedString("B4 (ISO)")},
	"ISOB5":                 {Name: cdd.MediaSizeISOB5, WidthMicrons: mmToMicrons(176), HeightMicrons: mmToMicrons(250), VendorID: "ISOB5", CustomDisplayNameLocalized: cdd.NewLocalizedString("B5 (ISO)")},
	"EnvISOB5":              {Name: cdd.MediaSizeISOB5, WidthMicrons: mmToMicrons(176), HeightMicrons: mmToMicrons(250), VendorID: "EnvISOB5", CustomDisplayNameLocalized: cdd.NewLocalizedString("B5 Envelope (ISO)")},
	"ISOB5Extra":            {Name: cdd.MediaSizeISOB5Extra, WidthMicrons: mmToMicrons(201), HeightMicrons: mmToMicrons(276), VendorID: "ISOB5Extra", CustomDisplayNameLocalized: cdd.NewLocalizedString("B5 Extra (ISO)")},
	"ISOB6":                 {Name: cdd.MediaSizeISOB6, WidthMicrons: mmToMicrons(125), HeightMicrons: mmToMicrons(176), VendorID: "ISOB6", CustomDisplayNameLocalized: cdd.NewLocalizedString("B6 (ISO)")},
	"ISOB7":                 {Name: cdd.MediaSizeISOB7, WidthMicrons: mmToMicrons(88), HeightMicrons: mmToMicrons(125), VendorID: "ISOB7", CustomDisplayNameLocalized: cdd.NewLocalizedString("B7 (ISO)")},
	"ISOB8":                 {Name: cdd.MediaSizeISOB8, WidthMicrons: mmToMicrons(62), HeightMicrons: mmToMicrons(88), VendorID: "ISOB8", CustomDisplayNameLocalized: cdd.NewLocalizedString("B8 (ISO)")},
	"ISOB9":                 {Name: cdd.MediaSizeISOB9, WidthMicrons: mmToMicrons(44), HeightMicrons: mmToMicrons(62), VendorID: "ISOB9", CustomDisplayNameLocalized: cdd.NewLocalizedString("B9 (ISO)")},
	"ISOB10":                {Name: cdd.MediaSizeISOB10, WidthMicrons: mmToMicrons(31), HeightMicrons: mmToMicrons(44), VendorID: "ISOB10", CustomDisplayNameLocalized: cdd.NewLocalizedString("B10 (ISO)")},
	"EnvC0":                 {Name: cdd.MediaSizeISOC0, WidthMicrons: mmToMicrons(917), HeightMicrons: mmToMicrons(1297), VendorID: "EnvC0", CustomDisplayNameLocalized: cdd.NewLocalizedString("C0 (ISO)")},
	"EnvC1":                 {Name: cdd.MediaSizeISOC1, WidthMicrons: mmToMicrons(648), HeightMicrons: mmToMicrons(917), VendorID: "EnvC1", CustomDisplayNameLocalized: cdd.NewLocalizedString("C1 (ISO)")},
	"EnvC2":                 {Name: cdd.MediaSizeISOC2, WidthMicrons: mmToMicrons(458), HeightMicrons: mmToMicrons(648), VendorID: "EnvC2", CustomDisplayNameLocalized: cdd.NewLocalizedString("C2 (ISO)")},
	"EnvC3":                 {Name: cdd.MediaSizeISOC3, WidthMicrons: mmToMicrons(324), HeightMicrons: mmToMicrons(458), VendorID: "EnvC3", CustomDisplayNameLocalized: cdd.NewLocalizedString("C3 (ISO)")},
	"EnvC4":                 {Name: cdd.MediaSizeISOC4, WidthMicrons: mmToMicrons(229), HeightMicrons: mmToMicrons(324), VendorID: "EnvC4", CustomDisplayNameLocalized: cdd.NewLocalizedString("C4 (ISO)")},
	"EnvC5":                 {Name: cdd.MediaSizeISOC5, WidthMicrons: mmToMicrons(162), HeightMicrons: mmToMicrons(229), VendorID: "EnvC5", CustomDisplayNameLocalized: cdd.NewLocalizedString("C5 (ISO)")},
	"EnvC6":                 {Name: cdd.MediaSizeISOC6, WidthMicrons: mmToMicrons(114), HeightMicrons: mmToMicrons(162), VendorID: "EnvC6", CustomDisplayNameLocalized: cdd.NewLocalizedString("C6 (ISO)")},
	"EnvC65":                {Name: cdd.MediaSizeISOC6c5, WidthMicrons: mmToMicrons(114), HeightMicrons: mmToMicrons(229), VendorID: "EnvC65", CustomDisplayNameLocalized: cdd.NewLocalizedString("C6c5 (ISO)")},
	"EnvC7":                 {Name: cdd.MediaSizeISOC7, WidthMicrons: mmToMicrons(81), HeightMicrons: mmToMicrons(114), VendorID: "EnvC7", CustomDisplayNameLocalized: cdd.NewLocalizedString("C7 (ISO)")},
	"EnvDL":                 {Name: cdd.MediaSizeISODL, WidthMicrons: mmToMicrons(110), HeightMicrons: mmToMicrons(220), VendorID: "EnvDL", CustomDisplayNameLocalized: cdd.NewLocalizedString("DL Envelope")},
	"DLEnv":                 {Name: cdd.MediaSizeISODL, WidthMicrons: mmToMicrons(110), HeightMicrons: mmToMicrons(220), VendorID: "DLEnv", CustomDisplayNameLocalized: cdd.NewLocalizedString("DL Envelope")},
	"RA0":                   {Name: cdd.MediaSizeISORA0, WidthMicrons: mmToMicrons(860), HeightMicrons: mmToMicrons(1220), VendorID: "RA0", CustomDisplayNameLocalized: cdd.NewLocalizedString("RA0")},
	"RA1":                   {Name: cdd.MediaSizeISORA1, WidthMicrons: mmToMicrons(610), HeightMicrons: mmToMicrons(860), VendorID: "RA1", CustomDisplayNameLocalized: cdd.NewLocalizedString("RA1")},
	"RA2":                   {Name: cdd.MediaSizeISORA2, WidthMicrons: mmToMicrons(430), HeightMicrons: mmToMicrons(610), VendorID: "RA2", CustomDisplayNameLocalized: cdd.NewLocalizedString("RA2")},
	"RA3":                   {Name: cdd.MediaSizeCustom, WidthMicrons: mmToMicrons(305), HeightMicrons: mmToMicrons(430), VendorID: "RA3", CustomDisplayNameLocalized: cdd.NewLocalizedString("RA3")},
	"RA4":                   {Name: cdd.MediaSizeCustom, WidthMicrons: mmToMicrons(215), HeightMicrons: mmToMicrons(305), VendorID: "RA4", CustomDisplayNameLocalized: cdd.NewLocalizedString("RA4")},
	"SRA0":                  {Name: cdd.MediaSizeISOSRA0, WidthMicrons: mmToMicrons(900), HeightMicrons: mmToMicrons(1280), VendorID: "SRA0", CustomDisplayNameLocalized: cdd.NewLocalizedString("SRA0")},
	"SRA1":                  {Name: cdd.MediaSizeISOSRA1, WidthMicrons: mmToMicrons(640), HeightMicrons: mmToMicrons(900), VendorID: "SRA1", CustomDisplayNameLocalized: cdd.NewLocalizedString("SRA1")},
	"SRA2":                  {Name: cdd.MediaSizeISOSRA2, WidthMicrons: mmToMicrons(450), HeightMicrons: mmToMicrons(640), VendorID: "SRA2", CustomDisplayNameLocalized: cdd.NewLocalizedString("SRA2")},
	"SRA3":                  {Name: cdd.MediaSizeCustom, WidthMicrons: mmToMicrons(320), HeightMicrons: mmToMicrons(450), VendorID: "SRA3", CustomDisplayNameLocalized: cdd.NewLocalizedString("SRA3")},
	"SRA4":                  {Name: cdd.MediaSizeCustom, WidthMicrons: mmToMicrons(225), HeightMicrons: mmToMicrons(320), VendorID: "SRA4", CustomDisplayNameLocalized: cdd.NewLocalizedString("SRA4")},
	"JISB0":                 {Name: cdd.MediaSizeJISB0, WidthMicrons: mmToMicrons(1030), HeightMicrons: mmToMicrons(1456), VendorID: "JISB0", CustomDisplayNameLocalized: cdd.NewLocalizedString("B0 (JIS)")},
	"B0JIS":                 {Name: cdd.MediaSizeJISB0, WidthMicrons: mmToMicrons(1030), HeightMicrons: mmToMicrons(1456), VendorID: "B0JIS", CustomDisplayNameLocalized: cdd.NewLocalizedString("B0 (JIS)")},
	"B0":                    {Name: cdd.MediaSizeJISB0, WidthMicrons: mmToMicrons(1030), HeightMicrons: mmToMicrons(1456), VendorID: "B0", CustomDisplayNameLocalized: cdd.NewLocalizedString("B0 (JIS)")},
	"JISB1":                 {Name: cdd.MediaSizeJISB1, WidthMicrons: mmToMicrons(728), HeightMicrons: mmToMicrons(1030), VendorID: "JISB1", CustomDisplayNameLocalized: cdd.NewLocalizedString("B1 (JIS)")},
	"B1JIS":                 {Name: cdd.MediaSizeJISB1, WidthMicrons: mmToMicrons(728), HeightMicrons: mmToMicrons(1030), VendorID: "B1JIS", CustomDisplayNameLocalized: cdd.NewLocalizedString("B1 (JIS)")},
	"B1":                    {Name: cdd.MediaSizeJISB1, WidthMicrons: mmToMicrons(728), HeightMicrons: mmToMicrons(1030), VendorID: "B1", CustomDisplayNameLocalized: cdd.NewLocalizedString("B1 (JIS)")},
	"JISB2":                 {Name: cdd.MediaSizeJISB2, WidthMicrons: mmToMicrons(515), HeightMicrons: mmToMicrons(728), VendorID: "JISB2", CustomDisplayNameLocalized: cdd.NewLocalizedString("B2 (JIS)")},
	"B2JIS":                 {Name: cdd.MediaSizeJISB2, WidthMicrons: mmToMicrons(515), HeightMicrons: mmToMicrons(728), VendorID: "B2JIS", CustomDisplayNameLocalized: cdd.NewLocalizedString("B2 (JIS)")},
	"B2":                    {Name: cdd.MediaSizeJISB2, WidthMicrons: mmToMicrons(515), HeightMicrons: mmToMicrons(728), VendorID: "B2", CustomDisplayNameLocalized: cdd.NewLocalizedString("B2 (JIS)")},
	"JISB3":                 {Name: cdd.MediaSizeJISB3, WidthMicrons: mmToMicrons(364), HeightMicrons: mmToMicrons(515), VendorID: "JISB3", CustomDisplayNameLocalized: cdd.NewLocalizedString("B3 (JIS)")},
	"B3JIS":                 {Name: cdd.MediaSizeJISB3, WidthMicrons: mmToMicrons(364), HeightMicrons: mmToMicrons(515), VendorID: "B3JIS", CustomDisplayNameLocalized: cdd.NewLocalizedString("B3 (JIS)")},
	"B3":                    {Name: cdd.MediaSizeJISB3, WidthMicrons: mmToMicrons(364), HeightMicrons: mmToMicrons(515), VendorID: "B3", CustomDisplayNameLocalized: cdd.NewLocalizedString("B3 (JIS)")},
	"JISB4":                 {Name: cdd.MediaSizeJISB4, WidthMicrons: mmToMicrons(257), HeightMicrons: mmToMicrons(364), VendorID: "JISB4", CustomDisplayNameLocalized: cdd.NewLocalizedString("B4 (JIS)")},
	"B4JIS":                 {Name: cdd.MediaSizeJISB4, WidthMicrons: mmToMicrons(257), HeightMicrons: mmToMicrons(364), VendorID: "B4JIS", CustomDisplayNameLocalized: cdd.NewLocalizedString("B4 (JIS)")},
	"B4":                    {Name: cdd.MediaSizeJISB4, WidthMicrons: mmToMicrons(257), HeightMicrons: mmToMicrons(364), VendorID: "B4", CustomDisplayNameLocalized: cdd.NewLocalizedString("B4 (JIS)")},
	"JISB5":                 {Name: cdd.MediaSizeJISB5, WidthMicrons: mmToMicrons(182), HeightMicrons: mmToMicrons(257), VendorID: "JISB5", CustomDisplayNameLocalized: cdd.NewLocalizedString("B5 (JIS)")},
	"B5JIS":                 {Name: cdd.MediaSizeJISB5, WidthMicrons: mmToMicrons(182), HeightMicrons: mmToMicrons(257), VendorID: "B5JIS", CustomDisplayNameLocalized: cdd.NewLocalizedString("B5 (JIS)")},
	"B5":                    {Name: cdd.MediaSizeJISB5, WidthMicrons: mmToMicrons(182), HeightMicrons: mmToMicrons(257), VendorID: "B5", CustomDisplayNameLocalized: cdd.NewLocalizedString("B5 (JIS)")},
	"JISB6":                 {Name: cdd.MediaSizeJISB6, WidthMicrons: mmToMicrons(128), HeightMicrons: mmToMicrons(182), VendorID: "JISB6", CustomDisplayNameLocalized: cdd.NewLocalizedString("B6 (JIS)")},
	"B6JIS":                 {Name: cdd.MediaSizeJISB6, WidthMicrons: mmToMicrons(128), HeightMicrons: mmToMicrons(182), VendorID: "B6JIS", CustomDisplayNameLocalized: cdd.NewLocalizedString("B6 (JIS)")},
	"B6":                    {Name: cdd.MediaSizeJISB6, WidthMicrons: mmToMicrons(128), HeightMicrons: mmToMicrons(182), VendorID: "B6", CustomDisplayNameLocalized: cdd.NewLocalizedString("B6 (JIS)")},
	"JISB7":                 {Name: cdd.MediaSizeJISB7, WidthMicrons: mmToMicrons(91), HeightMicrons: mmToMicrons(128), VendorID: "JISB7", CustomDisplayNameLocalized: cdd.NewLocalizedString("B7 (JIS)")},
	"B7JIS":                 {Name: cdd.MediaSizeJISB7, WidthMicrons: mmToMicrons(91), HeightMicrons: mmToMicrons(128), VendorID: "B7JIS", CustomDisplayNameLocalized: cdd.NewLocalizedString("B7 (JIS)")},
	"B7":                    {Name: cdd.MediaSizeJISB7, WidthMicrons: mmToMicrons(91), HeightMicrons: mmToMicrons(128), VendorID: "B7", CustomDisplayNameLocalized: cdd.NewLocalizedString("B7 (JIS)")},
	"JISB8":                 {Name: cdd.MediaSizeJISB8, WidthMicrons: mmToMicrons(64), HeightMicrons: mmToMicrons(91), VendorID: "JISB8", CustomDisplayNameLocalized: cdd.NewLocalizedString("B8 (JIS)")},
	"B8JIS":                 {Name: cdd.MediaSizeJISB8, WidthMicrons: mmToMicrons(64), HeightMicrons: mmToMicrons(91), VendorID: "B8JIS", CustomDisplayNameLocalized: cdd.NewLocalizedString("B8 (JIS)")},
	"B8":                    {Name: cdd.MediaSizeJISB8, WidthMicrons: mmToMicrons(64), HeightMicrons: mmToMicrons(91), VendorID: "B8", CustomDisplayNameLocalized: cdd.NewLocalizedString("B8 (JIS)")},
	"JISB9":                 {Name: cdd.MediaSizeJISB9, WidthMicrons: mmToMicrons(45), HeightMicrons: mmToMicrons(64), VendorID: "JISB9", CustomDisplayNameLocalized: cdd.NewLocalizedString("B9 (JIS)")},
	"B9JIS":                 {Name: cdd.MediaSizeJISB9, WidthMicrons: mmToMicrons(45), HeightMicrons: mmToMicrons(64), VendorID: "B9JIS", CustomDisplayNameLocalized: cdd.NewLocalizedString("B9 (JIS)")},
	"B9":                    {Name: cdd.MediaSizeJISB9, WidthMicrons: mmToMicrons(45), HeightMicrons: mmToMicrons(64), VendorID: "B9", CustomDisplayNameLocalized: cdd.NewLocalizedString("B9 (JIS)")},
	"JISB10":                {Name: cdd.MediaSizeJISB10, WidthMicrons: mmToMicrons(32), HeightMicrons: mmToMicrons(45), VendorID: "JISB10", CustomDisplayNameLocalized: cdd.NewLocalizedString("B10 (JIS)")},
	"B10JIS":                {Name: cdd.MediaSizeJISB10, WidthMicrons: mmToMicrons(32), HeightMicrons: mmToMicrons(45), VendorID: "B10JIS", CustomDisplayNameLocalized: cdd.NewLocalizedString("B10 (JIS)")},
	"B10":                   {Name: cdd.MediaSizeJISB10, WidthMicrons: mmToMicrons(32), HeightMicrons: mmToMicrons(45), VendorID: "B10", CustomDisplayNameLocalized: cdd.NewLocalizedString("B10 (JIS)")},
	"EnvChou4":              {Name: cdd.MediaSizeJPNChou4, WidthMicrons: mmToMicrons(90), HeightMicrons: mmToMicrons(205), VendorID: "EnvChou4", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvChou4")},
	"Hagaki":                {Name: cdd.MediaSizeJPNHagaki, WidthMicrons: mmToMicrons(100), HeightMicrons: mmToMicrons(148), VendorID: "Hagaki", CustomDisplayNameLocalized: cdd.NewLocalizedString("Hagaki")},
	"JapanesePostCard":      {Name: cdd.MediaSizeJPNHagaki, WidthMicrons: mmToMicrons(100), HeightMicrons: mmToMicrons(148), VendorID: "JapanesePostCard", CustomDisplayNameLocalized: cdd.NewLocalizedString("Japanese Postcard")},
	"Postcard":              {Name: cdd.MediaSizeJPNHagaki, WidthMicrons: mmToMicrons(100), HeightMicrons: mmToMicrons(148), VendorID: "Postcard", CustomDisplayNameLocalized: cdd.NewLocalizedString("Postcard")},
	"EnvYou4":               {Name: cdd.MediaSizeJPNYou4, WidthMicrons: mmToMicrons(105), HeightMicrons: mmToMicrons(235), VendorID: "EnvYou4", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvYou4")},
	"EnvChou3":              {Name: cdd.MediaSizeJPNChou3, WidthMicrons: mmToMicrons(120), HeightMicrons: mmToMicrons(235), VendorID: "EnvChou3", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvChou3")},
	"Oufuku":                {Name: cdd.MediaSizeJPNOufuku, WidthMicrons: mmToMicrons(148), HeightMicrons: mmToMicrons(200), VendorID: "Oufuku", CustomDisplayNameLocalized: cdd.NewLocalizedString("Oufuku")},
	"DoublePostcardRotated": {Name: cdd.MediaSizeJPNOufuku, WidthMicrons: mmToMicrons(148), HeightMicrons: mmToMicrons(200), VendorID: "DoublePostcardRotated", CustomDisplayNameLocalized: cdd.NewLocalizedString("Double Postcard Rotated")},
	"EnvKaku2":              {Name: cdd.MediaSizeJPNKaku2, WidthMicrons: mmToMicrons(240), HeightMicrons: mmToMicrons(332), VendorID: "EnvKaku2", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvKaku2")},
	"om_small-photo":        {Name: cdd.MediaSizeOMSmallPhoto, WidthMicrons: mmToMicrons(100), HeightMicrons: mmToMicrons(150), VendorID: "om_small-photo", CustomDisplayNameLocalized: cdd.NewLocalizedString("Small Photo")},
	"EnvItalian":            {Name: cdd.MediaSizeOMItalian, WidthMicrons: mmToMicrons(110), HeightMicrons: mmToMicrons(230), VendorID: "EnvItalian", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvItalian")},
	"om_large-photo":        {Name: cdd.MediaSizeOMLargePhoto, WidthMicrons: mmToMicrons(200), HeightMicrons: mmToMicrons(300), VendorID: "om_large-photo", CustomDisplayNameLocalized: cdd.NewLocalizedString("Large Photo")},
	"Folio":                 {Name: cdd.MediaSizeOMFolio, WidthMicrons: mmToMicrons(210), HeightMicrons: mmToMicrons(330), VendorID: "Folio", CustomDisplayNameLocalized: cdd.NewLocalizedString("Folio")},
	"FolioSP":               {Name: cdd.MediaSizeOMFolioSP, WidthMicrons: mmToMicrons(215), HeightMicrons: mmToMicrons(315), VendorID: "FolioSP", CustomDisplayNameLocalized: cdd.NewLocalizedString("FolioSP")},
	"EnvInvite":             {Name: cdd.MediaSizeOMInvite, WidthMicrons: mmToMicrons(220), HeightMicrons: mmToMicrons(220), VendorID: "EnvInvite", CustomDisplayNameLocalized: cdd.NewLocalizedString("EnvInvite")},
	"8Kai":                  {Name: cdd.MediaSizeCustom, WidthMicrons: inchesToMicrons(10.5), HeightMicrons: inchesToMicrons(15.375), VendorID: "8Kai", CustomDisplayNameLocalized: cdd.NewLocalizedString("8 Kai")},
	"8K":                    {Name: cdd.MediaSizeCustom, WidthMicrons: inchesToMicrons(10.5), HeightMicrons: inchesToMicrons(15.375), VendorID: "8K", CustomDisplayNameLocalized: cdd.NewLocalizedString("8 Kai")},
	"16Kai":                 {Name: cdd.MediaSizeCustom, WidthMicrons: inchesToMicrons(7.6875), HeightMicrons: inchesToMicrons(10.5), VendorID: "16Kai", CustomDisplayNameLocalized: cdd.NewLocalizedString("16 Kai")},
	"16K":                   {Name: cdd.MediaSizeCustom, WidthMicrons: inchesToMicrons(7.6875), HeightMicrons: inchesToMicrons(10.5), VendorID: "16K", CustomDisplayNameLocalized: cdd.NewLocalizedString("16 Kai")},
}

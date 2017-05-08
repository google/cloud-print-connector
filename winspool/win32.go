// Copyright 2015 Google Inc. All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// +build windows

package winspool

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	gdi32    = syscall.MustLoadDLL("gdi32.dll")
	kernel32 = syscall.MustLoadDLL("kernel32.dll")
	ntoskrnl = syscall.MustLoadDLL("ntoskrnl.exe")
	winspool = syscall.MustLoadDLL("winspool.drv")
	user32   = syscall.MustLoadDLL("user32.dll")

	abortDocProc                   = gdi32.MustFindProc("AbortDoc")
	closePrinterProc               = winspool.MustFindProc("ClosePrinter")
	createDCProc                   = gdi32.MustFindProc("CreateDCW")
	deleteDCProc                   = gdi32.MustFindProc("DeleteDC")
	deviceCapabilitiesProc         = winspool.MustFindProc("DeviceCapabilitiesW")
	documentPropertiesProc         = winspool.MustFindProc("DocumentPropertiesW")
	endDocProc                     = gdi32.MustFindProc("EndDoc")
	endPageProc                    = gdi32.MustFindProc("EndPage")
	enumPrintersProc               = winspool.MustFindProc("EnumPrintersW")
	getDeviceCapsProc              = gdi32.MustFindProc("GetDeviceCaps")
	getJobProc                     = winspool.MustFindProc("GetJobW")
	openPrinterProc                = winspool.MustFindProc("OpenPrinterW")
	resetDCProc                    = gdi32.MustFindProc("ResetDCW")
	rtlGetVersionProc              = ntoskrnl.MustFindProc("RtlGetVersion")
	setGraphicsModeProc            = gdi32.MustFindProc("SetGraphicsMode")
	setJobProc                     = winspool.MustFindProc("SetJobW")
	setWorldTransformProc          = gdi32.MustFindProc("SetWorldTransform")
	startDocProc                   = gdi32.MustFindProc("StartDocW")
	startPageProc                  = gdi32.MustFindProc("StartPage")
	registerDeviceNotificationProc = user32.MustFindProc("RegisterDeviceNotificationW")
)

// System error codes.
const (
	ERROR_SUCCESS   = 0
	ERROR_MORE_DATA = 234
)

// Errors returned by GetLastError().
const (
	NO_ERROR                  = syscall.Errno(0)
	ERROR_INVALID_PARAMETER   = syscall.Errno(87)
	ERROR_INSUFFICIENT_BUFFER = syscall.Errno(122)
)

// First parameter to EnumPrinters().
const (
	PRINTER_ENUM_DEFAULT     = 0x00000001
	PRINTER_ENUM_LOCAL       = 0x00000002
	PRINTER_ENUM_CONNECTIONS = 0x00000004
	PRINTER_ENUM_FAVORITE    = 0x00000004
	PRINTER_ENUM_NAME        = 0x00000008
	PRINTER_ENUM_REMOTE      = 0x00000010
	PRINTER_ENUM_SHARED      = 0x00000020
	PRINTER_ENUM_NETWORK     = 0x00000040
	PRINTER_ENUM_EXPAND      = 0x00004000
	PRINTER_ENUM_CONTAINER   = 0x00008000
	PRINTER_ENUM_ICONMASK    = 0x00ff0000
	PRINTER_ENUM_ICON1       = 0x00010000
	PRINTER_ENUM_ICON2       = 0x00020000
	PRINTER_ENUM_ICON3       = 0x00040000
	PRINTER_ENUM_ICON4       = 0x00080000
	PRINTER_ENUM_ICON5       = 0x00100000
	PRINTER_ENUM_ICON6       = 0x00200000
	PRINTER_ENUM_ICON7       = 0x00400000
	PRINTER_ENUM_ICON8       = 0x00800000
	PRINTER_ENUM_HIDE        = 0x01000000
)

// Registry value types.
const (
	REG_NONE                       = 0
	REG_SZ                         = 1
	REG_EXPAND_SZ                  = 2
	REG_BINARY                     = 3
	REG_DWORD                      = 4
	REG_DWORD_LITTLE_ENDIAN        = 4
	REG_DWORD_BIG_ENDIAN           = 5
	REG_LINK                       = 6
	REG_MULTI_SZ                   = 7
	REG_RESOURCE_LIST              = 8
	REG_FULL_RESOURCE_DESCRIPTOR   = 9
	REG_RESOURCE_REQUIREMENTS_LIST = 10
	REG_QWORD                      = 11
	REG_QWORD_LITTLE_ENDIAN        = 11
)

// PRINTER_INFO_2 attribute values
const (
	PRINTER_ATTRIBUTE_QUEUED            uint32 = 0x00000001
	PRINTER_ATTRIBUTE_DIRECT            uint32 = 0x00000002
	PRINTER_ATTRIBUTE_DEFAULT           uint32 = 0x00000004
	PRINTER_ATTRIBUTE_SHARED            uint32 = 0x00000008
	PRINTER_ATTRIBUTE_NETWORK           uint32 = 0x00000010
	PRINTER_ATTRIBUTE_HIDDEN            uint32 = 0x00000020
	PRINTER_ATTRIBUTE_LOCAL             uint32 = 0x00000040
	PRINTER_ATTRIBUTE_ENABLE_DEVQ       uint32 = 0x00000080
	PRINTER_ATTRIBUTE_KEEPPRINTEDJOBS   uint32 = 0x00000100
	PRINTER_ATTRIBUTE_DO_COMPLETE_FIRST uint32 = 0x00000200
	PRINTER_ATTRIBUTE_WORK_OFFLINE      uint32 = 0x00000400
	PRINTER_ATTRIBUTE_ENABLE_BIDI       uint32 = 0x00000800
	PRINTER_ATTRIBUTE_RAW_ONLY          uint32 = 0x00001000
	PRINTER_ATTRIBUTE_PUBLISHED         uint32 = 0x00002000
)

// PRINTER_INFO_2 status values.
const (
	PRINTER_STATUS_PAUSED               uint32 = 0x00000001
	PRINTER_STATUS_ERROR                uint32 = 0x00000002
	PRINTER_STATUS_PENDING_DELETION     uint32 = 0x00000004
	PRINTER_STATUS_PAPER_JAM            uint32 = 0x00000008
	PRINTER_STATUS_PAPER_OUT            uint32 = 0x00000010
	PRINTER_STATUS_MANUAL_FEED          uint32 = 0x00000020
	PRINTER_STATUS_PAPER_PROBLEM        uint32 = 0x00000040
	PRINTER_STATUS_OFFLINE              uint32 = 0x00000080
	PRINTER_STATUS_IO_ACTIVE            uint32 = 0x00000100
	PRINTER_STATUS_BUSY                 uint32 = 0x00000200
	PRINTER_STATUS_PRINTING             uint32 = 0x00000400
	PRINTER_STATUS_OUTPUT_BIN_FULL      uint32 = 0x00000800
	PRINTER_STATUS_NOT_AVAILABLE        uint32 = 0x00001000
	PRINTER_STATUS_WAITING              uint32 = 0x00002000
	PRINTER_STATUS_PROCESSING           uint32 = 0x00004000
	PRINTER_STATUS_INITIALIZING         uint32 = 0x00008000
	PRINTER_STATUS_WARMING_UP           uint32 = 0x00010000
	PRINTER_STATUS_TONER_LOW            uint32 = 0x00020000
	PRINTER_STATUS_NO_TONER             uint32 = 0x00040000
	PRINTER_STATUS_PAGE_PUNT            uint32 = 0x00080000
	PRINTER_STATUS_USER_INTERVENTION    uint32 = 0x00100000
	PRINTER_STATUS_OUT_OF_MEMORY        uint32 = 0x00200000
	PRINTER_STATUS_DOOR_OPEN            uint32 = 0x00400000
	PRINTER_STATUS_SERVER_UNKNOWN       uint32 = 0x00800000
	PRINTER_STATUS_POWER_SAVE           uint32 = 0x01000000
	PRINTER_STATUS_SERVER_OFFLINE       uint32 = 0x02000000
	PRINTER_STATUS_DRIVER_UPDATE_NEEDED uint32 = 0x04000000
)

// PRINTER_INFO_2 struct.
type PrinterInfo2 struct {
	pServerName         *uint16
	pPrinterName        *uint16
	pShareName          *uint16
	pPortName           *uint16
	pDriverName         *uint16
	pComment            *uint16
	pLocation           *uint16
	pDevMode            *DevMode
	pSepFile            *uint16
	pPrintProcessor     *uint16
	pDatatype           *uint16
	pParameters         *uint16
	pSecurityDescriptor uintptr
	attributes          uint32
	priority            uint32
	defaultPriority     uint32
	startTime           uint32
	untilTime           uint32
	status              uint32
	cJobs               uint32
	averagePPM          uint32
}

func (pi *PrinterInfo2) GetPrinterName() string {
	return utf16PtrToString(pi.pPrinterName)
}

func (pi *PrinterInfo2) GetPortName() string {
	return utf16PtrToString(pi.pPortName)
}

func (pi *PrinterInfo2) GetDriverName() string {
	return utf16PtrToString(pi.pDriverName)
}

func (pi *PrinterInfo2) GetLocation() string {
	return utf16PtrToString(pi.pLocation)
}

func (pi *PrinterInfo2) GetDevMode() *DevMode {
	return pi.pDevMode
}

func (pi *PrinterInfo2) GetAttributes() uint32 {
	return pi.attributes
}

func (pi *PrinterInfo2) GetStatus() uint32 {
	return pi.status
}

// PRINTER_ENUM_VALUES struct.
type PrinterEnumValues struct {
	pValueName  *uint16
	cbValueName uint32
	dwType      uint32
	pData       uintptr
	cbData      uint32
}

// DEVMODE constants.
const (
	CCHDEVICENAME = 32
	CCHFORMNAME   = 32

	DM_SPECVERSION uint16 = 0x0401
	DM_COPY        uint32 = 2
	DM_MODIFY      uint32 = 8

	DM_ORIENTATION        = 0x00000001
	DM_PAPERSIZE          = 0x00000002
	DM_PAPERLENGTH        = 0x00000004
	DM_PAPERWIDTH         = 0x00000008
	DM_SCALE              = 0x00000010
	DM_POSITION           = 0x00000020
	DM_NUP                = 0x00000040
	DM_DISPLAYORIENTATION = 0x00000080
	DM_COPIES             = 0x00000100
	DM_DEFAULTSOURCE      = 0x00000200
	DM_PRINTQUALITY       = 0x00000400
	DM_COLOR              = 0x00000800
	DM_DUPLEX             = 0x00001000
	DM_YRESOLUTION        = 0x00002000
	DM_TTOPTION           = 0x00004000
	DM_COLLATE            = 0x00008000
	DM_FORMNAME           = 0x00010000
	DM_LOGPIXELS          = 0x00020000
	DM_BITSPERPEL         = 0x00040000
	DM_PELSWIDTH          = 0x00080000
	DM_PELSHEIGHT         = 0x00100000
	DM_DISPLAYFLAGS       = 0x00200000
	DM_DISPLAYFREQUENCY   = 0x00400000
	DM_ICMMETHOD          = 0x00800000
	DM_ICMINTENT          = 0x01000000
	DM_MEDIATYPE          = 0x02000000
	DM_DITHERTYPE         = 0x04000000
	DM_PANNINGWIDTH       = 0x08000000
	DM_PANNINGHEIGHT      = 0x10000000
	DM_DISPLAYFIXEDOUTPUT = 0x20000000

	DMORIENT_PORTRAIT  int16 = 1
	DMORIENT_LANDSCAPE int16 = 2

	DMCOLOR_MONOCHROME int16 = 1
	DMCOLOR_COLOR      int16 = 2

	DMDUP_SIMPLEX    int16 = 1
	DMDUP_VERTICAL   int16 = 2
	DMDUP_HORIZONTAL int16 = 3

	DMCOLLATE_FALSE int16 = 0
	DMCOLLATE_TRUE  int16 = 1

	DMNUP_SYSTEM uint32 = 1
	DMNUP_ONEUP  uint32 = 2
)

// DEVMODE struct.
type DevMode struct {
	// WCHAR dmDeviceName[CCHDEVICENAME]
	dmDeviceName, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _ uint16

	dmSpecVersion   uint16
	dmDriverVersion uint16
	dmSize          uint16
	dmDriverExtra   uint16
	dmFields        uint32

	dmOrientation   int16
	dmPaperSize     int16
	dmPaperLength   int16
	dmPaperWidth    int16
	dmScale         int16
	dmCopies        int16
	dmDefaultSource int16
	dmPrintQuality  int16
	dmColor         int16
	dmDuplex        int16
	dmYResolution   int16
	dmTTOption      int16
	dmCollate       int16
	// WCHAR dmFormName[CCHFORMNAME]
	dmFormName, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _ uint16

	dmLogPixels        int16
	dmBitsPerPel       uint16
	dmPelsWidth        uint16
	dmPelsHeight       uint16
	dmNup              uint32
	dmDisplayFrequency uint32
	dmICMMethod        uint32
	dmICMIntent        uint32
	dmMediaType        uint32
	dmDitherType       uint32
	dmReserved1        uint32
	dmReserved2        uint32
	dmPanningWidth     uint32
	dmPanningHeight    uint32
}

func (dm *DevMode) String() string {
	s := []string{
		fmt.Sprintf("device name: %s", dm.GetDeviceName()),
		fmt.Sprintf("spec version: %d", dm.dmSpecVersion),
	}
	if dm.dmFields&DM_ORIENTATION != 0 {
		s = append(s, fmt.Sprintf("orientation: %d", dm.dmOrientation))
	}
	if dm.dmFields&DM_PAPERSIZE != 0 {
		s = append(s, fmt.Sprintf("paper size: %d", dm.dmPaperSize))
	}
	if dm.dmFields&DM_PAPERLENGTH != 0 {
		s = append(s, fmt.Sprintf("paper length: %d", dm.dmPaperLength))
	}
	if dm.dmFields&DM_PAPERWIDTH != 0 {
		s = append(s, fmt.Sprintf("paper width: %d", dm.dmPaperWidth))
	}
	if dm.dmFields&DM_SCALE != 0 {
		s = append(s, fmt.Sprintf("scale: %d", dm.dmScale))
	}
	if dm.dmFields&DM_COPIES != 0 {
		s = append(s, fmt.Sprintf("copies: %d", dm.dmCopies))
	}
	if dm.dmFields&DM_DEFAULTSOURCE != 0 {
		s = append(s, fmt.Sprintf("default source: %d", dm.dmDefaultSource))
	}
	if dm.dmFields&DM_PRINTQUALITY != 0 {
		s = append(s, fmt.Sprintf("print quality: %d", dm.dmPrintQuality))
	}
	if dm.dmFields&DM_COLOR != 0 {
		s = append(s, fmt.Sprintf("color: %d", dm.dmColor))
	}
	if dm.dmFields&DM_DUPLEX != 0 {
		s = append(s, fmt.Sprintf("duplex: %d", dm.dmDuplex))
	}
	if dm.dmFields&DM_YRESOLUTION != 0 {
		s = append(s, fmt.Sprintf("y-resolution: %d", dm.dmYResolution))
	}
	if dm.dmFields&DM_TTOPTION != 0 {
		s = append(s, fmt.Sprintf("TT option: %d", dm.dmTTOption))
	}
	if dm.dmFields&DM_COLLATE != 0 {
		s = append(s, fmt.Sprintf("collate: %d", dm.dmCollate))
	}
	if dm.dmFields&DM_FORMNAME != 0 {
		s = append(s, fmt.Sprintf("formname: %s", utf16PtrToString(&dm.dmFormName)))
	}
	if dm.dmFields&DM_LOGPIXELS != 0 {
		s = append(s, fmt.Sprintf("log pixels: %d", dm.dmLogPixels))
	}
	if dm.dmFields&DM_BITSPERPEL != 0 {
		s = append(s, fmt.Sprintf("bits per pel: %d", dm.dmBitsPerPel))
	}
	if dm.dmFields&DM_PELSWIDTH != 0 {
		s = append(s, fmt.Sprintf("pels width: %d", dm.dmPelsWidth))
	}
	if dm.dmFields&DM_PELSHEIGHT != 0 {
		s = append(s, fmt.Sprintf("pels height: %d", dm.dmPelsHeight))
	}
	if dm.dmFields&DM_NUP != 0 {
		s = append(s, fmt.Sprintf("display flags: %d", dm.dmNup))
	}
	if dm.dmFields&DM_DISPLAYFREQUENCY != 0 {
		s = append(s, fmt.Sprintf("display frequency: %d", dm.dmDisplayFrequency))
	}
	if dm.dmFields&DM_ICMMETHOD != 0 {
		s = append(s, fmt.Sprintf("ICM method: %d", dm.dmICMMethod))
	}
	if dm.dmFields&DM_ICMINTENT != 0 {
		s = append(s, fmt.Sprintf("ICM intent: %d", dm.dmICMIntent))
	}
	if dm.dmFields&DM_DITHERTYPE != 0 {
		s = append(s, fmt.Sprintf("dither type: %d", dm.dmDitherType))
	}
	if dm.dmFields&DM_PANNINGWIDTH != 0 {
		s = append(s, fmt.Sprintf("panning width: %d", dm.dmPanningWidth))
	}
	if dm.dmFields&DM_PANNINGHEIGHT != 0 {
		s = append(s, fmt.Sprintf("panning height: %d", dm.dmPanningHeight))
	}
	return strings.Join(s, ", ")
}

func (dm *DevMode) GetDeviceName() string {
	return utf16PtrToStringSize(&dm.dmDeviceName, CCHDEVICENAME*2)
}

func (dm *DevMode) GetOrientation() (int16, bool) {
	return dm.dmOrientation, dm.dmFields&DM_ORIENTATION != 0
}

func (dm *DevMode) SetOrientation(orientation int16) {
	dm.dmOrientation = orientation
	dm.dmFields |= DM_ORIENTATION
}

func (dm *DevMode) GetPaperSize() (int16, bool) {
	return dm.dmPaperSize, dm.dmFields&DM_PAPERSIZE != 0
}

func (dm *DevMode) SetPaperSize(paperSize int16) {
	dm.dmPaperSize = paperSize
	dm.dmFields |= DM_PAPERSIZE
}

func (dm *DevMode) ClearPaperSize() {
	dm.dmFields &^= DM_PAPERSIZE
}

func (dm *DevMode) GetPaperLength() (int16, bool) {
	return dm.dmPaperLength, dm.dmFields&DM_PAPERLENGTH != 0
}

func (dm *DevMode) SetPaperLength(length int16) {
	dm.dmPaperLength = length
	dm.dmFields |= DM_PAPERLENGTH
}

func (dm *DevMode) ClearPaperLength() {
	dm.dmFields &^= DM_PAPERLENGTH
}

func (dm *DevMode) GetPaperWidth() (int16, bool) {
	return dm.dmPaperWidth, dm.dmFields&DM_PAPERWIDTH != 0
}

func (dm *DevMode) SetPaperWidth(width int16) {
	dm.dmPaperWidth = width
	dm.dmFields |= DM_PAPERWIDTH
}

func (dm *DevMode) ClearPaperWidth() {
	dm.dmFields &^= DM_PAPERWIDTH
}

func (dm *DevMode) GetCopies() (int16, bool) {
	return dm.dmCopies, dm.dmFields&DM_COPIES != 0
}

func (dm *DevMode) SetCopies(copies int16) {
	dm.dmCopies = copies
	dm.dmFields |= DM_COPIES
}

func (dm *DevMode) GetColor() (int16, bool) {
	return dm.dmColor, dm.dmFields&DM_COLOR != 0
}

func (dm *DevMode) SetColor(color int16) {
	dm.dmColor = color
	dm.dmFields |= DM_COLOR
}

func (dm *DevMode) GetDuplex() (int16, bool) {
	return dm.dmDuplex, dm.dmFields&DM_DUPLEX != 0
}

func (dm *DevMode) SetDuplex(duplex int16) {
	dm.dmDuplex = duplex
	dm.dmFields |= DM_DUPLEX
}

func (dm *DevMode) GetCollate() (int16, bool) {
	return dm.dmCollate, dm.dmFields&DM_COLLATE != 0
}

func (dm *DevMode) SetCollate(collate int16) {
	dm.dmCollate = collate
	dm.dmFields |= DM_COLLATE
}

// DOCINFO struct.
type DocInfo struct {
	cbSize       int32
	lpszDocName  *uint16
	lpszOutput   *uint16
	lpszDatatype *uint16
	fwType       uint32
}

// Device parameters for GetDeviceCaps().
const (
	DRIVERVERSION   = 0
	TECHNOLOGY      = 2
	HORZSIZE        = 4
	VERTSIZE        = 6
	HORZRES         = 8 // Printable area of paper in pixels.
	VERTRES         = 10
	BITSPIXEL       = 12
	PLANES          = 14
	NUMBRUSHES      = 16
	NUMPENS         = 18
	NUMMARKERS      = 20
	NUMFONTS        = 22
	NUMCOLORS       = 24
	PDEVICESIZE     = 26
	CURVECAPS       = 28
	LINECAPS        = 30
	POLYGONALCAPS   = 32
	TEXTCAPS        = 34
	CLIPCAPS        = 36
	RASTERCAPS      = 38
	ASPECTX         = 40
	ASPECTY         = 42
	ASPECTXY        = 44
	LOGPIXELSX      = 88 // Pixels per inch.
	LOGPIXELSY      = 90
	SIZEPALETTE     = 104
	NUMRESERVED     = 106
	COLORRES        = 108
	PHYSICALWIDTH   = 110 // Paper width in pixels.
	PHYSICALHEIGHT  = 111
	PHYSICALOFFSETX = 112 // Paper margin in pixels.
	PHYSICALOFFSETY = 113
	SCALINGFACTORX  = 114
	SCALINGFACTORY  = 115
	VREFRESH        = 116
	DESKTOPVERTRES  = 117
	DESKTOPHORZRES  = 118
	BTLALIGNMENT    = 119
	SHADEBLENDCAPS  = 120
	COLORMGMTCAPS   = 121
)

// Device capabilities for DeviceCapabilities().
const (
	DC_FIELDS            = 1
	DC_PAPERS            = 2
	DC_PAPERSIZE         = 3
	DC_MINEXTENT         = 4
	DC_MAXEXTENT         = 5
	DC_BINS              = 6
	DC_DUPLEX            = 7
	DC_SIZE              = 8
	DC_EXTRA             = 9
	DC_VERSION           = 10
	DC_DRIVER            = 11
	DC_BINNAMES          = 12
	DC_ENUMRESOLUTIONS   = 13
	DC_FILEDEPENDENCIES  = 14
	DC_TRUETYPE          = 15
	DC_PAPERNAMES        = 16
	DC_ORIENTATION       = 17
	DC_COPIES            = 18
	DC_BINADJUST         = 19
	DC_EMF_COMPLAINT     = 20
	DC_DATATYPE_PRODUCED = 21
	DC_COLLATE           = 22
	DC_MANUFACTURER      = 23
	DC_MODEL             = 24
	DC_PERSONALITY       = 25
	DC_PRINTRATE         = 26
	DC_PRINTRATEUNIT     = 27
	DC_PRINTERMEM        = 28
	DC_MEDIAREADY        = 29
	DC_STAPLE            = 30
	DC_PRINTRATEPPM      = 31
	DC_COLORDEVICE       = 32
	DC_NUP               = 33
	DC_MEDIATYPENAMES    = 34
	DC_MEDIATYPES        = 35

	PRINTRATEUNIT_PPM = 1
	PRINTRATEUNIT_CPS = 2
	PRINTRATEUNIT_LPM = 3
	PRINTRATEUNIT_IPM = 4
)

func binaryRegValueToBytes(data uintptr, size uint32) []byte {
	hdr := reflect.SliceHeader{
		Data: data,
		Len:  int(size),
		Cap:  int(size),
	}
	return *(*[]byte)(unsafe.Pointer(&hdr))
}

func enumPrinters(level uint32) ([]byte, uint32, error) {
	var cbBuf, pcReturned uint32
	_, _, err := enumPrintersProc.Call(PRINTER_ENUM_LOCAL, 0, uintptr(level), 0, 0, uintptr(unsafe.Pointer(&cbBuf)), uintptr(unsafe.Pointer(&pcReturned)))
	if err != ERROR_INSUFFICIENT_BUFFER {
		return nil, 0, err
	}

	var pPrinterEnum []byte = make([]byte, cbBuf)
	r1, _, err := enumPrintersProc.Call(PRINTER_ENUM_LOCAL, 0, uintptr(level), uintptr(unsafe.Pointer(&pPrinterEnum[0])), uintptr(cbBuf), uintptr(unsafe.Pointer(&cbBuf)), uintptr(unsafe.Pointer(&pcReturned)))
	if r1 == 0 {
		return nil, 0, err
	}

	return pPrinterEnum, pcReturned, nil
}

func EnumPrinters2() ([]PrinterInfo2, error) {
	pPrinterEnum, pcReturned, err := enumPrinters(2)
	if err != nil {
		return nil, err
	}

	hdr := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(&pPrinterEnum[0])),
		Len:  int(pcReturned),
		Cap:  int(pcReturned),
	}
	printers := *(*[]PrinterInfo2)(unsafe.Pointer(&hdr))
	return printers, nil
}

type HANDLE uintptr

func OpenPrinter(printerName string) (HANDLE, error) {
	var pPrinterName *uint16
	pPrinterName, err := syscall.UTF16PtrFromString(printerName)
	if err != nil {
		return 0, err
	}

	var hPrinter HANDLE
	r1, _, err := openPrinterProc.Call(uintptr(unsafe.Pointer(pPrinterName)), uintptr(unsafe.Pointer(&hPrinter)), 0)
	if r1 == 0 {
		return 0, err
	}
	return hPrinter, nil
}

func (hPrinter *HANDLE) ClosePrinter() error {
	r1, _, err := closePrinterProc.Call(uintptr(*hPrinter))
	if r1 == 0 {
		return err
	}
	*hPrinter = 0
	return nil
}

func (hPrinter HANDLE) DocumentPropertiesGet(deviceName string) (*DevMode, error) {
	pDeviceName, err := syscall.UTF16PtrFromString(deviceName)
	if err != nil {
		return nil, err
	}

	r1, _, err := documentPropertiesProc.Call(0, uintptr(hPrinter), uintptr(unsafe.Pointer(pDeviceName)), 0, 0, 0)
	cbBuf := int32(r1)
	if cbBuf < 0 {
		return nil, err
	}

	var pDevMode []byte = make([]byte, cbBuf)
	devMode := (*DevMode)(unsafe.Pointer(&pDevMode[0]))
	devMode.dmSize = uint16(cbBuf)
	devMode.dmSpecVersion = DM_SPECVERSION

	r1, _, err = documentPropertiesProc.Call(0, uintptr(hPrinter), uintptr(unsafe.Pointer(pDeviceName)), uintptr(unsafe.Pointer(devMode)), uintptr(unsafe.Pointer(devMode)), uintptr(DM_COPY))
	if int32(r1) < 0 {
		return nil, err
	}

	return devMode, nil
}

func (hPrinter HANDLE) DocumentPropertiesSet(deviceName string, devMode *DevMode) error {
	pDeviceName, err := syscall.UTF16PtrFromString(deviceName)
	if err != nil {
		return err
	}

	r1, _, err := documentPropertiesProc.Call(0, uintptr(hPrinter), uintptr(unsafe.Pointer(pDeviceName)), uintptr(unsafe.Pointer(devMode)), uintptr(unsafe.Pointer(devMode)), uintptr(DM_COPY|DM_MODIFY))
	if int32(r1) < 0 {
		return err
	}

	return nil
}

// JOB_INFO_1 status values.
const (
	JOB_STATUS_PAUSED            uint32 = 0x00000001
	JOB_STATUS_ERROR             uint32 = 0x00000002
	JOB_STATUS_DELETING          uint32 = 0x00000004
	JOB_STATUS_SPOOLING          uint32 = 0x00000008
	JOB_STATUS_PRINTING          uint32 = 0x00000010
	JOB_STATUS_OFFLINE           uint32 = 0x00000020
	JOB_STATUS_PAPEROUT          uint32 = 0x00000040
	JOB_STATUS_PRINTED           uint32 = 0x00000080
	JOB_STATUS_DELETED           uint32 = 0x00000100
	JOB_STATUS_BLOCKED_DEVQ      uint32 = 0x00000200
	JOB_STATUS_USER_INTERVENTION uint32 = 0x00000400
	JOB_STATUS_RESTART           uint32 = 0x00000800
	JOB_STATUS_COMPLETE          uint32 = 0x00001000
	JOB_STATUS_RETAINED          uint32 = 0x00002000
	JOB_STATUS_RENDERING_LOCALLY uint32 = 0x00004000
)

// JOB_INFO_1 struct.
type JobInfo1 struct {
	jobID        uint32
	pPrinterName *uint16
	pMachineName *uint16
	pUserName    *uint16
	pDocument    *uint16
	pDatatype    *uint16
	pStatus      *uint16
	status       uint32
	priority     uint32
	position     uint32
	totalPages   uint32
	pagesPrinted uint32

	// SYSTEMTIME structure, in line.
	wSubmittedYear         uint16
	wSubmittedMonth        uint16
	wSubmittedDayOfWeek    uint16
	wSubmittedDay          uint16
	wSubmittedHour         uint16
	wSubmittedMinute       uint16
	wSubmittedSecond       uint16
	wSubmittedMilliseconds uint16
}

func (ji1 *JobInfo1) GetStatus() uint32 {
	return ji1.status
}

func (ji1 *JobInfo1) GetTotalPages() uint32 {
	return ji1.totalPages
}

func (ji1 *JobInfo1) GetPagesPrinted() uint32 {
	return ji1.pagesPrinted
}

func (hPrinter HANDLE) GetJob(jobID int32) (*JobInfo1, error) {
	var cbBuf uint32
	_, _, err := getJobProc.Call(uintptr(hPrinter), uintptr(jobID), 1, 0, 0, uintptr(unsafe.Pointer(&cbBuf)))
	if err != ERROR_INSUFFICIENT_BUFFER {
		return nil, err
	}

	var pJob []byte = make([]byte, cbBuf)
	r1, _, err := getJobProc.Call(uintptr(hPrinter), uintptr(jobID), 1, uintptr(unsafe.Pointer(&pJob[0])), uintptr(cbBuf), uintptr(unsafe.Pointer(&cbBuf)))
	if r1 == 0 {
		return nil, err
	}

	var ji1 JobInfo1 = *(*JobInfo1)(unsafe.Pointer(&pJob[0]))

	return &ji1, nil
}

// SetJob command values.
const (
	JOB_CONTROL_PAUSE             uint32 = 1
	JOB_CONTROL_RESUME            uint32 = 2
	JOB_CONTROL_CANCEL            uint32 = 3
	JOB_CONTROL_RESTART           uint32 = 4
	JOB_CONTROL_DELETE            uint32 = 5
	JOB_CONTROL_SENT_TO_PRINTER   uint32 = 6
	JOB_CONTROL_LAST_PAGE_EJECTED uint32 = 7
	JOB_CONTROL_RETAIN            uint32 = 8
	JOB_CONTROL_RELEASE           uint32 = 9
)

func (hPrinter HANDLE) SetJobCommand(jobID int32, command uint32) error {
	r1, _, err := setJobProc.Call(uintptr(hPrinter), uintptr(jobID), 0, 0, uintptr(command))
	if r1 == 0 {
		return err
	}
	return nil
}

func (hPrinter HANDLE) SetJobInfo1(jobID int32, ji1 *JobInfo1) error {
	r1, _, err := setJobProc.Call(uintptr(hPrinter), uintptr(jobID), 1, uintptr(unsafe.Pointer(ji1)), 0)
	if r1 == 0 {
		return err
	}
	return nil
}

func (hPrinter HANDLE) SetJobUserName(jobID int32, userName string) error {
	ji1, err := hPrinter.GetJob(jobID)
	if err != nil {
		return err
	}

	pUserName, err := syscall.UTF16PtrFromString(userName)
	if err != nil {
		return err
	}

	ji1.pUserName = pUserName
	ji1.position = 0 // To prevent a possible access denied error (0 is JOB_POSITION_UNSPECIFIED)
	err = hPrinter.SetJobInfo1(jobID, ji1)
	if err != nil {
		return err
	}
	return nil
}

type HDC uintptr

func CreateDC(deviceName string, devMode *DevMode) (HDC, error) {
	lpszDevice, err := syscall.UTF16PtrFromString(deviceName)
	if err != nil {
		return 0, err
	}
	r1, _, err := createDCProc.Call(0, uintptr(unsafe.Pointer(lpszDevice)), 0, uintptr(unsafe.Pointer(devMode)))
	if r1 == 0 {
		return 0, err
	}
	return HDC(r1), nil
}

func (hDC HDC) ResetDC(devMode *DevMode) error {
	r1, _, err := resetDCProc.Call(uintptr(hDC), uintptr(unsafe.Pointer(devMode)))
	if r1 == 0 {
		return err
	}
	return nil
}

func (hDC *HDC) DeleteDC() error {
	r1, _, err := deleteDCProc.Call(uintptr(*hDC))
	if r1 == 0 {
		return err
	}
	*hDC = 0
	return nil
}

func (hDC HDC) GetDeviceCaps(nIndex int32) int32 {
	// No error returned. r1 == 0 when nIndex == -1.
	r1, _, _ := getDeviceCapsProc.Call(uintptr(hDC), uintptr(nIndex))
	return int32(r1)
}

func (hDC HDC) StartDoc(docName string) (int32, error) {
	var docInfo DocInfo
	var err error
	docInfo.cbSize = int32(unsafe.Sizeof(docInfo))
	docInfo.lpszDocName, err = syscall.UTF16PtrFromString(docName)
	if err != nil {
		return 0, err
	}

	r1, _, err := startDocProc.Call(uintptr(hDC), uintptr(unsafe.Pointer(&docInfo)))
	if r1 <= 0 {
		return 0, err
	}
	return int32(r1), nil
}

func (hDC HDC) EndDoc() error {
	r1, _, err := endDocProc.Call(uintptr(hDC))
	if r1 <= 0 {
		return err
	}
	return nil
}

func (hDC HDC) AbortDoc() error {
	r1, _, err := abortDocProc.Call(uintptr(hDC))
	fmt.Println(r1, err, "using untested AbortDoc")
	return err
}

func (hDC HDC) StartPage() error {
	r1, _, err := startPageProc.Call(uintptr(hDC))
	if r1 <= 0 {
		return err
	}
	return nil
}

func (hDC HDC) EndPage() error {
	r1, _, err := endPageProc.Call(uintptr(hDC))
	if r1 <= 0 {
		return err
	}
	return nil
}

const (
	GM_COMPATIBLE int32 = 1
	GM_ADVANCED   int32 = 2
)

func (hDC HDC) SetGraphicsMode(iMode int32) error {
	r1, _, err := setGraphicsModeProc.Call(uintptr(hDC), uintptr(iMode))
	if r1 == 0 {
		return err
	}
	return nil
}

type XFORM struct {
	eM11 float32 // X scale.
	eM12 float32 // Always zero.
	eM21 float32 // Always zero.
	eM22 float32 // Y scale.
	eDx  float32 // X offset.
	eDy  float32 // Y offset.
}

func NewXFORM(xScale, yScale, xOffset, yOffset float32) *XFORM {
	return &XFORM{xScale, 0, 0, yScale, xOffset, yOffset}
}

func (hDC HDC) SetWorldTransform(xform *XFORM) error {
	r1, _, err := setWorldTransformProc.Call(uintptr(hDC), uintptr(unsafe.Pointer(xform)))
	if r1 == 0 {
		if err == NO_ERROR {
			return fmt.Errorf("SetWorldTransform call failed; return value %d", int32(r1))
		}
		return err
	}
	return nil
}

func DeviceCapabilitiesInt32(device, port string, fwCapability uint16) (int32, error) {
	pDevice, err := syscall.UTF16PtrFromString(device)
	if err != nil {
		return 0, err
	}
	pPort, err := syscall.UTF16PtrFromString(port)
	if err != nil {
		return 0, err
	}

	r1, _, _ := deviceCapabilitiesProc.Call(uintptr(unsafe.Pointer(pDevice)), uintptr(unsafe.Pointer(pPort)), uintptr(fwCapability), 0, 0)
	return int32(r1), nil
}

func deviceCapabilities(device, port string, fwCapability uint16, pOutput []byte) (int32, error) {
	pDevice, err := syscall.UTF16PtrFromString(device)
	if err != nil {
		return 0, err
	}
	pPort, err := syscall.UTF16PtrFromString(port)
	if err != nil {
		return 0, err
	}

	var r1 uintptr
	if pOutput == nil {
		r1, _, _ = deviceCapabilitiesProc.Call(uintptr(unsafe.Pointer(pDevice)), uintptr(unsafe.Pointer(pPort)), uintptr(fwCapability), 0, 0)
	} else {
		r1, _, _ = deviceCapabilitiesProc.Call(uintptr(unsafe.Pointer(pDevice)), uintptr(unsafe.Pointer(pPort)), uintptr(fwCapability), uintptr(unsafe.Pointer(&pOutput[0])), 0)
	}

	if int32(r1) == -1 {
		return 0, errors.New("DeviceCapabilities called with unsupported capability, or there was an error")
	}
	return int32(r1), nil
}

func DeviceCapabilitiesStrings(device, port string, fwCapability uint16, stringLength int32) ([]string, error) {
	nString, err := deviceCapabilities(device, port, fwCapability, nil)
	if err != nil {
		return nil, err
	}

	if nString <= 0 {
		return []string{}, nil
	}

	pOutput := make([]byte, stringLength*uint16Size*nString)
	_, err = deviceCapabilities(device, port, fwCapability, pOutput)
	if err != nil {
		return nil, err
	}

	values := make([]string, 0, nString)
	for i := int32(0); i < nString; i++ {
		value := utf16PtrToString((*uint16)(unsafe.Pointer(&pOutput[i*stringLength])))
		values = append(values, value)
	}

	return values, nil
}

const (
	uint16Size = 2
	int32Size  = 4
)

func DeviceCapabilitiesUint16Array(device, port string, fwCapability uint16) ([]uint16, error) {
	nValue, err := deviceCapabilities(device, port, fwCapability, nil)
	if err != nil {
		return nil, err
	}

	if nValue <= 0 {
		return []uint16{}, nil
	}

	pOutput := make([]byte, uint16Size*nValue)
	_, err = deviceCapabilities(device, port, fwCapability, pOutput)
	if err != nil {
		return nil, err
	}

	values := make([]uint16, 0, nValue)
	for i := int32(0); i < nValue; i++ {
		value := *(*uint16)(unsafe.Pointer(&pOutput[i*uint16Size]))
		values = append(values, value)
	}

	return values, nil
}

// DeviceCapabilitiesInt32Pairs returns a slice of an even quantity of int32.
func DeviceCapabilitiesInt32Pairs(device, port string, fwCapability uint16) ([]int32, error) {
	nValue, err := deviceCapabilities(device, port, fwCapability, nil)
	if err != nil {
		return nil, err
	}

	if nValue <= 0 {
		return []int32{}, nil
	}

	pOutput := make([]byte, int32Size*2*nValue)
	_, err = deviceCapabilities(device, port, fwCapability, pOutput)
	if err != nil {
		return nil, err
	}

	values := make([]int32, 0, nValue*2)
	for i := int32(0); i < nValue*2; i++ {
		value := *(*int32)(unsafe.Pointer(&pOutput[i*int32Size]))
		values = append(values, value)
	}

	return values, nil
}

// DevMode.dmPaperSize values.
const (
	DMPAPER_LETTER                        = 1
	DMPAPER_LETTERSMALL                   = 2
	DMPAPER_TABLOID                       = 3
	DMPAPER_LEDGER                        = 4
	DMPAPER_LEGAL                         = 5
	DMPAPER_STATEMENT                     = 6
	DMPAPER_EXECUTIVE                     = 7
	DMPAPER_A3                            = 8
	DMPAPER_A4                            = 9
	DMPAPER_A4SMALL                       = 10
	DMPAPER_A5                            = 11
	DMPAPER_B4                            = 12
	DMPAPER_B5                            = 13
	DMPAPER_FOLIO                         = 14
	DMPAPER_QUARTO                        = 15
	DMPAPER_10X14                         = 16
	DMPAPER_11X17                         = 17
	DMPAPER_NOTE                          = 18
	DMPAPER_ENV_9                         = 19
	DMPAPER_ENV_10                        = 20
	DMPAPER_ENV_11                        = 21
	DMPAPER_ENV_12                        = 22
	DMPAPER_ENV_14                        = 23
	DMPAPER_CSHEET                        = 24
	DMPAPER_DSHEET                        = 25
	DMPAPER_ESHEET                        = 26
	DMPAPER_ENV_DL                        = 27
	DMPAPER_ENV_C5                        = 28
	DMPAPER_ENV_C3                        = 29
	DMPAPER_ENV_C4                        = 30
	DMPAPER_ENV_C6                        = 31
	DMPAPER_ENV_C65                       = 32
	DMPAPER_ENV_B4                        = 33
	DMPAPER_ENV_B5                        = 34
	DMPAPER_ENV_B6                        = 35
	DMPAPER_ENV_ITALY                     = 36
	DMPAPER_ENV_MONARCH                   = 37
	DMPAPER_ENV_PERSONAL                  = 38
	DMPAPER_FANFOLD_US                    = 39
	DMPAPER_FANFOLD_STD_GERMAN            = 40
	DMPAPER_FANFOLD_LGL_GERMAN            = 41
	DMPAPER_ISO_B4                        = 42
	DMPAPER_JAPANESE_POSTCARD             = 43
	DMPAPER_9X11                          = 44
	DMPAPER_10X11                         = 45
	DMPAPER_15X11                         = 46
	DMPAPER_ENV_INVITE                    = 47
	DMPAPER_RESERVED_48                   = 48
	DMPAPER_RESERVED_49                   = 49
	DMPAPER_LETTER_EXTRA                  = 50
	DMPAPER_LEGAL_EXTRA                   = 51
	DMPAPER_TABLOID_EXTRA                 = 52
	DMPAPER_A4_EXTRA                      = 53
	DMPAPER_LETTER_TRANSVERSE             = 54
	DMPAPER_A4_TRANSVERSE                 = 55
	DMPAPER_LETTER_EXTRA_TRANSVERSE       = 56
	DMPAPER_A_PLUS                        = 57
	DMPAPER_B_PLUS                        = 58
	DMPAPER_LETTER_PLUS                   = 59
	DMPAPER_A4_PLUS                       = 60
	DMPAPER_A5_TRANSVERSE                 = 61
	DMPAPER_B5_TRANSVERSE                 = 62
	DMPAPER_A3_EXTRA                      = 63
	DMPAPER_A5_EXTRA                      = 64
	DMPAPER_B5_EXTRA                      = 65
	DMPAPER_A2                            = 66
	DMPAPER_A3_TRANSVERSE                 = 67
	DMPAPER_A3_EXTRA_TRANSVERSE           = 68
	DMPAPER_DBL_JAPANESE_POSTCARD         = 69
	DMPAPER_A6                            = 70
	DMPAPER_JENV_KAKU2                    = 71
	DMPAPER_JENV_KAKU3                    = 72
	DMPAPER_JENV_CHOU3                    = 73
	DMPAPER_JENV_CHOU4                    = 74
	DMPAPER_LETTER_ROTATED                = 75
	DMPAPER_A3_ROTATED                    = 76
	DMPAPER_A4_ROTATED                    = 77
	DMPAPER_A5_ROTATED                    = 78
	DMPAPER_B4_JIS_ROTATED                = 79
	DMPAPER_B5_JIS_ROTATED                = 80
	DMPAPER_JAPANESE_POSTCARD_ROTATED     = 81
	DMPAPER_DBL_JAPANESE_POSTCARD_ROTATED = 82
	DMPAPER_A6_ROTATED                    = 83
	DMPAPER_JENV_KAKU2_ROTATED            = 84
	DMPAPER_JENV_KAKU3_ROTATED            = 85
	DMPAPER_JENV_CHOU3_ROTATED            = 86
	DMPAPER_JENV_CHOU4_ROTATED            = 87
	DMPAPER_B6_JIS                        = 88
	DMPAPER_B6_JIS_ROTATED                = 89
	DMPAPER_12X11                         = 90
	DMPAPER_JENV_YOU4                     = 91
	DMPAPER_JENV_YOU4_ROTATED             = 92
	DMPAPER_P16K                          = 93
	DMPAPER_P32K                          = 94
	DMPAPER_P32KBIG                       = 95
	DMPAPER_PENV_1                        = 96
	DMPAPER_PENV_2                        = 97
	DMPAPER_PENV_3                        = 98
	DMPAPER_PENV_4                        = 99
	DMPAPER_PENV_5                        = 100
	DMPAPER_PENV_6                        = 101
	DMPAPER_PENV_7                        = 102
	DMPAPER_PENV_8                        = 103
	DMPAPER_PENV_9                        = 104
	DMPAPER_PENV_10                       = 105
	DMPAPER_P16K_ROTATED                  = 106
	DMPAPER_P32K_ROTATED                  = 107
	DMPAPER_P32KBIG_ROTATED               = 108
	DMPAPER_PENV_1_ROTATED                = 109
	DMPAPER_PENV_2_ROTATED                = 110
	DMPAPER_PENV_3_ROTATED                = 111
	DMPAPER_PENV_4_ROTATED                = 112
	DMPAPER_PENV_5_ROTATED                = 113
	DMPAPER_PENV_6_ROTATED                = 114
	DMPAPER_PENV_7_ROTATED                = 115
	DMPAPER_PENV_8_ROTATED                = 116
	DMPAPER_PENV_9_ROTATED                = 117
	DMPAPER_PENV_10_ROTATED               = 118
)

type RTLOSVersionInfo struct {
	dwOSVersionInfoSize uint32
	dwMajorVersion      uint32
	dwMinorVersion      uint32
	dwBuildNumber       uint32
	dwPlatformId        uint32

	// WCHAR szCSDVersion[128]
	szCSDVersion, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _ uint16
}

func GetWindowsVersion() string {
	var osVersionInfo RTLOSVersionInfo
	osVersionInfo.dwOSVersionInfoSize = uint32(unsafe.Sizeof(osVersionInfo))
	r1, _, _ := rtlGetVersionProc.Call(uintptr(unsafe.Pointer(&osVersionInfo)))
	if r1 != 0 {
		// This is unimportant, so "unknown" is fine.
		return "unknown (error)"
	}

	version := fmt.Sprintf("%d.%d.%d", osVersionInfo.dwMajorVersion, osVersionInfo.dwMinorVersion, osVersionInfo.dwBuildNumber)

	servicePackVersion := utf16PtrToString((*uint16)(&osVersionInfo.szCSDVersion))
	if servicePackVersion != "" {
		version = fmt.Sprintf("%s %s", version, servicePackVersion)
	}

	return version
}

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var PRINTERS_DEVICE_CLASS = GUID{
	0x4d36e979,
	0xe325,
	0x11ce,
	[8]byte{0xbf, 0xc1, 0x08, 0x00, 0x2b, 0xe1, 0x03, 0x18},
}

type DevBroadcastDevinterface struct {
	dwSize       uint32
	dwDeviceType uint32
	dwReserved   uint32
	classGuid    GUID
	szName       uint16
}

const (
	DEVICE_NOTIFY_SERVICE_HANDLE        = 1
	DEVICE_NOTIFY_ALL_INTERFACE_CLASSES = 4

	DBT_DEVTYP_DEVICEINTERFACE = 5
)

func RegisterDeviceNotification(handle windows.Handle) error {

	var notificationFilter DevBroadcastDevinterface
	notificationFilter.dwSize = uint32(unsafe.Sizeof(notificationFilter))
	notificationFilter.dwDeviceType = DBT_DEVTYP_DEVICEINTERFACE
	notificationFilter.dwReserved = 0
	// BUG(pastarmovj): This class is ignored for now. Figure out what the right GUID is.
	notificationFilter.classGuid = PRINTERS_DEVICE_CLASS
	notificationFilter.szName = 0

	r1, _, err := registerDeviceNotificationProc.Call(uintptr(handle), uintptr(unsafe.Pointer(&notificationFilter)), DEVICE_NOTIFY_SERVICE_HANDLE|DEVICE_NOTIFY_ALL_INTERFACE_CLASSES)
	if r1 == 0 {
		return err
	}
	return nil
}

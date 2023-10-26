package ota

type SearchRQ struct {
	Version
	ReturnExtras     bool   `xml:"returnExtras,attr"`
	SupplierInfo     bool   `xml:"supplierInfo,attr"`
	ResidenceCountry string `xml:"cor,attr"`
	Credentials
	PickUp    PickUp  `xml:"PickUp"`
	DropOff   DropOff `xml:"DropOff"`
	DriverAge int     `xml:"DriverAge"`
}

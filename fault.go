package soap

import (
	"encoding/xml"
	"errors"
	"fmt"
)

var (
	// ErrFaultDetailPresentButNotSpecified is returned if the SOAP Fault details element is present but
	// the fault was not constructed with a type for it.
	ErrFaultDetailPresentButNotSpecified = errors.New("fault detail element present but no type supplied")
)

// Fault is a SOAP fault code.
type Fault struct {
	// XMLName is the serialized name of this object.
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Fault"`

	Code   string `xml:"faultcode,omitempty"`
	String string `xml:"faultstring,omitempty"`
	Actor  string `xml:"faultactor,omitempty"`

	// DetailInternal is a handle to the internal fault detail type. Do not directly access;
	// this is made public only to allow for XML deserialization.
	// Use the Detail() method instead.
	DetailInternal *faultDetail `xml:"detail,omitempty"`
}

// NewFault returns a new XML fault struct
func NewFault() *Fault {
	return &Fault{}
}

// NewFaultWithDetail returns a new XML fault struct with a specified DetailInternal field
func NewFaultWithDetail(detail interface{}) *Fault {
	return &Fault{
		DetailInternal: &faultDetail{
			Content: detail,
		},
	}
}

// Detail exposes the type supplied during creation (if a type was supplied).
func (f *Fault) Detail() interface{} {
	if f.DetailInternal == nil {
		return nil
	}
	return f.DetailInternal.Content
}

// Error satisfies the Error() interface allowing us to return a fault as an error.
func (f *Fault) Error() string {
	return fmt.Sprintf("soap fault: %s (%s)", f.Code, f.String)
}

// faultDetail is an implementation detail of how we parse out the optional detail element of the XML fault.
type faultDetail struct {
	Content interface{} `xml:",omitempty"`
}

// UnmarshalXML is an overridden deserialization routine used to decode a SOAP fault.
// The elements are read from the decoder d, starting at the element start. The contents of the decode are stored
// in the invoking fault f. Any errors encountered are returned.
func (f *faultDetail) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	// We still want to decode what we can, even if we don't have a field to store the details in.
	if f.Content == nil {
		return ErrFaultDetailPresentButNotSpecified
	}

	for {
		token, err := d.Token()
		if err != nil {
			return err
		} else if token == nil {
			return nil
		}

		switch se := token.(type) {
		case xml.StartElement:
			if err = d.DecodeElement(f.Content, &se); err != nil {
				return err
			}
		case xml.EndElement:
			// If we're at the end XML element we are done and can return.
			return nil
		}
	}
}

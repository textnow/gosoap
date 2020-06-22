package soap

import (
	"encoding/xml"
	"errors"
)

const xsdNS = "http://www.w3.org/2001/XMLSchema"
const xsiNS = "http://www.w3.org/2001/XMLSchema-instance"
const soapEnvNS = "http://schemas.xmlsoap.org/soap/envelope/"

var (
	// ErrUnableToSignEmptyEnvelope is returned if the envelope to be signed is empty. This is not valid.
	ErrUnableToSignEmptyEnvelope = errors.New("unable to sign, envelope is empty")
	// ErrEnvelopeMisconfigured is returned if we attempt to deserialize a SOAP envelope without a type to deserialize the body or fault into.
	ErrEnvelopeMisconfigured = errors.New("envelope content or fault pointer empty")
)

// Envelope is a SOAP envelope.
type Envelope struct {
	// XMLName is the serialized name of this object.
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`

	// These are generic namespaces used by all messages.
	XMLNSXsd string `xml:"xmlns:xsd,attr,omitempty"`
	XMLNSXsi string `xml:"xmlns:xsi,attr,omitempty"`

	Header *Header
	Body   *Body
}

// NewEnvelope creates a new SOAP Envelope with the specified data as the content to serialize or deserialize.
// It defaults to a fault struct with no detail type.
// Headers are assumed to be omitted unless explicitly added via AddHeaders()
func NewEnvelope(content interface{}) *Envelope {
	return &Envelope{
		Body: &Body{
			Content: content,
		},
	}
}

// NewEnvelopeWithFault creates a new SOAP Envelope with the specified data as the content to serialize or deserialize.
// It uses the supplied fault detail struct when deserializing a potential SOAP fault.
// Headers are assumed to be omitted unless explicitly added via AddHeaders()
func NewEnvelopeWithFault(content interface{}, faultDetail interface{}) *Envelope {
	return &Envelope{
		Body: &Body{
			Content: content,
			Fault:   NewFaultWithDetail(faultDetail),
		},
	}
}

// AddHeaders adds additional headers to be serialized to the resulting SOAP envelope.
func (e *Envelope) AddHeaders(elems ...interface{}) {
	if e.Header == nil {
		e.Header = &Header{}
	}

	e.Header.Headers = append(e.Header.Headers, elems)
}

// signWithWSSEInfo takes the supplied auth info, uses the WS Security X.509 signing standard and adds the resulting header.
func (e *Envelope) signWithWSSEInfo(info *WSSEAuthInfo) error {
	e.XMLNSXsd = xsdNS
	e.XMLNSXsi = xsiNS

	if e.Body.Content == nil {
		return ErrUnableToSignEmptyEnvelope
	}

	e.Body.XMLNSWsu = wsuNS

	ids, err := generateWSSEAuthIds()
	if err != nil {
		return err
	}

	securityHeader, err := info.sign(*e.Body, ids)
	if err != nil {
		return err
	}

	e.AddHeaders(securityHeader)
	e.Body.ID = ids.bodyID

	return nil
}

// Header is a SOAP envelope header.
type Header struct {
	// XMLName is the serialized name of this object.
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Header"`

	// Headers is an array of envelope headers to send.
	Headers []interface{} `xml:",omitempty"`
}

// Body is a SOAP envelope body.
type Body struct {
	// XMLName is the serialized name of this object.
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Body"`

	// XMLNSWsu is the SOAP WS-Security utility namespace.
	XMLNSWsu string `xml:"xmlns:wsu,attr,omitempty"`
	// ID is a body ID used during WS-Security signing.
	ID string `xml:"wsu:Id,attr,omitempty"`

	// Fault is a SOAP fault we may detect in a response.
	Fault *Fault `xml:",omitempty"`
	// Body is a SOAP request or response body.
	Content interface{} `xml:",omitempty"`
}

// UnmarshalXML is an overridden deserialization routine used to decode a SOAP envelope body.
// The elements are read from the decoder d, starting at the element start. The contents of the decode are stored
// in the invoking body b. Any errors encountered are returned.
func (b *Body) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	if b.Content == nil {
		return ErrEnvelopeMisconfigured
	} else if b.Fault == nil {
		// We allow for a custom fault detail object to be supplied.
		// If it isn't there, we will set it to a default.
		// We can't set this on construction as we may be serializing a message and don't want to serialize an empty fault.
		b.Fault = NewFault()
	}

	for {
		token, err := d.Token()
		if err != nil {
			return err
		} else if token == nil {
			return nil
		}

		switch elem := token.(type) {
		case xml.StartElement:
			// If the start element is a fault decode it as a fault, otherwise parse it as content.
			if elem.Name.Space == soapEnvNS && elem.Name.Local == "Fault" {
				err = d.DecodeElement(b.Fault, &elem)
				if err != nil {
					return err
				}
				// Clear the content if we have a fault
				b.Content = nil
			} else {
				err = d.DecodeElement(b.Content, &elem)
				if err != nil {
					return err
				}
				// Clear the fault if we have content
				b.Fault = nil
			}
		case xml.EndElement:
			// We expect the Body to have a single entry, so once we encounter the end element we're done.
			return nil
		}
	}
}

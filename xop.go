package soap

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/beevik/etree"
	"io"
	"io/ioutil"
	"mime/multipart"
	"reflect"
	"strings"
)

// Implements an XOP decoder.
// This is used for any MIME multi-part SOAP responses we receive.

const (
	xopNS = "http://www.w3.org/2004/08/xop/include"
)

var (
	// ErrMultipartBodyEmpty is returned if a multi-part body that is empty is discovered
	ErrMultipartBodyEmpty = errors.New("multi-part body is empty")
	// ErrCannotSetBytesElement is an internal error that suggests our parse tree is malformed
	ErrCannotSetBytesElement = errors.New("cannot set the bytes element")
)

type xopDecoder struct {
	reader      io.Reader
	mediaParams map[string]string
	includes    map[string][]string
}

func newXopDecoder(r io.Reader, mediaParams map[string]string) *xopDecoder {
	d := &xopDecoder{
		includes:    make(map[string][]string),
		reader:      r,
		mediaParams: mediaParams,
	}
	return d
}

func getXopContentIDIncludePath(element *etree.Element) (string, []string) {
	for _, token := range element.Child {
		switch token := token.(type) {
		case *etree.Element:
			ns := token.Space
			href := ""

			for _, attr := range token.Attr {
				if attr.Key == ns {
					ns = attr.Value
				} else if attr.Key == "href" {
					href = attr.Value
				}
			}

			if ns == xopNS && token.Tag == "Include" {
				cleanedHref := strings.Replace(href, "cid:", "", 1)
				// This is a super ugly hack reflecting how these URIs are stored in the HTTP header
				return "<" + cleanedHref + ">", []string{}
			}

			retHref, retPath := getXopContentIDIncludePath(token)

			if len(retHref) > 0 {
				return retHref, append([]string{token.Tag}, retPath...)
			}
		default:
			continue
		}
	}

	return "", []string{}
}

func (d *xopDecoder) decode(respEnvelope *Envelope) error {
	parts := multipart.NewReader(d.reader, d.mediaParams["boundary"])

	for {
		part, err := parts.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		} else if part == nil {
			return ErrMultipartBodyEmpty
		}

		if strings.Contains(part.Header.Get("Content-Type"), "application/xop+xml") {
			doc := etree.NewDocument()
			_, err = doc.ReadFrom(part)
			if err != nil {
				return err
			}

			root := doc.Root()

			xopHref, xopObjPath := getXopContentIDIncludePath(root)

			if len(xopHref) > 0 {
				d.includes[xopHref] = xopObjPath
			}

			pipeReader, pipeWriter := io.Pipe()

			go func() {
				// Here we re-serialize the object to a pipe for easy deserialization by the standard XML library
				doc.WriteTo(pipeWriter)
				defer pipeWriter.Close()
			}()

			err = xml.NewDecoder(pipeReader).Decode(&respEnvelope)
			if err != nil {
				return err
			}

			if len(d.includes) < 1 {
				// We don't have anything more to parse.
				break
			}

			// Don't break here; we want to loop through and decode the XOP includes we found in the XML body above.
		} else if strings.Contains(part.Header.Get("Content-Type"), "application/octet-stream") {
			if xopObjPath, ok := d.includes[part.Header.Get("Content-ID")]; ok {
				if xopObjPath[0] != "Body" {
					return fmt.Errorf("invalid XOP Include object path (should start with Body): %v", xopObjPath)
				}

				partBytes, err := ioutil.ReadAll(part)
				if err != nil {
					return err
				}

				// We kind of cheat here and skip the normal 'Envelope/Body' parsing since we know the format
				// of the message is valid if we properly deserialized the XML object above.
				// We only support envelope-encoded messages at this point anyways,
				// and it prevents us from needing to hack around with the generic Content interface stuff here.
				rResponse := reflect.ValueOf(respEnvelope.Body.Content).Elem()

				// WARNING: Dragons below.
				// We iterate the Content struct.
				// We attempt to confirm that this is the proper object to store the data in, by
				// examining the XMLName field and confirming that it matches the first element found in the XOP include path.
				// Next, we attempt to find the field identified by the second element found in the XOP include path
				// by looking for the XML tag that matches.
				// We use Contains, instead of ==, because often the tag includes ',omitempty' as a trailer.
				// Once we've got it, we confirm we can set it and then call SetBytes on the element.
				for i := 0; i < rResponse.NumField(); i++ {
					rTypeField := rResponse.Type().Field(i)
					rValueField := rResponse.Field(i)

					if rTypeField.Name == "XMLName" {
						rXMLName := rValueField

						// The XMLName field has 2 properties; one called 'Space' and one called 'Local'.
						// We only care about 'Local' right now.
						// We assume the namespace is correct if the local name is correct.
						for j := 0; j < rXMLName.NumField(); j++ {
							rXMLTypeField := rXMLName.Type().Field(i)
							rXMLValueField := rXMLName.Field(i)

							if rXMLTypeField.Name == "Local" && rXMLValueField.String() != xopObjPath[1] {
								return fmt.Errorf("invalid XML object path, expected %s, got %s", rXMLValueField.String(), xopObjPath[1])
							}
						}
					} else if tagValue, tagOk := rTypeField.Tag.Lookup("xml"); tagOk {
						if !strings.Contains(tagValue, xopObjPath[2]) {
							continue
						}

						bytesElem := rValueField.Elem()

						if !bytesElem.CanSet() {
							return ErrCannotSetBytesElement
						}

						bytesElem.SetBytes(partBytes)
					}
				}
				break
			}
		} else {
			// If we have a multipart message with anything else, assume it isn't XOP formatted and return here.
			err = xml.NewDecoder(part).Decode(&respEnvelope)
			if err != nil {
				return err
			}

			break
		}
	}

	return nil
}

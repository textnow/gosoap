package soap

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"reflect"
	"strings"

	"github.com/beevik/etree"
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
	// ErrMissingXOPPart is returned if the decoded body was missing the XOP header
	ErrMissingXOPPart = errors.New("did not find an xop part for this multipart message")
)

var (
	errFieldNotFound = errors.New("field not found")
	errFieldNotArray = errors.New("field not an array")
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

func (d *xopDecoder) getXopContentIDIncludePath(element *etree.Element, path []string) {
	for _, token := range element.Child {
		switch token := token.(type) {
		case *etree.Element:
			ns := token.Space
			href := ""

			for _, attr := range token.Attr {
				if attr.Key == "xmlns" {
					ns = attr.Value
				} else if attr.Key == "href" {
					href = attr.Value
				}
			}

			if ns == xopNS && token.Tag == "Include" {
				cleanedHref := strings.Replace(href, "cid:", "", 1)
				// This is a super ugly hack reflecting how these URIs are stored in the HTTP header
				// This is an ugly way to make sure we copy the value of path without subsequent modifications
				d.includes["<" + cleanedHref + ">"] = append([]string(nil), path...)
				break
			}

			d.getXopContentIDIncludePath(token, append(path, []string{token.Tag}...))
		default:
			continue
		}
	}
}

func getFieldByName(element reflect.Value, name string) (reflect.Value, error) {
	for i := 0; i < element.NumField(); i++ {
		rTypeField := element.Type().Field(i)
		rValueField := element.Field(i)

		if rTypeField.Name == name {
			return rValueField, nil
		}
	}
	return reflect.Value{}, errFieldNotFound
}

func getFieldByXMLTagPrefix(element reflect.Value, prefix string) (reflect.Value, error) {
	if element.Kind() == reflect.Interface || element.Kind() == reflect.Ptr {
		element = element.Elem()
	}
	if element.Kind() == reflect.Array || element.Kind() == reflect.Slice {
		return getFieldByXMLTagPrefix(element.Index(0), prefix)
	}
	if element.Kind() != reflect.Struct {
		return reflect.Value{}, errFieldNotFound
	}

	for i := 0; i < element.NumField(); i++ {
		rTypeField := element.Type().Field(i)
		rValueField := element.Field(i)

		if tagValue, tagOk := rTypeField.Tag.Lookup("xml"); tagOk {
			if !strings.HasPrefix(tagValue, prefix) {
				continue
			}

			return rValueField, nil
		}
	}
	return reflect.Value{}, errFieldNotFound
}

func getFieldFromIncludePath(val reflect.Value, path []string) (reflect.Value, error) {
	if path[0] != "Body" {
		return reflect.Value{}, fmt.Errorf("invalid Include path (should start with Body): %v", path)
	}

	// WARNING: Dragons below.
	// We iterate the Content struct.
	// We attempt to confirm that this is the proper object to store the data in, by
	// examining the XMLName field and confirming that it matches the first element found in the XOP include path.
	// Next, we attempt to find the field identified by the subsequent elements found in the XOP include path
	// by looking for the XML tag that matches then repeating until we reach the end of the include path.
	// We use HasPrefix, instead of ==, because often the tag includes ',omitempty' as a trailer.
	// Once we've got it, we return the element.

	rXMLName, err := getFieldByName(val.Elem(), "XMLName")
	if err != nil {
		return rXMLName, err
	}

	// The XMLName field has 2 properties; one called 'Space' and one called 'Local'.
	// We only care about 'Local' right now.
	// We assume the namespace is correct if the local name is correct.
	for i := 0; i < rXMLName.NumField(); i++ {
		rXMLTypeField := rXMLName.Type().Field(i)
		rXMLValueField := rXMLName.Field(i)

		if rXMLTypeField.Name == "Local" && rXMLValueField.String() != path[1] {
			return reflect.Value{}, fmt.Errorf("invalid XML object path, expected %s, got %s", rXMLValueField.String(), path[1])
		}
	}

	rValueField := val.Elem()
	for i := 2; i < len(path); i++ {
		rValueField, err = getFieldByXMLTagPrefix(rValueField, path[i])
		if err != nil {
			return rValueField, err
		}
	}

	return rValueField, nil
}

func (d *xopDecoder) decode(respEnvelope *Envelope) error {
	parts := multipart.NewReader(d.reader, d.mediaParams["boundary"])
	parsedXOPHeader := false
	partNumber := 0

	for {
		part, err := parts.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		} else if part == nil {
			return ErrMultipartBodyEmpty
		} else if !parsedXOPHeader && partNumber > 0 {
			return ErrMissingXOPPart
		}

		partNumber++

		// If the content-type is xop+xml it means we have our first object, the one we will be storing things in.
		// Find the include paths in it, store them, and then we'll proceed to the rest of the parts to put them into this document.
		if strings.Contains(part.Header.Get("Content-Type"), "application/xop+xml") {
			parsedXOPHeader = true
			doc := etree.NewDocument()
			_, err = doc.ReadFrom(part)
			if err != nil {
				return err
			}

			root := doc.Root()

			d.getXopContentIDIncludePath(root, nil)

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

			// We do not attempt to handle the 'parts' parsing here. That will come on subsequent loop iterations.
			continue
		}

		// We're now going through the part to put this part into the proper 'bytes' field of the struct deserialized above.
		if xopObjPath, ok := d.includes[part.Header.Get("Content-ID")]; ok {
			// We kind of cheat here and skip the normal 'Envelope/Body' parsing since we know the format
			// of the message is valid if we properly deserialized the XML object above.
			// We only support envelope-encoded messages at this point anyways,
			// and it prevents us from needing to hack around with the generic Content interface stuff here.
			rResponse := reflect.ValueOf(respEnvelope.Body.Content)

			field, err := getFieldFromIncludePath(rResponse, xopObjPath)
			if err != nil {
				return err
			}

			// TODO: Double check this is a byte array
			if !field.CanSet() {
				return ErrCannotSetBytesElement
			}

			// We don't read the content until we know we're able to save it (no point reading something we'll never store).
			partBytes, err := ioutil.ReadAll(part)
			if err != nil {
				return err
			}

			field.SetBytes(partBytes)
		}
	}

	return nil
}

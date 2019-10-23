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
	xmlName = "XMLName"
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

	root := val.Elem()

	// in the following order, get the root's XML name from
	// - the tag of the XMLName field, if it is a struct
	// - the name of the field type
	rootName := ""
	if rootName := getExplicitXMLName(root); rootName == "" {
		rootName = root.Type().Name()
	}

	// verify that the root elements match
	if rootName != path[1] {
		return reflect.Value{}, fmt.Errorf("invalid XML object path, expected %s, got %s", rootName, path[1])
	}

	// reflect Values to be searched
	// embedded fields are added to elems so that their fields are also searched
	elems := []reflect.Value{root}

	// the index of the next node in the path
	i := 2

	// find the desired field from the include path
	for len(elems) > 0 {
		elem := elems[0]
		elems = elems[1:]

		for j := 0; j < elem.NumField(); j++ {
			typeField := elem.Type().Field(j)
			valueField := elem.Field(j)
			tag := typeField.Tag.Get("xml")

			// skip the XMLName field
			if typeField.Name == xmlName {
				continue
			}

			// skip omitted fields
			if tag == "-" {
				continue
			}

			// unwrap the value
			valueField = unwrapValue(valueField)

			// check if the value was unwrapped completely
			if valueField.Type().Kind() == reflect.Slice || valueField.Type().Kind() == reflect.Array {
				// if valueField is in path
				if getNameFromTag(tag) == path[i] {
					// if valueField is the desired field, return
					if i++; i == len(path) {
						return valueField, nil
					}

					// valueField is empty, there is nothing more to search
					return reflect.Value{}, errFieldNotFound
				}

				// valueField is not in path
				continue
			}

			// if the field is an embedded struct, search its fields
			if typeField.Anonymous {
				elems = append(elems, valueField)
				continue
			}

			// in the following order, get the field's XML name from
			// - the tag on the field
			// - the tag of the XMLName field of valueField, if it is a struct
			// - the name of the field type
			fieldName := ""
			if fieldName = getNameFromTag(tag); fieldName == "" {
				if fieldName = getExplicitXMLName(valueField); fieldName == "" {
					fieldName = typeField.Name
				}
			}

			// once the next elem in the path is found, restart with it as root
			if fieldName == path[i] {
				if i++; i == len(path) {
					return valueField, nil
				}

				elems = []reflect.Value{valueField}
				break
			}
		}
	}

	return reflect.Value{}, errFieldNotFound
}

// Unwrap value as much as possible. If a value is an empty array or slice, it can no longer be unwrapped and is returned
// This assumes, if it encounters an array field, that it will be filling in the first field and continuing
// TODO: support arbitrary array size.
func unwrapValue(val reflect.Value) reflect.Value {
	// if the value is an interface or pointer, get its value
	if val.Type().Kind() == reflect.Ptr || val.Type().Kind() == reflect.Interface {
		return unwrapValue(val.Elem())
	}

	// if the value is an array or a slice, assume that we are looking for its first element
	if val.Type().Kind() == reflect.Slice || val.Type().Kind() == reflect.Array {
		// if the value can no longer be unwrapped, return
		if val.Cap() == 0 {
			return val
		}

		return unwrapValue(val.Index(0))
	}

	// the value has been unwrapped
	return val
}

func getNameFromTag(tag string) string {
	// if the tag is not set then there is no name to obtain
	if tag == "" {
		return ""
	}

	// remove xml namespace from the front of the tag
	parts := strings.Split(tag, " ")
	tag = parts[len(parts) - 1]

	// return the XMLName from the front of the remaining tag
	return strings.Split(tag, ",")[0]
}

// getExplicitXMLName gets the xml name which is explicitly set in the xml tag on the XMLName field
func getExplicitXMLName(val reflect.Value) string {
	// only a value of type reflect.Struct can have an XMLName field
	if val.Type().Kind() == reflect.Struct {
		// get the XMLName from the XMLName field, if possible
		for i := 0; i < val.NumField(); i++ {
			typeField := val.Type().Field(i)

			if typeField.Name == xmlName {
				// get the XMLName
				name := getNameFromTag(typeField.Tag.Get("xml"))

				// if the name has been set, return
				if name != "" {
					return name
				}

				break
			}
		}
	}

	// xml name not explicitly set
	return ""
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

			if !field.CanSet() {
				return ErrCannotSetBytesElement
			}

			// double check field is a slice of bytes
			if field.Type().String() != "[]uint8" {
				return errFieldNotArray
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

package soap

import (
	"encoding/xml"
	"errors"
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

func getFieldFromPath(val reflect.Value, path []string) (reflect.Value, error) {
	val = unwrapValue(val)

	// val must be a struct and path must have length > 0
	if val.Type().Kind() != reflect.Struct || len(path) == 0 {
		return reflect.Value{}, errFieldNotFound
	}

	// search the struct fields with the path
	for i := 0; i < val.NumField(); i++ {
		typeField := val.Type().Field(i)
		valueField := val.Field(i)
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
		if valueField.Type().Kind() == reflect.Array || valueField.Type().Kind() == reflect.Slice || valueField.Type().Kind() == reflect.Ptr {
			// if valueField is in path
			if getNameFromTag(tag) == path[0] {
				// if valueField is the desired field, return
				if len(path) == 1 {
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
			result, err := getFieldFromPath(valueField, path)
			if err == nil {
				return result, nil
			}

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
		if fieldName == path[0] {
			if len(path) == 1 {
				return valueField, nil
			}

			return getFieldFromPath(valueField, path[1:])
		}
	}

	return reflect.Value{}, errFieldNotFound
}

// Unwrap value as much as possible. A value can no longer be unwrapped if:
// - it is an empty array or slice
// - it is a nil pointer
// This assumes, if it encounters an array field, that it will be filling in the first field and continuing
// TODO: support arbitrary array size.
func unwrapValue(val reflect.Value) reflect.Value {
	// if the value is an interface or pointer, get its value
	if val.Type().Kind() == reflect.Ptr || val.Type().Kind() == reflect.Interface {
		// if the value is a nil pointer
		if val.IsNil() {
			return val
		}

		return unwrapValue(val.Elem())
	}

	// if the value is an array or a slice, assume that we are looking for its first element
	if val.Type().Kind() == reflect.Slice || val.Type().Kind() == reflect.Array {
		// if the value is an empty array or slice
		if val.Cap() == 0 {
			return val
		}

		return unwrapValue(val.Index(0))
	}

	// the value has been unwrapped
	return val
}

// getNameFromTag gets the name from an xml tag
// tags take the form: "-" or "namespace name,flag1,flag2,..."
// the namespace, name and flags are all optional
// a tag cannot contain a namespace but not a name
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
	if val.Type().Kind() != reflect.Struct {
		return ""
	}

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
			rResponse := reflect.ValueOf(respEnvelope)

			field, err := getFieldFromPath(rResponse, xopObjPath)
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

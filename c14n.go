package soap

import (
	"errors"
	"fmt"

	"github.com/beevik/etree"
)

var (
	errInvalidCanonicalizationPath = errors.New("invalid path to canonicalize")
)

// Performs a basic version of the Exclusive C14N canonicalization required for WS-Security.
// The spec is the best reference for this, even if it is a bit involved.

// canonicalize takes a well-formed, serialized XML document and uses the Exclusive C14N canonicalization
// algorithm on the supplied root element search string, and returns the resulting document.
// See https://www.w3.org/TR/xml-exc-c14n/ for details on Exclusive C14N canonicalization
// NOTE: This is a basic implementation that supports trivial XML.
// It has not been tested with a comprehensive collection of possible input documents.
// It happens to work with the XML documents we are generating in this project.
func canonicalize(bytes []byte, rootElement string) ([]byte, error) {
	var nsIdx int
	nsMap := map[string]string{}
	nsIdx = 1

	existing := etree.NewDocument()
	err := existing.ReadFromBytes(bytes)
	if err != nil {
		return nil, err
	}

	canonicalDoc := etree.NewDocument()
	canonicalDoc.WriteSettings.CanonicalEndTags = true

	canonicalRoot := existing.Root().Copy()
	canonicalDoc.SetRoot(canonicalRoot)

	startElem := canonicalDoc.FindElement(rootElement)

	if startElem == nil {
		return nil, errInvalidCanonicalizationPath
	}

	canonicalizeChildren(startElem, &nsIdx, nsMap)

	return canonicalDoc.WriteToBytes()
}

// canonicalizeChildren takes an element and an existing map of namespaces, and recursively canonicalizes all child nodes.
// If a new namespace is encountered a handle is generated using the nsIdx value, and that namespace is added
// to the nsMap argument.
// If an existing namespace is found the existing entry in nsMap is used to prefix the element name.
// This will, upon completion, yield the Exclusive C14N XML representation.
// We skip the Envelope namespace since we don't want to remove the namespace of the root object.
// TODO: determine a cleaner way to handle this.
func canonicalizeChildren(element *etree.Element, nsIdx *int, nsMap map[string]string) {
	// This is a redundant namespace if we don't depend on it.
	for _, token := range element.Child {
		switch token := token.(type) {
		case *etree.Element:
			canonNs := token.Parent().Space
			for _, attr := range token.Attr {
				// Here we find or define a short-hand reference for the namespace
				if attr.Key == "xmlns" {
					if attr.Value == soapEnvNS {
						continue
					}
					if existingNs, ok := nsMap[attr.Value]; ok {
						canonNs = existingNs
					} else {
						canonNs = fmt.Sprintf("ns%d", *nsIdx)
						*nsIdx++
						nsMap[attr.Value] = canonNs
						token.CreateAttr("xmlns:"+canonNs, attr.Value)
					}
				}
			}

			token.Space = canonNs
			token.RemoveAttr("xmlns")
			canonicalizeChildren(token, nsIdx, nsMap)
		default:
			continue
		}
	}
}

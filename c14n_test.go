package soap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type canonicalizationTest struct {
	name             string
	origXML          []byte
	canonicalizeFrom string
	result           []byte
	err              error
}

var canonicalizationTests = []canonicalizationTest{
	{
		name:             "base case",
		origXML:          []byte(`<?xml version="1.0"?><request xmlns="http://example.com/interfaces/example/v1/request.xsd"><object1><subobject1><field1>asdf</field1><field2>2</field2></subobject1></object1><object2>1234asdf</object2></request>`),
		canonicalizeFrom: "",
		result:           []byte(`<ns1:request xmlns:ns1="http://example.com/interfaces/example/v1/request.xsd"><ns1:object1><ns1:subobject1><ns1:field1>asdf</ns1:field1><ns1:field2>2</ns1:field2></ns1:subobject1></ns1:object1><ns1:object2>1234asdf</ns1:object2></ns1:request>`),
		err:              nil,
	},
	{
		name:             "canonicalize child case",
		origXML:          []byte(`<?xml version="1.0"?><Envelope xmlns="http://schemas.xmlsoap.org/soap/envelope/" xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><Header xmlns="http://schemas.xmlsoap.org/soap/envelope/"><headerElement>asdf</headerElement></Header><Body xmlns="http://schemas.xmlsoap.org/soap/envelope/"><request xmlns="http://example.com/interfaces/example/v1/request.xsd"><object1><subobject1><field1>asdf</field1><field2>2</field2></subobject1></object1><object2>1234asdf</object2></request></Body></Envelope>`),
		canonicalizeFrom: "Envelope/Body",
		result:           []byte(`<Envelope xmlns="http://schemas.xmlsoap.org/soap/envelope/" xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><Header xmlns="http://schemas.xmlsoap.org/soap/envelope/"><headerElement>asdf</headerElement></Header><Body xmlns="http://schemas.xmlsoap.org/soap/envelope/"><ns1:request xmlns:ns1="http://example.com/interfaces/example/v1/request.xsd"><ns1:object1><ns1:subobject1><ns1:field1>asdf</ns1:field1><ns1:field2>2</ns1:field2></ns1:subobject1></ns1:object1><ns1:object2>1234asdf</ns1:object2></ns1:request></Body></Envelope>`),
		err:              nil,
	},
	{
		name:             "xml with added end tags case",
		origXML:          []byte(`<?xml version="1.0"?><request xmlns="http://example.com/interfaces/example/v1/request.xsd"><object1><subobject1><field1>asdf</field1><field2 attr1="1" /></subobject1></object1><object2>1234asdf</object2></request>`),
		canonicalizeFrom: "",
		result:           []byte(`<ns1:request xmlns:ns1="http://example.com/interfaces/example/v1/request.xsd"><ns1:object1><ns1:subobject1><ns1:field1>asdf</ns1:field1><ns1:field2 attr1="1"></ns1:field2></ns1:subobject1></ns1:object1><ns1:object2>1234asdf</ns1:object2></ns1:request>`),
		err:              nil,
	},
	{
		name:             "invalid path 'asdf' case",
		origXML:          []byte(`<?xml version="1.0"?><request xmlns="http://example.com/interfaces/example/v1/request.xsd"><object1><subobject1><field1>asdf</field1><field2>2</field2></subobject1></object1><object2>1234asdf</object2></request>`),
		canonicalizeFrom: "asdf",
		result:           nil,
		err:              errInvalidCanonicalizationPath,
	},
}

func TestCanonicalization(t *testing.T) {
	for _, tt := range canonicalizationTests {
		t.Run(tt.name, func(t *testing.T) {
			ret, err := canonicalize(tt.origXML, tt.canonicalizeFrom)
			assert.Equal(t, tt.err, err)
			assert.Equal(t, tt.result, ret)
		})
	}
}

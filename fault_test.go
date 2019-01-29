package soap

import (
	"bytes"
	"encoding/xml"
	"reflect"
	"strconv"
	"testing"
)

var faultName = xml.Name{
	Space: soapEnvNS,
	Local: "Fault",
}

type faultDetailExampleField struct {
	XMLName xml.Name `xml:"DetailField"`
	Attr1   string   `xml:"attr1,attr"`
	Attr2   int32    `xml:"attr2,attr"`
	Value   string   `xml:",chardata"`
}

type faultDetailExample struct {
	XMLName xml.Name                `xml:"DetailExample"`
	Attr1   int32                   `xml:"attr1,attr"`
	Field1  faultDetailExampleField `xml:"DetailField"`
}

type faultDecodeTest struct {
	in          string
	detailPtr   interface{}
	out         interface{}
	faultErrStr string
	err         error
}

var faultDecodeTests = []faultDecodeTest{
	{
		in: `<?xml version="1.0" encoding="UTF-8"?>
		<Fault xmlns="http://schemas.xmlsoap.org/soap/envelope/">
			<faultcode>FaultCodeValue</faultcode>
			<faultstring>FaultStringValue</faultstring>
			<faultactor>FaultActorValue</faultactor>
		</Fault>`,
		out: &Fault{
			XMLName: faultName,
			Code:    "FaultCodeValue",
			String:  "FaultStringValue",
			Actor:   "FaultActorValue",
		},
		faultErrStr: "soap fault: FaultCodeValue (FaultStringValue)",
	},
	{
		in: `<?xml version="1.0" encoding="UTF-8"?>
		<Fault xmlns="http://schemas.xmlsoap.org/soap/envelope/">
			<faultcode>FaultCodeValue</faultcode>
			<faultstring>FaultStringValue</faultstring>
			<faultactor>FaultActorValue</faultactor>
			<detail>
				<DetailExample attr1="10">
					<DetailField attr1="test" attr2="11">This is a test string</DetailField>
				</DetailExample>
			</detail>
		</Fault>`,
		detailPtr: new(faultDetailExample),
		out: &Fault{
			XMLName: faultName,
			Code:    "FaultCodeValue",
			String:  "FaultStringValue",
			Actor:   "FaultActorValue",
			DetailInternal: &faultDetail{
				Content: &faultDetailExample{
					XMLName: xml.Name{"", "DetailInternal"},
					Attr1:   10,
					Field1: faultDetailExampleField{
						XMLName: xml.Name{"", "DetailField"},
						Attr1:   "test",
						Attr2:   11,
						Value:   "This is a test string",
					},
				},
			},
		},
		faultErrStr: "soap fault: FaultCodeValue (FaultStringValue)",
	},
	{
		in: `<?xml version="1.0" encoding="UTF-8"?>
		<Fault xmlns="http://schemas.xmlsoap.org/soap/envelope/">
			<faultcode>FaultCodeValue</faultcode>
			<faultstring>FaultStringValue</faultstring>
			<faultactor>FaultActorValue</faultactor>
			<detail>
				<DetailExample attr1="10" />
			</detail>
		</Fault>`,
		detailPtr: new(faultDetailExample),
		out: &Fault{
			XMLName: faultName,
			Code:    "FaultCodeValue",
			String:  "FaultStringValue",
			Actor:   "FaultActorValue",
			DetailInternal: &faultDetail{
				Content: &faultDetailExample{
					XMLName: xml.Name{"", "DetailInternal"},
					Attr1:   10,
					Field1:  faultDetailExampleField{},
				},
			},
		},
		faultErrStr: "soap fault: FaultCodeValue (FaultStringValue)",
	},
	{
		in: `<?xml version="1.0" encoding="UTF-8"?>
		<Fault xmlns="http://schemas.xmlsoap.org/soap/envelope/">
			<faultcode>FaultCodeValue</faultcode>
			<faultstring>FaultStringValue</faultstring>
			<faultactor>FaultActorValue</faultactor>
			<detail>
				<DetailExample attr1="10">
					<DetailField attr1="test" attr2="11">This is a test string</DetailField>
				</DetailExample>
				<DetailExample attr1="11">
					<DetailField attr1="test2" attr2="12">This is a second test string</DetailField>
				</DetailExample>
			</detail>
		</Fault>`,
		detailPtr: new([]faultDetailExample),
		out: &Fault{
			XMLName: faultName,
			Code:    "FaultCodeValue",
			String:  "FaultStringValue",
			Actor:   "FaultActorValue",
			DetailInternal: &faultDetail{
				Content: []faultDetailExample{
					{
						XMLName: xml.Name{"", "DetailInternal"},
						Attr1:   10,
						Field1: faultDetailExampleField{
							XMLName: xml.Name{"", "DetailField"},
							Attr1:   "test",
							Attr2:   11,
							Value:   "This is a test string",
						},
					},
					{
						XMLName: xml.Name{"", "DetailInternal"},
						Attr1:   11,
						Field1: faultDetailExampleField{
							XMLName: xml.Name{"", "DetailField"},
							Attr1:   "test2",
							Attr2:   12,
							Value:   "This is a second test string",
						},
					},
				},
			},
		},
		faultErrStr: "soap fault: FaultCodeValue (FaultStringValue)",
	},
	{
		in: `<?xml version="1.0" encoding="UTF-8"?>
		<Fault xmlns="http://schemas.xmlsoap.org/soap/envelope/">
			<faultcode>FaultCodeValue</faultcode>
			<faultstring>FaultStringValue</faultstring>
			<faultactor>FaultActorValue</faultactor>
			<detail>
			</detail>
		</Fault>`,
		detailPtr: new(faultDetailExample),
		out: &Fault{
			XMLName: faultName,
			Code:    "FaultCodeValue",
			String:  "FaultStringValue",
			Actor:   "FaultActorValue",
			DetailInternal: &faultDetail{
				Content: &faultDetailExample{},
			},
		},
		faultErrStr: "soap fault: FaultCodeValue (FaultStringValue)",
	},
	{
		in: `<?xml version="1.0" encoding="UTF-8"?>
		<Fault xmlns="http://schemas.xmlsoap.org/soap/envelope/">
			<faultcode>FaultCodeValue</faultcode>
			<faultstring>FaultStringValue</faultstring>
			<faultactor>FaultActorValue</faultactor>
			<detail>
				<DetailExample attr1="10">
					<DetailField attr1="test" attr2="11">This is a test string</DetailField>
				</DetailExample>
			</detail>
		</Fault>`,
		detailPtr: nil,
		out: &Fault{
			XMLName: faultName,
			Code:    "FaultCodeValue",
			String:  "FaultStringValue",
			Actor:   "FaultActorValue",
		},
		err: ErrFaultDetailPresentButNotSpecified,
	},
	{
		in: `<?xml version="1.0" encoding="UTF-8"?>
		<Fault xmlns="http://schemas.xmlsoap.org/soap/envelope/">
			<faultcode>FaultCodeValue</faultcode>
			<faultstring>FaultStringValue</faultstring>
			<faultactor>FaultActorValue</faultactor>
			<detail>
				<DetailExample attr1="10
					<DetailField attr1="test" attr2="11">This is a test string</DetailField>
				</DetailExample>
			</detail>
		</Fault>`,
		detailPtr: new(faultDetailExample),
		out: &Fault{
			XMLName: faultName,
			Code:    "FaultCodeValue",
			String:  "FaultStringValue",
			Actor:   "FaultActorValue",
		},
		err: &xml.SyntaxError{Msg: "unescaped < inside quoted string", Line: 8},
	},
	{
		in: `<?xml version="1.0" encoding="UTF-8"?>
		<Fault xmlns="http://schemas.xmlsoap.org/soap/envelope/">
			<faultcode>FaultCodeValue</faultcode>
			<faultstring>FaultStringValue</faultstring>
			<faultactor>FaultActorValue</faultactor>
			<detail>
				<DetailExample attr1="asdf">
					<DetailField attr1="test" attr2="11">This is a test string</DetailField>
				</DetailExample>
			</detail>
		</Fault>`,
		detailPtr: new(faultDetailExample),
		out: &Fault{
			XMLName: faultName,
			Code:    "FaultCodeValue",
			String:  "FaultStringValue",
			Actor:   "FaultActorValue",
		},
		err: &strconv.NumError{Func: "ParseInt", Num: "asdf", Err: strconv.ErrSyntax},
	},
}

func TestFaultDecode(t *testing.T) {
	for i, tt := range faultDecodeTests {
		var val *Fault
		if tt.detailPtr != nil {
			val = NewFaultWithDetail(tt.detailPtr)
		} else {
			val = NewFault()
		}
		dec := xml.NewDecoder(bytes.NewReader([]byte(tt.in)))

		if err := dec.Decode(val); !reflect.DeepEqual(err, tt.err) {
			t.Errorf("#%d: %v, want %v", i, err, tt.err)
			continue
		} else if err != nil {
			continue
		}
		valStr, _ := xml.Marshal(val)
		outStr, _ := xml.Marshal(tt.out)
		if string(valStr) != string(outStr) {
			t.Errorf("#%d: mismatch\nhave: %#+v\nwant: %#+v", i, val, tt.out)
			println(string(valStr))
			println(string(outStr))
			continue
		}
		if tt.faultErrStr != val.Error() {
			t.Errorf("#%d: mismatch\nhave %s\n want: %s", i, val.Error(), tt.faultErrStr)
		}
		if !reflect.DeepEqual(tt.detailPtr, val.Detail()) {
			t.Errorf("#%d: mismatch\nhave %#+v\nwant: %#+v", i, tt.detailPtr, val.Detail())
		}
	}
}

package soap

import (
	"encoding/xml"
	"mime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type DataType string

type RunTimeSeriesReportResponse struct {
	XMLName xml.Name `xml:"RunTimeSeriesReportResponse"`

	*ReportResponse
}

type ReportResponse struct {
	*DelegateServiceResponse

	Report *Report `xml:"Report,omitempty"`
}

type DelegateServiceResponse struct {
	XMLName xml.Name `xml:"delegateServiceResponse"`

	*ResponseData
}

type ResponseData struct {
	Result Result `xml:"Result,omitempty"`

	Errors Errors `xml:"Errors,omitempty"`

	RequestProcessingTime int64 `xml:"RequestProcessingTime,omitempty"`
}

type Result string

type Errors struct {
	Error []*Error `xml:"Error,omitempty"`
}

type Error struct {
	Code string `xml:"Code,omitempty"`

	Message string `xml:"Message,omitempty"`
}

type Report struct {
	XMLName xml.Name `xml:"Report"`

	DataSets struct {
		DataSet []*ReportDataSet `xml:"DataSet,omitempty"`
	} `xml:"DataSets,omitempty"`

	ReportDuration int64 `xml:"ReportDuration,omitempty"`

	NumberOfDataSets int32 `xml:"NumberOfDataSets,omitempty"`
}

type ReportDataSet struct {
	Columns struct {
		Column []*Column `xml:"Column,omitempty"`
	} `xml:"Columns,omitempty"`

	CsvAttachment *CsvAttachment `xml:"CsvAttachment,omitempty"`

	Type string `xml:"Type,omitempty"`
}

type Column struct {
	Name string `xml:"Name,omitempty"`

	DataType *DataType `xml:"DataType,omitempty"`

	StaticValue string `xml:"StaticValue,omitempty"`

	DynamicIndex int32 `xml:"DynamicIndex,omitempty"`
}

type CsvAttachment struct {
	XMLName xml.Name `xml:"CsvAttachment"`

	CsvAttachmentFormat *CsvAttachmentFormat `xml:"CsvAttachmentFormat,omitempty"`

	CsvData []byte `xml:"CsvData,omitempty"`
}

type CsvAttachmentFormat struct {
	XMLName xml.Name `xml:"CsvAttachmentFormat"`

	*CharacterAttachmentFormat

	Headers bool `xml:"Headers,omitempty"`
}

type CharacterAttachmentFormat struct {
	XMLName xml.Name `xml:"CharacterAttachmentFormat"`

	*AttachmentFormat

	Encoding string `xml:"Encoding,omitempty"`
}

type CompressionType string

type AttachmentFormat struct {
	XMLName xml.Name `xml:"AttachmentFormat"`

	Compression *CompressionType `xml:"Compression,omitempty"`
}

const testMultipartWithCSVContentType = `multipart/related;start="<rootpart*d7287a84-8be6-4284-afeb-26ee43e46edd@example.jaxws.sun.com>";type="application/xop+xml";boundary="uuid:d7287a84-8be6-4284-afeb-26ee43e46edd";start-info="text/xml"`
const testMultipartWithCSV = `--uuid:d7287a84-8be6-4284-afeb-26ee43e46edd
Content-Id: <rootpart*d7287a84-8be6-4284-afeb-26ee43e46edd@example.jaxws.sun.com>
Content-Type: application/xop+xml;charset=utf-8;type="text/xml"
Content-Transfer-Encoding: binary

<?xml version="1.0" ?><S:Envelope xmlns:S="http://schemas.xmlsoap.org/soap/envelope/"><S:Body><ns2:RunTimeSeriesReportResponse xmlns:ns2="http://example.com"><Result>Success</Result><RequestProcessingTime>0</RequestProcessingTime><Report><DataSets><DataSet><Columns><Column><Name>Subscriber.Name</Name><DataType>String</DataType><DynamicIndex>0</DynamicIndex></Column><Column><Name>PeriodStart</Name><DataType>Timestamp</DataType><DynamicIndex>1</DynamicIndex></Column><Column><Name>TotalBytes</Name><DataType>Real64</DataType><DynamicIndex>2</DynamicIndex></Column></Columns><CsvAttachment><CsvAttachmentFormat><Compression>None</Compression><Encoding>UTF-8</Encoding><Headers>false</Headers></CsvAttachmentFormat><CsvData><Include xmlns="http://www.w3.org/2004/08/xop/include" href="cid:c9947101-675e-47c9-911b-0aba186b7201@example.jaxws.sun.com"/></CsvData></CsvAttachment><Type>Historical.TimeSeries.Subscriber.ApplicationProtocol.Basic</Type></DataSet></DataSets><ReportDuration>2230</ReportDuration><NumberOfDataSets>1</NumberOfDataSets></Report></ns2:RunTimeSeriesReportResponse></S:Body></S:Envelope>
--uuid:d7287a84-8be6-4284-afeb-26ee43e46edd
Content-Id: <c9947101-675e-47c9-911b-0aba186b7201@example.jaxws.sun.com>
Content-Type: text/csv
Content-Transfer-Encoding: binary

tn_prod-e03d921e-ed56-4d51-826d-c54f0288bfef,2019-08-19T10:20:59.000Z,332682498

--uuid:d7287a84-8be6-4284-afeb-26ee43e46edd--`

func TestMultipartResponseWithCSV(t *testing.T) {
	testResp := &RunTimeSeriesReportResponse{}
	envelope := NewEnvelope(testResp)

	mediaType, mediaParams, err := mime.ParseMediaType(testMultipartWithCSVContentType)
	assert.Nil(t, err)
	assert.True(t, strings.HasPrefix(mediaType, "multipart/"))

	err = newXopDecoder(strings.NewReader(testMultipartWithCSV), mediaParams).decode(envelope)
	assert.Nil(t, err)
	assert.Equal(t, int32(1), testResp.Report.NumberOfDataSets)
	assert.Equal(t, "Subscriber.Name", testResp.Report.DataSets.DataSet[0].Columns.Column[0].Name)
	assert.Equal(t, "UTF-8", testResp.Report.DataSets.DataSet[0].CsvAttachment.CsvAttachmentFormat.Encoding)
	assert.Equal(t, "tn_prod-e03d921e-ed56-4d51-826d-c54f0288bfef,2019-08-19T10:20:59.000Z,332682498\n", string(testResp.Report.DataSets.DataSet[0].CsvAttachment.CsvData))
	assert.Equal(t, int32(1), testResp.Report.NumberOfDataSets)
}

func TestGetNameFromTag(t *testing.T) {
	var TestGetNameFromTag = []struct {
		testName string
		tag      string
		xmlName  string
	}{
		{
			testName: "omitted field tag",
			tag: "-",
			xmlName: "-",
		},
		{
			testName: "tag with everything",
			tag:      "namespace name,flag1,flag2",
			xmlName:  "name",
		},
		{
			testName: "missing namespace",
			tag:      "name,flag1,flag2",
			xmlName:  "name",
		},
		{
			testName: "missing flags",
			tag:      "namespace name",
			xmlName:  "name",
		},
		{
			testName: "flags only",
			tag:      ",flag1,flag2",
			xmlName:  "",
		},
		{
			testName: "name only",
			tag:      "name",
			xmlName:  "name",
		},
		{
			testName: "nothing",
			tag:      "",
			xmlName:  "",
		},
	}

	for _, tt := range TestGetNameFromTag {
		xmlName := getNameFromTag(tt.tag)
		assert.Equal(t, tt.xmlName, xmlName)
	}
}

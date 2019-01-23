package soap

import (
	"encoding/xml"
	"mime"
	"net/http"
	"strings"
)

// Response contains the result of the request.
type Response struct {
	*http.Response

	body        interface{}
	fault       *Fault
	faultDetail interface{}
}

func newResponse(httpResp *http.Response, req *Request) *Response {
	return &Response{
		Response:    httpResp,
		body:        req.resp,
		faultDetail: req.fault,
	}
}

// Body returns the SOAP body. The value comes from what was passed into the linked request.
func (r *Response) Body() interface{} {
	return r.body
}

// Fault returns the SOAP fault encountered, if present
func (r *Response) Fault() *Fault {
	return r.fault
}

func (r *Response) deserialize() error {
	mediaType, mediaParams, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return err
	}

	envelope := NewEnvelopeWithFault(r.body, r.faultDetail)

	if strings.HasPrefix(mediaType, "multipart/") {
		// Here we handle any SOAP requests embedded in a MIME multipart response.
		err = newXopDecoder(r.Response.Body, mediaParams).decode(envelope)
	} else if strings.Contains(mediaType, "text/xml") {
		// This is normal SOAP XML response handling.
		err = xml.NewDecoder(r.Response.Body).Decode(&envelope)
	} else {
		err = ErrUnsupportedContentType
	}

	if err != nil {
		return err
	}

	// Propagate the changes from parsing the envelope to the response struct
	if envelope.Body.Fault != nil {
		r.fault = envelope.Body.Fault
	}

	return nil
}

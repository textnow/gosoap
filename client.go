package soap

import (
	"context"
	"errors"
	"net/http"
)

var (
	// ErrInvalidPEMFileSpecified is returned if the PEM file specified for WS signing is invalid
	ErrInvalidPEMFileSpecified = errors.New("invalid PEM key specified")
	// ErrEncryptedPEMFileSpecified is returnedd if the PEM file specified for WS signing is encrypted
	ErrEncryptedPEMFileSpecified = errors.New("encrypted PEM key specified")
	// ErrUnsupportedContentType is returned if we encounter a non-supported content type while querying
	ErrUnsupportedContentType = errors.New("unsupported content-type in response")
)

// Client is an opaque handle to a SOAP service.
type Client struct {
	http *http.Client
}

// NewClient creates a new Client that will access a SOAP service.
// Requests made using this client will all be wrapped in a SOAP envelope.
// See https://www.w3schools.com/xml/xml_soap.asp for more details.
// The default HTTP client used has no timeout nor circuit breaking. Override with SettHTTPClient. You have been warned.
func NewClient(http *http.Client) *Client {
	return &Client{
		http: http,
	}
}

// Do invokes the SOAP request using its internal parameters.
// The request argument is serialized to XML, and if the call is successful the received XML
// is deserialized into the response argument.
// Any errors that are encountered are returned.
// If a SOAP fault is detected, then the 'details' property of the SOAP envelope will be deserialized into the faultDetailType argument.
func (c *Client) Do(ctx context.Context, req *Request) (*Response, error) {
	httpReq, err := req.httpRequest()
	if err != nil {
		return nil, err
	}

	httpResp, err := c.http.Do(httpReq.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	resp := newResponse(httpResp, req)
	err = resp.deserialize()
	if err != nil {
		return nil, err
	}

	return resp, nil
}

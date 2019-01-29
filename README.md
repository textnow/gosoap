# gosoap

This library provides primitives for operating on a SOAP-based web service. The library supports encrypting the SOAP request using the WS-Security x.509 protocol, enabling SOAP calls against secured web services.

A basic example usage would be as follows:

```
include (
    "context"
    "github.com/Enflick/gosoap"
)

main() {
	var (
		certFile = flag.String("cert", "", "A PEM encoded cert file")
		keyFile  = flag.String("key", "", "A PEM encoded key file")
	)

	flag.Parse()

	wsseInfo, authErr := soap.NewWSSEAuthInfo(*certFile, *keyFile)
	if authErr != nil {
		fmt.Printf("Auth error: %s\n", authErr.Error())
		return
	}
	
	// Setup your request structure
	// ...
	//

    // Create the SOAP request
    // call.action is the SOAP action (i.e. method name)
    // service.url is the fully qualified path to the SOAP endpoint
    // call.requestData is the structure mapping to the SOAP request
    // call.ResponseData is an output structure mapping to the SOAP response
    // call.FaultData is an output structure mapping to the SOAP fault details
    soapReq := soap.NewRequest(call.action, service.url, call.requestData, call.ResponseData, call.FaultData)
    
    // Potentially add custom headers
    soapReq.AddHeader(...)
    soapReq.AddHeader(...)
    
    // Sign the request
    soapReq.SignWith(wsseInfo)
    
    // Create the SOAP client
    soapClient := soap.NewClient(&http.Client{})
    
    // Make the request
    soapResp, err := soapClient.Do(context.Background(), soapReq)
	if err != nil {
		fmt.Printf("Unable to validate: %s\n", err.Error())
		return
	} else if soapResp.StatusCode != http.StatusOK {
		fmt.Printf("Unable to validate (status code invalid): %d\n", soapResp.StatusCode)
		return
	} else if soapResp.Fault() != nil {
		fmt.Printf("SOAP fault experienced during call: %s\n", soapResp.Fault().Error())
		// We can access the FaultData struct passed in for a type-safe way to get at the details.
		return
	}
	
	// Now we can handle the response itself.
	// Do our custom processing
	// ...
	//
	
	fmt.Printf("Done!\n")
}
```

The code is very loosely based off the SOAP client that is part of the https://github.com/hooklift/gowsdl project.

See https://github.com/rmrobinson-textnow/gowsdl for a heavily forked version of the above gowsdl project that auto-generates code from WSDL files that uses this library for performing the SOAP requests.

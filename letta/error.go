package letta

import "fmt"

var (
	ErrJsonMarshal   = fmt.Errorf("Failed to marshal JSON input")
	ErrJsonUnmarshal = fmt.Errorf("Failed to unmarshal JSON response")
	ErrRequest       = fmt.Errorf("Letta request failed")
	ErrResponse      = fmt.Errorf("Bad Letta API response")
	ErrBadStatusCode = fmt.Errorf("Bad status code in Letta response")
)

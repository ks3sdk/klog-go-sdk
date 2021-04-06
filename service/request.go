package service

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

/**
real request
*/

type Request struct {
	*Service
	Handlers     Handlers
	Time         time.Time
	HTTPRequest  *http.Request
	HTTPResponse *http.Response
	Body         io.ReadSeeker
	bodyStart    int64 // offset from beginning of Body that the request body starts
	Error        error
	RequestID    string
	RetryCount   uint
	Retryable    SettableBool
	RetryDelay   time.Duration

	data  []byte
	built bool
}

// NewRequest returns a new Request pointer for the service API
// operation and parameters.
func NewRequest(service *Service, op *Operation, inputs []byte) *Request {
	httpReq, _ := http.NewRequest(op.Method, "", nil)

	u, _ := url.Parse(service.Endpoint)
	u.Path = op.Path
	u.RawQuery = op.Params.Encode()

	httpReq.URL = u

	r := &Request{
		Service:     service,
		Handlers:    service.Handlers.copy(),
		Time:        time.Now(),
		HTTPRequest: httpReq,
		Body:        nil,
		Error:       nil,
		data:        inputs,
	}
	r.SetBufferBody(inputs)

	return r
}

// SetBufferBody will set the request's body bytes that will be sent to
// the service API.
func (r *Request) SetBufferBody(buf []byte) {
	r.SetReaderBody(bytes.NewReader(buf))
}

// SetReaderBody will set the request's body reader.
func (r *Request) SetReaderBody(reader io.ReadSeeker) {
	r.HTTPRequest.Body = ioutil.NopCloser(reader)
	r.Body = reader
}

// SetStringBody sets the body of the request to be backed by a string.
func (r *Request) SetStringBody(s string) {
	r.SetReaderBody(strings.NewReader(s))
}

// WillRetry returns if the request's can be retried.
func (r *Request) WillRetry() bool {
	return r.Error != nil && r.Retryable.Get() && r.RetryCount < r.Service.MaxRetries()
}

// An Operation is the service API operation to be made.
type Operation struct {
	Name   string
	Method string
	Path   string
	Params *url.Values
	*Paginator
}

// Paginator keeps track of pagination configuration for an API operation.
type Paginator struct {
	InputTokens     []string
	OutputTokens    []string
	LimitToken      string
	TruncationToken string
}

// Send will send the request returning error if errors are encountered.
//
// Send will sign the request prior to sending. All Send Handlers will
// be executed in the order they were set.
func (r *Request) Send() error {
	for {
		r.Sign()

		if r.Error != nil {
			return r.Error
		}
		if r.Retryable.Get() {
			// Re-seek the body back to the original point in for a retry so that
			// send will send the body's contents again in the upcoming request.
			r.Body.Seek(r.bodyStart, 0)
		}
		r.Retryable.Reset()

		r.Handlers.Send.Run(r)
		if r.Error != nil {
			r.Handlers.Retry.Run(r)
			r.Handlers.AfterRetry.Run(r)
			if r.Error != nil {
				return r.Error
			}
			continue
		}

		r.Handlers.UnmarshalMeta.Run(r)
		r.Handlers.ValidateResponse.Run(r)
		if r.Error != nil {
			r.Handlers.UnmarshalError.Run(r)
			r.Handlers.Retry.Run(r)
			r.Handlers.AfterRetry.Run(r)
			if r.Error != nil {
				return r.Error
			}
			continue
		}

		r.Handlers.Unmarshal.Run(r)
		if r.Error != nil {
			r.Handlers.Retry.Run(r)
			r.Handlers.AfterRetry.Run(r)
			if r.Error != nil {
				return r.Error
			}
			continue
		}

		break
	}

	return nil
}

// Sign will sign the request retuning error if errors are encountered.
//
// Send will build the request prior to signing. All Sign Handlers will
// be executed in the order they were set.
func (r *Request) Sign() error {
	r.Build()
	if r.Error != nil {
		return r.Error
	}

	r.Handlers.Sign.Run(r)
	return r.Error
}

// Build will build the request's object so it can be signed and sent
// to the service. Build will also validate all the request's parameters.
// Anny additional build Handlers set on this request will be run
// in the order they were set.
//
// The request will only be built once. Multiple calls to build will have
// no effect.
//
// If any Validate or Build errors occur the build will stop and the error
// which occurred will be returned.
func (r *Request) Build() error {
	if !r.built {
		r.Error = nil
		r.Handlers.Validate.Run(r)
		if r.Error != nil {
			return r.Error
		}
		r.Handlers.Build.Run(r)
		r.built = true
	}

	return r.Error
}

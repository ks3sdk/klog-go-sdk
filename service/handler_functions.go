package service

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/ks3sdklib/aws-sdk-go/aws/awserr"
	"github.com/pierrec/lz4"
	"io"
	"io/ioutil"
	"ksyun.com/cbd/klog-sdk/internal/apierr"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

var (
	// ErrMissingEndpoint is an error that is returned if an endpoint cannot be
	// resolved for a service.
	ErrMissingEndpoint error = apierr.New("MissingEndpoint", "'Endpoint' configuration is required for this service", nil)
)

func ValidateEndpointHandler(r *Request) {
	if r.Service.Endpoint == "" {
		r.Error = ErrMissingEndpoint
	}
}

// UserAgentHandler is a request handler for injecting User agent into requests.
func UserAgentHandler(r *Request) {
	r.HTTPRequest.Header.Set("User-Agent", SDKName+"/"+SDKVersion)
}

func RequestIdHandler(r *Request) {
	reqId := RandomString()
	r.RequestID = reqId
	r.HTTPRequest.Header.Set("X-KSC-REQUEST-ID", reqId)
}

func CommonHeaderHandler(r *Request) {
	r.HTTPRequest.Header.Set("Content-Type", "application/x-protobuf")
	r.HTTPRequest.Header.Set("klog-Host", r.Endpoint)
	r.HTTPRequest.Header.Set("x-klog-api-version", "0.1.0")
	r.HTTPRequest.Header.Set("x-klog-signature-method", "hmac-sha1")
}

func CompressLz4(r *Request) {
	dst := new(bytes.Buffer)
	z := lz4.NewWriter(dst)
	_, err := z.Write(r.data)
	if err != nil {
		r.Error = apierr.New("CompressLz4", "failed to write", err)
		return
	}
	err = z.Close()
	if err != nil {
		r.Error = apierr.New("CompressLz4", "failed to close", err)
		return
	}

	compressed := dst.Bytes()

	r.SetBufferBody(compressed)
	r.HTTPRequest.Header.Set("x-klog-compress-type", "lz4")
}

func ContentMD5(r *Request) {
	h := md5.New()

	// hash the body.  seek back to the first position after reading to reset
	// the body for transmission.  copy errors may be assumed to be from the
	// body.
	_, err := io.Copy(h, r.Body)
	if err != nil {
		r.Error = apierr.New("ContentMD5", "failed to read body", err)
		return
	}
	_, err = r.Body.Seek(0, 0)
	if err != nil {
		r.Error = apierr.New("ContentMD5", "failed to seek body", err)
		return
	}

	// encode the md5 checksum in base64 and set the request header.
	sum := h.Sum(nil)
	sum64 := make([]byte, base64.StdEncoding.EncodedLen(len(sum)))
	base64.StdEncoding.Encode(sum64, sum)
	r.HTTPRequest.Header.Set("Content-MD5", string(sum64))
}

var sleepDelay = func(delay time.Duration) {
	time.Sleep(delay)
}

// Interface for matching types which also have a Len method.
type lener interface {
	Len() int
}

// BuildContentLength builds the content length of a request based on the body,
// or will use the HTTPRequest.Header's "Content-Length" if defined. If unable
// to determine request body length and no "Content-Length" was specified it will panic.
func BuildContentLength(r *Request) {
	if slength := r.HTTPRequest.Header.Get("Content-Length"); slength != "" {
		length, _ := strconv.ParseInt(slength, 10, 64)
		r.HTTPRequest.ContentLength = length
		return
	}

	var length int64
	switch body := r.Body.(type) {
	case nil:
		length = 0
	case lener:
		length = int64(body.Len())
	case io.Seeker:
		r.bodyStart, _ = body.Seek(0, 1)
		end, _ := body.Seek(0, 2)
		body.Seek(r.bodyStart, 0) // make sure to seek back to original location
		length = end - r.bodyStart
	default:
		panic("Cannot get length of body, must provide `ContentLength`")
	}

	r.HTTPRequest.ContentLength = length
	r.HTTPRequest.Header.Set("Content-Length", fmt.Sprintf("%d", length))
}

var reStatusCode = regexp.MustCompile(`^(\d+)`)

// SendHandler is a request handler to send service request using HTTP client.
func SendHandler(r *Request) {

	var err error
	if r.HTTPRequest.ContentLength <= 0 {
		r.HTTPRequest.Body = http.NoBody
	}
	r.HTTPResponse, err = r.Service.Config.HTTPClient.Do(r.HTTPRequest)
	if err != nil {
		// Capture the case where url.Error is returned for error processing
		// response. e.g. 301 without location header comes back as string
		// error and r.HTTPResponse is nil. Other url redirect errors will
		// comeback in a similar method.
		if e, ok := err.(*url.Error); ok {
			if s := reStatusCode.FindStringSubmatch(e.Error()); s != nil {
				code, _ := strconv.ParseInt(s[1], 10, 64)
				r.HTTPResponse = &http.Response{
					StatusCode: int(code),
					Status:     http.StatusText(int(code)),
					Body:       ioutil.NopCloser(bytes.NewReader([]byte{})),
				}
				return
			}
		}
		// Catch all other request errors.
		r.Error = apierr.New("RequestError", "send request failed", err)
		r.Retryable.Set(true) // network errors are retryable
	}
}

// AfterRetryHandler performs final checks to determine if the request should
// be retried and how long to delay.
func AfterRetryHandler(r *Request) {
	// If one of the other handlers already set the retry state
	// we don't want to override it based on the service's state
	if !r.Retryable.IsSet() {
		r.Retryable.Set(r.Service.ShouldRetry(r))
	}

	if r.WillRetry() {
		r.RetryDelay = r.Service.RetryRules(r)
		sleepDelay(r.RetryDelay)

		// when the expired token exception occurs the credentials
		// need to be expired locally so that the next request to
		// get credentials will trigger a credentials refresh.
		if r.Error != nil {
			if err, ok := r.Error.(awserr.Error); ok {
				if isCodeExpiredCreds(err.Code()) {
					r.Config.Credentials.Expire()
				}
			}
		}

		r.RetryCount++
		r.Error = nil
	}
}

type responseMessage struct {
	ErrorCode    string `json:"ErrorCode"`
	ErrorMessage string `json:"ErrorMessage"`
}

// ValidateResponseHandler is a request handler to validate service response.
func ValidateResponseHandler(r *Request) {
	if r.HTTPResponse.StatusCode == 0 || r.HTTPResponse.StatusCode >= 300 {
		if r.HTTPResponse.Body != nil {
			defer r.HTTPResponse.Body.Close()
			if responseBody, err := ioutil.ReadAll(r.HTTPResponse.Body); err == nil {
				result := new(responseMessage)
				err = json.Unmarshal(responseBody, result)
				if err == nil {
					r.Error = apierr.New(result.ErrorCode, result.ErrorMessage, nil)
					return
				}
			}
		}

		message := fmt.Sprintf("unknown error, code=%v", r.HTTPResponse.StatusCode)
		r.Error = apierr.New("UnknownError", message, nil)
	}
}

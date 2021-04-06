package service

import (
	"github.com/ks3sdklib/aws-sdk-go/aws/awserr"
	"math"
	"net/http"
	"net/http/httputil"
	"regexp"
	"time"
)

type Service struct {
	Config            *Config
	Handlers          Handlers
	APIVersion        string
	Endpoint          string
	JSONVersion       string
	RetryRules        func(*Request) time.Duration
	ShouldRetry       func(*Request) bool
	DefaultMaxRetries uint
}

var schemeRE = regexp.MustCompile("^([^:]+)://")

func NewService(config *Config) *Service {
	service := &Service{
		Config: config,
	}

	service.Initialize()
	return service
}

func (service *Service) Initialize() {
	if service.Config == nil {
		service.Config = &Config{}
	}

	if service.Config.HTTPClient == nil {
		service.Config.HTTPClient = http.DefaultClient
	}

	if service.RetryRules == nil {
		service.RetryRules = retryRules
	}

	if service.ShouldRetry == nil {
		service.ShouldRetry = shouldRetry
	}

	service.DefaultMaxRetries = 3
	service.Handlers.Validate.PushBack(ValidateEndpointHandler)
	service.Handlers.Build.PushBack(UserAgentHandler)
	service.Handlers.Build.PushBack(RequestIdHandler)
	service.Handlers.Build.PushBack(CommonHeaderHandler)

	if service.Config.CompressMethod == CompressMethodLz4 {
		service.Handlers.Build.PushBack(CompressLz4)
	}

	if !service.Config.DisableComputeChecksums {
		service.Handlers.Build.PushBack(ContentMD5)
	}

	service.Handlers.Sign.PushBack(BuildContentLength)
	service.Handlers.Send.PushBack(SendHandler)
	service.Handlers.AfterRetry.PushBack(AfterRetryHandler)
	service.Handlers.ValidateResponse.PushBack(ValidateResponseHandler)
	if service.Config.Debug {
		service.AddDebugHandlers()
	}
	service.buildEndpoint()
}

// buildEndpoint builds the endpoint values the service will use to make requests with.
func (service *Service) buildEndpoint() {
	service.Endpoint = service.Config.Endpoint
	if service.Endpoint != "" && !schemeRE.MatchString(service.Endpoint) {
		scheme := "https"
		if service.Config.DisableSSL {
			scheme = "http"
		}
		service.Endpoint = scheme + "://" + service.Endpoint
	}
}

// AddDebugHandlers injects debug logging handlers into the service to log request
// debug information.
func (service *Service) AddDebugHandlers() {
	service.Handlers.Send.PushFront(func(r *Request) {
		dumpedBody, _ := httputil.DumpRequestOut(r.HTTPRequest, false)

		r.Config.Logger.Infof("---[ REQUEST POST-SIGN ]-----------------------------\nREQUEST BODY SIZE %d\n%s\n-----------------------------------------------------\n",
			len(string(dumpedBody)), string(dumpedBody))
	})
	service.Handlers.Send.PushBack(func(r *Request) {
		if r.HTTPResponse != nil {
			dumpedBody, _ := httputil.DumpResponse(r.HTTPResponse, false)

			r.Config.Logger.Infof("---[ RESPONSE ]--------------------------------------\n%s\n-----------------------------------------------------\n",
				string(dumpedBody))

		} else if r.Error != nil {
			r.Config.Logger.Infof("---[ RESPONSE ]--------------------------------------\n%s\n-----------------------------------------------------\n",
				r.Error.Error())
		}
	})
}

// MaxRetries returns the number of maximum returns the service will use to make
// an individual API request.
func (service *Service) MaxRetries() uint {
	if service.Config.MaxRetries < 0 {
		return service.DefaultMaxRetries
	}
	return uint(service.Config.MaxRetries)
}

// retryRules returns the delay duration before retrying this request again
func retryRules(r *Request) time.Duration {
	delay := time.Duration(math.Pow(2, float64(r.RetryCount))) * 30
	return delay * time.Millisecond
}

// shouldRetry returns if the request should be retried.
func shouldRetry(r *Request) bool {
	if r.HTTPResponse.StatusCode >= 500 {
		return true
	}
	if r.Error != nil {
		if err, ok := r.Error.(awserr.Error); ok {
			return isCodeRetryable(err.Code())
		}
	}
	return false
}

// retryableCodes is a collection of service response codes which are retry-able
// without any further action.
var retryableCodes = map[string]struct{}{
	"RequestError":                           {},
	"ProvisionedThroughputExceededException": {},
	"Throttling":                             {},
}

func isCodeExpiredCreds(code string) bool {
	_, ok := credsExpiredCodes[code]
	return ok
}

// credsExpiredCodes is a collection of error codes which signify the credentials
// need to be refreshed. Expired tokens require refreshing of credentials, and
// resigning before the request can be retried.
var credsExpiredCodes = map[string]struct{}{
	"ExpiredToken":          {},
	"ExpiredTokenException": {},
	"RequestExpired":        {}, // EC2 Only
}

func isCodeRetryable(code string) bool {
	if _, ok := retryableCodes[code]; ok {
		return true
	}

	return isCodeExpiredCreds(code)
}

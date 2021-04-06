package service

import (
	"ksyun.com/cbd/klog-sdk/credentials"
	"net/http"
)

// DefaultChainCredentials is a Credentials which will find the first available
// credentials Value from the list of Providers.
//
// This should be used in the default case. Once the type of credentials are
// known switching to the specific Credentials will be more efficient.
var DefaultChainCredentials = credentials.NewChainCredentials(
	[]credentials.Provider{
		&credentials.EnvProvider{},
		&credentials.SharedCredentialsProvider{Filename: "", Profile: ""},
	})

const (
	// The default number of retries for a service. The value of -1 indicates that
	// the service specific retry default will be used.
	DefaultRetries = -1

	// supported compress methods
	CompressMethodNone = ""
	CompressMethodLz4  = "lz4"
)

var DefaultConfig = &Config{
	Credentials:             DefaultChainCredentials,
	Endpoint:                "",
	DisableSSL:              true,
	HTTPClient:              http.DefaultClient,
	Logger:                  new(EmptyLogger),
	Debug:                   false,
	MaxRetries:              DefaultRetries,
	DisableComputeChecksums: false,
	CompressMethod:          CompressMethodLz4,
}

type Config struct {
	Credentials             *credentials.Credentials
	Endpoint                string
	DisableSSL              bool
	HTTPClient              *http.Client
	Logger                  Logger
	Debug                   bool
	MaxRetries              int
	DisableComputeChecksums bool
	CompressMethod          string
}

// Merge merges the newcfg attribute values into this Config. Each attribute
// will be merged into this config if the newcfg attribute's value is non-zero.
// Due to this, newcfg attributes with zero values cannot be merged in. For
// example bool attributes cannot be cleared using Merge, and must be explicitly
// set on the Config structure.
func (c Config) Merge(newcfg *Config) *Config {
	if newcfg == nil {
		return &c
	}

	cfg := Config{}

	if newcfg.Credentials != nil {
		cfg.Credentials = newcfg.Credentials
	} else {
		cfg.Credentials = c.Credentials
	}

	if newcfg.Endpoint != "" {
		cfg.Endpoint = newcfg.Endpoint
	} else {
		cfg.Endpoint = c.Endpoint
	}

	if newcfg.DisableSSL {
		cfg.DisableSSL = newcfg.DisableSSL
	} else {
		cfg.DisableSSL = c.DisableSSL
	}

	if newcfg.HTTPClient != nil {
		cfg.HTTPClient = newcfg.HTTPClient
	} else {
		cfg.HTTPClient = c.HTTPClient
	}

	if newcfg.Logger != nil {
		cfg.Logger = newcfg.Logger
	} else {
		cfg.Logger = c.Logger
	}

	if newcfg.Debug != c.Debug {
		cfg.Debug = newcfg.Debug
	} else {
		cfg.Debug = c.Debug
	}

	if newcfg.MaxRetries != DefaultRetries {
		cfg.MaxRetries = newcfg.MaxRetries
	} else {
		cfg.MaxRetries = c.MaxRetries
	}

	if newcfg.DisableComputeChecksums {
		cfg.DisableComputeChecksums = newcfg.DisableComputeChecksums
	} else {
		cfg.DisableComputeChecksums = c.DisableComputeChecksums
	}

	if newcfg.CompressMethod != "" {
		cfg.CompressMethod = newcfg.CompressMethod
	} else {
		cfg.CompressMethod = c.CompressMethod
	}

	return &cfg
}

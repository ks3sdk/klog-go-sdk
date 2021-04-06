package v2

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"ksyun.com/cbd/klog-sdk/credentials"
	"ksyun.com/cbd/klog-sdk/service"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	timeFormat = "Mon, 02 Jan 2006 15:04:05 GMT"
)

type signer struct {
	Service     *service.Service
	Request     *http.Request
	Time        time.Time
	CredValues  credentials.Value
	Credentials *credentials.Credentials

	formattedTime string

	canonicalHeaders  string
	canonicalResource string
	stringToSign      string
	signature         string
	authorization     string
}

func Sign(req *service.Request) {
	if req.Service.Config.Credentials == credentials.AnonymousCredentials {
		return
	}

	s := signer{
		Service:     req.Service,
		Request:     req.HTTPRequest,
		Time:        req.Time,
		Credentials: req.Service.Config.Credentials,
	}

	req.Error = s.sign()
}

func (v2 *signer) sign() error {
	if v2.isRequestSigned() {
		return nil
	}

	var err error
	v2.CredValues, err = v2.Credentials.Get()
	if err != nil {
		return err
	}

	v2.build()

	if v2.Service.Config.Debug {
		v2.logSigningInfo()
	}

	return nil
}

func (v2 *signer) logSigningInfo() {
	v2.Service.Config.Logger.Infof("---[ STRING TO SIGN ]--------------------------------\n%s\n-----------------------------------------------------\n",
		v2.stringToSign)
}

func (v2 *signer) build() {

	v2.buildTime()              // no depends
	v2.buildCanonicalHeaders()  // depends on cred string
	v2.buildCanonicalResource() // depends on canon headers / signed headers
	v2.buildStringToSign()      // depends on canon string
	v2.buildSignature()         // depends on string to sign
	v2.Request.Header.Set("Authorization", "KLOG "+v2.CredValues.AccessKeyID+":"+v2.signature)
}

func (v2 *signer) buildTime() {
	v2.formattedTime = v2.Time.UTC().Format(timeFormat)
	v2.Request.Header.Set("Date", v2.formattedTime)
}

func (v2 *signer) buildCanonicalHeaders() {
	var headers []string
	for k := range v2.Request.Header {
		if strings.HasPrefix(strings.ToLower(http.CanonicalHeaderKey(k)), "x-klog-") {
			headers = append(headers, k)
		}
	}
	sort.Strings(headers)

	headerValues := make([]string, len(headers))
	for i, k := range headers {
		headerValues[i] = strings.ToLower(http.CanonicalHeaderKey(k)) + ":" +
			strings.Join(v2.Request.Header[http.CanonicalHeaderKey(k)], ",")
	}

	v2.canonicalHeaders = strings.Join(headerValues, "\n")
}

func (v2 *signer) buildCanonicalResource() {
	v2.canonicalResource = v2.Request.URL.RequestURI()
}

func (v2 *signer) buildStringToSign() {
	md5 := ""
	if len(v2.Request.Header["Content-Md5"]) > 0 {
		md5 = v2.Request.Header["Content-Md5"][0]
	}

	contentType := ""
	if len(v2.Request.Header["Content-Type"]) > 0 {
		contentType = v2.Request.Header["Content-Type"][0]
	}

	signItems := []string{v2.Request.Method, md5, contentType, v2.formattedTime}
	if v2.canonicalHeaders != "" {
		signItems = append(signItems, v2.canonicalHeaders)
	}
	signItems = append(signItems, v2.canonicalResource)

	v2.stringToSign = strings.Join(signItems, "\n")

}

func (v2 *signer) buildSignature() {
	secret := v2.CredValues.SecretAccessKey
	signature := string(base64Encode(makeHmac([]byte(secret), []byte(v2.stringToSign))))
	v2.signature = signature
}

// isRequestSigned returns if the request is currently signed or presigned
func (v2 *signer) isRequestSigned() bool {
	if v2.Request.Header.Get("Authorization") != "" {
		return true
	}
	return false
}

func makeHmac(key []byte, data []byte) []byte {
	hash := hmac.New(sha1.New, key)
	hash.Write(data)
	return hash.Sum(nil)
}
func base64Encode(src []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(src))
}

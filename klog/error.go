package klog

import "ksyun.com/cbd/klog-sdk/internal/apierr"

var (
	InternalServerError      = "InternalServerError"
	SignatureNotMatch        = "SignatureNotMatch"
	PostBodyTooLarge         = "PostBodyTooLarge"
	PostBodyInvalid          = "PostBodyInvalid" // deprecated
	ProjectOrLogPoolNotExist = "ProjectOrLogPoolNotExist"
	UserNotExist             = "UserNotExist"
	MaxBulkSizeExceeded      = "MaxBulkSizeExceeded"
	MaxKeyCountExceeded      = "MaxKeyCountExceeded"
	MaxKeySizeExceeded       = "MaxKeySizeExceeded"
	MaxValueSizeExceeded     = "MaxValueSizeExceeded"
	MaxLogSizeExceeded       = "MaxLogSizeExceeded"
	InvalidUtf8InKey         = "InvalidUtf8InKey"
	InvalidUtf8InValue       = "InvalidUtf8InValue"
)

func IsError(err error, code string) bool {
	if e, ok := err.(*apierr.BaseError); ok {
		return e.Code() == code
	}
	return false
}

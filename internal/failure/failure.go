package failure

import (
	"errors"
	"fmt"
)

type Code string

const (
	CodeInvalidInput     Code = "INVALID_INPUT"
	CodeDownloadFailed   Code = "DOWNLOAD_FAILED"
	CodeChecksumMismatch Code = "CHECKSUM_MISMATCH"
	CodeExtractFailed    Code = "EXTRACT_FAILED"
	CodeRuntimeMissing   Code = "RUNTIME_NOT_INSTALLED"
	CodeUnknownGame      Code = "UNKNOWN_GAME"
	CodeBadProfile       Code = "BAD_PROFILE"
	CodePrefixFailed     Code = "PREFIX_FAILED"
	CodeLaunchFailed     Code = "LAUNCH_FAILED"
	CodeSteamNotFound    Code = "STEAM_NOT_FOUND"
	CodeUnknownBuild     Code = "UNKNOWN_STEAM_BUILD"
	CodeSteamRunning     Code = "STEAM_RUNNING"
	CodeHookNotEnabled   Code = "STEAM_HOOK_NOT_ENABLED"
	CodeInvalidVDF       Code = "INVALID_VDF"
	CodeFileFailed       Code = "FILE_FAILED"
)

type Error struct {
	Code    Code
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func Errorf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func CodeOf(err error) Code {
	var known *Error
	if errors.As(err, &known) {
		return known.Code
	}
	return ""
}

package sandbox

import "fmt"

type Code string

const (
	CodeInvalidInput Code = "INVALID_INPUT"
	CodeMissingTool  Code = "MISSING_TOOL"
	CodeGitHubFailed Code = "GITHUB_REQUEST_FAILED"
	CodeNotFound     Code = "SANDBOX_NOT_FOUND"
	CodeNotReady     Code = "SANDBOX_NOT_READY"
	CodeStopped      Code = "SANDBOX_STOPPED"
	CodeKeyNotHere   Code = "SANDBOX_KEY_NOT_HERE"
	CodeLimitReached Code = "SANDBOX_LIMIT_REACHED"
	CodeStartFailed  Code = "SANDBOX_START_FAILED"
	CodeCopyFailed   Code = "COPY_FAILED"
	CodeExtendFailed Code = "EXTEND_FAILED"
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

package cli_IO

import "log/slog"

type CliErrorCode string

const (
	INVALID_TX            CliErrorCode = "INVALID_TX"
	INCONSISTENT_PREVOUTS CliErrorCode = "INCONSISTENT_PREVOUTS"
)

type CliErrorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CliError struct {
	Ok    bool            `json:"ok"`
	Error CliErrorDetails `json:"error"`
}

func CliErrorCodeToString(code CliErrorCode) string {
	switch code {
	case INVALID_TX:
		return "INVALID_TX"
	default:
		return "UNKNOWN_ERROR"
	}
}

func NewErrorWithLog(err error, ok bool, code CliErrorCode, message string) CliError {
	slog.Error("ErrorDetails happen", err)
	return CliError{
		Ok: ok,
		Error: CliErrorDetails{
			Code:    CliErrorCodeToString(code),
			Message: message,
		},
	}

}

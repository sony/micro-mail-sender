package mailsender

import "github.com/cockroachdb/errors"

// AppError wraps an application error with an HTTP response code.
type AppError struct {
	Code     int    // HTTP response code
	Message  string // custom message
	Internal error  // original error, if any
}

// AppErr returns a new AppError including the given HTTP response code.
func AppErr(code int, message string) *AppError {
	return &AppError{Code: code, Message: message, Internal: nil}
}

// WrapErr returns a new AppError wrapping the given error.
func WrapErr(code int, err error) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{Code: code, Message: err.Error(), Internal: err}
}

// Error returns the error message.
func (e *AppError) Error() string {
	return e.Message
}

// appendError combines two errors into a single error using errors.Join.
func appendError(err1, err2 error) error {
	if err1 == nil && err2 == nil {
		return nil
	}

	if err1 == nil {
		return err2
	}

	if err2 == nil {
		return err1
	}

	return errors.Join(err1, err2)
}

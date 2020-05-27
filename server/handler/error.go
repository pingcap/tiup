package handler

import "net/http"

type statusError interface {
	StatusCode() int
	Status() string
	Error() string
}

type handlerError struct {
	code    int
	status  string
	message string
}

func newHandlerError(code int, status, message string) statusError {
	return &handlerError{
		code:    code,
		status:  status,
		message: message,
	}
}

func (e *handlerError) StatusCode() int {
	return e.code
}

func (e *handlerError) Status() string {
	return e.status
}

func (e *handlerError) Error() string {
	return e.message
}

var (
	// ErrorTarbalConflict indicates that the same tarbal is being concurrent uploading
	ErrorTarbalConflict = newHandlerError(http.StatusConflict, "TARBAL CONFLICT", "another tarbal with the same hash is being uploading")
	// ErrorSessionMissing indicates that the specified session not found
	ErrorSessionMissing = newHandlerError(http.StatusNotFound, "SESSION NOT FOUND", "session with specified identity not found")
	// ErrorManifestMissing indicates that the specified component doesn't have manifest yet
	ErrorManifestMissing = newHandlerError(http.StatusNotFound, "MANIFEST NOT FOUND", "that component doesn't have manifest yet")
	// ErrorInvalidTarbal indicates that the tarbal is not valid (eg. too large)
	ErrorInvalidTarbal = newHandlerError(http.StatusBadRequest, "INVALID TARBAL", "the tarbal content is not valid")
	// ErrorInternalError indicates that an internal error happened
	ErrorInternalError = newHandlerError(http.StatusInternalServerError, "INTERNAL ERROR", "an internal error happened")
	// ErrorManifestConflict indicates that the uploaded manifest is not new enough
	ErrorManifestConflict = newHandlerError(http.StatusConflict, "MANIFEST CONFLICT", "the manifest provided is not new enough")
)

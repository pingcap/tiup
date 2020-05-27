package handler

import (
	"context"

	"github.com/pingcap/fn"
)

type errorMessage struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func init() {
	fn.SetErrorEncoder(func(ctx context.Context, err error) interface{} {
		if e, ok := err.(statusError); ok {
			return &errorMessage{
				Status:  e.Status(),
				Message: e.Error(),
			}
		}
		return &errorMessage{
			Status:  "UNKNOWN_ERROR",
			Message: "make sure your request is valid",
		}
	})
}

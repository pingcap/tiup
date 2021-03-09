package dmpb

import (
	"go.uber.org/zap/zapcore"
)

var (
	// HidePwdFunc should be overwrite by utils.HidePassword, this variable is for avoiding cycle import
	HidePwdFunc = func(s string) string {
		return s
	}
)

// MarshalLogObject implements zapcore.ObjectMarshaler
func (m *StartTaskRequest) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("HidePasswordObject", HidePwdFunc(m.String()))
	return nil
}

// MarshalLogObject implements zapcore.ObjectMarshaler
func (m *OperateSourceRequest) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("HidePasswordObject", HidePwdFunc(m.String()))
	return nil
}

package cerrors

import (
	"errors"
	"fmt"
)

// AppendMessage は err が *CustomError の場合、AppendMessage の呼び出し元をキャプチャしてコンテキストを追加する
func AppendMessage(err error, msg string) error {
	var ce *CustomError
	if errors.As(err, &ce) {
		ce.addMessage(msg, 3)
		return ce
	}
	return err
}

// AppendMessagef は err が *CustomError の場合、AppendMessagef の呼び出し元をキャプチャしてコンテキストを追加する
func AppendMessagef(err error, format string, args ...any) error {
	var ce *CustomError
	if errors.As(err, &ce) {
		ce.addMessage(fmt.Sprintf(format, args...), 3)
		return ce
	}
	return err
}

// AppendCheckpoint
func AppendCheckpoint(err error, options ...CheckpointOption) error {
	var ce *CustomError
	if errors.As(err, &ce) {
		ce.addCheckpoint(3, options...)
		return ce
	}
	return err
}

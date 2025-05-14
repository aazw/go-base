package cerrors

import (
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"

	sentry "github.com/getsentry/sentry-go"
)

// https://github.com/cockroachdb/errors/blob/master/withstack_api.go#L42
//     // ReportableStackTrace aliases the type of the same name in the sentry
//     // package. This is used by SendReport().
//     type ReportableStackTrace = withstack.ReportableStackTrace
//
// https://github.com/cockroachdb/errors/blob/master/withstack/reportable.go#L32
//     // ReportableStackTrace aliases the type of the same name in the sentry
//     // package. This is used by the 'report' error package.
//     type ReportableStackTrace = sentry.Stacktrace
//
// https://github.com/getsentry/sentry-go/blob/master/stacktrace.go#L21-L24
//     type Stacktrace struct {
//         Frames        []Frame `json:"frames,omitempty"`
//         FramesOmitted []uint  `json:"frames_omitted,omitempty"`
//     }
//
// https://github.com/getsentry/sentry-go/blob/master/stacktrace.go#L162-L191
//     type Frame struct {
//         Function string `json:"function,omitempty"`
//         Symbol   string `json:"symbol,omitempty"`
//         // Module is, despite the name, the Sentry protocol equivalent of a Go
//         // package's import path.
//         Module      string                 `json:"module,omitempty"`
//         Filename    string                 `json:"filename,omitempty"`
//         AbsPath     string                 `json:"abs_path,omitempty"`
//         Lineno      int                    `json:"lineno,omitempty"`
//         Colno       int                    `json:"colno,omitempty"`
//         PreContext  []string               `json:"pre_context,omitempty"`
//         ContextLine string                 `json:"context_line,omitempty"`
//         PostContext []string               `json:"post_context,omitempty"`
//         InApp       bool                   `json:"in_app"`
//         Vars        map[string]interface{} `json:"vars,omitempty"`
//         // Package and the below are not used for Go stack trace frames.  In
//         // other platforms it refers to a container where the Module can be
//         // found.  For example, a Java JAR, a .NET Assembly, or a native
//         // dynamic library.  They exists for completeness, allowing the
//         // construction and reporting of custom event payloads.
//         Package         string `json:"package,omitempty"`
//         InstructionAddr string `json:"instruction_addr,omitempty"`
//         AddrMode        string `json:"addr_mode,omitempty"`
//         SymbolAddr      string `json:"symbol_addr,omitempty"`
//         ImageAddr       string `json:"image_addr,omitempty"`
//         Platform        string `json:"platform,omitempty"`
//         StackStart      bool   `json:"stack_start,omitempty"`
//     }

type stackTrace []sentry.Frame

// Checkpoint
type checkpoint struct {
	sentry.Frame
}

// message はコンテキストメッセージと、それが追加されたファイル:行番号を保持
type message struct {
	sentry.Frame

	Message string `json:"message,omitempty"` // コンテキストを表すメッセージ本体
}

// CustomError はカスタムエラー型で、エラーコード、詳細メッセージ、実行時コンテキスト情報、原因エラー、およびスタックトレースを保持
type CustomError struct {
	errCode     string       // エラーを一意に識別するコード
	detail      string       // エラー型を説明する静的な情報
	cause       error        // このエラーの原因となった元のエラー
	stack       stackTrace   // stacktraceを自分で保持 (errors.WithStackにすると、*withstack.withStack型になるので、slog.LogValuerのLogValue()が呼ばれない)
	messages    []message    // 実行時のコンテキストを説明するメッセージ等情報 (contextual message)
	checkpoints []checkpoint // 明示的に設けられたチェックポイントを実行時に通過したかを記録するためのもの
}

func (e *CustomError) newFrame(skip int) sentry.Frame {

	var module string
	var funcName string

	// 呼び出し元の情報をキャプチャ
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		file = "?"
		line = 0
	} else {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			fullName := fn.Name()
			module = fullName[:strings.LastIndex(fullName, ".")]
			funcName = fullName[strings.LastIndex(fullName, ".")+1:]
		}
	}

	return sentry.Frame{
		Function: funcName,
		Module:   module,
		Filename: file, // 絶対パス
		AbsPath:  file,
		Lineno:   line,
	}
}

// Functional Options Pattern
type Option func(*CustomError)

// WithMessage はフォーマットなしでコンテキストメッセージを追加する
// 呼び出し元のファイルと行番号をキャプチャする
func WithMessage(msg string) Option {
	return func(e *CustomError) {
		frame := e.newFrame(3)

		e.messages = append(e.messages, message{
			Message: msg,
			Frame:   frame,
		})
	}
}

// WithMessagef はフォーマット付きでコンテキストメッセージを追加する
// 呼び出し元のファイルと行番号をキャプチャする
func WithMessagef(format string, args ...any) Option {
	return func(e *CustomError) {
		frame := e.newFrame(3)

		e.messages = append(e.messages, message{
			Message: fmt.Sprintf(format, args...),
			Frame:   frame,
		})
	}
}

// WithCause はエラーの原因(err)を CustomError に設定する
func WithCause(err error) Option {
	return func(e *CustomError) {
		e.cause = err
	}
}

// addContext はskipに基づいて呼び出し元情報を取得する内部ヘルパー
func (e *CustomError) addMessage(msg string, skip int) {
	frame := e.newFrame(skip)

	e.messages = append(e.messages, message{
		Message: msg,
		Frame:   frame,
	})
}

// AddMessage は作成後に新しいコンテキストメッセージを追加し、直接の呼び出し元をキャプチャする
func (e *CustomError) AddMessage(msg string) {
	e.addMessage(msg, 2)
}

// AddMessagef は作成後に新しいコンテキストメッセージを追加し、直接の呼び出し元をキャプチャする
func (e *CustomError) AddMessagef(format string, args ...any) {
	e.addMessage(fmt.Sprintf(format, args...), 2)
}

type CheckpointOption func(*checkpoint)

func WithCheckpointMessage(msg string) CheckpointOption {
	return func(cp *checkpoint) {
	}
}

func WithCheckpointMessagef(format string, args ...any) CheckpointOption {
	return func(*checkpoint) {
	}
}

func (e *CustomError) addCheckpoint(skip int, options ...CheckpointOption) {
	frame := e.newFrame(skip)

	cp := checkpoint{
		Frame: frame,
	}

	// Functional Options Pattern でのオプションの処理
	for _, option := range options {
		option(&cp)
	}

	e.checkpoints = append(e.checkpoints, cp)
}

// Error は CustomError のコード、詳細メッセージ、追加されたコンテキスト情報、および原因エラーを組み合わせた文字列表現を返す
// 組み込みの error インターフェースを実装
func (e *CustomError) Error() string {

	base := fmt.Sprintf("[%s] %s", e.errCode, e.detail)

	for _, cxtMsg := range e.messages {
		base += fmt.Sprintf(": %s (%s:%d)", cxtMsg.Message, cxtMsg.Filename, cxtMsg.Lineno)
	}

	if e.cause != nil {
		base += ": " + e.cause.Error()
	}

	return base
}

// Unwrap は errors.Is/As を処理できるようにする
func (e *CustomError) Unwrap() error {
	return e.cause
}

// Code はエラーコードを返す
func (e *CustomError) Code() string {
	return e.errCode
}

// Detail は静的な詳細情報を返します
func (e *CustomError) Detail() string {
	return e.detail
}

// Messages はコンテキストメッセージのスライスを返す
func (e *CustomError) Messages() []message {
	return e.messages
}

// LogValue は構造化ロギングのための slog.LogValuer を実装したもの
func (e *CustomError) LogValue() slog.Value {

	attrs := []slog.Attr{
		slog.String("code", e.errCode),
		slog.String("detail", e.detail),
	}

	if e.cause != nil {
		attrs = append(attrs, slog.Any("cause", e.cause))
	}

	if e.stack != nil {
		attrs = append(attrs, slog.Any("stacktrace", e.stack))
		attrs = append(attrs, slog.String("stacktrace_order", string(stackTraceOder)))
	}

	if len(e.messages) > 0 {
		attrs = append(attrs, slog.Any("messages", e.messages))
	}

	if len(e.checkpoints) > 0 {
		attrs = append(attrs, slog.Any("checkpoints", e.checkpoints))
	}

	return slog.GroupValue(attrs...)
}

// Format は fmt.Formatter を実装し、%+v指定子でスタックトレースの出力をサポートする
func (e *CustomError) Format(f fmt.State, c rune) {
	if c == 'v' && f.Flag('+') {
		io.WriteString(f, e.Error())

		// stacktrace
		if e.stack != nil {
			for _, fr := range e.stack {
				if fr.InApp {
					fmt.Fprintf(f, "\n\t%s\n\t\t%s:%d", fr.Function, fr.Filename, fr.Lineno)
				}
			}
		}

		// messages
		// TODO

		// checkpoints
		// TODO

		return
	}
	io.WriteString(f, e.Error())
}

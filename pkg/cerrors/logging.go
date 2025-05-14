package cerrors

import (
	"context"
	stdErrors "errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// PrettyTextHandler は slog.Handler をラップし、CustomError のスタックトレースを整形して出力する
type PrettyTextHandler struct {
	slog.Handler

	writer io.Writer
}

// NewPrettyTextHandler は出力先とハンドラオプションを指定して PrettyTextHandler を生成する
func NewPrettyTextHandler(w io.Writer, o *slog.HandlerOptions) *PrettyTextHandler {
	return &PrettyTextHandler{
		Handler: slog.NewTextHandler(w, o),
		writer:  w,
	}
}

// slog.Handler.Handle のオーバーライド
// Handle はログレコードを処理し、 CustomError の属性を展開してフラットに追加し、スタックトレースをインデント付きで出力する
func (h *PrettyTextHandler) Handle(ctx context.Context, r slog.Record) error {

	// contextual messages 属性と stacktrace 属性を取り出すために、CustomErrorを取り出す
	var ce *CustomError
	attrs := make([]slog.Attr, 0, 8)
	r.Attrs(func(a slog.Attr) bool {
		if errVal, ok := a.Value.Any().(error); ok {
			if stdErrors.As(errVal, &ce) {
				return false // 本体レコードには含めない
			}
		}
		attrs = append(attrs, a)
		return true
	})

	// CustomError の中身を平坦な slog.Attr として追加
	if ce != nil {
		// code, detail
		attrs = append(attrs,
			slog.String("err.code", ce.Code()),
			slog.String("err.detail", ce.Detail()),
		)

		// cause
		if ca := ce.Unwrap(); ca != nil {
			// スタックを含む error.String() を改行で分割し、先頭行だけを使う
			causeStr := ca.Error()
			if idx := strings.Index(causeStr, "\n"); idx >= 0 {
				causeStr = causeStr[:idx]
			}
			attrs = append(attrs, slog.String("err.cause", causeStr))
		}
	}

	rec := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	rec.AddAttrs(attrs...)
	if err := h.Handler.Handle(ctx, rec); err != nil {
		return err
	}

	// stacktrace を別行でインデント付きで出力
	if ce != nil && ce.stack != nil {
		fmt.Fprintln(h.writer, "  stacktrace:")
		for _, f := range ce.stack {
			if !f.InApp {
				continue
			}
			fmt.Fprintf(h.writer, "    %-30s %s:%d\n",
				f.Function, f.Filename, f.Lineno,
			)
		}
	}

	// messages を別行でインデント付きで出力
	if len(ce.messages) > 0 {
		fmt.Fprintln(h.writer, "  context:")
		for _, ctxMsg := range ce.messages {
			fmt.Fprintf(h.writer, "    %-30s %s:%d\n",
				ctxMsg.Message, ctxMsg.Filename, ctxMsg.Lineno,
			)
		}
	}

	// checkpoints
	// TODO

	return nil
}

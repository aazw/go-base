package cerrors

import (
	"bytes"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	crErrors "github.com/cockroachdb/errors"
)

func TestCustomError_Basic(t *testing.T) {
	// cause に標準 errors を使ってみる
	cause := stdErrors.New("root cause")
	err := ErrUnknown.New(
		WithMessage("ctx msg"),
		WithCause(cause),
	)

	// 1) Error() の文字列フォーマット
	s := err.Error()
	wantPrefix := "[UNKNOWN_ERROR] an unknown error occurred"
	if !strings.HasPrefix(s, wantPrefix) {
		t.Errorf("Error() = %q; want prefix %q", s, wantPrefix)
	}
	if !strings.Contains(s, "ctx msg") || !strings.HasSuffix(s, ": root cause") {
		t.Errorf("Error() = %q; want include context and cause", s)
	}

	// 2) Unwrap / errors.Is / errors.As の確認
	if !stdErrors.Is(err, cause) {
		t.Error("errors.Is(err, cause) = false; want true")
	}
	var ce *CustomError
	if !stdErrors.As(err, &ce) {
		t.Error("errors.As(err, *CustomError) = false; want true")
	}

	// 3) アクセサメソッド
	if got := ce.Code(); got != "UNKNOWN_ERROR" {
		t.Errorf("Code() = %q; want %q", got, "UNKNOWN_ERROR")
	}
	if got := ce.Detail(); got != "an unknown error occurred" {
		t.Errorf("Detail() = %q; want %q", got, "an unknown error occurred")
	}

	// 4) Messages() のテスト
	messages := ce.Messages()
	if len(messages) != 1 {
		t.Errorf("Messages() len = %d; want %d", len(messages), 1)
	} else {
		wantMsg := "ctx msg"
		if got := messages[0].Message; got != wantMsg {
			t.Errorf("Messages()[0].Msg = %q; want %q", got, wantMsg)
		}
		if messages[0].Filename == "" || messages[0].Lineno <= 0 {
			t.Errorf("Messages()[0] = %+v; want non-empty File and positive Line", messages[0])
		}
	}

	// 5) fmt.Sprintf("%+v", err) で少なくとも一つのスタックフレームが出力されること
	formatted := fmt.Sprintf("%+v", err)
	if !strings.Contains(formatted, "\n\t") {
		t.Errorf("%%+v output missing any stacktrace frame: %s", formatted)
	}
}

func TestCustomError_JSONLogging(t *testing.T) {
	var buf bytes.Buffer

	// JSONHandler で LogValue() が呼ばれることを確認
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		AddSource: false, // テストではソース不要
	})
	logger := slog.New(handler)

	err := ErrUnknown.New(
		WithMessage("ctx msg"),
		WithCause(crErrors.New("root cause")),
	)
	logger.Error("operation failed", "err", err)

	// 出力された JSON をパース
	var rec map[string]any
	if err2 := json.Unmarshal(buf.Bytes(), &rec); err2 != nil {
		t.Fatalf("json unmarshal error: %v\nraw: %s", err2, buf.String())
	}

	// "err" フィールドがオブジェクトになっていること
	obj, ok := rec["err"].(map[string]any)
	if !ok {
		t.Fatalf("rec[\"err\"] has wrong type: %T", rec["err"])
	}

	// code / detail を検証
	if got, _ := obj["code"].(string); got != "UNKNOWN_ERROR" {
		t.Errorf(`json err.code = %q; want "UNKNOWN_ERROR"`, got)
	}
	if got, _ := obj["detail"].(string); got != "an unknown error occurred" {
		t.Errorf(`json err.detail = %q; want "an unknown error occurred"`, got)
	}

	// messages は配列、最初の要素に message があること
	ctxRaw, ok := obj["messages"].([]any)
	if !ok || len(ctxRaw) != 1 {
		t.Fatalf("json err.messages type = %T, len=%d; want []any len 1", ctxRaw, len(ctxRaw))
	}
	ctx0, ok := ctxRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("json err.messages[0] type = %T; want map[string]any", ctxRaw[0])
	}
	if msg, _ := ctx0["message"].(string); msg != "ctx msg" {
		t.Errorf(`json err.messages[0].message = %q; want "ctx msg"`, msg)
	}

	// cause を検証
	if ca, _ := obj["cause"].(string); ca != "root cause" {
		t.Errorf(`json err.cause = %q; want "root cause"`, ca)
	}

	// stacktrace.frames が非空スライスであることをチェック
	stRaw, exists := obj["stacktrace"]
	if !exists {
		t.Error("json err.stacktrace missing")
	}
	framesArr, ok := stRaw.([]any)
	if !ok || len(framesArr) == 0 {
		t.Errorf("json err.stacktrace.frames = %v; want non-empty array", stRaw)
	}
}

// Test WithMessagef produces a formatted message.
func TestCustomError_WithMessagef(t *testing.T) {
	err := ErrUnknown.New(
		WithMessagef("value=%d", 42),
	)
	ce := &CustomError{}
	if !stdErrors.As(err, &ce) {
		t.Fatal("could not cast to *CustomError")
	}
	msgs := ce.Messages()
	if len(msgs) != 1 {
		t.Fatalf("CtxMsgs len = %d; want 1", len(msgs))
	}
	want := "value=42"
	if got := msgs[0].Message; got != want {
		t.Errorf("CtxMsgs()[0].Message = %q; want %q", got, want)
	}
}

// Test multiple WithMessage calls accumulate messages in order.
func TestCustomError_MultipleContexts(t *testing.T) {
	err := ErrUnknown.New(
		WithMessage("first"),
		WithMessage("second"),
	)
	ce := &CustomError{}
	if !stdErrors.As(err, &ce) {
		t.Fatal("could not cast to *CustomError")
	}
	msgs := ce.Messages()
	if len(msgs) != 2 {
		t.Fatalf("CtxMsgs len = %d; want 2", len(msgs))
	}
	if msgs[0].Message != "first" || msgs[1].Message != "second" {
		t.Errorf("messages = %q; want [\"first\" \"second\"]", []string{msgs[0].Message, msgs[1].Message})
	}
}

// Test nested CustomError causes still unwrap to the original cause.
func TestCustomError_NestedCause(t *testing.T) {
	root := stdErrors.New("root")
	inner := ErrUnknown.New(WithCause(root))
	outer := ErrUnknown.New(WithCause(inner))
	if !stdErrors.Is(outer, root) {
		t.Errorf("errors.Is(outer, root) = false; want true")
	}
}

// Test no-options ErrUnknown returns only code and detail.
func TestCustomError_NoOptions(t *testing.T) {
	err := ErrUnknown.New()
	s := err.Error()
	want := "[UNKNOWN_ERROR] an unknown error occurred"
	if s != want {
		t.Errorf("Error() = %q; want %q", s, want)
	}
	ce := &CustomError{}
	if !stdErrors.As(err, &ce) {
		t.Fatal("could not cast to *CustomError")
	}
	if len(ce.Messages()) != 0 {
		t.Errorf("messages len = %d; want 0", len(ce.Messages()))
	}
}

// Test %+v includes error code then in-app stack frames in correct order.
func TestCustomError_FormatStackOrder(t *testing.T) {
	// Create an error and format with %+v
	err := ErrUnknown.New()
	out := fmt.Sprintf("%+v", err)
	// Should start with [UNKNOWN_ERROR]
	if !strings.HasPrefix(out, "[UNKNOWN_ERROR]") {
		t.Errorf("%%+v output = %q; want prefix %q", out, "[UNKNOWN_ERROR]")
	}

	// スタックトレースのインデント付きフレーム行が含まれることを確認
	if !strings.Contains(out, "\n\t") {
		t.Errorf("%%+v output missing in-app frame: %s", out)
	}
}

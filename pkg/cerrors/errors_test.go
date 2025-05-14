package cerrors

import (
	"errors"
	"testing"
)

// TestErrorKindConstructors は、定義されたすべての ErrorKind に対して
// constructors マップに対応する customErrorConstructor が存在し、
// k.New() が CustomError を返すことを検証する
// 新しい ErrorKind を追加したら、以下の kinds スライスにも追加すること
func TestErrorKindConstructors(t *testing.T) {
	kinds := []errorKind{
		ErrUnknown,
		ErrInvalidInput,
		// 例: ErrFoo,
	}

	// constructors のカバレッジチェック
	if len(constructors) != len(kinds) {
		t.Fatalf("ErrorKind の数(%d) と constructors マップの要素数(%d) が一致しません", len(kinds), len(constructors))
	}

	for _, k := range kinds {
		err := k.New()
		var ce *CustomError
		if !errors.As(err, &ce) {
			t.Errorf("ErrorKind %v: expected *CustomError, got %T (%v)", k, err, err)
		}
	}
}

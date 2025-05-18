// pkg/api/ptrconv.go
package api

// Ptr は v のポインタを返す
// どんな型でも利用可能
func Ptr[T any](v T) *T {
	return &v
}

// PtrOrNil は v が T のゼロ値（== zero）なら nil を返し、
// そうでなければ v のポインタを返す
// 比較可能な型（comparable）でのみ利用可能
func PtrOrNil[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

// UpdateOrKeep は、newVal が nil でなくかつゼロ値でないなら *newVal を返し、
// それ以外は current を返す
// T は comparable 制約なので、ゼロ値チェックのために == zero が利用可能
func UpdateOrKeep[T comparable](current T, newVal *T) T {
	var zero T
	if newVal != nil && *newVal != zero {
		return *newVal
	}
	return current
}

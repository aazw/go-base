// pkg/webapi/ptrconv.go
package webapi

func stringPointer(str string) *string {
	if str == "" {
		return nil
	}
	return &str
}

func int32Pointer(i int32) *int32 {
	return &i
}

func intPointer(i int) *int32 {
	j := int32(i)
	return &j
}

package slices

func Merge[T any](a []T, b []T) []T {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	return append(a[:len(a):len(a)], b...)
}

func Shrink[T any](a []T) []T {
	if cap(a) > len(a) {
		a = append(make([]T, 0, len(a)), a...)
	}
	return a
}

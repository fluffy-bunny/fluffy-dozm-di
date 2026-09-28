package util

import "slices"

// ReverseSlice reverses s in place.
func ReverseSlice[T any](s []T) {
	slices.Reverse(s)
}

func ClipSlice[T any](s []T) []T {
	return s[:len(s):len(s)]
}

package stringslice

import (
	"fmt"
	"slices"
	"sort"
)

// Index returns the index of the s element in l.
// If s is not present in l, return -1.
func Index(s string, l []string) int {
	for i, e := range l {
		if e == s {
			return i
		}
	}
	return -1
}

// Remove returns a slice containing the elements of <slice> except <element>
func Remove[T comparable](slice []T, element T) []T {
	var result []T
	for _, item := range slice {
		if item != element {
			result = append(result, item)
		}
	}
	return result
}

// Equal returns a boolean reporting whether a == b
func Equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func Map(a []string, fn func(string) string) []string {
	var b []string
	for _, e := range a {
		b = append(b, fn(e))
	}
	return b
}

func first(data sort.Interface) {
	sort.Sort(data)
}

// next returns false when it cannot permute any more
// http://en.wikipedia.org/wiki/Permutation#Generation_in_lexicographic_order
func next(data sort.Interface) bool {
	var k, l int
	for k = data.Len() - 2; ; k-- {
		if k < 0 {
			return false
		}
		if data.Less(k, k+1) {
			break
		}
	}
	for l = data.Len() - 1; !data.Less(k, l); l-- {
	}
	data.Swap(k, l)
	for i, j := k+1, data.Len()-1; i < j; i++ {
		data.Swap(i, j)
		j--
	}
	return true
}

// Permute returns all possible permutations of string slice.
func Permute(slice []string) [][]string {
	first(sort.StringSlice(slice))

	copied1 := make([]string, len(slice)) // we need to make a copy!
	copy(copied1, slice)
	result := [][]string{copied1}

	for {
		isDone := next(sort.StringSlice(slice))
		if !isDone {
			break
		}

		// https://groups.google.com/d/msg/golang-nuts/ApXxTALc4vk/z1-2g1AH9jQJ
		// Lesson from Dave Cheney:
		// A slice is just a pointer to the underlying back array, your storing multiple
		// copies of the slice header, but they all point to the same backing array.

		// NOT
		// result = append(result, slice)

		copied2 := make([]string, len(slice))
		copy(copied2, slice)
		result = append(result, copied2)
	}

	combNum := 1
	for i := 0; i < len(slice); i++ {
		combNum *= i + 1
	}
	if len(result) != combNum {
		fmt.Printf("Expected %d combinations but %+v because of duplicate elements", combNum, result)
	}

	return result
}

// Diff returns removed, added from diff between a and b
func Diff(a, b []string) (removed, added []string) {
	for _, v := range a {
		if !slices.Contains(b, v) {
			removed = append(removed, v)
		}
	}
	for _, v := range b {
		if !slices.Contains(a, v) {
			added = append(added, v)
		}
	}
	return
}

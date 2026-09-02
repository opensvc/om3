// Package deepcopy copies the values the daemon datasets hold in fields
// typed any, where the concrete type is not known at compile time and so
// cannot be copied by a hand-written DeepCopy method.
//
// The datasets fill those fields two ways: decoded from a peer's heartbeat
// message, where the value is whatever encoding/json produces, or set
// directly by a driver on the local node, where it is any Go value at all.
// Reflection is what covers both without the callers having to enumerate
// types they do not control.
package deepcopy

import "reflect"

// Any returns a deep copy of v.
//
// Maps, slices, arrays, pointers and structs are rebuilt, so no reference
// is shared with v. Everything else is copied by assignment, which is what
// a scalar, a string or a chan needs.
//
// Unexported struct fields are copied as they are, since reflection cannot
// set them. That is correct for the value-like structs that reach here,
// time.Time among them, and no worse than the json round trip this
// replaces, which dropped them.
func Any[T any](v T) T {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		// v is a nil interface: there is nothing to copy.
		return v
	}
	copied, _ := copyValue(rv).Interface().(T)
	return copied
}

// Slice returns a copy of s sharing no backing array with it, and nil for
// a nil s, so that a caller serializing the result gets null where it got
// null and [] where it got [].
//
// The elements are copied by assignment, which is a deep copy only for a
// slice of values. Use Any for a slice whose elements hold references.
func Slice[S ~[]E, E any](s S) S {
	if s == nil {
		return nil
	}
	c := make(S, len(s))
	copy(c, s)
	return c
}

func copyValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Map:
		if v.IsNil() {
			// Keep nil distinct from empty: they do not serialize alike.
			return v
		}
		m := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			m.SetMapIndex(copyValue(iter.Key()), copyValue(iter.Value()))
		}
		return m
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		s := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			s.Index(i).Set(copyValue(v.Index(i)))
		}
		return s
	case reflect.Array:
		a := reflect.New(v.Type()).Elem()
		for i := range v.Len() {
			a.Index(i).Set(copyValue(v.Index(i)))
		}
		return a
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		p := reflect.New(v.Type().Elem())
		p.Elem().Set(copyValue(v.Elem()))
		return p
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		// Copy the concrete value, then re-wrap it in the interface type
		// the field is declared with.
		i := reflect.New(v.Type()).Elem()
		i.Set(copyValue(v.Elem()))
		return i
	case reflect.Struct:
		s := reflect.New(v.Type()).Elem()
		// Assign first, so the unexported fields reflection may not set
		// are carried over, then deepen the exported ones.
		s.Set(v)
		for i := range v.NumField() {
			if f := s.Field(i); f.CanSet() {
				f.Set(copyValue(v.Field(i)))
			}
		}
		return s
	default:
		return v
	}
}

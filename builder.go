// Package builder provides a method for writing fluent immutable builders.
package builder

import (
	"go/ast"
	"reflect"
	"github.com/mndrix/ps"
)

// Builder stores a set of named values.
// New types can be declared with underlying type Builder and used with the
// functions in this package. See the example.
// Instances of Builder should be treated as immutable. It is up to the
// implementor to ensure mutable values set on a Builder are not mutated while
// the Builder is in use.
type Builder struct {
	builderMap ps.Map
}

var emptyBuilderValue = reflect.ValueOf(Builder{ps.NewMap()})

func getBuilderMap(builder interface{}) ps.Map {
	b := convert(builder, Builder{}).(Builder)

	if b.builderMap == nil {
		return ps.NewMap()
	}

	return b.builderMap
}

// Set returns a copy of the given builder with a new value set for the given
// name.
// Set (and all other functions taking a builder in this package) will panic if
// the given builder's underlying type is not Builder.
func Set(builder interface{}, name string, v interface{}) interface{} {
	b := Builder{getBuilderMap(builder).Set(name, v)}
	return convert(b, builder)
}

// Append returns a copy of the given builder with new value(s) appended to the
// named list. If the value was previously unset or set with Set (even to a e.g.
// slice values), the new value(s) will be appended to an empty list.
func Append(builder interface{}, name string, vs ...interface{}) interface{} {
	return Extend(builder, name, vs)
}

// Extend behaves like Append, except it takes a single slice or array value
// which will be concatenated to the named list.
// Unlike a variadic call to Append - which requires a []interface{} value -
// Extend accepts slices or arrays of any type.
// Extend will panic if the given value is not a slice or array.
func Extend(builder interface{}, name string, vs interface{}) interface{} {
	maybeList, ok := getBuilderMap(builder).Lookup(name)

	var list ps.List
	if ok {
		list, ok = maybeList.(ps.List)
	}
	if !ok {
		list = ps.NewList()
	}

	forEach(vs, func(v interface{}) {
		list = list.Cons(v)
	})

	return Set(builder, name, list)
}

func listToSlice(list ps.List, arrayType reflect.Type) reflect.Value {
	size := list.Size()
	slice := reflect.MakeSlice(arrayType, size, size)
	for i := size - 1; i >= 0; i--  {
		val := reflect.ValueOf(list.Head())
		slice.Index(i).Set(val)
		list = list.Tail()
	}
	return slice
}

var anyArrayType = reflect.TypeOf([]interface{}{})

// Get retrieves a single named value from the given builder.
// If the value has not been set, it returns (nil, false). Otherwise, it will
// return (value, true).
//
// If the named value was last set with Append or Extend, the returned value
// will be a slice. If the given Builder has been registered with Register or
// RegisterType and the given name is an exported field of the registered
// struct, the returned slice will have the same type as that field. Otherwise
// the slice will have type []interface{}.
//
// Get will panic if the given name is a registered struct's exported field and
// the value set on the Builder is not assignable to the field.
func Get(builder interface{}, name string) (interface{}, bool) {
	val, ok := getBuilderMap(builder).Lookup(name)
	if !ok {
		return nil, false
	}

	list, isList := val.(ps.List)
	if isList {
		arrayType := anyArrayType

		if ast.IsExported(name) {
			structType := getBuilderStructType(reflect.TypeOf(builder))
			if structType != nil {
				field, ok := (*structType).FieldByName(name)
				if ok {
					arrayType = field.Type
				}
			}
		}

		val = listToSlice(list, arrayType).Interface()
	}

	return val, true
}

// GetStruct builds a new struct from the given registered builder.
// It will return nil if the given builder's type has not been registered with
// Register or RegisterValue.
//
// All values set on the builder with names that start with an uppercase letter
// (i.e. which would be exported if they were identifiers) are assigned to the
// corresponding exported fields of a new struct.
//
// GetStruct will panic if any of these "exported" values are not assignable to
// their corresponding struct fields.
func GetStruct(builder interface{}) interface{} {
	structVal := newBuilderStruct(reflect.TypeOf(builder))
	if structVal == nil {
		return nil
	}

	getBuilderMap(builder).ForEach(func(name string, val ps.Any) {
		if ast.IsExported(name) {
			field := structVal.FieldByName(name)

			list, isList := val.(ps.List)
			if isList {
				val = listToSlice(list, field.Type()).Interface()
			}

			field.Set(reflect.ValueOf(val))
		}
	})

	return structVal.Interface()
}
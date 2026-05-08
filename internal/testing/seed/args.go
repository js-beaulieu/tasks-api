package seed

import "testing"

func arg[T any](t *testing.T, args []any, index int, name string) T {
	t.Helper()

	if len(args) <= index {
		var zero T
		t.Fatalf("missing seed argument %q", name)
		return zero
	}
	if args[index] == nil {
		var zero T
		return zero
	}
	value, ok := args[index].(T)
	if !ok {
		var zero T
		t.Fatalf("seed argument %q has type %T", name, args[index])
		return zero
	}
	return value
}

func optionalArg[T any](args []any, index int) (T, bool) {
	if len(args) <= index || args[index] == nil {
		var zero T
		return zero, false
	}
	value, ok := args[index].(T)
	return value, ok
}

package tree

import (
	"fmt"
	"testing"
)

var result any

func BenchmarkIntegration(b *testing.B) {
	var res any

	for _, size := range []int{1, 5, 50, 500} {
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			in := getInputData(size)

			for b.Loop() {
				flatten, _ := New(in)
				flatten.Move([]string{"a", "*", "b", "*", "c"}, []string{"a", "*", "b", "*", "x"})
				flatten.Move([]string{"b", "b"}, []string{"c"})
				flatten.Del([]string{"b"})
				res = flatten.Get([]string{})
			}
		})
	}
	result = res
}

func BenchmarkExpand(b *testing.B) {
	var res any

	for _, size := range []int{1, 5, 50, 500} {
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			flatten, _ := New(getInputData(size))
			flatten.Move([]string{"a", "*", "b", "*", "c"}, []string{"a", "*", "b", "*", "x"})
			flatten.Move([]string{"b", "b"}, []string{"c"})
			flatten.Del([]string{"b"})

			for b.Loop() {
				res = flatten.Get([]string{})
			}
		})
	}
	result = res
}

func BenchmarkNew(b *testing.B) {
	var res any

	for _, size := range []int{1, 5, 50, 500} {
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			in := getInputData(size)

			for b.Loop() {
				res, _ = New(in)
			}
		})
	}
	result = res
}

func BenchmarkMove(b *testing.B) {
	var res *Tree

	for _, size := range []int{1, 5, 50, 500} {
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {

			res, _ = New(getInputData(size))

			b.ResetTimer()

			for n := range b.N {
				if n%2 == 0 {
					res.Move([]string{"a", "*", "b", "*", "c"}, []string{"a", "*", "b", "*", "x"})
					res.Move([]string{"b", "b"}, []string{"b", "c"})
				} else {
					res.Move([]string{"a", "*", "b", "*", "x"}, []string{"a", "*", "b", "*", "c"})
					res.Move([]string{"b", "c"}, []string{"b", "b"})
				}
			}
		})
	}
	result = res
}

func getInputData(size int) map[string]any {
	first := map[string]any{
		"b": []any{
			map[string]any{
				"c": map[string]any{
					"a": 1,
				},
				"aa": 1,
			},
			map[string]any{
				"c": map[string]any{
					"a": 2,
				},
				"aa": 1,
			},
		},
		"aa": 1,
	}
	second := map[string]any{
		"b": []any{
			map[string]any{
				"c": map[string]any{
					"a": 1,
				},
				"aa": 1,
			},
		},
		"aa": 1,
	}

	collection := make([]any, 2*size)
	for i := range size {
		collection[2*i] = first
		collection[2*i+1] = second
	}

	return map[string]any{
		"a": collection,
		"b": map[string]any{
			"a":  42,
			"bb": true,
			"b":  map[string]any{"a": 42},
		},
		"tupu": false,
	}
}

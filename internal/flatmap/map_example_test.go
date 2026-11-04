package flatmap

import (
	"fmt"
	"github.com/9edang/inkgate/internal/json"
)

func ExampleMap() {
	sample := map[string]any{
		"supu": 42,
		"tupu": false,
		"foo":  "bar",
		"aa":   23,
		"a": map[string]any{
			"aa":  23,
			"ac":  23,
			"abc": 23,
			"a": map[string]any{
				"a": map[string]any{
					"b": true,
					"c": 42,
					"d": "tupu",
				},
				"b": true,
				"c": 42,
				"d": "tupu",
			},
			"ab": map[string]any{
				"b": true,
				"c": 42,
				"d": "tupu",
			},
			"b": true,
			"c": 42,
			"d": "tupu",
			"collection": []any{
				map[string]any{
					"b":  false,
					"d":  "aaaa",
					"dx": "foobar",
				},
				map[string]any{
					"b": true,
					"c": 42,
					"d": "tupu",
				},
			},
			"collection2": []any{
				map[string]any{
					"d": []any{
						map[string]any{
							"a": 1,
							"d": []int{1, 2, 3, 4, 5},
						},
						map[string]any{
							"d": []int{1, 2, 3, 4, 5},
						},
						map[string]any{
							"d": []int{1, 2, 3, 4, 5},
						},
					},
				},
				map[string]any{
					"d": []int{1, 2, 3, 4, 5},
				},
				map[string]any{
					"d": []int{1, 2, 3, 4, 5},
				},
			},
		},
	}

	flatten, _ := Flatten(sample, DefaultTokenizer)

	flatten.Del("a.b")
	flatten.Del("a.*.a")
	flatten.Del("a.collection.1.b")
	flatten.Del("a.collection.*.d")
	flatten.Del("a.collection2.*.d.*.d")
	flatten.Move("tupu", "tuputupu")
	flatten.Move("a.c", "a.cb")
	flatten.Move("a.a", "acb")
	flatten.Del("a.ab")

	res := flatten.Expand()

	b, _ := json.MarshalIndent(res, "", "\t")
	fmt.Println(string(b))

	// output:
	// {
	// 	"a": {
	// 		"aa": 23,
	// 		"abc": 23,
	// 		"ac": 23,
	// 		"cb": 42,
	// 		"collection": [
	// 			{
	// 				"b": false,
	// 				"dx": "foobar"
	// 			},
	// 			{
	// 				"c": 42
	// 			}
	// 		],
	// 		"collection2": [
	// 			{
	// 				"d": [
	// 					{
	// 						"a": 1
	// 					},
	// 					null,
	// 					null
	// 				]
	// 			},
	// 			{
	// 				"d": [
	// 					1,
	// 					2,
	// 					3,
	// 					4,
	// 					5
	// 				]
	// 			},
	// 			{
	// 				"d": [
	// 					1,
	// 					2,
	// 					3,
	// 					4,
	// 					5
	// 				]
	// 			}
	// 		],
	// 		"d": "tupu"
	// 	},
	// 	"aa": 23,
	// 	"acb": {
	// 		"b": true,
	// 		"c": 42,
	// 		"d": "tupu"
	// 	},
	// 	"foo": "bar",
	// 	"supu": 42,
	// 	"tuputupu": false
	// }
}

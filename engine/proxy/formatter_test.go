// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"github.com/9edang/inkgate/context"
	"reflect"
	"testing"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
)

func TestEntityFormatterFunc(t *testing.T) {
	expected := Response{Data: map[string]any{"one": 1}, IsComplete: true}
	f := func(_ Response) Response { return expected }
	formatter := EntityFormatterFunc(f)
	result := formatter.Format(Response{})
	if result.Data["one"].(int) != 1 {
		t.Error("unexpected result:", result.Data)
	}
	if !result.IsComplete {
		t.Error("unexpected result:", result)
	}
}

func TestEntityFormatter_newAllowFilter(t *testing.T) {
	sample := Response{
		Data: map[string]any{
			"supu": 42,
			"tupu": false,
			"foo":  "bar",
			"a": map[string]any{
				"b": true,
				"c": 42,
				"d": "tupu",
			},
		},
		IsComplete: true,
	}
	expected := Response{
		Data: map[string]any{
			"supu": 42,
			"a": map[string]any{
				"b": true,
				"c": 42,
			},
		},
		IsComplete: true,
	}
	f := NewEntityFormatter(&config.Backend{AllowList: []string{"supu", "a.b", "a.c", "foo.unknown"}})
	result := f.Format(sample)
	if v, ok := result.Data["supu"]; !ok || v != expected.Data["supu"] {
		t.Errorf("The formatter returned an unexpected result for the field supu: %v\n", result)
	}
	v, ok := result.Data["a"]
	if !ok {
		t.Errorf("The formatter returned an unexpected result for the fields a.b & a.c: %v\n", result)
	}
	tmp := v.(map[string]any)
	if b, okk := tmp["b"]; !okk || !b.(bool) {
		t.Errorf("The formatter returned an unexpected result for the field a.b: %v\n", result)
	}
	if c, okk := tmp["c"]; !okk || c.(int) != 42 {
		t.Errorf("The formatter returned an unexpected result for the field a.c: %v\n", result)
	}
	if len(tmp) != 2 {
		t.Errorf("The formatter returned an unexpected result size for the field a: %v\n", result)
	}
	if len(result.Data) != 2 || result.IsComplete != expected.IsComplete {
		t.Errorf("The formatter returned an unexpected result size: %v\n", result)
	}
}

func TestEntityFormatter_newAllowDeepFields(t *testing.T) {
	sample := Response{
		Data: map[string]any{
			"id": 42,
			"tupu": map[string]any{
				"muku": map[string]any{
					"supu": 1,
					"muku": 2,
					"gutu": map[string]any{
						"kugu": 42,
					},
				},
				"supu": map[string]any{
					"supu": 3,
					"muku": 4,
				},
			},
		},
		IsComplete: true,
	}
	expectedSupuChild := 1

	var ok bool
	f := NewEntityFormatter(&config.Backend{AllowList: []string{"tupu.muku.supu", "tupu.muku.gutu.kugu"}})
	res := f.Format(sample)
	var tupu map[string]any
	var muku map[string]any
	var gutu map[string]any
	var kugu int
	var supuChild int
	if tupu, ok = res.Data["tupu"].(map[string]any); !ok {
		t.Errorf("The formatter does not have field tupu\n")
	}
	if muku, ok = tupu["muku"].(map[string]any); !ok {
		t.Errorf("The formatter does not have field tupu.muku\n")
	}
	if supuChild, ok = muku["supu"].(int); !ok || supuChild != expectedSupuChild {
		t.Errorf("The formatter does not have field tupu.muku.supu or wrong value\n")
	}
	if _, ok = tupu["supu"].(map[string]any); ok {
		t.Errorf("The formatter should have removed tupu.supu\n")
	}
	if _, ok = muku["muku"]; ok {
		t.Errorf("The formatter should have removed tupu.muku.muku\n")
	}
	if gutu, ok = muku["gutu"].(map[string]any); !ok {
		t.Errorf("The formatter does not have field tupu.muku.gutu\n")
	}
	if kugu, ok = gutu["kugu"].(int); !ok || kugu != 42 {
		t.Errorf("The formatter does not have field tupu.muku.gutu.kugu\n")
	}
}

func TestEntityFormatter_newDenyFilter(t *testing.T) {
	sample := Response{
		Data: map[string]any{
			"supu": 42,
			"tupu": false,
			"foo":  "bar",
			"a": map[string]any{
				"a": map[string]any{
					"b": true,
					"c": 42,
					"d": "tupu",
					"deeper": map[string]any{
						"a": map[string]any{
							"aa": "deleteme deeper.a.aa",
							"bb": "do not deleteme deeper.a.bb",
						},
						"b": map[string]any{
							"aa": "deleteme deeper.b.aa",
							"bb": "do not deleteme deeper.b.bb",
						},
					},
				},
				"b": true,
				"c": 42,
				"d": "tupu",
			},
		},
		IsComplete: true,
	}
	expected := Response{
		Data: map[string]any{
			"tupu": false,
			"foo":  "bar",
			"a": map[string]any{
				"a": map[string]any{
					"c": 42,
					"d": "tupu",
					"deeper": map[string]any{
						"a": map[string]any{
							"bb": "do not deleteme deeper.a.bb",
						},
						"b": map[string]any{
							"bb": "do not deleteme deeper.b.bb",
						},
					},
				},
				"d": "tupu",
			},
		},
		IsComplete: true,
	}
	f := NewEntityFormatter(&config.Backend{DenyList: []string{
		"supu",
		"a.b",
		"a.c",
		"foo.unknown",
		"a.a.b",
		"a.a.deeper.a.aa",
		"a.a.deeper.b.aa",
	}})
	result := f.Format(sample)
	if v, ok := result.Data["tupu"]; !ok || v != expected.Data["tupu"] {
		t.Errorf("The formatter returned an unexpected result for the field tupu: %v\n", result)
	}
	if v, ok := result.Data["foo"]; !ok || v != expected.Data["foo"] {
		t.Errorf("The formatter returned an unexpected result for the field foo: %v\n", result)
	}
	v, ok := result.Data["a"]
	if !ok {
		t.Errorf("The formatter returned an unexpected result for the field a.d: %v\n", result)
	}
	tmp := v.(map[string]any)
	if d, okk := tmp["d"]; !okk || d != "tupu" {
		t.Errorf("The formatter returned an unexpected result for the field a.d: %v\n", result)
	}
	if len(tmp) != 2 {
		// a.a should exist , and a.d should exist
		t.Errorf("The formatter returned an unexpected result size for the field a: %v\n", result)
	}
	if len(result.Data) != 3 || result.IsComplete != expected.IsComplete {
		t.Errorf("The formatter returned an unexpected result size: %v\n", result)
	}

	if !reflect.DeepEqual(expected.Data, result.Data) {
		t.Errorf("unexpected response. have: %+v, want: %+v", result.Data, expected.Data)
	}
}

func TestEntityFormatter_grouping(t *testing.T) {
	preffix := "group1"
	sample := Response{
		Data: map[string]any{
			"supu": 42,
			"tupu": false,
			"foo":  "bar",
		},
		IsComplete: true,
	}
	expected := Response{
		Data: map[string]any{
			preffix: map[string]any{
				"supu": 42,
				"tupu": false,
				"foo":  "bar",
			},
		},
		IsComplete: true,
	}
	f := NewEntityFormatter(&config.Backend{Group: preffix})
	result := f.Format(sample)
	if len(result.Data) != 1 || result.IsComplete != expected.IsComplete {
		t.Fail()
	}
	if _, ok := result.Data[preffix]; !ok {
		t.Fail()
	}
	group := result.Data[preffix].(map[string]any)
	for k, expectedValue := range expected.Data[preffix].(map[string]any) {
		if v, ok := group[k]; !ok || v != expectedValue {
			t.Fail()
		}
	}
}

func TestEntityFormatter_mapping(t *testing.T) {
	mapping := map[string]string{"supu": "SUPUUUUU", "tupu": "TUPUUUUU", "a.b": "a.BOOOOO"}

	sub := map[string]any{
		"b": true,
		"c": 42,
		"d": "tupu",
	}
	sample := Response{
		Data: map[string]any{
			"supu": 42,
			"tupu": false,
			"foo":  "bar",
			"a":    sub,
		},
		IsComplete: true,
	}
	expected := Response{
		Data: map[string]any{
			"SUPUUUUU": 42,
			"TUPUUUUU": false,
			"foo":      "bar",
			"a":        sub,
		},
		IsComplete: true,
	}
	f := NewEntityFormatter(&config.Backend{Mapping: mapping})
	result := f.Format(sample)

	if len(result.Data) != 4 || result.IsComplete != expected.IsComplete {
		t.Errorf("The formatter returned an unexpected result size: %v\n", result.Data)
	}
	for k, expectedValue := range expected.Data {
		if k == "a" {
			continue
		}
		if v, ok := result.Data[k]; !ok || v != expectedValue {
			t.Errorf("The formatter returned an unexpected result for the key %s: %v\n", k, v)
		}
	}
	group := result.Data["a"].(map[string]any)
	for k, expectedValue := range expected.Data["a"].(map[string]any) {
		if v, ok := group[k]; !ok || v != expectedValue {
			t.Errorf("The formatter returned an unexpected result for the key %s: %v\n", k, v)
		}
	}

	if len(group) != 3 {
		t.Errorf("The formatter returned an unexpected result size for the subentity: %v\n", group)
	}
}

func TestEntityFormatter_targeting(t *testing.T) {
	target := "group1"
	sub := map[string]any{
		"b": true,
		"c": 42,
		"d": "tupu",
	}
	sample := Response{
		Data: map[string]any{
			"supu": 42,
			"tupu": false,
			"foo":  "bar",
			target: sub,
		},
		IsComplete: true,
	}
	expected := Response{
		Data:       sub,
		IsComplete: true,
	}
	f := NewEntityFormatter(&config.Backend{Target: target})
	result := f.Format(sample)
	if len(result.Data) != 3 || result.IsComplete != expected.IsComplete {
		t.Errorf("The formatter returned an unexpected result size: %v\n", result)
	}
	for k, expectedValue := range expected.Data {
		if v, ok := result.Data[k]; !ok || v != expectedValue {
			t.Errorf("The formatter returned an unexpected result for the key %s: %v\n", k, v)
		}
	}
}

func TestEntityFormatter_targetingNested(t *testing.T) {
	target := "group1"
	sub := map[string]any{
		"b": true,
		"c": 42,
		"d": "tupu",
	}
	sample := Response{
		Data: map[string]any{
			target: map[string]any{
				"supu": 42,
				"tupu": false,
				"foo":  "bar",
				target: sub,
			},
		},
		IsComplete: true,
	}
	expected := Response{
		Data:       sub,
		IsComplete: true,
	}
	f := NewEntityFormatter(&config.Backend{Target: target + "." + target})
	result := f.Format(sample)
	if len(result.Data) != 3 || result.IsComplete != expected.IsComplete {
		t.Errorf("The formatter returned an unexpected result size: %v\n", result)
	}
	for k, expectedValue := range expected.Data {
		if v, ok := result.Data[k]; !ok || v != expectedValue {
			t.Errorf("The formatter returned an unexpected result for the key %s: %v\n", k, v)
		}
	}
}

func TestEntityFormatter_targetingUnknownFields(t *testing.T) {
	target := "group1"
	sample := Response{
		Data: map[string]any{
			"supu": 42,
			"tupu": false,
			"foo":  "bar",
		},
		IsComplete: true,
	}
	f := NewEntityFormatter(&config.Backend{Target: target})
	result := f.Format(sample)
	if len(result.Data) != 0 || result.IsComplete != sample.IsComplete {
		t.Errorf("The formatter returned an unexpected result size: %v\n", result)
	}
}

func TestEntityFormatter_targetingNonObjects(t *testing.T) {
	target := "group1"
	sample := Response{
		Data: map[string]any{
			"supu": 42,
			"tupu": false,
			"foo":  "bar",
			target: false,
		},
		IsComplete: true,
	}
	f := NewEntityFormatter(&config.Backend{Target: target})
	result := f.Format(sample)
	if len(result.Data) != 0 || result.IsComplete != sample.IsComplete {
		t.Errorf("The formatter returned an unexpected result size: %v\n", result)
	}
}

func TestEntityFormatter_altogether(t *testing.T) {
	sample := Response{
		Data: map[string]any{
			"supu": 42,
			"tupu": false,
			"foo":  "bar",
			"a": map[string]any{
				"b": true,
				"c": 42,
				"d": "tupu",
			},
		},
		IsComplete: true,
	}
	expected := Response{
		Data: map[string]any{
			"group": map[string]any{
				"D": "tupu",
			},
		},
		IsComplete: true,
	}
	f := NewEntityFormatter(&config.Backend{
		Target:    "a",
		AllowList: []string{"d"},
		Group:     "group",
		Mapping:   map[string]string{"d": "D"},
	})
	result := f.Format(sample)
	v, ok := result.Data["group"]
	if !ok {
		t.Errorf("The formatter returned an unexpected result for the field group.D: %v\n", result)
	}
	tmp := v.(map[string]any)
	if d, okk := tmp["D"]; !okk || d != "tupu" {
		t.Errorf("The formatter returned an unexpected result for the field group.D: %v\n", result)
	}
	if len(tmp) != 1 {
		t.Errorf("The formatter returned an unexpected result size for the field group: %v\n", result)
	}
	if len(result.Data) != 1 || result.IsComplete != expected.IsComplete {
		t.Errorf("The formatter returned an unexpected result size: %v\n", result)
	}
}

func TestEntityFormatter_flatmap(t *testing.T) {
	sub := map[string]any{
		"b": true,
		"c": 42,
		"d": "tupu",
		"e": []any{1, 2, 3, 4},
	}
	sample := Response{
		Data: map[string]any{
			"content": map[string]any{
				"supu":       42,
				"tupu":       false,
				"foo":        "bar",
				"a":          sub,
				"collection": []any{sub, sub, sub, sub},
				"y":          []any{0, 1, 2, 3, 4, 5, 6},
				"z":          []any{10, 11, 12, 13, 14, 15, 16},
			},
		},
		IsComplete: true,
	}
	expected := Response{
		Data: map[string]any{
			"group": map[string]any{
				"SUPUUUUU": 42,
				"tupu":     false,
				"foo":      "bar",
				"a": map[string]any{
					"BOOOOO": true,
					"c":      42,
					"d":      "tupu",
					"e":      []any{1, 2, 3, 4},
				},
				"collection": []any{
					map[string]any{"x": 42},
					map[string]any{"x": 42},
					map[string]any{"x": 42},
					map[string]any{"x": 42},
				},
				"z": []any{10, 11, 12, 13, 14, 15, 16, 0, 1, 2, 3, 4, 5, 6},
			},
		},
		IsComplete: true,
	}
	f := NewEntityFormatter(&config.Backend{
		Target: "content",
		Group:  "group",
		ExtraConfig: config.ExtraConfig{
			Namespace: map[string]any{
				flatmapKey: []any{
					map[string]any{
						"type": "del",
						"args": []any{"c"},
					},
					map[string]any{
						"type": "append",
						"args": []any{"y", "z"},
					},
					map[string]any{
						"type": "move",
						"args": []any{"supu", "SUPUUUUU"},
					},
					map[string]any{
						"type": "move",
						"args": []any{"a.b", "a.BOOOOO"},
					},
					map[string]any{
						"type": "del",
						"args": []any{
							"collection.*.b",
							"collection.*.d",
							"collection.*.e",
						},
					},
					map[string]any{
						"type": "move",
						"args": []any{"collection.*.c", "collection.*.x"},
					},
				},
			},
		},
	})
	result := f.Format(sample)

	if len(result.Data) != len(expected.Data) || result.IsComplete != expected.IsComplete {
		t.Errorf("The formatter returned an unexpected result size: %v\n", result.Data)
	}

	if !reflect.DeepEqual(expected.Data, result.Data) {
		t.Errorf("unexpected result: %v", result.Data)
	}
}

func TestNewFlatmapMiddleware(t *testing.T) {
	sub := map[string]any{
		"b": true,
		"c": 42,
		"d": "tupu",
		"e": []any{1, 2, 3, 4},
	}
	sample := Response{
		Data: map[string]any{
			"supu":       42,
			"tupu":       false,
			"foo":        "bar",
			"a":          sub,
			"collection": []any{sub, sub, sub, sub},
			"y":          []any{0, 1, 2, 3, 4, 5, 6},
			"z":          []any{10, 11, 12, 13, 14, 15, 16},
		},
		IsComplete: true,
	}
	expected := Response{
		Data: map[string]any{
			"SUPUUUUU": 42,
			"tupu":     false,
			"foo":      "bar",
			"a": map[string]any{
				"BOOOOO": true,
				"c":      42,
				"d":      "tupu",
				"e":      []any{1, 2, 3, 4},
			},
			"collection": []any{
				map[string]any{"x": 42},
				map[string]any{"x": 42},
				map[string]any{"x": 42},
				map[string]any{"x": 42},
			},
			"z": []any{10, 11, 12, 13, 14, 15, 16, 0, 1, 2, 3, 4, 5, 6},
		},
		IsComplete: true,
	}
	p := NewFlatmapMiddleware(
		logging.NoOp,
		&config.EndpointConfig{
			ExtraConfig: config.ExtraConfig{
				Namespace: map[string]any{
					flatmapKey: []any{
						map[string]any{
							"type": "del",
							"args": []any{"c"},
						},
						map[string]any{
							"type": "append",
							"args": []any{"y", "z"},
						},
						map[string]any{
							"type": "move",
							"args": []any{"supu", "SUPUUUUU"},
						},
						map[string]any{
							"type": "move",
							"args": []any{"a.b", "a.BOOOOO"},
						},
						map[string]any{
							"type": "del",
							"args": []any{"collection.*.b"},
						},
						map[string]any{
							"type": "del",
							"args": []any{"collection.*.d"},
						},
						map[string]any{
							"type": "del",
							"args": []any{"collection.*.e"},
						},
						map[string]any{
							"type": "move",
							"args": []any{"collection.*.c", "collection.*.x"},
						},
					},
				},
			},
		},
	)(func(_ context.Context, _ *Request) (*Response, error) {
		return &sample, nil
	})

	result, err := p(context.TODO(), nil)

	if err != nil {
		t.Error(err)
	}

	if len(result.Data) != len(expected.Data) {
		t.Errorf("The formatter returned an unexpected result size: %v\n", result.Data)
	}

	if result.IsComplete != expected.IsComplete {
		t.Errorf("The formatter returned an unexpected completion flag: %v\n", result.IsComplete)
	}

	if !reflect.DeepEqual(expected.Data, result.Data) {
		t.Errorf("unexpected result: %v", result.Data)
	}
}

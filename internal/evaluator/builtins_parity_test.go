package evaluator

import "testing"

func TestBuiltinNames(t *testing.T) {
	expected := map[string]bool{
		"print":            true,
		"len":              true,
		"str":              true,
		"join":             true,
		"keys":             true,
		"values":           true,
		"range":            true,
		"append":           true,
		"push":             true,
		"count":            true,
		"remove":           true,
		"get":              true,
		"pop":              true,
		"hasKey":           true,
		"sort":             true,
		"max":              true,
		"abs":              true,
		"sum":              true,
		"reverse":          true,
		"any":              true,
		"all":              true,
		"map":              true,
		"mean":             true,
		"error":            true,
		"writeFile":        true,
		"sqrt":             true,
		"input":            true,
		"getpass":          true,
		"math_floor":       true,
		"math_sqrt":        true,
		"math_sin":         true,
		"math_cos":         true,
		"gfx_open":         true,
		"gfx_close":        true,
		"gfx_shouldClose":  true,
		"gfx_beginFrame":   true,
		"gfx_endFrame":     true,
		"gfx_clear":        true,
		"gfx_rect":         true,
		"gfx_pixel":        true,
		"gfx_time":         true,
		"gfx_keyDown":      true,
		"gfx_mouseX":       true,
		"gfx_mouseY":       true,
		"gfx_present":      true,
		"image_new":        true,
		"image_set":        true,
		"image_fill":       true,
		"image_fill_rect":  true,
		"image_fade":       true,
		"image_fade_white": true,
		"image_width":      true,
		"image_height":     true,
		"group_digits":     true,
		"format_float":     true,
		"format_percent":   true,
	}

	if len(builtins) != len(expected) {
		t.Fatalf("expected %d builtins, got %d", len(expected), len(builtins))
	}
	for name := range expected {
		if _, ok := builtins[name]; !ok {
			t.Fatalf("missing builtin: %s", name)
		}
	}
	for name := range builtins {
		if !expected[name] {
			t.Fatalf("unexpected builtin: %s", name)
		}
	}
}

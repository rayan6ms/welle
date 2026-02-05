package spec_test

import (
	"testing"

	"welle/internal/spectest"
)

type specCase struct {
	name      string
	source    string
	files     map[string]string
	entry     string
	maxMemory int64
	expect    map[spectest.Mode]spectest.Expectation
}

func TestSpecBaseline(t *testing.T) {
	cases := []specCase{
		{
			name: "strings_and_escapes",
			source: "print(\"line\\nbreak\")\n" +
				"print(`raw \\n not escaped`)\n" +
				"print(\"\"\"multi\nline\"\"\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "line\nbreak\nraw \\n not escaped\nmulti\nline\n",
			}),
		},
		{
			name: "arithmetic_precedence",
			source: "print(1 + 2 * 3)\n" +
				"print((1 + 2) * 3)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "7\n9\n",
			}),
		},
		{
			name: "bitwise_basic",
			source: "print(5 | 2)\n" +
				"print(5 & 2)\n" +
				"print(5 ^ 2)\n" +
				"print(~0)\n" +
				"print(5 << 1)\n" +
				"print(5 >> 1)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "7\n0\n7\n-1\n10\n2\n",
			}),
		},
		{
			name: "bitwise_precedence",
			source: "print(1 | 2 & 3)\n" +
				"print((1 | 2) & 3)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\n3\n",
			}),
		},
		{
			name: "bitwise_shift_precedence",
			source: "print(1 + 2 << 3)\n" +
				"print(1 << 2 < 3)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "24\nfalse\n",
			}),
		},
		{
			name: "membership_basic",
			source: "print(2 in [1, 2, 3])\n" +
				"print(\"ell\" in \"hello\")\n" +
				"print(\"a\" in #{\"a\": 1})\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "true\ntrue\ntrue\n",
			}),
		},
		{
			name:   "membership_array_type_error",
			source: "print(1 in [\"1\"])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "type mismatch: INTEGER == STRING",
			}),
		},
		{
			name:   "membership_string_lhs_type_error",
			source: "print(1 in \"abc\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "left operand of 'in' must be string when right operand is string",
			}),
		},
		{
			name:   "membership_non_iterable_error",
			source: "print(1 in 2)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "cannot use 'in' with INTEGER",
			}),
		},
		{
			name:   "membership_dict_key_error",
			source: "print([1] in #{})\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "unusable as dict key: ARRAY",
			}),
		},
		{
			name: "sqrt_basic",
			source: "print(sqrt(9))\n" +
				"print(sqrt(9) == math_sqrt(9))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\ntrue\n",
			}),
		},
		{
			name:   "sqrt_arity_error",
			source: "sqrt()\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "math_sqrt expects 1 argument",
			}),
		},
		{
			name:   "sqrt_type_error",
			source: "sqrt(\"x\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "math_sqrt expects NUMBER",
			}),
		},
		{
			name:   "input_noninteractive_error",
			source: "input()\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "input is not available in non-interactive mode",
			}),
		},
		{
			name:   "getpass_noninteractive_error",
			source: "getpass()\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "getpass is not available in non-interactive mode",
			}),
		},
		{
			name:   "bitwise_shift_range_error",
			source: "print(1 << 64)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "shift count out of range",
			}),
		},
		{
			name:   "bitwise_shift_negative_error",
			source: "print(1 >> -1)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "shift count cannot be negative",
			}),
		},
		{
			name:   "bitwise_type_error",
			source: "print(1 | 1.0)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "unsupported operand types for |: INTEGER, FLOAT",
			}),
		},
		{
			name:   "bitwise_type_error_string",
			source: "print(\"a\" | 1)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "unsupported operand types for |: STRING, INTEGER",
			}),
		},
		{
			name:   "bitwise_unary_type_error",
			source: "print(~1.5)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "unsupported operand type for ~: FLOAT",
			}),
		},
		{
			name: "numeric_literals_modern",
			source: "print(0b1010)\n" +
				"print(0o755)\n" +
				"print(0xFF)\n" +
				"print(0xFF_FF)\n" +
				"print(1_000_000)\n" +
				"print(3.141_592)\n" +
				"print(1_2.3_4)\n" +
				"print(1e3)\n" +
				"print(1.25e-2)\n" +
				"print(10E+2)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "10\n493\n255\n65535\n1000000\n3.141592\n12.34\n1000\n0.0125\n1000\n",
			}),
		},
		{
			name:   "numeric_literal_rejects_leading_underscore",
			source: "_1\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name:   "numeric_literal_rejects_trailing_underscore",
			source: "1_\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name:   "numeric_literal_rejects_double_underscore",
			source: "1__2\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name:   "numeric_literal_rejects_underscore_before_dot",
			source: "1_.2\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name:   "numeric_literal_rejects_underscore_after_dot",
			source: "1._2\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name:   "numeric_literal_rejects_prefix_underscore",
			source: "0x_FF\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name:   "numeric_literal_rejects_prefix_trailing_underscore",
			source: "0xFF_\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name:   "numeric_literal_rejects_exponent_underscore",
			source: "1e_3\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name:   "numeric_literal_rejects_mantissa_underscore_before_exponent",
			source: "1_e3\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name:   "numeric_literal_rejects_exponent_missing_digits",
			source: "1e\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name:   "numeric_literal_rejects_exponent_missing_digits_plus",
			source: "1e+\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name:   "numeric_literal_rejects_exponent_missing_digits_minus",
			source: "1e-\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name: "logic_precedence_and_short_circuit",
			source: "x = 0\n" +
				"func bump() { x = x + 1\n" +
				"  return true\n" +
				"}\n" +
				"print(not false or false)\n" +
				"print(true and 0)\n" +
				"print(false or 0)\n" +
				"false and bump()\n" +
				"true or bump()\n" +
				"print(x)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "true\ntrue\ntrue\n0\n",
			}),
		},
		{
			name: "ternary_basic",
			source: "print(true ? 1 : 2)\n" +
				"print(false ? 1 : 2)\n" +
				"print(nil ? 1 : 2)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1\n2\n2\n",
			}),
		},
		{
			name: "ternary_right_associative",
			source: "print(true ? 1 : false ? 2 : 3)\n" +
				"print(false ? 1 : true ? 2 : 3)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1\n2\n",
			}),
		},
		{
			name: "ternary_short_circuit",
			source: "func boom() { throw \"no\" }\n" +
				"print(true ? 1 : boom())\n" +
				"print(false ? boom() : 2)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1\n2\n",
			}),
		},
		{
			name: "null_alias_basic",
			source: "print(null == nil)\n" +
				"print(null != nil)\n" +
				"print(not null)\n" +
				"print(null != 0)\n" +
				"print(null)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "true\nfalse\ntrue\ntrue\nnil\n",
			}),
		},
		{
			name:   "null_identifier_error",
			source: "null = 1\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "invalid assignment target",
			}),
		},
		{
			name: "nullish_coalescing_basic",
			source: "print(nil ?? 123)\n" +
				"print(null ?? 123)\n" +
				"print(false ?? 123)\n" +
				"print(0 ?? 123)\n" +
				"print(\"[\" + (\"\" ?? \"x\") + \"]\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "123\n123\nfalse\n0\n[]\n",
			}),
		},
		{
			name: "nullish_short_circuit_no_eval",
			source: "func boom() { throw \"boom\" }\n" +
				"x = 0\n" +
				"y = x ?? boom()\n" +
				"print(y)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "0\n",
			}),
		},
		{
			name:   "nullish_short_circuit_eval",
			source: "x = nil\n" + "y = x ?? (1 / 0)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "division by zero",
			}),
		},
		{
			name:   "nullish_right_associative",
			source: "print(nil ?? nil ?? 7)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "7\n",
			}),
		},
		{
			name: "nullish_precedence_with_or",
			source: "x = nil ?? 1\n" +
				"print(x)\n" +
				"print(false or nil ?? 1)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1\nfalse\n",
			}),
		},
		{
			name: "ternary_precedence",
			source: "x = true ? 1 : 2\n" +
				"print(x)\n" +
				"print(true or false ? 1 : 2)\n" +
				"d = #{\"x\": true ? 1 : 2}\n" +
				"print(d[\"x\"])\n" +
				"print(max([true ? 1 : 2, 0]))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1\n1\n1\n1\n",
			}),
		},
		{
			name: "bang_operator_truthiness",
			source: "print(!false)\n" +
				"print(!nil)\n" +
				"print(!0)\n" +
				"print(!\"\")\n" +
				"print(![1, 2])\n" +
				"print(!(1 < 2))\n" +
				"print(!false == true)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "true\ntrue\nfalse\nfalse\nfalse\nfalse\ntrue\n",
			}),
		},
		{
			name: "null_alias_nil",
			source: "print(null == nil)\n" +
				"print(null != nil)\n" +
				"if (null) { print(\"t\") } else { print(\"f\") }\n" +
				"d = #{}\n" +
				"print(d[\"x\"] == null)\n" +
				"print((null or true) == true)\n" +
				"print((null and true) == false)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "true\nfalse\nf\ntrue\ntrue\ntrue\n",
			}),
		},
		{
			name:   "if_requires_parentheses",
			source: "if true { print(1) }\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name: "if_single_stmt_return",
			source: "func f(x) { if (x) return 1; return 2 }\n" +
				"print(f(true))\n" +
				"print(f(false))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1\n2\n",
			}),
		},
		{
			name: "if_single_stmt_loop_control",
			source: "i = 0\n" +
				"sum = 0\n" +
				"while (i < 5) {\n" +
				"  i = i + 1\n" +
				"  if (i == 2) continue\n" +
				"  if (i == 4) break\n" +
				"  sum = sum + i\n" +
				"}\n" +
				"print(sum)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "4\n",
			}),
		},
		{
			name: "if_single_stmt_throw",
			source: "caught = false\n" +
				"try { if (true) throw \"err\" } catch (e) { caught = true }\n" +
				"print(caught)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "true\n",
			}),
		},
		{
			name:   "if_single_stmt_dangling_else",
			source: "if (true) if (false) print(1) else print(2)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "2\n",
			}),
		},
		{
			name: "while_loop",
			source: "i = 0\n" +
				"sum = 0\n" +
				"while (i < 3) { sum = sum + i; i = i + 1 }\n" +
				"print(sum)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\n",
			}),
		},
		{
			name: "for_c_style",
			source: "sum = 0\n" +
				"for (i = 0; i < 4; i = i + 1) { sum = sum + i }\n" +
				"print(sum)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "6\n",
			}),
		},
		{
			name: "break_continue",
			source: "for (i = 0; i < 6; i = i + 1) {\n" +
				"  if (i == 2) { continue }\n" +
				"  if (i == 4) { break }\n" +
				"  print(i)\n" +
				"}\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "0\n1\n3\n",
			}),
		},
		{
			name: "pass_statement_noop",
			source: "x = 1\n" +
				"pass\n" +
				"if (true) { pass }\n" +
				"for (i = 0; i < 1; i = i + 1) { pass }\n" +
				"print(x)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1\n",
			}),
		},
		{
			name: "builtins_core_qol",
			source: "print(abs(-3))\n" +
				"print(abs(-2.5))\n" +
				"print(sum([1, 2, 3]))\n" +
				"print(sum([]))\n" +
				"print(sum([1, 2.5, 3]))\n" +
				"a = [1, 2, 3]\n" +
				"b = reverse(a)\n" +
				"print(a[0])\n" +
				"print(b[0])\n" +
				"print(reverse(\"ab😊\"))\n" +
				"print(max([1, 2.5, 2]))\n" +
				"print(max([\"a\", \"z\", \"m\"]))\n" +
				"print(any([nil, false, 0]))\n" +
				"print(all([true, 1, \"\", []]))\n" +
				"print(any([]))\n" +
				"print(all([]))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\n2.5\n6\n0\n6.5\n1\n3\n😊ba\n2.5\nz\ntrue\ntrue\nfalse\ntrue\n",
			}),
		},
		{
			name: "builtin_join",
			source: "print(join([\"a\", \"b\", \"c\"], \",\"))\n" +
				"print(\"[\" + join([], \"-\") + \"]\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "a,b,c\n[]\n",
			}),
		},
		{
			name:   "builtin_join_bad_element",
			source: "join([\"ok\", 1], \",\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "join() array elements must be STRING",
			}),
		},
		{
			name: "string_methods",
			source: "s = \"\\t hi \\n\"\n" +
				"print(\"[\" + s.strip() + \"]\")\n" +
				"print(\"[\" + \"already\".strip() + \"]\")\n" +
				"print(\"[\" + \"\".strip() + \"]\")\n" +
				"print(\"[\" + \"\\t\\n\".strip() + \"]\")\n" +
				"print(\"hELLO\".capitalize())\n" +
				"print(\"ñandú\".capitalize())\n" +
				"print(\"MiXeD\".uppercase())\n" +
				"print(\"MiXeD\".lowercase())\n" +
				"print(\"abc\".startswith(\"\"))\n" +
				"print(\"abc\".endswith(\"\"))\n" +
				"print(\"abc\".startswith(\"ab\"))\n" +
				"print(\"abc\".endswith(\"bc\"))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "[hi]\n[already]\n[]\n[]\nHello\nÑandú\nMIXED\nmixed\ntrue\ntrue\ntrue\ntrue\n",
			}),
		},
		{
			name: "string_repetition",
			source: "print(\"a\" * 3)\n" +
				"print(\"ab\" * 0)\n" +
				"print(\"🙂\" * 3)\n" +
				"print(3 * \"a\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "aaa\n\n🙂🙂🙂\naaa\n",
			}),
		},
		{
			name:   "string_repetition_negative",
			source: "\"a\" * -1\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "repeat count must be non-negative",
			}),
		},
		{
			name:   "string_repetition_non_int",
			source: "\"a\" * 2.5\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "repeat count must be INTEGER",
			}),
		},
		{
			name:   "string_method_startswith_wrong_type",
			source: "\"abc\".startswith(1)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "startswith() prefix must be STRING",
			}),
		},
		{
			name:   "string_method_slice_wrong_type",
			source: "\"abc\".slice(\"1\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "slice low must be INTEGER, got: STRING",
			}),
		},
		{
			name:   "string_method_slice_arity",
			source: "\"abc\".slice(1, 2, 3)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "slice() takes 0, 1, or 2 arguments",
			}),
		},
		{
			name:   "string_method_strip_arity",
			source: "\"abc\".strip(1)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "strip() takes 0 arguments",
			}),
		},
		{
			name: "number_format_method",
			source: "print((1.234).format(2))\n" +
				"print((1.2).format(4))\n" +
				"print((1).format(3))\n" +
				"print((-1.235).format(2))\n" +
				"print((12.9).format(0))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1.23\n1.2000\n1.000\n-1.24\n13\n",
			}),
		},
		{
			name:   "number_format_negative_decimals",
			source: "1.2.format(-1)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "format() decimals must be >= 0",
			}),
		},
		{
			name:   "number_format_wrong_type",
			source: "1.2.format(2.5)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "format() decimals must be INTEGER",
			}),
		},
		{
			name: "parity_formatting_program",
			source: "s = \"  mixED  \"\n" +
				"parts = [s.strip().capitalize(), (1.5).format(1), (2).format(0)]\n" +
				"print(join(parts, \"|\"))\n" +
				"print(\"hello\".startswith(\"he\"))\n" +
				"print(\"hello\".endswith(\"lo\"))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "Mixed|1.5|2\ntrue\ntrue\n",
			}),
		},
		{
			name: "parity_string_methods_all",
			source: "s = \"\\n\\t  Hi🙂  \\t\"\n" +
				"print(\"[\" + s.strip() + \"]\")\n" +
				"print(\"hELLO\".capitalize())\n" +
				"print(\"AbÇ🙂\".uppercase())\n" +
				"print(\"AbÇ🙂\".lowercase())\n" +
				"print(\"hello\".startswith(\"he\"))\n" +
				"print(\"hello\".endswith(\"lo\"))\n" +
				"print(\"café\".slice(1, 3))\n" +
				"print(\"café\".slice(-1))\n" +
				"print(\"abc\".slice(5))\n" +
				"print(\"abc\".slice())\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "[Hi🙂]\nHello\nABÇ🙂\nabç🙂\ntrue\ntrue\naf\né\n\nabc\n",
			}),
		},
		{
			name:   "builtin_max_empty",
			source: "max([])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "max() arg is an empty sequence",
			}),
		},
		{
			name:   "builtin_sum_non_numeric",
			source: "sum([1, \"a\"])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "sum() requires all elements to be NUMBER",
			}),
		},
		{
			name:   "builtin_reverse_wrong_type",
			source: "reverse(1)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "reverse() expects ARRAY or STRING",
			}),
		},
		{
			name:   "builtin_any_wrong_type",
			source: "any(1)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "any() expects ARRAY",
			}),
		},
		{
			name: "arrays_dicts_and_indexing",
			source: "a = [10, 20, 30]\n" +
				"print(a[0])\n" +
				"d = #{\"a\": 1, \"b\": 2}\n" +
				"print(d[\"b\"])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "10\n2\n",
			}),
		},
		{
			name: "dict_shorthand_basic",
			source: "name = \"Alice\"\n" +
				"age = 25\n" +
				"person = #{name, age}\n" +
				"print(person.name)\n" +
				"print(person.age)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "Alice\n25\n",
			}),
		},
		{
			name: "dict_shorthand_mixed_and_duplicate_last_wins",
			source: "name = \"Alice\"\n" +
				"person = #{name, \"name\": \"Bob\", \"role\": \"admin\"}\n" +
				"print(person.name)\n" +
				"print(person.role)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "Bob\nadmin\n",
			}),
		},
		{
			name:   "dict_shorthand_unknown_identifier",
			source: "person = #{missing}\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "unknown identifier: missing",
			}),
		},
		{
			name:   "assignment_expr_value",
			source: "print(x = 3)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\n",
			}),
		},
		{
			name: "assignment_expr_chaining",
			source: "a = 0\n" +
				"b = 0\n" +
				"a = b = 7\n" +
				"print(a)\n" +
				"print(b)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "7\n7\n",
			}),
		},
		{
			name: "walrus_defines_and_returns",
			source: "x = 1\n" +
				"y = (z := x + 2) + 10\n" +
				"print(z)\n" +
				"print(y)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\n13\n",
			}),
		},
		{
			name: "walrus_in_if_condition",
			source: "a = [1, 2, 3]\n" +
				"if ((n := len(a)) > 0) { print(n) }\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\n",
			}),
		},
		{
			name: "walrus_shadows_outer_scope",
			source: "x = 1\n" +
				"func f() {\n" +
				"  x := 2\n" +
				"  print(x)\n" +
				"}\n" +
				"f()\n" +
				"print(x)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "2\n1\n",
			}),
		},
		{
			name:   "walrus_redeclare_same_scope_error",
			source: "func f() { a := 1; a := 2 }\nf()\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "cannot redeclare \"a\" in this scope",
			}),
		},
		{
			name:   "walrus_redeclare_after_assign_error",
			source: "a = 1\na := 2\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "cannot redeclare \"a\" in this scope",
			}),
		},
		{
			name: "assignment_expr_compound",
			source: "x = 1\n" +
				"print(x += 2)\n" +
				"print(x)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\n3\n",
			}),
		},
		{
			name: "assignment_expr_index_member_order",
			source: "ticks = 0\n" +
				"func tick() { ticks = ticks + 1; return ticks }\n" +
				"func make() { ticks = ticks + 10; return #{\"x\": 1} }\n" +
				"a = [0, 0, 0]\n" +
				"print(a[tick()] += tick())\n" +
				"print(ticks)\n" +
				"print(a[1])\n" +
				"ticks = 0\n" +
				"print(make().x += tick())\n" +
				"print(ticks)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "2\n2\n2\n12\n11\n",
			}),
		},
		{
			name: "compound_assign_variables",
			source: "x = 1\n" +
				"x += 2\n" +
				"print(x)\n" +
				"x = 10\n" +
				"x /= 2\n" +
				"print(x)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\n5\n",
			}),
		},
		{
			name: "compound_assign_index_member",
			source: "a = [1, 2]\n" +
				"a[0] += 5\n" +
				"print(a[0])\n" +
				"d = #{\"x\": 1}\n" +
				"d.x += 2\n" +
				"print(d.x)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "6\n3\n",
			}),
		},
		{
			name: "compound_assign_eval_order",
			source: "i = 0\n" +
				"func idx() { i = i + 1; return 0 }\n" +
				"func rhs() { i = i + 10; return 1 }\n" +
				"a = [1]\n" +
				"a[idx()] += rhs()\n" +
				"print(i)\n" +
				"print(a[0])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "11\n2\n",
			}),
		},
		{
			name: "compound_assign_string_concat",
			source: "x = \"a\"\n" +
				"x += \"b\"\n" +
				"print(x)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "ab\n",
			}),
		},
		{
			name:   "compound_assign_type_error",
			source: "x = \"a\"\n" + "x += 1\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "type mismatch",
			}),
		},
		{
			name: "negative_indexing",
			source: "a = [1, 2, 3]\n" +
				"print(a[-1])\n" +
				"s = \"café\"\n" +
				"print(s[-1])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\né\n",
			}),
		},
		{
			name: "slicing_and_clamping",
			source: "s = \"café\"\n" +
				"print(s[1:3])\n" +
				"print(s[-10:10])\n" +
				"a = [1, 2, 3, 4]\n" +
				"print(a[1:3])\n" +
				"print(a[3:1])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "af\ncafé\n[2, 3]\n[]\n",
			}),
		},
		{
			name: "dict_missing_key_returns_nil",
			source: "d = #{\"a\": 1}\n" +
				"print(d[\"b\"])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "nil\n",
			}),
		},
		{
			name:   "dict_missing_member_errors",
			source: "d = #{\"a\": 1}\nprint(d.b)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "unknown member: b",
			}),
		},
		{
			name: "closure_captures_reads",
			source: "func makeAdder(x) {\n" +
				"  func add(y) { return x + y }\n" +
				"  return add\n" +
				"}\n" +
				"f = makeAdder(2)\n" +
				"print(f(3))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "5\n",
			}),
		},
		{
			name: "func_literal_basic_call",
			source: "f = func(x) { return x + 1 }\n" +
				"print(f(2))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\n",
			}),
		},
		{
			name: "call_spread_tuple",
			source: "func f(a, b, c, d) { print(a); print(b); print(c); print(d) }\n" +
				"t = (1, 2)\n" +
				"f(0, ...t, 3)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "0\n1\n2\n3\n",
			}),
		},
		{
			name: "call_spread_multiple",
			source: "func g(a, b, c, d) { print(a); print(b); print(c); print(d) }\n" +
				"t1 = (1, 2)\n" +
				"t2 = (3, 4)\n" +
				"g(...t1, ...t2)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1\n2\n3\n4\n",
			}),
		},
		{
			name:   "call_spread_empty_tuple",
			source: "func f() { print(\"ok\") }\n" + "f(...())\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "ok\n",
			}),
		},
		{
			name: "call_spread_nested_call",
			source: "func g() { return 1, 2 }\n" +
				"func f(a, b) { print(a); print(b) }\n" +
				"f(...(g()))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1\n2\n",
			}),
		},
		{
			name: "call_spread_array",
			source: "func f(a, b, c, d) { print(a); print(b); print(c); print(d) }\n" +
				"a = [1, 2]\n" +
				"f(0, ...a, 3)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "0\n1\n2\n3\n",
			}),
		},
		{
			name:   "call_spread_non_tuple_errors",
			source: "func f(a) { return a }\n" + "f(...1)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "cannot spread INTEGER in call arguments",
			}),
		},
		{
			name: "func_literal_closure_capture",
			source: "func makeAdder(x) {\n" +
				"  return func(y) { return x + y }\n" +
				"}\n" +
				"add2 = makeAdder(2)\n" +
				"print(add2(5))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "7\n",
			}),
		},
		{
			name: "closure_write_single_level",
			source: "func make() {\n" +
				"  x = 0\n" +
				"  return func() { x = x + 1; return x }\n" +
				"}\n" +
				"f = make()\n" +
				"print(f())\n" +
				"print(f())\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1\n2\n",
			}),
		},
		{
			name: "closure_write_two_level",
			source: "func outer() {\n" +
				"  x = 1\n" +
				"  func mid() {\n" +
				"    return func() { x = x + 2; return x }\n" +
				"  }\n" +
				"  return mid()\n" +
				"}\n" +
				"f = outer()\n" +
				"print(f())\n" +
				"print(f())\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\n5\n",
			}),
		},
		{
			name: "closure_write_compound_assign",
			source: "func make() {\n" +
				"  x = 1\n" +
				"  return func() { x += 2; return x }\n" +
				"}\n" +
				"f = make()\n" +
				"print(f())\n" +
				"print(f())\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\n5\n",
			}),
		},
		{
			name: "closure_write_isolated_instances",
			source: "func make() {\n" +
				"  x = 0\n" +
				"  return func() { x = x + 1; return x }\n" +
				"}\n" +
				"a = make()\n" +
				"b = make()\n" +
				"print(a())\n" +
				"print(b())\n" +
				"print(a())\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1\n1\n2\n",
			}),
		},
		{
			name:   "func_literal_immediate_invocation",
			source: "print((func(x) { return x * 2 })(21))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "42\n",
			}),
		},
		{
			name: "func_literal_in_structures",
			source: "a = [func(x) { return x + 1 }, func(x) { return x + 2 }]\n" +
				"print(a[0](10) + a[1](10))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "23\n",
			}),
		},
		{
			name: "try_catch_finally_ordering",
			source: "order = 0\n" +
				"try { order = order * 10 + 1; throw \"boom\" } catch (e) { order = order * 10 + 2 } finally { order = order * 10 + 3 }\n" +
				"print(order)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "123\n",
			}),
		},
		{
			name: "finally_always_runs",
			source: "order = 0\n" +
				"try { order = order * 10 + 1 } finally { order = order * 10 + 2 }\n" +
				"print(order)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "12\n",
			}),
		},
		{
			name: "defer_runs_on_return_and_throw",
			source: "order = 0\n" +
				"func add(n) { order = order * 10 + n }\n" +
				"func f() { defer add(1); defer add(2); return 0 }\n" +
				"func g() { defer add(3); throw \"boom\" }\n" +
				"f()\n" +
				"try { g() } catch (e) {}\n" +
				"print(order)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "213\n",
			}),
		},
		{
			name: "module_import_std_and_aliasing",
			source: "import \"std:math\" as math\n" +
				"from \"std:math\" import add as plus\n" +
				"print(math.add(2, 3))\n" +
				"print(plus(4, 5))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "5\n9\n",
			}),
		},
		{
			name: "module_exports_and_from_import",
			files: map[string]string{
				"mod.wll": "export x = 42\n" +
					"y = 1\n" +
					"export func add(a, b) { return a + b }\n",
			},
			source: "import \"./mod.wll\" as m\n" +
				"print(m.x)\n" +
				"print(m.add(1, 2))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "42\n3\n",
			}),
		},
		{
			name: "from_import_missing_export_errors",
			files: map[string]string{
				"mod2.wll": "x = 1\n",
			},
			source: "from \"./mod2.wll\" import x\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "missing export",
			}),
		},
		{
			name: "logical_short_circuit_throw",
			source: "func boom() { throw \"boom\" }\n" +
				"print(false and boom())\n" +
				"print(true or boom())\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "false\ntrue\n",
			}),
		},
		{
			name: "string_ops",
			source: "print(\"a\" + \"b\")\n" +
				"print(\"a\" == \"a\")\n" +
				"print(\"a\" != \"b\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "ab\ntrue\ntrue\n",
			}),
		},
		{
			name:   "string_compare_type_mismatch",
			source: "print(\"a\" == 1)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "type mismatch: STRING == INTEGER",
			}),
		},
		{
			name: "for_in_array_sum",
			source: "sum = 0\n" +
				"for x in range(4) { sum = sum + x }\n" +
				"print(sum)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "6\n",
			}),
		},
		{
			name: "for_in_nested",
			source: "sum = 0\n" +
				"for x in [1, 2] { for y in [10, 20] { sum = sum + x + y } }\n" +
				"print(sum)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "66\n",
			}),
		},
		{
			name: "for_in_dict_keys",
			source: "d = #{\"b\": 2, \"a\": 1}\n" +
				"for k in d { print(k) }\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "a\nb\n",
			}),
		},
		{
			name: "for_in_dict_destructure",
			source: "d = #{\"b\": 2, \"a\": 1}\n" +
				"for (k, v) in d { print(k); print(v) }\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "a\n1\nb\n2\n",
			}),
		},
		{
			name: "for_in_dict_destructure_discard",
			source: "d = #{\"b\": 2, \"a\": 1}\n" +
				"sum = 0\n" +
				"for (_, v) in d { sum = sum + v }\n" +
				"print(sum)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "3\n",
			}),
		},
		{
			name:   "for_in_destructure_non_dict",
			source: "for (k, v) in [1, 2] { print(k) }\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "for-in destructuring requires dict, got ARRAY",
			}),
		},
		{
			name: "tuple_literals_and_print",
			source: "t = (1, 2)\n" +
				"print(t)\n" +
				"print(())\n" +
				"print((1,))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "(1, 2)\n()\n(1,)\n",
			}),
		},
		{
			name: "multi_return_tuple_value",
			source: "func f() { return 1, 2 }\n" +
				"t = f()\n" +
				"print(t)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "(1, 2)\n",
			}),
		},
		{
			name: "destructure_assign_from_tuple_and_func",
			source: "func f() { return 3, 4 }\n" +
				"(a, b) = (1, 2)\n" +
				"(x, y) = f()\n" +
				"(p, _) = (5, 6)\n" +
				"print(a)\n" +
				"print(b)\n" +
				"print(x)\n" +
				"print(y)\n" +
				"print(p)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1\n2\n3\n4\n5\n",
			}),
		},
		{
			name:   "destructure_assign_arity_mismatch_short",
			source: "(a, b) = (1,)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "tuple arity mismatch",
			}),
		},
		{
			name:   "destructure_assign_arity_mismatch_long",
			source: "(a,) = (1, 2)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "tuple arity mismatch",
			}),
		},
		{
			name: "tuple_destructure_from_var",
			source: "x = (7, 8)\n" +
				"(a, b) = x\n" +
				"print(a)\n" +
				"print(b)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "7\n8\n",
			}),
		},
		{
			name: "single_return_still_works",
			source: "func g() { return 9 }\n" +
				"print(g())\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "9\n",
			}),
		},
		{
			name: "tuple_equality",
			source: "print((1, 2) == (1, 2))\n" +
				"print((1, 2) != (1, 3))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "true\ntrue\n",
			}),
		},
		{
			name:      "runtime_max_mem_error",
			source:    "s = \"hello\"\n",
			maxMemory: 10,
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "max memory exceeded (10 bytes)",
			}),
		},
		{
			name:      "runtime_max_mem_ok",
			source:    "print(\"ok\")\n",
			maxMemory: 1000,
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "ok\n",
			}),
		},
		{
			name: "collections_api_parity",
			source: "d = #{\"a\": 1, \"b\": 2}\n" +
				"print(d.count())\n" +
				"print(d.get(\"a\"))\n" +
				"print(d.get(\"c\"))\n" +
				"print(d.get(\"c\", 9))\n" +
				"print(d.pop(\"b\"))\n" +
				"print(d.count())\n" +
				"print(d.pop(\"missing\", 7))\n" +
				"print(d.count())\n" +
				"print(d.remove(\"a\"))\n" +
				"print(d.count())\n" +
				"\n" +
				"a = [1, 2, 1]\n" +
				"print(a.count(1))\n" +
				"print(a.pop())\n" +
				"print(a)\n" +
				"print(a.remove(1))\n" +
				"print(a)\n" +
				"\n" +
				"s = \" Hello \"\n" +
				"print(s.strip())\n" +
				"print(\"Hello\".startswith(\"He\"))\n" +
				"print(\"Hello\".endswith(\"lo\"))\n" +
				"print(\"École\".lowercase())\n" +
				"print(\"straße\".uppercase())\n" +
				"print(\"hELLo\".capitalize())\n" +
				"\n" +
				"func inc(x) { return x + 1 }\n" +
				"print(map(inc, [1, 2, 3]))\n" +
				"print(map(func(x) { return x * 2 }, [2, 3]))\n" +
				"print(mean([1, 2, 3]))\n" +
				"print(mean([1, 2]))\n" +
				"print(mean([1.0, 2, 3]))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "2\n1\nnil\n9\n2\n1\n7\n1\nnil\n0\n2\n1\n[1, 2]\ntrue\n[2]\nHello\ntrue\ntrue\nécole\nSTRAßE\nHello\n[2, 3, 4]\n[4, 6]\n2\n1.5\n2\n",
			}),
		},
		{
			name:   "dict_pop_missing_error",
			source: "d = #{\"a\": 1}\n" + "d.pop(\"b\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "key not found",
			}),
		},
		{
			name:   "dict_remove_missing_error",
			source: "d = #{\"a\": 1}\n" + "d.remove(\"b\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "key not found",
			}),
		},
		{
			name:   "dict_get_unhashable_key",
			source: "d = #{}\n" + "d.get([1], 0)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "unusable as dict key: ARRAY",
			}),
		},
		{
			name:   "array_pop_empty_error",
			source: "a = []\n" + "a.pop()\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "pop from empty array",
			}),
		},
		{
			name:   "array_remove_missing_false",
			source: "a = [1, 2]\n" + "print(a.remove(3))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "false\n",
			}),
		},
		{
			name:   "array_count_equality_error",
			source: "a = [1, \"x\"]\n" + "a.count(\"x\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "type mismatch",
			}),
		},
		{
			name:   "array_remove_equality_error",
			source: "a = [1, \"x\"]\n" + "a.remove(\"x\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "type mismatch",
			}),
		},
		{
			name: "array_method_arg_errors",
			source: "a = [1]\n" +
				"a.pop(1)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "pop() takes 0 arguments, got 1",
			}),
		},
		{
			name: "dict_method_arg_errors",
			source: "d = #{}\n" +
				"d.get()\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "get() takes 1 or 2 arguments, got 0",
			}),
		},
		{
			name: "dict_method_receiver_error",
			source: "x = 1\n" +
				"x.get(\"a\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "get() receiver must be DICT",
			}),
		},
		{
			name: "builtin_collection_aliases",
			source: "a = [1, 2, 1]\n" +
				"print(count(a, 1))\n" +
				"print(remove(a, 2))\n" +
				"print(a)\n" +
				"print(pop(a))\n" +
				"print(a)\n" +
				"d = #{\"a\": 1}\n" +
				"print(get(d, \"a\"))\n" +
				"print(get(d, \"b\", 9))\n" +
				"print(pop(d, \"a\"))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "2\ntrue\n[1, 1]\n1\n[1]\n1\n9\n1\n",
			}),
		},
		{
			name: "dict_update_assign",
			source: "d = #{\"a\": 1}\n" +
				"other = #{\"a\": 2, \"b\": 3}\n" +
				"d |= other\n" +
				"print(d)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "#{\"a\": 2, \"b\": 3}\n",
			}),
		},
		{
			name: "dict_update_index_member",
			source: "w = #{\"d\": #{\"a\": 1}}\n" +
				"w[\"d\"] |= #{\"b\": 2}\n" +
				"print(w)\n" +
				"m = #{\"d\": #{\"a\": 1}}\n" +
				"m.d |= #{\"a\": 2}\n" +
				"print(m)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "#{\"d\": #{\"a\": 1, \"b\": 2}}\n#{\"d\": #{\"a\": 2}}\n",
			}),
		},
		{
			name: "dict_update_errors",
			source: "a = []\n" +
				"a |= #{}\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "|= left operand must be dict",
			}),
		},
		{
			name: "dict_update_rhs_error",
			source: "d = #{}\n" +
				"d |= 1\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "|= right operand must be dict",
			}),
		},
		{
			name:   "map_propagates_error",
			source: "map(func(x) { return x + \"a\" }, [1])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "type mismatch",
			}),
		},
		{
			name:   "mean_empty_error",
			source: "mean([])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "mean() arg is an empty sequence",
			}),
		},
		{
			name:   "mean_non_numeric_error",
			source: "mean([1, \"a\"])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "mean() requires all elements to be NUMBER",
			}),
		},
		{
			name:   "list_comprehension_basic",
			source: "print([x + 1 for x in [1, 2, 3]])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "[2, 3, 4]\n",
			}),
		},
		{
			name:   "list_comprehension_filter",
			source: "print([x for x in [1, 2, 3, 4] if x % 2 == 0])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "[2, 4]\n",
			}),
		},
		{
			name:   "list_comprehension_cond_expr",
			source: "print([(x if x > 1 else 0) for x in [0, 1, 2]])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "[0, 0, 2]\n",
			}),
		},
		{
			name:   "list_comprehension_dict_order",
			source: "d = #{\"b\": 2, \"a\": 1}\nprint([k for k in d])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "[a, b]\n",
			}),
		},
		{
			name:   "list_comprehension_string",
			source: "print([c for c in \"café\"])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "[c, a, f, é]\n",
			}),
		},
		{
			name:   "list_comprehension_scoping",
			source: "i = 7\n[i for i in [1, 2, 3]]\nprint(i)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "7\n",
			}),
		},
		{
			name:   "list_comprehension_non_iterable_error",
			source: "print([x for x in 3])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "cannot iterate INTEGER in comprehension",
			}),
		},
		{
			name: "slice_step_array",
			source: "print([1, 2, 3, 4, 5][::-1])\n" +
				"print([1, 2, 3, 4, 5][::2])\n" +
				"print([0, 1, 2, 3, 4, 5][1:5:2])\n" +
				"print([0, 1, 2, 3, 4, 5][5:1:-1])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "[5, 4, 3, 2, 1]\n[1, 3, 5]\n[1, 3]\n[5, 4, 3, 2]\n",
			}),
		},
		{
			name:   "slice_step_string",
			source: "print(\"café\"[::-1])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "éfac\n",
			}),
		},
		{
			name:   "slice_step_zero_error",
			source: "print([1, 2][::0])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "slice step cannot be 0",
			}),
		},
		{
			name:   "slice_step_non_int_error",
			source: "print([1, 2][::1.5])\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "slice step must be INTEGER, got: FLOAT",
			}),
		},
		{
			name:   "destructure_star_basic",
			source: "(a, *_, b) = (1, 2, 3, 4)\nprint(a)\nprint(b)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1\n4\n",
			}),
		},
		{
			name:   "destructure_star_empty_mid",
			source: "(a, *mid, b) = (1, 2)\nprint(mid)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "[]\n",
			}),
		},
		{
			name:   "destructure_star_only",
			source: "(*rest) = (1, 2, 3)\nprint(rest)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "[1, 2, 3]\n",
			}),
		},
		{
			name:   "destructure_star_array_rhs",
			source: "(a, *mid, b) = [1, 2, 3, 4]\nprint(mid)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "[2, 3]\n",
			}),
		},
		{
			name:   "destructure_star_too_short",
			source: "(a, *mid, b) = (1,)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "not enough values to unpack (expected at least 2, got 1)",
			}),
		},
		{
			name:   "destructure_star_non_sequence",
			source: "(a, *mid) = 3\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "cannot unpack non-sequence",
			}),
		},
		{
			name:   "destructure_star_parse_error",
			source: "(*a, *b) = (1, 2)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name: "template_untagged_basic",
			source: "name = \"welle\"\n" +
				"print(t\"hello ${name}!\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "hello welle!\n",
			}),
		},
		{
			name: "template_untagged_eval_order",
			source: "x = 0\n" +
				"func next() {\n" +
				"  x = x + 1\n" +
				"  return x\n" +
				"}\n" +
				"print(t\"${next()} ${next()}\")\n" +
				"print(x)\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1 2\n2\n",
			}),
		},
		{
			name: "template_tagged_basic",
			source: "func joiner(parts, a, b) {\n" +
				"  return parts[0] + str(a) + parts[1] + str(b) + parts[2]\n" +
				"}\n" +
				"print(joiner t\"hello ${1} and ${2}!\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "hello 1 and 2!\n",
			}),
		},
		{
			name:   "template_unterminated_error",
			source: "print(t\"hello ${name}\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name:   "template_malformed_interpolation_error",
			source: "name = \"x\"\nprint(t\"hi ${name\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrCode: "WP0001",
			}),
		},
		{
			name:   "template_tag_not_callable_error",
			source: "tag = 1\nprint(tag t\"x\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "attempted to call non-function: INTEGER",
			}),
		},
		{
			name:   "template_interpolation_runtime_error",
			source: "print(t\"${1 / 0}\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "division by zero",
			}),
		},
		{
			name: "formatting_group_digits",
			source: "print(group_digits(\"14_310_023\"))\n" +
				"print(group_digits(-14310023))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "14,310,023\n-14,310,023\n",
			}),
		},
		{
			name: "formatting_float_percent",
			source: "print(format_float(1.234, 2))\n" +
				"print(format_float(1, 3))\n" +
				"print(format_percent(0.1234, 1))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "1.23\n1.000\n12.3%\n",
			}),
		},
		{
			name:   "formatting_errors",
			source: "print(group_digits(\"12a\"))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "group_digits() string contains non-digit characters",
			}),
		},
		{
			name:   "formatting_group_error",
			source: "print(group_digits(123, \",\", 0))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "group_digits() group must be > 0",
			}),
		},
		{
			name:   "formatting_decimals_error",
			source: "print(format_float(1.2, -1))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "format_float() decimals must be >= 0",
			}),
		},
		{
			name:   "formatting_type_error",
			source: "print(format_percent(\"x\", 1))\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				ErrContains: "format_percent() x must be NUMBER",
			}),
		},
		{
			name: "identity_is_semantics",
			source: "a = [1]\n" +
				"b = a\n" +
				"c = [1]\n" +
				"func make() { return func() { return 1 } }\n" +
				"f = make()\n" +
				"g = f\n" +
				"h = make()\n" +
				"print(nil is nil)\n" +
				"print(true is true)\n" +
				"print(false is true)\n" +
				"print(256 is 256)\n" +
				"print(257 is 257)\n" +
				"print(1 is 1.0)\n" +
				"print(\"ab\" is \"ab\")\n" +
				"print(a is b)\n" +
				"print(a is c)\n" +
				"print(f is g)\n" +
				"print(f is h)\n" +
				"print(1 is \"1\")\n",
			expect: spectest.ExpectBoth(spectest.Expectation{
				Stdout: "true\ntrue\nfalse\ntrue\ntrue\nfalse\ntrue\ntrue\nfalse\ntrue\nfalse\nfalse\n",
			}),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for mode, exp := range tc.expect {
				mode := mode
				exp := exp
				t.Run(string(mode), func(t *testing.T) {
					res := spectest.Run(t, spectest.Options{
						Mode:      mode,
						Source:    tc.source,
						Files:     tc.files,
						Entry:     tc.entry,
						MaxMemory: tc.maxMemory,
					})
					spectest.Assert(t, res, exp)
				})
			}
		})
	}
}

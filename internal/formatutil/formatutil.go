package formatutil

import (
	"fmt"
	"strconv"
	"strings"
)

func GroupDigitsFromString(raw, sep string, group int) (string, error) {
	if group <= 0 {
		return "", fmt.Errorf("group_digits() group must be > 0")
	}

	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if ch == '_' {
			continue
		}
		if ch < '0' || ch > '9' {
			return "", fmt.Errorf("group_digits() string contains non-digit characters")
		}
		b.WriteByte(ch)
	}
	digits := b.String()
	if digits == "" {
		return "", fmt.Errorf("group_digits() string contains non-digit characters")
	}
	return groupDigits(digits, false, sep, group), nil
}

func GroupDigitsFromInt(v int64, sep string, group int) (string, error) {
	if group <= 0 {
		return "", fmt.Errorf("group_digits() group must be > 0")
	}
	neg := v < 0
	digits := strconv.FormatInt(v, 10)
	if neg {
		digits = digits[1:]
	}
	return groupDigits(digits, neg, sep, group), nil
}

func FormatFloat(x float64, decimals int) (string, error) {
	if decimals < 0 {
		return "", fmt.Errorf("format_float() decimals must be >= 0")
	}
	return strconv.FormatFloat(x, 'f', decimals, 64), nil
}

func FormatPercent(x float64, decimals int) (string, error) {
	f, err := FormatFloat(x*100, decimals)
	if err != nil {
		return "", err
	}
	return f + "%", nil
}

func groupDigits(digits string, neg bool, sep string, group int) string {
	if len(digits) <= group {
		if neg {
			return "-" + digits
		}
		return digits
	}

	var out strings.Builder
	if neg {
		out.WriteByte('-')
	}
	first := len(digits) % group
	if first == 0 {
		first = group
	}
	out.WriteString(digits[:first])
	for i := first; i < len(digits); i += group {
		out.WriteString(sep)
		out.WriteString(digits[i : i+group])
	}
	return out.String()
}

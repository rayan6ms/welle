package numlit

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type IntLiteral struct {
	Base       int
	Digits     string
	Normalized string
}

type FloatLiteral struct {
	Normalized  string
	HasExponent bool
}

func NormalizeIntLiteral(lit string) (IntLiteral, error) {
	base := 10
	digits := lit
	if len(lit) >= 2 && lit[0] == '0' {
		switch lit[1] {
		case 'x', 'X':
			base = 16
			digits = lit[2:]
		case 'b', 'B':
			base = 2
			digits = lit[2:]
		case 'o', 'O':
			base = 8
			digits = lit[2:]
		}
	}
	if err := validateDigits(digits, base); err != nil {
		return IntLiteral{}, fmt.Errorf("invalid integer literal: %w", err)
	}
	normalized := stripUnderscores(digits)
	return IntLiteral{
		Base:       base,
		Digits:     digits,
		Normalized: normalized,
	}, nil
}

func ParseIntLiteral(lit string) (int64, error) {
	info, err := NormalizeIntLiteral(lit)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseInt(info.Normalized, info.Base, 64)
	if err != nil {
		var numErr *strconv.NumError
		if errors.As(err, &numErr) && numErr.Err == strconv.ErrRange {
			return 0, fmt.Errorf("integer literal out of range")
		}
		return 0, fmt.Errorf("invalid integer literal")
	}
	return v, nil
}

func NormalizeFloatLiteral(lit string) (FloatLiteral, error) {
	if len(lit) >= 2 && lit[0] == '0' {
		switch lit[1] {
		case 'x', 'X', 'b', 'B', 'o', 'O':
			return FloatLiteral{}, fmt.Errorf("float literal cannot use base prefix")
		}
	}

	mantissa := lit
	expPart := ""
	hasExp := false
	if idx := strings.IndexAny(lit, "eE"); idx >= 0 {
		hasExp = true
		mantissa = lit[:idx]
		expPart = lit[idx+1:]
		if expPart == "" {
			return FloatLiteral{}, fmt.Errorf("exponent requires digits")
		}
	}

	mantissaNorm, err := normalizeMantissa(mantissa)
	if err != nil {
		return FloatLiteral{}, err
	}

	expNorm := ""
	if hasExp {
		sign := ""
		if expPart[0] == '+' || expPart[0] == '-' {
			sign = expPart[:1]
			expPart = expPart[1:]
		}
		if expPart == "" {
			return FloatLiteral{}, fmt.Errorf("exponent requires digits")
		}
		if err := validateDigits(expPart, 10); err != nil {
			return FloatLiteral{}, fmt.Errorf("invalid float literal: %w", err)
		}
		expNorm = "e" + sign + stripUnderscores(expPart)
	}

	return FloatLiteral{
		Normalized:  mantissaNorm + expNorm,
		HasExponent: hasExp,
	}, nil
}

func ParseFloatLiteral(lit string) (float64, error) {
	info, err := NormalizeFloatLiteral(lit)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseFloat(info.Normalized, 64)
	if err != nil {
		var numErr *strconv.NumError
		if errors.As(err, &numErr) && numErr.Err == strconv.ErrRange {
			return 0, fmt.Errorf("float literal out of range")
		}
		return 0, fmt.Errorf("invalid float literal")
	}
	return v, nil
}

func normalizeMantissa(mantissa string) (string, error) {
	if mantissa == "" {
		return "", fmt.Errorf("float literal requires digits")
	}
	if strings.Contains(mantissa, ".") {
		parts := strings.SplitN(mantissa, ".", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", fmt.Errorf("float literal requires digits on both sides of decimal point")
		}
		if err := validateDigits(parts[0], 10); err != nil {
			return "", fmt.Errorf("invalid float literal: %w", err)
		}
		if err := validateDigits(parts[1], 10); err != nil {
			return "", fmt.Errorf("invalid float literal: %w", err)
		}
		return stripUnderscores(parts[0]) + "." + stripUnderscores(parts[1]), nil
	}

	if err := validateDigits(mantissa, 10); err != nil {
		return "", fmt.Errorf("invalid float literal: %w", err)
	}
	return stripUnderscores(mantissa), nil
}

func validateDigits(s string, base int) error {
	if s == "" {
		return fmt.Errorf("digits required")
	}
	prevUnderscore := false
	seenDigit := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '_' {
			if !seenDigit || prevUnderscore {
				return fmt.Errorf("underscores must separate digits")
			}
			prevUnderscore = true
			continue
		}
		if !isDigitForBase(ch, base) {
			return fmt.Errorf("invalid digit %q for base %d", ch, base)
		}
		seenDigit = true
		prevUnderscore = false
	}
	if !seenDigit {
		return fmt.Errorf("digits required")
	}
	if prevUnderscore {
		return fmt.Errorf("underscores must separate digits")
	}
	return nil
}

func isDigitForBase(ch byte, base int) bool {
	switch {
	case ch >= '0' && ch <= '9':
		return int(ch-'0') < base
	case base == 16 && ch >= 'a' && ch <= 'f':
		return true
	case base == 16 && ch >= 'A' && ch <= 'F':
		return true
	default:
		return false
	}
}

func stripUnderscores(s string) string {
	if strings.IndexByte(s, '_') == -1 {
		return s
	}
	return strings.ReplaceAll(s, "_", "")
}

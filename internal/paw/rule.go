package paw

import (
	"bytes"
	"fmt"
)

const (
	lowercase = "abcdefghijklmnopqrstuvwxyz"
	uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits    = "0123456789"
	symbols   = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
)

// Format represents the format for a rule
type Format int

const (
	// LowercaseFormat specify a format with all lowercase chars
	LowercaseFormat Format = 1 << iota
	// LowercaseFormat specify a format with all uppercase chars
	UppercaseFormat
	// DigitsFormat specify a format with all digits chars
	DigitsFormat
	// DigitsFormat specify a format with all symbols chars
	SymbolsFormat
)

// Rule defines the policy for password generation
type Rule struct {
	Length int
	Tpl    []byte
	Filter []byte
}

// NewRule defines a policy for password generation specifying the
// lenght and the desired format
func NewRule(length int, format Format) (*Rule, error) {
	r := &Rule{
		Length: length,
	}
	if (format & LowercaseFormat) != 0 {
		r.Tpl = append(r.Tpl, lowercase...)
	}
	if (format & UppercaseFormat) != 0 {
		r.Tpl = append(r.Tpl, uppercase...)
	}
	if (format & DigitsFormat) != 0 {
		r.Tpl = append(r.Tpl, digits...)
	}
	if (format & SymbolsFormat) != 0 {
		r.Tpl = append(r.Tpl, symbols...)
	}

	return r, nil
}

// WithFilter filters characters from the password
func (r *Rule) WithFilter(filter []byte) {
	if len(filter) == 0 {
		return
	}
	r.Filter = make([]byte, len(filter))
	copy(r.Filter, filter)
}

// Len returns the desired password length
func (r *Rule) Len() int {
	return r.Length
}

// Encode encodes the password policy as a byte template
func (r *Rule) Template() (string, error) {
	if len(r.Filter) == 0 {
		return string(r.Tpl), nil
	}

	var filtered bytes.Buffer
	for _, b := range r.Tpl {
		if pos := bytes.IndexByte(r.Filter, b); pos != -1 {
			fmt.Printf("found %s\n", string(b))
			continue
		}
		err := filtered.WriteByte(b)
		if err != nil {
			return "", err
		}
	}

	return filtered.String(), nil
}

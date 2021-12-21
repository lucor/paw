package paw

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRule(t *testing.T) {
	type args struct {
		Length  int
		formats Format
		filter  []byte
	}
	tests := []struct {
		name         string
		args         args
		wantTemplate string
	}{
		{
			name: "digits",
			args: args{
				Length:  9,
				formats: DigitsFormat,
			},
			wantTemplate: "0123456789",
		},
		{
			name: "digits with filtered template",
			args: args{
				Length:  9,
				formats: DigitsFormat,
				filter:  []byte{'5'},
			},
			wantTemplate: "012346789",
		},
		{
			name: "digits with filter without effect",
			args: args{
				Length:  9,
				formats: DigitsFormat,
				filter:  []byte{'a'},
			},
			wantTemplate: "0123456789",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := NewRule(tt.args.Length, tt.args.formats)
			assert.Nil(t, err)
			rule.WithFilter(tt.args.filter)
			template, err := rule.Template()
			assert.Nil(t, err)
			assert.Equal(t, tt.wantTemplate, template)

			assert.Equal(t, tt.args.Length, rule.Len())
		})
	}
}

func TestRuleEncodeRoundTrip(t *testing.T) {
	type args struct {
		Length  int
		formats Format
		filter  []byte
	}
	tests := []struct {
		name string
		args args
		want *Rule
	}{
		{
			name: "digits",
			args: args{
				Length:  9,
				formats: DigitsFormat,
			},
			want: &Rule{
				Length: 9,
				Tpl:    []byte(digits),
			},
		},
		{
			name: "digits with filtered template",
			args: args{
				Length:  9,
				formats: DigitsFormat,
				filter:  []byte{'5'},
			},
			want: &Rule{
				Length: 9,
				Tpl:    []byte(digits),
				Filter: []byte{'5'},
			},
		},
		{
			name: "digits with filter without effect",
			args: args{
				Length:  64,
				formats: LowercaseFormat,
				filter:  []byte{'A'},
			},
			want: &Rule{
				Length: 64,
				Tpl:    []byte(lowercase),
				Filter: []byte{'A'},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := NewRule(tt.args.Length, tt.args.formats)
			assert.Nil(t, err)
			rule.WithFilter(tt.args.filter)
			assert.Equal(t, tt.want, rule)

			var buf bytes.Buffer
			err = json.NewEncoder(&buf).Encode(rule)
			assert.Nil(t, err)

			var ruleDec *Rule
			err = json.NewDecoder(&buf).Decode(&ruleDec)
			assert.Nil(t, err)
			assert.Equal(t, rule, ruleDec)
		})
	}
}

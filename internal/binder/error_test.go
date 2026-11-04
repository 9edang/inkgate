package binder

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorSourceSyntaxError(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		expected string
	}{
		{
			name:     "Inline",
			src:      "local p = person.new('Steeve');lokal t = 'fail'",
			expected: "Line 1: 't': parse error",
		},
		{
			name:     "Parsing error",
			src:      "local p = person.new('Steeve')\nlokal t = 'fail'",
			expected: "Line 2: 't': parse error",
		},
		{
			name:     "Bad token",
			src:      "local p = person.new('Steeve')\nlocal t & 'fail'",
			expected: "Line 2: '&': Invalid token",
		},
		{
			name:     "Unterminated string",
			src:      "local p = person.new('Steeve)\nlocal t = 'okay'",
			expected: "Line 2: 'Steeve)': unterminated string",
		},
		{
			name:     "End of file",
			src:      "local p = person.new('Steeve')\nprint(p:email()",
			expected: "Line 0: syntax error at EOF",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := getBinder()

			if err := b.DoString(test.src); err != nil {
				switch err.(type) {
				case *Error:
					e := err.(*Error)
					if e.Error() != test.expected {
						t.Errorf("Error message does not match, expected=\"%s\" got=\"%s\"", test.expected, e.Error())
					}
				default:
					t.Error("Must return error", err)
				}
			}
		})
	}
}

func TestErrorSourceSmall_Func(t *testing.T) {
	b := getBinder()

	if err := b.DoString(`
local p = person.new('Steeve')
print(p:email())
    `); err != nil {
		switch err.(type) {
		case *Error:
			e := err.(*Error)
			s := strings.Split(e.Source(), "\n")
			l := len(s)

			if l != 4 {
				t.Errorf("Source must have %d lines, received %d", 4, l)
			}

			break
		default:
			t.Error("Must return error", err)
		}
	}
}

func TestErrorSourceBig_Func(t *testing.T) {
	b := getBinder()

	if err := b.DoString(`









local p = person.new('Steeve')
print(p:email())










    `); err != nil {
		switch err.(type) {
		case *Error:
			e := err.(*Error)
			s := strings.Split(e.Source(), "\n")
			l := len(s)

			need := errorLinesBefore + errorLinesAfter + 1
			if l != need {
				t.Errorf("Source must have %d lines, received %d", need, l)
			}

			break
		default:
			t.Error("Must return error", err)
		}
	}
}

func getBinder() *Binder {
	b := New()
	tbl := b.Table("person")
	tbl.Static("new", func(c *Context) error {
		if c.Top() == 0 {
			return errors.New("need arguments")
		}
		n := c.Arg(1).String()

		c.Push().Data(&Person{n}, "person")
		return nil
	})

	tbl.Dynamic("name", func(c *Context) error {
		p, ok := c.Arg(1).Data().(*Person)
		if !ok {
			return errors.New("person expected")
		}

		if c.Top() == 1 {
			c.Push().String(p.Name)
		} else {
			p.Name = c.Arg(2).String()
		}

		return nil
	})

	return b
}

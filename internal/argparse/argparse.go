// Package argparse provides GNU-style command line argument parsing.
//
// Neither pflag nor the standard library flag package is used because this tool's
// -p/--project needs an optional value: no value means the current directory, a
// value means an explicit project name. Neither library supports such a flag.
package argparse

import (
	"fmt"
	"strings"
)

// Kind decides how a flag consumes the tokens that follow it.
type Kind int

const (
	// Bool never consumes the next token.
	Bool Kind = iota
	// String requires a value; a missing value is an error.
	String
	// StringSlice behaves like String but may repeat, accumulating into a list.
	StringSlice
	// OptionalString consumes the next token if it exists and does not start with "-"; otherwise it counts as given without a value.
	OptionalString
)

// Spec describes one flag. Short omits the leading "-" and may be empty.
type Spec struct {
	Long  string
	Short string
	Kind  Kind
	Usage string
}

// Set is one subcommand's collection of flag definitions.
type Set struct{ specs []Spec }

// New builds a Set.
func New(specs ...Spec) *Set { return &Set{specs: specs} }

type value struct {
	set      bool
	hasValue bool
	str      string
	strs     []string
}

// Result holds the outcome of one parse.
type Result struct {
	vals map[string]*value
	args []string
}

// Changed reports whether the flag appeared on the command line.
func (r *Result) Changed(long string) bool {
	v, ok := r.vals[long]
	return ok && v.set
}

// Bool reports whether a boolean flag was given.
func (r *Result) Bool(long string) bool { return r.Changed(long) }

// String returns a string flag's value, or "" when not given (use Changed to tell them apart).
func (r *Result) String(long string) string {
	if v, ok := r.vals[long]; ok {
		return v.str
	}
	return ""
}

// Strings returns the values a repeatable flag accumulated.
func (r *Result) Strings(long string) []string {
	if v, ok := r.vals[long]; ok {
		return v.strs
	}
	return nil
}

// Optional returns an optional-value flag's value and whether one was given.
// Three states: Changed false means absent; Changed true with hasValue false means given without a value.
func (r *Result) Optional(long string) (string, bool) {
	v, ok := r.vals[long]
	if !ok || !v.set {
		return "", false
	}
	return v.str, v.hasValue
}

// Args returns the positional arguments.
func (r *Result) Args() []string { return r.args }

// Usage renders the flag help text.
func (s *Set) Usage() string {
	var b strings.Builder
	for _, sp := range s.specs {
		name := "    --" + sp.Long
		if sp.Short != "" {
			name = "-" + sp.Short + ", --" + sp.Long
		}
		fmt.Fprintf(&b, "  %-20s %s\n", name, sp.Usage)
	}
	return b.String()
}

func (s *Set) find(name string, long bool) (Spec, bool) {
	for _, sp := range s.specs {
		if long && sp.Long == name {
			return sp, true
		}
		if !long && sp.Short != "" && sp.Short == name {
			return sp, true
		}
	}
	return Spec{}, false
}

func cut(s string) (name, val string, has bool) {
	if k := strings.IndexByte(s, '='); k >= 0 {
		return s[:k], s[k+1:], true
	}
	return s, "", false
}

// Parse parses args, which exclude the program name and the subcommand name.
func (s *Set) Parse(args []string) (*Result, error) {
	r := &Result{vals: map[string]*value{}}
	for _, sp := range s.specs {
		r.vals[sp.Long] = &value{}
	}
	for i := 0; i < len(args); {
		a := args[i]
		switch {
		case a == "--":
			r.args = append(r.args, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "--"):
			name, inline, hasInline := cut(a[2:])
			sp, ok := s.find(name, true)
			if !ok {
				return nil, fmt.Errorf("unknown flag --%s", name)
			}
			used, err := s.assign(r, sp, inline, hasInline, args, i)
			if err != nil {
				return nil, err
			}
			i += used
		case len(a) > 1 && strings.HasPrefix(a, "-"):
			name, inline, hasInline := cut(a[1:])
			sp, ok := s.find(name, false)
			if !ok {
				return nil, fmt.Errorf("unknown flag -%s", name)
			}
			used, err := s.assign(r, sp, inline, hasInline, args, i)
			if err != nil {
				return nil, err
			}
			i += used
		default:
			r.args = append(r.args, a)
			i++
		}
	}
	return r, nil
}

// assign applies one flag and reports how many tokens it consumed.
func (s *Set) assign(r *Result, sp Spec, inline string, hasInline bool, args []string, i int) (int, error) {
	v := r.vals[sp.Long]
	v.set = true
	switch sp.Kind {
	case Bool:
		if hasInline {
			return 0, fmt.Errorf("flag --%s takes no value", sp.Long)
		}
		return 1, nil
	case String, StringSlice:
		val, used := inline, 1
		if !hasInline {
			if i+1 >= len(args) {
				return 0, fmt.Errorf("flag --%s needs a value", sp.Long)
			}
			val, used = args[i+1], 2
		}
		if sp.Kind == String {
			v.str, v.hasValue = val, true
		} else {
			v.strs = append(v.strs, val)
		}
		return used, nil
	case OptionalString:
		if hasInline {
			v.str, v.hasValue = inline, true
			return 1, nil
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			v.str, v.hasValue = args[i+1], true
			return 2, nil
		}
		return 1, nil
	}
	return 1, nil
}

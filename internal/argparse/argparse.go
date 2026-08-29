// Package argparse 提供 GNU 風格的命令列參數解析。
//
// 不使用 pflag 或標準庫 flag 的原因：本工具的 -p/--project 需要「可選值」——
// 不給值代表當前目錄，給值代表指定專案名。兩個現成庫都不支援這種 flag。
package argparse

import (
	"fmt"
	"strings"
)

// Kind 決定一個 flag 如何吃掉後續的 token。
type Kind int

const (
	// Bool 永不吃下一個 token。
	Bool Kind = iota
	// String 必須有值，缺值視為錯誤。
	String
	// StringSlice 同 String，但可重複出現、累積成清單。
	StringSlice
	// OptionalString 的下一個 token 若存在且不以 "-" 開頭就吃掉，否則視為「有給但無值」。
	OptionalString
)

// Spec 描述一個 flag。Short 不含前導的 "-"，可為空字串。
type Spec struct {
	Long  string
	Short string
	Kind  Kind
	Usage string
}

// Set 是一個子指令的 flag 定義集合。
type Set struct{ specs []Spec }

// New 建立 Set。
func New(specs ...Spec) *Set { return &Set{specs: specs} }

type value struct {
	set      bool
	hasValue bool
	str      string
	strs     []string
}

// Result 是一次解析的結果。
type Result struct {
	vals map[string]*value
	args []string
}

// Changed 回報該 flag 有沒有出現在命令列上。
func (r *Result) Changed(long string) bool {
	v, ok := r.vals[long]
	return ok && v.set
}

// Bool 回傳布林 flag 是否出現。
func (r *Result) Bool(long string) bool { return r.Changed(long) }

// String 回傳字串 flag 的值；沒給時為空字串（用 Changed 區分）。
func (r *Result) String(long string) string {
	if v, ok := r.vals[long]; ok {
		return v.str
	}
	return ""
}

// Strings 回傳可重複 flag 累積的值。
func (r *Result) Strings(long string) []string {
	if v, ok := r.vals[long]; ok {
		return v.strs
	}
	return nil
}

// Optional 回傳可選值 flag 的值與「有沒有帶值」。
// 三態判讀：Changed 為 false 是沒給；Changed 為 true 但 hasValue 為 false 是給了但無值。
func (r *Result) Optional(long string) (string, bool) {
	v, ok := r.vals[long]
	if !ok || !v.set {
		return "", false
	}
	return v.str, v.hasValue
}

// Args 回傳位置參數。
func (r *Result) Args() []string { return r.args }

// Usage 產生 flag 說明文字。
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

// Parse 解析 args（不含程式名與子指令名）。
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
				return nil, fmt.Errorf("未知的 flag：--%s", name)
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
				return nil, fmt.Errorf("未知的 flag：-%s", name)
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

// assign 套用一個 flag，回傳吃掉幾個 token。
func (s *Set) assign(r *Result, sp Spec, inline string, hasInline bool, args []string, i int) (int, error) {
	v := r.vals[sp.Long]
	v.set = true
	switch sp.Kind {
	case Bool:
		if hasInline {
			return 0, fmt.Errorf("flag --%s 不接受值", sp.Long)
		}
		return 1, nil
	case String, StringSlice:
		val, used := inline, 1
		if !hasInline {
			if i+1 >= len(args) {
				return 0, fmt.Errorf("flag --%s 需要一個值", sp.Long)
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

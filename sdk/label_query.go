package sdk

import (
	"fmt"
	"strings"
)

type LabelExpr interface {
	Eval(has func(label string) bool) bool
}

type EqExpr struct{ Label, Value string }

func (e EqExpr) Eval(has func(label string) bool) bool {
	return has(e.Label + "=" + e.Value)
}

type InExpr struct {
	Label  string
	Values []string
}

func (e InExpr) Eval(has func(label string) bool) bool {
	for _, v := range e.Values {
		if has(e.Label + "=" + v) {
			return true
		}
	}
	return false
}

type HasExpr struct{ Label string }

func (e HasExpr) Eval(has func(label string) bool) bool {
	return has(e.Label)
}

type NotExpr struct{ Label string }

func (e NotExpr) Eval(has func(label string) bool) bool {
	return !has(e.Label)
}

type Query struct {
	AND []LabelExpr
	OR  [][]LabelExpr
}

func ParseQuery(s string) (Query, error) {
	q := Query{}
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return q, nil
	}
	if strings.Contains(s, "|") {
		parts := strings.Split(s, "|")
		q.OR = make([][]LabelExpr, 0, len(parts))
		for _, p := range parts {
			exprs, err := parseAND(p)
			if err != nil {
				return Query{}, err
			}
			q.OR = append(q.OR, exprs)
		}
		return q, nil
	}
	exprs, err := parseAND(s)
	if err != nil {
		return Query{}, err
	}
	q.AND = exprs
	return q, nil
}

func parseAND(part string) ([]LabelExpr, error) {
	part = strings.ToLower(strings.TrimSpace(part))
	if part == "" {
		return nil, nil
	}
	toks := make([]string, 0)
	var cur strings.Builder
	depth := 0
	for _, r := range part {
		switch r {
		case '(':
			depth++
			cur.WriteRune(r)
		case ')':
			if depth > 0 {
				depth--
			}
			cur.WriteRune(r)
		case ',':
			if depth == 0 {
				toks = append(toks, strings.TrimSpace(cur.String()))
				cur.Reset()
			} else {
				cur.WriteRune(r)
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		toks = append(toks, strings.TrimSpace(cur.String()))
	}
	out := make([]LabelExpr, 0, len(toks))
	for _, t := range toks {
		if t == "" {
			continue
		}
		expr, err := parseExpr(t)
		if err != nil {
			return nil, err
		}
		out = append(out, expr)
	}
	return out, nil
}

func parseExpr(tok string) (LabelExpr, error) {
	tok = strings.ToLower(strings.TrimSpace(tok))
	if strings.HasPrefix(tok, "!") {
		return NotExpr{Label: strings.TrimSpace(tok[1:])}, nil
	}
	if strings.Contains(tok, " in ") {
		parts := strings.SplitN(tok, " in ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid IN expression: %s", tok)
		}
		label := strings.TrimSpace(parts[0])
		vals := strings.TrimSpace(parts[1])
		if len(vals) < 2 || vals[0] != '(' || vals[len(vals)-1] != ')' {
			return nil, fmt.Errorf("invalid IN expression: %s", tok)
		}
		vals = vals[1 : len(vals)-1]
		items := strings.Split(vals, ",")
		values := make([]string, 0, len(items))
		for _, it := range items {
			it = strings.TrimSpace(it)
			if it != "" {
				values = append(values, it)
			}
		}
		return InExpr{Label: label, Values: values}, nil
	}
	if strings.Contains(tok, "=") {
		parts := strings.SplitN(tok, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid expression: %s", tok)
		}
		return EqExpr{Label: strings.TrimSpace(parts[0]), Value: strings.TrimSpace(parts[1])}, nil
	}
	return HasExpr{Label: tok}, nil
}

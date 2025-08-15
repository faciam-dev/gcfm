package sdk

import (
	"regexp"
	"strings"
)

func normalizeString(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

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
	s = normalizeString(s)
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
	part = normalizeString(part)
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
				toks = append(toks, normalizeString(cur.String()))
				cur.Reset()
			} else {
				cur.WriteRune(r)
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		toks = append(toks, normalizeString(cur.String()))
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
	tok = normalizeString(tok)
	if strings.HasPrefix(tok, "!") {
		return NotExpr{Label: normalizeString(tok[1:])}, nil
	}
	if matches := inExprRe.FindStringSubmatch(tok); matches != nil {
		label := normalizeString(matches[1])
		vals := matches[2]
		items := strings.Split(vals, ",")
		values := make([]string, 0, len(items))
		for _, it := range items {
			if v := normalizeString(it); v != "" {
				values = append(values, v)
			}
		}
		return InExpr{Label: label, Values: values}, nil
	}
	if i := strings.IndexByte(tok, '='); i >= 0 {
		return EqExpr{Label: normalizeString(tok[:i]), Value: normalizeString(tok[i+1:])}, nil
	}
	return HasExpr{Label: tok}, nil
}

// QueryFromLabels builds a Query that ANDs all label strings.
// "k=v" becomes EqExpr and plain "k" becomes HasExpr.
func QueryFromLabels(labels []string) Query {
	q := Query{AND: make([]LabelExpr, 0, len(labels))}
	for _, l := range labels {
		if i := strings.IndexByte(l, '='); i > 0 {
			q.AND = append(q.AND, EqExpr{Label: l[:i], Value: l[i+1:]})
		} else if l != "" {
			q.AND = append(q.AND, HasExpr{Label: l})
		}
	}
	return q
}

var inExprRe = regexp.MustCompile(`^([a-z0-9_\-]+)\s+in\s+\(([^)]*)\)$`)

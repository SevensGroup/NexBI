package semantic

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Formula is a derived-metric expression such as "revenue / spend".
type Formula struct {
	Expression string
	expr       exprNode
}

type exprNode interface {
	eval(vals map[string]float64) (float64, bool)
	deps() []string
}

type identNode struct{ name string }
type numberNode struct{ value float64 }
type binaryNode struct {
	op    byte
	left  exprNode
	right exprNode
}

func (n identNode) eval(vals map[string]float64) (float64, bool) {
	v, ok := vals[normalizeKey(n.name)]
	return v, ok
}

func (n identNode) deps() []string { return []string{n.name} }

func (n numberNode) eval(map[string]float64) (float64, bool) { return n.value, true }
func (n numberNode) deps() []string                          { return nil }

func (n binaryNode) eval(vals map[string]float64) (float64, bool) {
	l, okL := n.left.eval(vals)
	r, okR := n.right.eval(vals)
	if !okL || !okR {
		return 0, false
	}
	switch n.op {
	case '+':
		return l + r, true
	case '-':
		return l - r, true
	case '*':
		return l * r, true
	case '/':
		if r == 0 {
			return 0, false
		}
		return l / r, true
	default:
		return 0, false
	}
}

func (n binaryNode) deps() []string {
	return append(n.left.deps(), n.right.deps()...)
}

// ParseFormula compiles a simple arithmetic formula over metric identifiers.
func ParseFormula(expression string) (*Formula, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, fmt.Errorf("formula is required")
	}
	p := formulaParser{src: expression}
	node, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.peek() != 0 {
		return nil, fmt.Errorf("unexpected trailing formula input %q", p.src[p.i:])
	}
	return &Formula{Expression: expression, expr: node}, nil
}

// Dependencies returns metric identifiers referenced by the formula.
func (f *Formula) Dependencies() []string {
	if f == nil || f.expr == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, d := range f.expr.deps() {
		k := normalizeKey(d)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, d)
	}
	return out
}

// Eval computes the formula against named values. Missing deps or divide-by-zero yield ok=false.
func (f *Formula) Eval(vals map[string]float64) (float64, bool) {
	if f == nil || f.expr == nil {
		return 0, false
	}
	normalized := make(map[string]float64, len(vals))
	for k, v := range vals {
		normalized[normalizeKey(k)] = v
	}
	return f.expr.eval(normalized)
}

type formulaParser struct {
	src string
	i   int
}

func (p *formulaParser) skip() {
	for p.i < len(p.src) && unicode.IsSpace(rune(p.src[p.i])) {
		p.i++
	}
}

func (p *formulaParser) peek() byte {
	p.skip()
	if p.i >= len(p.src) {
		return 0
	}
	return p.src[p.i]
}

func (p *formulaParser) parseExpr() (exprNode, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	for {
		op := p.peek()
		if op != '+' && op != '-' {
			return left, nil
		}
		p.i++
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op, left: left, right: right}
	}
}

func (p *formulaParser) parseTerm() (exprNode, error) {
	left, err := p.parseFactor()
	if err != nil {
		return nil, err
	}
	for {
		op := p.peek()
		if op != '*' && op != '/' {
			return left, nil
		}
		p.i++
		right, err := p.parseFactor()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op, left: left, right: right}
	}
}

func (p *formulaParser) parseFactor() (exprNode, error) {
	switch p.peek() {
	case 0:
		return nil, fmt.Errorf("unexpected end of formula")
	case '(':
		p.i++
		n, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek() != ')' {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		p.i++
		return n, nil
	}
	if c := p.peek(); c >= '0' && c <= '9' || c == '.' {
		return p.parseNumber()
	}
	return p.parseIdent()
}

func (p *formulaParser) parseNumber() (exprNode, error) {
	p.skip()
	start := p.i
	for p.i < len(p.src) {
		c := p.src[p.i]
		if c >= '0' && c <= '9' || c == '.' {
			p.i++
			continue
		}
		break
	}
	v, err := strconv.ParseFloat(p.src[start:p.i], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number %q", p.src[start:p.i])
	}
	return numberNode{value: v}, nil
}

func (p *formulaParser) parseIdent() (exprNode, error) {
	p.skip()
	start := p.i
	for p.i < len(p.src) {
		c := rune(p.src[p.i])
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' {
			p.i++
			continue
		}
		break
	}
	if start == p.i {
		return nil, fmt.Errorf("expected identifier at %q", p.src[p.i:])
	}
	return identNode{name: p.src[start:p.i]}, nil
}

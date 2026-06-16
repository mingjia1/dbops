package alertexpr

import (
	"fmt"
	"strconv"
	"strings"
	"text/scanner"
)

type Evaluator struct {
	Metrics map[string]float64
}

func NewEvaluator(metrics map[string]float64) *Evaluator {
	return &Evaluator{Metrics: metrics}
}

// Eval parses and evaluates a compound alert expression.
// Supported: > >= < <= == != AND OR ( )
// Example: "cpu > 90 AND mem > 80"  or  "(connections > 500 OR qps > 10000) AND disk > 95"
func (e *Evaluator) Eval(expr string) (bool, error) {
	tokens, err := tokenize(expr)
	if err != nil {
		return false, fmt.Errorf("alertexpr: tokenize: %w", err)
	}
	p := &parser{tokens: tokens, pos: 0}
	result, err := p.parseExpr()
	if err != nil {
		return false, fmt.Errorf("alertexpr: parse: %w", err)
	}
	return e.eval(result)
}

func (e *Evaluator) eval(n node) (bool, error) {
	switch n := n.(type) {
	case *binaryNode:
		left, err := e.eval(n.left)
		if err != nil {
			return false, err
		}
		right, err := e.eval(n.right)
		if err != nil {
			return false, err
		}
		switch n.op {
		case "AND":
			return left && right, nil
		case "OR":
			return left || right, nil
		default:
			return false, fmt.Errorf("unknown logical op: %s", n.op)
		}
	case *comparisonNode:
		lv, err := e.resolveValue(n.left)
		if err != nil {
			return false, err
		}
		rv, err := e.resolveValue(n.right)
		if err != nil {
			return false, err
		}
		switch n.op {
		case ">":
			return lv > rv, nil
		case ">=":
			return lv >= rv, nil
		case "<":
			return lv < rv, nil
		case "<=":
			return lv <= rv, nil
		case "==", "=":
			return lv == rv, nil
		case "!=":
			return lv != rv, nil
		default:
			return false, fmt.Errorf("unknown comparison op: %s", n.op)
		}
	default:
		return false, fmt.Errorf("unknown node type %T", n)
	}
}

func (e *Evaluator) resolveValue(s string) (float64, error) {
	if val, ok := e.Metrics[s]; ok {
		return val, nil
	}
	return strconv.ParseFloat(s, 64)
}

// --- Lexer / tokenizer ---

type tokenKind int

const (
	tokIdent tokenKind = iota
	tokNumber
	tokOp
	tokLParen
	tokRParen
	tokEOF
)

type token struct {
	kind  tokenKind
	value string
}

func tokenize(expr string) ([]token, error) {
	var s scanner.Scanner
	s.Init(strings.NewReader(expr))
	s.Mode = scanner.ScanIdents | scanner.ScanInts | scanner.ScanFloats
	s.Error = func(_ *scanner.Scanner, msg string) {}

	var tokens []token
	for {
		r := s.Scan()
		if r == scanner.EOF {
			break
		}
		text := s.TokenText()
		switch r {
		case scanner.Int, scanner.Float:
			tokens = append(tokens, token{kind: tokNumber, value: text})
		case scanner.Ident:
			upper := strings.ToUpper(text)
			if upper == "AND" || upper == "OR" {
				tokens = append(tokens, token{kind: tokOp, value: upper})
			} else {
				tokens = append(tokens, token{kind: tokIdent, value: text})
			}
		case '(':
			tokens = append(tokens, token{kind: tokLParen, value: "("})
		case ')':
			tokens = append(tokens, token{kind: tokRParen, value: ")"})
		default:
			if strings.ContainsRune("><=!", r) {
				// peek next for >=, <=, ==, !=
				next := s.Peek()
				if next == '=' {
					s.Scan()
					tokens = append(tokens, token{kind: tokOp, value: string(r) + "="})
				} else {
					tokens = append(tokens, token{kind: tokOp, value: string(r)})
				}
			} else {
				return nil, fmt.Errorf("unexpected character: %c", r)
			}
		}
	}
	tokens = append(tokens, token{kind: tokEOF})
	return tokens, nil
}

// --- Parser (recursive descent) ---
// Grammar:
//   expr     := term (OR term)*
//   term     := factor (AND factor)*
//   factor   := comparison | "(" expr ")"
//   comparison := ident op number_or_ident

type node interface{}

type binaryNode struct {
	op    string
	left  node
	right node
}

type comparisonNode struct {
	op    string
	left  string
	right string
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) peek() token {
	return p.tokens[p.pos]
}

func (p *parser) advance() token {
	t := p.tokens[p.pos]
	p.pos++
	return t
}

func (p *parser) expect(kind tokenKind, values ...string) (token, error) {
	t := p.peek()
	if t.kind != kind {
		return t, fmt.Errorf("expected %v, got %v (%s)", kind, t.kind, t.value)
	}
	if len(values) > 0 {
		match := false
		for _, v := range values {
			if t.value == v {
				match = true
				break
			}
		}
		if !match {
			return t, fmt.Errorf("expected one of %v, got %s", values, t.value)
		}
	}
	return p.advance(), nil
}

func (p *parser) parseExpr() (node, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOp && p.peek().value == "OR" {
		op := p.advance().value
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		left = &binaryNode{op: op, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseTerm() (node, error) {
	left, err := p.parseFactor()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOp && p.peek().value == "AND" {
		op := p.advance().value
		right, err := p.parseFactor()
		if err != nil {
			return nil, err
		}
		left = &binaryNode{op: op, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseFactor() (node, error) {
	if p.peek().kind == tokLParen {
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokRParen); err != nil {
			return nil, err
		}
		return expr, nil
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() (node, error) {
	t := p.peek()
	if t.kind != tokIdent && t.kind != tokNumber {
		return nil, fmt.Errorf("expected identifier or number, got %s", t.value)
	}
	left := p.advance().value

	opTok, err := p.expect(tokOp, ">", ">=", "<", "<=", "==", "=", "!=")
	if err != nil {
		return nil, err
	}

	t2 := p.peek()
	if t2.kind != tokIdent && t2.kind != tokNumber {
		return nil, fmt.Errorf("expected identifier or number for right operand, got %s", t2.value)
	}
	right := p.advance().value

	return &comparisonNode{op: opTok.value, left: left, right: right}, nil
}



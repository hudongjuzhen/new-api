package runninghub

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
)

// Per-second billing needs one number before the run starts: how many seconds
// to pre-charge. Which node carries that number is app-specific (执行秒数,
// 时长, duration …) and the interesting cases are ranges the upstream derives
// from two inputs rather than a single field. App.SecondsExpr therefore holds a
// small arithmetic expression over node ids, e.g.
//
//	212                    node 212's submitted value
//	nodeId=212             same, explicit form
//	229-212                node 229 minus node 212 (结束 - 开始)
//	nodeId=229-nodeId=212  same, explicit form
//	(229-212)*2 + 1.5      arithmetic with literals and parentheses
//
// A bare number is resolved as a node id when the app's schema declares that
// node, and read as a literal constant otherwise — so "229-212" means what the
// admin sees in the parameter list, while "212-2" still works as arithmetic.
// The evaluated result is clamped to [1, MaxTaskDurationSeconds] (the same
// bound coerceValueByType applies to duration-typed parameters) so the value
// can never become an unbounded quota multiplier.

// secondsNodeRef is one node reference inside an expression.
type secondsNodeRef struct {
	nodeID    string
	fieldName string
}

func (r secondsNodeRef) String() string {
	if r.fieldName == "" {
		return "nodeId=" + r.nodeID
	}
	return "nodeId=" + r.nodeID + "." + r.fieldName
}

type secondsExprKind int

const (
	secondsExprLiteral secondsExprKind = iota
	secondsExprRef
	secondsExprBinary
)

// secondsExprNode is one node of the compiled expression tree. Literal nodes
// keep the raw token in bareNodeID when it was written as a plain number so
// evaluation can still treat it as a node id first (see the package comment).
type secondsExprNode struct {
	kind       secondsExprKind
	value      float64
	bareNodeID string
	ref        secondsNodeRef
	op         secondsExprTokenKind
	left       *secondsExprNode
	right      *secondsExprNode
}

type secondsExprTokenKind int

const (
	secondsTokEOF secondsExprTokenKind = iota
	secondsTokNumber
	secondsTokRef
	secondsTokPlus
	secondsTokMinus
	secondsTokStar
	secondsTokSlash
	secondsTokLParen
	secondsTokRParen
)

type secondsExprToken struct {
	kind secondsExprTokenKind
	text string
	pos  int
}

func (k secondsExprTokenKind) String() string {
	switch k {
	case secondsTokNumber:
		return "数字"
	case secondsTokRef:
		return "字段引用"
	case secondsTokPlus:
		return "+"
	case secondsTokMinus:
		return "-"
	case secondsTokStar:
		return "*"
	case secondsTokSlash:
		return "/"
	case secondsTokLParen:
		return "("
	case secondsTokRParen:
		return ")"
	default:
		return "表达式结尾"
	}
}

// validateSecondsExpr checks the expression syntax only. It is used by the
// admin save path so a malformed expression is rejected while the admin is
// still looking at the form, rather than at the next user submission.
func validateSecondsExpr(expr string) error {
	_, err := parseSecondsExpr(expr)
	return err
}

// secondsFromExpr evaluates expr against the values submitted for this run.
// Any unresolvable reference or malformed arithmetic is reported as an error so
// the caller can reject the submit instead of silently billing a fallback.
func secondsFromExpr(expr string, schema []rhparser.SchemaParam, values map[string]any) (float64, error) {
	root, err := parseSecondsExpr(expr)
	if err != nil {
		return 0, err
	}
	n, err := root.eval(nodeSecondsLookup(schema, values))
	if err != nil {
		return 0, err
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("秒数表达式 %q 的结果非法", expr)
	}
	if n < 1 {
		n = 1
	}
	if n > float64(relaycommon.MaxTaskDurationSeconds) {
		n = float64(relaycommon.MaxTaskDurationSeconds)
	}
	return n, nil
}

// nodeSecondsLookup resolves a node reference to the number the user submitted
// for it. A nodeId-only reference matches the first submitted parameter on that
// node (rh schemas can declare several fields on one node).
func nodeSecondsLookup(schema []rhparser.SchemaParam, values map[string]any) func(string, string) (float64, bool) {
	return func(nodeID, fieldName string) (float64, bool) {
		for _, p := range schema {
			if strings.TrimSpace(p.NodeID) != nodeID {
				continue
			}
			if fieldName != "" && strings.TrimSpace(p.FieldName) != fieldName {
				continue
			}
			raw, ok := values[schemaFieldKey(p.NodeID, p.FieldName)]
			if !ok {
				if fieldName != "" {
					return 0, false
				}
				continue
			}
			n, ok := asNumber(raw)
			if !ok {
				return 0, false
			}
			return n, true
		}
		return 0, false
	}
}

func (n *secondsExprNode) eval(lookup func(nodeID, fieldName string) (float64, bool)) (float64, error) {
	switch n.kind {
	case secondsExprLiteral:
		if n.bareNodeID != "" {
			if v, ok := lookup(n.bareNodeID, ""); ok {
				return v, nil
			}
		}
		return n.value, nil
	case secondsExprRef:
		v, ok := lookup(n.ref.nodeID, n.ref.fieldName)
		if !ok {
			return 0, fmt.Errorf("秒数表达式引用的字段在本次提交中不可用：%s", n.ref)
		}
		return v, nil
	case secondsExprBinary:
		left, err := n.left.eval(lookup)
		if err != nil {
			return 0, err
		}
		right, err := n.right.eval(lookup)
		if err != nil {
			return 0, err
		}
		return applySecondsOp(n.op, left, right)
	default:
		return 0, fmt.Errorf("秒数表达式节点类型非法")
	}
}

func applySecondsOp(op secondsExprTokenKind, left, right float64) (float64, error) {
	switch op {
	case secondsTokPlus:
		return left + right, nil
	case secondsTokMinus:
		return left - right, nil
	case secondsTokStar:
		return left * right, nil
	case secondsTokSlash:
		if right == 0 {
			return 0, fmt.Errorf("秒数表达式出现除以 0")
		}
		return left / right, nil
	default:
		return 0, fmt.Errorf("秒数表达式运算符非法")
	}
}

// -- parsing ---------------------------------------------------------------

func parseSecondsExpr(expr string) (*secondsExprNode, error) {
	tokens, err := tokenizeSecondsExpr(expr)
	if err != nil {
		return nil, err
	}
	p := &secondsExprParser{tokens: tokens}
	root, err := p.parseSum()
	if err != nil {
		return nil, err
	}
	if tok := p.peek(); tok.kind != secondsTokEOF {
		return nil, fmt.Errorf("秒数表达式 %q 在 %q 处无法解析", expr, tok.text)
	}
	return root, nil
}

type secondsExprParser struct {
	tokens []secondsExprToken
	pos    int
}

func (p *secondsExprParser) peek() secondsExprToken {
	if p.pos >= len(p.tokens) {
		return secondsExprToken{kind: secondsTokEOF}
	}
	return p.tokens[p.pos]
}

func (p *secondsExprParser) next() secondsExprToken {
	tok := p.peek()
	p.pos++
	return tok
}

func (p *secondsExprParser) parseSum() (*secondsExprNode, error) {
	left, err := p.parseProduct()
	if err != nil {
		return nil, err
	}
	for {
		op := p.peek().kind
		if op != secondsTokPlus && op != secondsTokMinus {
			return left, nil
		}
		p.next()
		right, err := p.parseProduct()
		if err != nil {
			return nil, err
		}
		left = &secondsExprNode{kind: secondsExprBinary, op: op, left: left, right: right}
	}
}

func (p *secondsExprParser) parseProduct() (*secondsExprNode, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		op := p.peek().kind
		if op != secondsTokStar && op != secondsTokSlash {
			return left, nil
		}
		p.next()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &secondsExprNode{kind: secondsExprBinary, op: op, left: left, right: right}
	}
}

func (p *secondsExprParser) parseUnary() (*secondsExprNode, error) {
	switch p.peek().kind {
	case secondsTokMinus:
		p.next()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &secondsExprNode{
			kind:  secondsExprBinary,
			op:    secondsTokMinus,
			left:  &secondsExprNode{kind: secondsExprLiteral, value: 0},
			right: operand,
		}, nil
	case secondsTokPlus:
		p.next()
		return p.parseUnary()
	default:
		return p.parsePrimary()
	}
}

func (p *secondsExprParser) parsePrimary() (*secondsExprNode, error) {
	tok := p.next()
	switch tok.kind {
	case secondsTokNumber:
		v, err := strconv.ParseFloat(tok.text, 64)
		if err != nil {
			return nil, fmt.Errorf("秒数表达式里的数字 %q 非法", tok.text)
		}
		return &secondsExprNode{kind: secondsExprLiteral, value: v, bareNodeID: tok.text}, nil
	case secondsTokRef:
		return &secondsExprNode{kind: secondsExprRef, ref: parseSecondsRef(tok.text)}, nil
	case secondsTokLParen:
		inner, err := p.parseSum()
		if err != nil {
			return nil, err
		}
		if closing := p.next(); closing.kind != secondsTokRParen {
			return nil, fmt.Errorf("秒数表达式缺少右括号")
		}
		return inner, nil
	default:
		return nil, fmt.Errorf("秒数表达式在 %q 处缺少数字或字段引用", tok.text)
	}
}

// parseSecondsRef splits a "@229" / "@229.value" token body into its parts.
func parseSecondsRef(body string) secondsNodeRef {
	nodeID, fieldName, _ := strings.Cut(body, ".")
	return secondsNodeRef{nodeID: strings.TrimSpace(nodeID), fieldName: strings.TrimSpace(fieldName)}
}

func tokenizeSecondsExpr(expr string) ([]secondsExprToken, error) {
	var tokens []secondsExprToken
	for i := 0; i < len(expr); {
		c := expr[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '+', c == '-', c == '*', c == '/', c == '(', c == ')':
			tokens = append(tokens, secondsExprToken{kind: secondsOperatorToken(c), text: string(c), pos: i})
			i++
		case c == '@':
			end := scanSecondsRefBody(expr, i+1)
			if end == i+1 {
				return nil, fmt.Errorf("秒数表达式在位置 %d 处的字段引用为空", i)
			}
			tokens = append(tokens, secondsExprToken{kind: secondsTokRef, text: expr[i+1 : end], pos: i})
			i = end
		case isSecondsExprIdentStart(c):
			// Either the "nodeId=" prefix (then a reference body follows) or a
			// bare numeric token; anything else is a typo worth reporting.
			if rest, ok := consumeSecondsRefPrefix(expr, i); ok {
				end := scanSecondsRefBody(expr, rest)
				if end == rest {
					return nil, fmt.Errorf("秒数表达式在位置 %d 处的字段引用为空", i)
				}
				tokens = append(tokens, secondsExprToken{kind: secondsTokRef, text: expr[rest:end], pos: i})
				i = end
				continue
			}
			end := scanSecondsRefBody(expr, i)
			text := expr[i:end]
			if _, err := strconv.ParseFloat(text, 64); err != nil {
				return nil, fmt.Errorf("秒数表达式里的 %q 无法识别；字段引用请写成 nodeId=%s 或 @%s", text, text, text)
			}
			tokens = append(tokens, secondsExprToken{kind: secondsTokNumber, text: text, pos: i})
			i = end
		default:
			return nil, fmt.Errorf("秒数表达式包含非法字符 %q", string(c))
		}
	}
	return append(tokens, secondsExprToken{kind: secondsTokEOF, pos: len(expr)}), nil
}

func secondsOperatorToken(c byte) secondsExprTokenKind {
	switch c {
	case '+':
		return secondsTokPlus
	case '-':
		return secondsTokMinus
	case '*':
		return secondsTokStar
	case '/':
		return secondsTokSlash
	case '(':
		return secondsTokLParen
	default:
		return secondsTokRParen
	}
}

func isSecondsExprIdentStart(c byte) bool {
	return c >= '0' && c <= '9' || c == '.' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// scanSecondsRefBody returns the end index of a reference body starting at i:
// digits/letters/underscore plus an optional "." field suffix.
func scanSecondsRefBody(expr string, i int) int {
	for i < len(expr) {
		c := expr[i]
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '.' {
			i++
			continue
		}
		break
	}
	return i
}

// consumeSecondsRefPrefix recognises "nodeId" (any case) + optional spaces +
// "=" at i and returns the index just past the "=".
func consumeSecondsRefPrefix(expr string, i int) (int, bool) {
	const prefix = "nodeid"
	if len(expr)-i < len(prefix) || !strings.EqualFold(expr[i:i+len(prefix)], prefix) {
		return 0, false
	}
	j := i + len(prefix)
	for j < len(expr) && (expr[j] == ' ' || expr[j] == '\t') {
		j++
	}
	if j >= len(expr) || expr[j] != '=' {
		return 0, false
	}
	j++
	for j < len(expr) && (expr[j] == ' ' || expr[j] == '\t') {
		j++
	}
	return j, true
}

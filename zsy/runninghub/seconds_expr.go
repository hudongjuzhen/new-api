package runninghub

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
)

// Node-value expressions are the shared input of the two metered billing modes:
// per-second and per-character. Both need one number before the run starts —
// how many seconds, or how many characters — read out of what the customer
// submitted for this run.
//
// A per-second app needs a run length (执行秒数, 时长, duration …) and the
// interesting cases are ranges the upstream derives from two inputs rather than
// a single field. App.SecondsExpr therefore holds a small arithmetic expression
// over node ids, e.g.
//
//	212                    node 212's submitted value
//	nodeId=212             same, explicit form
//	229-212                node 229 minus node 212 (结束 - 开始)
//	nodeId=229-nodeId=212  same, explicit form
//	(229-212)*2 + 1.5      arithmetic with literals and parentheses
//
// A per-character app needs the size of a text input (提示词, 文案 …).
// App.CharCountExpr holds the same kind of expression, with one addition: len()
// reads a referenced value as text and yields its rune count.
//
//	212                    rune count of node 212's submitted text
//	nodeId=212             same, explicit form
//	len(212)               same, explicit form
//	len(122) + len(123)    two text inputs counted together
//	len(122)*2 + 10        arithmetic over a counted value
//
// A bare number is resolved as a node id when the app's schema declares that
// node, and read as a literal constant otherwise — so "229-212" means what the
// admin sees in the parameter list, while an id-free expression such as
// "10*2+1" is plain arithmetic. Because a declared node id always wins, a
// per-character expression that needs a literal must be written so it cannot be
// read as an id.
//
// The evaluated seconds value is clamped to [1, MaxTaskDurationSeconds] (the
// same bound coerceValueByType applies to duration-typed parameters) and the
// character count to [0, maxExprRunes] so neither can become an unbounded quota
// multiplier.

// maxExprRunes bounds the character count one run may be billed for. Parameter
// schemas already cap text inputs (a few thousand runes in practice); the bound
// exists so a malformed expression or an absurd payload can never feed a huge
// multiplier into quota math. See the billing-safety rules in AGENTS.md.
const maxExprRunes = 1_000_000

// exprNodeRef is one node reference inside an expression.
type exprNodeRef struct {
	nodeID    string
	fieldName string
}

func (r exprNodeRef) String() string {
	if r.fieldName == "" {
		return "nodeId=" + r.nodeID
	}
	return "nodeId=" + r.nodeID + "." + r.fieldName
}

type exprKind int

const (
	exprLiteral exprKind = iota
	exprRef
	exprBinary
	exprLen
)

// exprNode is one node of the compiled expression tree. Literal nodes keep the
// raw token in bareNodeID when it was written as a plain number so evaluation
// can still treat it as a node id first (see the package comment).
type exprNode struct {
	kind       exprKind
	value      float64
	bareNodeID string
	ref        exprNodeRef
	op         exprTokenKind
	left       *exprNode
	right      *exprNode
	// operand is the single child of exprLen.
	operand *exprNode
}

type exprTokenKind int

const (
	exprTokEOF exprTokenKind = iota
	exprTokNumber
	exprTokRef
	exprTokLen
	exprTokPlus
	exprTokMinus
	exprTokStar
	exprTokSlash
	exprTokLParen
	exprTokRParen
)

type exprToken struct {
	kind exprTokenKind
	text string
	pos  int
}

func (k exprTokenKind) String() string {
	switch k {
	case exprTokNumber:
		return "数字"
	case exprTokRef:
		return "字段引用"
	case exprTokLen:
		return "len"
	case exprTokPlus:
		return "+"
	case exprTokMinus:
		return "-"
	case exprTokStar:
		return "*"
	case exprTokSlash:
		return "/"
	case exprTokLParen:
		return "("
	case exprTokRParen:
		return ")"
	default:
		return "表达式结尾"
	}
}

// exprValue is one resolved node value. text marks a value the customer typed
// as text (as opposed to a number the upstream parses), which is what decides
// whether a bare node reference contributes a character count or its numeric
// value.
type exprValue struct {
	number float64
	text   string
	isText bool
}

// asText returns the value's canonical string form for character counting: the
// submitted text itself, or the number as the upstream would receive it.
func (v exprValue) asText() string {
	if v.isText {
		return v.text
	}
	return numberToString(v.number)
}

// asTerm returns the value as one term of an expression. Text contributes its
// rune count when the expression bills characters (textCountsAsLength), and
// otherwise the number it parses to; a number always contributes its value.
func (v exprValue) asTerm(textCountsAsLength bool) float64 {
	if textCountsAsLength && v.isText {
		return float64(utf8.RuneCountInString(v.text))
	}
	if v.isText {
		n, _ := asNumber(v.text)
		return n
	}
	return v.number
}

// exprResolver resolves a node reference to the value submitted for it. An
// unresolvable reference is reported as not-found so the caller can reject the
// submit instead of silently billing a fallback.
type exprResolver func(ref exprNodeRef) (exprValue, bool)

// validateSecondsExpr and validateCharCountExpr check the expression syntax
// only. They are used by the admin save path so a malformed expression is
// rejected while the admin is still looking at the form, rather than at the
// next user submission.
func validateSecondsExpr(expr string) error {
	_, err := parseExpression(expr)
	return err
}

func validateCharCountExpr(expr string) error {
	_, err := parseExpression(expr)
	return err
}

// secondsFromExpr evaluates expr against the values submitted for this run.
// Any unresolvable reference or malformed arithmetic is reported as an error so
// the caller can reject the submit instead of silently billing a fallback.
func secondsFromExpr(expr string, schema []rhparser.SchemaParam, values map[string]any) (float64, error) {
	n, err := evalExpression(expr, nodeNumberResolver(schema, values), false)
	if err != nil {
		return 0, err
	}
	if n < 1 {
		n = 1
	}
	if n > float64(relaycommon.MaxTaskDurationSeconds) {
		n = float64(relaycommon.MaxTaskDurationSeconds)
	}
	return n, nil
}

// charsFromExpr evaluates expr against the values submitted for this run and
// returns the rune count to bill. Unlike seconds there is no floor of one: an
// empty text input is 0 characters and costs nothing, and a negative result
// from a contrived expression (len(a)-len(b)) clamps to 0 rather than becoming
// a credit. The result is still bounded, so the value that reaches the quota
// math is always finite and in range.
func charsFromExpr(expr string, schema []rhparser.SchemaParam, values map[string]any) (float64, error) {
	n, err := evalExpression(expr, nodeTextResolver(schema, values), true)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, nil
	}
	if n > maxExprRunes {
		n = maxExprRunes
	}
	return n, nil
}

// evalExpression parses expr, evaluates it and rejects a non-finite result.
// textCountsAsLength selects how a reference to submitted *text* is read: in a
// per-character expression it contributes its rune count, otherwise the number
// its text parses to.
func evalExpression(expr string, resolve exprResolver, textCountsAsLength bool) (float64, error) {
	root, err := parseExpression(expr)
	if err != nil {
		return 0, err
	}
	n, err := root.eval(resolve, textCountsAsLength)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("表达式 %q 的结果非法", expr)
	}
	return n, nil
}

// nodeNumberResolver resolves a node reference to the number the user submitted
// for it. A nodeId-only reference matches the first submitted parameter on that
// node (rh schemas can declare several fields on one node).
func nodeNumberResolver(schema []rhparser.SchemaParam, values map[string]any) exprResolver {
	return func(ref exprNodeRef) (exprValue, bool) {
		raw, ok := lookupSubmittedValue(schema, values, ref)
		if !ok {
			return exprValue{}, false
		}
		n, ok := asNumber(raw)
		if !ok {
			return exprValue{}, false
		}
		return exprValue{number: n}, true
	}
}

// nodeTextResolver resolves a node reference to the text the user submitted for
// it: a text-typed value is reported as text (so it can be counted), a number
// keeps its numeric value (so it can be used in arithmetic). Either way the
// canonical string form is attached, which is what the node field sends
// upstream.
func nodeTextResolver(schema []rhparser.SchemaParam, values map[string]any) exprResolver {
	return func(ref exprNodeRef) (exprValue, bool) {
		raw, ok := lookupSubmittedValue(schema, values, ref)
		if !ok {
			return exprValue{}, false
		}
		switch v := raw.(type) {
		case string:
			// An all-space string is not a number: it is text whose count is
			// whatever the trim left, which is nothing.
			n, isNumber := asNumber(v)
			if !isNumber && strings.TrimSpace(v) == "" {
				return exprValue{text: strings.TrimSpace(v), isText: true}, true
			}
			if !isNumber {
				return exprValue{text: v, isText: true}, true
			}
			return exprValue{number: n, text: v}, true
		case bool:
			s := strconv.FormatBool(v)
			return exprValue{text: s, isText: true}, true
		case nil:
			return exprValue{text: "", isText: true}, true
		}
		n, ok := asNumber(raw)
		if !ok {
			return exprValue{}, false
		}
		return exprValue{number: n, text: numberToString(n)}, true
	}
}

// lookupSubmittedValue finds the submitted value a reference points at. A
// nodeId-only reference matches the first submitted field on that node.
func lookupSubmittedValue(schema []rhparser.SchemaParam, values map[string]any, ref exprNodeRef) (any, bool) {
	for _, p := range schema {
		if strings.TrimSpace(p.NodeID) != ref.nodeID {
			continue
		}
		if ref.fieldName != "" && strings.TrimSpace(p.FieldName) != ref.fieldName {
			continue
		}
		raw, ok := values[schemaFieldKey(p.NodeID, p.FieldName)]
		if !ok {
			if ref.fieldName != "" {
				return nil, false
			}
			continue
		}
		return raw, true
	}
	return nil, false
}

func (n *exprNode) eval(resolve exprResolver, textCountsAsLength bool) (float64, error) {
	switch n.kind {
	case exprLiteral:
		// A declared node id always wins over the literal reading of a bare
		// number token: "212" is node 212 whenever the app declares it.
		if n.bareNodeID != "" {
			if v, ok := resolve(exprNodeRef{nodeID: n.bareNodeID}); ok {
				return v.asTerm(textCountsAsLength), nil
			}
		}
		return n.value, nil
	case exprRef:
		v, ok := resolve(n.ref)
		if !ok {
			return 0, fmt.Errorf("表达式引用的字段在本次提交中不可用：%s", n.ref)
		}
		return v.asTerm(textCountsAsLength), nil
	case exprLen:
		return n.evalRuneCount(resolve, textCountsAsLength)
	case exprBinary:
		left, err := n.left.eval(resolve, textCountsAsLength)
		if err != nil {
			return 0, err
		}
		right, err := n.right.eval(resolve, textCountsAsLength)
		if err != nil {
			return 0, err
		}
		return applyExprOp(n.op, left, right)
	default:
		return 0, fmt.Errorf("表达式节点类型非法")
	}
}

// evalRuneCount evaluates len(…). Its operand is counted as text when it names
// a submitted node; any other expression contributes the digit count of its own
// (numeric) result, so len() never fails on a well-formed operand.
func (n *exprNode) evalRuneCount(resolve exprResolver, textCountsAsLength bool) (float64, error) {
	switch n.operand.kind {
	case exprRef:
		v, ok := resolve(n.operand.ref)
		if !ok {
			return 0, fmt.Errorf("表达式引用的字段在本次提交中不可用：%s", n.operand.ref)
		}
		return float64(utf8.RuneCountInString(v.asText())), nil
	case exprLiteral:
		if n.operand.bareNodeID != "" {
			if v, ok := resolve(exprNodeRef{nodeID: n.operand.bareNodeID}); ok {
				return float64(utf8.RuneCountInString(v.asText())), nil
			}
		}
	}
	value, err := n.operand.eval(resolve, textCountsAsLength)
	if err != nil {
		return 0, err
	}
	return float64(utf8.RuneCountInString(numberToString(value))), nil
}

func applyExprOp(op exprTokenKind, left, right float64) (float64, error) {
	switch op {
	case exprTokPlus:
		return left + right, nil
	case exprTokMinus:
		return left - right, nil
	case exprTokStar:
		return left * right, nil
	case exprTokSlash:
		if right == 0 {
			return 0, fmt.Errorf("表达式出现除以 0")
		}
		return left / right, nil
	default:
		return 0, fmt.Errorf("表达式运算符非法")
	}
}

// -- parsing ---------------------------------------------------------------

func parseExpression(expr string) (*exprNode, error) {
	tokens, err := tokenizeExpression(expr)
	if err != nil {
		return nil, err
	}
	p := &exprParser{tokens: tokens}
	root, err := p.parseSum()
	if err != nil {
		return nil, err
	}
	if tok := p.peek(); tok.kind != exprTokEOF {
		return nil, fmt.Errorf("表达式 %q 在 %q 处无法解析", expr, tok.text)
	}
	return root, nil
}

type exprParser struct {
	tokens []exprToken
	pos    int
}

func (p *exprParser) peek() exprToken {
	if p.pos >= len(p.tokens) {
		return exprToken{kind: exprTokEOF}
	}
	return p.tokens[p.pos]
}

func (p *exprParser) next() exprToken {
	tok := p.peek()
	p.pos++
	return tok
}

func (p *exprParser) parseSum() (*exprNode, error) {
	left, err := p.parseProduct()
	if err != nil {
		return nil, err
	}
	for {
		op := p.peek().kind
		if op != exprTokPlus && op != exprTokMinus {
			return left, nil
		}
		p.next()
		right, err := p.parseProduct()
		if err != nil {
			return nil, err
		}
		left = &exprNode{kind: exprBinary, op: op, left: left, right: right}
	}
}

func (p *exprParser) parseProduct() (*exprNode, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		op := p.peek().kind
		if op != exprTokStar && op != exprTokSlash {
			return left, nil
		}
		p.next()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &exprNode{kind: exprBinary, op: op, left: left, right: right}
	}
}

func (p *exprParser) parseUnary() (*exprNode, error) {
	switch p.peek().kind {
	case exprTokMinus:
		p.next()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &exprNode{
			kind:  exprBinary,
			op:    exprTokMinus,
			left:  &exprNode{kind: exprLiteral, value: 0},
			right: operand,
		}, nil
	case exprTokPlus:
		p.next()
		return p.parseUnary()
	default:
		return p.parsePrimary()
	}
}

func (p *exprParser) parsePrimary() (*exprNode, error) {
	tok := p.next()
	switch tok.kind {
	case exprTokNumber:
		v, err := strconv.ParseFloat(tok.text, 64)
		if err != nil {
			return nil, fmt.Errorf("表达式里的数字 %q 非法", tok.text)
		}
		return &exprNode{kind: exprLiteral, value: v, bareNodeID: tok.text}, nil
	case exprTokRef:
		return &exprNode{kind: exprRef, ref: parseExprRef(tok.text)}, nil
	case exprTokLen:
		return p.parseLen()
	case exprTokLParen:
		inner, err := p.parseSum()
		if err != nil {
			return nil, err
		}
		if closing := p.next(); closing.kind != exprTokRParen {
			return nil, fmt.Errorf("表达式缺少右括号")
		}
		return inner, nil
	default:
		return nil, fmt.Errorf("表达式在 %q 处缺少数字或字段引用", tok.text)
	}
}

// parseLen reads the "(expression)" tail of a len() call.
func (p *exprParser) parseLen() (*exprNode, error) {
	if p.next().kind != exprTokLParen {
		return nil, fmt.Errorf("len 后面必须跟括号，例如 len(212)")
	}
	operand, err := p.parseSum()
	if err != nil {
		return nil, err
	}
	if closing := p.next(); closing.kind != exprTokRParen {
		return nil, fmt.Errorf("len 缺少右括号")
	}
	return &exprNode{kind: exprLen, operand: operand}, nil
}

// parseExprRef splits a "@229" / "@229.value" token body into its parts.
func parseExprRef(body string) exprNodeRef {
	nodeID, fieldName, _ := strings.Cut(body, ".")
	return exprNodeRef{nodeID: strings.TrimSpace(nodeID), fieldName: strings.TrimSpace(fieldName)}
}

func tokenizeExpression(expr string) ([]exprToken, error) {
	var tokens []exprToken
	for i := 0; i < len(expr); {
		c := expr[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '+', c == '-', c == '*', c == '/', c == '(', c == ')':
			tokens = append(tokens, exprToken{kind: exprOperatorToken(c), text: string(c), pos: i})
			i++
		case c == '@':
			end := scanExprRefBody(expr, i+1)
			if end == i+1 {
				return nil, fmt.Errorf("表达式在位置 %d 处的字段引用为空", i)
			}
			tokens = append(tokens, exprToken{kind: exprTokRef, text: expr[i+1 : end], pos: i})
			i = end
		case isExprIdentStart(c):
			// Either the "len(" / "nodeId=" keyword, or a bare numeric token;
			// anything else is a typo worth reporting.
			if end := consumeExprKeyword(expr, i, "len"); end >= 0 {
				tokens = append(tokens, exprToken{kind: exprTokLen, text: "len", pos: i})
				i = end
				continue
			}
			if rest, ok := consumeExprRefPrefix(expr, i); ok {
				end := scanExprRefBody(expr, rest)
				if end == rest {
					return nil, fmt.Errorf("表达式在位置 %d 处的字段引用为空", i)
				}
				tokens = append(tokens, exprToken{kind: exprTokRef, text: expr[rest:end], pos: i})
				i = end
				continue
			}
			end := scanExprRefBody(expr, i)
			text := expr[i:end]
			if _, err := strconv.ParseFloat(text, 64); err != nil {
				return nil, fmt.Errorf("表达式里的 %q 无法识别；字段引用请写成 nodeId=%s、@%s 或 len(%s)", text, text, text, text)
			}
			tokens = append(tokens, exprToken{kind: exprTokNumber, text: text, pos: i})
			i = end
		default:
			return nil, fmt.Errorf("表达式包含非法字符 %q", string(c))
		}
	}
	return append(tokens, exprToken{kind: exprTokEOF, pos: len(expr)}), nil
}

func exprOperatorToken(c byte) exprTokenKind {
	switch c {
	case '+':
		return exprTokPlus
	case '-':
		return exprTokMinus
	case '*':
		return exprTokStar
	case '/':
		return exprTokSlash
	case '(':
		return exprTokLParen
	default:
		return exprTokRParen
	}
}

func isExprIdentStart(c byte) bool {
	return c >= '0' && c <= '9' || c == '.' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// scanExprRefBody returns the end index of a reference body starting at i:
// digits/letters/underscore plus an optional "." field suffix.
func scanExprRefBody(expr string, i int) int {
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

// consumeExprKeyword recognises an identifier (any case) equal to keyword that
// is followed by "(" and returns the index just past that "(". A keyword with
// no call parenthesis is not a keyword, so -1 lets the caller keep scanning.
func consumeExprKeyword(expr string, i int, keyword string) int {
	if len(expr)-i < len(keyword)+1 || !strings.EqualFold(expr[i:i+len(keyword)], keyword) {
		return -1
	}
	j := i + len(keyword)
	for j < len(expr) && (expr[j] == ' ' || expr[j] == '\t') {
		j++
	}
	if j >= len(expr) || expr[j] != '(' {
		return -1
	}
	return j
}

// consumeExprRefPrefix recognises "nodeId" (any case) + optional spaces + "="
// at i and returns the index just past the "=".
func consumeExprRefPrefix(expr string, i int) (int, bool) {
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

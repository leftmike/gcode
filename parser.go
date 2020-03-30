package gcode

/*
To Do:
- RepRap: support {} instead of [] for expressions
- _ prefix for global parameter names
- LinuxCNC
-- numbers are equal if their absolute difference is less that 0.0001
-- #1 to #30 are subroutine parameters and are local to the subroutine
-- #<name> are local to the scope where it is assigned; scoped to subroutines
-- #31 + and #<_name> are global

<line> = <prefix> <body> <suffix> ('\r' | '\n')
<prefix> = (<whitespace> | <inline-comment>)* ['N' <number>]
<suffix> = ['*' <number> <whitespace>*] [<trailing-comment>]
<body> = (<whitespace> | <inline-comment> | <command> | <assignment>)*
<beagleg-body> =
      <body>
    | 'IF' <expr> 'THEN' <assignment> ('ELSEIF' <expr> 'THEN' <assignment>)* ['ELSE' <assignment>]
    | 'WHILE' <expr> 'DO' <suffix> <line>* <suffix> 'END'
<command> = <code> <expr>
<assignment> =
      <parameter> <whitespace>* <assign-op> <whitespace>* <expr>
    | <parameter> <whitespace>* '++'
    | <parameter> <whitespace>* '--'
<parameter> =
      '#' <integer>
    | '#' <initial-name-char> <name-char>* ;; BeagleG
    | '#' <name>
<expr> =
      <reference>
    | '[' <sub-expr> ']'
    | <number>
    | <name>
    | <string>
<sub-expr> =
      <number>
    | '-' <sub-expr>
    | '!' <sub-expr>
    | '[' <sub-expr> ']'
    | <sub-expr> <op> <sub-expr>
    | <reference>
    | <name>
    | <string>
    | <func> '[' [<sub-expr> [',' ...]] ']'
<op> = '+' '-' '*' '/'
    | '==' '!=' '<' '<=' '>' '>='
    | '&&' '||'
<reference> = '#'* <parameter>
<trailing-comment> = (';' | '%') <any-char>*
<inline-comment> = '(' <any-char>* ')'
<code> = 'A' ... 'Z' | 'a' ... 'z'
<name> = '<' <name-char>+ '>'
<initial-name-char> = 'A' ... 'Z' | 'a' ... 'z' | '_'
<name-char> = <initial-name-char> | '0' ... '9'
<assign-op> = '=' | '-=' | '+=' | '*=' | '/='
<whitespace> = ' ' | '\t'
<any-char> = any character except '\r' or '\n'
*/

import (
	"fmt"
	"io"
	"math"
	"runtime"
	"strconv"
)

type Code byte
type Name string
type Number float64
type String string

type Value interface {
	AsName() (Name, bool)
	AsNumber() (Number, bool)
	AsString() (String, bool)
}

type Dialect int

const (
	BeagleG Dialect = iota
	RepRap
	LinuxCNC
)

type Parser struct {
	Scanner io.ByteScanner
	Dialect Dialect

	// LineEndComment is called when line end comments are parsed: start with ; and %.
	LineEndComment func(comment string) error

	// InlineParsed is called when inline comments are parsed: delimited by ( and ).
	InlineParsed func(comment string) (int, string, error)

	// InlinedExecuted will be called if InlinedParsed returns non-zero or a non-empty string.
	InlineExecuted func(i int, s string) error

	// GetNumParam returns the value of a number parameter.
	GetNumParam func(num int) (Number, error)

	// SetNumParam sets the value of a number parameter.
	SetNumParam func(num int, val Number) error

	lineState    lineState
	physicalLine int // Count of lines
	virtualLine  int // Lines as tracked by Nnnn
	nameParams   map[Name]Value
}

const (
	minimumDelta = 0.0001
)

type lineState byte

const (
	beforeLineNum lineState = iota // Before Nnnn.
	afterLineNum                   // After Nnnn.
	inBody                         // After a code or an assignment is parsed.
	afterChecksum                  // After *nnn at the end of the line.
)

type endFunc func(p *Parser)

type action interface {
	evaluate(p *Parser, endFuncs []endFunc) (Code, Value, []endFunc)
}

type expression interface {
	evaluate(p *Parser) Value
}

const (
	nilCode   Code = 0
	firstCode Code = 'A'
	lastCode  Code = 'Z'
)

type assignOp byte

const (
	assign assignOp = iota
	assignPlus
	assignMinus
	assignTimes
	assignDivide
	plusPlus
	minusMinus
)

var (
	opPrecedence = [...]int{
		orOp:           1,
		andOp:          2,
		notOp:          3,
		equalOp:        4,
		notEqualOp:     4,
		greaterThanOp:  5,
		greaterEqualOp: 5,
		lessThanOp:     5,
		lessEqualOp:    5,
		subtractOp:     7,
		addOp:          7,
		divideOp:       8,
		multiplyOp:     8,
		negateOp:       9,
		noOp:           11,
	}

	calls = map[string]struct {
		fn      callFunc
		numArgs int
	}{
		"ABS":   {fn: abs, numArgs: 1},
		"ACOS":  {fn: acos, numArgs: 1},
		"ASIN":  {fn: asin, numArgs: 1},
		"ATAN":  {fn: atan, numArgs: 1},
		"CEIL":  {fn: ceil, numArgs: 1},
		"COS":   {fn: cos, numArgs: 1},
		"FLOOR": {fn: floor, numArgs: 1},
		"ROUND": {fn: round, numArgs: 1},
		"SIN":   {fn: sin, numArgs: 1},
		"SQRT":  {fn: sqrt, numArgs: 1},
		"TAN":   {fn: tan, numArgs: 1},
	}
)

type param struct {
	refs int
	expr expression
}

type op int

const (
	negateOp op = iota
	notOp
	noOp
	andOp
	orOp
	equalOp
	notEqualOp
	greaterThanOp
	greaterEqualOp
	lessThanOp
	lessEqualOp
	subtractOp
	addOp
	divideOp
	multiplyOp
)

type unary struct {
	op   op
	expr expression
}

type binary struct {
	op    op
	left  expression
	right expression
}

type callFunc func(p *Parser, args []Value) Value

type call struct {
	fn   callFunc
	args []expression
}

func (n Number) String() string {
	return strconv.FormatFloat(float64(n), 'f', 6, 64)
}

func (_ Number) AsName() (Name, bool) {
	return "", false
}

func (n Number) AsNumber() (Number, bool) {
	return n, true
}

func (_ Number) AsString() (String, bool) {
	return "", false
}

func (n Number) evaluate(p *Parser) Value {
	return n
}

func (n Name) String() string {
	return fmt.Sprintf("<%s>", string(n))
}

func (n Name) AsName() (Name, bool) {
	return n, true
}

func (_ Name) AsNumber() (Number, bool) {
	return 0, false
}

func (_ Name) AsString() (String, bool) {
	return "", false
}

func (n Name) evaluate(p *Parser) Value {
	return n
}

func (s String) String() string {
	return fmt.Sprintf(`"%s"`, string(s))
}

func (_ String) AsName() (Name, bool) {
	return "", false
}

func (_ String) AsNumber() (Number, bool) {
	return 0, false
}

func (s String) AsString() (String, bool) {
	return s, true
}

func (s String) evaluate(p *Parser) Value {
	return s
}

func (prm param) evaluate(p *Parser) Value {
	v := prm.expr.evaluate(p)
	if num, ok := v.AsNumber(); ok {
		if p.GetNumParam == nil {
			p.error("getting number parameters not supported")
		}
		for refs := 0; refs < prm.refs; refs += 1 {
			var err error
			num, err = p.GetNumParam(int(num))
			if err != nil {
				p.error(err.Error())
			}
		}
		return num
	}

	for refs := 0; refs < prm.refs; refs += 1 {
		n, ok := v.AsName()
		if !ok {
			p.error("expected a name parameter")
		}
		v, ok = p.nameParams[n]
		if !ok {
			p.error(fmt.Sprintf("undefined name parameter: %s", n))
		}
	}

	return v
}

func (u *unary) evaluate(p *Parser) Value {
	switch u.op {
	case negateOp:
		return -p.wantNumber(u.expr.evaluate(p))
	case notOp:
		n := p.wantNumber(u.expr.evaluate(p))
		if n == 0 {
			return Number(1)
		} else {
			return Number(0)
		}
	case noOp:
		return u.expr.evaluate(p)
	default:
		panic(fmt.Sprintf("unexpected unary op: %d", u.op))
	}
}

func (op op) precedence() int {
	return opPrecedence[op]
}

func logicNumber(n Number) Value {
	if n == 0 {
		return Number(0)
	} else {
		return Number(1)
	}
}

func logicBool(b bool) Value {
	if b {
		return Number(1)
	} else {
		return Number(0)
	}
}

func (b *binary) evaluate(p *Parser) Value {
	switch b.op {
	case andOp:
		n := p.wantNumber(b.left.evaluate(p))
		if n == 0 {
			return Number(0)
		}
		return logicNumber(p.wantNumber(b.right.evaluate(p)))
	case orOp:
		n := p.wantNumber(b.left.evaluate(p))
		if n != 0 {
			return Number(1)
		}
		return logicNumber(p.wantNumber(b.right.evaluate(p)))
	case equalOp:
		return logicBool(p.wantNumber(b.left.evaluate(p)) == p.wantNumber(b.right.evaluate(p)))
	case notEqualOp:
		return logicBool(p.wantNumber(b.left.evaluate(p)) != p.wantNumber(b.right.evaluate(p)))
	case greaterThanOp:
		return logicBool(p.wantNumber(b.left.evaluate(p)) > p.wantNumber(b.right.evaluate(p)))
	case greaterEqualOp:
		return logicBool(p.wantNumber(b.left.evaluate(p)) >= p.wantNumber(b.right.evaluate(p)))
	case lessThanOp:
		return logicBool(p.wantNumber(b.left.evaluate(p)) < p.wantNumber(b.right.evaluate(p)))
	case lessEqualOp:
		return logicBool(p.wantNumber(b.left.evaluate(p)) <= p.wantNumber(b.right.evaluate(p)))
	case subtractOp:
		return p.wantNumber(b.left.evaluate(p)) - p.wantNumber(b.right.evaluate(p))
	case addOp:
		return p.wantNumber(b.left.evaluate(p)) + p.wantNumber(b.right.evaluate(p))
	case divideOp:
		return p.wantNumber(b.left.evaluate(p)) / p.wantNumber(b.right.evaluate(p))
	case multiplyOp:
		return p.wantNumber(b.left.evaluate(p)) * p.wantNumber(b.right.evaluate(p))
	default:
		panic(fmt.Sprintf("unexpected binary op: %d", b.op))
	}
}

func (c *call) evaluate(p *Parser) Value {
	args := make([]Value, len(c.args))
	for i, a := range c.args {
		args[i] = a.evaluate(p)
	}
	return c.fn(p, args)
}

func (p *Parser) Parse() (code Code, val Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
			code = 0
			val = nil
		}
	}()

	var endFuncs []endFunc
	for {
		act := p.parse()

		var code Code
		var val Value
		code, val, endFuncs = act.evaluate(p, endFuncs)
		if code >= firstCode && code <= lastCode {
			return code, val, nil
		}
	}
}

func abs(p *Parser, args []Value) Value {
	return Number(math.Abs(float64(p.wantNumber(args[0]))))
}

func toDegrees(r float64) Number {
	return Number((r * 180) / math.Pi)
}

func toRadians(d Number) float64 {
	return (float64(d) * math.Pi) / 180
}

func acos(p *Parser, args []Value) Value {
	return toDegrees(math.Acos(float64(p.wantNumber(args[0]))))
}

func asin(p *Parser, args []Value) Value {
	return toDegrees(math.Asin(float64(p.wantNumber(args[0]))))
}

func atan(p *Parser, args []Value) Value {
	return toDegrees(math.Atan(float64(p.wantNumber(args[0]))))
}

func ceil(p *Parser, args []Value) Value {
	return Number(math.Ceil(float64(p.wantNumber(args[0]))))
}

func cos(p *Parser, args []Value) Value {
	return Number(math.Cos(toRadians(p.wantNumber(args[0]))))
}

func floor(p *Parser, args []Value) Value {
	return Number(math.Floor(float64(p.wantNumber(args[0]))))
}

func round(p *Parser, args []Value) Value {
	return Number(math.Round(float64(p.wantNumber(args[0]))))
}

func sin(p *Parser, args []Value) Value {
	return Number(math.Sin(toRadians(p.wantNumber(args[0]))))
}

func sqrt(p *Parser, args []Value) Value {
	return Number(math.Sqrt(float64(p.wantNumber(args[0]))))
}

func tan(p *Parser, args []Value) Value {
	return Number(math.Tan(toRadians(p.wantNumber(args[0]))))
}

func (p *Parser) wantNumber(v Value) Number {
	n, ok := v.AsNumber()
	if !ok {
		p.error("expected a number")
	}
	return n
}

func (p *Parser) error(msg string) {
	panic(fmt.Errorf("%s: %s", p.where(), msg))
}

func (p *Parser) readByte() byte {
	b, err := p.Scanner.ReadByte()
	if err != nil {
		if err == io.EOF {
			panic(err)
		}
		p.error(err.Error())
	}
	return b
}

func (p *Parser) unreadByte() {
	err := p.Scanner.UnreadByte()
	if err != nil {
		p.error(err.Error())
	}
}

func (p *Parser) where() string {
	if p.physicalLine == p.virtualLine {
		return fmt.Sprintf("%d", p.physicalLine)
	}
	return fmt.Sprintf("%d(%d)", p.physicalLine, p.virtualLine)
}

func (p *Parser) skipWhitespace() {
	for {
		b := p.readByte()
		if b != ' ' && b != '\t' {
			break
		}
	}
	p.unreadByte()
}

func (p *Parser) parseNumber() expression {
	var bytes []byte
	var neg bool
	b := p.readByte()
	if b == '-' {
		neg = true
	} else if b != '+' {
		bytes = append(bytes, b)
	}

	for {
		b := p.readByte()
		if b >= '0' && b <= '9' {
			bytes = append(bytes, b)
		} else if b == '.' {
			bytes = append(bytes, b)
			for {
				b := p.readByte()
				if b >= '0' && b <= '9' {
					bytes = append(bytes, b)
				} else {
					break
				}
			}
			break
		} else {
			break
		}
	}
	p.unreadByte()

	n, err := strconv.ParseFloat(string(bytes), 64)
	if err != nil {
		p.error("not a number")
	}

	if neg {
		n *= -1
	}
	return Number(n)
}

func (p *Parser) parseSubExpr() expression {
	p.skipWhitespace()
	b := p.readByte()

	var e expression
	switch b {
	case '-':
		// - <expr>
		e = &unary{op: negateOp, expr: p.parseSubExpr()}
	case '!':
		// ! <expr>
		e = &unary{op: notOp, expr: p.parseSubExpr()}
	case '[':
		// [ <expr> ]
		e = &unary{op: noOp, expr: p.parseSubExpr()}
		p.skipWhitespace()
		b = p.readByte()
		if b != ']' {
			p.error(fmt.Sprintf("expected closing brace, got %c", b))
		}
	case '#':
		e = p.parseReference()
	case '<':
		e = p.parseName()
	case '"':
		e = p.parseString()
	default:
		b = upcaseByte(b)
		if b >= 'A' && b <= 'Z' {
			sym := p.parseSymbol(b)
			if sym == "" {
				p.error("expected a function name")
			}

			fi, ok := calls[sym]
			if !ok {
				p.error(fmt.Sprintf("function not found: %s", sym))
			}
			p.skipWhitespace()
			b = p.readByte()
			if b != '[' {
				p.error(fmt.Sprintf("expected [ following function name; got %c", b))
			}
			c := call{fn: fi.fn}

			p.skipWhitespace()
			b = p.readByte()
			if b != ']' {
				p.unreadByte()
				for {
					c.args = append(c.args, p.parseSubExpr())
					p.skipWhitespace()
					b = p.readByte()
					if b == ']' {
						break
					} else if b != ',' {
						p.error("expected a comma (,) between arguments")
					}
				}
			}
			if len(c.args) != fi.numArgs {
				p.error(
					fmt.Sprintf("wrong number of arguments to function %s: got %d, want %d",
						sym, len(c.args), fi.numArgs))
			}
			e = &c
		} else {
			p.unreadByte()
			e = p.parseNumber()
		}
	}

	var op op
	p.skipWhitespace()
	b = p.readByte()
	switch b {
	case '+':
		op = addOp
	case '-':
		op = subtractOp
	case '*':
		op = multiplyOp
	case '/':
		op = divideOp
	case '=':
		b = p.readByte()
		if b != '=' {
			p.error(fmt.Sprintf("expected ==, got =%c", b))
		}
		op = equalOp
	case '!':
		b = p.readByte()
		if b != '=' {
			p.error(fmt.Sprintf("expected !=, got !%c", b))
		}
		op = notEqualOp
	case '<':
		b = p.readByte()
		if b == '=' {
			op = lessEqualOp
		} else {
			p.unreadByte()
			op = lessThanOp
		}
	case '>':
		b = p.readByte()
		if b == '=' {
			op = greaterEqualOp
		} else {
			p.unreadByte()
			op = greaterThanOp
		}
	case '&':
		b = p.readByte()
		if b != '&' {
			p.error(fmt.Sprintf("expected &&, got &%c", b))
		}
		op = andOp
	case '|':
		b = p.readByte()
		if b != '|' {
			p.error(fmt.Sprintf("expected ||, got |%c", b))
		}
		op = orOp
	default:
		p.unreadByte()
		return e
	}

	return &binary{op: op, left: e, right: p.parseSubExpr()}
}

func adjustPrecedence(e expression) expression {
	switch e := e.(type) {
	case *unary:
		e.expr = adjustPrecedence(e.expr)
		if e.op == noOp {
			return e
		}

		// - [2 * 3]  --> [- 2] * 3
		if b, ok := e.expr.(*binary); ok && b.op.precedence() < e.op.precedence() {
			e.expr = b.left
			b.left = e
			return adjustPrecedence(b)
		}
	case *binary:
		e.left = adjustPrecedence(e.left)
		e.right = adjustPrecedence(e.right)

		// 1 * [2 + 3] --> [1 * 2] + 3
		if b, ok := e.right.(*binary); ok && b.op.precedence() <= e.op.precedence() {
			e.right = b.left
			b.left = e
			return adjustPrecedence(b)
		}

		// [1 + 2] * 3 --> 1 + [2 * 3]
		if b, ok := e.left.(*binary); ok && b.op.precedence() < e.op.precedence() {
			e.left = b.right
			b.right = e
			return adjustPrecedence(b)
		}
	case *call:
		for i, a := range e.args {
			e.args[i] = adjustPrecedence(a)
		}
	}

	return e
}

func (p *Parser) parseReference() expression {
	// #+ <param>
	refs := 1
	b := p.readByte()
	for b == '#' {
		refs += 1
		b = p.readByte()
	}
	p.unreadByte()

	if b == '[' {
		return param{refs: refs, expr: p.parseExpr()}
	}

	return param{refs: refs, expr: p.parseParameter().(expression)}
}

func (p *Parser) parseExpr() expression {
	p.skipWhitespace()
	b := p.readByte()
	switch b {
	case '#':
		return p.parseReference()
	case '[':
		e := adjustPrecedence(p.parseSubExpr())
		p.skipWhitespace()
		b = p.readByte()
		if b != ']' {
			p.error(fmt.Sprintf("expected closing brace, got %c", b))
		}
		return e
	case '<':
		return p.parseName()
	case '"':
		return p.parseString()
	default:
		p.unreadByte()
		return p.parseNumber()
	}
}

func (p *Parser) wantInteger() int {
	var num int64
	var cnt int
	for {
		b := p.readByte()
		if b >= '0' && b <= '9' {
			cnt += 1
			num = (num * 10) + int64(b-'0')
			if num > math.MaxInt32 {
				p.error("number too big")
			}
		} else {
			break
		}
	}

	if cnt == 0 {
		p.error("expected a number")
	}

	p.unreadByte()
	return int(num)
}

func symbolByte(b byte) bool {
	return b >= 'A' && b <= 'Z'
}

func (p *Parser) parseSymbol(b byte) string {
	n := upcaseByte(p.readByte())
	if !symbolByte(n) {
		p.unreadByte()
		return ""
	}
	symbol := []byte{b, n}
	for {
		n = upcaseByte(p.readByte())
		if !symbolByte(n) {
			break
		}
		symbol = append(symbol, n)
	}
	p.unreadByte()
	return string(symbol)
}

func upcaseByte(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return (b - 'a') + 'A'
	}
	return b
}

func nameByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_'
}

func (p *Parser) parseParameter() Value {
	b := p.readByte()

	if b >= '0' && b <= '9' {
		num := int(b - '0')
		for {
			b = p.readByte()
			if b < '0' || b > '9' {
				break
			}
			num = num*10 + int(b-'0')
			if num > math.MaxInt16 {
				p.error("number parameter too big")
			}
		}

		p.Scanner.UnreadByte()
		return Number(num)
	} else if (p.Dialect == BeagleG && nameByte(b)) || b == '<' {
		var delim bool
		if b == '<' {
			delim = true
			b = p.readByte()
		}

		name := []byte{b}
		for {
			b = p.readByte()
			if nameByte(b) {
				name = append(name, b)
			} else {
				break
			}
		}

		if delim {
			if b != '>' {
				p.error("missing > at end of parameter")
			}
		} else {
			p.Scanner.UnreadByte()
		}

		return Name(name)
	}

	p.error(fmt.Sprintf("expected parameter name or number; got %c", b))
	return nil
}

func (p *Parser) parseString() String {
	var b byte
	var s []byte
	for {
		b = p.readByte()
		if b == '\n' || b == '\r' {
			p.error("strings may not contain newlines")
		}

		if b == '"' {
			break
		} else if b == '\\' {
			b = p.readByte()
		}
		s = append(s, b)
	}
	return String(s)
}

func (p *Parser) parseName() Name {
	var b byte
	var name []byte
	for {
		b = p.readByte()
		if nameByte(b) {
			name = append(name, b)
		} else {
			break
		}
	}

	if b != '>' {
		p.error("missing > at end of parameter")
	} else if len(name) == 0 {
		p.error("empty names not allowed")
	}
	return Name(name)
}

func (p *Parser) parseAssignOp() assignOp {
	p.skipWhitespace()
	b := p.readByte()
	if b == '=' {
		return assign
	}

	if b == '-' || b == '+' || b == '*' || b == '/' {
		n := p.readByte()
		switch b {
		case '-':
			if n == '-' {
				return minusMinus
			} else if n == '=' {
				return assignMinus
			}
		case '+':
			if n == '+' {
				return plusPlus
			} else if n == '=' {
				return assignPlus
			}

		case '*':
			if n == '=' {
				return assignTimes
			}
		case '/':
			if n == '=' {
				return assignDivide
			}
		}
	}

	p.error("expected an assignment operator (=, +=, -=, *=, /=, ++, --)")
	return 0
}

func (p *Parser) parseAssignment() action {
	param := p.parseParameter()
	assignOp := p.parseAssignOp()
	var expr expression
	if assignOp == plusPlus || assignOp == minusMinus {
		expr = Number(1)
	} else {
		expr = p.parseExpr()
	}
	if num, ok := param.(Number); ok {
		return numAssignAction{num: int(num), assignOp: assignOp, expr: expr}
	}
	return nameAssignAction{name: param.(Name), assignOp: assignOp, expr: expr}

}

func (p *Parser) wantEndOfLine() {
	p.skipWhitespace()
	b := p.readByte()
	// XXX: handle end of line comment
	if b != '\n' && b != '\r' {
		p.error("expected end of line")
	}
}

func (p *Parser) parseExprThenAssignBeagleG() (expression, action) {
	// <expr> 'THEN' <assignment>

	ifTest := p.parseExpr()

	p.skipWhitespace()
	b := upcaseByte(p.readByte())
	if b != 'T' || p.parseSymbol(b) != "THEN" {
		p.error("expected keyword THEN")
	}

	p.skipWhitespace()
	if p.readByte() != '#' {
		p.error("expected an assignment")
	}
	thenAssign := p.parseAssignment()

	return ifTest, thenAssign
}

func (p *Parser) parseIfBeagleG() action {
	// 'IF' <expr> 'THEN' <assignment> ('ELSEIF' <expr> 'THEN' <assignment>)* ['ELSE' <assignment>]

	ifTest, thenAssign := p.parseExprThenAssignBeagleG()

	var elseifTests []expression
	var elseifAssigns []action
	var elseAssign action
	for {
		p.skipWhitespace()
		b := p.readByte()
		if b == '\n' || b == '\r' {
			break
		}
		b = upcaseByte(b)
		if b != 'E' {
			p.error("expected keyword ELSEIF or ELSE")
		}
		kw := p.parseSymbol(b)
		if kw == "ELSEIF" {
			test, assign := p.parseExprThenAssignBeagleG()
			elseifTests = append(elseifTests, test)
			elseifAssigns = append(elseifAssigns, assign)
		} else if kw == "ELSE" {
			p.skipWhitespace()
			if p.readByte() != '#' {
				p.error("expected an assignment")
			}
			elseAssign = p.parseAssignment()
			p.wantEndOfLine()
			break
		} else {
			p.error("expected keyword ELSEIF or ELSE")
		}
	}

	return ifActionBeagleG{
		ifTest:        ifTest,
		thenAssign:    thenAssign,
		elseifTests:   elseifTests,
		elseifAssigns: elseifAssigns,
		elseAssign:    elseAssign,
	}
}

type codeAction struct {
	code Code
	expr expression
}

func (ca codeAction) evaluate(p *Parser, endFuncs []endFunc) (Code, Value, []endFunc) {
	return ca.code, ca.expr.evaluate(p), endFuncs
}

type numAssignAction struct {
	num      int
	assignOp assignOp
	expr     expression
}

func numAssignment(p *Parser, num int, assignOp assignOp, val Number) {
	switch assignOp {
	case assign:
		err := p.SetNumParam(num, val)
		if err != nil {
			p.error(err.Error())
		}
	case assignPlus, plusPlus:
		cur, err := p.GetNumParam(num)
		if err != nil {
			p.error(err.Error())
		}
		err = p.SetNumParam(num, cur+val)
		if err != nil {
			p.error(err.Error())
		}
	case assignMinus, minusMinus:
		cur, err := p.GetNumParam(num)
		if err != nil {
			p.error(err.Error())
		}
		err = p.SetNumParam(num, cur-val)
		if err != nil {
			p.error(err.Error())
		}
	case assignTimes:
		cur, err := p.GetNumParam(num)
		if err != nil {
			p.error(err.Error())
		}
		err = p.SetNumParam(num, cur*val)
		if err != nil {
			p.error(err.Error())
		}
	case assignDivide:
		cur, err := p.GetNumParam(num)
		if err != nil {
			p.error(err.Error())
		}
		err = p.SetNumParam(num, cur/val)
		if err != nil {
			p.error(err.Error())
		}
	default:
		panic(fmt.Sprintf("unexpected assign op: %d", assignOp))
	}
}

func (naa numAssignAction) evaluate(p *Parser, endFuncs []endFunc) (Code, Value, []endFunc) {
	if p.GetNumParam == nil || p.SetNumParam == nil {
		p.error("setting number parameters not supported")
	}

	if p.Dialect == LinuxCNC {
		return nilCode, nil, append(endFuncs,
			func(p *Parser) {
				numAssignment(p, naa.num, naa.assignOp, p.wantNumber(naa.expr.evaluate(p)))
			})
	} else {
		numAssignment(p, naa.num, naa.assignOp, p.wantNumber(naa.expr.evaluate(p)))

		return nilCode, nil, endFuncs
	}
}

type nameAssignAction struct {
	name     Name
	assignOp assignOp
	expr     expression
}

func nameAssignment(p *Parser, name Name, assignOp assignOp, val Value) {
	if p.nameParams == nil {
		p.nameParams = map[Name]Value{}
	}

	switch assignOp {
	case assign:
		p.nameParams[name] = val
	case assignPlus, plusPlus:
		cur, ok := p.nameParams[name]
		if !ok {
			p.error(fmt.Sprintf("undefined name parameter: %s", name))
		}
		p.nameParams[name] = p.wantNumber(cur) + p.wantNumber(val)
	case assignMinus, minusMinus:
		cur, ok := p.nameParams[name]
		if !ok {
			p.error(fmt.Sprintf("undefined name parameter: %s", name))
		}
		p.nameParams[name] = p.wantNumber(cur) - p.wantNumber(val)
	case assignTimes:
		cur, ok := p.nameParams[name]
		if !ok {
			p.error(fmt.Sprintf("undefined name parameter: %s", name))
		}
		p.nameParams[name] = p.wantNumber(cur) * p.wantNumber(val)
	case assignDivide:
		cur, ok := p.nameParams[name]
		if !ok {
			p.error(fmt.Sprintf("undefined name parameter: %s", name))
		}
		p.nameParams[name] = p.wantNumber(cur) / p.wantNumber(val)
	default:
		panic(fmt.Sprintf("unexpected assign op: %d", assignOp))
	}
}

func (naa nameAssignAction) evaluate(p *Parser, endFuncs []endFunc) (Code, Value, []endFunc) {
	if p.Dialect == LinuxCNC {
		return nilCode, nil, append(endFuncs,
			func(p *Parser) {
				nameAssignment(p, naa.name, naa.assignOp, naa.expr.evaluate(p))
			})
	} else {
		nameAssignment(p, naa.name, naa.assignOp, naa.expr.evaluate(p))

		return nilCode, nil, endFuncs
	}
}

type commentAction struct {
	intVal    int
	stringVal string
}

func (ca commentAction) evaluate(p *Parser, endFuncs []endFunc) (Code, Value, []endFunc) {
	err := p.InlineExecuted(ca.intVal, ca.stringVal)
	if err != nil {
		p.error(err.Error())
	}
	return nilCode, nil, endFuncs
}

type ifActionBeagleG struct {
	ifTest        expression
	thenAssign    action
	elseifTests   []expression
	elseifAssigns []action
	elseAssign    action
}

func (ia ifActionBeagleG) evaluate(p *Parser, endFuncs []endFunc) (Code, Value, []endFunc) {
	if math.Abs(float64(p.wantNumber(ia.ifTest.evaluate(p)))) >= minimumDelta {
		return ia.thenAssign.evaluate(p, endFuncs)
	}

	for edx := range ia.elseifTests {
		if math.Abs(float64(p.wantNumber(ia.elseifTests[edx].evaluate(p)))) >= minimumDelta {
			return ia.elseifAssigns[edx].evaluate(p, endFuncs)
		}
	}

	if ia.elseAssign != nil {
		return ia.elseAssign.evaluate(p, endFuncs)
	}
	return nilCode, nil, endFuncs
}

type eolAction struct{}

func (ea eolAction) evaluate(p *Parser, endFuncs []endFunc) (Code, Value, []endFunc) {
	for _, efn := range endFuncs {
		efn(p)
	}
	return nilCode, nil, nil
}

func (p *Parser) parse() action {
	for {
		p.skipWhitespace()
		b := upcaseByte(p.readByte())

		if b == '\n' || b == '\r' {
			p.lineState = beforeLineNum
			p.physicalLine += 1
			p.virtualLine += 1

			return eolAction{}
		} else if b == ';' || b == '%' {
			var bytes []byte
			for {
				b := p.readByte()
				if b == '\n' || b == '\r' {
					p.lineState = beforeLineNum
					p.physicalLine += 1
					p.virtualLine += 1
					break
				}
				bytes = append(bytes, b)
			}

			if p.LineEndComment != nil {
				err := p.LineEndComment(string(bytes))
				if err != nil {
					p.error(err.Error())
				}
			}

			return eolAction{}
		} else if b == '(' {
			var bytes []byte
			for {
				b := p.readByte()
				if b == '\n' || b == '\r' {
					p.error("inline comments must be on one line")
				}
				if b == ')' {
					break
				}
				bytes = append(bytes, b)
			}

			if p.InlineParsed != nil {
				i, s, err := p.InlineParsed(string(bytes))
				if err != nil {
					p.error(err.Error())
				}
				if p.InlineExecuted != nil && (i != 0 || s != "") {
					return commentAction{intVal: i, stringVal: s}
				}
			}
		} else if b == '*' {
			// Parse and ignore *nnn; check it is the last command on the line.

			p.wantInteger()
			p.lineState = afterChecksum
		} else if b == '#' {
			if p.lineState == afterChecksum {
				p.error("checksum (*nnn) must be at end of line")
			}
			p.lineState = inBody

			return p.parseAssignment()
		} else if b < 'A' || b > 'Z' {
			p.error(fmt.Sprintf("unexpected command: %d", b))
		} else if kw := p.parseSymbol(b); kw != "" {
			if p.lineState == afterChecksum {
				p.error("checksum (*nnn) must be at end of line")
			}
			if p.lineState == inBody {
				p.error("keyword must come first on line")
			}
			p.lineState = inBody

			if p.Dialect == BeagleG {
				switch kw {
				case "WHILE":
					// return p.parseWhileBeagleG()
				case "IF":
					return p.parseIfBeagleG()
				}
			}

			p.error("unexpected keyword")
		} else if b == 'N' {
			// Parse Nnnn.

			if p.lineState != beforeLineNum {
				p.error("N code must be first on line")
			}

			num := p.wantInteger()
			if num <= p.virtualLine {
				p.error(fmt.Sprintf("N%d invalid", num))
			}
			p.virtualLine = num - 1

			p.lineState = afterLineNum
		} else {
			if p.lineState == afterChecksum {
				p.error("checksum (*nnn) must be at end of line")
			}
			p.lineState = inBody

			// Parse all the other codes (A to Z except N).
			return codeAction{code: Code(b), expr: p.parseExpr()}
		}
	}
}

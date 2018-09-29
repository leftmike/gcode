package gcode

import (
	"fmt"
	"io"
	"math"
	"runtime"
)

// XXX: use fixed point numbers or float64?
type Number int64
type Code byte

type Parser struct {
	Scanner io.ByteScanner

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

	withinLine   bool // Used to validate that Nnnn is at the beginning of a line
	sawChecksum  bool // Used to validate that *nnn is at the end of a line
	physicalLine int  // Count of lines
	virtualLine  int  // Lines as tracked by Nnnn
	nameParams   map[string]Number
}

type expression interface {
	evaluate(p *Parser) Number
}

type commandType byte

const (
	assignmentType commandType = iota + 1
	commentType
	whileKeyword
	doKeyword
	endKeyword
	ifKeyword
	thenKeyword
	elseKeyword
	elseifKeyword

	firstCodeType commandType = 'A'
	lastCodeType  commandType = 'Z'
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

type command struct {
	typ       commandType
	assignOp  assignOp
	expr      expression
	intVal    int    // num parameter
	stringVal string // name parameter
}

var (
	keywordMap = map[string]commandType{
		"WHILE":  whileKeyword,
		"DO":     doKeyword,
		"END":    endKeyword,
		"IF":     ifKeyword,
		"THEN":   thenKeyword,
		"ELSE":   elseKeyword,
		"ELSEIF": elseifKeyword,
	}

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
		"ABS": {fn: abs, numArgs: 1},
	}
)

type numParam int
type nameParam string

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

type callFunc func(p *Parser, args []Number) Number

type call struct {
	fn   callFunc
	args []expression
}

func (n Number) evaluate(p *Parser) Number {
	return n
}

func (np numParam) evaluate(p *Parser) Number {
	if p.GetNumParam == nil || p.SetNumParam == nil {
		p.error("number parameters not supported")
	}
	num, err := p.GetNumParam(int(np))
	if err != nil {
		p.error(err.Error())
	}
	return num
}

func (np nameParam) evaluate(p *Parser) Number {
	val, ok := p.nameParams[string(np)]
	if !ok {
		p.error(fmt.Sprintf("undefined name parameter: %s", string(np)))
	}
	return val
}

func (u *unary) evaluate(p *Parser) Number {
	switch u.op {
	case negateOp:
		return -u.expr.evaluate(p)
	case notOp:
		n := u.expr.evaluate(p)
		if n == 0 {
			return 1
		} else {
			return 0
		}
	case noOp:
		return u.expr.evaluate(p)
	default:
		panic(fmt.Sprintf("unexpected unary op: %d", u.op))
	}

	return 0
}

func (op op) precedence() int {
	return opPrecedence[op]
}

func logicNumber(n Number) Number {
	if n == 0 {
		return 0
	} else {
		return 1
	}
}

func logicBool(b bool) Number {
	if b {
		return 1
	} else {
		return 0
	}
}

func (b *binary) evaluate(p *Parser) Number {
	switch b.op {
	case andOp:
		n := b.left.evaluate(p)
		if n == 0 {
			return 0
		}
		return logicNumber(b.right.evaluate(p))
	case orOp:
		n := b.left.evaluate(p)
		if n != 0 {
			return 1
		}
		return logicNumber(b.right.evaluate(p))
	case equalOp:
		return logicBool(b.left.evaluate(p) == b.right.evaluate(p))
	case notEqualOp:
		return logicBool(b.left.evaluate(p) != b.right.evaluate(p))
	case greaterThanOp:
		return logicBool(b.left.evaluate(p) > b.right.evaluate(p))
	case greaterEqualOp:
		return logicBool(b.left.evaluate(p) >= b.right.evaluate(p))
	case lessThanOp:
		return logicBool(b.left.evaluate(p) < b.right.evaluate(p))
	case lessEqualOp:
		return logicBool(b.left.evaluate(p) <= b.right.evaluate(p))
	case subtractOp:
		return b.left.evaluate(p) - b.right.evaluate(p)
	case addOp:
		return b.left.evaluate(p) + b.right.evaluate(p)
	case divideOp:
		return b.left.evaluate(p) / b.right.evaluate(p)
	case multiplyOp:
		return b.left.evaluate(p) * b.right.evaluate(p)
	default:
		panic(fmt.Sprintf("unexpected binary op: %d", b.op))
	}

	return 0
}

func (c *call) evaluate(p *Parser) Number {
	args := make([]Number, len(c.args))
	for i, a := range c.args {
		args[i] = a.evaluate(p)
	}
	return c.fn(p, args)
}

func (p *Parser) Parse() (code Code, num Number, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
			code = 0
			num = 0
		}
	}()

	for {
		var cmd command
		p.parse(&cmd)
		if cmd.typ >= firstCodeType && cmd.typ <= lastCodeType {
			// Codes

			num := cmd.expr.evaluate(p)
			return Code(cmd.typ), num, nil
		} else if cmd.typ == assignmentType {
			// Assignments

			p.evaluateAssignment(&cmd)
		} else if cmd.typ == commentType {
			// Inline Comments

			err = p.InlineExecuted(cmd.intVal, cmd.stringVal)
			if err != nil {
				p.error(err.Error())
			}
		} else {
			// Keywords

			p.error(fmt.Sprintf("keyword %d not implemented", cmd.typ)) // XXX
		}
	}
}

func abs(p *Parser, args []Number) Number {
	if args[0] < 0 {
		return -args[0]
	}
	return args[0]
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
		if b == '\n' || b == '\r' {
			p.withinLine = false
			p.sawChecksum = false
			p.physicalLine += 1
			p.virtualLine += 1
		} else if b != ' ' && b != '\t' {
			break
		}
	}
	p.unreadByte()
}

func (p *Parser) parseNumber() expression {
	var neg bool
	b := p.readByte()
	if b == '-' {
		neg = true
	} else if b != '+' {
		p.Scanner.UnreadByte()
	}

	var whole uint64
	var fraction uint64
	var cnt int
	for {
		b := p.readByte()
		if b >= '0' && b <= '9' {
			cnt += 1
			whole = (whole * 10) + uint64(b-'0')
			// XXX: check for overflow
		} else if b == '.' {
			for {
				b := p.readByte()
				if b >= '0' && b <= '9' {
					cnt += 1
					fraction = (fraction * 10) + uint64(b-'0')
					// XXX: check for overflow
				} else {
					break
				}
			}
			break
		} else {
			break
		}
	}

	if cnt == 0 {
		p.error("expected a number")
	}

	// XXX: also need the fractional part
	num := Number(whole)
	if neg {
		num *= -1
	}
	p.unreadByte()
	return num
}

/*
<expr> = <num>
    | '-' <expr>
    | '!' <expr>
    | '[' <expr> ']'
    | <expr> <op> <expr>
    | '#' <param>
    | <func> '[' [<expr> [',' ...]] ']'
<op> = '+' '-' '*' '/'
    | '==' '!=' '<' '<=' '>' '>='
    | '&&' '||'
*/

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
		// # <param>
		num, name := p.parseParameter()
		if name == "" {
			e = numParam(num)
		} else {
			e = nameParam(name)
		}
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
			return &c
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

func (p *Parser) parseExpr() expression {
	p.skipWhitespace()
	b := p.readByte()
	switch b {
	case '#':
		num, name := p.parseParameter()
		if name == "" {
			return numParam(num)
		} else {
			return nameParam(name)
		}
	case '[':
		e := adjustPrecedence(p.parseSubExpr())
		p.skipWhitespace()
		b = p.readByte()
		if b != ']' {
			p.error(fmt.Sprintf("expected closing brace, got %c", b))
		}
		return e
	default:
		p.unreadByte()
		return p.parseNumber()
	}
	return nil
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

func parameterByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_'
}

func (p *Parser) parseParameter() (int, string) {
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
		return num, ""
	} else if parameterByte(b) || b == '<' {
		var delim bool
		if b == '<' {
			delim = true
			b = p.readByte()
			if b >= '0' && b <= '9' {
				p.error("bracketed (#<>) numeric parameters not allowed")
			}
		}

		name := []byte{b}
		for {
			b = p.readByte()
			if parameterByte(b) {
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

		return 0, string(name)
	}

	p.error(fmt.Sprintf("expected parameter name or number; got %c", b))
	return 0, ""
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

func (p *Parser) evaluateAssignment(cmd *command) {
	val := cmd.expr.evaluate(p)

	if cmd.stringVal == "" {
		if p.GetNumParam == nil || p.SetNumParam == nil {
			p.error("number parameters not supported")
		}

		switch cmd.assignOp {
		case assign:
			err := p.SetNumParam(cmd.intVal, val)
			if err != nil {
				p.error(err.Error())
			}
		case assignPlus, plusPlus:
			cur, err := p.GetNumParam(cmd.intVal)
			if err != nil {
				p.error(err.Error())
			}
			err = p.SetNumParam(cmd.intVal, cur+val)
			if err != nil {
				p.error(err.Error())
			}
		case assignMinus, minusMinus:
			cur, err := p.GetNumParam(cmd.intVal)
			if err != nil {
				p.error(err.Error())
			}
			err = p.SetNumParam(cmd.intVal, cur-val)
			if err != nil {
				p.error(err.Error())
			}
		case assignTimes:
			cur, err := p.GetNumParam(cmd.intVal)
			if err != nil {
				p.error(err.Error())
			}
			err = p.SetNumParam(cmd.intVal, cur*val)
			if err != nil {
				p.error(err.Error())
			}
		case assignDivide:
			cur, err := p.GetNumParam(cmd.intVal)
			if err != nil {
				p.error(err.Error())
			}
			err = p.SetNumParam(cmd.intVal, cur/val)
			if err != nil {
				p.error(err.Error())
			}
		default:
			panic(fmt.Sprintf("unexpected assign op: %d", cmd.assignOp))
		}
	} else {
		if p.nameParams == nil {
			p.nameParams = map[string]Number{}
		}

		switch cmd.assignOp {
		case assign:
			p.nameParams[cmd.stringVal] = val
		case assignPlus, plusPlus:
			cur, ok := p.nameParams[cmd.stringVal]
			if !ok {
				p.error(fmt.Sprintf("undefined name parameter: %s", cmd.stringVal))
			}
			p.nameParams[cmd.stringVal] = cur + val
		case assignMinus, minusMinus:
			cur, ok := p.nameParams[cmd.stringVal]
			if !ok {
				p.error(fmt.Sprintf("undefined name parameter: %s", cmd.stringVal))
			}
			p.nameParams[cmd.stringVal] = cur - val
		case assignTimes:
			cur, ok := p.nameParams[cmd.stringVal]
			if !ok {
				p.error(fmt.Sprintf("undefined name parameter: %s", cmd.stringVal))
			}
			p.nameParams[cmd.stringVal] = cur * val
		case assignDivide:
			cur, ok := p.nameParams[cmd.stringVal]
			if !ok {
				p.error(fmt.Sprintf("undefined name parameter: %s", cmd.stringVal))
			}
			p.nameParams[cmd.stringVal] = cur / val
		default:
			panic(fmt.Sprintf("unexpected assign op: %d", cmd.assignOp))
		}
	}
}

func (p *Parser) parse(cmd *command) {
	for {
		p.skipWhitespace()
		b := upcaseByte(p.readByte())

		if b == ';' || b == '%' {
			var bytes []byte
			for {
				b := p.readByte()
				if b == '\n' || b == '\r' {
					p.withinLine = false
					p.sawChecksum = false
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
					cmd.typ = commentType
					cmd.intVal = i
					cmd.stringVal = s
					break
				}
			}
		} else if b == '*' {
			// Parse and ignore *nnn; check it is the last command on the line.

			p.wantInteger()
			p.sawChecksum = true
			p.withinLine = true
		} else if b == '#' {
			if p.sawChecksum {
				p.error("checksum (*nnn) must be at end of line")
			}

			cmd.typ = assignmentType
			cmd.intVal, cmd.stringVal = p.parseParameter()
			cmd.assignOp = p.parseAssignOp()
			if cmd.assignOp == plusPlus || cmd.assignOp == minusMinus {
				cmd.expr = Number(1)
			} else {
				cmd.expr = p.parseExpr()
			}

			p.withinLine = true
			break
		} else if b < 'A' || b > 'Z' {
			p.error(fmt.Sprintf("unexpected command: %d", b))
		} else if kw := p.parseSymbol(b); kw != "" {
			if p.sawChecksum {
				p.error("checksum (*nnn) must be at end of line")
			}

			if typ, ok := keywordMap[kw]; !ok {
				p.error(fmt.Sprintf("unexpected keyword: %s", kw))
			} else {
				cmd.typ = typ

				p.withinLine = true
				break
			}
		} else if b == 'N' {
			// Parse Nnnn.

			if p.withinLine {
				p.error("N code must be first on line")
			}

			num := p.wantInteger()
			if num <= p.virtualLine {
				p.error(fmt.Sprintf("N%d invalid", num))
			}
			p.virtualLine = num - 1

			p.withinLine = true
		} else {
			// Parse all the other codes (A to Z except N).
			cmd.typ = commandType(b)

			if p.sawChecksum {
				p.error("checksum (*nnn) must be at end of line")
			}

			cmd.expr = p.parseExpr()
			p.withinLine = true
			break
		}
	}
}

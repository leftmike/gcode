package gcode

import (
	"errors"
	"fmt"
	"io"
	"math"
	"runtime"
	"strings"
	"testing"
)

type cmd struct {
	code Code
	num  Number
}

func TestParser(t *testing.T) {
	cases := []struct {
		s    string
		fail bool
		cmds []cmd
	}{
		{s: "G10\n", cmds: []cmd{{'G', 10}}},
		{s: "g10\n", cmds: []cmd{{'G', 10}}},
		{s: " G 10\n", cmds: []cmd{{'G', 10}}},
		{s: "G\n10\n", cmds: []cmd{{'G', 10}}},
		{s: "(comment)G10\n", cmds: []cmd{{'G', 10}}},
		{s: "(comment) G10\n", cmds: []cmd{{'G', 10}}},
		{s: "(comment\n) G10\n", fail: true},
		{s: "; comment\nG10\n", cmds: []cmd{{'G', 10}}},
		{s: "% comment\nG10\n", cmds: []cmd{{'G', 10}}},
		{s: "G;comment\n10\n", fail: true},
		{s: "G%comment\n10\n", fail: true},
		{s: "G(comment)10\n", fail: true},
		{s: "GG\n", fail: true},
		{s: "$$$\n", fail: true},
		{s: "G-10\n", cmds: []cmd{{'G', -10}}},
		{s: "G+10\n", cmds: []cmd{{'G', 10}}},
		{s: "G+\n", fail: true},
		{s: "G-\n", fail: true},
		{s: "G+.\n", fail: true},
		{s: "G-.\n", fail: true},
		{s: "G.\n", fail: true},
		{s: "G+0\n", cmds: []cmd{{'G', 0}}},
		{s: "G-0\n", cmds: []cmd{{'G', 0}}},
		{s: "G+.0\n", cmds: []cmd{{'G', 0}}},
		{s: "G-.0\n", cmds: []cmd{{'G', 0}}},
		{s: "G+0.\n", cmds: []cmd{{'G', 0}}},
		{s: "G-0.\n", cmds: []cmd{{'G', 0}}},
		{s: "G0.\n", cmds: []cmd{{'G', 0}}},
		{s: "G.0\n", cmds: []cmd{{'G', 0}}},
		{s: "G-10.20\n", cmds: []cmd{{'G', -10.20}}},
		{s: "G+10.20\n", cmds: []cmd{{'G', 10.20}}},

		{s: "G10 *20\n", cmds: []cmd{{'G', 10}}},
		{s: "G10 *20 ;comment\nG30\n", cmds: []cmd{{'G', 10}, {'G', 30}}},
		{s: "G10 *20 G30\n", cmds: []cmd{{'G', 10}}, fail: true},

		{s: "N10 G20\n", cmds: []cmd{{'G', 20}}},
		{s: "G10\nG20\n\nN1 G2\n", fail: true,
			cmds: []cmd{{'G', 10}, {'G', 20}}},
		{s: "N10 G-\n", fail: true},
		{s: "N9999999999999999 G10\n", fail: true},
		{s: "*123 G10\n", fail: true},
		{s: "*123 WHILE\n", fail: true},
		{s: "    G10X1Y 2Z3\n", cmds: []cmd{{'G', 10}, {'X', 1}, {'Y', 2}, {'Z', 3}}},
	}

	for i, c := range cases {
		p := Parser{
			Scanner: strings.NewReader(c.s),
			Dialect: BeagleG,
		}
		for _, cmd := range c.cmds {
			code, num, err := p.Parse()
			if err != nil {
				t.Errorf("Parse(%s) failed with %s", c.s, err)
			} else if code != cmd.code || num != cmd.num {
				t.Errorf("Parse(%s)[%d]: got %c%s want %c%s", c.s, i, code, num, cmd.code, cmd.num)
			}
		}
		_, _, err := p.Parse()
		if c.fail {
			if err == nil || err == io.EOF {
				t.Errorf("Parse(%s) did not fail", c.s)
			}
		} else if err != io.EOF {
			t.Errorf("Parse(%s) not at EOF: %s", c.s, err)
		}
	}
}

func parseParameter(p *Parser) (num int, nam string, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
			num = 0
			nam = ""
		}
	}()

	num, nam = p.parseParameter()
	return
}

func TestParseParameter(t *testing.T) {
	cases := []struct {
		s    string
		fail bool
		num  int
		name string
	}{
		{s: "#123 ", num: 123},
		{s: "#123G", num: 123},
		{s: "#<123> ", name: "123"},
		{s: "#abc ", name: "abc"},
		{s: "#abc_123 ", name: "abc_123"},
		{s: "#<abc>", name: "abc"},
		{s: "#<abc123>", name: "abc123"},
		{s: "#<abc ", fail: true},
		{s: "#$$$ ", fail: true},
		{s: "#123456789 ", fail: true},
		{s: "#<>", fail: true},
	}

	for _, c := range cases {
		p := Parser{
			Scanner: strings.NewReader(c.s),
			Dialect: BeagleG,
		}
		b, err := p.Scanner.ReadByte()
		if b != '#' {
			t.Fatalf("parameters must start with #")
		}

		num, name, err := parseParameter(&p)
		if c.fail {
			if err == nil {
				t.Errorf("parseParameter(%s) did not fail", c.s)
			}
		} else if num != c.num {
			t.Errorf("parseParameter(%s) got %d want %d", c.s, num, c.num)
		} else if name != c.name {
			t.Errorf("parseParameter(%s) got %s want %s", c.s, name, c.name)
		}
	}
}

func parseAssignOp(p *Parser) (aop assignOp, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
			aop = 0
		}
	}()

	aop = p.parseAssignOp()
	return
}

func TestParseAssignOp(t *testing.T) {
	cases := []struct {
		s        string
		fail     bool
		assignOp assignOp
	}{
		{s: "= ", assignOp: assign},
		{s: "+= ", assignOp: assignPlus},
		{s: "-= ", assignOp: assignMinus},
		{s: "*= ", assignOp: assignTimes},
		{s: "/= ", assignOp: assignDivide},
		{s: "++ ", assignOp: plusPlus},
		{s: "-- ", assignOp: minusMinus},
		{s: "$$ ", fail: true},
		{s: "+- ", fail: true},
		{s: "-+ ", fail: true},
		{s: "+$ ", fail: true},
		{s: "-$ ", fail: true},
		{s: "*$ ", fail: true},
		{s: "/$ ", fail: true},
	}

	for _, c := range cases {
		p := Parser{
			Scanner: strings.NewReader(c.s),
			Dialect: BeagleG,
		}
		assignOp, err := parseAssignOp(&p)
		if c.fail {
			if err == nil {
				t.Errorf("parseAssignOp(%s) did not fail", c.s)
			}
		} else if assignOp != c.assignOp {
			t.Errorf("parseAssignOp(%s) got %d want %d", c.s, assignOp, c.assignOp)
		}
	}
}

func TestParseNameAssignment(t *testing.T) {
	cases := []struct {
		s    string
		name Name
		val  Number
		fail bool
	}{
		{s: "#abc=11\nG10\n", name: "abc", val: 11},
		{s: "#abc =11\nG10\n", name: "abc", val: 11},
		{s: "#abc= 11\nG10\n", name: "abc", val: 11},
		{s: "#abc = 11\nG10\n", name: "abc", val: 11},

		{s: "#abc+=11\nG10\n", fail: true},
		{s: "#abc=11\n#abc+=11\nG10\n", name: "abc", val: 22},
		{s: "#abc=11\n#abc++\nG10\n", name: "abc", val: 12},

		{s: "#abc-=11\nG10\n", fail: true},
		{s: "#abc=11\n#abc-=22\nG10\n", name: "abc", val: -11},
		{s: "#abc=11\n#abc--\nG10\n", name: "abc", val: 10},

		{s: "#abc*=11\nG10\n", fail: true},
		{s: "#abc=0\n#abc*=11\nG10\n", name: "abc", val: 0},
		{s: "#abc=11\n#abc*=8\nG10\n", name: "abc", val: 88},

		{s: "#abc/=11\nG10\n", fail: true},
		{s: "#abc=0\n#abc/=2\nG10\n", name: "abc", val: 0},
		{s: "#abc=22\n#abc/=2\nG10\n", name: "abc", val: 11},

		{s: "*123 #abc=10\n", fail: true},
		{s: "#abc=10 N123\n", fail: true},
	}

	for _, c := range cases {
		p := Parser{
			Scanner: strings.NewReader(c.s),
			Dialect: BeagleG,
		}
		_, _, err := p.Parse()
		if c.fail {
			if err == nil {
				t.Errorf("Parse(%s) did not fail", c.s)
			}
		} else if err != nil {
			t.Errorf("Parse(%s) failed with %s", c.s, err)
		} else {
			val, ok := p.nameParams[c.name]
			if !ok {
				t.Errorf("Parse(%s): name parameter %s not found", c.s, c.name)
			} else if val != c.val {
				t.Errorf("Parse(%s): got %s want %s", c.s, val, c.val)
			}
		}
	}
}

func TestParseNumAssignment(t *testing.T) {
	cases := []struct {
		s    string
		num  int
		val  Number
		fail bool
	}{
		{s: "#999=11\nG10\n", num: 999, val: 11},
		{s: "#777=11\n", fail: true},

		{s: "#999+=11\nG10\n", num: 999, val: 11},
		{s: "#999=11\n#999+=11\nG10\n", num: 999, val: 22},
		{s: "#999=11\n#999++\nG10\n", num: 999, val: 12},
		{s: "#666+=11\n", fail: true},
		{s: "#666++\n", fail: true},
		{s: "#777+=11\n", fail: true},
		{s: "#777++\n", fail: true},

		{s: "#999-=11\nG10\n", num: 999, val: -11},
		{s: "#999=11\n#999-=22\nG10\n", num: 999, val: -11},
		{s: "#999=11\n#999--\nG10\n", num: 999, val: 10},
		{s: "#666-=11\n", fail: true},
		{s: "#666--\n", fail: true},
		{s: "#777-=11\n", fail: true},
		{s: "#777--\n", fail: true},

		{s: "#999*=11\nG10\n", num: 999, val: 0},
		{s: "#999=11\n#999*=8\nG10\n", num: 999, val: 88},
		{s: "#666*=11\n", fail: true},
		{s: "#777*=11\n", fail: true},

		{s: "#999/=2\nG10\n", num: 999, val: 0},
		{s: "#999=22\n#999/=2\nG10\n", num: 999, val: 11},
		{s: "#666/=11\n", fail: true},
		{s: "#777/=11\n", fail: true},
	}

	s := "#1=1\nG10\n"
	p := Parser{
		Scanner: strings.NewReader(s),
		Dialect: BeagleG,
	}
	_, _, err := p.Parse()
	if err == nil {
		t.Errorf("Parse(%s) did not fail", s)
	}

	s = "G#1\n"
	p = Parser{
		Scanner: strings.NewReader(s),
		Dialect: BeagleG,
	}
	_, _, err = p.Parse()
	if err == nil {
		t.Errorf("Parse(%s) did not fail", s)
	}

	for _, c := range cases {
		numParams := map[int]Number{}
		p := Parser{
			Scanner: strings.NewReader(c.s),
			Dialect: BeagleG,
			GetNumParam: func(num int) (Number, error) {
				if num == 666 {
					return 0, errors.New("failed")
				}
				return numParams[num], nil
			},
			SetNumParam: func(num int, val Number) error {
				if num == 777 {
					return errors.New("failed")
				}
				numParams[num] = val
				return nil
			},
		}
		_, _, err := p.Parse()
		if c.fail {
			if err == nil {
				t.Errorf("Parse(%s) did not fail", c.s)
			}
		} else if err != nil {
			t.Errorf("Parse(%s) failed with %s", c.s, err)
		} else {
			val, ok := numParams[c.num]
			if !ok {
				t.Errorf("Parse(%s): num parameter %d not found", c.s, c.num)
			} else if val != c.val {
				t.Errorf("Parse(%s): got %s want %s", c.s, val, c.val)
			}
		}
	}
}

func TestParseComments(t *testing.T) {
	cases := []struct {
		s         string
		comment   string
		intVal    int
		stringVal string
		lineEnd   bool
		parsed    bool
		executed  bool
		fail      bool
	}{
		{s: " ;abcd\nG10 ", comment: "abcd", lineEnd: true},
		{s: "(abcd) G10 ", comment: "abcd", parsed: true},
		{s: "(abcd) G10 ", comment: "abcd", intVal: 1, parsed: true, executed: true},
		{s: "(abcd) G10 ", comment: "abcd", stringVal: "xyz", parsed: true, executed: true},
		{s: "(abcd) G10 ", comment: "abcd", intVal: 1, stringVal: "xyz", parsed: true,
			executed: true},
		{s: " ;fail\nG10 ", fail: true},
		{s: "(fail) G10 ", fail: true},
		{s: "(abcd) G10 ", comment: "abcd", intVal: -1, parsed: true, fail: true},
	}

	for _, c := range cases {
		var lineEnd, parsed, executed bool
		p := Parser{
			Scanner: strings.NewReader(c.s),
			Dialect: BeagleG,
			LineEndComment: func(comment string) error {
				if comment == "fail" {
					return errors.New("failed")
				}
				if comment != c.comment {
					t.Errorf("Parse(%s): LineEndComment: got %s want %s", c.s, comment, c.comment)
				}
				lineEnd = true
				return nil
			},
			InlineParsed: func(comment string) (int, string, error) {
				if comment == "fail" {
					return 0, "", errors.New("failed")
				}
				if comment != c.comment {
					t.Errorf("Parse(%s): InlineParsed: got %s want %s", c.s, comment, c.comment)
				}
				parsed = true
				return c.intVal, c.stringVal, nil
			},
			InlineExecuted: func(i int, s string) error {
				if i == -1 {
					return errors.New("failed")
				}
				if i != c.intVal {
					t.Errorf("Parse(%s): InlineExecuted: got %d want %d", c.s, i, c.intVal)
				}
				if s != c.stringVal {
					t.Errorf("Parse(%s): InlineParsed: got %s want %s", c.s, s, c.stringVal)
				}
				executed = true
				return nil
			},
		}

		_, _, err := p.Parse()
		if c.fail {
			if err == nil {
				t.Errorf("Parse(%s) did not fail", c.s)
			}
		} else if err != nil {
			t.Errorf("Parse(%s) failed with %s", c.s, err)
		}
		if lineEnd != c.lineEnd {
			if lineEnd {
				t.Errorf("Parse(%s): LineEndComment should not have been called", c.s)
			} else {
				t.Errorf("Parse(%s): LineEndComment should have been called", c.s)
			}
		}
		if parsed != c.parsed {
			if parsed {
				t.Errorf("Parse(%s): InlineParsed should not have been called", c.s)
			} else {
				t.Errorf("Parse(%s): InlineParsed should have been called", c.s)
			}
		}
		if executed != c.executed {
			if executed {
				t.Errorf("Parse(%s): InlineExecuted should not have been called", c.s)
			} else {
				t.Errorf("Parse(%s): InlineExecuted should have been called", c.s)
			}
		}
	}
}

func TestParameters(t *testing.T) {
	cases := []struct {
		s    string
		fail bool
		code Code
		num  Number
	}{
		{s: "#abc=11\nG#abc\n", code: 'G', num: 11},
		{s: "G#abc\n", fail: true},

		{s: "#999=22\nG#999\n", code: 'G', num: 22},
		{s: "G#888\n", fail: true},
		{s: "#1=2 #2=3\nG##1\n", code: 'G', num: 3},
		{s: "#3=4\nG#[1+2]\n", code: 'G', num: 4},
		{s: "#3=5\n#4=#[1+2]\nG#4\n", code: 'G', num: 5},

		{s: "#abc=<def> #def=11\nG##abc\n", code: 'G', num: 11},

		{s: "#abc=10\n*#abc ", fail: true},
		{s: "#abc=10\nN#abc ", fail: true},
	}

	for _, c := range cases {
		numParams := map[int]Number{}
		p := Parser{
			Scanner: strings.NewReader(c.s),
			Dialect: BeagleG,
			GetNumParam: func(num int) (Number, error) {
				val, ok := numParams[num]
				if !ok {
					return 0, errors.New("not found")
				}
				return val, nil
			},
			SetNumParam: func(num int, val Number) error {
				numParams[num] = val
				return nil
			},
		}
		code, num, err := p.Parse()
		if c.fail {
			if err == nil {
				t.Errorf("Parse(%s) did not fail", c.s)
			}
		} else if err != nil {
			t.Errorf("Parse(%s) failed with %s", c.s, err)
		} else if code != c.code {
			t.Errorf("Parse(%s) got %c want %c", c.s, code, c.code)
		} else if num != c.num {
			t.Errorf("Parse(%s) got %s want %s", c.s, num, c.num)
		}
	}
}

func parseExpr(p *Parser) (expr expression, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
			expr = nil
		}
	}()

	expr = p.parseExpr()
	return
}

func evaluateExpr(p *Parser, expr expression) (num Number, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
			num = 0
		}
	}()

	val := expr.evaluate(p)
	var ok bool
	num, ok = val.AsNumber()
	if !ok {
		err = fmt.Errorf("expected a number: %v", val)
	}
	return
}

func notEq(n1, n2 Number) bool {
	return math.Abs(float64(n1-n2)) > 0.000001
}

func TestExpressions(t *testing.T) {
	cases := []struct {
		s            string
		pfail, efail bool
		num          Number
	}{
		{s: "123 ", num: 123},
		{s: "#99 ", num: 199},
		{s: "#101 ", efail: true},
		{s: "[123] ", num: 123},
		{s: "[#99] ", num: 199},
		{s: "[#101] ", efail: true},
		{s: "[123 G", pfail: true},
		{s: "[123 + [456 * 789 [ ", pfail: true},
		{s: "[-123] ", num: -123},
		{s: "[-#1] ", num: -101},
		{s: "[12 + 34] ", num: 46},
		{s: "[12+34] ", num: 46},
		{s: "[12+ 34] ", num: 46},
		{s: "[12 +34] ", num: 46},
		{s: "[12+34+56] ", num: 102},
		{s: "[1 + 2 * 3] ", num: 7},
		{s: "[2 * 3 + 4] ", num: 10},
		{s: "[[1 + 2] * 3] ", num: 9},
		{s: "[2 * [3 + 4]] ", num: 14},
		{s: "[- [2 * 3]] ", num: -6},
		{s: "[- 2 * 3] ", num: -6},
		{s: "[101 == ] ", pfail: true},
		{s: "[1 + 100 == #2] ", num: 0},
		{s: "[! 0] ", num: 1},
		{s: "[! 1] ", num: 0},
		{s: "[! 102 == #1] ", num: 1},
		{s: "[! [102 == #1]] ", num: 1},
		{s: "[1 || #111]] ", num: 1},
		{s: "[0 || #111]] ", efail: true},
		{s: "[0 && #111]] ", num: 0},
		{s: "[1 && #111]] ", efail: true},
		{s: "[101 == #1] ", num: 1},
		{s: "[100 == #1] ", num: 0},
		{s: "[101 < #1] ", num: 0},
		{s: "[99 < #1] ", num: 1},
		{s: "[102 <= #1] ", num: 0},
		{s: "[101 <= #1] ", num: 1},
		{s: "[100 != #1] ", num: 1},
		{s: "[101 != #1] ", num: 0},
		{s: "[101 > #1] ", num: 0},
		{s: "[102 > #1] ", num: 1},
		{s: "[100 >= #1] ", num: 0},
		{s: "[101 >= #1] ", num: 1},
		{s: "[10 - 5] ", num: 5},
		{s: "[5 - 10] ", num: -5},
		{s: "[5 * 10] ", num: 50},
		{s: "[5 * - 10] ", num: -50},
		{s: "[50 / 10] ", num: 5},
		{s: "[-50 / 10] ", num: -5},
		{s: "[50 / -10] ", num: -5},
		{s: "[-50 / -10] ", num: 5},
		{s: "[1 =! 2] ", pfail: true},
		{s: "[1 !- 2] ", pfail: true},
		{s: "[1 &| 2] ", pfail: true},
		{s: "[1 |& 2] ", pfail: true},
		{s: "[2 + 3 + 4 * 5] ", num: 25},
		{s: "[2 + 3 * 4 + 5] ", num: 19},
		{s: "[2 * 3 + 4 + 5] ", num: 15},
		{s: "[2 * 3 + 4 * 5] ", num: 26},
		{s: "[#test] ", num: 10},

		{s: "[a3[123]] ", pfail: true},
		{s: "[abc[123]] ", pfail: true},
		{s: "[abs +] ", pfail: true},
		{s: "[abs 123] ", pfail: true},
		{s: "[abs[]] ", pfail: true},
		{s: "[abs[123,456]] ", pfail: true},
		{s: "[abs[123 456]] ", pfail: true},
		{s: "[abs[123,456,]] ", pfail: true},

		{s: "[abs[123]] ", num: 123},
		{s: "[abs[-123]] ", num: 123},
		{s: "[abs[123.456]] ", num: 123.456},
		{s: "[abs[-123.456]] ", num: 123.456},

		{s: "[sin[0]] ", num: 0},
		{s: "[asin[0]] ", num: 0},
		{s: "[sin[30]] ", num: 0.5},
		{s: "[asin[0.5]] ", num: 30},
		{s: "[sin[45]] ", num: Number(1 / math.Sqrt(2))},
		{s: "[asin[1 / sqrt[2]]] ", num: 45},
		{s: "[sin[60]] ", num: Number(math.Sqrt(3) / 2)},
		{s: "[asin[sqrt[3] / 2]] ", num: 60},
		{s: "[sin[90]] ", num: 1},
		{s: "[asin[1]] ", num: 90},

		{s: "[cos[0]] ", num: 1},
		{s: "[acos[1]] ", num: 0},
		{s: "[cos[30]] ", num: Number(math.Sqrt(3) / 2)},
		{s: "[acos[sqrt[3] / 2]] ", num: 30},
		{s: "[cos[45]] ", num: Number(1 / math.Sqrt(2))},
		{s: "[acos[1 / sqrt[2]]] ", num: 45},
		{s: "[cos[60]] ", num: 0.5},
		{s: "[acos[0.5]] ", num: 60},
		{s: "[cos[90]] ", num: 0},
		{s: "[acos[0]] ", num: 90},

		{s: "[tan[0]] ", num: 0},
		{s: "[atan[0]] ", num: 0},
		{s: "[tan[30]] ", num: Number(1 / math.Sqrt(3))},
		{s: "[atan[1 / sqrt[3]]] ", num: 30},
		{s: "[tan[45]] ", num: 1},
		{s: "[atan[1]] ", num: 45},
		{s: "[tan[60]] ", num: Number(math.Sqrt(3))},
		{s: "[atan[sqrt[3]]] ", num: 60},

		{s: "[ceil[12.34]] ", num: 13},
		{s: "[ceil[-12.34]] ", num: -12},
		{s: "[floor[12.34]] ", num: 12},
		{s: "[floor[-12.34]] ", num: -13},
		{s: "[round[12.34]] ", num: 12},
		{s: "[round[-12.34]] ", num: -12},
		{s: "[round[34.56]] ", num: 35},
		{s: "[round[-34.56]] ", num: -35},
	}

	for _, c := range cases {
		p := Parser{
			Scanner: strings.NewReader(c.s),
			Dialect: BeagleG,
			GetNumParam: func(num int) (Number, error) {
				if num < 100 {
					return Number(num) + 100, nil
				}
				return 0, errors.New("not found")
			},
			SetNumParam: func(num int, val Number) error {
				return errors.New("should not be called")
			},
		}
		p.nameParams = map[Name]Value{}
		p.nameParams["test"] = Number(10)

		e, err := parseExpr(&p)
		if c.pfail {
			if err == nil {
				t.Errorf("parseExpr(%s) did not fail", c.s)
			}
			continue
		} else if err != nil {
			t.Errorf("parseExpr(%s) failed with %s", c.s, err)
			continue
		}

		n, err := evaluateExpr(&p, e)
		if c.efail {
			if err == nil {
				t.Errorf("evaluateExpr(%s) did not fail", c.s)
			}
		} else if err != nil {
			t.Errorf("evaluateExpr(%s) failed with %s", c.s, err)
		} else if notEq(n, c.num) {
			t.Errorf("evaluateExpr(%s) got %s, want %s", c.s, n, c.num)
		}
	}
}

func valuesEqual(v1, v2 Value) bool {
	if n1, ok := v1.AsNumber(); ok {
		n2, ok := v2.AsNumber()
		return ok && n1 == n2
	} else if n1, ok := v1.AsName(); ok {
		n2, ok := v2.AsName()
		return ok && n1 == n2
	} else if s1, ok := v1.AsString(); ok {
		s2, ok := v2.AsString()
		return ok && s1 == s2
	}
	return false
}

func TestValues(t *testing.T) {
	cases := []struct {
		s     string
		pfail bool
		v     Value
	}{
		{s: "123 ", v: Number(123)},

		{s: `"abc"`, v: String("abc")},
		{s: `"abc`, pfail: true},
		{s: `"abc\"def"`, v: String(`abc"def`)},

		{s: "<abc>", v: Name("abc")},
		{s: "<123>", v: Name("123")},
		{s: "<>", pfail: true},
		{s: `<abc"`, pfail: true},
	}

	for _, c := range cases {
		p := Parser{
			Scanner: strings.NewReader(c.s),
			Dialect: BeagleG,
		}

		e, err := parseExpr(&p)
		if c.pfail {
			if err == nil {
				t.Errorf("parseExpr(%s) did not fail", c.s)
			}
			continue
		} else if err != nil {
			t.Errorf("parseExpr(%s) failed with %s", c.s, err)
			continue
		}

		v := e.evaluate(&p)
		if !valuesEqual(v, c.v) {
			t.Errorf("evaluate(%s) got %s, want %s", c.s, v, c.v)
		}
	}
}

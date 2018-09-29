package gcode

import (
	"errors"
	"io"
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
		{s: "G-10.20\n", cmds: []cmd{{'G', -10}}}, // XXX: missing fraction
		{s: "G+10.20\n", cmds: []cmd{{'G', 10}}},  // XXX: missing fraction

		{s: "G10 *20\n", cmds: []cmd{{'G', 10}}},
		{s: "G10 *20 ;comment\nG30\n", cmds: []cmd{{'G', 10}, {'G', 30}}},
		{s: "G10 *20 G30\n", cmds: []cmd{{'G', 10}}, fail: true},

		{s: "N10 G20\n", cmds: []cmd{{'G', 20}}},
		{s: "G10\nG20\n\nN1 G2\n", fail: true, cmds: []cmd{{'G', 10}, {'G', 20}}},
		{s: "N10 G-\n", fail: true},
		{s: "N9999999999999999 G10\n", fail: true},
		{s: "*123 G10\n", fail: true},
		{s: "*123 WHILE\n", fail: true},
	}

	for _, c := range cases {
		p := Parser{
			Scanner: strings.NewReader(c.s),
		}
		for i, cmd := range c.cmds {
			code, num, err := p.Parse()
			if err != nil {
				t.Errorf("Parse(%s) failed with %s", c.s, err)
			} else if code != cmd.code || num != cmd.num {
				t.Errorf("Parse(%s)[%d]: got %c%d want %c%d", c.s, i, code, num, cmd.code, cmd.num)
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
		{s: "#<123> ", fail: true},
		{s: "#abc ", name: "abc"},
		{s: "#abc_123 ", name: "abc_123"},
		{s: "#<abc>", name: "abc"},
		{s: "#<abc ", fail: true},
		{s: "#$$$ ", fail: true},
		{s: "#123456789 ", fail: true},
	}

	for _, c := range cases {
		p := Parser{
			Scanner: strings.NewReader(c.s),
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
		name string
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
				t.Errorf("Parse(%s): got %d want %d", c.s, val, c.val)
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
	}
	_, _, err := p.Parse()
	if err == nil {
		t.Errorf("Parse(%s) did not fail", s)
	}

	s = "G#1\n"
	p = Parser{
		Scanner: strings.NewReader(s),
	}
	_, _, err = p.Parse()
	if err == nil {
		t.Errorf("Parse(%s) did not fail", s)
	}

	for _, c := range cases {
		numParams := map[int]Number{}
		p := Parser{
			Scanner: strings.NewReader(c.s),
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
				t.Errorf("Parse(%s): got %d want %d", c.s, val, c.val)
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

		{s: "#abc=10\n*#abc ", fail: true},
		{s: "#abc=10\nN#abc ", fail: true},
	}

	for _, c := range cases {
		numParams := map[int]Number{}
		p := Parser{
			Scanner: strings.NewReader(c.s),
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
			t.Errorf("Parse(%s) got %d want %d", c.s, num, c.num)
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

	num = expr.evaluate(p)
	return
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

		{s: "[abs[123]] ", num: 123},
		{s: "[abs[-123]] ", num: 123},
		{s: "[a3[123]] ", pfail: true},
		{s: "[abc[123]] ", pfail: true},
		{s: "[abs +] ", pfail: true},
		{s: "[abs 123] ", pfail: true},
		{s: "[abs[]] ", pfail: true},
		{s: "[abs[123,456]] ", pfail: true},
		{s: "[abs[123 456]] ", pfail: true},
		{s: "[abs[123,456,]] ", pfail: true},
	}

	for _, c := range cases {
		p := Parser{
			Scanner: strings.NewReader(c.s),
			GetNumParam: func(num int) (Number, error) {
				if num < 100 {
					return Number(num + 100), nil
				}
				return 0, errors.New("not found")
			},
			SetNumParam: func(num int, val Number) error {
				return errors.New("should not be called")
			},
		}
		p.nameParams = map[string]Number{}
		p.nameParams["test"] = 10

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
		} else if n != c.num {
			t.Errorf("evaluateExpr(%s) got %d, want %d", c.s, n, c.num)
		}
	}
}

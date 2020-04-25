package gcode

import (
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func codesEqual(c1, c2 []Code) bool {
	if len(c1) != len(c2) {
		return false
	}

	for i := range c1 {
		if c1[i] != c2[i] {
			return false
		}
	}

	return true
}

func TestParser(t *testing.T) {
	cases := []struct {
		s     string
		fail  bool
		codes []Code
	}{
		{s: "G10\n", codes: []Code{{'G', Number(10)}}},
		{s: "g10\n", codes: []Code{{'G', Number(10)}}},
		{s: " G 10\n", codes: []Code{{'G', Number(10)}}},
		{s: "(comment)G10\n", codes: []Code{{'G', Number(10)}}},
		{s: "(comment) G10\n", codes: []Code{{'G', Number(10)}}},
		{s: "(comment\n) G10\n", fail: true},
		{s: "; comment\nG10\n", codes: []Code{{'G', Number(10)}}},
		{s: "% comment\nG10\n", codes: []Code{{'G', Number(10)}}},
		{s: "G;comment\n10\n", fail: true},
		{s: "G%comment\n10\n", fail: true},
		{s: "G(comment)10\n", fail: true},
		{s: "GG\n", fail: true},
		{s: "$$$\n", fail: true},
		{s: "G-10\n", codes: []Code{{'G', Number(-10)}}},
		{s: "G+10\n", codes: []Code{{'G', Number(10)}}},
		{s: "G+\n", fail: true},
		{s: "G-\n", fail: true},
		{s: "G+.\n", fail: true},
		{s: "G-.\n", fail: true},
		{s: "G.\n", fail: true},
		{s: "G+0\n", codes: []Code{{'G', Number(0)}}},
		{s: "G-0\n", codes: []Code{{'G', Number(0)}}},
		{s: "G+.0\n", codes: []Code{{'G', Number(0)}}},
		{s: "G-.0\n", codes: []Code{{'G', Number(0)}}},
		{s: "G+0.\n", codes: []Code{{'G', Number(0)}}},
		{s: "G-0.\n", codes: []Code{{'G', Number(0)}}},
		{s: "G0.\n", codes: []Code{{'G', Number(0)}}},
		{s: "G.0\n", codes: []Code{{'G', Number(0)}}},
		{s: "G-10.20\n", codes: []Code{{'G', Number(-10.20)}}},
		{s: "G+10.20\n", codes: []Code{{'G', Number(10.20)}}},

		{s: "G10 *20\n", codes: []Code{{'G', Number(10)}}},
		{s: "G10 G30 *20 ;comment\n", codes: []Code{{'G', Number(10)}, {'G', Number(30)}}},
		{s: "G10 *20 G30\n", fail: true},

		{s: "N10 G20\n", codes: []Code{{'G', Number(20)}}},
		{s: "N10 G-\n", fail: true},
		{s: "N9999999999999999 G10\n", fail: true},
		{s: "*123 G10\n", fail: true},
		{s: "*123 WHILE\n", fail: true},
		{s: "    G10X1Y 2Z3\n",
			codes: []Code{{'G', Number(10)}, {'X', Number(1)}, {'Y', Number(2)}, {'Z', Number(3)}}},
	}

	for i, c := range cases {
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
		}

		codes, err := p.Parse()
		if c.fail {
			if err == nil {
				t.Errorf("Parse(%s) did not fail", c.s)
			}
		} else if err != nil {
			t.Errorf("Parse(%s) failed with %s", c.s, err)
		} else if !codesEqual(codes, c.codes) {
			t.Errorf("Parse(%s)[%d]: got %v want %v", c.s, i, codes, c.codes)
		} else {
			_, err = p.Parse()
			if err != io.EOF {
				t.Errorf("Parse(%s) not at EOF: %s", c.s, err)
			}
		}
	}
}

func TestParserLines(t *testing.T) {
	cases := []struct {
		s     string
		lines [][]Code
	}{
		{
			s: `
G10 X1 Y2
G11 (comment) X1 Y2
G12 X1 (comment) Y2
G13 X1 Y2 (comment)
G14 X1 Y2 ; comment
`,
			lines: [][]Code{
				{{'G', Number(10)}, {'X', Number(1)}, {'Y', Number(2)}},
				{{'G', Number(11)}, {'X', Number(1)}, {'Y', Number(2)}},
				{{'G', Number(12)}, {'X', Number(1)}, {'Y', Number(2)}},
				{{'G', Number(13)}, {'X', Number(1)}, {'Y', Number(2)}},
				{{'G', Number(14)}, {'X', Number(1)}, {'Y', Number(2)}},
			},
		},
	}

	for i, c := range cases {
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
		}

		for _, line := range c.lines {
			codes, err := p.Parse()
			if err != nil {
				t.Errorf("Parse(%s) failed with %s", c.s, err)
			} else if !codesEqual(codes, line) {
				t.Errorf("Parse(%s)[%d]: got %v want %v", c.s, i, codes, line)
			}
		}

		_, err := p.Parse()
		if err != io.EOF {
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

	val := p.parseParameter()
	if n, ok := val.(Number); ok {
		num = int(n)
	} else if s, ok := val.(Name); ok {
		nam = string(s)
	}
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
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
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
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
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
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
		}
		_, err := p.Parse()
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
		Scanner:  strings.NewReader(s),
		Features: AllFeatures,
	}
	_, err := p.Parse()
	if err == nil {
		t.Errorf("Parse(%s) did not fail", s)
	}

	s = "G#1\n"
	p = Parser{
		Scanner:  strings.NewReader(s),
		Features: AllFeatures,
	}
	_, err = p.Parse()
	if err == nil {
		t.Errorf("Parse(%s) did not fail", s)
	}

	for _, c := range cases {
		numParams := map[int]Number{}
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
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
		_, err := p.Parse()
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

func TestParseIfBeagleG(t *testing.T) {
	cases := []struct {
		s    string
		num  int
		val  Number
		fail bool
	}{
		{s: "#1=0\nIF 1 THEN #1=1\nG1\n", num: 1, val: 1},
		{s: "#1=0\nIF 0 THEN #1=1\nG1\n", num: 1, val: 0},
		{s: "#1=1 #2=0\nIF #1 THEN #2=1\nG1\n", num: 2, val: 1},
		{s: "#1=0 #2=0\nIF #1 THEN #2=1\nG1\n", num: 2, val: 0},
		{s: "#1=1\nIF #1 THEN #2=1 ELSE #2=2\nG1\n", num: 2, val: 1},
		{s: "#1=0\nIF #1 THEN #2=1 ELSE #2=2\nG1\n", num: 2, val: 2},

		{s: "IF 0\n", fail: true},
		{s: "IF 0 THEN\n", fail: true},
		{s: "IF 0 THEN [1 + 2]\n", fail: true},
		{s: "IF G1\n", fail: true},
		{s: "IF 0 THEN #1=1\n", fail: true},
		{s: "IF 0 THEN #1=1 THEN\n", fail: true},
		{s: "IF 0 THEN #1=1 ELSE 123\n", fail: true},
		{s: "IF 0 THEN #1=1 ELSENOT\n", fail: true},
		{s: "G0 IF 0 THEN #1=1\n", fail: true},

		{s: "#1=0\nIF 1 THEN #1=1 ELSEIF 1 THEN #1=2 ELSE #1=3\nG1\n", num: 1, val: 1},
		{s: "#1=0\nIF 0 THEN #1=1 ELSEIF 1 THEN #1=2 ELSE #1=3\nG1\n", num: 1, val: 2},
		{s: "#1=0\nIF 0 THEN #1=1 ELSEIF 0 THEN #1=2 ELSE #1=3\nG1\n", num: 1, val: 3},

		{s: "#1=0\nIF 1 THEN #1=1 ELSEIF 1 THEN #1=2 ELSEIF 1 THEN #1=3 ELSE #1=4\nG1\n",
			num: 1, val: 1},
		{s: "#1=0\nIF 0 THEN #1=1 ELSEIF 1 THEN #1=2 ELSEIF 1 THEN #1=3 ELSE #1=4\nG1\n",
			num: 1, val: 2},
		{s: "#1=0\nIF 0 THEN #1=1 ELSEIF 0 THEN #1=2 ELSEIF 1 THEN #1=3 ELSE #1=4\nG1\n",
			num: 1, val: 3},
		{s: "#1=0\nIF 0 THEN #1=1 ELSEIF 0 THEN #1=2 ELSEIF 0 THEN #1=3 ELSE #1=4\nG1\n",
			num: 1, val: 4},
	}

	for _, c := range cases {
		numParams := map[int]Number{}
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
			GetNumParam: func(num int) (Number, error) {
				return numParams[num], nil
			},
			SetNumParam: func(num int, val Number) error {
				numParams[num] = val
				return nil
			},
		}
		_, err := p.Parse()
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

func TestParseWhileBeagleG(t *testing.T) {
	cases := []struct {
		s    string
		num  int
		val  Number
		fail bool
	}{
		{s: "END\n", fail: true},
		{s: "WHILE 0\n", fail: true},
		{s: "WHILE 0 DO G1\n", fail: true},
		{s: "WHILE 0 DO\n#1=1\n", fail: true},
		{s: "WHILE DO\n", fail: true},
		{s: "WHILE 0 DO\n#1=1\nEND G1\n", fail: true},
		{s: `
#1=0
WHILE [#1 < 10] DO
    #1 += 1
END
G1
`, num: 1, val: 10},
		{s: `
#1=0
#2=1
WHILE [#1 < 4] DO
    #1 += 1
    #2 *= 2
END
G1
`, num: 2, val: 16},
		{s: `
#1=0
#2=0
WHILE [#2 < 10] DO
    #3=0
    WHILE [#3 < 10] DO
        #1 += 1
        #3 += 1
    END
    #2 += 1
END
G1
`, num: 1, val: 100},
	}

	for _, c := range cases {
		numParams := map[int]Number{}
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
			GetNumParam: func(num int) (Number, error) {
				return numParams[num], nil
			},
			SetNumParam: func(num int, val Number) error {
				numParams[num] = val
				return nil
			},
		}
		_, err := p.Parse()
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

type executor struct {
	fail     bool
	executed *bool
}

func (exec executor) Execute() error {
	*exec.executed = true
	if exec.fail {
		return errors.New("failed")
	}
	return nil
}

func TestParseComments(t *testing.T) {
	cases := []struct {
		s        string
		comment  string
		inline   bool
		executed bool
		fail     bool
	}{
		{s: " ;abcd\nG10\n", comment: "abcd"},
		{s: "(abcd) G10\n", comment: "abcd", inline: true},
		{s: "(execute) G10\n", comment: "execute", inline: true, executed: true},
		{s: ";execute\nG10\n", comment: "execute", executed: true},
		{s: "(exec-fail) G10\n", comment: "exec-fail", inline: true, executed: true, fail: true},
		{s: ";exec-fail\nG10\n", comment: "exec-fail", executed: true, fail: true},
		{s: " ;fail\nG10\n", comment: "fail", fail: true},
		{s: "(fail) G10\n", comment: "fail", fail: true, inline: true},
	}

	for _, c := range cases {
		var executed, called bool
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
			Comment: func(comment string, inline bool) (Executor, error) {
				called = true

				if comment != c.comment {
					t.Errorf("Parse(%s): Comment: got %s want %s", c.s, comment, c.comment)
				}
				if inline != c.inline {
					t.Errorf("Parse(%s): Comment: for inline got %v want %v", c.s, inline, c.inline)
				}

				if comment == "fail" {
					return nil, errors.New("failed")
				} else if comment == "execute" {
					return executor{
						fail:     false,
						executed: &executed,
					}, nil
				} else if comment == "exec-fail" {
					return executor{
						fail:     true,
						executed: &executed,
					}, nil
				}
				return nil, nil
			},
		}

		_, err := p.Parse()
		if c.fail {
			if err == nil {
				t.Errorf("Parse(%s) did not fail", c.s)
			}
		} else if err != nil {
			t.Errorf("Parse(%s) failed with %s", c.s, err)
		}
		if !called {
			t.Errorf("Parse(%s): Comment should have been called", c.s)
		}
		if executed != c.executed {
			if executed {
				t.Errorf("Parse(%s): Execute should not have been called", c.s)
			} else {
				t.Errorf("Parse(%s): Execute should have been called", c.s)
			}
		}
	}
}

func TestParameters(t *testing.T) {
	cases := []struct {
		s     string
		fail  bool
		codes []Code
		f     Features
	}{
		{s: "#abc=11\nG#abc\n", codes: []Code{{'G', Number(11)}}},
		{s: "G#abc\n", fail: true},
		{s: "#\"abcd\"=123\n", fail: true},

		{s: "#999=22\nG#999\n", codes: []Code{{'G', Number(22)}}},
		{s: "G#888\n", fail: true},
		{s: "#1=2 #2=3\nG##1\n", codes: []Code{{'G', Number(3)}}},
		{s: "#3=4\nG#[1+2]\n", codes: []Code{{'G', Number(4)}}},
		{s: "#3=5\n#4=#[1+2]\nG#4\n", codes: []Code{{'G', Number(5)}}},

		{s: "#abc=<def> #def=11\nG##abc\n", codes: []Code{{'G', Number(11)}}},
		{s: "#abc=123\nG##abc\n", fail: true},

		{s: "#abc=1\n#abc=2 G#abc\n", codes: []Code{{'G', Number(2)}}, f: BeagleG},
		{s: "#<abc>=1\n#<abc>=2 G#<abc>\n", codes: []Code{{'G', Number(1)}}, f: LinuxCNC},
		{s: "#1=1\n#1=2 % comment\nG#1\n", codes: []Code{{'G', Number(2)}}, f: BeagleG},
		{s: "#1=1\n#1=2 % comment\nG#1\n", codes: []Code{{'G', Number(2)}}, f: LinuxCNC},

		{s: "#abc=10\n*#abc ", fail: true},
		{s: "#abc=10\nN#abc ", fail: true},
	}

	for _, c := range cases {
		f := c.f
		if f == 0 {
			f = AllFeatures
		}
		numParams := map[int]Number{}
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: f,
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
		codes, err := p.Parse()
		if c.fail {
			if err == nil {
				t.Errorf("Parse(%s) did not fail", c.s)
			}
		} else if err != nil {
			t.Errorf("Parse(%s) failed with %s", c.s, err)
		} else if !reflect.DeepEqual(codes, c.codes) {
			t.Errorf("Parse(%s) got %v want %v", c.s, codes, c.codes)
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
		{s: "[1 && 2] ", num: 1},
		{s: "[1 && 0] ", num: 0},
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

		{s: `[123+"abc"] `, efail: true},
		{s: `[<abc>+123] `, efail: true},
	}

	for _, c := range cases {
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
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
		{s: "abc\ndef", pfail: true},

		{s: "<abc>", v: Name("abc")},
		{s: "<123>", v: Name("123")},
		{s: "<>", pfail: true},
		{s: `<abc"`, pfail: true},
		{s: "<abc\ndef>", pfail: true},
	}

	for _, c := range cases {
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
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

	num := Number(123.456)
	if _, ok := num.AsName(); ok {
		t.Errorf("%#v.AsName() did not fail", num)
	}
	if _, ok := num.AsString(); ok {
		t.Errorf("%#v.AsString() did not fail", num)
	}
	if _, ok := num.AsInteger(); ok {
		t.Errorf("%#v.AsInteger() did not fail", num)
	}

	num = Number(123.0)
	if n, ok := num.AsInteger(); !ok || n != 123 {
		t.Errorf("%#v.AsInteger() failed: %d", num, n)
	}

	nam := Name("abc")
	if _, ok := nam.AsString(); ok {
		t.Errorf("%#v.AsString() did not fail", nam)
	}
}

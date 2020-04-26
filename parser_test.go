package gcode

import (
	"bytes"
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

	val := p.parseParameter(p.Scanner)
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
		nameParams := map[Name]Value{}
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
			GetNameParam: func(name Name) (Value, bool) {
				v, ok := nameParams[name]
				return v, ok
			},
			SetNameParam: func(name Name, val Value) error {
				nameParams[name] = val
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
			val, ok := nameParams[c.name]
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

	s := "#100=1\nG10\n"
	p := Parser{
		Scanner:  strings.NewReader(s),
		Features: AllFeatures,
	}
	_, err := p.Parse()
	if err == nil {
		t.Errorf("Parse(%s) did not fail", s)
	}

	s = "G#100\n"
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
		numParams[999] = Number(0)
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
			GetNumParam: func(num int) (Number, bool) {
				if num == 666 {
					return 0, false
				}
				n, ok := numParams[num]
				return n, ok
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
		{s: "#100=0\nIF 1 THEN #100=1\nG1\n", num: 100, val: 1},
		{s: "#100=0\nIF 0 THEN #100=1\nG1\n", num: 100, val: 0},
		{s: "#100=1 #200=0\nIF #100 THEN #200=1\nG1\n", num: 200, val: 1},
		{s: "#100=0 #200=0\nIF #100 THEN #200=1\nG1\n", num: 200, val: 0},
		{s: "#100=1\nIF #100 THEN #200=1 ELSE #200=2\nG1\n", num: 200, val: 1},
		{s: "#100=0\nIF #100 THEN #200=1 ELSE #200=2\nG1\n", num: 200, val: 2},

		{s: "IF 0\n", fail: true},
		{s: "IF 0 THEN\n", fail: true},
		{s: "IF 0 THEN [1 + 2]\n", fail: true},
		{s: "IF G1\n", fail: true},
		{s: "IF 0 THEN #100=1\n", fail: true},
		{s: "IF 0 THEN #100=1 THEN\n", fail: true},
		{s: "IF 0 THEN #100=1 ELSE 123\n", fail: true},
		{s: "IF 0 THEN #100=1 ELSENOT\n", fail: true},
		{s: "G0 IF 0 THEN #100=1\n", fail: true},

		{s: "#100=0\nIF 1 THEN #100=1 ELSEIF 1 THEN #100=2 ELSE #100=3\nG1\n", num: 100, val: 1},
		{s: "#100=0\nIF 0 THEN #100=1 ELSEIF 1 THEN #100=2 ELSE #100=3\nG1\n", num: 100, val: 2},
		{s: "#100=0\nIF 0 THEN #100=1 ELSEIF 0 THEN #100=2 ELSE #100=3\nG1\n", num: 100, val: 3},

		{s: "#100=0\nIF 1 THEN #100=1 ELSEIF 1 THEN #100=2 ELSEIF 1 THEN #100=3 ELSE #100=4\nG1\n",
			num: 100, val: 1},
		{s: "#100=0\nIF 0 THEN #100=1 ELSEIF 1 THEN #100=2 ELSEIF 1 THEN #100=3 ELSE #100=4\nG1\n",
			num: 100, val: 2},
		{s: "#100=0\nIF 0 THEN #100=1 ELSEIF 0 THEN #100=2 ELSEIF 1 THEN #100=3 ELSE #100=4\nG1\n",
			num: 100, val: 3},
		{s: "#100=0\nIF 0 THEN #100=1 ELSEIF 0 THEN #100=2 ELSEIF 0 THEN #100=3 ELSE #100=4\nG1\n",
			num: 100, val: 4},
	}

	for _, c := range cases {
		numParams := map[int]Number{}
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
			GetNumParam: func(num int) (Number, bool) {
				n, ok := numParams[num]
				return n, ok
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
		{s: "WHILE 0 DO\n#100=1\n", fail: true},
		{s: "WHILE DO\n", fail: true},
		{s: "WHILE 0 DO\n#100=1\nEND G1\n", fail: true},
		{s: `
#100=0
WHILE [#100 < 10] DO
    #100 += 1
END
G1
`, num: 100, val: 10},
		{s: `
#100=0
#200=1
WHILE [#100 < 4] DO
    #100 += 1
    #200 *= 2
END
G1
`, num: 200, val: 16},
		{s: `
#100=0
#200=0
WHILE [#200 < 10] DO
    #300=0
    WHILE [#300 < 10] DO
        #100 += 1
        #300 += 1
    END
    #200 += 1
END
G1
`, num: 100, val: 100},
	}

	for _, c := range cases {
		numParams := map[int]Number{}
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
			GetNumParam: func(num int) (Number, bool) {
				n, ok := numParams[num]
				return n, ok
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

func (exec executor) Execute(p *Parser) error {
	*exec.executed = true
	if exec.fail {
		return errors.New("failed")
	}
	return nil
}

func TestParseComments(t *testing.T) {
	cases := []struct {
		s    string
		outW string
		errW string
		fail bool
	}{
		{s: " ;abcd\nG10\n"},
		{s: "(abcd) G10\n"},
		{s: "(msg,message) G10\n", outW: "message\n"},
		{s: "(debug,debug message) G10\n", outW: "debug message\n"},
		{s: "(print,print message) G10\n", errW: "print message\n"},
		{s: "G10 ;msg,message\nG10\n", outW: "message\n"},
		{s: "G10 ;debug,debug message\nG10\n", outW: "debug message\n"},
		{s: "G10 ;print,print message\nG10\n", errW: "print message\n"},
		{
			s: `
#123=456
#456=321
#<abc>=789
#<def>="a string"
#<ghi>=<name>
(msg,#123 #<abc>)
(debug,#123 #<abc>)
(print,#123 #<abc>)
(debug,#<def> #<ghi> #456)
G10
`,
			outW: `#123 #<abc>
456.0000 789.0000
a string <name> 321.0000
`,
			errW: "456.0000 789.0000\n",
		},
		{s: `#5599=0
(debug,no message)
#5599=1
(debug,need message)
G10
`,
			outW: "need message\n",
		},
		{s: "(debug,#<abc>) G10\n", fail: true},
		{s: "(debug,# ) G10\n", fail: true},
		{s: "(debug, #) G10\n", fail: true},
		{s: "(debug, #<abc) G10\n", fail: true},
		{s: "(debug, #1234567890) G10\n", fail: true},
	}

	for _, c := range cases {
		var outW bytes.Buffer
		var errW bytes.Buffer
		numParams := map[int]Number{}
		nameParams := map[Name]Value{}

		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
			OutW:     &outW,
			ErrW:     &errW,
			GetNumParam: func(num int) (Number, bool) {
				n, ok := numParams[num]
				return n, ok
			},
			SetNumParam: func(num int, val Number) error {
				numParams[num] = val
				return nil
			},
			GetNameParam: func(name Name) (Value, bool) {
				v, ok := nameParams[name]
				return v, ok
			},
			SetNameParam: func(name Name, val Value) error {
				nameParams[name] = val
				return nil
			},
		}

		_, err := p.Parse()
		if c.fail {
			if err == nil {
				t.Errorf("Parse(%s) did not fail", c.s)
			}
			continue
		} else if err != nil {
			t.Errorf("Parse(%s) failed with %s", c.s, err)
		}

		o := outW.String()
		if o != c.outW {
			t.Errorf("Parse(%s) outW: got %s want %s", c.s, o, c.outW)
		}
		e := errW.String()
		if e != c.errW {
			t.Errorf("Parse(%s) errW: got %s want %s", c.s, e, c.errW)
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

		{s: "#123456789=0\n", fail: true},
		{s: "#1=-1 \nG##1\n", fail: true},
		{s: "#1=2.1 #2=0\nG##1\n", fail: true},
	}

	for _, c := range cases {
		f := c.f
		if f == 0 {
			f = AllFeatures
		}
		numParams := map[int]Number{}
		nameParams := map[Name]Value{}
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: f,
			GetNumParam: func(num int) (Number, bool) {
				n, ok := numParams[num]
				return n, ok
			},
			SetNumParam: func(num int, val Number) error {
				numParams[num] = val
				return nil
			},
			GetNameParam: func(name Name) (Value, bool) {
				v, ok := nameParams[name]
				return v, ok
			},
			SetNameParam: func(name Name, val Value) error {
				nameParams[name] = val
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
		nameParams := map[Name]Value{}
		nameParams["test"] = Number(10)
		p := Parser{
			Scanner:  strings.NewReader(c.s),
			Features: AllFeatures,
			GetNumParam: func(num int) (Number, bool) {
				if num < 100 {
					return Number(num) + 100, true
				}
				return 0, false
			},
			SetNumParam: func(num int, val Number) error {
				return errors.New("should not be called")
			},
			GetNameParam: func(name Name) (Value, bool) {
				v, ok := nameParams[name]
				return v, ok
			},
			SetNameParam: func(name Name, val Value) error {
				nameParams[name] = val
				return nil
			},
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

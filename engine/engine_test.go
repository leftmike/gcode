package engine_test

import (
	"fmt"
	"strings"
	"testing"

	engine "github.com/leftmike/gcode/engine"
	parser "github.com/leftmike/gcode/parser"
)

const (
	setFeed = iota
	rapidTo
	linearTo
)

type action struct {
	cmd        int
	x, y, z, f float64
}

type machine struct {
	actions []action
	adx     int
}

func (m *machine) checkAction(act action) error {
	if m.adx >= len(m.actions) {
		return fmt.Errorf("test: more than %d actions: %#v", len(m.actions), act)
	}

	if act != m.actions[m.adx] {
		return fmt.Errorf("test: expected %#v; got %#v", m.actions[m.adx], act)
	}

	m.adx += 1
	return nil
}

func (m *machine) SetFeed(feed float64) error {
	return m.checkAction(action{cmd: setFeed, f: feed})
}

func (m *machine) RapidTo(pos engine.Position) error {
	return m.checkAction(action{cmd: rapidTo, x: pos.X, y: pos.Y, z: pos.Z})
}

func (m *machine) LinearTo(pos engine.Position) error {
	return m.checkAction(action{cmd: linearTo, x: pos.X, y: pos.Y, z: pos.Z})
}

func TestEvaluate(t *testing.T) {
	cases := []struct {
		s       string
		actions []action
	}{
		{s: `
G21
G91
G0 X1 Y1
G1 F1
X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 1.0, y: 1.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 1.0},
			},
		},
		{s: `
G21
G90
G0 X2 Y2
G1 F1
X4
Y4
X2
Y2
`,
			actions: []action{
				{cmd: rapidTo, x: 2.0, y: 2.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 4.0, y: 2.0},
				{cmd: linearTo, x: 4.0, y: 4.0},
				{cmd: linearTo, x: 2.0, y: 4.0},
				{cmd: linearTo, x: 2.0, y: 2.0},
			},
		},
		{s: `
G21
G10 L2 P1 X-1 Y-1
G54
G90
G0 X0 Y0
G91
G1 F1
X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 1.0, y: 1.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 1.0},
			},
		},
		{s: `
G21
G90
G0 X1 Y1
G10 L20 P1 X0 Y0
G54
G91
G1 X1 F1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 1.0, y: 1.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 1.0},
			},
		},
		{s: `
G21
G90
G0 X1 Y1
G10 L20 P1 X-1 Y-1
G0 X0 Y0
G54
G91
G1 F1
X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 1.0, y: 1.0},
				{cmd: rapidTo, x: 2.0, y: 2.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 3.0, y: 2.0},
				{cmd: linearTo, x: 3.0, y: 3.0},
				{cmd: linearTo, x: 2.0, y: 3.0},
				{cmd: linearTo, x: 2.0, y: 2.0},
			},
		},
		{s: `
G21
G90
G0 X1 Y1
G91
G1 F1
X1
Y1
X-1
Y-1
G92 X-2 Y0
G90
G0 X0 Y0
G91
G1 X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 1.0, y: 1.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 1.0},
				{cmd: rapidTo, x: 3.0, y: 1.0},
				{cmd: linearTo, x: 4.0, y: 1.0},
				{cmd: linearTo, x: 4.0, y: 2.0},
				{cmd: linearTo, x: 3.0, y: 2.0},
				{cmd: linearTo, x: 3.0, y: 1.0},
			},
		},
		{s: `
G21
G90
G0 X1 Y1
G91
G1 F1
X1
Y1
X-1
Y-1
G92 X-1
G90
G0 X0 Y0
G91
G1 X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 1.0, y: 1.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 1.0},

				{cmd: rapidTo, x: 2.0, y: 0.0},
				{cmd: linearTo, x: 3.0, y: 0.0},
				{cmd: linearTo, x: 3.0, y: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0},
				{cmd: linearTo, x: 2.0, y: 0.0},
			},
		},
		{s: `
G21
G90
G0 X0 Y0
G1 F1
X1
Y1
X0
Y0
G91
G92 X-1.5
G90
G0 X0 Y0
G1 X1
Y1
X0
Y0
G92 Y-1.5
G0 X0 Y0
G1 X1
Y1
X0
Y0
G92 X1.5
G0 X0 Y0
G1 X1
Y1
X0
Y0
`,
			actions: []action{
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 1.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 0.0},

				{cmd: rapidTo, x: 1.5, y: 0.0},
				{cmd: linearTo, x: 2.5, y: 0.0},
				{cmd: linearTo, x: 2.5, y: 1.0},
				{cmd: linearTo, x: 1.5, y: 1.0},
				{cmd: linearTo, x: 1.5, y: 0.0},

				{cmd: rapidTo, x: 1.5, y: 1.5},
				{cmd: linearTo, x: 2.5, y: 1.5},
				{cmd: linearTo, x: 2.5, y: 2.5},
				{cmd: linearTo, x: 1.5, y: 2.5},
				{cmd: linearTo, x: 1.5, y: 1.5},

				{cmd: rapidTo, x: 0.0, y: 1.5},
				{cmd: linearTo, x: 1.0, y: 1.5},
				{cmd: linearTo, x: 1.0, y: 2.5},
				{cmd: linearTo, x: 0.0, y: 2.5},
				{cmd: linearTo, x: 0.0, y: 1.5},
			},
		},
		{s: `
G21
G90
G92 X-1.5
G0 X0 Y0
G1 F1 X1
Y1
X0
Y0
G92.1
G0 X0 Y0
G1 X1
Y1
X0
Y0
`,
			actions: []action{
				{cmd: rapidTo, x: 1.5, y: 0.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.5, y: 0.0},
				{cmd: linearTo, x: 2.5, y: 1.0},
				{cmd: linearTo, x: 1.5, y: 1.0},
				{cmd: linearTo, x: 1.5, y: 0.0},

				{cmd: rapidTo, x: 0.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 0.0},
			},
		},
		{s: `
G21
G90
G92 X-1.5
G0 X0 Y0
G1 F1 X1
Y1
X0
Y0
G92.1
G0 X0 Y0
G1 X1
Y1
X0
Y0
G92.3
G92 Y-1.5
G0 X0 Y0
G1 X1
Y1
X0
Y0
`,
			actions: []action{
				{cmd: rapidTo, x: 1.5, y: 0.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.5, y: 0.0},
				{cmd: linearTo, x: 2.5, y: 1.0},
				{cmd: linearTo, x: 1.5, y: 1.0},
				{cmd: linearTo, x: 1.5, y: 0.0},

				{cmd: rapidTo, x: 0.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 0.0},

				{cmd: rapidTo, x: 0.0, y: 1.5},
				{cmd: linearTo, x: 1.0, y: 1.5},
				{cmd: linearTo, x: 1.0, y: 2.5},
				{cmd: linearTo, x: 0.0, y: 2.5},
				{cmd: linearTo, x: 0.0, y: 1.5},
			},
		},
		{s: `
G21
G90
G92 X-1.5
G0 X0 Y0
G1 F1 X1
Y1
X0
Y0
G92.2
G0 X0 Y0
G1 X1
Y1
X0
Y0
G92.3
G92 Y-1.5
G0 X0 Y0
G1 X1
Y1
X0
Y0
`,
			actions: []action{
				{cmd: rapidTo, x: 1.5, y: 0.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.5, y: 0.0},
				{cmd: linearTo, x: 2.5, y: 1.0},
				{cmd: linearTo, x: 1.5, y: 1.0},
				{cmd: linearTo, x: 1.5, y: 0.0},

				{cmd: rapidTo, x: 0.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 0.0},

				{cmd: rapidTo, x: 1.5, y: 1.5},
				{cmd: linearTo, x: 2.5, y: 1.5},
				{cmd: linearTo, x: 2.5, y: 2.5},
				{cmd: linearTo, x: 1.5, y: 2.5},
				{cmd: linearTo, x: 1.5, y: 1.5},
			},
		},
		{s: `
G21
G90
G0 X5 Y5
G28.1
G0 X0 Y0
G1 F1
X1
Y1
X0
Y0
G28
G91
G1 X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 5.0, y: 5.0},

				{cmd: rapidTo, x: 0.0, y: 0.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 1.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 0.0},

				{cmd: rapidTo, x: 5.0, y: 5.0},
				{cmd: linearTo, x: 6.0, y: 5.0},
				{cmd: linearTo, x: 6.0, y: 6.0},
				{cmd: linearTo, x: 5.0, y: 6.0},
				{cmd: linearTo, x: 5.0, y: 5.0},
			},
		},
		{s: `
G21
G90
G0 X4 Y4
G30.1
G0 X0 Y0
G1 F1
X1
Y1
X0
Y0
G30
G91
G1 X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 4.0, y: 4.0},

				{cmd: rapidTo, x: 0.0, y: 0.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 1.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 0.0},

				{cmd: rapidTo, x: 4.0, y: 4.0},
				{cmd: linearTo, x: 5.0, y: 4.0},
				{cmd: linearTo, x: 5.0, y: 5.0},
				{cmd: linearTo, x: 4.0, y: 5.0},
				{cmd: linearTo, x: 4.0, y: 4.0},
			},
		},
		{s: `
G21
G10 L2 P1 X0 Y0
G10 L2 P2 X0 Y-2
G90
G1 F1

G54
G0 X0 Y0
G1 X1
Y1
X0
Y0

G55
G0 X0 Y0
G1 X1
Y1
X0
Y0

G54
G0 X0 Y0
G92 X-2
G0 X0 Y0
G1 X1
Y1
X0
Y0

G55
G0 X0 Y0
G1 X1
Y1
X0
Y0

G92 X-2
G0 X0 Y0
G1 X1
Y1
X0
Y0

G54
G0 X0 Y0
G1 X1
Y1
X0
Y0
`,
			actions: []action{
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 1.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 0.0},

				{cmd: rapidTo, x: 0.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 3.0},
				{cmd: linearTo, x: 0.0, y: 3.0},
				{cmd: linearTo, x: 0.0, y: 2.0},

				{cmd: rapidTo, x: 0.0, y: 0.0},
				{cmd: rapidTo, x: 2.0, y: 0.0},
				{cmd: linearTo, x: 3.0, y: 0.0},
				{cmd: linearTo, x: 3.0, y: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0},
				{cmd: linearTo, x: 2.0, y: 0.0},

				{cmd: rapidTo, x: 2.0, y: 2.0},
				{cmd: linearTo, x: 3.0, y: 2.0},
				{cmd: linearTo, x: 3.0, y: 3.0},
				{cmd: linearTo, x: 2.0, y: 3.0},
				{cmd: linearTo, x: 2.0, y: 2.0},

				{cmd: rapidTo, x: 4.0, y: 2.0},
				{cmd: linearTo, x: 5.0, y: 2.0},
				{cmd: linearTo, x: 5.0, y: 3.0},
				{cmd: linearTo, x: 4.0, y: 3.0},
				{cmd: linearTo, x: 4.0, y: 2.0},

				{cmd: rapidTo, x: 4.0, y: 0.0},
				{cmd: linearTo, x: 5.0, y: 0.0},
				{cmd: linearTo, x: 5.0, y: 1.0},
				{cmd: linearTo, x: 4.0, y: 1.0},
				{cmd: linearTo, x: 4.0, y: 0.0},
			},
		},
	}

	for i, c := range cases {
		eng := engine.NewEngine(&machine{actions: c.actions}, parser.BeagleG)
		err := eng.Evaluate(strings.NewReader(c.s))
		if err != nil {
			t.Errorf("Evaluate(%d) failed: %s", i, err)
		}
	}
}

func TestCoordSys(t *testing.T) {
	cases := []struct {
		s       string
		actions []action
	}{
		{s: `
G21
G10 L2 P1 X0 Y0
G10 L2 P2 X-1 Y-1
G10 L2 P3 X-2 Y-2
G10 L2 P4 X-2 Y-2
G10 L2 P5 X-2 Y-2
G10 L2 P6 X-2 Y-2
G10 L2 P7 X-2 Y-2
G10 L2 P8 X-2 Y-2
G10 L2 P9 X-2 Y-2

G55
G90
G0 X0 Y0
G91
G1 F1
X1
Y1
X-1
Y-1

G%s
G90
G0 X0 Y0
G91
G1 F1
X1
Y1
X-1
Y-1

G54
G90
G0 X0 Y0
G91
G1 F1
X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 1.0, y: 1.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 1.0},

				{cmd: rapidTo, x: 2.0, y: 2.0},
				{cmd: linearTo, x: 3.0, y: 2.0},
				{cmd: linearTo, x: 3.0, y: 3.0},
				{cmd: linearTo, x: 2.0, y: 3.0},
				{cmd: linearTo, x: 2.0, y: 2.0},

				{cmd: rapidTo, x: 0.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 0.0},
			},
		},
		{s: `
G21
G90
G0 X0 Y0
G10 L20 P1 X0 Y0
G0 X1 Y1
G10 L20 P2 X0 Y0
G10 L20 P3 X-1 Y-1
G0 X5 Y5
G10 L20 P4 X3 Y3
G0 X0
G10 L20 P5 X-2 Y3
G0 X5 Y0
G10 L20 P6 X3 Y-2
G0 X0 Y0
G10 L20 P7 X-2 Y-2
G10 L20 P8 X-2 Y-2
G10 L20 P9 X-2 Y-2

G55
G90
G0 X0 Y0
G91
G1 F1
X1
Y1
X-1
Y-1

G%s
G90
G0 X0 Y0
G91
G1 F1
X1
Y1
X-1
Y-1

G54
G90
G0 X0 Y0
G91
G1 F1
X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 1.0, y: 1.0},
				{cmd: rapidTo, x: 5.0, y: 5.0},
				{cmd: rapidTo, x: 0.0, y: 5.0},
				{cmd: rapidTo, x: 5.0, y: 0.0},
				{cmd: rapidTo, x: 0.0, y: 0.0},

				{cmd: rapidTo, x: 1.0, y: 1.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 2.0},
				{cmd: linearTo, x: 1.0, y: 1.0},

				{cmd: rapidTo, x: 2.0, y: 2.0},
				{cmd: linearTo, x: 3.0, y: 2.0},
				{cmd: linearTo, x: 3.0, y: 3.0},
				{cmd: linearTo, x: 2.0, y: 3.0},
				{cmd: linearTo, x: 2.0, y: 2.0},

				{cmd: rapidTo, x: 0.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 0.0},
				{cmd: linearTo, x: 1.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 1.0},
				{cmd: linearTo, x: 0.0, y: 0.0},
			},
		},
	}

	for _, cs := range []string{"56", "57", "58", "59", "59.1", "59.2", "59.3"} {
		for i, c := range cases {
			eng := engine.NewEngine(&machine{actions: c.actions}, parser.BeagleG)
			err := eng.Evaluate(strings.NewReader(fmt.Sprintf(c.s, cs)))
			if err != nil {
				t.Errorf("Evaluate(%d) failed: %s", i, err)
			}
		}
	}
}

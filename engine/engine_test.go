package engine_test

import (
	"fmt"
	"strings"
	"testing"

	engine "github.com/leftmike/gcode/engine"
	parser "github.com/leftmike/gcode/parser"
)

const (
	rapidTo = iota
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
		return fmt.Errorf("test: more than %d actions", len(m.actions))
	}

	if act != m.actions[m.adx] {
		return fmt.Errorf("test: expected %#v; got %#v", m.actions[m.adx], act)
	}

	m.adx += 1
	return nil
}

func (m *machine) RapidTo(pos engine.Position) error {
	return m.checkAction(action{cmd: rapidTo, x: pos.X, y: pos.Y, z: pos.Z})
}

func (m *machine) LinearTo(pos engine.Position, feed float64) error {
	return m.checkAction(action{cmd: linearTo, x: pos.X, y: pos.Y, z: pos.Z, f: feed})
}

func TestEvaluate(t *testing.T) {
	cases := []struct {
		s       string
		actions []action
	}{
		{s: `
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
				{cmd: linearTo, x: 1.0, y: 1.0, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0, f: 1.0},
				{cmd: linearTo, x: 1.0, y: 2.0, f: 1.0},
				{cmd: linearTo, x: 1.0, y: 1.0, f: 1.0},
			},
		},
		{s: `
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
				{cmd: linearTo, x: 1.0, y: 1.0, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0, f: 1.0},
				{cmd: linearTo, x: 1.0, y: 2.0, f: 1.0},
				{cmd: linearTo, x: 1.0, y: 1.0, f: 1.0},
			},
		},
		{s: `
G90
G0 X1 Y1
G10 L20 P1 X0 Y0
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
				{cmd: linearTo, x: 1.0, y: 1.0, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0, f: 1.0},
				{cmd: linearTo, x: 1.0, y: 2.0, f: 1.0},
				{cmd: linearTo, x: 1.0, y: 1.0, f: 1.0},
			},
		},
		{s: `
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
				{cmd: linearTo, x: 2.0, y: 2.0, f: 1.0},
				{cmd: linearTo, x: 3.0, y: 2.0, f: 1.0},
				{cmd: linearTo, x: 3.0, y: 3.0, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 3.0, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0, f: 1.0},
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

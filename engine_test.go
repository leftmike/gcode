package gcode_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/leftmike/gcode"
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
	if m.actions == nil {
		return nil
	}

	if m.adx >= len(m.actions) {
		return fmt.Errorf("test: more than %d actions: %#v", len(m.actions), act)
	}

	if act != m.actions[m.adx] {
		return fmt.Errorf("test: at %d expected %#v; got %#v", m.adx, m.actions[m.adx], act)
	}

	m.adx += 1
	return nil
}

func (m *machine) SetFeed(feed float64) error {
	return m.checkAction(action{cmd: setFeed, f: feed})
}

func (m *machine) SetSpindle(speed float64, clockwise bool) error {
	return fmt.Errorf("unexpected set spindle: %d %v", int(speed), clockwise)
}

func (m *machine) SpindleOff() error {
	return errors.New("unexpected spindle off")
}

func (m *machine) SelectTool(tool uint) error {
	return fmt.Errorf("unexpected select tool: %d", tool)
}

func (m *machine) RapidTo(pos gcode.Position) error {
	return m.checkAction(action{cmd: rapidTo, x: pos.X, y: pos.Y, z: pos.Z})
}

func (m *machine) LinearTo(pos gcode.Position) error {
	return m.checkAction(action{cmd: linearTo, x: pos.X, y: pos.Y, z: pos.Z})
}

func (m *machine) HandleUnknown(code gcode.Code, codes []gcode.Code,
	setCurPos func(pos gcode.Position) error) ([]gcode.Code, error) {

	return nil, fmt.Errorf("unexpected code: %s: %v", code, codes)
}

func TestEvaluate(t *testing.T) {
	cases := []struct {
		s       string
		actions []action
	}{
		{s: `
G21
G91
G0 X1 Y1 Z1
G1 F1
X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 1.0, y: 1.0, z: 1.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0, z: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0, z: 1.0},
				{cmd: linearTo, x: 1.0, y: 2.0, z: 1.0},
				{cmd: linearTo, x: 1.0, y: 1.0, z: 1.0},
			},
		},
		{s: `
G20
G91
G0 X0 Y0
G1 F1
X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: setFeed, f: 25.4},
				{cmd: linearTo, x: 25.4, y: 0.0},
				{cmd: linearTo, x: 25.4, y: 25.4},
				{cmd: linearTo, x: 0.0, y: 25.4},
				{cmd: linearTo, x: 0.0, y: 0.0},
			},
		},
		{s: `
G21
G90
G0 X2 Y2 Z2
G28.1
G0 X4 Y4 Z0
G28 Z1
G91
G1 F1 X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 2.0, y: 2.0, z: 2.0},
				{cmd: rapidTo, x: 4.0, y: 4.0, z: 0.0},
				{cmd: rapidTo, x: 4.0, y: 4.0, z: 1.0},
				{cmd: rapidTo, x: 4.0, y: 4.0, z: 2.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 5.0, y: 4.0, z: 2.0},
				{cmd: linearTo, x: 5.0, y: 5.0, z: 2.0},
				{cmd: linearTo, x: 4.0, y: 5.0, z: 2.0},
				{cmd: linearTo, x: 4.0, y: 4.0, z: 2.0},
			},
		},
		{s: `
G21
G90
G0 X2 Y2 Z2
G28.1
G0 X4 Y4 Z0
G28 X1 Y1
G91
G1 F1 X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 2.0, y: 2.0, z: 2.0},
				{cmd: rapidTo, x: 4.0, y: 4.0, z: 0.0},
				{cmd: rapidTo, x: 1.0, y: 1.0, z: 0.0},
				{cmd: rapidTo, x: 2.0, y: 2.0, z: 0.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 3.0, y: 2.0, z: 0.0},
				{cmd: linearTo, x: 3.0, y: 3.0, z: 0.0},
				{cmd: linearTo, x: 2.0, y: 3.0, z: 0.0},
				{cmd: linearTo, x: 2.0, y: 2.0, z: 0.0},
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
G56
G10 L2 P0 X-1 Y-1
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
G0 X1 Y1 Z1
G10 L20 P1 X0 Y0 Z0
G54
G91
G1 X1 F1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 1.0, y: 1.0, z: 1.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0, z: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0, z: 1.0},
				{cmd: linearTo, x: 1.0, y: 2.0, z: 1.0},
				{cmd: linearTo, x: 1.0, y: 1.0, z: 1.0},
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
F2
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
				{cmd: setFeed, f: 2.0},

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
G92 X-1.5 Z-1.0
G0 X0 Y0 Z0
G1 F1 X1
Y1
X0
Y0
G92.1
G0 X0 Y0 Z0
G1 X1
Y1
X0
Y0
`,
			actions: []action{
				{cmd: rapidTo, x: 1.5, y: 0.0, z: 1.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.5, y: 0.0, z: 1.0},
				{cmd: linearTo, x: 2.5, y: 1.0, z: 1.0},
				{cmd: linearTo, x: 1.5, y: 1.0, z: 1.0},
				{cmd: linearTo, x: 1.5, y: 0.0, z: 1.0},

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
		{s: `
G21
G55
G10 L2 P2 X-1 Y-1 Z-1

G55
G90
G0 X0 Y0 Z0
G91
G1 F1
X1
Y1
X-1
Y-1
`,
			actions: []action{
				{cmd: rapidTo, x: 1.0, y: 1.0, z: 1.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0, z: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0, z: 1.0},
				{cmd: linearTo, x: 1.0, y: 2.0, z: 1.0},
				{cmd: linearTo, x: 1.0, y: 1.0, z: 1.0},
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
G90
G53 G0 X2 Y2
G53 G1 X3 Y2
G53
G1 X3 Y3
G53 G1 X2 Y3
G53 G1 X2 Y2
G0 X2 Y2
G1 X3 Y2
G1 X3 Y3
G1 X2 Y3
G1 X2 Y2
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

				{cmd: rapidTo, x: 3.0, y: 3.0},
				{cmd: linearTo, x: 4.0, y: 3.0},
				{cmd: linearTo, x: 4.0, y: 4.0},
				{cmd: linearTo, x: 3.0, y: 4.0},
				{cmd: linearTo, x: 3.0, y: 3.0},
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
G90
G53 G0 X2 Y2
G91
G53 G1 X1 Y0
G53 G1 X0 Y1
G53 G1 X-1 Y0
G53 G1 X0 Y-1
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
M2
G90
G0 X0 Y0
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
G91
G1 F1
X1
Y1
X-1
Y-1
M30
G90
G0 X0 Y0
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
	}

	for i, c := range cases {
		eng := gcode.NewEngine(&machine{actions: c.actions}, gcode.AllFeatures)
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
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 3.0, y: 2.0},
				{cmd: linearTo, x: 3.0, y: 3.0},
				{cmd: linearTo, x: 2.0, y: 3.0},
				{cmd: linearTo, x: 2.0, y: 2.0},

				{cmd: rapidTo, x: 0.0, y: 0.0},
				{cmd: setFeed, f: 1.0},
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
G1 X1
Y1
X-1
Y-1

G54
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
			eng := gcode.NewEngine(&machine{actions: c.actions}, gcode.AllFeatures)
			err := eng.Evaluate(strings.NewReader(fmt.Sprintf(c.s, cs)))
			if err != nil {
				t.Errorf("Evaluate(%d) failed: %s", i, err)
			}
		}
	}
}

func TestEvaluateFail(t *testing.T) {
	cases := []string{
		"G0 L0\n",
		"G0 P0\n",
		"G0 X1 X2\n",
		"G0 D1\n",
		"G0 X<name>\n",
		`G0 X"string"
`,
		"G10 L2 X1\n",
		"G10 P2 X1\n",
		"G10 L2 P10 X1\n",
		"G10 L200 P1 X1\n",
		"G92\n",
		"GG\n",
		"G=\n",
		"G<name>\n",
		"G0 X0 Y0\nF1\n",
		"G53 G2 X1 Y1\n",
		"G53\nG3 X1 Y1 R1\n",
	}

	for _, c := range cases {
		eng := gcode.NewEngine(&machine{}, gcode.AllFeatures)
		err := eng.Evaluate(strings.NewReader(c))
		if err == nil {
			t.Errorf("Evaluate(%s) did not fail", c)
		}
	}
}

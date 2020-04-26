package gcode_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/leftmike/gcode"
)

const (
	setFeed = iota
	setSpindle
	spindleOff
	selectTool
	rapidTo
	linearTo
)

type action struct {
	cmd        int
	x, y, z, f float64
	speed      float64
	clockwise  bool
	tool       uint
}

func (act1 action) equal(act2 action) bool {
	return act1.cmd == act2.cmd &&
		gcode.Number(act1.x).Equal(gcode.Number(act2.x)) &&
		gcode.Number(act1.y).Equal(gcode.Number(act2.y)) &&
		gcode.Number(act1.z).Equal(gcode.Number(act2.z)) &&
		gcode.Number(act1.f).Equal(gcode.Number(act2.f)) &&
		gcode.Number(act1.speed).Equal(gcode.Number(act2.speed)) &&
		act1.clockwise == act2.clockwise &&
		act1.tool == act2.tool
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

	if !act.equal(m.actions[m.adx]) {
		return fmt.Errorf("test: at %d expected %#v; got %#v", m.adx, m.actions[m.adx], act)
	}

	m.adx += 1
	return nil
}

func (m *machine) SetFeed(feed float64) error {
	return m.checkAction(action{cmd: setFeed, f: feed})
}

func (m *machine) SetSpindle(speed float64, clockwise bool) error {
	return m.checkAction(action{cmd: setSpindle, speed: speed, clockwise: clockwise})
}

func (m *machine) SpindleOff() error {
	return m.checkAction(action{cmd: spindleOff})
}

func (m *machine) SelectTool(tool uint) error {
	return m.checkAction(action{cmd: selectTool, tool: tool})
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
;G0 X2 Y2 Z2
;G28.1
#5161=2
#5162=2
#5163=2
G0 X4 Y4 Z0
G28 X1 Y1
G91
G1 F1 X1
Y1
X-1
Y-1
`,
			actions: []action{
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
G10 L2 P3 Y-1
#5261=-1
#5220=3
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
;G92 X-1.5 Z-1.0
#5211=-1.5
#5213=-1.0
#5210=1
G0 X0 Y0 Z0
G1 F1 X1
Y1
X0
Y0
;G92.1
#5210=0
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
G90
;G30.1
#5181=4
#5182=4
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
G53 G0 X2 Y2 Z1
G53 G1 X3 Y2 Z1
G53
G1 X3 Y3 Z1
G53 G1 X2 Y3 Z1
G53 G1 X2 Y2 Z1
G0 X2 Y2 Z0
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

				{cmd: rapidTo, x: 2.0, y: 2.0, z: 1.0},
				{cmd: linearTo, x: 3.0, y: 2.0, z: 1.0},
				{cmd: linearTo, x: 3.0, y: 3.0, z: 1.0},
				{cmd: linearTo, x: 2.0, y: 3.0, z: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0, z: 1.0},

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
G53 G0 X2 Y2 Z1
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

				{cmd: rapidTo, x: 2.0, y: 2.0, z: 1.0},
				{cmd: linearTo, x: 3.0, y: 2.0, z: 1.0},
				{cmd: linearTo, x: 3.0, y: 3.0, z: 1.0},
				{cmd: linearTo, x: 2.0, y: 3.0, z: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0, z: 1.0},
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
M3 S3
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
				{cmd: setSpindle, clockwise: true},
				{cmd: setSpindle, speed: 3.0, clockwise: true},
				{cmd: spindleOff},
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
		{s: `
G21
T1
T0
M3
G91
G0 X1 Y1 Z1
S3
M4
G1 F1
X1
Y1
X-1
Y-1
M5
`,
			actions: []action{
				{cmd: selectTool, tool: 1},
				{cmd: selectTool, tool: 0},
				{cmd: setSpindle, clockwise: true},
				{cmd: rapidTo, x: 1.0, y: 1.0, z: 1.0},
				{cmd: setSpindle, speed: 3.0, clockwise: true},
				{cmd: setSpindle, speed: 3.0},
				{cmd: setFeed, f: 1.0},
				{cmd: linearTo, x: 2.0, y: 1.0, z: 1.0},
				{cmd: linearTo, x: 2.0, y: 2.0, z: 1.0},
				{cmd: linearTo, x: 1.0, y: 2.0, z: 1.0},
				{cmd: linearTo, x: 1.0, y: 1.0, z: 1.0},
				{cmd: spindleOff},
			},
		},
	}

	for i, c := range cases {
		eng := gcode.NewEngine(&machine{actions: c.actions}, gcode.AllFeatures, os.Stdout,
			os.Stderr)
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

%s
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
G10 L20 P7 X-2 Y-2 Z10
#5343=0
;G10 L20 P8 X-2 Y-2
#5361=-2
#5362=-2
;G10 L20 P9 X-2 Y-2
#5381=-2
#5382=-2

G55
G90
G0 X0 Y0
G91
G1 F1
X1
Y1
X-1
Y-1

%s
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

	for _, cs := range []string{"G56", "G57", "G58", "G59", "G59.1", "G59.2", "G59.3",
		"#5220=3", "#5220=4", "#5220=5", "#5220=6", "#5220=7", "#5220=8", "#5220=9"} {

		for i, c := range cases {
			eng := gcode.NewEngine(&machine{actions: c.actions}, gcode.AllFeatures, os.Stdout,
				os.Stderr)
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
		"G28 F8\n",
		"G0 I1\n",
		"G0 J2\n",
		"G0 K3\n",
		"G0 R4\n",
		"S-1\n",
		"T-1\n",
		"T1.1\n",
	}

	for _, c := range cases {
		eng := gcode.NewEngine(&machine{}, gcode.AllFeatures, os.Stdout, os.Stderr)
		err := eng.Evaluate(strings.NewReader(c))
		if err == nil {
			t.Errorf("Evaluate(%s) did not fail", c)
		}
	}
}

func TestMisc(t *testing.T) {
	pos := gcode.Position{1, 2, 3}
	if pos.String() != "{x: 1.0000, y: 2.0000, z: 3.0000}" {
		t.Errorf("Position.String() got %s", pos)
	}
}

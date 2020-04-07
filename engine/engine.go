package engine

import (
	"errors"
	"fmt"
	"io"

	parser "github.com/leftmike/gcode/parser"
)

const (
	mmPerInch = 25.4
)

type Position struct {
	X, Y, Z float64
}

var (
	zeroPosition = Position{0.0, 0.0, 0.0}
)

type Machine interface {
	SetFeed(feed float64) error
	RapidTo(pos Position) error
	LinearTo(pos Position) error
}

type moveMode byte

const (
	rapidMove               moveMode = iota // G0
	linearMove                              // G1
	clockwiseArcMove                        // G2
	counterClockwiseArcMove                 // G3
)

type engine struct {
	machine      Machine
	dialect      parser.Dialect
	numParams    map[int]parser.Number
	units        float64 // 1.0 for mm and 25.4 for in
	homePos      Position
	curPos       Position
	maxPos       Position
	curCoordSys  int
	coordSysPos  [9]Position
	workPos      Position
	savedWorkPos Position
	moveMode     moveMode
	absoluteMode bool
}

func NewEngine(m Machine, d parser.Dialect) *engine {
	return &engine{
		machine:     m,
		dialect:     d,
		numParams:   map[int]parser.Number{},
		units:       1.0, // default units is mm
		homePos:     zeroPosition,
		curPos:      zeroPosition, // default current position at home
		maxPos:      Position{mmPerInch * 12.0, mmPerInch * 12.0, mmPerInch * 4.0},
		curCoordSys: 0,
		coordSysPos: [9]Position{
			zeroPosition, zeroPosition, zeroPosition,
			zeroPosition, zeroPosition, zeroPosition,
			zeroPosition, zeroPosition, zeroPosition,
		},
		workPos:      zeroPosition,
		savedWorkPos: zeroPosition,
		moveMode:     linearMove,
		absoluteMode: false,
	}
}

func (eng *engine) getNumParam(num int) (parser.Number, error) {
	val, ok := eng.numParams[num]
	if !ok {
		return 0, fmt.Errorf("engine: number parameter %d not found", num)
	}
	return val, nil
}

func (eng *engine) setNumParam(num int, val parser.Number) error {
	eng.numParams[num] = val
	return nil
}

func (eng *engine) setFeed(feed float64) error {
	return eng.machine.SetFeed(feed)
}

func (eng *engine) rapidTo(pos Position) error {
	if pos == eng.curPos {
		return nil
	}
	err := eng.machine.RapidTo(pos)
	if err != nil {
		return err
	}
	eng.curPos = pos
	return nil
}

func (eng *engine) linearTo(pos Position) error {
	if pos == eng.curPos {
		return nil
	}
	err := eng.machine.LinearTo(pos)
	if err != nil {
		return err
	}
	eng.curPos = pos
	return nil
}

type arg struct {
	letter parser.Letter
	num    parser.Number
}

type argSet int

const (
	fArg = 1 << iota
	lArg
	pArg
	xArg
	yArg
	zArg
)

func parseArgs(codes []parser.Code, allowed argSet) ([]arg, []parser.Code, error) {
	var args []arg
	for len(codes) > 0 {
		code := codes[0]
		switch code.Letter {
		case 'F':
			if (allowed & fArg) == 0 {
				return nil, nil, fmt.Errorf("arg not allowed: %s", code)
			}
		case 'L':
			if (allowed & lArg) == 0 {
				return nil, nil, fmt.Errorf("arg not allowed: %s", code)
			}
		case 'P':
			if (allowed & pArg) == 0 {
				return nil, nil, fmt.Errorf("arg not allowed: %s", code)
			}
		case 'X':
			if (allowed & xArg) == 0 {
				return nil, nil, fmt.Errorf("arg not allowed: %s", code)
			}
		case 'Y':
			if (allowed & yArg) == 0 {
				return nil, nil, fmt.Errorf("arg not allowed: %s", code)
			}
		case 'Z':
			if (allowed & zArg) == 0 {
				return nil, nil, fmt.Errorf("arg not allowed: %s", code)
			}
		default:
			break
		}

		for _, arg := range args {
			if arg.letter == code.Letter {
				fmt.Errorf("duplicate arg specified: %s", code)
			}
		}
		num, ok := code.Value.AsNumber()
		if !ok {
			return nil, nil, fmt.Errorf("expected a number: %v", code.Value)
		}

		args = append(args, arg{code.Letter, num})
		codes = codes[1:]
	}

	return args, codes, nil
}

func requireArg(args []arg, letter parser.Letter) (parser.Number, error) {
	for _, arg := range args {
		if arg.letter == letter {
			return arg.num, nil
		}
	}

	return 0, fmt.Errorf("missing require arg: %c", letter)
}

func (eng *engine) toMachineX(x float64, absolute bool) float64 {
	if absolute {
		return x - eng.coordSysPos[eng.curCoordSys].X - eng.workPos.X
	}
	// relative
	return eng.curPos.X + x
}

func (eng *engine) toMachineY(y float64, absolute bool) float64 {
	if absolute {
		return y - eng.coordSysPos[eng.curCoordSys].Y - eng.workPos.Y
	}
	// relative
	return eng.curPos.Y + y
}

func (eng *engine) toMachineZ(z float64, absolute bool) float64 {
	if absolute {
		return z - eng.coordSysPos[eng.curCoordSys].Z - eng.workPos.Z
	}
	// relative
	return eng.curPos.Z + z
}

func (eng *engine) moveTo(codes []parser.Code) ([]parser.Code, error) {
	var err error
	var args []arg
	args, codes, err = parseArgs(codes, fArg|xArg|yArg|zArg)
	if err != nil {
		return nil, err
	}

	pos := eng.curPos
	for _, arg := range args {
		switch arg.letter {
		case 'F':
			err = eng.setFeed(float64(arg.num) * eng.units)
			if err != nil {
				return nil, err
			}
		case 'X':
			pos.X = eng.toMachineX(float64(arg.num)*eng.units, eng.absoluteMode)
		case 'Y':
			pos.Y = eng.toMachineY(float64(arg.num)*eng.units, eng.absoluteMode)
		case 'Z':
			pos.Z = eng.toMachineZ(float64(arg.num)*eng.units, eng.absoluteMode)
		}
	}

	switch eng.moveMode {
	case rapidMove:
		err = eng.rapidTo(pos)
	case linearMove:
		err = eng.linearTo(pos)
	default:
		panic(fmt.Sprintf("unexpected moveMode: %d", eng.moveMode))
	}
	if err != nil {
		return nil, err
	}

	return codes, nil
}

func (eng *engine) setCoordinateSystemPosition(args []arg, machine bool) error {
	p, err := requireArg(args, 'P')
	if err != nil {
		return err
	}
	var coordSys int
	if p.Equal(0.0) {
		coordSys = eng.curCoordSys
	} else if p.Equal(1.0) {
		coordSys = 0
	} else if p.Equal(2.0) {
		coordSys = 1
	} else if p.Equal(3.0) {
		coordSys = 2
	} else if p.Equal(4.0) {
		coordSys = 3
	} else if p.Equal(5.0) {
		coordSys = 4
	} else if p.Equal(6.0) {
		coordSys = 5
	} else if p.Equal(7.0) {
		coordSys = 6
	} else if p.Equal(8.0) {
		coordSys = 7
	} else if p.Equal(1.0) {
		coordSys = 8
	} else {
		return fmt.Errorf("expected a coordinate system: P%s", p)
	}

	for _, arg := range args {
		switch arg.letter {
		case 'X':
			if machine {
				eng.coordSysPos[coordSys].X = float64(arg.num) * eng.units
			} else {
				eng.coordSysPos[coordSys].X = float64(arg.num)*eng.units - eng.curPos.X
			}
		case 'Y':
			if machine {
				eng.coordSysPos[coordSys].Y = float64(arg.num) * eng.units
			} else {
				eng.coordSysPos[coordSys].Y = float64(arg.num)*eng.units - eng.curPos.Y
			}
		case 'Z':
			if machine {
				eng.coordSysPos[coordSys].Z = float64(arg.num) * eng.units
			} else {
				eng.coordSysPos[coordSys].Z = float64(arg.num)*eng.units - eng.curPos.Z
			}
		}
	}

	return nil
}

func (eng *engine) modifyPositions(codes []parser.Code) ([]parser.Code, error) {
	var err error
	var args []arg
	args, codes, err = parseArgs(codes, lArg|pArg|xArg|yArg|zArg)
	if err != nil {
		return nil, err
	}
	l, err := requireArg(args, 'L')
	if err != nil {
		return nil, err
	}

	if l.Equal(2.0) { // G10 L2: set coordinate system offset (machine)
		err = eng.setCoordinateSystemPosition(args, true)
		if err != nil {
			return nil, err
		}
		return codes, nil
	} else if l.Equal(20.0) { // G10 L20: set coordinate system offset (relative)
		err = eng.setCoordinateSystemPosition(args, false)
		if err != nil {
			return nil, err
		}
		return codes, nil
	}

	return nil, fmt.Errorf("unexpected L value to G10: L%s", l)
}

func (eng *engine) setWorkPosition(codes []parser.Code) ([]parser.Code, error) {
	var err error
	var args []arg
	args, codes, err = parseArgs(codes, xArg|yArg|zArg)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, errors.New("expected at least one X, Y, or Z arg")
	}

	for _, arg := range args {
		switch arg.letter {
		case 'X':
			eng.workPos.X += eng.toMachineX(float64(arg.num)*eng.units, true) - eng.curPos.X
		case 'Y':
			eng.workPos.Y += eng.toMachineY(float64(arg.num)*eng.units, true) - eng.curPos.Y
		case 'Z':
			eng.workPos.Z += eng.toMachineZ(float64(arg.num)*eng.units, true) - eng.curPos.Z
		}
	}

	return codes, nil
}

func (eng *engine) Evaluate(s io.ByteScanner) error {
	p := parser.Parser{
		Scanner:     s,
		Dialect:     eng.dialect,
		GetNumParam: eng.getNumParam,
		SetNumParam: eng.setNumParam,
	}

	for {
		codes, err := p.Parse()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		for len(codes) > 0 {
			code := codes[0]
			num, ok := code.Value.AsNumber()
			if !ok {
				return fmt.Errorf("expected a number: %s", code)
			}

			switch code.Letter {
			case 'G':
				codes = codes[1:]

				if num.Equal(0.0) { // G0: rapid move
					eng.moveMode = rapidMove
					codes, err = eng.moveTo(codes)
					if err != nil {
						return err
					}
				} else if num.Equal(1.0) { // G1: linear move
					eng.moveMode = linearMove
					codes, err = eng.moveTo(codes)
					if err != nil {
						return err
					}
				} else if num.Equal(10.0) { // G10
					codes, err = eng.modifyPositions(codes)
					if err != nil {
						return nil
					}
				} else if num.Equal(20.0) { // G20: coordinates in inches
					eng.units = mmPerInch
				} else if num.Equal(21.0) { // G21: coordinates in mm
					eng.units = 1.0
				} else if num.Equal(28.0) { // G28: home the machine
					// XXX: handle the optional waypoint to go through
					err = eng.rapidTo(eng.homePos)
					if err != nil {
						return err
					}
				} else if num.Equal(54.0) { // G54: use coordinate system one
					eng.curCoordSys = 0
				} else if num.Equal(55.0) { // G54: use coordinate system two
					eng.curCoordSys = 1
				} else if num.Equal(56.0) { // G54: use coordinate system three
					eng.curCoordSys = 2
				} else if num.Equal(57.0) { // G54: use coordinate system four
					eng.curCoordSys = 3
				} else if num.Equal(58.0) { // G54: use coordinate system five
					eng.curCoordSys = 4
				} else if num.Equal(59.0) { // G54: use coordinate system six
					eng.curCoordSys = 5
				} else if num.Equal(59.1) { // G54: use coordinate system seven
					eng.curCoordSys = 6
				} else if num.Equal(59.2) { // G54: use coordinate system eight
					eng.curCoordSys = 7
				} else if num.Equal(59.3) { // G54: use coordinate system nine
					eng.curCoordSys = 8
				} else if num.Equal(90.0) { // G90: absolute distance mode
					eng.absoluteMode = true
				} else if num.Equal(91.0) { // G91: incremental distance mode
					eng.absoluteMode = false
				} else if num.Equal(92.0) { // G92: set work position
					codes, err = eng.setWorkPosition(codes)
					if err != nil {
						return nil
					}
				} else if num.Equal(92.1) { // G92.1: zero work position
					eng.workPos = zeroPosition
					eng.savedWorkPos = zeroPosition
				} else if num.Equal(92.2) { // G92.2: save work position, then zero
					eng.savedWorkPos = eng.workPos
					eng.workPos = zeroPosition
				} else if num.Equal(92.3) { // G92.3: restore saved work position
					eng.workPos = eng.savedWorkPos
				} else {
					/*
					   G2, G3: arc move
					   G5: cubic spline
					   G5.1: quadratic spline
					   G17: XY plane selection
					   G18: ZX plane selection
					   G19: YZ plane selection
					   G53: use absolute coordinates
					*/
					return fmt.Errorf("unexpected code: %s: %v", code, codes)
				}
			case 'M':
				// XXX
				codes = nil
			case 'F':
				if eng.moveMode == linearMove {
					codes, err = eng.moveTo(codes)
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("arg not allowed: %s", code)
				}
			case 'X', 'Y', 'Z':
				switch eng.moveMode {
				case rapidMove, linearMove:
					codes, err = eng.moveTo(codes)
					if err != nil {
						return err
					}
				default:
					return fmt.Errorf("arg not allowed: %s", code)
				}
			default:
				return fmt.Errorf("unexpected code: %s", code)
			}
		}
	}

	return nil
}

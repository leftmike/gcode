package gcode

/*
To Do:
- RepRap:
-- parser: support {} instead of [] for expressions

- LinuxCNC:
-- #1 to #30 are subroutine parameters and are local to the subroutine
-- #<name> are local to the scope where it is assigned; scoped to subroutines
-- #31 and above, and #<_name> are global
-- O codes
-- O codes can be followed by a number or a name
-- Subroutines can change the value of parameters above #30 and those changes will be visible to the calling code. Subroutines may also change the value of global named parameters.

- predefined parameters
*/

import (
	"errors"
	"fmt"
	"io"
)

const (
	mmPerInch = 25.4
)

type Position struct {
	X, Y, Z float64
}

func (pos Position) String() string {
	return fmt.Sprintf("{x: %s, y: %s, z: %s}", Number(pos.X), Number(pos.Y), Number(pos.Z))
}

var (
	zeroPosition = Position{0.0, 0.0, 0.0}
)

type Machine interface {
	SetFeed(feed float64) error
	SetSpindle(speed float64, clockwise bool) error
	SpindleOff() error
	SelectTool(tool uint) error
	RapidTo(pos Position) error
	LinearTo(pos Position) error
	HandleUnknown(code Code, codes []Code, setCurPos func(pos Position) error) ([]Code, error)
}

type moveMode byte

const (
	rapidMove               moveMode = iota // G0
	linearMove                              // G1
	clockwiseArcMove                        // G2
	counterClockwiseArcMove                 // G3
)

type Plane byte

const (
	XYPlane Plane = iota // G17
	ZXPlane              // G18
	YZPlane              // G19
)

type engine struct {
	machine          Machine
	features         Features
	outW             io.Writer
	errW             io.Writer
	numParams        map[int]Number
	nameParams       map[Name]Value
	units            float64 // 1.0 for mm and 25.4 for in
	homePos          Position
	secondPos        Position
	curPos           Position
	maxPos           Position
	curCoordSys      int
	coordSysPos      [9]Position
	workPos          Position
	savedWorkPos     Position
	moveMode         moveMode
	absoluteMode     bool
	absoluteArcMode  bool
	arcPlane         Plane
	spindleOn        bool
	spindleSpeed     float64
	spindleClockwise bool
}

func NewEngine(m Machine, f Features, outW, errW io.Writer) *engine {
	return &engine{
		machine:     m,
		features:    f,
		outW:        outW,
		errW:        errW,
		numParams:   map[int]Number{},
		nameParams:  map[Name]Value{},
		units:       1.0, // default units is mm
		homePos:     zeroPosition,
		secondPos:   zeroPosition,
		curPos:      zeroPosition, // default current position at home
		maxPos:      Position{mmPerInch * 12.0, mmPerInch * 12.0, mmPerInch * 4.0},
		curCoordSys: 0,
		coordSysPos: [9]Position{
			zeroPosition, zeroPosition, zeroPosition,
			zeroPosition, zeroPosition, zeroPosition,
			zeroPosition, zeroPosition, zeroPosition,
		},
		workPos:          zeroPosition,
		savedWorkPos:     zeroPosition,
		moveMode:         linearMove,
		absoluteMode:     true,
		absoluteArcMode:  false,
		arcPlane:         XYPlane,
		spindleOn:        false,
		spindleSpeed:     0.0,
		spindleClockwise: true,
	}
}

func (eng *engine) endProgram() error {
	eng.moveMode = linearMove
	eng.curCoordSys = 0
	eng.arcPlane = XYPlane
	eng.absoluteMode = true
	if eng.spindleOn {
		eng.spindleOn = false
		return eng.spindleOff()
	}
	return nil
}

func (eng *engine) getNumParam(num int) (Number, bool) {
	val, ok := eng.numParams[num]
	if !ok {
		return 0, false
	}
	return val, true
}

func (eng *engine) setNumParam(num int, val Number) error {
	eng.numParams[num] = val
	return nil
}

func (eng *engine) getNameParam(name Name) (Value, bool) {
	val, ok := eng.nameParams[name]
	if !ok {
		return nil, false
	}
	return val, true
}

func (eng *engine) setNameParam(name Name, val Value) error {
	eng.nameParams[name] = val
	return nil
}

func (eng *engine) setFeed(feed float64) error {
	return eng.machine.SetFeed(feed)
}

func (eng *engine) setSpindle(speed float64, clockwise bool) error {
	return eng.machine.SetSpindle(speed, clockwise)
}

func (eng *engine) spindleOff() error {
	return eng.machine.SpindleOff()
}

func (eng *engine) selectTool(tool uint) error {
	return eng.machine.SelectTool(tool)
}

func (eng *engine) handleUnknown(code Code, codes []Code,
	setCurPos func(pos Position) error) ([]Code, error) {

	return eng.machine.HandleUnknown(code, codes, setCurPos)
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

func (eng *engine) setCurrentPosition(pos Position) error {
	eng.curPos = pos
	return nil
}

type arg struct {
	letter Letter
	num    Number
}

type argSet int

const (
	fArg = 1 << iota
	iArg
	jArg
	kArg
	lArg
	pArg
	rArg
	xArg
	yArg
	zArg
)

func parseArgs(codes []Code, allowed argSet) ([]arg, []Code, error) {
	var args []arg
	for len(codes) > 0 {
		code := codes[0]
		switch code.Letter {
		case 'F':
			if (allowed & fArg) == 0 {
				return nil, nil, fmt.Errorf("arg not allowed: %s", code)
			}
		case 'I':
			if (allowed & iArg) == 0 {
				return nil, nil, fmt.Errorf("arg not allowed: %s", code)
			}
		case 'J':
			if (allowed & jArg) == 0 {
				return nil, nil, fmt.Errorf("arg not allowed: %s", code)
			}
		case 'K':
			if (allowed & kArg) == 0 {
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
		case 'R':
			if (allowed & rArg) == 0 {
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
			return args, codes, nil
		}

		for _, arg := range args {
			if arg.letter == code.Letter {
				return nil, nil, fmt.Errorf("duplicate arg specified: %s", code)
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

func requireArg(args []arg, letter Letter) (Number, error) {
	for _, arg := range args {
		if arg.letter == letter {
			return arg.num, nil
		}
	}

	return 0, fmt.Errorf("missing require arg: %c", letter)
}

func hasArg(args []arg, letter Letter) bool {
	for _, arg := range args {
		if arg.letter == letter {
			return true
		}
	}

	return false
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

func (eng *engine) moveTo(codes []Code, useMachine bool) ([]Code, error) {
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
			if useMachine {
				if eng.absoluteMode {
					pos.X = float64(arg.num) * eng.units
				} else {
					pos.X = eng.curPos.X + float64(arg.num)*eng.units
				}
			} else {
				pos.X = eng.toMachineX(float64(arg.num)*eng.units, eng.absoluteMode)
			}
		case 'Y':
			if useMachine {
				if eng.absoluteMode {
					pos.Y = float64(arg.num) * eng.units
				} else {
					pos.Y = eng.curPos.Y + float64(arg.num)*eng.units
				}
			} else {
				pos.Y = eng.toMachineY(float64(arg.num)*eng.units, eng.absoluteMode)
			}
		case 'Z':
			if useMachine {
				if eng.absoluteMode {
					pos.Z = float64(arg.num) * eng.units
				} else {
					pos.Z = eng.curPos.Z + float64(arg.num)*eng.units
				}
			} else {
				pos.Z = eng.toMachineZ(float64(arg.num)*eng.units, eng.absoluteMode)
			}
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

func (eng *engine) moveToPredefined(codes []Code, pos Position) ([]Code, error) {
	var err error
	var args []arg
	args, codes, err = parseArgs(codes, xArg|yArg|zArg)
	if err != nil {
		return nil, err
	}

	if len(args) == 0 {
		err = eng.rapidTo(pos)
		if err != nil {
			return nil, err
		}
	} else {
		way := eng.curPos
		final := eng.curPos
		for _, arg := range args {
			switch arg.letter {
			case 'X':
				way.X = eng.toMachineX(float64(arg.num)*eng.units, eng.absoluteMode)
				final.X = pos.X
			case 'Y':
				way.Y = eng.toMachineY(float64(arg.num)*eng.units, eng.absoluteMode)
				final.Y = pos.Y
			case 'Z':
				way.Z = eng.toMachineZ(float64(arg.num)*eng.units, eng.absoluteMode)
				final.Z = pos.Z
			}
		}

		err = eng.rapidTo(way)
		if err != nil {
			return nil, err
		}
		err = eng.rapidTo(final)
		if err != nil {
			return nil, err
		}
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
	} else if p.Equal(9.0) {
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

func (eng *engine) modifyPositions(codes []Code) ([]Code, error) {
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

func (eng *engine) setWorkPosition(codes []Code) ([]Code, error) {
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
	eng.savedWorkPos = eng.workPos

	return codes, nil
}

func (eng *engine) Evaluate(s io.ByteScanner) error {
	p := Parser{
		Scanner:      s,
		Features:     eng.features,
		OutW:         eng.outW,
		ErrW:         eng.errW,
		GetNumParam:  eng.getNumParam,
		SetNumParam:  eng.setNumParam,
		GetNameParam: eng.getNameParam,
		SetNameParam: eng.setNameParam,
	}

	for {
		codes, err := p.Parse()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		useMachine := false
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
					codes, err = eng.moveTo(codes, useMachine)
					if err != nil {
						return err
					}
				} else if num.Equal(1.0) { // G1: linear move
					eng.moveMode = linearMove
					codes, err = eng.moveTo(codes, useMachine)
					if err != nil {
						return err
					}
				} else if num.Equal(2.0) { // G2: clockwise arc move
					eng.moveMode = clockwiseArcMove
					codes, err = eng.arcTo(codes, useMachine)
					if err != nil {
						return err
					}
				} else if num.Equal(3.0) { // G3: counter-clockwise arc move
					eng.moveMode = counterClockwiseArcMove
					codes, err = eng.arcTo(codes, useMachine)
					if err != nil {
						return err
					}
				} else if num.Equal(10.0) { // G10
					codes, err = eng.modifyPositions(codes)
					if err != nil {
						return err
					}
				} else if num.Equal(17.0) { // G17: XY plane selection
					eng.arcPlane = XYPlane
				} else if num.Equal(18.0) { // G18: ZX plane selection
					eng.arcPlane = ZXPlane
				} else if num.Equal(19.0) { // G19: YZ plane selection
					eng.arcPlane = YZPlane
				} else if num.Equal(20.0) { // G20: coordinates in inches
					eng.units = mmPerInch
				} else if num.Equal(21.0) { // G21: coordinates in mm
					eng.units = 1.0
				} else if num.Equal(28.0) { // G28: go home
					codes, err = eng.moveToPredefined(codes, eng.homePos)
					if err != nil {
						return err
					}
				} else if num.Equal(28.1) { // G28.1: set home
					eng.homePos = eng.curPos
				} else if num.Equal(30.0) { // G30: go predefined position
					codes, err = eng.moveToPredefined(codes, eng.secondPos)
					if err != nil {
						return err
					}
				} else if num.Equal(30.1) { // G30.1: set predefined position
					eng.secondPos = eng.curPos
				} else if num.Equal(53.0) { // G53: move in machine coordinates
					useMachine = true
					if len(codes) == 0 {
						codes, err = p.Parse()
						if err == io.EOF {
							return nil
						} else if err != nil {
							return err
						}
					}
				} else if num.Equal(54.0) { // G54: use coordinate system one
					eng.curCoordSys = 0
				} else if num.Equal(55.0) { // G55: use coordinate system two
					eng.curCoordSys = 1
				} else if num.Equal(56.0) { // G56: use coordinate system three
					eng.curCoordSys = 2
				} else if num.Equal(57.0) { // G57: use coordinate system four
					eng.curCoordSys = 3
				} else if num.Equal(58.0) { // G58: use coordinate system five
					eng.curCoordSys = 4
				} else if num.Equal(59.0) { // G59: use coordinate system six
					eng.curCoordSys = 5
				} else if num.Equal(59.1) { // G59.1: use coordinate system seven
					eng.curCoordSys = 6
				} else if num.Equal(59.2) { // G59.2: use coordinate system eight
					eng.curCoordSys = 7
				} else if num.Equal(59.3) { // G59.3: use coordinate system nine
					eng.curCoordSys = 8
				} else if num.Equal(90.0) { // G90: absolute distance mode
					eng.absoluteMode = true
				} else if num.Equal(90.1) { // G90.1: absolute arc mode
					eng.absoluteArcMode = true
				} else if num.Equal(91.0) { // G91: incremental distance mode
					eng.absoluteMode = false
				} else if num.Equal(91.1) { // G91.1: incremental arc mode
					eng.absoluteArcMode = false
				} else if num.Equal(92.0) { // G92: set work position
					codes, err = eng.setWorkPosition(codes)
					if err != nil {
						return err
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
					codes, err = eng.handleUnknown(code, codes, eng.setCurrentPosition)
					if err != nil {
						return err
					}
				}
			case 'M':
				codes = codes[1:]

				if num.Equal(2.0) || num.Equal(30.0) { // M2, M3: end program
					return eng.endProgram()
				} else if num.Equal(3.0) { // M3: spindle on clockwise
					eng.spindleOn = true
					eng.spindleClockwise = true
					err = eng.setSpindle(eng.spindleSpeed, eng.spindleClockwise)
					if err != nil {
						return err
					}
				} else if num.Equal(4.0) { // M4: spindle on counter-clockwise
					eng.spindleOn = true
					eng.spindleClockwise = false
					err = eng.setSpindle(eng.spindleSpeed, eng.spindleClockwise)
					if err != nil {
						return err
					}
				} else if num.Equal(5.0) { // M5: spindle off
					eng.spindleOn = false
					err = eng.spindleOff()
					if err != nil {
						return err
					}
				} else {
					codes, err = eng.handleUnknown(code, codes, eng.setCurrentPosition)
					if err != nil {
						return err
					}
				}
			case 'F':
				switch eng.moveMode {
				case linearMove:
					codes, err = eng.moveTo(codes, useMachine)
					if err != nil {
						return err
					}
				case clockwiseArcMove, counterClockwiseArcMove:
					codes, err = eng.arcTo(codes, useMachine)
					if err != nil {
						return err
					}
				default:
					return fmt.Errorf("arg not allowed: %s", code)
				}
			case 'I', 'J', 'K', 'P', 'R':
				switch eng.moveMode {
				case clockwiseArcMove, counterClockwiseArcMove:
					codes, err = eng.arcTo(codes, useMachine)
					if err != nil {
						return err
					}
				default:
					return fmt.Errorf("arg not allowed: %s", code)
				}
			case 'S':
				if num < 0.0 {
					return fmt.Errorf("spindle speed must not be negative: %s", num)
				}

				codes = codes[1:]
				eng.spindleSpeed = float64(num)
				if eng.spindleOn {
					err = eng.setSpindle(eng.spindleSpeed, eng.spindleClockwise)
					if err != nil {
						return err
					}
				}
			case 'T':
				codes = codes[1:]
				tool, ok := num.AsInteger()
				if !ok || tool < 0 {
					return fmt.Errorf("expected a non-negative integer: T%s", num)
				}
				err = eng.selectTool(uint(tool))
				if err != nil {
					return err
				}
			case 'X', 'Y', 'Z':
				switch eng.moveMode {
				case rapidMove, linearMove:
					codes, err = eng.moveTo(codes, useMachine)
					if err != nil {
						return err
					}
				case clockwiseArcMove, counterClockwiseArcMove:
					codes, err = eng.arcTo(codes, useMachine)
					if err != nil {
						return err
					}
				default:
					return fmt.Errorf("arg not allowed: %s", code)
				}
			default:
				codes = codes[1:]
				codes, err = eng.handleUnknown(code, codes, eng.setCurrentPosition)
				if err != nil {
					return err
				}
			}
		}
	}

	// Never reached.
}

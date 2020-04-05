package engine

import (
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

func (pos *Position) add(pos2 Position) {
	pos.X += pos2.X
	pos.Y += pos2.Y
	pos.Z += pos2.Z
}

type Machine interface {
	RapidTo(pos Position) error
	LinearTo(pos Position, feed float64) error
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
	feed         float64
	units        float64 // 1.0 for mm and 25.4 for in
	homePos      Position
	curPos       Position
	maxPos       Position
	curCoord     int
	coordPos     [9]Position
	moveMode     moveMode
	absoluteMode bool
}

func NewEngine(m Machine, d parser.Dialect) *engine {
	return &engine{
		machine:   m,
		dialect:   d,
		numParams: map[int]parser.Number{},
		feed:      0.0,
		units:     1.0, // default units is mm
		homePos:   Position{0.0, 0.0, 0.0},
		curPos:    Position{0.0, 0.0, 0.0}, // default current position at home
		maxPos:    Position{mmPerInch * 12.0, mmPerInch * 12.0, mmPerInch * 4.0},
		curCoord:  0,
		coordPos: [9]Position{
			{0.0, 0.0, 0.0}, {0.0, 0.0, 0.0}, {0.0, 0.0, 0.0},
			{0.0, 0.0, 0.0}, {0.0, 0.0, 0.0}, {0.0, 0.0, 0.0},
			{0.0, 0.0, 0.0}, {0.0, 0.0, 0.0}, {0.0, 0.0, 0.0},
		},
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

type axis struct {
	letter parser.Letter
	num    parser.Number
}

type axes int

const (
	fAxis = 1 << iota
	xAxis
	yAxis
	zAxis
)

func parseAxes(codes []parser.Code, allowed axes) ([]axis, []parser.Code, error) {
	var axes []axis
	for len(codes) > 0 {
		code := codes[0]
		switch code.Letter {
		case 'F':
			if (allowed & fAxis) == 0 {
				return nil, nil, fmt.Errorf("axis not allowed: %s", code)
			}
		case 'X':
			if (allowed & xAxis) == 0 {
				return nil, nil, fmt.Errorf("axis not allowed: %s", code)
			}
		case 'Y':
			if (allowed & yAxis) == 0 {
				return nil, nil, fmt.Errorf("axis not allowed: %s", code)
			}
		case 'Z':
			if (allowed & zAxis) == 0 {
				return nil, nil, fmt.Errorf("axis not allowed: %s", code)
			}
		default:
			break
		}

		for _, axis := range axes {
			if axis.letter == code.Letter {
				fmt.Errorf("duplicate axis specified: %s", code)
			}
		}
		num, ok := code.Value.AsNumber()
		if !ok {
			return nil, nil, fmt.Errorf("expected a number: %v", code.Value)
		}

		axes = append(axes, axis{code.Letter, num})
		codes = codes[1:]
	}

	return axes, codes, nil
}

func (eng *engine) moveTo(codes []parser.Code) ([]parser.Code, error) {
	var err error
	var axes []axis
	axes, codes, err = parseAxes(codes, fAxis|xAxis|yAxis|zAxis)
	if err != nil {
		return nil, err
	}

	var pos Position
	for _, axis := range axes {
		switch axis.letter {
		case 'F':
			eng.feed = float64(axis.num)
		case 'X':
			pos.X = float64(axis.num)
		case 'Y':
			pos.Y = float64(axis.num)
		case 'Z':
			pos.Z = float64(axis.num)
		}
	}

	if eng.absoluteMode {
		pos.add(eng.coordPos[eng.curCoord])
	} else {
		pos.add(eng.curPos)
	}

	switch eng.moveMode {
	case rapidMove:
		err = eng.machine.RapidTo(pos)
	case linearMove:
		err = eng.machine.LinearTo(pos, eng.feed)
	default:
		panic(fmt.Sprintf("unexpected moveMode: %d", eng.moveMode))
	}
	if err != nil {
		return nil, err
	}
	eng.curPos = pos

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
			codes = codes[1:]
			num, ok := code.Value.AsNumber()
			if !ok {
				return fmt.Errorf("expected a number: %v", code.Value)
			}

			switch code.Letter {
			case 'G':
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
				} else if num.Equal(20.0) { // G20: coordinates in inches
					eng.units = mmPerInch
				} else if num.Equal(21.0) { // G21: coordinates in mm
					eng.units = 1.0
				} else if num.Equal(28.0) { // G28: home the machine
					// XXX: handle the optional waypoint to go through
					err = eng.machine.RapidTo(eng.homePos)
					if err != nil {
						return err
					}
					eng.curPos = eng.homePos
				} else if num.Equal(54.0) { // G54: use coordinate system one
					eng.curCoord = 0
				} else if num.Equal(55.0) { // G54: use coordinate system two
					eng.curCoord = 1
				} else if num.Equal(56.0) { // G54: use coordinate system three
					eng.curCoord = 2
				} else if num.Equal(57.0) { // G54: use coordinate system four
					eng.curCoord = 3
				} else if num.Equal(58.0) { // G54: use coordinate system five
					eng.curCoord = 4
				} else if num.Equal(59.0) { // G54: use coordinate system six
					eng.curCoord = 5
				} else if num.Equal(59.1) { // G54: use coordinate system seven
					eng.curCoord = 6
				} else if num.Equal(59.2) { // G54: use coordinate system eight
					eng.curCoord = 7
				} else if num.Equal(59.3) { // G54: use coordinate system nine
					eng.curCoord = 8
				} else if num.Equal(90.0) { // G90: absolute distance mode
					eng.absoluteMode = true
				} else if num.Equal(91.0) { // G91: incremental distance mode
					eng.absoluteMode = false
				} else {
					/*
					   G2, G3: arc move
					   G5: cubic spline
					   G5.1: quadratic spline
					   G10 L2: set coordinate system offset
					   G10 L20: set coordinate system offset
					   G17: XY plane selection
					   G18: ZX plane selection
					   G19: YZ plane selection
					   G53: use absolute coordinates
					   G92: set position
					*/
					fmt.Printf("%v\n", codes)
					return fmt.Errorf("unexpected code: %s", code)
				}
			case 'M':
			case 'F':
			case 'X':
			case 'Y':
			case 'Z':
			default:
				return fmt.Errorf("unexpected code: %s", code)
			}

			//			fmt.Printf("%s ", code)
		}
		//		fmt.Println()
	}

	return nil
}

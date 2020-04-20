package gcode

import (
	"errors"
	"fmt"
	"math"
)

func hypot(pos1, pos2 Position) float64 {
	return math.Hypot(pos1.X-pos2.X, pos1.Y-pos2.Y)
}

func radiusCenter(curPos, endPos Position, radius float64, clockwise bool) (Position, error) {
	if curPos.X == endPos.X && curPos.Y == endPos.Y {
		return Position{}, errors.New("expected endpoint different than current with radius")
	}

	dist := hypot(curPos, endPos)
	delta := dist - math.Abs(radius)*2
	if delta > minimumDelta {
		return Position{}, errors.New("radius too small")
	} else if delta > 0.0 {
		dist = math.Abs(radius) * 2
	}

	theta := math.Atan2(endPos.Y-curPos.Y, endPos.X-curPos.X)
	if (clockwise && radius > 0.0) || (!clockwise && radius < 0.0) {
		theta -= (math.Pi / 2.0)
	} else {
		theta += (math.Pi / 2.0)
	}

	offset := math.Abs(radius) * math.Cos(math.Asin(dist/(math.Abs(radius)*2)))
	return Position{
		X: ((curPos.X + endPos.X) / 2) + offset*math.Cos(theta),
		Y: ((curPos.Y + endPos.Y) / 2) + offset*math.Sin(theta),
	}, nil
}

// arcTo expects the positions to be mapped to the XYZ plane, with Z being the axis of rotation
// and the arc drawn in the XY plane
func arcTo(curPos, endPos, centerPos Position, radius float64, turns uint, clockwise bool,
	linearTo func(pos Position) error) error {

	if radius != 0.0 {
		if centerPos.X != curPos.X || centerPos.Y != curPos.Y {
			return errors.New("both center point and radius specified for arc")
		}

		var err error
		centerPos, err = radiusCenter(curPos, endPos, radius, clockwise)
		if err != nil {
			return err
		}

		radius = math.Abs(radius)
	} else if centerPos.X != curPos.X || centerPos.Y != curPos.Y {
		radius = hypot(curPos, centerPos)
		// XXX: warn if hypot(endPos, centerPos) is significantly different than radius
	} else {
		return errors.New("expected center point or radius for arc")
	}

	normal := endPos.Z - curPos.Z
	if math.Abs(normal) < minimumDelta {
		normal = 0.0
		if turns > 2 {
			turns = 2
		}
	}

	x := curPos.X - centerPos.X
	y := curPos.Y - centerPos.Y
	angle := math.Atan2(y, x)
	if angle < 0.0 {
		angle += math.Pi * 2
	}
	endAngle := math.Atan2(endPos.Y-centerPos.Y, endPos.X-centerPos.X)
	if endAngle < 0.0 {
		endAngle += math.Pi * 2
	}

	var angleDir float64
	if clockwise {
		angleDir = -1.0
	} else {
		angleDir = 1.0
	}

	angleTotal := float64(turns-1) * math.Pi * 2
	if angle == endAngle {
		angleTotal += math.Pi * 2
	} else if angle < endAngle {
		if clockwise {
			angleTotal += math.Pi*2 - (endAngle - angle)
		} else {
			angleTotal += endAngle - angle
		}
	} else {
		if clockwise {
			angleTotal += angle - endAngle
		} else {
			angleTotal += math.Pi*2 - (angle - endAngle)
		}
	}

	travelTotal := math.Hypot(angleTotal*radius, math.Abs(normal))
	numSteps := math.Floor(travelTotal / 0.1)
	stepAngle := angleTotal / numSteps
	stepNormal := normal / numSteps

	for step := float64(1.0); step < numSteps; step += 1.0 {
		err := linearTo(
			Position{
				X: centerPos.X + radius*math.Cos(angle+step*stepAngle*angleDir),
				Y: centerPos.Y + radius*math.Sin(angle+step*stepAngle*angleDir),
				Z: curPos.Z + step*stepNormal,
			})
		if err != nil {
			return err
		}
	}

	return linearTo(endPos)
}

func (eng *engine) toArcPlane(pos Position) Position {
	switch eng.arcPlane {
	case XYPlane:
		return pos
	case ZXPlane:
		return Position{X: pos.Z, Y: pos.X, Z: pos.Y}
	case YZPlane:
		return Position{X: pos.Y, Y: pos.Z, Z: pos.X}
	default:
		panic(fmt.Sprintf("unexpected arcPlane: %d", eng.arcPlane))
	}
}

func (eng *engine) fromArcPlane(pos Position) Position {
	switch eng.arcPlane {
	case XYPlane:
		return pos
	case ZXPlane:
		return Position{X: pos.Y, Y: pos.Z, Z: pos.X}
	case YZPlane:
		return Position{X: pos.Z, Y: pos.X, Z: pos.Y}
	default:
		panic(fmt.Sprintf("unexpected arcPlane: %d", eng.arcPlane))
	}
}

func (eng *engine) arcTo(codes []Code, useMachine bool) ([]Code, error) {
	if useMachine {
		return nil, errors.New("G53 not allowed with arcs")
	}

	var err error
	var args []arg
	args, codes, err = parseArgs(codes, fArg|iArg|jArg|kArg|pArg|rArg|xArg|yArg|zArg)
	if err != nil {
		return nil, err
	}

	endPos := eng.curPos
	centerPos := eng.curPos
	var radius float64
	turns := uint(1)
	for _, arg := range args {
		switch arg.letter {
		case 'F':
			err = eng.setFeed(float64(arg.num) * eng.units)
			if err != nil {
				return nil, err
			}
		case 'I':
			if eng.arcPlane == YZPlane && !arg.num.Equal(0.0) {
				return nil, errors.New("unexpected I for arc in YZ plane")
			}
			centerPos.X = eng.toMachineX(float64(arg.num)*eng.units, eng.absoluteArcMode)
		case 'J':
			if eng.arcPlane == ZXPlane && !arg.num.Equal(0.0) {
				return nil, errors.New("unexpected J for arc in ZX plane")
			}
			centerPos.Y = eng.toMachineY(float64(arg.num)*eng.units, eng.absoluteArcMode)
		case 'K':
			if eng.arcPlane == XYPlane && !arg.num.Equal(0.0) {
				return nil, errors.New("unexpected K for arc in XY plane")
			}
			centerPos.Z = eng.toMachineZ(float64(arg.num)*eng.units, eng.absoluteArcMode)
		case 'P':
			num, ok := arg.num.AsInteger()
			if !ok || num < 1 {
				return nil, fmt.Errorf("expected a positive number of turns: P%s", arg.num)
			}
			turns = uint(num)
		case 'R':
			radius = float64(arg.num)
			if radius <= 0.0 {
				return nil, fmt.Errorf("expected a positive radius: R%s", arg.num)
			}
		case 'X':
			endPos.X = eng.toMachineX(float64(arg.num)*eng.units, eng.absoluteMode)
		case 'Y':
			endPos.Y = eng.toMachineY(float64(arg.num)*eng.units, eng.absoluteMode)
		case 'Z':
			endPos.Z = eng.toMachineZ(float64(arg.num)*eng.units, eng.absoluteMode)
		}
	}

	if eng.moveMode != clockwiseArcMove && eng.moveMode != counterClockwiseArcMove {
		panic(fmt.Sprintf("unexpected moveMode: %d", eng.moveMode))
	}

	err = arcTo(eng.toArcPlane(eng.curPos), eng.toArcPlane(endPos), eng.toArcPlane(centerPos),
		radius, turns, eng.moveMode == clockwiseArcMove,
		func(pos Position) error {
			return eng.linearTo(eng.fromArcPlane(pos))
		})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

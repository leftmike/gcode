package gcode

import (
	"fmt"
)

const (
	curCoordSysParam  = 5220
	coordSysParam     = 5221
	coordSysParamStep = 20
)

func (eng *engine) getCoordSysParam(num int) (Number, bool) {
	num -= coordSysParam
	coordSys := num / coordSysParamStep
	switch num % coordSysParamStep {
	case 0:
		return Number(eng.coordSysPos[coordSys].X), true
	case 1:
		return Number(eng.coordSysPos[coordSys].Y), true
	case 2:
		return Number(eng.coordSysPos[coordSys].Z), true
	}

	return 0, true
}

func (eng *engine) setCoordSysParam(num int, val Number) error {
	num -= coordSysParam
	coordSys := num / coordSysParamStep
	switch num % coordSysParamStep {
	case 0:
		eng.coordSysPos[coordSys].X = float64(val)
	case 1:
		eng.coordSysPos[coordSys].Y = float64(val)
	case 2:
		eng.coordSysPos[coordSys].Z = float64(val)
	}

	return nil
}

func (eng *engine) getNumParam(num int) (Number, bool) {
	switch {
	case num == curCoordSysParam:
		return Number(eng.curCoordSys + 1), true
	case num >= coordSysParam && num < coordSysParam*coordSysParamStep*9:
		return eng.getCoordSysParam(num)
	}

	val, ok := eng.numParams[num]
	if !ok {
		return 0, false
	}
	return val, true
}

func (eng *engine) setNumParam(num int, val Number) error {
	switch {
	case num == curCoordSysParam:
		n, ok := val.AsInteger()
		if !ok || n < 1 || n > 9 {
			return fmt.Errorf("#%d: expected an integer between 1 and 9: %s", num, val)
		}
		eng.curCoordSys = n - 1
		return nil
	case num >= coordSysParam && num < coordSysParam*coordSysParamStep*9:
		return eng.setCoordSysParam(num, val)
	}

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

package gcode

import (
	"fmt"
)

const (
	homePosXParam     = 5161
	homePosYParam     = 5162
	homePosZParam     = 5163
	secondPosXParam   = 5181
	secondPosYParam   = 5182
	secondPosZParam   = 5183
	workPosEnabled    = 5210
	workPosXParam     = 5211
	workPosYParam     = 5212
	workPosZParam     = 5213
	curCoordSysParam  = 5220
	coordSysParam     = 5221 // Nine sets of coordinate system parameters starting here.
	coordSysParamStep = 20   // Gap between each coordinate system's parameters.
)

func (eng *engine) getCoordSysParam(num int) (Number, bool) {
	num -= coordSysParam
	coordSys := num / coordSysParamStep
	switch num % coordSysParamStep {
	case 0:
		return Number(eng.coordSysPos[coordSys].X / eng.units), true
	case 1:
		return Number(eng.coordSysPos[coordSys].Y / eng.units), true
	case 2:
		return Number(eng.coordSysPos[coordSys].Z / eng.units), true
	}

	return 0, true
}

func (eng *engine) setCoordSysParam(num int, val Number) error {
	num -= coordSysParam
	coordSys := num / coordSysParamStep
	switch num % coordSysParamStep {
	case 0:
		eng.coordSysPos[coordSys].X = float64(val) * eng.units
	case 1:
		eng.coordSysPos[coordSys].Y = float64(val) * eng.units
	case 2:
		eng.coordSysPos[coordSys].Z = float64(val) * eng.units
	}

	return nil
}

func (eng *engine) getNumParam(num int) (Number, bool) {
	switch num {
	case homePosXParam:
		return Number(eng.homePos.X / eng.units), true
	case homePosYParam:
		return Number(eng.homePos.Y / eng.units), true
	case homePosZParam:
		return Number(eng.homePos.Z / eng.units), true
	case secondPosXParam:
		return Number(eng.secondPos.X / eng.units), true
	case secondPosYParam:
		return Number(eng.secondPos.Y / eng.units), true
	case secondPosZParam:
		return Number(eng.secondPos.Z / eng.units), true
	case workPosEnabled:
		if eng.useWorkPos {
			return 1, true
		}
		return 0, true
	case workPosXParam:
		return Number(eng.workPos.X / eng.units), true
	case workPosYParam:
		return Number(eng.workPos.Y / eng.units), true
	case workPosZParam:
		return Number(eng.workPos.Z / eng.units), true
	case curCoordSysParam:
		return Number(eng.curCoordSys + 1), true
	}

	if num >= coordSysParam && num < coordSysParam*coordSysParamStep*9 {
		return eng.getCoordSysParam(num)
	}

	val, ok := eng.numParams[num]
	if !ok {
		return 0, false
	}
	return val, true
}

func (eng *engine) setNumParam(num int, val Number) error {
	switch num {
	case homePosXParam:
		eng.homePos.X = float64(val) * eng.units
		return nil
	case homePosYParam:
		eng.homePos.Y = float64(val) * eng.units
		return nil
	case homePosZParam:
		eng.homePos.Z = float64(val) * eng.units
		return nil
	case secondPosXParam:
		eng.secondPos.X = float64(val) * eng.units
		return nil
	case secondPosYParam:
		eng.secondPos.Y = float64(val) * eng.units
		return nil
	case secondPosZParam:
		eng.secondPos.Z = float64(val) * eng.units
		return nil
	case workPosEnabled:
		if val.Equal(0.0) {
			eng.useWorkPos = false
		} else {
			eng.useWorkPos = true
		}
		return nil
	case workPosXParam:
		eng.workPos.X = float64(val) * eng.units
		return nil
	case workPosYParam:
		eng.workPos.Y = float64(val) * eng.units
		return nil
	case workPosZParam:
		eng.workPos.Z = float64(val) * eng.units
		return nil
	case curCoordSysParam:
		n, ok := val.AsInteger()
		if !ok || n < 1 || n > 9 {
			return fmt.Errorf("#%d: expected an integer between 1 and 9: %s", num, val)
		}
		eng.curCoordSys = n - 1
		return nil
	}

	if num >= coordSysParam && num < coordSysParam*coordSysParamStep*9 {
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

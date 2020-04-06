package engine_test

import (
	"testing"

	engine "github.com/leftmike/gcode/engine"
	parser "github.com/leftmike/gcode/parser"
)

type machine struct{}

func (m machine) RapidTo(pos engine.Position) error {
	return nil
}

func (m machine) LinearTo(pos engine.Position, feed float64) error {
	return nil
}

func TestEvaluate(t *testing.T) {
	eng := engine.NewEngine(machine{}, parser.BeagleG)
	_ = eng
}

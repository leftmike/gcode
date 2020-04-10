package main

/*
To Do:
- RepRap:
-- parser: support {} instead of [] for expressions
-- _ prefix for global parameter names

- LinuxCNC:
-- #1 to #30 are subroutine parameters and are local to the subroutine
-- #<name> are local to the scope where it is assigned; scoped to subroutines
-- #31 and above, and #<_name> are global
-- O codes

- parser: change dialect to feature flags
- G10 L2: support R for rotation
- predefined parameters
- Machine.UnknownGCode()
- Machine.UnknownMCode()
- Engine.SetCurrentPosition()
*/

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/leftmike/gcode"
)

type machine struct{}

func (m machine) SetFeed(feed float64) error {
	return nil
}

func (m machine) RapidTo(pos gcode.Position) error {
	return nil
}

func (m machine) LinearTo(pos gcode.Position) error {
	return nil
}

func main() {
	if len(os.Args) <= 1 {
		eng := gcode.NewEngine(machine{}, gcode.BeagleG)
		err := eng.Evaluate(bufio.NewReader(os.Stdin))
		if err != nil {
			log.Print(err)
		}
	} else {
		for adx := 1; adx < len(os.Args); adx += 1 {
			f, err := os.Open(os.Args[adx])
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()

			fmt.Println(os.Args[adx])
			eng := gcode.NewEngine(machine{}, gcode.BeagleG)
			err = eng.Evaluate(bufio.NewReader(f))
			if err != nil {
				log.Print(err)
			}
			fmt.Println()
		}
	}
}

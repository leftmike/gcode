package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/leftmike/gcode"
)

// startBrowser tries to open the URL in a browser
// and reports whether it succeeds.
// XXX: copied from https://github.com/golang/tools/cmd/cover/html.go
func startBrowser(url string) bool {
	// try to start the browser
	var args []string
	switch runtime.GOOS {
	case "darwin":
		args = []string{"open"}
	case "windows":
		args = []string{"cmd", "/c", "start"}
	default:
		args = []string{"xdg-open"}
	}
	cmd := exec.Command(args[0], append(args[1:], url)...)
	return cmd.Start() == nil
}

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

func (m machine) HandleGCode(code gcode.Code, codes []gcode.Code,
	setCurPos func(pos gcode.Position) error) ([]gcode.Code, error) {

	return nil, fmt.Errorf("unexpected code: %s: %v", code, codes)
}

func (m machine) HandleMCode(code gcode.Code, codes []gcode.Code,
	setCurPos func(pos gcode.Position) error) ([]gcode.Code, error) {

	return nil, fmt.Errorf("unexpected code: %s: %v", code, codes)
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

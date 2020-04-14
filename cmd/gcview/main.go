package main

//go:generate sh -c "awk -f index.awk index.html > index.go"

/*
To Do:
- zoom in and out
- adjust workspace and default zoom based on min & max
- console.log sizes
- console.log rotates and zooms
- derive work piece size based on non-rapid moves
*/

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/leftmike/gcode"
)

var (
	beagleGFeature  = flag.Bool("beagleg", false, "enable BeagleG dialect")
	linuxCNCFeature = flag.Bool("linuxcnc", false, "enable LinuxCNC dialect")
	repRapFeature   = flag.Bool("reprap", false, "enable RepRap dialect")
)

func startBrowser(url string) {
	// copied from https://github.com/golang/tools/cmd/cover/html.go

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
	cmd.Start()
}

type machine struct {
	w       strings.Builder
	base    string
	homePos gcode.Position
	maxPos  gcode.Position
}

func (m *machine) SetFeed(feed float64) error {
	return nil
}

func (m *machine) updateRange(pos gcode.Position) {
	if pos.X < m.homePos.X {
		m.homePos.X = pos.X
	}
	if pos.X > m.maxPos.X {
		m.maxPos.X = pos.X
	}
	if pos.Y < m.homePos.Y {
		m.homePos.Y = pos.Y
	}
	if pos.Y > m.maxPos.Y {
		m.maxPos.Y = pos.Y
	}
	if pos.Z < m.homePos.Z {
		m.homePos.Z = pos.Z
	}
	if pos.Z > m.maxPos.Z {
		m.maxPos.Z = pos.Z
	}
}

func floatString(f float64) string {
	return strconv.FormatFloat(f, 'f', 6, 64)
}

func posString(pos gcode.Position) string {
	return fmt.Sprintf("{x: %s, y: %s, z: %s}",
		floatString(pos.X), floatString(pos.Y), floatString(pos.Z))
}

func (m *machine) RapidTo(pos gcode.Position) error {
	m.updateRange(pos)
	fmt.Fprintf(&m.w, "  {rapidTo: %s},\n", posString(pos))
	return nil
}

func (m *machine) LinearTo(pos gcode.Position) error {
	m.updateRange(pos)
	fmt.Fprintf(&m.w, "  {linearTo: %s},\n", posString(pos))
	return nil
}

func (m *machine) HandleUnknown(code gcode.Code, codes []gcode.Code,
	setCurPos func(pos gcode.Position) error) ([]gcode.Code, error) {

	fmt.Fprintf(os.Stderr, "%s: unknown: %v\n", m.base, append([]gcode.Code{code}, codes...))
	return nil, nil
}

const (
	configFormat = `
  homePos: %s,
  maxPos: %s,
`
)

func (m *machine) config() string {
	return fmt.Sprintf(configFormat, posString(m.homePos), posString(m.maxPos))
}

func (m *machine) htmlOutput(base string) (string, error) {
	w, err := os.Create(filepath.Join(os.TempDir(), base+".html"))
	if err != nil {
		return "", err
	}
	defer w.Close()

	fmt.Fprintf(w, indexHTML, filepath.Base(w.Name()), m.config(), m.w.String())
	return w.Name(), nil
}

func main() {
	flag.Parse()
	args := flag.Args()
	var features gcode.Features
	if *beagleGFeature {
		features |= gcode.BeagleG
	}
	if *linuxCNCFeature {
		features |= gcode.LinuxCNC
	}
	if *repRapFeature {
		features |= gcode.RepRap
	}
	if features == 0 {
		features = gcode.AllFeatures
	}

	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "gcview: no gcode file(s) specified")
		os.Exit(1)
	}

	for adx, arg := range args {
		f, err := os.Open(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gcview: %s\n", err)
			continue
		}
		defer f.Close()

		base := filepath.Base(arg)
		m := machine{
			base: base,
		}
		eng := gcode.NewEngine(&m, features)
		err = eng.Evaluate(bufio.NewReader(f))
		if err != nil {
			fmt.Fprintf(os.Stderr, "gcview: %s: %s\n", base, err)
			continue
		}

		out, err := m.htmlOutput(base)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gcview: %s\n", err)
			continue
		}

		fmt.Printf("%s -> %s\n", base, out)
		fmt.Printf("%s -> %s\n", posString(m.homePos), posString(m.maxPos))

		if adx < 4 {
			startBrowser("file://" + out)
		}
	}
}

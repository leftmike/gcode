package main

//go:generate sh -c "awk -f index.awk index.html > index.go"

/*
To Do:
- console.log the size and rendering time of the gcode
- for gcode over a certain size, zoom and rotate the workspace, and rendering the drawing at the end
*/

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func (m *machine) SetSpindle(speed float64, clockwise bool) error {
	return nil
}

func (m *machine) SpindleOff() error {
	return nil
}

func (m *machine) SelectTool(tool uint) error {
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
	if pos.Z > m.homePos.Z {
		m.homePos.Z = pos.Z
	}
	if pos.Z < m.maxPos.Z {
		m.maxPos.Z = pos.Z
	}
}

func (m *machine) RapidTo(pos gcode.Position) error {
	fmt.Fprintf(&m.w, "  {rapidTo: %s},\n", pos)
	return nil
}

func (m *machine) LinearTo(pos gcode.Position) error {
	m.updateRange(pos)
	fmt.Fprintf(&m.w, "  {linearTo: %s},\n", pos)
	return nil
}

func (m *machine) HandleUnknown(code gcode.Code, codes []gcode.Code,
	setCurPos func(pos gcode.Position) error) ([]gcode.Code, error) {

	fmt.Fprintf(os.Stderr, "%s: unknown: %v\n", m.base, append([]gcode.Code{code}, codes...))
	return nil, nil
}

func (m *machine) config() string {
	xside := m.maxPos.X - m.homePos.X
	yside := m.maxPos.Y - m.homePos.Y
	if xside > yside {
		yside = xside
	} else {
		xside = yside
	}
	if xside < 12 {
		xside = 12
		yside = 12
	}

	zside := m.homePos.Z - m.maxPos.Z
	if zside < 4 {
		zside = 4
	}

	maxPos := gcode.Position{
		X: m.homePos.Z + xside,
		Y: m.homePos.Y + yside,
		Z: m.homePos.Z - zside,
	}
	return fmt.Sprintf(`
  homePos: %s,
  maxPos: %s,
`, m.homePos, maxPos)
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
		fmt.Printf("%s -> %s\n", m.homePos, m.maxPos)

		if adx < 4 {
			startBrowser("file://" + out)
		}
	}
}

package main

/*
To Do:
- zoom in and out
- adjust workspace and default zoom based on min & max
- console.log sizes
- console.log rotates and zooms
*/

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/leftmike/gcode"
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

	fmt.Fprintf(w, indexHTML, w.Name(), m.config(), m.w.String())
	return w.Name(), nil
}

func main() {
	if len(os.Args) <= 1 {
		fmt.Fprintln(os.Stderr, "gcview: no gcode file(s) specified")
		os.Exit(1)
	}

	for adx := 1; adx < len(os.Args); adx += 1 {
		f, err := os.Open(os.Args[adx])
		if err != nil {
			fmt.Fprintf(os.Stderr, "gcview: %s\n", err)
			continue
		}
		defer f.Close()

		base := filepath.Base(os.Args[adx])
		m := machine{
			base: base,
		}
		eng := gcode.NewEngine(&m, gcode.BeagleG)
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

const indexHTML = `
<!DOCTYPE html>
<html>
  <head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
    <title>%s</title>
    <style type="text/css">
      canvas { border: 1px solid black; }
    </style>
    <script src="https://unpkg.com/zdog@1/dist/zdog.dist.js"></script>
  </head>
  <body>
    <canvas class="gcode-view" width="600" height="600"></canvas>
    <script type="text/javascript">
const config = {
%s
}

const cmds = [
%s
]
    </script>
    <script type="text/javascript">
let displaySize = 600;

console.log("homePos: ", config.homePos)
console.log("maxPos: ", config.maxPos)

let gcodeView = document.querySelector(".gcode-view")

let illo = new Zdog.Illustration({
  element: gcodeView,
  scale: {x: 1.0, y: -1.0, z: 1.0},
  rotate: {x: 1.1, y: 0, z: -0.3},
  zoom: 30,
});

gcodeView.onwheel = function(event) {
  illo.zoom += (event.deltaY * 0.1)
  if (illo.zoom < 1.0) {
    illo.zoom = 1.0
  }
  console.log("zoom: ", illo.zoom)
  animate()
}

let dragStartRX, dragStartRZ;
let isDragging = false;

new Zdog.Dragger({
  startElement: gcodeView,
  onDragStart: function() {
    dragStartRX = illo.rotate.x;
    dragStartRZ = illo.rotate.z;
    isDragging = true;
    animate();
  },
  onDragMove: function( pointer, moveX, moveY ) {
    illo.rotate.x = dragStartRX - ( moveY / displaySize * Zdog.TAU );
    illo.rotate.z = dragStartRZ - ( moveX / displaySize * Zdog.TAU );
  },
  onDragEnd: function () {
    isDragging = false;
  },
});

// Workspace
let workspace = new Zdog.Anchor({
  addTo: illo,
  translate: {x: -6, y: -6, z: 2},
})

new Zdog.Shape({
  addTo: workspace,
  stroke: 0.01,
  color: 'grey',
  path: [
    {x: 0, y: 0, z: 0},
    {x: 12, y: 0, z: 0},
    {x: 12, y: 12, z: 0},
    {x: 0, y: 12, z: 0},
    {x: 0, y: 0, z: 0},

    {move: {x: 0, y: 0, z: -4}},
    {x: 12, y: 0, z: -4},
    {x: 12, y: 12, z: -4},
    {x: 0, y: 12, z: -4},
    {x: 0, y: 0, z: -4},

    {move: {x: 0, y: 0, z: 0}},
    {x: 0, y: 0, z: -4},

    {move: {x: 12, y: 0, z: 0}},
    {x: 12, y: 0, z: -4},

    {move: {x: 12, y: 12, z: 0}},
    {x: 12, y: 12, z: -4},

    {move: {x: 0, y: 12, z: 0}},
    {x: 0, y: 12, z: -4},
  ],
})

// Axes
new Zdog.Shape({
  addTo: workspace,
  stroke: 0.1,
  color: 'red',
  path: [
    {x: -1, y: 0, z: 0},
    {x: 1, y: 0, z: 0},
  ],
})

new Zdog.Shape({
  addTo: workspace,
  stroke: 0.1,
  color: 'green',
  path: [
    {x: 0, y: -1, z: 0},
    {x: 0, y: 1, z: 0},
  ],
})

new Zdog.Shape({
  addTo: workspace,
  stroke: 0.1,
  color: 'blue',
  path: [
    {x: 0, y: 0, z: -1},
    {x: 0, y: 0, z: 1},
  ],
})

let curPt = {x: 0, y: 0, z: 0}

function rapidTo(pt) {
  new Zdog.Shape({
    addTo: workspace,
    stroke: 0.02,
    color: 'red',
    path: [curPt, pt],
  })
  curPt = pt
}

function linearTo(pt) {
  new Zdog.Shape({
    addTo: workspace,
    stroke: 0.02,
    color: 'green',
    path: [curPt, pt],
  })
  curPt = pt
}

for (cmd of cmds) {
  if (cmd.rapidTo !== undefined) {
    rapidTo(cmd.rapidTo)
  } else if (cmd.linearTo !== undefined) {
    linearTo(cmd.linearTo)
  }
}

function animate() {
  illo.updateRenderGraph()
  if (isDragging) {
    requestAnimationFrame(animate)
  }
}
animate();
    </script>
 </body>
</html>
`

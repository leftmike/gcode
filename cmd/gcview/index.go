package main

const indexHTML = `
<!DOCTYPE html>
<html>
  <head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
    <style type="text/css">
      canvas { border: 1px solid black; }
    </style>
    <script src="https://unpkg.com/zdog@1/dist/zdog.dist.js"></script>
  </head>
  <body>
    <canvas class="gcode-view" width="600" height="600"></canvas>
    <script type="text/javascript">
document.title = "%s"

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

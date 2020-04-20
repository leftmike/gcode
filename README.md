## gcode
G-code parser and engine
* Library for parsing and executing G-code
* Tool to display G-code toolpath in the browser

## G-code
The goal is to support multiple dialects of G-code.
* [LinuxCNC](http://linuxcnc.org/docs/html/)
* [BeagleG](https://github.com/hzeller/beagleg/blob/master/G-code.md)
* [RepRap](https://reprap.org/wiki/G-code)
* [Smoothie](http://smoothieware.org/supported-g-codes)

| Code | Arguments | Description |
|--------|-----------|-------------|
| G0 | F*n.n* X*n.n* Y*n.n* Z*n.n* | rapid move |
| G1 | F*n.n* X*n.n* Y*n.n* Z*n.n* | linear move |
| G2 | F*n.n* X*n.n* Y*n.n* Z*n.n* I*n.n* J*n.n* K*n.n* | clockwise arc move with center |
| G2 | F*n.n* X*n.n* Y*n.n* Z*n.n* R*n.n* | clockwise arc move with radius |
| G3 | F*n.n* X*n.n* Y*n.n* Z*n.n* I*n.n* J*n.n* K*n.n* | counter-clockwise arc move with center |
| G3 | F*n.n* X*n.n* Y*n.n* Z*n.n* R*n.n* | counter-clockwise arc move with radius |
| G10 | L2 P*n* X*n.n* Y*n.n* Z*n.n* | set coordinate system using absolute machine coordinates |
| G10 | L20 P*n* X*n.n* Y*n.n* Z*n.n* | set coordinate system using relative machine coordinates |
| G17 | | XY plane selection (default) |
| G18 | | ZX plane selection |
| G19 | | YZ plane selection |
| G20 | | coordinates in inches |
| G21 | | coordinates in mm (default) |
| G28 | X*n.n* Y*n.n* Z*n.n* | go home |
| G28.1 | | set home |
| G30 | X*n.n* Y*n.n* Z*n.n* | go predefined position |
| G30.1 | | set predefined position |
| G53 | G0 F*n.n* X*n.n* Y*n.n* Z*n.n* | rapid move using machine coordinates |
| G53 | G1 F*n.n* X*n.n* Y*n.n* Z*n.n* | linear move using machine coordinates |
| G54 | | use coordinate system one (default) |
| G55 | | use coordinate system two |
| G56 | | use coordinate system three |
| G57 | | use coordinate system four |
| G58 | | use coordinate system five |
| G59 | | use coordinate system six |
| G59.1 | | use coordinate system seven |
| G59.2 | | use coordinate system eight |
| G59.3 | | use coordinate system nine |
| G90 | | absolute distance mode for X, Y, and, Z (default) |
| G90.1 | | absolute arc mode for I, J, and K |
| G91 | | relative distance mode for X, Y, and, Z |
| G91.1 | | relative arc mode for I, J, and K (default) |
| G92 | X*n.n* Y*n.n* Z*n.n* | set work position |
| G92.1 | | zero work position |
| G92.2 | | save work position, then zero |
| G92.3 | | restore saved work position |
| M3 | | spindle on clockwise |
| M4 | | spindle on counter-clockwise |
| M5 | | spindle off |
| S*n.n* | | spindle speed |
| T*n* | | select tool |

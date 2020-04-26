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
| G1 | F*n.n* X*n.n* Y*n.n* Z*n.n* | linear move (default) |
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
| M2 | | end program |
| M3 | | spindle on clockwise |
| M4 | | spindle on counter-clockwise |
| M5 | | spindle off |
| M30 | | end program |
| S*n.n* | | spindle speed |
| T*n* | | select tool |

## Parameters

| Parameter | Default | Persistent | Description |
|-----------|---------|------------|-------------|
| 5161, 5162, 5163 | 0, 0, 0 | yes | X, Y, Z for home position (G28) |
| 5181, 5182, 5183 | 0, 0, 0 | yes | X, Y, Z for predefined position (G30) |
| 5210 | 0 | yes | flag to control works offsets; 0 means off |
| 5211, 5212, 5213 | 0, 0, 0 | yes | X, Y, Z for work offsets (G92) |
| 5220 | 1 | yes | current coordinate system (G54 to G59.3) |
| 5221, 5222, 5223 | 0, 0, 0 | yes | X, Y, Z for coordinate system 1 offsets (G54) |
| 5241, 5242, 5243 | 0, 0, 0 | yes | X, Y, Z for coordinate system 2 offsets (G55) |
| 5261, 5262, 5263 | 0, 0, 0 | yes | X, Y, Z for coordinate system 3 offsets (G56) |
| 5281, 5282, 5283 | 0, 0, 0 | yes | X, Y, Z for coordinate system 4 offsets (G57) |
| 5301, 5302, 5303 | 0, 0, 0 | yes | X, Y, Z for coordinate system 5 offsets (G58) |
| 5321, 5322, 5323 | 0, 0, 0 | yes | X, Y, Z for coordinate system 6 offsets (G59) |
| 5341, 5342, 5343 | 0, 0, 0 | yes | X, Y, Z for coordinate system 7 offsets (G59.1) |
| 5361, 5362, 5363 | 0, 0, 0 | yes | X, Y, Z for coordinate system 8 offsets (G59.2) |
| 5381, 5382, 5383 | 0, 0, 0 | yes | X, Y, Z for coordinate system 9 offsets (G59.3) |
| 5599 | 1 | no | flag to control output of `(debug,...)` comments; 0 means off |

## Syntax

### Expression Syntax

```
<parameter> =
      '#' <integer>
    | '#' <name>
<expr> =
      <reference>
    | '[' <sub-expr> ']'
    | <number>
    | <name>
    | <string>
<sub-expr> =
      <number>
    | '-' <sub-expr>
    | '!' <sub-expr>
    | '[' <sub-expr> ']'
    | <sub-expr> <op> <sub-expr>
    | <reference>
    | <name>
    | <string>
    | <func> '[' [<sub-expr> [',' ...]] ']'
<op> = '+' '-' '*' '/'
    | '==' '!=' '<' '<=' '>' '>='
    | '&&' '||'
<reference> = '#'* <parameter>
<name> = '<' <name-char>+ '>'
<initial-name-char> = 'A' ... 'Z' | 'a' ... 'z' | '_'
<name-char> = <initial-name-char> | '0' ... '9'
```

### LinuxCNC Specific Syntax

Comments which begin with `msg,` or `debug,` are written to standard output. For example,
`(msg,hello world!)` will write `hello world!`.

Comments which begin with `print,` are written to standard error.

In addition, in `debug,` and `print,` comments, the values of parameters in the body of the
comment will be expanded to their values. For example, the following code will write
`value of parameter 123: 456` to standard error.

```
#123=456
(print,value of parameter 123: #123)
```

### BeagleG Specific Syntax

* IF *expression* THEN *assignment*
* IF *expression* THEN *assignment* ELSE *assignment*
* IF *expression* THEN *assignment* ELSEIF *assignment* ...
* IF *expression* THEN *assignment* ELSEIF *assignment* ... ELSE *assignment*
* WHILE *expression* DO<br>
*line* ...<br>
END

Alphanumeric parameters may be specified with and without bracketing `<` and `>` (eg.
`#param` or `#<param>`).

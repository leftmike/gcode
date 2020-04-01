package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"

	parser "github.com/leftmike/gcode/parser"
)

func main() {
	for adx := 1; adx < len(os.Args); adx += 1 {
		f, err := os.Open(os.Args[adx])
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		numParams := map[int]parser.Number{}
		p := parser.Parser{
			Scanner: bufio.NewReader(f),
			Dialect: parser.BeagleG,
			GetNumParam: func(num int) (parser.Number, error) {
				return numParams[num], nil
			},
			SetNumParam: func(num int, val parser.Number) error {
				numParams[num] = val
				return nil
			},
		}

		fmt.Print(os.Args[adx])
		for {
			code, val, err := p.Parse()
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatal(err)
			}

			if code == 'G' || code == 'M' {
				fmt.Println()
			}
			fmt.Printf("%c%d ", code, int(val.(parser.Number)))
		}
		fmt.Println()
	}
}

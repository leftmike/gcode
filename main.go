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

		fmt.Println(os.Args[adx])
		for {
			codes, err := p.Parse()
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatal(err)
			}

			for _, code := range codes {
				fmt.Printf("%c%d ", code.Letter, int(code.Value.(parser.Number)))
			}
			fmt.Println()
		}
		fmt.Println()
	}
}

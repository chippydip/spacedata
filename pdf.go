package main

import (
	"fmt"
	"math"
	"os"
	"strings"

	"rsc.io/pdf"
)

func readPdf() {
	reader, err := pdf.Open("16_vi4b_422.pdf")
	check(err)

	f, err := os.Create("tmp.txt")
	check(err)

	for i := 2; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		content := page.Content()
		var lines []string
		var line string
		x, y := 0.0, 10000.0
		for _, t := range content.Text {
			if t.Y != y {
				if math.Abs(t.Y-y) > 5 {
					_, err := f.WriteString(fmt.Sprintf("%v\n", line))
					check(err)
					// new line
					lines = append(lines, line)
					line = ""
					x = t.X
				} else if t.Y > y {
					line += "^"
				} else {
					line += "_"
				}
				y = t.Y
			}
			if t.X > x+1 {
				// add space
				if t.X > x+5 {
					if t.Font == "CMR10" || t.Font == "CMMI10" || t.Font == "CMSY10" {
						line += ", "
					} else {
						//fmt.Println(t.Font)
						line += strings.Repeat(" ", int((t.X-x)/5))
					}
				} else {
					line += " "
				}
				x = t.X
			}
			x += t.W
			line += t.S
		}
		_, err := f.WriteString(fmt.Sprintf("%v\n", line))
		check(err)
		lines = append(lines, line)
	}
}

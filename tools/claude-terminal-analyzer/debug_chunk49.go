//go:build ignore
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hinshun/vt10x"
)

type TerminalDebugInfo struct {
	Item struct {
		Rows             int      `json:"rows"`
		Cols             int      `json:"cols"`
		ScrollbackChunks []string `json:"scrollbackChunks"`
	} `json:"item"`
}

func main() {
	file, _ := os.Open("cc第一次检测失败.json")
	defer file.Close()
	var data TerminalDebugInfo
	json.NewDecoder(file).Decode(&data)

	rows, cols := data.Item.Rows, data.Item.Cols
	term := vt10x.New(vt10x.WithSize(cols, rows))

	// Process chunks 0-49
	for i := 0; i <= 49; i++ {
		term.Write([]byte(data.Item.ScrollbackChunks[i]))
	}

	// Print visible lines
	fmt.Println("=== Screen at Chunk #49 ===")
	for i := 0; i < rows; i++ {
		line := ""
		for j := 0; j < cols; j++ {
			cell := term.Cell(j, i)
			if cell.Char != 0 {
				line += string(cell.Char)
			} else {
				line += " "
			}
		}
		trimmed := strings.TrimRight(line, " ")
		if trimmed != "" {
			fmt.Printf("%2d: %s\n", i, trimmed)
		}
	}
}

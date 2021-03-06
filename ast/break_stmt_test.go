package ast

import (
	"testing"
)

func TestBreakStmt(t *testing.T) {
	nodes := map[string]Node{
		`0x7fca2d8070e0 <col:11, col:23>`: &BreakStmt{
			Address:  "0x7fca2d8070e0",
			Position: "col:11, col:23",
			Children: []Node{},
		},
	}

	runNodeTests(t, nodes)
}

package ast

import (
	"testing"
)

func TestIfStmt(t *testing.T) {
	nodes := map[string]Node{
		`0x7fc0a69091d0 <line:11:7, line:18:7>`: &IfStmt{
			Address:  "0x7fc0a69091d0",
			Position: "line:11:7, line:18:7",
			Children: []Node{},
		},
	}

	runNodeTests(t, nodes)
}

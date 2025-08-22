// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package tree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Node struct {
	Value string
	Child []Node
}

func Print(node Node) {
	fmt.Println(node.Value)
	print(node.Child, 0, "")

}

func PrintDir(dir string) {
	node := Node{}
	dirEntryToNode(dir, &node)
	Print(node)
}

func print(nodes []Node, level int, prefix string) {
	for i := 0; i < len(nodes); i++ {
		node := nodes[i]
		lastNode := i == len(nodes)-1

		nodePrefix := "├──"
		childPrefix := prefix + "│" + strings.Repeat(" ", 3)
		if lastNode {
			childPrefix = prefix + strings.Repeat(" ", 4)
			nodePrefix = "└──"
		}

		fmt.Printf("%s%s %s\n", prefix, nodePrefix, node.Value)
		print(node.Child, level+1, childPrefix)
	}
}

func dirEntryToNode(dir string, tn *Node) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		node := Node{}
		node.Value = e.Name()
		if e.IsDir() {
			dirEntryToNode(filepath.Join(dir, e.Name()), &node)
		}
		tn.Child = append(tn.Child, node)
	}
}

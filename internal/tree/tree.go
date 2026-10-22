// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package tree

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Node struct {
	Value string
	Child []Node
}

func Print(node Node) {
	_ = Fprint(os.Stdout, node)
}

// Fprint renders a tree to w.
func Fprint(w io.Writer, node Node) error {
	if _, err := fmt.Fprintln(w, node.Value); err != nil {
		return err
	}
	return print(w, node.Child, "")
}

func PrintDir(dir string) {
	node := Node{}
	dirEntryToNode(dir, &node)
	Print(node)
}

func print(w io.Writer, nodes []Node, prefix string) error {
	for i := 0; i < len(nodes); i++ {
		node := nodes[i]
		lastNode := i == len(nodes)-1

		nodePrefix := "├──"
		childPrefix := prefix + "│" + strings.Repeat(" ", 3)
		if lastNode {
			childPrefix = prefix + strings.Repeat(" ", 4)
			nodePrefix = "└──"
		}

		if _, err := fmt.Fprintf(w, "%s%s %s\n", prefix, nodePrefix, node.Value); err != nil {
			return err
		}
		if err := print(w, node.Child, childPrefix); err != nil {
			return err
		}
	}
	return nil
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

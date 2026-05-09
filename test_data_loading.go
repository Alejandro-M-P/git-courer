package main

import (
	"fmt"
	"github.com/Alejandro-M-P/git-courer/internal/data"
)

func main() {
	// Test data loading directly
	goNodes, ok := data.GetLanguageNodes("Go")
	if !ok {
		fmt.Println("Go language not found")
		return
	}
	
	fmt.Printf("Go nodes: %+v\n", goNodes)
	fmt.Printf("Go test patterns: %+v\n", goNodes.TestPatterns)
	fmt.Printf("Number of test patterns: %d\n", len(goNodes.TestPatterns))
	
	// Test GetAllLanguageNames
	names := data.GetAllLanguageNames()
	fmt.Printf("Total languages: %d\n", len(names))
	
	countWithPatterns := 0
	for _, name := range names {
		nodes, ok := data.GetLanguageNodes(name)
		if ok && len(nodes.TestPatterns) > 0 {
			countWithPatterns++
		}
	}
	fmt.Printf("Languages with test patterns: %d\n", countWithPatterns)
}
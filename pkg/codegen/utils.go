package main

import "fmt"

const generatedComment = "// This code was generated via go generate, do not modify directly"

func getHeader(pkg string) string {
	return fmt.Sprintf("%s\npackage %s\n\n", generatedComment, pkg)
}

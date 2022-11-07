package main

import (
	"fmt"
	kubedump "kubedump/pkg/cmd"
	"os"
)

func main() {
	app := kubedump.NewKubedumpApp()

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%s", err)
		os.Exit(1)
	}
}

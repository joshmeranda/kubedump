package main

import (
	kubedumpcmd "kubedump/pkg/cmd"
	"os"
)

func main() {
	app := kubedumpcmd.NewKubedumpServerApp()

	if err := app.Run(os.Args); err != nil {
		kubedumpcmd.CmdLogger.Errorf("%s", err)
		os.Exit(1)
	}
}

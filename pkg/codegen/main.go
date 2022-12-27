package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
	"os/exec"
	"path"
	"runtime"
)

var (
	_, thisFilePath, _, _ = runtime.Caller(0)
	thisDirPath           = path.Dir(thisFilePath)
	projectRoot           = path.Join(thisDirPath, "..", "..")

	filterSrcPath  = path.Join(projectRoot, "pkg", "filter")
	codegenSrcPath = path.Join(projectRoot, "pkg", "codegen")

	parserYaccPath = path.Join(codegenSrcPath, "parser.y")

	parserGoPath = path.Join(filterSrcPath, "yyparser.go")
)

func parserGen(_ *cli.Context) error {

	cmd := exec.Command("goyacc", "-o", parserGoPath, parserYaccPath)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Could not generate filter parser: %w", err)
	}

	return nil
}

func main() {
	app := cli.App{
		Name:  "codegen",
		Usage: "code generation for kubedump",
		Commands: cli.Commands{
			{
				Name:        "parser",
				Description: "generate the filter parser",
				Action:      parserGen,
				Flags:       cli.FlagsByName{},
			},
		},
		Authors: []*cli.Author{
			{
				Name:  "Josh Meranda",
				Email: "joshmeranda@gmail.com",
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}
}

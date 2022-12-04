package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"text/template"
)

var (
	_, thisFilePath, _, _ = runtime.Caller(0)
	thisDirPath           = path.Dir(thisFilePath)
	projectRoot           = path.Join(thisDirPath, "..", "..")

	controllerSrcPath = path.Join(projectRoot, "pkg", "controller")
	filterSrcPath     = path.Join(projectRoot, "pkg", "filter")
	codegenSrcPath    = path.Join(projectRoot, "pkg", "codegen")

	handlerTemplatePath = path.Join(codegenSrcPath, "handler.tpl")
	parserYaccPath      = path.Join(codegenSrcPath, "parser.y")

	parserGoPath = path.Join(filterSrcPath, "yyparser.go")
)

type Options struct {
	TypePath          string
	TypeName          string
	ConditionTypePath string
	AdditionalImports string
}

func getConditionType(typePath string, ctx *cli.Context) string {
	if ctx.IsSet("condition-type") {
		return ctx.String("condition-type")
	}

	return typePath + "Condition"
}

func getTypeName(typePath string) string {
	if split := strings.SplitN(typePath, ".", 2); len(split) == 1 {
		return typePath
	} else {
		return split[1]
	}
}

func handlerGen(ctx *cli.Context) error {
	typePath := ctx.String("type-path")

	opts := Options{
		TypePath:          typePath,
		TypeName:          getTypeName(typePath),
		ConditionTypePath: getConditionType(typePath, ctx),
		AdditionalImports: strings.Join(ctx.StringSlice("additional-imports"), "\n\t"),
	}

	tpl, err := template.ParseFiles(handlerTemplatePath)
	if err != nil {
		return fmt.Errorf("could not parse template at '%s': %w", handlerTemplatePath, err)
	}

	handlerPath := path.Join(controllerSrcPath, fmt.Sprintf("%s.go", strings.ToLower(opts.TypeName)))
	builder := strings.Builder{}

	if err := tpl.Execute(&builder, opts); err != nil {
		return fmt.Errorf("could not execute template: %w", err)
	}

	if err := os.WriteFile(handlerPath, []byte(builder.String()), 0644); err != nil {
		return fmt.Errorf("error writing to file '%s': %w", handlerPath, err)
	}

	return nil
}

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
				Name:        "handler",
				Description: "generate the the handle for a specified type",
				Action:      handlerGen,
				Flags: cli.FlagsByName{
					&cli.StringFlag{
						Name:     "type-path",
						Usage:    "the path to the type for the handler",
						Required: true,
						Aliases:  []string{"p"},
					},
					&cli.StringFlag{
						Name:    "condition-type",
						Usage:   "use the given type for status checking (defaults to the type path + Condition)",
						Aliases: []string{"c"},
					},
					&cli.StringSliceFlag{
						Name:    "additional-imports",
						Aliases: []string{"i"},
					},
				},
			},
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

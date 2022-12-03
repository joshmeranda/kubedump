package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
	"path"
	"strings"
	"text/template"
)

const (
	handlerTemplatePath = "../codegen/handler.tpl"
	handlerDirPath      = "../controller"
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

func codegen(ctx *cli.Context) error {
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

	handlerPath := path.Join(handlerDirPath, fmt.Sprintf("%s.go", strings.ToLower(opts.TypeName)))
	builder := strings.Builder{}

	if err := tpl.Execute(&builder, opts); err != nil {
		return fmt.Errorf("could not execute template: %w", err)
	}

	if err := os.WriteFile(handlerPath, []byte(builder.String()), 0644); err != nil {
		return fmt.Errorf("error writing to file '%s': %w", handlerPath, err)
	}

	return nil
}

func main() {
	app := cli.App{
		Name:      "codegen",
		Usage:     "code generation for kubedump",
		UsageText: "codegen <typePath> <importPath>",
		Action:    codegen,
		Flags: []cli.Flag{
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
		Authors: []*cli.Author{
			{
				Name:  "Josh Meranda",
				Email: "joshmeranda@gmail.com",
			},
		},
		CustomAppHelpTemplate:  "",
		UseShortOptionHandling: true,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
	"path"
	"strings"
)

const (
	handlerTemplatePath = "../codegen/handler.go.template"
	handlerDirPath      = "../controller"

	typeNameTemplate          = "{{typeName}}"
	typePathTemplate          = "{{typePath}}"
	additionalImportsTemplate = "{{additionalImports}}"
	conditionTypePathTemplate = "{{conditionTypePath}}"
)

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
	typeName := getTypeName(typePath)
	conditionTypePath := getConditionType(typePath, ctx)
	additionalImports := strings.Join(ctx.StringSlice("additional-imports"), "\n\t")

	rawTemplate, err := os.ReadFile(handlerTemplatePath)
	if err != nil {
		return fmt.Errorf("could not read template at '%s': %w", handlerTemplatePath, err)
	}

	data := strings.ReplaceAll(string(rawTemplate), typePathTemplate, typePath)
	data = strings.ReplaceAll(data, typeNameTemplate, typeName)
	data = strings.ReplaceAll(data, conditionTypePathTemplate, conditionTypePath)
	data = strings.ReplaceAll(data, additionalImportsTemplate, additionalImports)
	handlerPath := path.Join(handlerDirPath, fmt.Sprintf("%s.go", strings.ToLower(typeName)))

	if err := os.WriteFile(handlerPath, []byte(data), 0644); err != nil {
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

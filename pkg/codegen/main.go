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

	typeNameTemplate   = "{{typeName}}"
	typePathTemplate   = "{{typePath}}"
	importPathTemplate = "{{importPath}}"
)

func codegen(ctx *cli.Context) error {
	if n := ctx.NArg(); n == 0 {
		return fmt.Errorf("expected type path and name but found none")
	} else if n == 1 {
		return fmt.Errorf("expected type name but found none")
	} else if n > 2 {
		return fmt.Errorf("recevied unexpected arguments")
	}

	typePath := ctx.Args().Get(0)

	splitPath := strings.Split(typePath, ".")

	typeName := splitPath[1]
	importPath := fmt.Sprintf("%s \"%s\"", splitPath[0], ctx.Args().Get(1))

	rawTemplate, err := os.ReadFile(handlerTemplatePath)
	if err != nil {
		return fmt.Errorf("could not read template at '%s': %w", handlerTemplatePath, err)
	}

	data := strings.ReplaceAll(string(rawTemplate), typePathTemplate, typePath)
	data = strings.ReplaceAll(data, typeNameTemplate, typeName)
	data = strings.ReplaceAll(data, importPathTemplate, importPath)

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
		UsageText: "codege <typePath> <importPath>",
		Action:    codegen,
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

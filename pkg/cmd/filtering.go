package kubedump

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	"github.com/joshmeranda/kubedump/pkg/filter"
	cp "github.com/otiai10/copy"
)

type filteringOptions struct {
	Filter              filter.Expression
	DestinationBasePath string
	Logger              *slog.Logger
}

func isSymlink(filePath string) (bool, error) {
	info, err := os.Lstat(filePath)

	if err != nil {
		return false, fmt.Errorf("could not stat file '%s': %s", filePath, err)
	}

	return info.Mode()&os.ModeSymlink == os.ModeSymlink, nil
}

func filterKubedumpDir(dir string, opts filteringOptions) error {
	if err := os.MkdirAll(opts.DestinationBasePath, 0755); err != nil {
		return fmt.Errorf("could not create destination: %w", err)
	}

	if err := kubedump.ForEachResource(dir, func(builder kubedump.ResourcePathBuilder) error {
		return filterResourceDir(builder, opts)
	}); err != nil {
		return err
	}

	return nil
}

func filterResourceDir(builder kubedump.ResourcePathBuilder, opts filteringOptions) error {
	resourceDir := builder.Build()
	resourceFile := path.Join(resourceDir, builder.Name+".yaml")
	resource, err := kubedump.NewResourceFromFile(resourceFile)
	if err != nil {
		return fmt.Errorf("could not unmarshal resource file: %w", err)
	}

	if !opts.Filter.Matches(resource) {
		return nil
	}

	if err := copyResourceDir(resource, resourceDir, opts); err != nil {
		opts.Logger.Error(fmt.Sprintf("could not copy resource '%s': %s", resource, err))
	}

	return nil
}

func copyResourceDir(resource kubedump.Resource, resourceDir string, opts filteringOptions) error {
	resourceDestinationDir := kubedump.ResourcePathBuilder{}.
		WithBase(opts.DestinationBasePath).
		WithResource(resource).
		Build()

	// todo: will probably skip symlinks
	// copy resource dir
	if err := cp.Copy(resourceDir, resourceDestinationDir, cp.Options{}); err != nil {
		return fmt.Errorf("could not copy resource dir: %s", resourceDir)
	}

	// check for child resources
	entries, err := os.ReadDir(resourceDir)
	if err != nil {
		return fmt.Errorf("could not read resource dir '%s': %w", resourceDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if err := copySubResourceKind(entry.Name(), path.Join(resourceDir, entry.Name()), resource, opts); err != nil {
				opts.Logger.Error(fmt.Sprintf("could not copy '%s' resource for '%s': %s", entry.Name(), resource, err))
			}
		} else if entry.Name() != resource.GetName()+".yaml" && !strings.HasSuffix(entry.Name(), ".log") {
			opts.Logger.Warn(fmt.Sprintf("found unexpected file: %s", path.Join(resourceDir, entry.Name())))
		}
	}

	return nil
}

func copySubResourceKind(kind string, dir string, parent kubedump.Resource, opts filteringOptions) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("could not read dir for kind '%s' in resource '%s': %w", kind, parent, err)
	}

	for _, entry := range entries {
		linkPath := path.Join(dir, entry.Name())

		if isLink, err := isSymlink(linkPath); isLink {
			linkDest, err := os.Readlink(linkPath)
			if err != nil {
				opts.Logger.Error(fmt.Sprintf("could not read link at '%s': %s", linkPath, err))
			}

			realPath := path.Clean(path.Join(dir, linkDest))

			resourceFile := path.Join(realPath, entry.Name()+".yaml")
			resource, err := kubedump.NewResourceFromFile(resourceFile)
			if err != nil {
				return fmt.Errorf("could not unmarshal resoruce file: %w", err)
			}

			if err := copyResourceDir(resource, realPath, opts); err != nil {
				opts.Logger.Error(fmt.Sprintf("could not copy resourec '%s': %s", resource, err))
			}
		} else if err != nil {
			opts.Logger.Error(fmt.Sprintf("could not check if path '%s' is a symbolic link: %s", linkPath, err))
		} else {
			opts.Logger.Warn(fmt.Sprintf("encountered unexpected file '%s'", path.Join(dir, entry.Name())))
		}
	}

	return nil
}

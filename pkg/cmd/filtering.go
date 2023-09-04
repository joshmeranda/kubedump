package kubedump

import (
	"fmt"
	"os"
	"path"
	"strings"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	"github.com/joshmeranda/kubedump/pkg/filter"
	cp "github.com/otiai10/copy"
	"go.uber.org/zap"
)

type filteringOptions struct {
	Filter              filter.Expression
	DestinationBasePath string
	Logger              *zap.SugaredLogger
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

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("could not read dump dir '%s': %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if err := filterNamespaceDir(entry.Name(), path.Join(dir, entry.Name()), opts); err != nil {
				opts.Logger.Errorf("could not filter namespace '%s': %s", entry.Name(), err)
			}
		} else {
			if entry.Name() != LogFileName {
				opts.Logger.Warnf("encountered unexpected file '%s'", path.Join(dir, entry.Name()))
			}
		}
	}

	return nil
}

func filterNamespaceDir(namespace string, dir string, opts filteringOptions) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("could not read dir for namespace '%s': %w", namespace, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if err := filterKindDir(namespace, entry.Name(), path.Join(dir, entry.Name()), opts); err != nil {
				opts.Logger.Errorf("could not filter kind '%s' in namespace '%s': %s", entry.Name(), namespace, err)
			}
		} else {
			opts.Logger.Warnf("encountered unexpected file '%s'", path.Join(dir, entry.Name()))
		}
	}

	return nil
}

func filterKindDir(namespace string, kind string, dir string, opts filteringOptions) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("could not read dir for kind '%s' in namespace '%s': %w", kind, namespace, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if err := filterResourceDir(entry.Name(), path.Join(dir, entry.Name()), opts); err != nil {
				opts.Logger.Errorf("could not filter resource '%s/%s' in namespace '%s': %s", kind, entry.Name(), namespace, err)
			}
		} else {
			opts.Logger.Warnf("encountered unexpected file '%s'", path.Join(dir, entry.Name()))
		}
	}

	return nil
}

func filterResourceDir(name string, dir string, opts filteringOptions) error {
	resourceFile := path.Join(dir, name+".yaml")
	resource, err := kubedump.NewResourceFromFile(resourceFile)
	if err != nil {
		return fmt.Errorf("could not unmarshal resource file: %w", err)
	}

	if !opts.Filter.Matches(resource) {
		return nil
	}

	if err := copyResourceDir(resource, dir, opts); err != nil {
		opts.Logger.Errorf("could not copy resource '%s': %s", resource, err)
	}

	return nil
}

func copyResourceDir(resource kubedump.Resource, dir string, opts filteringOptions) error {
	resourceDestinationDir := kubedump.NewResourcePathBuilder().
		WithBase(opts.DestinationBasePath).
		WithResource(resource).
		Build()

	// todo: will probably skip symlinks
	// copy resource dir
	if err := cp.Copy(dir, resourceDestinationDir, cp.Options{}); err != nil {
		return fmt.Errorf("could not copy resource dir: %s", dir)
	}

	// check for child resources
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("could not read resource dir '%s': %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if err := copySubResourceKind(entry.Name(), path.Join(dir, entry.Name()), resource, opts); err != nil {
				opts.Logger.Errorf("could not copy '%s' resource for '%s': %s", entry.Name(), resource, err)
			}
		} else if entry.Name() != resource.GetName()+".yaml" && !strings.HasSuffix(entry.Name(), ".log") {
			opts.Logger.Warnf("found unexpected file: %s", path.Join(dir, entry.Name()))
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
				opts.Logger.Errorf("could not read link at '%s': %s", linkPath, err)
			}

			realPath := path.Clean(path.Join(dir, linkDest))

			resourceFile := path.Join(realPath, entry.Name()+".yaml")
			resource, err := kubedump.NewResourceFromFile(resourceFile)
			if err != nil {
				return fmt.Errorf("could not unmarshal resoruce file: %w", err)
			}

			if err := copyResourceDir(resource, realPath, opts); err != nil {
				opts.Logger.Errorf("could not copy resourec '%s': %s", resource, err)
			}
		} else if err != nil {
			opts.Logger.Errorf("could not check if path '%s' is a symbolic link: %s", linkPath, err)
		} else {
			opts.Logger.Warnf("encountered unexpected file '%s'", path.Join(dir, entry.Name()))
		}
	}

	return nil
}

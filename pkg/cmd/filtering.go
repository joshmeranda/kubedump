package kubedump

import (
	"fmt"
	cp "github.com/otiai10/copy"
	"go.uber.org/zap"
	kubedump "kubedump/pkg"
	"kubedump/pkg/filter"
	"os"
	"path"
)

type filterOptions struct {
	Filter              filter.Expression
	DestinationBasePath string
	Logger              *zap.SugaredLogger
}

func filterKubedumpDir(dir string, opts filterOptions) error {
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

func filterNamespaceDir(namespace string, dir string, opts filterOptions) error {
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

func filterKindDir(namespace string, kind string, dir string, opts filterOptions) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("could not read dir for kind '%s' in namespace '%s': %w", kind, namespace, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if err := filterResourceDir(kind, entry.Name(), path.Join(dir, entry.Name()), opts); err != nil {
				opts.Logger.Errorf("could not filter resource '%s/%s' in namespace '%s': %s", kind, entry.Name(), namespace, err)
			}
		} else {
			opts.Logger.Warnf("encountered unexpected file '%s'", path.Join(dir, entry.Name()))
		}
	}

	return nil
}

func filterResourceDir(kind string, name string, dir string, opts filterOptions) error {
	resourceFile := path.Join(dir, name+".yaml")
	handledResource, err := kubedump.NewHandledResourceFromFile(kubedump.HandleFilter, kind, resourceFile)
	if err != nil {
		return fmt.Errorf("could not unmarshal resoruce file: %w", err)
	}

	resourceDestinationDir := kubedump.NewResourcePathBuilder().
		WithBase(opts.DestinationBasePath).
		WithResource(handledResource).
		Build()

	if opts.Filter.Matches(handledResource) {
		opts.Logger.Debugf("resource '%s' matched filter", handledResource)

		// todo: will probably skip symlinks
		if err := cp.Copy(dir, resourceDestinationDir, cp.Options{}); err != nil {
			return fmt.Errorf("could not copy resource dir: %s", dir)
		}
	}

	return nil
}

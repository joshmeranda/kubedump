package kubedump

import (
	"fmt"
	"os"
)

type ForEachFunc = func(ResourcePathBuilder) error

func ForEachResource(base string, fn ForEachFunc) error {
	return ForEachKind(base, func(builder ResourcePathBuilder) error {
		dir := builder.BuildKind()
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("could not read directory '%s': %w", dir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				kindBuilder := builder.WithName(entry.Name())
				if err := fn(kindBuilder); err != nil {
					return fmt.Errorf("ForEachFunc failed for namespace '%s': %w", entry.Name(), err)
				}
			}
		}

		return nil
	})
}

func ForEachKind(base string, fn ForEachFunc) error {
	return ForEachNamespace(base, func(builder ResourcePathBuilder) error {
		dir := builder.BuildNamespace()
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("could not read directory '%s': %w", dir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				kindBuilder := builder.WithKind(entry.Name())
				if err := fn(kindBuilder); err != nil {
					return fmt.Errorf("ForEachFunc failed for namespace '%s': %w", entry.Name(), err)
				}
			}
		}

		return nil
	})
}

// ForEachNamespace iterates over each namespace directory and passes the ResourcePathBuilder to fn.
func ForEachNamespace(base string, fn ForEachFunc) error {
	entries, err := os.ReadDir(base)
	if err != nil {
		return fmt.Errorf("could not read directory '%s': %w", base, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			builder := ResourcePathBuilder{}.WithBase(base).WithNamespace(entry.Name())
			if err := fn(builder); err != nil {
				return fmt.Errorf("ForEachFunc failed for namespace '%s': %w", entry.Name(), err)
			}
		}
	}

	return nil
}

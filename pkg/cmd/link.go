package kubedump

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	apicorev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/yaml"
)

// createPathParents ensures that the parent directory for filePath exists.
// todo: duplicated in pkg/controller/utils.go
func createPathParents(filePath string) error {
	dirname := path.Dir(filePath)

	if err := os.MkdirAll(dirname, 0755); err != nil {
		return err
	}

	return nil
}

func linkToParent(childBuilder kubedump.ResourcePathBuilder, parentBuilder kubedump.ResourcePathBuilder) error {
	ownerPath := parentBuilder.Build()
	if _, err := os.Lstat(path.Dir(ownerPath)); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("parent does not exist, doing nothing")
	}

	resourcePath := childBuilder.Build()
	linkBuilder := childBuilder.WithParentKind(parentBuilder.Kind).WithParentName(parentBuilder.Name)
	linkDest := linkBuilder.BuildWithParent()

	relative, err := filepath.Rel(linkDest, resourcePath)
	if err != nil {
		return fmt.Errorf("could not get relative path for '%s' and '%s': %w", linkDest, resourcePath, err)
	}

	if err := createPathParents(linkDest); err != nil {
		return fmt.Errorf("could not create parents for '%s': %w", linkDest, err)
	}

	if err := os.Symlink(relative, linkDest); err != nil && !os.IsExist(err) {
		return fmt.Errorf("could not create symlink '%s': %w", linkDest, err)
	}

	return nil
}

func linkPod(podPathBuilder kubedump.ResourcePathBuilder) error {
	data, err := os.ReadFile(path.Join(podPathBuilder.Build(), podPathBuilder.Name+".yaml"))
	if err != nil {
		return fmt.Errorf("could not read resource file: %w", err)
	}

	pod := apicorev1.Pod{}
	if err := yaml.Unmarshal(data, &pod); err != nil {
		return fmt.Errorf("could not unmarshal data to suntructured: %w", err)
	}

	for _, volume := range pod.Spec.Volumes {
		var kind string
		if src := volume.VolumeSource; src.Secret != nil {
			kind = "Secret"
		} else if src.ConfigMap != nil {
			kind = "ConfigMap"
		} else {
			continue
		}

		volumeBuilder := kubedump.ResourcePathBuilder{}.
			WithBase(podPathBuilder.BasePath).
			WithNamespace(podPathBuilder.Namespace).
			WithKind(kind).
			WithName(volume.Name)

		if err := linkToParent(volumeBuilder, podPathBuilder); err != nil {
			return fmt.Errorf("could not link create link: %w", err)
		}
	}

	return nil
}

func linkResource(builder kubedump.ResourcePathBuilder) error {
	resourcePath := builder.Build()
	resourceFilePath := path.Join(resourcePath, builder.Name+".yaml")
	resource, err := kubedump.NewResourceFromFile(resourceFilePath)
	if err != nil {
		return fmt.Errorf("could not read resource from file resource: %w", err)
	}

	if resource.GetKind() == "Pod" {
		if err := linkPod(builder); err != nil {
			return fmt.Errorf("could not link pod '%s': %w", resource.GetName(), err)
		}
	}

	for _, owner := range resource.GetOwnershipReferences() {
		ownerBuilder := kubedump.ResourcePathBuilder{}.
			WithBase(builder.BasePath).
			WithNamespace(builder.Namespace).
			WithKind(owner.Kind).
			WithName(owner.Name)

		if err := linkToParent(builder, ownerBuilder); err != nil {
			return fmt.Errorf("could not link create link: %w", err)
		}
	}

	return nil
}

func LinkDump(base string) error {
	return kubedump.ForEachResource(base, func(builder kubedump.ResourcePathBuilder) error {
		if err := linkResource(builder); err != nil {
			return fmt.Errorf("could not link resource '%s': %w", builder.Name, err)
		}

		return nil
	})
}

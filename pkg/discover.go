package kubedump

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

func Discover(config *rest.Config) ([]schema.GroupVersionResource, error) {
	client, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create client for discovery: %w", err)
	}

	apiResources, err := discovery.ServerPreferredNamespacedResources(client)
	if err != nil {
		return nil, fmt.Errorf("could not get server resources: %w", err)
	}

	resourceGroupVersions := make([]schema.GroupVersionResource, 0)
	for _, list := range apiResources {
		groupVersion, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			return nil, fmt.Errorf("could not parse group version '%s': %w", list.GroupVersion, err)
		}

		for _, resource := range list.APIResources {
			resourceGroupVersions = append(resourceGroupVersions, schema.GroupVersionResource{
				Group:    groupVersion.Group,
				Version:  groupVersion.Version,
				Resource: resource.Name,
			})
		}
	}

	return resourceGroupVersions, nil
}

package kubedump

import (
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type RESTClientGetter struct {
	Namespace  string
	Kubeconfig []byte
	Restconfig *rest.Config
}

func (getter *RESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	if getter.Restconfig != nil {
		return getter.Restconfig, nil
	}

	if getter.Kubeconfig != nil {
		return clientcmd.RESTConfigFromKubeConfig(getter.Kubeconfig)
	}

	return nil, fmt.Errorf("could not establish restconfig")
}

func (getter *RESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := getter.ToRESTConfig()

	if err != nil {
		return nil, fmt.Errorf("could not Create discovery client: %w", err)
	}

	discoveryClient, _ := discovery.NewDiscoveryClientForConfig(config)

	return memory.NewMemCacheClient(discoveryClient), nil
}

func (getter *RESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := getter.ToDiscoveryClient()

	if err != nil {
		return nil, fmt.Errorf("could not Create rest mapper: %w", err)
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)

	return expander, nil
}

func (getter *RESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	overrides := &clientcmd.ConfigOverrides{
		ClusterDefaults: clientcmd.ClusterDefaults,
	}

	overrides.Context.Namespace = getter.Namespace

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}

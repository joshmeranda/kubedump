package main

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
	namespace  string
	kubeConfig []byte
	restConfig *rest.Config

	//opts []RESTCLientOption
}

func (getter *RESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	if getter.restConfig != nil {
		return getter.restConfig, nil
	}

	if getter.kubeConfig != nil {
		return clientcmd.RESTConfigFromKubeConfig(getter.kubeConfig)
	}

	return nil, fmt.Errorf("could not establish restconfig")
}

func (getter *RESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := getter.ToRESTConfig()

	if err != nil {
		return nil, fmt.Errorf("could not create discovery client: %w", err)
	}

	discoveryClient, _ := discovery.NewDiscoveryClientForConfig(config)

	return memory.NewMemCacheClient(discoveryClient), nil
}

func (getter *RESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := getter.ToDiscoveryClient()

	if err != nil {
		return nil, fmt.Errorf("could not create rest mapper: %w", err)
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

	overrides.Context.Namespace = getter.namespace

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}

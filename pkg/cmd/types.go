package kubedump

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"go.uber.org/zap"
	"io"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"kubedump/pkg/controller"
	"kubedump/pkg/filter"
	"net/http"
	"os"
	"sync"
	"time"
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

type HandlerOptions struct {
	LogSyncTimeout time.Duration
}

type Handler struct {
	HandlerOptions

	clusterController *controller.Controller
	lock              *sync.Mutex

	logger *zap.SugaredLogger
}

func NewHandler(logger *zap.SugaredLogger, opts HandlerOptions) Handler {
	return Handler{
		lock:   &sync.Mutex{},
		logger: logger.Named("http"),
	}
}

func (handler *Handler) errorResponse(w http.ResponseWriter, message string, statusCode int) {
	handler.logger.Errorf(message)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := make(map[string]string)
	response["message"] = message

	jsonResponse, _ := json.Marshal(response)
	w.Write(jsonResponse)
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler.logger.Debugf("recevied request to '%s'", r.URL.String())

	switch r.URL.Path {
	case "/health":
		handler.handleHealth(w, r)
	case "/tar":
		handler.handleTar(w, r)
	case "/start":
		handler.handleStart(w, r)
	case "/stop":
		handler.handleStop(w, r)
	default:
		handler.errorResponse(w, "unknown path: "+r.URL.Path, http.StatusNotFound)
	}
}

func (handler *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("OK"))
}

func (handler *Handler) handleTar(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		file, err := os.Create("/tmp/archive.tar.gz")

		if err != nil {
			handler.errorResponse(w, fmt.Sprintf("could not open temporary archive file: %s", err), http.StatusInternalServerError)
			return
		}

		handler.clusterController.Sync()

		// todo: support better speed / compression
		compressor := gzip.NewWriter(file)
		archiver := tar.NewWriter(compressor)

		err = archiveTree(BasePath, archiver)

		if err != nil {
			handler.errorResponse(w, fmt.Sprintf("could not archive '%s': %s", BasePath, err), http.StatusInternalServerError)
			return
		}

		// flush archive writes
		if err := archiver.Close(); err != nil {
			handler.errorResponse(w, fmt.Sprintf("could not close arhive writer: %s", err), http.StatusInternalServerError)
			return
		}

		if err := compressor.Close(); err != nil {
			handler.errorResponse(w, fmt.Sprintf("could not close arhive writer: %s", err), http.StatusInternalServerError)
			return
		}

		_, err = file.Seek(0, io.SeekStart)

		if err != nil {
			handler.errorResponse(w, fmt.Sprintf("could not seek start of archive: %s", err), http.StatusInternalServerError)
			return
		}

		_, err = io.Copy(w, file)

		if err != nil {
			handler.errorResponse(w, fmt.Sprintf("could not copy archive to response: %s", err), http.StatusInternalServerError)
		}

		w.Header().Set("Content-Type", "application/tar")
	default:
		handler.errorResponse(w, fmt.Sprintf("method is not supported: %s", r.Method), http.StatusMethodNotAllowed)
	}
}

func (handler *Handler) handleStart(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		f, err := filter.Parse(r.URL.Query().Get("filter"))
		if err != nil {
			handler.errorResponse(w, fmt.Sprintf("could not parse query filter as filter: %s", err), http.StatusBadRequest)
		}

		opts := controller.Options{
			BasePath:       BasePath,
			Filter:         f,
			LogSyncTimeout: handler.LogSyncTimeout,
		}

		config, err := rest.InClusterConfig()
		if err != nil {
			handler.errorResponse(w, fmt.Sprintf("could not create internal config: %s", err), http.StatusInternalServerError)
			return
		}

		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			handler.errorResponse(w, fmt.Sprintf("could not create internal client: %s", err), http.StatusInternalServerError)
			return
		}

		c, err := controller.NewController(client, opts)
		if err != nil {
			handler.errorResponse(w, fmt.Sprintf("could not create controller: %s", err), http.StatusInternalServerError)
			return
		}

		handler.lock.Lock()

		handler.clusterController = c

		handler.logger.Infof("starting collector for cluster")
		if err := handler.clusterController.Start(5); err != nil {
			handler.errorResponse(w, fmt.Sprintf("could not start collector for cluster: %s", err), http.StatusInternalServerError)
		}

		handler.lock.Unlock()
	default:
		handler.errorResponse(w, fmt.Sprintf("method is not supported: %s", r.Method), http.StatusMethodNotAllowed)
	}
}

func (handler *Handler) handleStop(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if handler.clusterController == nil {
			handler.errorResponse(w, "cluster collector is not running", http.StatusInternalServerError)
		}

		handler.lock.Lock()

		if err := handler.clusterController.Stop(); err != nil {
			handler.errorResponse(w, fmt.Sprintf("could not stop collector for cluster: %s", err), http.StatusInternalServerError)
		}

		handler.clusterController = nil

		handler.lock.Unlock()
	default:
		handler.errorResponse(w, fmt.Sprintf("method is not supported: %s", r.Method), http.StatusMethodNotAllowed)
	}
}

package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubedump "kubedump/pkg"
	"kubedump/pkg/collector"
	"kubedump/pkg/filter"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ParentPath = "/var/lib/kubedump"
)

func errorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := make(map[string]string)
	response["message"] = message

	jsonResponse, _ := json.Marshal(response)
	w.Write(jsonResponse)
}

func queryFloat64OrDefault(value string, defaultValue float64) (float64, error) {
	if value == "" {
		return defaultValue, nil
	}

	v, err := strconv.ParseFloat(value, 64)

	if err != nil {
		return 0, err
	}

	return v, nil
}

func durationFromSeconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second) * float64(time.Millisecond))
}

func getArchivePath(dir string, name string) string {
	trimmed := strings.TrimPrefix(dir, ParentPath)
	return path.Join(path.Base(ParentPath), trimmed, name)
}

func archiveTree(dir string, writer *tar.Writer) error {
	entries, err := os.ReadDir(dir)

	if err != nil {
		return fmt.Errorf("could not read directory '%s': %w", dir, err)
	}

	for _, entry := range entries {
		entryPath := path.Join(dir, entry.Name())

		logrus.Infof("looking at entry '%s'", entryPath)

		if entry.IsDir() {
			if err = archiveTree(entryPath, writer); err != nil {
				return err
			}

			continue
		}

		file, err := os.Open(entryPath)

		if err != nil {
			return fmt.Errorf("could not open file at '%s': %w", entryPath, err)
		}

		info, err := entry.Info()

		if err != nil {
			return fmt.Errorf("could not get file info for file '%s': %w", entryPath, err)
		}

		hdr, err := tar.FileInfoHeader(info, entry.Name())
		hdr.Name = getArchivePath(dir, entry.Name())

		if err != nil {
			return fmt.Errorf("could not construct header for file '%s': %w", entryPath, err)
		}

		err = writer.WriteHeader(hdr)

		if err != nil {
			return fmt.Errorf("could not write header for file '%s': %w", entryPath, err)
		}

		_, err = io.Copy(writer, file)

		if err != nil {
			return fmt.Errorf("could not copy file '%s' to archive: %w", entryPath, err)
		}
	}

	return nil
}

type KubedumpHandler struct {
	clusterCollector *collector.ClusterCollector
	lock             *sync.Mutex
}

func NewHandler() KubedumpHandler {
	return KubedumpHandler{
		lock: &sync.Mutex{},
	}
}

func (handler *KubedumpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("recevied request to '%s'", r.URL.String())

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
		errorResponse(w, "unknown path: "+r.URL.Path, http.StatusNotFound)
	}
}

func (handler *KubedumpHandler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("OK"))
}

func (handler *KubedumpHandler) handleTar(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		file, err := os.Create("/tmp/archive.tar.gz")

		if err != nil {
			errorResponse(w, fmt.Sprintf("could not open temporary archive file: %s", err), http.StatusInternalServerError)
			return
		}

		// todo: support better speed / compression
		compressor := gzip.NewWriter(file)
		archiver := tar.NewWriter(compressor)

		err = archiveTree(ParentPath, archiver)

		if err != nil {
			errorResponse(w, fmt.Sprintf("could not archive '%s': %s", ParentPath, err), http.StatusInternalServerError)
			return
		}

		// flush archive writes
		if err := archiver.Close(); err != nil {
			errorResponse(w, fmt.Sprintf("could not close arhive writer: %s", err), http.StatusInternalServerError)
			return
		}

		if err := compressor.Close(); err != nil {
			errorResponse(w, fmt.Sprintf("could not close arhive writer: %s", err), http.StatusInternalServerError)
			return
		}

		_, err = file.Seek(0, io.SeekStart)

		if err != nil {
			errorResponse(w, fmt.Sprintf("could not seek start of archive: %s", err), http.StatusInternalServerError)
			return
		}

		_, err = io.Copy(w, file)

		if err != nil {
			errorResponse(w, fmt.Sprintf("could not copy archive to response: %s", err), http.StatusInternalServerError)
		}
	default:
		errorResponse(w, fmt.Sprintf("method is not supported: %s", r.Method), http.StatusMethodNotAllowed)
	}
}

func (handler *KubedumpHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		podLogInterval, err := queryFloat64OrDefault(r.URL.Query().Get("pod-log-interval"), kubedump.DefaultPodLogInterval)

		if err != nil {
			errorResponse(w, fmt.Sprintf("could not parse query pod-log-interval as float: %s", err), http.StatusBadRequest)
			return
		}

		podDescInterval, err := queryFloat64OrDefault(r.URL.Query().Get("pod-desc-interval"), kubedump.DefaultPodDescriptionInterval)

		if err != nil {
			errorResponse(w, fmt.Sprintf("could not parse query pod-desc-interval as float: %s", err), http.StatusBadRequest)
			return
		}

		jobDescInterval, err := queryFloat64OrDefault(r.URL.Query().Get("job-desc-interval"), kubedump.DefaultJobDescriptionInterval)

		if err != nil {
			errorResponse(w, fmt.Sprintf("could not parse query job-desc-interval as float: %s", err), http.StatusBadRequest)
			return
		}

		f, err := filter.Parse(r.URL.Query().Get("filter"))

		if err != nil {
			errorResponse(w, fmt.Sprintf("could not parse query filter as filter: %s", err), http.StatusBadRequest)
		}

		opts := collector.ClusterCollectorOptions{
			ParentPath: ParentPath,
			Filter:     f,
			NamespaceCollectorOptions: collector.NamespaceCollectorOptions{
				ParentPath: ParentPath,
				Filter:     f,
				PodCollectorOptions: collector.PodCollectorOptions{
					ParentPath:          ParentPath,
					LogInterval:         durationFromSeconds(podLogInterval),
					DescriptionInterval: durationFromSeconds(podDescInterval),
				},
				JobCollectorOptions: collector.JobCollectorOptions{
					ParentPath:          ParentPath,
					DescriptionInterval: durationFromSeconds(jobDescInterval),
				},
			},
		}

		config, err := rest.InClusterConfig()

		if err != nil {
			errorResponse(w, fmt.Sprintf("could not create internal config: %s", err), http.StatusInternalServerError)
			return
		}

		client, err := kubernetes.NewForConfig(config)

		if err != nil {
			errorResponse(w, fmt.Sprintf("could not create internal client: %s", err), http.StatusInternalServerError)
			return
		}

		clusterCollector := collector.NewClusterCollector(client, opts)

		handler.lock.Lock()

		handler.clusterCollector = clusterCollector

		logrus.Infof("starting collector for cluster")
		if err := handler.clusterCollector.Start(); err != nil {
			errorResponse(w, fmt.Sprintf("could not start collector for cluster: %s", err), http.StatusInternalServerError)
		}

		handler.lock.Unlock()
	default:
		errorResponse(w, fmt.Sprintf("method is not supported: %s", r.Method), http.StatusMethodNotAllowed)
	}
}

func (handler *KubedumpHandler) handleStop(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if handler.clusterCollector == nil {
			errorResponse(w, "cluster collector is not running", http.StatusInternalServerError)
		}

		handler.lock.Lock()

		if err := handler.clusterCollector.Stop(); err != nil {
			errorResponse(w, fmt.Sprintf("could not stop collector for cluster: %s", err), http.StatusInternalServerError)
		}

		handler.clusterCollector = nil

		handler.lock.Unlock()
	default:
		errorResponse(w, fmt.Sprintf("method is not supported: %s", r.Method), http.StatusMethodNotAllowed)
	}
}

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	handler := NewHandler()

	logrus.Infof("starting server...")

	err := http.ListenAndServe(fmt.Sprintf(":%d", kubedump.Port), &handler)

	if err != nil {
		logrus.Fatal("error starting http server: %s", err)
	}
}

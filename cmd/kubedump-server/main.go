package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"io"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubedump "kubedump/pkg"
	"kubedump/pkg/controller"
	"kubedump/pkg/filter"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
)

var (
	ParentPath = path.Join(string(os.PathSeparator), "var", "lib", "kubedump")
)

func errorResponse(w http.ResponseWriter, message string, statusCode int) {
	logrus.Errorf(message)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := make(map[string]string)
	response["message"] = message

	jsonResponse, _ := json.Marshal(response)
	w.Write(jsonResponse)
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
	clusterController *controller.Controller
	lock              *sync.Mutex
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

		handler.clusterController.Sync()

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
		f, err := filter.Parse(r.URL.Query().Get("filter"))

		if err != nil {
			errorResponse(w, fmt.Sprintf("could not parse query filter as filter: %s", err), http.StatusBadRequest)
		}

		opts := controller.Options{
			ParentPath: ParentPath,
			Filter:     f,
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

		c := controller.NewController(client, opts)

		handler.lock.Lock()

		handler.clusterController = c

		logrus.Infof("starting collector for cluster")
		if err := handler.clusterController.Start(5); err != nil {
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
		if handler.clusterController == nil {
			errorResponse(w, "cluster collector is not running", http.StatusInternalServerError)
		}

		handler.lock.Lock()

		if err := handler.clusterController.Stop(); err != nil {
			errorResponse(w, fmt.Sprintf("could not stop collector for cluster: %s", err), http.StatusInternalServerError)
		}

		handler.clusterController = nil

		handler.lock.Unlock()
	default:
		errorResponse(w, fmt.Sprintf("method is not supported: %s", r.Method), http.StatusMethodNotAllowed)
	}
}

func run(ctx *cli.Context) error {
	if ctx.Bool("verbose") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	handler := NewHandler()

	logrus.Infof("starting server...")

	err := http.ListenAndServe(fmt.Sprintf(":%d", kubedump.Port), &handler)

	if err != nil {
		logrus.Fatal("error starting http server: %s", err)
	}

	return nil
}

func main() {
	app := &cli.App{
		Name:    "kubedump-server",
		Usage:   "collect k8s cluster resources and logs using a local client",
		Version: "0.2.0",
		Action:  run,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Usage:   "run kubedump-server verbosely",
				Aliases: []string{"V"},
				EnvVars: []string{"KUBEDUMP_SERVER_DEBUG"},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Errorf("Error: %s", err)
	}
}

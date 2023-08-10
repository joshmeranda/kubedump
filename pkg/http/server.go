package http

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joshmeranda/kubedump/pkg/controller"
	"github.com/joshmeranda/kubedump/pkg/filter"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

const (
	healthOK = "OK"
)

type ServerOptions struct {
	LogSyncTimeout time.Duration
	Logger         *zap.SugaredLogger
	BasePath       string
	Address        string
	Context        context.Context
	ClientConfig   *rest.Config
	FakeClient     *fake.Clientset
}

type Server struct {
	ServerOptions

	clusterControllerMu *sync.RWMutex
	clusterController   *controller.Controller

	server http.Server
}

func NewServer(opts ServerOptions) (*Server, error) {
	server := &Server{
		ServerOptions:       opts,
		clusterControllerMu: &sync.RWMutex{},
	}

	engine := gin.Default()

	engine.GET("/health", server.handleHealth)
	engine.GET("/start", server.handleStart)
	engine.GET("/stop", server.handleStop)
	engine.GET("/tar", server.handleTar)

	server.server = http.Server{
		Handler: engine,
		Addr:    opts.Address,
	}

	return server, nil
}

func (server *Server) Start() error {
	var err error

	go func() {
		server.Logger.Infof("server listening on '%s'", server.Address)

		if err = server.server.ListenAndServe(); err != nil {
			err = fmt.Errorf("failed to start server: %s", err)
		}
	}()

	return nil
}

func (server *Server) Shutdown() error {
	server.Logger.Infof("stopping server")

	if err := server.server.Shutdown(server.Context); err != nil {
		return fmt.Errorf("error shutting down server: %s", err)
	}

	return nil
}

func (server *Server) handleHealth(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

// todo: this should support all the same options as "kubedump dump"
func (server *Server) handleStart(ctx *gin.Context) {
	f, err := filter.Parse(ctx.Param("filter"))
	if err != nil {
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
	}

	opts := controller.Options{
		BasePath:       server.BasePath,
		LogSyncTimeout: server.LogSyncTimeout,
		Logger:         server.Logger,

		FakeClient: server.FakeClient,
	}

	if server.ClientConfig == nil {
		server.ClientConfig, err = rest.InClusterConfig()
		if err != nil {
			_ = ctx.AbortWithError(http.StatusBadRequest, err)
			return
		}
	}

	cont, err := controller.NewController(server.ClientConfig, opts)
	if err != nil {
		_ = ctx.AbortWithError(http.StatusBadRequest, err)
	}

	server.clusterControllerMu.Lock()
	defer server.clusterControllerMu.Unlock()

	server.clusterController = cont

	if err := server.clusterController.Start(5, f); err != nil {
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
}

func (server *Server) handleStop(ctx *gin.Context) {
	server.clusterControllerMu.Lock()
	defer server.clusterControllerMu.Unlock()

	if server.clusterController == nil {
		_ = ctx.AbortWithError(http.StatusBadRequest, fmt.Errorf("controller is not running"))
		return
	}

	if err := server.clusterController.Stop(); err != nil {
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	server.clusterController = nil
}

func (server *Server) handleTar(ctx *gin.Context) {
	file, err := os.CreateTemp(os.TempDir(), "kubedump-archive")
	if err != nil {
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer file.Close()

	// todo: support better speed / compression
	compressor := gzip.NewWriter(file)
	archiver := tar.NewWriter(compressor)

	err = archiveTree(server.BasePath, archiver)
	if err != nil {
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if err := archiver.Close(); err != nil {
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if err := compressor.Close(); err != nil {
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// todo: this part is likely unneeded
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.Header("Content-Type", "application/tar")
	ctx.File(file.Name())
}

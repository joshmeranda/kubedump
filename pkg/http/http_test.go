package http

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	kubedump "kubedump/pkg"
	"net"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"
)

const TestWaitDuration = time.Second * 5

// GetFreePort asks the kernel for a free open port that is ready to use.
func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func setupServerTest(t *testing.T) (func(), string, Client) {
	gin.SetMode(gin.TestMode)

	basePath := t.TempDir()
	logFilePath := path.Join(basePath, "kubedump.log")

	port, err := getFreePort()
	if err != nil {
		t.Fatalf("failed to get free ports: %s", err)
	}

	loggerOptions := []kubedump.LoggerOption{
		//kubedump.WithLevel(zap.NewAtomicLevelAt(zap.DebugLevel)),
		kubedump.WithPaths(logFilePath),
	}

	logger := kubedump.NewLogger(loggerOptions...)

	requestAddr := fmt.Sprintf("http://127.0.0.1:%d", port)
	listenAddr := fmt.Sprintf(":%d", port)

	kubeClientSet := fake.NewSimpleClientset()

	server, err := NewServer(ServerOptions{
		BasePath:       basePath,
		LogSyncTimeout: time.Second,
		Logger:         logger.Named("server"),
		Address:        listenAddr,
		Context:        context.Background(),
		kubeClientSet:  kubeClientSet,
	})
	if err != nil {
		t.Fatalf("could not create server: %s", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("could not start sever: %s", err)
	}

	u, err := url.Parse(requestAddr)
	if err != nil {
		t.Fatalf("cannot parse address '%s': %s", requestAddr, err)
	}

	opts := ClientOptions{
		Address: *u,
		Logger:  logger.Named("client"),
	}
	client, err := NewHttpClient(opts)
	if err != nil {
		t.Fatalf("could not create client: %s", err)
	}

	waitForHealthy(t, context.Background(), client)

	teardown := func() {
		server.Shutdown()
	}

	return teardown, basePath, client
}

func waitForHealthy(t *testing.T, ctx context.Context, client Client) {
	ctx, cancel := context.WithTimeout(ctx, TestWaitDuration)

	wait.UntilWithContext(ctx, func(ctx context.Context) {
		healthy, err := client.Health(HealthRequest{})

		if err == nil && healthy {
			cancel()
		}
	}, 0)

	if err := ctx.Err(); err != nil && !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("failed waiting for server to be healthy")
	}
}

func TestServerHealth(t *testing.T) {
	teardown, _, client := setupServerTest(t)
	defer teardown()

	healthy, err := client.Health(HealthRequest{})
	assert.NoError(t, err)
	assert.True(t, healthy)
}

func TestServerStart(t *testing.T) {
	teardown, _, client := setupServerTest(t)
	defer teardown()

	err := client.Start(StartRequest{})
	assert.NoError(t, err)
}

func TestServerStopNotRunningServer(t *testing.T) {
	teardown, _, client := setupServerTest(t)
	defer teardown()

	err := client.Stop(StopRequest{})
	assert.Error(t, err)
}

func TestServerStopRunningServer(t *testing.T) {
	teardown, _, client := setupServerTest(t)
	defer teardown()

	if err := client.Start(StartRequest{}); err != nil {
		t.Fatalf("server could not be started: %s", err)
	}

	err := client.Stop(StopRequest{})
	assert.NoError(t, err)
}

func TestServerTar(t *testing.T) {
	teardown, basePath, client := setupServerTest(t)
	defer teardown()

	tarPath := path.Join(basePath, "kubedump.tar.gz")

	if err := client.Start(StartRequest{}); err != nil {
		t.Fatalf("server could not be started: %s", err)
	}

	if err := client.Stop(StopRequest{}); err != nil {
		t.Fatalf("server could not be started: %s", err)
	}

	err := client.Tar(tarPath, TarRequest{})
	assert.NoError(t, err)
	assert.FileExists(t, tarPath)

	// todo: we probably want to actually check the contents of the received tar
}

package controller

import (
	"context"
	"fmt"
	kubedump "github.com/joshmeranda/kubedump/pkg"
	"io"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"os"
	"path"
	"time"
)

type Stream interface {
	// Sync new data from stream source into stream destination.
	Sync() error

	// Close the streamer.
	Close() error
}

type LogStreamOptions struct {
	Pod           *apicorev1.Pod
	Container     *apicorev1.Container
	Context       context.Context
	KubeClientSet kubernetes.Interface
	BasePath      string
	Timeout       time.Duration
}

type logStream struct {
	LogStreamOptions

	cancel context.CancelFunc

	out io.WriteCloser

	lastRead time.Time
}

func NewLogStream(opts LogStreamOptions) (Stream, error) {
	podDir := kubedump.NewResourcePathBuilder().
		WithBase(opts.BasePath).
		WithNamespace(opts.Pod.Namespace).
		WithKind("Pod").
		WithName(opts.Pod.Name).
		Build()

	logFilePath := path.Join(podDir, opts.Container.Name+".log")

	if err := createPathParents(logFilePath); err != nil {
		return nil, fmt.Errorf("could not create log file '%s': %w", logFilePath, err)
	}

	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("could not create log file '%s': %w", logFilePath, err)
	}

	return &logStream{
		LogStreamOptions: opts,
		out:              logFile,
		lastRead:         time.Time{},
	}, nil
}

func (stream *logStream) Sync() error {
	request := stream.KubeClientSet.CoreV1().Pods(stream.Pod.Namespace).GetLogs(stream.Pod.Name, &apicorev1.PodLogOptions{
		Container: stream.Container.Name,
		Follow:    false,
		Previous:  false,
		SinceTime: &apimetav1.Time{
			Time: stream.lastRead,
		},
	})

	ctx, cancel := context.WithTimeout(stream.Context, stream.Timeout)
	defer cancel()

	// todo: stream.lastRead may not be set immediately and cause race conditions
	stream.lastRead = time.Now()

	response := request.Do(ctx)
	if err := response.Error(); err != nil {
		return fmt.Errorf("error requesting logs: %w", err)
	}

	body, err := response.Raw()
	if err != nil {
		return fmt.Errorf("error requesting logs: %w", err)

	}

	if _, err := stream.out.Write(body); err != nil {
		return fmt.Errorf("error witing log response: %w", err)
	}

	return nil
}

func (stream *logStream) Close() error {
	err := stream.out.Close()

	return err
}

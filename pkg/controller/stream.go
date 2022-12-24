package controller

import (
	"context"
	"fmt"
	"io"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"os"
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
	ParentPath    string
}

type logStream struct {
	LogStreamOptions

	cancel context.CancelFunc

	out io.WriteCloser

	lastRead time.Time
}

func NewLogStream(opts LogStreamOptions) (Stream, error) {
	logFilePath := containerLogFilePath(opts.ParentPath, opts.Pod, opts.Container)

	if err := createPathParents(logFilePath); err != nil {
		return nil, fmt.Errorf("could not create log file '%s': %w", logFilePath, err)
	}

	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("could not create log file '%s': %w", logFilePath, err)
	}

	ctx, cancel := context.WithCancel(opts.Context)
	opts.Context = ctx

	return &logStream{
		LogStreamOptions: opts,
		cancel:           cancel,
		out:              logFile,
		lastRead:         time.Time{},
	}, nil
}

func (stream *logStream) Sync() error {
	// todo: stream.lastRead may not be set immediately and cause race conditions
	request := stream.KubeClientSet.CoreV1().Pods(stream.Pod.Namespace).GetLogs(stream.Pod.Name, &apicorev1.PodLogOptions{
		Container: stream.Container.Name,
		Follow:    false,
		Previous:  false,
		SinceTime: &apimetav1.Time{
			Time: stream.lastRead,
		},
		// todo: this might be interesting if we only want to collect logs while the logs are collected
		Timestamps: true,
	})

	stream.lastRead = time.Now()
	response := request.Do(stream.Context)
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
	stream.cancel()

	err := stream.out.Close()

	return err
}

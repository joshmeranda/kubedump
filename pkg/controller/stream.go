package controller

import (
	"context"
	"fmt"
	"io"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"os"
)

type Stream interface {
	// Sync new data from stream source into stream destination.
	Sync() error

	// Close the stream source and destinations and return both errors in that order.
	Close() (error, error)
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

	in  io.ReadCloser
	out io.WriteCloser
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

	request := opts.KubeClientSet.CoreV1().Pods(opts.Pod.Namespace).GetLogs(opts.Pod.Name, &apicorev1.PodLogOptions{
		Container: opts.Container.Name,
		Follow:    true,
		Previous:  false,
		//SinceSeconds:                 nil,
		//SinceTime:                    nil,
		// todo: this might be interesting if we only want to collect logs while the logs are collected
		//Timestamps:                   false,
		//TailLines:                    nil,
		//LimitBytes:                   nil,
		//InsecureSkipTLSVerifyBackend: false,
	})

	stream, err := request.Stream(opts.Context)
	if err != nil {
		return nil, fmt.Errorf("could not create log stream for container: %w", err)
	}

	ctx, cancel := context.WithCancel(opts.Context)
	opts.Context = ctx

	return &logStream{
		LogStreamOptions: opts,
		cancel:           cancel,
		in:               stream,
		out:              logFile,
	}, nil
}

func (stream *logStream) Sync() error {
	var buff []byte

	if _, err := stream.in.Read(buff); err != nil && err != context.Canceled {
		return fmt.Errorf("erro syncing stream: %w", err)
	}

	return nil
}

func (stream *logStream) Close() (error, error) {
	stream.cancel()

	inErr := stream.in.Close()
	outErr := stream.out.Close()

	return inErr, outErr
}

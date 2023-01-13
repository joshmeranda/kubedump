package http

import (
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"os"
)

type HealthRequest struct{}

type StartRequest struct{}

type StopRequest struct{}

type TarRequest struct{}

type ClientOptions struct {
	Address url.URL
	Logger  *zap.SugaredLogger
}

type Client interface {
	Health(HealthRequest) (bool, error)

	Start(StartRequest) error

	Stop(StopRequest) error

	// Tar requests a tar archive of the dumped resources from the server, and stores the file at the given path. The
	// given path cannot be empty.
	Tar(string, TarRequest) error
}

type httpClient struct {
	ClientOptions

	client http.Client
}

func NewHttpClient(opts ClientOptions) (Client, error) {
	return &httpClient{
		ClientOptions: opts,
		client:        http.Client{},
	}, nil
}

func (client httpClient) Health(HealthRequest) (bool, error) {
	client.Address.Path = "/health"

	response, err := client.client.Get(client.Address.String())
	if err != nil {
		return false, requestFailedError(err)
	}

	if !isResponseOk(response) {
		return false, responseCodeNotOk(response.StatusCode)
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read health from '%s': %w", client.Address.String(), err)
	}
	response.Body.Close()

	switch string(data) {
	case healthOK:
		return true, nil
	default:
		return false, fmt.Errorf("unexpected response from '%s': %s", client.Address.String(), string(data))
	}
}

func (client httpClient) Start(StartRequest) error {
	client.Address.Path = "/start"

	response, err := client.client.Get(client.Address.String())
	if err != nil {
		return requestFailedError(err)
	}

	if !isResponseOk(response) {
		return responseCodeNotOk(response.StatusCode)
	}

	return nil
}

func (client httpClient) Stop(StopRequest) error {
	client.Address.Path = "/stop"

	response, err := client.client.Get(client.Address.String())
	if err != nil {
		return requestFailedError(err)
	}

	if !isResponseOk(response) {
		return responseCodeNotOk(response.StatusCode)
	}

	return nil
}

func (client httpClient) Tar(tarPath string, _ TarRequest) error {
	if tarPath == "" {
		return fmt.Errorf("tarPath cannot be empty")
	}

	client.Address.Path = "/tar"

	response, err := client.client.Get(client.Address.String())

	if err != nil {
		return requestFailedError(err)
	}

	if !isResponseOk(response) {
		return responseCodeNotOk(response.StatusCode)
	}

	defer response.Body.Close()

	switch contentType := response.Header.Get("Content-Type"); contentType {
	case "application/json":
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("could not read respone body: %w", err)
		}

		var data map[string]string
		if err := json.Unmarshal(body, &data); err != nil {
			return fmt.Errorf("could not marshal response body: %w", err)
		}

		return fmt.Errorf("could not pull archive: %s", data["message"])
	case "application/tar":
		f, err := os.Create(tarPath)

		if err != nil {
			return fmt.Errorf("could not Create file: %w", err)
		}
		defer f.Close()

		_, err = io.Copy(f, response.Body)

		if err != nil {
			return fmt.Errorf("could not copy response body to file: %w", err)
		}
		client.Logger.Infof("copied tar to '%s'", tarPath)
	default:
		return fmt.Errorf("unsupported Content-Type '%s'", contentType)
	}

	return nil
}

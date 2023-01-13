package http

import (
	"archive/tar"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/util/json"
	"net/http"
	"os"
	"path"
	"strings"
)

func requestFailedError(err error) error {
	return fmt.Errorf("request failed: %w", err)
}

func responseCodeNotOk(code int) error {
	return fmt.Errorf("recevied response with status code '%d'", code)
}

func getArchivePath(dir string, name string) string {
	basePath := "/kubedump"
	trimmed := strings.TrimPrefix(dir, basePath)
	return path.Join(path.Base(basePath), trimmed, name)
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

func isResponseOk(response *http.Response) bool {
	return response.StatusCode >= 200 && response.StatusCode < 400
}

func unmarshalResponse(response *http.Response, obj interface{}) error {
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed reading response body: %w", err)
	}
	response.Body.Close()

	if err := json.Unmarshal(data, obj); err != nil {
		return fmt.Errorf("failed to unmarshal data to type '%T': %w", obj, err)
	}

	return nil
}

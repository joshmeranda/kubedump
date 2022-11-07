package kubedump

import (
	"archive/tar"
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"io"
	"io/ioutil"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kubedump "kubedump/pkg"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	chartVersion = "0.1.0"
	appVersion   = "0.2.0"

	chartReleaseUrl = fmt.Sprintf("https://github.com/joshmeranda/kubedump/releases/download/%s/kubedump-server-%s.tgz", appVersion, chartVersion)

	ParentPath = path.Join(string(os.PathSeparator), "var", "lib", "kubedump")
)

func serviceUrl(ctx *cli.Context, path string, queries map[string]string) (url.URL, error) {
	if endpoint := ctx.String("service-url"); endpoint != "" {
		u, err := url.Parse(endpoint)
		u.Path = path

		return *u, err
	}

	config, err := clientcmd.BuildConfigFromFlags("", ctx.String("kubeconfig"))

	if err != nil {
		return url.URL{}, fmt.Errorf("could not load config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		return url.URL{}, fmt.Errorf("could not load kubeconfig: %w", err)
	}

	service, err := client.CoreV1().Services(kubedump.Namespace).Get(context.TODO(), kubedump.ServiceName, apismeta.GetOptions{})

	if err != nil {
		return url.URL{}, fmt.Errorf("could not access service '%s': %w", kubedump.ServiceName, err)
	}

	q := url.Values{}
	for k, v := range queries {
		if v != "" {
			q.Set(k, v)
		}
	}

	serviceUrl := url.URL{
		Scheme:   "http",
		Host:     fmt.Sprintf("%s:%d", service.Spec.ClusterIP, service.Spec.Ports[0].Port),
		Path:     path,
		RawQuery: q.Encode(),
	}

	return serviceUrl, nil
}

func responseErrorMessage(response *http.Response) string {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return ""
	}

	if response.Header.Get("Content-Type") != "application/json" {
		return "could not read response from server"
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "could not read response body"
	}
	defer response.Body.Close()

	var data map[string]string
	err = json.Unmarshal(body, &data)

	if err != nil {
		return fmt.Sprintf("could not parse response from server: %s", err)
	}

	return data["error"]
}

func getAppDataDir() (string, error) {
	home, err := os.UserHomeDir()

	if err != nil {
		return "", fmt.Errorf("could not determine user home directory: %w", err)
	}

	return path.Join(home, ".local", "share", "kubedump"), nil
}

func getChartPath() (string, error) {
	dataDir, err := getAppDataDir()
	if err != nil {
		return "", fmt.Errorf("could not determine location for chart: %w", err)
	}

	return path.Join(dataDir, fmt.Sprintf("%s-%s.tgz", kubedump.HelmReleaseName, chartVersion)), nil
}

func pullChart(rawUrl string) (string, error) {
	dataDir, err := getAppDataDir()
	if err != nil {
		return "", fmt.Errorf("could not determine location for chart: %w", err)
	}

	return pullChartInto(rawUrl, dataDir)
}

func pullChartInto(rawUrl string, dir string) (string, error) {
	parsedUrl, err := url.Parse(rawUrl)
	if err != nil {
		return "", fmt.Errorf("could not parse given url '%s': %w", rawUrl, err)
	}

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", fmt.Errorf("could not Create parent directory for chart '%s': %w", dir, err)
	}

	fileName := path.Join(dir, filepath.Base(parsedUrl.Path))

	logrus.Infof("getting chart from: %s", rawUrl)
	resp, err := http.Get(rawUrl)
	if err != nil {
		return "", fmt.Errorf("could not Pull chart tat '%s': %w", rawUrl, err)
	}
	defer resp.Body.Close()

	f, err := os.Create(fileName)
	if err != nil {
		return "", fmt.Errorf("could not Create file '%s': %w", fileName, err)
	}
	defer f.Close()

	// todo: handle bad response
	if _, err = io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("could not copy response to file '%s': %w", fileName, err)
	}

	return fileName, nil
}

func ensureDefaultChart() (string, error) {
	chartFile, err := getChartPath()
	if err != nil {
		return "", fmt.Errorf("could not determine location for default chart: %w", err)
	}

	if _, err := os.Stat(chartFile); err == nil {
		return chartFile, nil
	}

	if _, err := pullChart(chartReleaseUrl); err != nil {
		return "", err
	}

	return chartFile, nil
}

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

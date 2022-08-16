package main

import (
	"context"
	"fmt"
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
	"time"
)

var (
	chartVersion    = "0.1.0"
	chartFile       = path.Join("var", "lib", fmt.Sprintf("kubedump-server-%s.tgz", chartVersion))
	chartReleaseUrl = fmt.Sprintf("https://github.com/joshmeranda/kubedump/releases/%s/downloads/kubedump-server-%s.tgz", chartVersion, chartVersion)
)

func serviceUrl(ctx *cli.Context, path string, queries map[string]string) (url.URL, error) {
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
		q.Set(k, v)
	}

	serviceUrl := url.URL{
		Scheme:   "http",
		Host:     fmt.Sprintf("%s:%d", service.Spec.ClusterIP, service.Spec.Ports[0].Port),
		Path:     path,
		RawQuery: q.Encode(),
	}

	return serviceUrl, nil
}

func durationFromSeconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second) * float64(time.Millisecond))
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

func pullChart(rawUrl string) (string, error) {
	return pullChartInto(rawUrl, path.Join("var", "lib"))
}

func pullChartInto(rawUrl string, dir string) (string, error) {
	parsedUrl, err := url.Parse(rawUrl)
	if err != nil {
		return "", fmt.Errorf("could not parse given url '%s': %w", rawUrl, err)
	}

	fileName := path.Join(dir, filepath.Base(parsedUrl.Path))

	resp, err := http.Get(rawUrl)
	if err != nil {
		return "", fmt.Errorf("could not pull chart tat '%s': %w", rawUrl, err)
	}
	defer resp.Body.Close()

	f, err := os.Create(fileName)
	if err != nil {
		return "", fmt.Errorf("could not create file '%s': %w", fileName, err)
	}
	defer f.Close()

	if _, err = io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("could not copy response to file '%s': %w", fileName, err)
	}

	return fileName, nil
}

func ensureDefaultChart() (string, error) {
	if _, err := os.Stat(chartFile); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("could not access file at '%s': %w", chartFile, err)
	}

	if _, err := pullChart(chartReleaseUrl); err != nil {
		return "", err
	}

	return chartFile, nil
}

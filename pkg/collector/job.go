package collector

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	kubedump "kubedump/pkg"
	"os"
	"sigs.k8s.io/yaml"
	"strconv"
	"sync"
	"time"
)

type JobCollector struct {
	rootPath                 string
	job                      *batchv1.Job
	jobClient                v1.JobInterface
	lastSyncedTransitionTime time.Time

	collecting bool
	wg         sync.WaitGroup
}

func NewJobCollector(rootPath string, jobClient v1.JobInterface, job *batchv1.Job) *JobCollector {
	return &JobCollector{
		rootPath:   rootPath,
		job:        job,
		jobClient:  jobClient,
		collecting: false,
		wg:         sync.WaitGroup{},
	}
}

func (collector *JobCollector) dumpCurrentJob() error {
	yamlPath := jobYamlPath(collector.rootPath, collector.job)

	if exists(yamlPath) {
		if err := os.Truncate(yamlPath, 0); err != nil {
			return fmt.Errorf("error truncating pod ymal file '%s' : %w", yamlPath, err)
		}
	}

	f, err := os.OpenFile(yamlPath, os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		return fmt.Errorf("could not open file '%s': %w", yamlPath, err)
	}

	podYaml, err := yaml.Marshal(collector.job)

	if err != nil {
		return fmt.Errorf("could not marshal jod: %w", err)
	}

	_, err = f.Write(podYaml)

	if err != nil {
		return fmt.Errorf("could not write to file '%s': %w", yamlPath, err)
	}

	return nil
}

func (collector *JobCollector) collectDescription(jobRefreshDuration time.Duration) {
	collector.wg.Add(1)

	logrus.Infof("collecting description for job '%s'", collector.job.Name)

	for collector.collecting {
		job, err := collector.jobClient.Get(context.TODO(), collector.job.Name, apismeta.GetOptions{})

		if err != nil {
			logrus.Errorf("could not job job '%s' in '%s': %s", collector.job.Name, collector.job.Namespace, err)
			continue
		}

		newestTransition := mostRecentJobTransitionTime(job.Status.Conditions)

		if newestTransition.After(collector.lastSyncedTransitionTime) {
			collector.job = job
			collector.lastSyncedTransitionTime = newestTransition

			if err := collector.dumpCurrentJob(); err != nil {
				logrus.Errorf("%s", err)
			}
		}

		time.Sleep(jobRefreshDuration)
	}

	logrus.Infof("stopping description for job '%s'", collector.job.Name)

	collector.wg.Done()
}

func (collector *JobCollector) Start() error {
	jobDirPath := jobDirPath(collector.rootPath, collector.job)

	if err := createPathParents(jobDirPath); err != nil {
		return fmt.Errorf("could not create collector: %w", err)
	}

	jobRefreshInterval, err := strconv.ParseFloat(os.Getenv(kubedump.PodRefreshIntervalEnv), 64)

	if err != nil {
		return fmt.Errorf("could not parse env '%s' to float64: %w", kubedump.JobRefreshIntervalEnv, err)
	}

	jobRefreshDuration := time.Duration(float64(time.Second) * jobRefreshInterval)

	collector.collecting = true

	go collector.collectDescription(jobRefreshDuration)

	return nil
}

func (collector *JobCollector) Stop() error {
	collector.collecting = false

	collector.wg.Done()

	return nil
}

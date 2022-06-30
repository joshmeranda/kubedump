package collector

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	apibatchv1 "k8s.io/api/batch/v1"
	apismeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	batchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	"os"
	"sigs.k8s.io/yaml"
	"sync"
	"time"
)

type JobCollectorOptions struct {
	ParentPath          string
	DescriptionInterval time.Duration
}

type JobCollector struct {
	job                      *apibatchv1.Job
	jobClient                batchv1.JobInterface
	lastSyncedTransitionTime time.Time

	collecting bool
	wg         sync.WaitGroup

	opts JobCollectorOptions
}

func NewJobCollector(jobClient batchv1.JobInterface, job *apibatchv1.Job, opts JobCollectorOptions) *JobCollector {
	return &JobCollector{
		job:        job,
		jobClient:  jobClient,
		collecting: false,
		wg:         sync.WaitGroup{},
		opts:       opts,
	}
}

func (collector *JobCollector) dumpCurrentJob() error {
	yamlPath := jobYamlPath(collector.opts.ParentPath, collector.job)

	if exists(yamlPath) {
		if err := os.Truncate(yamlPath, 0); err != nil {
			return fmt.Errorf("error truncating pod ymal file '%s' : %w", yamlPath, err)
		}
	} else {
		if err := createPathParents(yamlPath); err != nil {
			return fmt.Errorf("error creating parents for job file '%s': %s", yamlPath, err)
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

	logrus.WithFields(resourceFields(collector.job)).Info("collecting description for job")

	for collector.collecting {
		job, err := collector.jobClient.Get(context.TODO(), collector.job.Name, apismeta.GetOptions{})

		if err != nil {
			logrus.WithFields(resourceFields(collector.job)).Errorf("could not collect for job: %s", err)
			continue
		}

		newestTransition := mostRecentJobTransitionTime(job.Status.Conditions)

		if newestTransition.After(collector.lastSyncedTransitionTime) {
			collector.job = job
			collector.lastSyncedTransitionTime = newestTransition

			if err := collector.dumpCurrentJob(); err != nil {
				logrus.WithFields(resourceFields(collector.job)).Error(err)
			}
		}

		time.Sleep(jobRefreshDuration)
	}

	logrus.WithFields(resourceFields(collector.job)).Infof("stopping description for job")

	collector.wg.Done()
}

func (collector *JobCollector) Start() error {
	jobDirPath := jobDirPath(collector.opts.ParentPath, collector.job)

	if err := createPathParents(jobDirPath); err != nil {
		return fmt.Errorf("could not create collector: %w", err)
	}

	collector.collecting = true

	go collector.collectDescription(collector.opts.DescriptionInterval)

	return nil
}

func (collector *JobCollector) Stop() error {
	collector.collecting = false

	collector.wg.Wait()

	return nil
}

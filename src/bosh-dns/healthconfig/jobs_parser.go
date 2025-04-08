package healthconfig

import (
	"encoding/json"
	"io/ioutil" //nolint:staticcheck
	"os"
	"path/filepath"
)

type Job struct {
	HealthExecutablePath string
	Groups               []LinkMetadata
}

type LinkMetadata struct {
	Group string `json:"group"`
	Name  string `json:"name"`
	Type  string `json:"type"`

	JobName string
}

func ParseJobs(jobsDir string, executablePath string) ([]Job, error) {
	var jobs []Job

	jobDirs, err := ioutil.ReadDir(jobsDir) //nolint:staticcheck
	if err != nil {
		return nil, err
	}

	for _, jobDir := range jobDirs {
		isDir, err := isDirectory(filepath.Join(jobsDir, jobDir.Name()))
		if err != nil {
			return nil, err
		}

		if !isDir {
			continue
		}

		groups, err := parseLinkGroups(jobsDir, jobDir.Name())
		if err != nil {
			return nil, err
		}

		job := Job{Groups: groups}

		jobExecutablePath := filepath.Join(jobsDir, jobDir.Name(), executablePath)
		exists, err := fileExists(jobExecutablePath)
		if err != nil {
			return nil, err
		}

		if exists {
			job.HealthExecutablePath = jobExecutablePath
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

func isDirectory(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return info.IsDir(), nil
}

func parseLinkGroups(jobsDir, jobName string) ([]LinkMetadata, error) {
	linksPath := filepath.Join(jobsDir, jobName, ".bosh", "links.json")
	f, err := os.Open(linksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []LinkMetadata{}, nil
		}

		return nil, err
	}
	defer f.Close() //nolint:errcheck

	var links []LinkMetadata
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&links)
	if err != nil {
		return nil, err
	}

	for i := range links {
		links[i].JobName = jobName
	}

	return links, nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

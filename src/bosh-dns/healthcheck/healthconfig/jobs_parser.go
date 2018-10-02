package healthconfig

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Job struct {
	HealthExecutablePath string
	Groups               []string
}

type linkMetadata struct {
	Group string `json:group`
}

func ParseJobs(jobsDir string, executablePath string) ([]Job, error) {
	var jobs []Job

	jobDirs, err := ioutil.ReadDir(jobsDir)
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

		groups, err := parseLinkGroups(filepath.Join(jobsDir, jobDir.Name(), ".bosh", "links.json"))
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

func parseLinkGroups(linksPath string) ([]string, error) {
	f, err := os.Open(linksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}

		return nil, err
	}
	defer f.Close()

	var links []linkMetadata
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&links)
	if err != nil {
		return nil, err
	}

	var groups []string
	for _, link := range links {
		groups = append(groups, link.Group)
	}

	return groups, nil
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

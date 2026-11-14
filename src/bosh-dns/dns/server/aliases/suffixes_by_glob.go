package aliases

import (
	"maps"
	"path/filepath"
	"slices"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

const suffixesLogTag = "AliasSuffixes"

//counterfeiter:generate . NamedSuffixesLoader

type NamedSuffixesLoader interface {
	Load(string) (Suffixes, []error, error)
}

// SuffixesFromGlob logs and skips files and entries that cannot be used.
// Suffixes in ownFile win, then the first file in glob order.
func SuffixesFromGlob(globber ConfigGlobber, loader NamedSuffixesLoader, ownFile string, glob string, logger boshlog.Logger) Suffixes {
	var files []string
	if ownFile != "" {
		files = append(files, ownFile)
	}

	matches, err := globber.Glob(glob)
	if err != nil {
		logger.Warn(suffixesLogTag, "Ignoring alias suffix files matching %s because the glob failed: %s", glob, err)
	}
	for _, match := range matches {
		if ownFile == "" || filepath.Clean(match) != filepath.Clean(ownFile) {
			files = append(files, match)
		}
	}

	merged := NewSuffixes()
	sources := map[string]string{}

	for _, file := range files {
		suffixes, entryErrs, err := loader.Load(file)
		if err != nil {
			logger.Warn(suffixesLogTag, "Ignoring alias suffix file %s: %s", file, err)
			continue
		}

		for _, entryErr := range entryErrs {
			logger.Warn(suffixesLogTag, "Ignoring an entry in alias suffix file %s: %s", file, entryErr)
		}

		mergeSuffixes(merged.suffixes, suffixes.suffixes, "", file, sources, logger)
		mergeSuffixes(merged.underscoreSuffixes, suffixes.underscoreSuffixes, "_.", file, sources, logger)
	}

	return merged
}

// mergeSuffixes adds the entries of next that merged does not already have,
// and notes in sources which file each one came from. It logs when a later
// file gives an alias a different suffix, and keeps the earlier one.
func mergeSuffixes(merged map[string]string, next map[string]string, prefix string, file string, sources map[string]string, logger boshlog.Logger) {
	for _, host := range slices.Sorted(maps.Keys(next)) {
		suffix := next[host]
		alias := prefix + host

		existing, found := merged[host]
		if !found {
			merged[host] = suffix
			sources[alias] = file
			continue
		}

		if existing != suffix {
			logger.Info(suffixesLogTag, "Ignoring the suffix %s for alias %s from %s because %s gives it the suffix %s", suffix, alias, file, sources[alias], existing)
		}
	}
}

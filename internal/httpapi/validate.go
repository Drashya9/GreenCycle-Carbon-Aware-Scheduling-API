package httpapi

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	// maxDurationHours matches the 24-hour forecast horizon; anything longer can never fit.
	maxDurationHours = 24
	// maxMultiZips bounds the provider calls one /calculate/multi request can trigger.
	maxMultiZips = 10
)

var zipPattern = regexp.MustCompile(`^\d{5}$`)

func validateZip(zip string) error {
	if !zipPattern.MatchString(zip) {
		return fmt.Errorf("zip %q must be a 5-digit US zip code", zip)
	}
	return nil
}

// parseZipList splits a comma-separated zip list, dropping blanks and duplicates (first one
// wins, order kept) and validating each zip.
func parseZipList(raw string) ([]string, error) {
	seen := map[string]bool{}
	var zips []string
	for _, z := range strings.Split(raw, ",") {
		z = strings.TrimSpace(z)
		if z == "" || seen[z] {
			continue
		}
		if err := validateZip(z); err != nil {
			return nil, err
		}
		seen[z] = true
		zips = append(zips, z)
	}

	if len(zips) == 0 {
		return nil, errors.New("zips is required (comma-separated)")
	}
	if len(zips) > maxMultiZips {
		return nil, fmt.Errorf("at most %d distinct zip codes per request, got %d", maxMultiZips, len(zips))
	}
	return zips, nil
}

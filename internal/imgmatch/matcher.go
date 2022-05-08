package imgmatch

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/corona10/goimagehash"
	"github.com/sirupsen/logrus"
)

func NewMatcher(samplesDir string, interestingThreshold, suspiciousThreshold int) (*Matcher, error) {
	m := &Matcher{
		interestingSampleThreshold: interestingThreshold,
		suspiciousSampleThreshold:  suspiciousThreshold,
		samples:                    make(map[string]*goimagehash.ImageHash),
	}
	entries, err := os.ReadDir(samplesDir)
	if err != nil {
		return nil, fmt.Errorf("listing samples directory: %w", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") || entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(samplesDir, entry.Name())
		file, err := os.Open(fullPath)
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", fullPath, err)
		}

		img, err := jpeg.Decode(file)
		if err != nil {
			return nil, fmt.Errorf("decoding %s as jpeg: %w", fullPath, err)
		}
		if _, err := m.AddSample(entry.Name(), img); err != nil {
			return nil, fmt.Errorf("adding sample %s: %w", fullPath, err)
		}
	}
	logrus.Debugf("Loaded %v spam image samples", len(m.samples))
	return m, nil
}

type Matcher struct {
	interestingSampleThreshold int
	suspiciousSampleThreshold  int

	mu      sync.RWMutex
	samples map[string]*goimagehash.ImageHash
}

func (m *Matcher) AddSample(name string, img image.Image) (added bool, err error) {
	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return false, fmt.Errorf("calculating hash: %w", err)
	}
	logger := logrus.WithField("image_sample", name)
	logger.Debugf("Got new sample with hash %v", hash.ToString())

	m.mu.Lock()
	defer m.mu.Unlock()
	for otherName, otherHash := range m.samples {
		dist, err := hash.Distance(otherHash)
		if err != nil {
			return false, fmt.Errorf("calculating distance to hash %s: %w", otherName, err)
		}
		if dist <= m.interestingSampleThreshold {
			logger.Debugf("New sample matches %s with distance %d", otherName, dist)
			return false, nil
		}
	}
	m.samples[name] = hash
	return true, nil
}

func (m *Matcher) CheckSample(img image.Image) (match bool, err error) {
	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return false, fmt.Errorf("calculating hash: %w", err)
	}
	logger := logrus.WithField("sample_hash", hash.ToString())
	logger.Debugf("Checking sample")

	m.mu.RLock()
	defer m.mu.RUnlock()
	for otherName, otherHash := range m.samples {
		dist, err := hash.Distance(otherHash)
		if err != nil {
			return false, fmt.Errorf("calculating distance to hash %s: %w", otherName, err)
		}
		if dist <= m.suspiciousSampleThreshold {
			logger.Debugf("Sample matches %s with dist %v", otherName, dist)
			return true, nil
		}
	}
	return false, nil
}

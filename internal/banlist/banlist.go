package banlist

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

func New(path string) (*BanList, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer f.Close()
	scanner := bufio.NewScanner(f)

	l := BanList{}
	for scanner.Scan() {
		if pattern := scanner.Text(); pattern != "" {
			l.patterns = append(l.patterns, pattern)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading dictionary: %w", err)
	}
	logrus.Infof("Loaded %d banned patterns", len(l.patterns))
	return &l, nil
}

type BanList struct {
	patterns []string
}

func (l *BanList) Contains(s string) bool {
	for _, pattern := range l.patterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}

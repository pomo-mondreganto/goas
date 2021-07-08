package bot

import (
	"context"
	"fmt"
	"image/jpeg"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/corona10/goimagehash"
	"github.com/google/uuid"
)

const interestingSampleThreshold = 5

func (b *Bot) addSample(ctx context.Context, fileID string) error {
	filename := fmt.Sprintf("sample_%s.jpeg", uuid.New())
	dst := filepath.Join(b.samplesPath, filename)
	b.logger.Debugf("Saving new sample to %s", dst)
	file, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("opening sample file: %w", err)
	}
	interesting := true
	defer func() {
		if err := file.Close(); err != nil {
			b.logger.Errorf("Error closing sample file: %v", err)
		}
		if !interesting {
			if err := os.Remove(file.Name()); err != nil {
				b.logger.Errorf("Error removing sample: %v", err)
			}
		}
	}()

	img, err := b.downloadImg(ctx, fileID, file)
	if err != nil {
		return fmt.Errorf("downloading image: %w", err)
	}

	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return fmt.Errorf("calculating hash: %w", err)
	}

	b.logger.Debugf("Got sample %s with hash %s", filename, hash.ToString())
	for name, other := range b.spamSamples {
		dist, err := hash.Distance(other)
		if err != nil {
			b.logger.Errorf("Error calculating distance to sample %s: %v", name, err)
			continue
		}
		if dist <= interestingSampleThreshold {
			b.logger.Debugf("New sample matches %s with distance %d", name, dist)
			interesting = false
			break
		}
	}
	if interesting {
		b.logger.Debugf("Add interesting sample %s with hash %s", filename, hash.ToString())
		b.spamSamples[filename] = hash
	}

	return nil
}

func loadSamples(path string) (map[string]*goimagehash.ImageHash, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("listing directory: %w", err)
	}
	result := make(map[string]*goimagehash.ImageHash)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), ".") || f.IsDir() {
			continue
		}
		fname := filepath.Join(path, f.Name())
		file, err := os.Open(fname)
		if err != nil {
			return nil, fmt.Errorf("opening file %s: %w", fname, err)
		}

		img, err := jpeg.Decode(file)
		if err != nil {
			return nil, fmt.Errorf("decoding jpeg: %w", err)
		}

		hash, err := goimagehash.PerceptionHash(img)
		if err != nil {
			return nil, fmt.Errorf("calculating hash: %w", err)
		}
		result[f.Name()] = hash
	}
	return result, nil
}

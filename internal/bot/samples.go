package bot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

func (b *Bot) addImageSample(ctx context.Context, fileID string) error {
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
	if interesting, err = b.imgMatcher.AddSample(filename, img); err != nil {
		return fmt.Errorf("checking image: %w", err)
	}
	return nil
}

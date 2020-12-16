package bot

import (
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"net/http"
)

func (b *Bot) downloadImg(fileID string, saveTo io.Writer) (image.Image, error) {
	url, err := b.api.GetFileDirectURL(fileID)
	if err != nil {
		return nil, fmt.Errorf("getting file link: %w", err)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("downloaing image: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			b.logger.Errorf("Error closing response body: %v", err)
		}
	}()

	if saveTo != nil {
		resp.Body = ioutil.NopCloser(io.TeeReader(resp.Body, saveTo))
	}

	img, err := jpeg.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}
	return img, nil
}

package bot

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	voteSpamCallback    = "vote_spam"
	voteNotSpamCallback = "vote_not_spam"
)

func (b *Bot) downloadImg(ctx context.Context, fileID string, saveTo io.Writer) (image.Image, error) {
	url, err := b.api.GetFileDirectURL(fileID)
	if err != nil {
		return nil, fmt.Errorf("getting file link: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
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

func getSpamVoteMarkup() *tgbotapi.InlineKeyboardMarkup {
	return &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			{
				tgbotapi.InlineKeyboardButton{
					Text:         "Spam",
					CallbackData: &voteSpamCallback,
				},
				tgbotapi.InlineKeyboardButton{
					Text:         "Not spam",
					CallbackData: &voteNotSpamCallback,
				},
			},
		},
	}
}

func getSpamVoteMessage(msg *tgbotapi.Message, content string) tgbotapi.MessageConfig {
	m := tgbotapi.NewMessage(msg.Chat.ID, content)
	m.ParseMode = "markdown"
	m.ReplyToMessageID = msg.MessageID
	m.ReplyMarkup = getSpamVoteMarkup()
	return m
}

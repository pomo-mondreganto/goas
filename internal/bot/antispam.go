package bot

import (
	"fmt"
	"github.com/corona10/goimagehash"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	"image/jpeg"
	"net/http"
	"strings"
	"sync"
	"time"
)

type spamVerdict int

const (
	notSpam spamVerdict = iota
	mightBeSpam
	definitelySpam
)

const (
	trustAfterDays     = 30 * 6 // 6 months
	trustAfterMessages = 10

	suspiciousForwardMsgThreshold = 3
	suspiciousPhotoMsgThreshold   = 5

	imageDistanceThreshold = 24
)

var bannedStrings = []string{
	"/joinchat/",
}

var imageHashes = []uint64{
	10332248237452678527,
	13577964751561916522,
	13879425976505494127,
	13794120005494080568,
	10044012355472643967,
	13781154564668688256,
	12637545215187122845,
}

func (b *Bot) isChatMessageSuspicious(upd tgbotapi.Update) (spamVerdict, error) {
	authorID := upd.Message.From.ID
	chatID := upd.Message.Chat.ID

	logger := b.logger.WithField("authorID", authorID).WithField("chatID", chatID)
	logger.Debugf("Checking message for spam")

	if b.storage.IsUserAdmin(authorID) {
		logger.Debug("Admin, not suspicious")
		return notSpam, nil
	}
	trusted, err := b.storage.IsUserTrusted(authorID)
	if err != nil {
		return notSpam, fmt.Errorf("checking trusted: %w", err)
	}
	if trusted {
		logger.Debug("Trusted, not suspicious")
		return notSpam, nil
	}

	now := time.Now()
	firstSeen, err := b.storage.GetOrSetUserFirstSeen(authorID, now)
	if err != nil {
		return notSpam, fmt.Errorf("getting first seen: %w", err)
	}
	if firstSeen.Add(time.Hour * 24 * trustAfterDays).Before(now) {
		logger.Debugf("Joined at %v, not suspicious", firstSeen)
		return notSpam, nil
	}

	msgCount, err := b.storage.GetUserChatMessageCount(authorID, chatID)
	if err != nil {
		return notSpam, fmt.Errorf("getting message count: %w", err)
	}
	if msgCount > trustAfterMessages {
		logger.Debugf("Sent %d messages to chat, not suspicious", msgCount)
		return notSpam, nil
	}

	logger.Debugf("Message count: %d", msgCount)

	content := upd.Message.Text
	for _, s := range bannedStrings {
		if strings.Contains(content, s) {
			logger.Debugf("Contains banned string %s, suspicious", s)
			return mightBeSpam, nil
		}
	}

	logger.Debugf("Photos info: %v", upd.Message.Photo)
	if msgCount < suspiciousPhotoMsgThreshold && b.checkPhoto(logger, upd) {
		logger.Debugf("Photo matches spam sample")
		return definitelySpam, nil
	}

	logger.Debugf(
		"Forward info: from msg %d, user %v, chat %v at %v",
		upd.Message.ForwardFromMessageID,
		upd.Message.ForwardFrom,
		upd.Message.ForwardFromChat,
		upd.Message.ForwardDate,
	)
	if msgCount < suspiciousForwardMsgThreshold && b.checkForward(logger, upd) {
		logger.Debugf("Forward with only %d messages, suspicious", msgCount)
		return mightBeSpam, nil
	}

	logger.Debug("Checks passed, not suspicious")
	return notSpam, nil
}

func (b *Bot) checkForward(logger *logrus.Entry, upd tgbotapi.Update) bool {
	if upd.Message.ForwardDate == 0 {
		logger.Debug("Not a forward")
		return false
	}
	chatID := upd.Message.Chat.ID
	if upd.Message.ForwardFromChat != nil && upd.Message.ForwardFromChat.ID == chatID {
		logger.Debug("Same chat forward")
		return false
	}
	return true
}

func (b *Bot) checkPhoto(logger *logrus.Entry, upd tgbotapi.Update) bool {
	if upd.Message.Photo == nil {
		logrus.Debug("No photos")
		return false
	}

	result := atomic.Bool{}
	wg := sync.WaitGroup{}
	wg.Add(len(*upd.Message.Photo))
	for _, ps := range *upd.Message.Photo {
		go func(fileID string) {
			match, err := b.checkPhotoHashMatches(fileID, &wg, logger)
			if err != nil {
				logger.Errorf("Error checking photo hash: %v", err)
			}
			if match {
				result.Store(true)
			}
		}(ps.FileID)
	}
	wg.Wait()

	return result.Load()
}

func (b *Bot) checkPhotoHashMatches(fileID string, wg *sync.WaitGroup, logger *logrus.Entry) (bool, error) {
	defer wg.Done()
	url, err := b.api.GetFileDirectURL(fileID)
	if err != nil {
		return false, fmt.Errorf("getting file link: %w", err)
	}

	resp, err := http.Get(url)
	if err != nil {
		return false, fmt.Errorf("downloaing image: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Errorf("Error closing response body: %v", err)
		}
	}()

	img, err := jpeg.Decode(resp.Body)
	if err != nil {
		return false, fmt.Errorf("decoding image: %w", err)
	}

	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return false, fmt.Errorf("calculating hash: %w", err)
	}

	for i, other := range imageHashes {
		ih := goimagehash.NewImageHash(other, goimagehash.PHash)
		dist, err := hash.Distance(ih)
		if err != nil {
			logger.Errorf("Error calculating distance: %v", err)
			continue
		}
		logrus.Debugf("Distance to sample %d is %d", i, dist)
		if dist <= imageDistanceThreshold {
			logger.Infof("Image matches with sample %d with distance %d", i, dist)
			return true, nil
		}
	}
	return false, nil
}

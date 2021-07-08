package bot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/corona10/goimagehash"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
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

	votesToBan = 5
)

var bannedStrings = []string{
	"/joinchat/",
	"/bit.ly/",
}

func (b *Bot) isChatMessageSuspicious(ctx context.Context, upd tgbotapi.Update) (spamVerdict, error) {
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
	if msgCount < suspiciousPhotoMsgThreshold && b.checkPhoto(ctx, logger, upd) {
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

func (b *Bot) checkPhoto(ctx context.Context, logger *logrus.Entry, upd tgbotapi.Update) bool {
	if upd.Message.Photo == nil {
		logrus.Debug("No photos")
		return false
	}

	result := atomic.Bool{}
	wg := sync.WaitGroup{}
	wg.Add(len(upd.Message.Photo))
	for _, ps := range upd.Message.Photo {
		go func(fileID string) {
			match, err := b.checkPhotoHashMatches(ctx, fileID, &wg, logger)
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

func (b *Bot) checkPhotoHashMatches(ctx context.Context, fileID string, wg *sync.WaitGroup, logger *logrus.Entry) (bool, error) {
	defer wg.Done()

	img, err := b.downloadImg(ctx, fileID, nil)
	if err != nil {
		return false, fmt.Errorf("downloading img: %w", err)
	}

	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return false, fmt.Errorf("calculating hash: %w", err)
	}

	for name, other := range b.spamSamples {
		dist, err := hash.Distance(other)
		if err != nil {
			logger.Errorf("Error calculating distance: %v", err)
			continue
		}
		logrus.Debugf("Distance to sample %s is %d", name, dist)
		if dist <= imageDistanceThreshold {
			logger.Infof("Image matches with sample %s with distance %d", name, dist)
			return true, nil
		}
	}
	return false, nil
}

func (b *Bot) banSender(msg *tgbotapi.Message) {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	msgID := msg.MessageID

	if userID == b.api.Self.ID {
		logrus.Warning("Trying to ban me")
		return
	}

	if b.storage.IsUserAdmin(userID) {
		logrus.Warning("Trying to ban admin")
		return
	}

	b.logger.Infof("Deleting message %d from %d", msgID, userID)
	b.requestDelete(chatID, msgID)

	kickCfg := tgbotapi.KickChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userID,
		},
	}
	b.logger.Infof("Banning user %d", msg.From.ID)
	b.requestSend(kickCfg)
}

func (b *Bot) checkVotes(votesFor, votesAgainst int, userID int64, userVote bool) (finish bool, verdict bool) {
	if b.storage.IsUserAdmin(userID) {
		return true, userVote
	}
	if votesFor >= votesToBan || votesAgainst >= votesToBan {
		return true, votesFor > votesAgainst
	}
	return false, false
}

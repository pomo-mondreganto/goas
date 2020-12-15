package bot

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

const (
	trustAfterDays     = 30 * 6 // 6 months
	trustAfterMessages = 10

	suspiciousForwardMsgThreshold = 3
	suspiciousPhotoMsgThreshold   = 3
)

var bannedStrings = []string{
	"/joinchat/",
}

func (b *Bot) IsChatMessageSuspicious(upd tgbotapi.Update) (bool, error) {
	authorID := upd.Message.From.ID
	chatID := upd.Message.Chat.ID

	logger := b.logger.WithField("authorID", authorID).WithField("chatID", chatID)
	logger.Debugf("Checking message for spam")

	if b.storage.IsUserAdmin(authorID) {
		logger.Debug("Admin, not suspicious")
		return false, nil
	}
	trusted, err := b.storage.IsUserTrusted(authorID)
	if err != nil {
		return false, fmt.Errorf("checking trusted: %w", err)
	}
	if trusted {
		logger.Debug("Trusted, not suspicious")
		return false, nil
	}

	now := time.Now()
	firstSeen, err := b.storage.GetOrSetUserFirstSeen(authorID, now)
	if err != nil {
		return false, fmt.Errorf("getting first seen: %w", err)
	}
	if firstSeen.Add(time.Hour * 24 * trustAfterDays).Before(now) {
		logger.Debugf("Joined at %v, not suspicious", firstSeen)
		return false, nil
	}

	msgCount, err := b.storage.GetUserChatMessageCount(authorID, chatID)
	if err != nil {
		return false, fmt.Errorf("getting message count: %w", err)
	}
	if msgCount > trustAfterMessages {
		logger.Debugf("Sent %d messages to chat, not suspicious", msgCount)
		return false, nil
	}

	logger.Debugf("Message count: %d", msgCount)

	content := upd.Message.Text
	for _, s := range bannedStrings {
		if strings.Contains(content, s) {
			logger.Debugf("Contains banned string %s, suspicious", s)
			return true, nil
		}
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
		return true, nil
	}

	logger.Debugf("Photos info: %v", upd.Message.Photo)
	if msgCount < suspiciousPhotoMsgThreshold && upd.Message.Photo != nil {
		logger.Debugf("Photo with only %d messages, suspicious", msgCount)
		return true, nil
	}

	logger.Debug("Checks passed, not suspicious")
	return false, nil
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

package bot

import (
	"context"
	"fmt"
	"sync"
	"time"

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

	votesToBan = 3
)

func (b *Bot) isChatMessageSuspicious(ctx context.Context, msg *tgbotapi.Message) (spamVerdict, error) {
	authorID := msg.From.ID
	chatID := msg.Chat.ID

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

	// Check message text.
	if b.checkMessage(msg) {
		logger.Debug("Contains banned string, suspicious")
		return mightBeSpam, nil
	}

	logger.Debugf("Photos info: %v", msg.Photo)
	if msgCount < suspiciousPhotoMsgThreshold && b.checkPhoto(ctx, logger, msg) {
		logger.Debugf("Photo matches spam sample")
		return definitelySpam, nil
	}

	logger.Debugf(
		"Forward info: from msg %d, user %v, chat %v at %v",
		msg.ForwardFromMessageID,
		msg.ForwardFrom,
		msg.ForwardFromChat,
		msg.ForwardDate,
	)
	if msgCount < suspiciousForwardMsgThreshold && b.checkForward(logger, msg) {
		logger.Debugf("Forward with only %d messages, suspicious", msgCount)
		return mightBeSpam, nil
	}

	logger.Debug("Checks passed, not suspicious")
	return notSpam, nil
}

func (b *Bot) checkForward(logger *logrus.Entry, msg *tgbotapi.Message) bool {
	if msg.ForwardDate == 0 {
		logger.Debug("Not a forward")
		return false
	}
	chatID := msg.Chat.ID
	if msg.ForwardFromChat != nil && msg.ForwardFromChat.ID == chatID {
		logger.Debug("Same chat forward")
		return false
	}
	return true
}

func (b *Bot) checkPhoto(ctx context.Context, logger *logrus.Entry, msg *tgbotapi.Message) bool {
	if msg.Photo == nil {
		logger.Debug("No photos")
		return false
	}

	result := atomic.NewBool(false)
	wg := sync.WaitGroup{}
	wg.Add(len(msg.Photo))
	for _, ps := range msg.Photo {
		go func(fileID string) {
			defer wg.Done()
			match, err := b.checkPhotoHashMatches(ctx, fileID)
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

func (b *Bot) checkMessage(msg *tgbotapi.Message) bool {
	b.logger.Debugf("Checking message %v", msg.Text)
	return b.banlist.Contains(msg.Text)
}

func (b *Bot) checkPhotoHashMatches(ctx context.Context, fileID string) (bool, error) {
	img, err := b.downloadImg(ctx, fileID, nil)
	if err != nil {
		return false, fmt.Errorf("downloading image: %w", err)
	}
	suspicious, err := b.imgMatcher.CheckSample(img)
	if err != nil {
		return false, fmt.Errorf("checking image: %w", err)
	}
	return suspicious, nil
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

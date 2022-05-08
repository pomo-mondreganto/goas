package bot

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

func (b *Bot) processNewMembersMessage(msg *tgbotapi.Message) error {
	b.logger.Info("Processing new members message")
	for _, member := range msg.NewChatMembers {
		if _, err := b.storage.GetOrSetUserFirstSeen(member.ID, time.Now()); err != nil {
			return fmt.Errorf("setting user %d first seen: %w", member.ID, err)
		}
	}
	b.logger.Info("Deleting new members message")
	b.requestDelete(msg.Chat.ID, msg.MessageID)
	return nil
}

func (b *Bot) processMemberLeftMessage(msg *tgbotapi.Message) {
	b.logger.Info("Deleting member left message")
	b.requestDelete(msg.Chat.ID, msg.MessageID)
}

func (b *Bot) processChatMessage(ctx context.Context, msg *tgbotapi.Message) error {
	b.logger.Info("Processing chat message")

	if _, err := b.storage.GetOrSetUserFirstSeen(msg.From.ID, time.Now()); err != nil {
		return fmt.Errorf("getting user first seen: %w", err)
	}
	if _, err := b.storage.IncUserChatMessageCount(msg.From.ID, msg.Chat.ID, 1); err != nil {
		return fmt.Errorf("incrementing message count: %w", err)
	}

	if msg.IsCommand() {
		b.logger.Infof("User %d sent command %s", msg.From.ID, msg.Command())
		// Only admins can use commands.
		if b.storage.IsUserAdmin(msg.From.ID) {
			switch msg.Command() {
			case "trust":
				if err := b.processTrustCommand(msg); err != nil {
					return fmt.Errorf("processing trust command: %w", err)
				}
			case "ban":
				b.processBanCommand(msg)
			case "spam":
				if err := b.processSpamCommand(ctx, msg); err != nil {
					return fmt.Errorf("processing spam command: %w", err)
				}
			}
		}
		b.logger.Info("Deleting command message in public chat")
		b.requestDelete(msg.Chat.ID, msg.MessageID)
		return nil
	}

	verdict, err := b.isChatMessageSuspicious(ctx, msg)
	if err != nil {
		return fmt.Errorf("checking suspicious message: %w", err)
	}

	if verdict == mightBeSpam {
		b.processSuspiciousMessage(msg)
	} else if verdict == definitelySpam {
		b.processSpamMessage(msg)
	}

	return nil
}

func (b *Bot) processTrustCommand(msg *tgbotapi.Message) error {
	if msg.ReplyToMessage == nil {
		b.logger.Warning("Trust command called without reply")
		return nil
	}
	toTrust := msg.ReplyToMessage.From.ID
	if err := b.storage.TrustUser(toTrust); err != nil {
		return fmt.Errorf("trusting user: %w", err)
	}
	b.logger.Infof("Marked user %d as trusted", toTrust)
	return nil
}

func (b *Bot) processBanCommand(msg *tgbotapi.Message) {
	if msg.ReplyToMessage == nil {
		b.logger.Warning("Ban command called without reply")
		return
	}
	b.banSender(msg.ReplyToMessage)
}

func (b *Bot) processSpamCommand(ctx context.Context, msg *tgbotapi.Message) error {
	if msg.ReplyToMessage == nil {
		b.logger.Warning("Spam command called without reply")
		return nil
	}
	reply := msg.ReplyToMessage
	userID := reply.From.ID
	if userID == b.api.Self.ID {
		logrus.Warning("My messages are not spam")
		return nil
	}

	b.logger.Infof("Received spam message from %d", userID)
	for _, ps := range reply.Photo {
		if err := b.addImageSample(ctx, ps.FileID); err != nil {
			return fmt.Errorf("adding image sample: %w", err)
		}
	}
	b.banSender(reply)
	return nil
}

func (b *Bot) processSuspiciousMessage(msg *tgbotapi.Message) {
	m := getSpamVoteMessage(msg, "Is this message spam?")
	b.logger.Info("Sending suspicious message notification")
	b.requestSend(m)
}

func (b *Bot) processSpamMessage(msg *tgbotapi.Message) {
	m := getSpamVoteMessage(msg, "This message looks like spam. Is it?")
	m.ParseMode = "markdown"
	m.ReplyToMessageID = msg.MessageID
	b.logger.Info("Sending spam message notification")
	b.requestSend(m)
}

func (b *Bot) processCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) error {
	b.logger.Debugf("Received callback: %+v", callback)
	if callback.Message == nil {
		b.logger.Warning("Callback without message, skipping")
		return nil
	}
	if callback.Message.Chat == nil {
		b.logger.Warning("Callback not in chat, skipping")
		return nil
	}
	if callback.Message.ReplyToMessage == nil {
		b.logger.Warning("Callback message is not a reply, skipping")
		return nil
	}
	userID := callback.From.ID
	chatID := callback.Message.Chat.ID
	msgID := callback.Message.MessageID
	reply := callback.Message.ReplyToMessage
	b.logger.Debugf("User id: %d, Message ID: %d, reply: %#v", userID, msgID, reply)

	vote := callback.Data == voteSpamCallback

	if err := b.storage.VoteSpam(userID, chatID, msgID, vote); err != nil {
		return fmt.Errorf("voting: %w", err)
	}

	votesFor, votesAgainst, err := b.storage.GetVotes(chatID, msgID)
	if err != nil {
		return fmt.Errorf("getting votes: %w", err)
	}

	final, ban := b.checkVotes(votesFor, votesAgainst, userID, vote)
	b.logger.Debugf("Verdict for vote: final=%t ban=%t", final, ban)

	if final {
		defer b.requestDelete(chatID, msgID)

		if ban {
			defer b.banSender(reply)

			b.logger.Infof("Decided to ban user with %d for, %d against votes", votesFor, votesAgainst)
			if len(reply.Photo) > 0 {
				b.logger.Info("Adding photos as samples")
				for _, ps := range reply.Photo {
					if err := b.addImageSample(ctx, ps.FileID); err != nil {
						return fmt.Errorf("adding image sample: %w", err)
					}
				}
			}
		} else {
			b.logger.Infof("Decided not to ban user with %d for, %d against votes", votesFor, votesAgainst)
		}
	} else {
		edit := tgbotapi.EditMessageTextConfig{
			BaseEdit: tgbotapi.BaseEdit{
				ChatID:      chatID,
				MessageID:   msgID,
				ReplyMarkup: getSpamVoteMarkup(),
			},
			ParseMode: "markdown",
		}
		edit.Text = fmt.Sprintf("Is this spam?\n\nVotes `spam`: %d\n\nVotes `not spam`: %d", votesFor, votesAgainst)
		b.requestSend(edit)
	}

	return nil
}

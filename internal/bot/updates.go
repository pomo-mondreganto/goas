package bot

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/sirupsen/logrus"
	"time"
)

func (b *Bot) processNewMembersUpdate(upd tgbotapi.Update) error {
	b.logger.Info("Processing new members message")
	for _, member := range *upd.Message.NewChatMembers {
		if _, err := b.storage.GetOrSetUserFirstSeen(member.ID, time.Now()); err != nil {
			return fmt.Errorf("setting user %d first seen: %w", member.ID, err)
		}
	}
	b.logger.Info("Deleting new members message")
	b.requestDelete(upd.Message.Chat.ID, upd.Message.MessageID)
	return nil
}

func (b *Bot) processMemberLeftUpdate(upd tgbotapi.Update) error {
	b.logger.Info("Deleting member left message")
	b.requestDelete(upd.Message.Chat.ID, upd.Message.MessageID)
	return nil
}

func (b *Bot) processChatMessageUpdate(upd tgbotapi.Update) error {
	b.logger.Info("Processing chat message")

	msg := upd.Message
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
				if err := b.processBanCommand(msg); err != nil {
					return fmt.Errorf("processing ban command: %w", err)
				}
			case "spam":
				if err := b.processSpamCommand(msg); err != nil {
					return fmt.Errorf("processing spam command: %w", err)
				}
			}
		}
		b.logger.Info("Deleting command message in public chat")
		b.requestDelete(upd.Message.Chat.ID, upd.Message.MessageID)
		return nil
	}

	verdict, err := b.isChatMessageSuspicious(upd)
	if err != nil {
		return fmt.Errorf("checking suspicious message: %w", err)
	}

	if verdict == mightBeSpam {
		if err := b.processSuspiciousMessage(upd); err != nil {
			return fmt.Errorf("processing suspicious message: %w", err)
		}
	} else if verdict == definitelySpam {
		if err := b.processSpamMessage(upd); err != nil {
			return fmt.Errorf("processing spam message: %w", err)
		}
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

func (b *Bot) processBanCommand(msg *tgbotapi.Message) error {
	if msg.ReplyToMessage == nil {
		b.logger.Warning("Ban command called without reply")
		return nil
	}
	return b.banSender(msg.ReplyToMessage)
}

func (b *Bot) processSpamCommand(msg *tgbotapi.Message) error {
	if msg.ReplyToMessage == nil {
		b.logger.Warning("Spam command called without reply")
		return nil
	}
	reply := msg.ReplyToMessage
	userID := reply.From.ID
	if userID == b.api.Self.ID {
		logrus.Warningf("My messages are not spam")
		return nil
	}

	b.logger.Infof("Received spam message from %d", userID)
	if reply.Photo != nil {
		for _, ps := range *reply.Photo {
			if err := b.addSample(ps.FileID); err != nil {
				return fmt.Errorf("adding image sample: %w", err)
			}
		}
	}
	if err := b.banSender(reply); err != nil {
		return fmt.Errorf("banning user: %w", err)
	}
	return nil
}

func (b *Bot) processSuspiciousMessage(upd tgbotapi.Update) error {
	m := getSpamVoteMessage(upd.Message, "Is this message spam?")
	b.logger.Info("Sending suspicious message notification")
	b.requestSend(m)
	return nil
}

func (b *Bot) processSpamMessage(upd tgbotapi.Update) error {
	m := getSpamVoteMessage(upd.Message, "This message looks like spam. Is it?")
	m.ParseMode = "markdown"
	m.ReplyToMessageID = upd.Message.MessageID
	b.logger.Info("Sending spam message notification")
	b.requestSend(m)
	return nil
}

func (b *Bot) processCallback(upd tgbotapi.Update) error {
	cb := *upd.CallbackQuery
	b.logger.Debugf("Received callback: %+v", cb)
	if cb.Message == nil {
		b.logger.Warning("Callback without message, skipping")
	}
	if cb.Message.Chat == nil {
		b.logger.Warning("Callback not in chat, skipping")
	}
	if cb.Message.ReplyToMessage == nil {
		b.logger.Warning("Callback message is not a reply, skipping")
	}
	userID := cb.From.ID
	chatID := cb.Message.Chat.ID
	msgID := cb.Message.MessageID
	reply := cb.Message.ReplyToMessage
	b.logger.Debugf("User id: %d, Message ID: %d, reply: %#v", userID, msgID, reply)

	vote := cb.Data == voteSpamCallback

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
		if ban {
			b.logger.Infof("Decided to ban user with %d for, %d against votes", votesFor, votesAgainst)
			if reply.Photo != nil {
				b.logger.Info("Adding photos as samples")
				for _, ps := range *reply.Photo {
					if err := b.addSample(ps.FileID); err != nil {
						return fmt.Errorf("adding image sample: %w", err)
					}
				}
			}

			if err := b.banSender(reply); err != nil {
				return fmt.Errorf("banning sender: %w", err)
			}
		} else {
			b.logger.Infof("Decided not to ban user with %d for, %d against votes", votesFor, votesAgainst)
		}
		b.requestDelete(chatID, msgID)
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

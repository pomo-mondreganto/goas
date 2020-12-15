package bot

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/sirupsen/logrus"
	"time"
)

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
	userID := msg.ReplyToMessage.From.ID
	if userID == b.api.Self.ID {
		logrus.Warningf("Trying to ban me")
		return nil
	}

	b.logger.Infof("Received command to ban %d", userID)
	kickCfg := tgbotapi.KickChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: msg.Chat.ID,
			UserID: userID,
		},
		UntilDate: 0,
	}
	b.logger.Infof("Kicking user %d", userID)
	if _, err := b.api.KickChatMember(kickCfg); err != nil {
		return fmt.Errorf("kicking user: %w", err)
	}
	b.logger.Infof("Deleting message %d from %d", msg.ReplyToMessage.MessageID, userID)
	b.requestDelete(msg.Chat.ID, msg.ReplyToMessage.MessageID)
	return nil
}

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
		if msg.Command() == "trust" && b.storage.IsUserAdmin(msg.From.ID) {
			if err := b.processTrustCommand(msg); err != nil {
				return fmt.Errorf("processing trust command: %w", err)
			}
		}
		if msg.Command() == "trust" && b.storage.IsUserAdmin(msg.From.ID) {
			if err := b.processBanCommand(msg); err != nil {
				return fmt.Errorf("processing ban command: %w", err)
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

func (b *Bot) processSuspiciousMessage(upd tgbotapi.Update) error {
	m := tgbotapi.NewMessage(upd.Message.Chat.ID, "This might be spam")
	m.ParseMode = "markdown"
	m.ReplyToMessageID = upd.Message.MessageID
	b.logger.Info("Sending suspicious message notification")
	b.requestSend(m)
	return nil
}

func (b *Bot) processSpamMessage(upd tgbotapi.Update) error {
	m := tgbotapi.NewMessage(upd.Message.Chat.ID, "This is definitely spam")
	m.ParseMode = "markdown"
	m.ReplyToMessageID = upd.Message.MessageID
	b.logger.Info("Sending spam message notification")
	b.requestSend(m)
	return nil
}

package bot

import (
	"context"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/pomo-mondreganto/goas/internal/storage"
	"github.com/sirupsen/logrus"
	"sync"
)

func New(ctx context.Context, token string, debug bool, s *storage.Storage) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("creating bot api: %w", err)
	}
	if debug {
		api.Debug = true
	}
	logger := logrus.WithField("account", api.Self.UserName)
	logger.Infof("Authorized successfully")

	b := Bot{
		api:     api,
		toSend:  make(chan tgbotapi.Chattable, 100),
		updates: make(chan tgbotapi.Update, 100),
		logger:  logger,
		ctx:     ctx,
		storage: s,
	}

	b.setUpdatesPolling()
	b.wg.Add(1)
	go b.processEvents()

	return &b, nil
}

type Bot struct {
	api      *tgbotapi.BotAPI
	updates  chan tgbotapi.Update
	toSend   chan tgbotapi.Chattable
	toDelete chan tgbotapi.DeleteMessageConfig
	logger   *logrus.Entry
	ctx      context.Context
	wg       sync.WaitGroup
	storage  *storage.Storage
}

func (b *Bot) Wait() {
	b.wg.Wait()
	b.logger.Infof("Shutdown complete")
}

func (b *Bot) setUpdatesPolling() {
	uConf := tgbotapi.NewUpdate(0)
	uConf.Timeout = 60

	updatesChan, err := b.api.GetUpdatesChan(uConf)
	if err != nil {
		b.logger.Fatal("Error getting updated channel: ", err)
	}

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
	loop:
		for {
			select {
			case <-b.ctx.Done():
				break loop
			case upd := <-updatesChan:
				b.updates <- upd
			}
		}
	}()
}

func (b *Bot) processEvents() {
	defer b.wg.Done()

loop:
	for {
		b.logger.Debug("Waiting for events")
		select {
		case upd := <-b.updates:
			if upd.Message == nil || upd.Message.Chat == nil || upd.Message.Chat.IsPrivate() {
				break
			}

			b.logger.Info("Received an update: %v", upd)

			if upd.Message.NewChatMembers != nil {
				if err := b.processNewMembersUpdate(upd); err != nil {
					b.logger.Error("Error deleting message: ", err)
					break
				}
			}

			if err := b.processChatMessageUpdate(upd); err != nil {
				b.logger.Error("Error processing chat message: ", err)
				break
			}

		case m := <-b.toSend:
			b.logger.Debugf("Sending a message: %v", m)
			_, err := b.api.Send(m)
			if err != nil {
				b.logger.Error("Could not send message: ", err)
				break
			}

		case dmc := <-b.toDelete:
			b.logger.Debugf("Deleting a message: %v", dmc)
			_, err := b.api.DeleteMessage(dmc)
			if err != nil {
				b.logger.Error("Could not delete message: ", err)
				break
			}

		case <-b.ctx.Done():
			b.logger.Info("Context cancelled, exiting")
			break loop
		}
	}
}

func (b *Bot) requestSend(msg tgbotapi.Chattable) {
	b.toSend <- msg
}

func (b *Bot) requestDelete(chatID int64, messageID int) {
	b.toDelete <- tgbotapi.DeleteMessageConfig{ChatID: chatID, MessageID: messageID}
}
package bot

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/corona10/goimagehash"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/pomo-mondreganto/goas/internal/storage"
	"github.com/sirupsen/logrus"
)

func New(ctx context.Context, token string, debug bool, samplesPath string, s *storage.Storage) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("creating bot api: %w", err)
	}
	if debug {
		api.Debug = true
	}
	logger := logrus.WithField("account", api.Self.UserName)
	logger.Infof("Authorized successfully")

	samples, err := loadSamples(samplesPath)
	if err != nil {
		return nil, fmt.Errorf("reading spam samples: %w", err)
	}
	logger.Infof("Loaded %d spam samples", len(samples))

	b := Bot{
		api:         api,
		requests:    make(chan tgbotapi.Chattable, 100),
		updates:     make(chan tgbotapi.Update, 100),
		logger:      logger,
		storage:     s,
		spamSamples: samples,
		samplesPath: samplesPath,
	}

	b.wg.Add(2)
	go b.setUpdatesPolling(ctx)
	go b.processEvents(ctx)

	return &b, nil
}

type Bot struct {
	api         *tgbotapi.BotAPI
	updates     chan tgbotapi.Update
	requests    chan tgbotapi.Chattable
	logger      *logrus.Entry
	wg          sync.WaitGroup
	storage     *storage.Storage
	spamSamples map[string]*goimagehash.ImageHash
	samplesPath string
}

func (b *Bot) Wait() {
	b.wg.Wait()
	b.logger.Infof("Shutdown complete")
}

func (b *Bot) setUpdatesPolling(ctx context.Context) {
	defer b.wg.Done()

	uConf := tgbotapi.NewUpdate(0)
	uConf.Timeout = 60

	updatesChan := b.api.GetUpdatesChan(uConf)

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case upd := <-updatesChan:
			b.updates <- upd
		}
	}
}

func (b *Bot) processEvents(ctx context.Context) {
	defer b.wg.Done()

loop:
	for {
		b.logger.Debug("Waiting for events")
		select {
		case upd := <-b.updates:
			if upd.CallbackQuery != nil {
				if err := b.processCallback(ctx, upd); err != nil {
					b.logger.Errorf("Error processing callback: %v", err)
				}
				break
			}

			if upd.Message == nil || upd.Message.Chat == nil || upd.Message.Chat.IsPrivate() {
				break
			}

			b.logger.Infof("Received an update: %v", upd)

			if upd.Message.NewChatMembers != nil {
				if err := b.processNewMembersUpdate(upd); err != nil {
					b.logger.Errorf("Error processing new members: %v", err)
				}
				break
			}

			if upd.Message.LeftChatMember != nil {
				b.processMemberLeftUpdate(upd)
				break
			}

			if err := b.processChatMessageUpdate(ctx, upd); err != nil {
				b.logger.Errorf("Error processing chat message: %v", err)
				break
			}

		case m := <-b.requests:
			b.logger.Debugf("Received a request: %#v", m)

			var err error
			switch m.(type) {
			case tgbotapi.EditMessageTextConfig:
				if _, err = b.api.Send(m); err != nil && strings.Contains(err.Error(), "not modified") {
					err = nil
				}
			case tgbotapi.DeleteMessageConfig, tgbotapi.KickChatMemberConfig:
				_, err = b.api.Request(m)
			default:
				_, err = b.api.Send(m)
			}

			if err != nil {
				b.logger.Errorf("Error sending request: %v", err)
				break
			}

		case <-ctx.Done():
			b.logger.Info("Context cancelled, exiting")
			break loop
		}
	}
}

func (b *Bot) requestSend(msg tgbotapi.Chattable) {
	b.requests <- msg
}

func (b *Bot) requestDelete(chatID int64, messageID int) {
	b.requests <- tgbotapi.DeleteMessageConfig{ChatID: chatID, MessageID: messageID}
}

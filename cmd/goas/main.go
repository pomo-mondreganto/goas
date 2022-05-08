package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pomo-mondreganto/goas/internal/banlist"
	"github.com/pomo-mondreganto/goas/internal/bot"
	"github.com/pomo-mondreganto/goas/internal/config"
	"github.com/pomo-mondreganto/goas/internal/imgmatch"
	"github.com/pomo-mondreganto/goas/internal/storage"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	cfg := setupConfig()
	initLogger()
	setLogLevel(cfg)

	ctx, cancel := context.WithCancel(context.Background())

	s := createStorage(cfg)
	l := createDictionary(cfg)
	m := createImageMatcher(cfg)
	b := createBot(ctx, cfg, s, l, m)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-c

	cancel()

	b.Wait()

	logrus.Info("Shutdown successful")
}

func setupConfig() *config.Config {
	pflag.String("log_level", "INFO", "Log level {INFO|DEBUG|WARNING|ERROR}")
	pflag.StringP("data", "d", "data", "Data directory")
	pflag.StringP("samples", "s", "resources", "Spam samples directory")
	pflag.String("dictionary", "banlist.txt", "Path to banned patterns text file")
	pflag.Int("interesting_threshold", 10, "Threshold to consider new sample interesting")
	pflag.Int("suspicious_threshold", 15, "Threshold to consider an image suspicious")

	pflag.Parse()

	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		logrus.Fatalf("Error binding flags: %v", err)
	}

	viper.SetEnvPrefix("GOAS")
	viper.AutomaticEnv()
	return config.Get()
}

func initLogger() {
	mainFormatter := &logrus.TextFormatter{}
	mainFormatter.FullTimestamp = true
	mainFormatter.ForceColors = true
	mainFormatter.PadLevelText = true
	mainFormatter.TimestampFormat = "2006-01-02 15:04:05"
	logrus.SetFormatter(mainFormatter)
}

func setLogLevel(cfg *config.Config) {
	switch strings.ToUpper(cfg.LogLevel) {
	case "DEBUG":
		logrus.SetLevel(logrus.DebugLevel)
	case "INFO":
		logrus.SetLevel(logrus.InfoLevel)
	case "WARNING":
		logrus.SetLevel(logrus.WarnLevel)
	case "ERROR":
		viper.Set("debug", true)
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.Errorf("Invalid log level provided: %s", cfg.LogLevel)
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func createStorage(cfg *config.Config) *storage.Storage {
	s, err := storage.New(cfg.Data)
	if err != nil {
		logrus.Fatalf("Error creating storage: %v", err)
	}
	return s
}

func createBot(
	ctx context.Context,
	cfg *config.Config,
	s *storage.Storage,
	l *banlist.BanList,
	m *imgmatch.Matcher,
) *bot.Bot {
	b, err := bot.New(ctx, cfg.Token, cfg.Debug, s, l, m)
	if err != nil {
		logrus.Fatalf("Error creating bot: %v", err)
	}
	return b
}

func createDictionary(cfg *config.Config) *banlist.BanList {
	l, err := banlist.New(cfg.Dictionary)
	if err != nil {
		logrus.Fatalf("Error creating dictionary: %v", err)
	}
	return l
}

func createImageMatcher(cfg *config.Config) *imgmatch.Matcher {
	m, err := imgmatch.NewMatcher(cfg.Samples, cfg.InterestingThreshold, cfg.SuspiciousThreshold)
	if err != nil {
		logrus.Fatalf("Error creating image matcher: %v", err)
	}
	return m
}

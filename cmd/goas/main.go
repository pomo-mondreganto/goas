package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/pomo-mondreganto/goas/internal/banlist"
	"github.com/pomo-mondreganto/goas/internal/bot"
	"github.com/pomo-mondreganto/goas/internal/storage"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	setupConfig()
	initLogger()
	setLogLevel()

	ctx, cancel := context.WithCancel(context.Background())

	s := createStorage()
	l := createDictionary()
	b := createBot(ctx, s, l)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-c

	cancel()

	b.Wait()

	logrus.Info("Shutdown successful")
}

func setupConfig() {
	pflag.String("log_level", "INFO", "Log level {INFO|DEBUG|WARNING|ERROR}")
	pflag.StringP("data_dir", "d", "data", "Data directory")
	pflag.StringP("samples_dir", "s", "resources", "Spam samples directory")
	pflag.String("dictionary", "banlist.txt", "Path to banned patterns text file")

	pflag.Parse()

	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		logrus.Fatalf("Error binding flags: %v", err)
	}

	viper.SetEnvPrefix("GOAS")
	viper.AutomaticEnv()
}

func initLogger() {
	mainFormatter := &logrus.TextFormatter{}
	mainFormatter.FullTimestamp = true
	mainFormatter.ForceColors = true
	mainFormatter.PadLevelText = true
	mainFormatter.TimestampFormat = "2006-01-02 15:04:05"
	logrus.SetFormatter(mainFormatter)
}

func setLogLevel() {
	ll := viper.GetString("log_level")
	switch ll {
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
		logrus.Errorf("Invalid log level provided: %s", ll)
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func createStorage() *storage.Storage {
	s, err := storage.New(viper.GetString("data_dir"))
	if err != nil {
		logrus.Fatalf("Error creating storage: %v", err)
	}
	return s
}

func createBot(ctx context.Context, s *storage.Storage, l *banlist.BanList) *bot.Bot {
	token := viper.GetString("token")
	debug := viper.GetBool("debug")
	samples := viper.GetString("samples_dir")
	b, err := bot.New(ctx, token, debug, samples, s, l)
	if err != nil {
		logrus.Fatalf("Error creating bot: %v", err)
	}
	return b
}

func createDictionary() *banlist.BanList {
	l, err := banlist.New(viper.GetString("dictionary"))
	if err != nil {
		logrus.Fatalf("Error creating dictionary: %v", err)
	}
	return l
}

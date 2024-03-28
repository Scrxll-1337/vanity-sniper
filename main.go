package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
)

var (
	guilds             = map[string]string{}
	clients            []func() error
	guildsIndex        = 0
	sameGuildIntervals = map[string]*time.Time{}
)

var logger = log.NewWithOptions(os.Stderr, log.Options{
	ReportCaller:    false,
	ReportTimestamp: true,
	TimeFormat:      time.TimeOnly,
	Level:           log.DebugLevel,
	Prefix:          "Vanity Sniper",
})

func main() {
	initializeConfig()

	for _, token := range config.Tokens {
		if token != "" {
			disconnect := createClient(token)
			clients = append(clients, disconnect)
		}
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	exit()
}

func exit() {
	if len(clients) > 0 {
		logger.Infof("Exiting. Terminating %v clients.", len(clients))
		for _, disconnect := range clients {
			disconnect()
		}
	}

	os.Exit(0)
}

func If[T any](cond bool, vtrue, vfalse T) T {
	if cond {
		return vtrue
	}

	return vfalse
}

func strip(content string, length int) string {
	if len(content) < length {
		return content
	}

	return content[0:length] + strings.Repeat("x", len(content)-length)
}

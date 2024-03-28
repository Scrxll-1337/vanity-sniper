package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/bwmarrin/discordgo"
)

func createClient(token string) func() error {
	logger.Infof("Logging in with token: %v", strip(token, 30))

	dg, err := discordgo.New(token)

	if err != nil {
		logger.Errorf("Failed to log in with %v: %v", strip(token, 30), err)
		return func() error { return nil }
	}

	dg.AddHandler(ready)
	dg.AddHandler(guildUpdate)
	dg.AddHandler(guildCreate)
	dg.AddHandler(guildDelete)

	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentDirectMessages

	err = dg.Open()

	if err != nil {
		logger.Errorf("Failed to open connection: %v", err)
		return func() error { return nil }
	}

	return func() error {
		return dg.Close()
	}
}

func ready(s *discordgo.Session, event *discordgo.Ready) {
	logger.Infof("Logged in as %v with %v guilds.", event.User.Username, len(event.Guilds))

	for _, guild := range event.Guilds {
		if guild.VanityURLCode != "" {
			guilds[guild.ID] = guild.VanityURLCode
		}
	}
}

func guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	if event.Guild.VanityURLCode != "" {
		logger.Infof("Queued %v for sniping. (Vanity: %v)", event.Guild.Name, event.VanityURLCode)
		guilds[event.Guild.ID] = event.Guild.VanityURLCode
	}
}

func guildUpdate(s *discordgo.Session, event *discordgo.GuildUpdate) {
	if event.Guild.Unavailable {
		return
	}

	if config.IgnoreHostGuilds && slices.Contains(config.Guilds, event.ID) {
		return
	}

	if event.VanityURLCode != guilds[event.ID] {
		_, exists := guilds[event.ID]

		if guilds[event.ID] == "" {
			return
		}

		logger.Infof("Vanity URL changed: %v -> %v", If(exists, guilds[event.ID], "None"), If(event.VanityURLCode != "", event.VanityURLCode, "None"))

		if config.SameGuildTimeout != 0 {
			interval, existing := sameGuildIntervals[config.Guilds[guildsIndex]]

			if existing {
				difference := time.Until(*interval)

				if difference > 0 {
					logger.Warnf("Guild %v is on timeout for %.2fs. Ignoring vanity change.", config.Guilds[guildsIndex], difference.Seconds())
					return
				} else {
					delete(sameGuildIntervals, config.Guilds[guildsIndex])
				}
			}
		}

		snipe(guilds[event.ID], s.Token, 0)

		guilds[event.ID] = event.VanityURLCode
	}
}

func guildDelete(s *discordgo.Session, event *discordgo.GuildDelete) {
	if event.VanityURLCode == "" {
		return
	}

	if config.IgnoreHostGuilds && slices.Contains(config.Guilds, event.ID) {
		return
	}

	logger.Infof("Guild %v was deleted. The vanity may be free: %v", event.BeforeDelete.Name, event.VanityURLCode)
	if config.SameGuildTimeout != 0 {
		interval, existing := sameGuildIntervals[config.Guilds[guildsIndex]]

		if existing {
			difference := time.Until(*interval)

			if difference > 0 {
				logger.Warnf("Guild %v is on timeout for %.2fs. Ignoring guild deletion.", config.Guilds[guildsIndex], difference.Seconds())
				return
			} else {
				delete(sameGuildIntervals, config.Guilds[guildsIndex])
			}
		}
	}

	snipe(event.VanityURLCode, s.Token, 0)
}

type RatelimitedResponse struct {
	RetryAfter float64 `json:"retry_after,omitempty"`
	Message    string  `json:"message,omitempty"`
	Code       string  `json:"code,omitempty"`
}

type CodeResponse struct {
	Uses int    `json:"uses,omitempty"`
	Code string `json:"code,omitempty"`
}

type FailedResponse struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func snipe(vanity string, token string, tries int) {
	logger.Infof("Attempting to snipe vanity: %v", vanity)

	payload, err := json.Marshal(map[string]string{"code": vanity})

	if err != nil {
		logger.Fatalf("Failed to marshall code: %v", err)
	}

	client := &http.Client{}
	guild := config.Guilds[guildsIndex]
	url := fmt.Sprintf("https://discord.com/api/v%v/guilds/%v/vanity-url", config.APIVersion, guild)
	request, err := http.NewRequest("PATCH", url, bytes.NewBuffer(payload))

	if err != nil {
		logger.Errorf("Failed to make request: %v", err)
		return
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", token)
	request.Header.Set("X-Super-Properties", superProperties)
	request.Header.Set("User-Agent", config.Properties.UserAgent)

	start := time.Now()
	res, err := client.Do(request)
	elapsed := time.Since(start)

	if err != nil {
		logger.Errorf("Failed to complete request: %v", err)
		return
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		logger.Errorf("Failed to decode body: %v", err)
		return
	}

	if res.StatusCode == 429 {
		jsonBody := RatelimitedResponse{}
		err := json.Unmarshal([]byte(body), &jsonBody)

		if err != nil {
			logger.Errorf("Failed to unmarshall body: %v", err)
		}

		ratelimit := time.Duration(jsonBody.RetryAfter) * time.Second

		logger.Warnf("Ratelimited for %v while trying to snipe vanity: %v (Time: %.2fs)", ratelimit, vanity, elapsed.Seconds())
		time.Sleep(ratelimit)

		if config.Retries > tries {
			tries += 1
			logger.Infof("Retrying snipe for vanity: %v (Retry: #%v)", vanity, tries)
			snipe(vanity, token, tries)
			return
		} else {
			logger.Infof("Failed sniping %v after %v attempts.", vanity, config.Retries)
		}

		return
	}

	if res.StatusCode == 400 {
		jsonBody := FailedResponse{}
		err := json.Unmarshal([]byte(body), &jsonBody)

		if err != nil {
			logger.Errorf("Failed to unmarshall body: %v", err)
		}

		logger.Warnf("Failed to snipe vanity: %v (Reason: %v, Time: %.2fs)", vanity, jsonBody.Message, elapsed.Seconds())
		return
	}

	if res.StatusCode == 200 {
		jsonBody := CodeResponse{}
		err := json.Unmarshal([]byte(body), &jsonBody)

		if err != nil {
			logger.Errorf("Failed to unmarshall body: %v", err)
		}

		logger.Infof("Successfully sniped vanity: %v to guild %v (%.2fs)", vanity, guild, elapsed.Seconds())
		guildsIndex += 1

		sendToWebhook(fmt.Sprintf("Successfully sniped vanity: %v to guild %v (%.2fs)", vanity, guild, elapsed.Seconds()))

		if config.SameGuildTimeout != 0 {
			date := time.Now().Add(time.Duration(config.SameGuildTimeout) * time.Millisecond)
			sameGuildIntervals[guild] = &date
		}

		if guildsIndex >= (len(config.Guilds) - 1) {
			if config.RotateGuilds {
				logger.Warnf("Used up all available guilds for vanity sniping. As rotate guilds is turned on, we will re-use them in order.")
				guildsIndex = 0
			} else {
				logger.Warnf("Ran out of guilds to use. as config.rotateGuilds is turned off, the process will now exit.")
				exit()
			}
		}
	}

	logger.Warnf("Got unknown response code. (Status: %v, Body: %v)", res.StatusCode, string(body))
}

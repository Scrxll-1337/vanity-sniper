package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Config struct {
	Debug            bool     `json:"debug"`
	Tokens           []string `json:"tokens"`
	Webhook          string   `json:"webhook"`
	Retries          int      `json:"retries"`
	APIVersion       string   `json:"apiVersion"`
	RotateGuilds     bool     `json:"rotateGuilds"`
	IgnoreHostGuilds bool     `json:"ignoreConfiguredGuilds"`
	SameGuildTimeout int      `json:"sameGuildTimeout"`
	Guilds           []string `json:"guilds"`
	Properties       struct {
		OS                     string `json:"os"`
		Browser                string `json:"browser"`
		Device                 string `json:"device"`
		SystemLocale           string `json:"system_locale"`
		UserAgent              string `json:"browser_user_agent"`
		BrowserVersion         string `json:"browser_version"`
		OSVersion              string `json:"os_version"`
		Referrer               string `json:"referrer"`
		ReferringDomain        string `json:"referring_domain"`
		ReferrerCurrent        string `json:"referrer_current"`
		ReferringDomainCurrent string `json:"referring_domain_current"`
		ReleaseChannel         string `json:"release_channel"`
	} `json:"properties"`
}

var (
	config            Config
	superProperties   string
	clientBuildNumber *string
	assetsRegex       = regexp.MustCompile("/assets/[0-9]{1,5}.*?.js")
)

const BUILD_NUMBER_LENGTH = 6

func initializeConfig() {
	executable, err := os.Executable()

	if err != nil {
		logger.Fatalf("Failed to get current directory: %v", err)
	}

	dir := path.Dir(executable)
	file := path.Join(dir, "config.json")

	if present, _ := exists(file); !present {
		logger.Fatalf("Configuration file config.json does not exist in current directory.")
	}

	content, err := os.ReadFile(file)

	if err != nil {
		logger.Fatalf("Failed to read configuration file: %v", err)
	}

	err = json.Unmarshal([]byte(content), &config)

	if err != nil {
		logger.Fatalf("Failed to decode configuration file: %v", err)
	}

	getLatestBuild()

	propertiesInterface := map[string]interface{}{
		"os":                       config.Properties.OS,
		"browser":                  config.Properties.Browser,
		"device":                   config.Properties.Device,
		"system_locale":            config.Properties.SystemLocale,
		"browser_user_agent":       config.Properties.UserAgent,
		"browser_version":          config.Properties.BrowserVersion,
		"os_version":               config.Properties.OSVersion,
		"referrer":                 config.Properties.Referrer,
		"referring_domain":         config.Properties.ReferringDomain,
		"referrer_current":         config.Properties.ReferrerCurrent,
		"referring_domain_current": config.Properties.ReferringDomainCurrent,
		"release_channel":          config.Properties.ReleaseChannel,
		"client_build_number":      clientBuildNumber,
		"client_event_source":      nil,
	}

	res, err := json.Marshal(propertiesInterface)

	if err != nil {
		logger.Fatalf("Failed to marshall super properties: %v", err)
	}

	superProperties = base64.StdEncoding.EncodeToString(res)

	logger.Infof("Configuration loaded from file: %v", file)
}

func getLatestBuild() {
	logger.Infof("Getting latest client build number to avoid account suspensions...")

	client := &http.Client{}
	request, err := http.NewRequest("GET", "https://discord.com/app", nil)

	if err != nil {
		logger.Errorf("Failed to make request: %v", err)
		return
	}

	request.Header.Set("User-Agent", config.Properties.UserAgent)

	response, err := client.Do(request)

	if err != nil {
		logger.Errorf("Failed to complete request while getting latest build number: %v", err)
		return
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)

	if err != nil {
		logger.Errorf("Failed to read body while getting latest build number: %v", err)
		return
	}

	assets := assetsRegex.FindAll(body, -1)

	// Build number is usually in the last few assets
	slices.Reverse(assets)

	for _, asset := range assets {
		url := fmt.Sprintf("https://discord.com%v", string(asset))

		request, err := http.NewRequest("GET", url, nil)

		if err != nil {
			logger.Errorf("Failed to make request for %v: %v", asset, err)
			continue
		}

		response, err := client.Do(request)

		if err != nil {
			logger.Errorf("Failed to complete request while fetching asset: %v", err)
			return
		}

		defer response.Body.Close()

		bodyBytes, err := io.ReadAll(response.Body)

		if err != nil {
			logger.Errorf("Failed to read body while getting latest build number: %v", err)
			return
		}

		lookup := "build_number:\""
		body := string(bodyBytes)
		idx := strings.Index(body, lookup)

		if idx != -1 {
			start := idx + len(lookup)
			end := start + BUILD_NUMBER_LENGTH

			buildNumber := body[start:end]

			// Check if build number is acutally a number
			_, err := strconv.Atoi(buildNumber)

			if err != nil {
				logger.Errorf("Failed to convert build number %v to integer: %v", buildNumber, err)
				continue
			}

			clientBuildNumber = &buildNumber
			logger.Infof("Got latest build number: %v", buildNumber)
			break
		}
	}

	if clientBuildNumber == nil {
		logger.Infof("Failed to get the latest client build number. This is not safe at all and can lead to account suspensions.")
		exit()
	}
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)

	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

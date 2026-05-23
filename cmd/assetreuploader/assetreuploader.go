package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/Obey72/backend/internal/app/config"
	"github.com/Obey72/backend/internal/color"
	"github.com/Obey72/backend/internal/files"
	"github.com/Obey72/backend/internal/roblox"
)

// watch the parent reup app process and exit the instant it goes away
// tauri passes its own pid via the reup_parent_pid env var when spawning us
// previously we polled every 1s with a 5s grace, the grace caused orphaned
// backends to linger which the user explicitly does not want, drop both
func startParentWatch() {
	pidStr := os.Getenv("REUP_PARENT_PID")
	if pidStr == "" {
		return
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return
	}
	go func() {
		// windows path uses a blocking wait on the parent handle so we wake
		// the moment the os signals it, unix has no equivalent for an arbitrary
		// pid so we tight-poll every 200ms
		if !waitForProcessExit(pid) {
			return
		}
		fmt.Println("parent reup app is gone shutting down")
		os.Exit(0)
	}()
}

var (
	cookieFile = config.Get("cookie_file")
	port       = config.Get("port")
)

func main() {
	cookie, _ := files.Read(cookieFile)
	roblosCookie, legacyAPIKey := parseCookieFile(cookie)
	roblosCookie = strings.TrimSpace(roblosCookie)
	legacyAPIKey = strings.TrimSpace(legacyAPIKey)
	if legacyAPIKey != "" && strings.TrimSpace(config.Get("api_key")) == "" {
		config.Set("api_key", legacyAPIKey)
		if err := config.PersistAPIKey(); err != nil {
			color.Error.Println("Failed to save API key: ", err)
		}
	}

	// start with whatever cookie we have  electron will push a valid one via file before spawning
	c, _ := roblox.NewClient(roblosCookie)

	startParentWatch()

	fmt.Println("localhost started on port " + port + ". Waiting to start reuploading.")
	if err := serve(c); err != nil {
		log.Fatal(err)
	}
}

func parseCookieFile(content string) (roblosCookie, apiKey string) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "api-key:") {
			apiKey = strings.TrimSpace(strings.TrimPrefix(line, "api-key:"))
		} else if line != "" {
			roblosCookie = line
		}
	}
	return roblosCookie, apiKey
}

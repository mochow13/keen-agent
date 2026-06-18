package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const requestTimeout = 3 * time.Second

type release struct {
	TagName string `json:"tag_name"`
}

func CheckLatest(ctx context.Context, currentVersion, owner, repo string) (string, bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	return checkURL(ctx, http.DefaultClient, currentVersion, url)
}

func checkURL(ctx context.Context, client *http.Client, currentVersion, url string) (string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", false, err
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	return latest, isNewer(current, latest), nil
}

func isNewer(current, latest string) bool {
	cParts := strings.Split(current, ".")
	lParts := strings.Split(latest, ".")

	maxLen := len(cParts)
	if len(lParts) > maxLen {
		maxLen = len(lParts)
	}

	for i := 0; i < maxLen; i++ {
		var cVal, lVal int
		if i < len(cParts) {
			cVal, _ = strconv.Atoi(cParts[i])
		}
		if i < len(lParts) {
			lVal, _ = strconv.Atoi(lParts[i])
		}
		if lVal > cVal {
			return true
		}
		if lVal < cVal {
			return false
		}
	}
	return false
}

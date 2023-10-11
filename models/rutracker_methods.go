package models

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

var RTUser *TrackerUser

var (
	rtBaseURL  = "https://rutracker.org"
	rtLoginURL = "https://rutracker.org/forum/login.php"
)

// RTLogin is a method for logging in to the tracker rutracker.org
func (t *TrackerUser) RTLogin(loginUrl, userAgent, baseURL string) error {
	jar, _ := cookiejar.New(nil)
	t.Client = &http.Client{
		Jar: jar,
		// Set timeout for http client to 10 seconds
		Timeout: 100 * time.Second,
		// Check redirect
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// If redirect to login page
			if strings.Contains(req.URL.String(), "login.php") {
				// Return error
				return fmt.Errorf("redirect to login page")
			}
			// Else return nil
			return nil
		},
	}

	data := url.Values{
		"login_username": {t.Username},
		"login_password": {t.Password},
		"login":          {"Вход"},
	}

	req, err := http.NewRequest("POST", loginUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)

	resp, err := t.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	return nil
}

func init() {
	RTUser = &TrackerUser{
		Username: globalConfig.RtUsername,
		Password: globalConfig.RtPassword,
	}
	err := RTUser.RTLogin(rtLoginURL, userAgent, rtBaseURL)
	if err != nil {
		log.Error("rutracker_init", "Error while logging in", map[string]string{"error": err.Error()})
		// If error is not nil, then exit
		os.Exit(1)
	}
	log.Info("rutracker_init", "Rutracker user logged in", nil)

}

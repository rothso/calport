package main

import (
	"context"
	"log"
	"github.com/chromedp/chromedp"
	"time"
	"fmt"
	"os"
	"io/ioutil"
	"path/filepath"
	"bufio"
	"golang.org/x/crypto/ssh/terminal"
	"syscall"
	"strings"
)

func main() {
	// Download the schedule page if it hasn't been downloaded yet
	cacheDir := "./.cache"
	file := filepath.Join(cacheDir, "schedule.html");
	if _, err := os.Stat(file); os.IsNotExist(err) {
		username, password := readCredentials()
		html, err := getRawSchedule(username, password)
		if err != nil {
			if err.Error() == "timeout waiting for initial target" {
				log.Fatal("ChromeDriver timed out while waiting for Chrome to open. Please try again.")
			} else {
				log.Fatal(err)
			}
		}

		log.Printf("Success! Saving raw schedule to %s\n", file)
		os.Mkdir(cacheDir, os.ModePerm)
		ioutil.WriteFile(file, []byte(html), 0644)
	} else {
		log.Println("Cached schedule found! Nothing to download")
	}

	// TODO: parse results, integrate with Google Calendar
}

func readCredentials() (string, string) {
	// Read username
	fmt.Print("Username for 'mywings.unf.edu': ")
	username, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	username = strings.TrimSpace(username)

	// Read password (masked)
	fmt.Printf("Password for '%s': ", username)
	passwordBytes, _ := terminal.ReadPassword(syscall.Stdin)
	password := strings.TrimSpace(string(passwordBytes))

	fmt.Print("\n")
	return username, password
}

func getRawSchedule(username, password string) (string, error) {
	var html string

	// Create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create chrome instance
	log.Println("Launching Chrome...")
	suppressErrors := chromedp.WithErrorf(func(string, ...interface{}) {})
	c, err := chromedp.New(ctxt, suppressErrors)
	if err != nil {
		return "", err
	}

	// Log in and get the schedule for the current term
	// TODO: split into login & scrapeSchedule to handle login failures
	log.Println("Logging into myWings...")
	err = c.Run(ctxt, scrapeSchedule(username, password, &html))
	if err != nil {
		return "", err
	}

	// Shutdown chrome
	err = c.Shutdown(ctxt)
	if err != nil {
		return "", err
	}

	// Wait for chrome to finish
	err = c.Wait()
	if err != nil {
		return "", err
	}

	return html, nil
}

func scrapeSchedule(username, password string, res *string) chromedp.Tasks {
	loginUrl := "https://mywings.unf.edu/"
	schedUrl := "http://mywings2.unf.edu/cp/ip/login?sys=sctssb" +
		"&url=https://banner.unf.edu/pls/nfpo/bwskfshd.P_CrseSchdDetl"

	loginInput := fmt.Sprintf("%s\t%s\n", username, password);
	return chromedp.Tasks{
		chromedp.Navigate(loginUrl),
		chromedp.Sleep(2 * time.Second), // if we log in too fast, it fails
		chromedp.SendKeys(`#userID`, loginInput, chromedp.ByID),
		chromedp.WaitVisible(`#alertAdminMessageDiv`, chromedp.ByID),
		chromedp.Navigate(schedUrl),
		chromedp.WaitVisible(`form`, chromedp.ByQuery),
		chromedp.Click(`input[type="submit"]`), // fixme: sometimes stuck on this page
		chromedp.WaitNotPresent(`select`, chromedp.ByQuery),
		chromedp.WaitVisible(`.pagebodydiv`, chromedp.ByQuery),
		chromedp.InnerHTML(`.pagebodydiv`, res, chromedp.ByQuery), // TODO: store term name as well
	}
}

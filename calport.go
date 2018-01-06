package main

import (
	"context"
	"log"
	"github.com/chromedp/chromedp"
	"time"
	"fmt"
)

func main() {
	var username, password string // TODO: read from CLI
	var err error

	// Create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create chrome instance
	c, err := chromedp.New(ctxt, chromedp.WithDebugf(log.Printf))
	if err != nil {
		log.Fatal(err)
	}

	// Log in and get the schedule for the current term
	var res string
	err = c.Run(ctxt, getSchedule(username, password, &res))
	if err != nil {
		log.Fatal(err)
	}

	// Shutdown chrome
	err = c.Shutdown(ctxt)
	if err != nil {
		log.Fatal(err)
	}

	// Wait for chrome to finish
	err = c.Wait()
	if err != nil {
		log.Fatal(err)
	}

	// TODO: parse results, integrate with Google Calendar, add caching/lockfile
}

func getSchedule(username, password string, res *string) chromedp.Tasks {
	loginUrl := "https://mywings.unf.edu/"
	schedUrl := "http://mywings2.unf.edu/cp/ip/login?sys=sctssb" +
		"&url=https://banner.unf.edu/pls/nfpo/bwskfshd.P_CrseSchdDetl"

	loginInput := fmt.Sprintf("%s\t%s\n", username, password);
	return chromedp.Tasks{
		chromedp.Navigate(loginUrl),
		chromedp.Sleep(1 * time.Second), // if we log in too fast, it fails
		chromedp.SendKeys(`#userID`, loginInput, chromedp.ByID),
		chromedp.WaitVisible(`#alertAdminMessageDiv`, chromedp.ByID),
		chromedp.Navigate(schedUrl),
		chromedp.WaitVisible(`form`, chromedp.ByQuery),
		chromedp.Click(`input[type="submit"]`), // fixme: sometimes stuck on this page
		chromedp.WaitNotPresent(`select`, chromedp.ByQuery),
		chromedp.WaitVisible(`.pagebodydiv`, chromedp.ByQuery),
		chromedp.InnerHTML(`.pagebodydiv`, res, chromedp.ByQuery),
	}
}

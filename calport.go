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
	"golang.org/x/crypto/ssh/terminal"
	"syscall"
	"strings"
	"github.com/PuerkitoBio/goquery"
	"regexp"
	"github.com/k0kubun/pp"
)

type Course struct {
	Code       string
	Name       string
	Instructor string
	Location   string
	Days       []byte
	DateStart  time.Time
	DateEnd    time.Time
	TimeStart  time.Time
	TimeEnd    time.Time
}

type Schedule []Course

var cacheDir = "./.cache"

func main() {
	username := os.Args[1] // myWings username

	// Download and parse schedule
	document, err := downloadSchedule(username, readPassword)
	if err != nil {
		log.Fatal(err)
	}
	schedule := parseSchedule(document)
	log.Printf("Found %d courses, dumping:\n", len(schedule))
	pp.Println(schedule)

	// TODO: integrate with Google Calendar
}

func downloadSchedule(username string, getPassword func(string) string) (*goquery.Document, error) {
	// Check if there's a cached copy first
	filename := filepath.Join(cacheDir, username+".html")
	if file, err := os.Open(filename); err == nil {
		return goquery.NewDocumentFromReader(file) // TODO: bust cache for new terms
	}
	// Fetch schedule using ChromeDriver
	password := getPassword(username)
	raw, err := fetchSchedule(username, password)
	for err != nil && err.Error() == "timeout waiting for initial target" {
		log.Println("Failed to launch ChromeDriver before timeout, retrying...")
		raw, err = fetchSchedule(username, password)
	}
	// Cache for later
	os.Mkdir(cacheDir, os.ModePerm)
	ioutil.WriteFile(filename, []byte(raw), 0644)
	return goquery.NewDocumentFromReader(strings.NewReader(raw))
}

func readPassword(username string) string {
	// Read password (masked)
	fmt.Printf("Password for '%s': ", username)
	passwordBytes, _ := terminal.ReadPassword(syscall.Stdin)
	password := strings.TrimSpace(string(passwordBytes))

	fmt.Print("\n")
	return password
}

func fetchSchedule(username, password string) (string, error) {
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

	loginInput := fmt.Sprintf("%s\t%s\n", username, password)
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

func parseSchedule(doc *goquery.Document) Schedule {
	courses := make([]Course, 0)

	detailsTables := doc.Find(`.datadisplaytable[summary*="course detail"]`)
	meetingTables := doc.Find(`.datadisplaytable[summary*="meeting times"]`)

	// Iterate over each listed course
	meetingTables.Each(func(i int, meeting *goquery.Selection) {
		details := detailsTables.Eq(i) // corresponding details table
		data := meeting.Find("td")

		// Course code and name
		header := strings.Split(details.Find("caption").First().Text(), " - ")
		code := header[1]
		name := header[0]

		// Instructor's full name
		instructorR := regexp.MustCompile(`([\s\w-]+) \(P\)`)
		instructor := instructorR.FindStringSubmatch(data.Last().Text())[1]

		// Full location: building number, name, and room
		location := data.Eq(3).Text()

		// Days of the week
		days := []byte(data.Eq(2).Text())

		// Starting and ending calendar dates
		dates := strings.Split(data.Eq(4).Text(), " - ")
		dateFmt := "Jan 02, 2006"
		dateStart, _ := time.Parse(dateFmt, dates[0])
		dateEnd, _ := time.Parse(dateFmt, dates[1])

		// Start and end class times
		times := strings.Split(data.Eq(1).Text(), " - ")
		timeFmt := "3:04 pm"
		timeStart, _ := time.Parse(timeFmt, times[0])
		timeEnd, timeErr := time.Parse(timeFmt, times[1])

		if timeErr != nil {
			log.Printf("Omitting %s (%s): online course\n", code, name)
			return // courses without a time can't be placed onto a calendar
		}

		courses = append(courses, Course{
			Code:       code,
			Name:       name,
			Instructor: instructor,
			Location:   location,
			Days:       days,
			DateStart:  dateStart,
			DateEnd:    dateEnd,
			TimeStart:  timeStart,
			TimeEnd:    timeEnd,
		})
	})

	return courses
}

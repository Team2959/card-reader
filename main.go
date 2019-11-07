package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

const (
	tokenFile      = "token.json"
	credentialFile = "credentials.json"
	rawDataFile    = "raw.csv"
	sheetID        = "1URmh10PxCqXjTNDHYlV7UkPxg6sxyiXzF2qLOWA_4Go"
)

type scan struct {
	uid       uint64
	timestamp time.Time
}

func getClient(config *oauth2.Config) *http.Client {
	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokenFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to following URL: %v\n Then type the authorization code: \n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code. %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web. %v", err)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token. %v\n", err)
	}
	defer f.Close()
	err = json.NewEncoder(f).Encode(token)
	if err != nil {
		log.Fatalf("Unable to save token. %v", err)
	}
}

func appendFile(file string, data string) error {
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = f.WriteString(data); err != nil {
		return err
	}
	return nil
}

func scanHandler(srvc *sheets.Service, c chan scan) {

	for s := range c {
		// s is a new scan
		// The new scan needs to be processed and written to whatever persistance system is being used
		fmt.Printf("New Scan: %v at %v\n", s.uid, s.timestamp.String())
		if err := appendFile(rawDataFile, fmt.Sprintf("%v, %v\n", s.uid, s.timestamp.String())); err != nil {
			log.Fatalf("Failed to append to local file. %v", err)
		}
		// Write the values to a google sheet
		vals := &sheets.ValueRange{
			Values:         [][]interface{}{{s.uid, s.timestamp.String()}},
			MajorDimension: "ROWS",
		}
		call := srvc.Spreadsheets.Values.Append(sheetID, "Raw Scans!A1:B", vals)
		call.ValueInputOption("USER_ENTERED")
		resp, err := call.Do()
		if err != nil || resp.HTTPStatusCode != 200 {
			fmt.Printf("Unable to update spreadsheet. %v", err)
			//log.Fatalf("Unable to update spreadsheet. %v", err)
		}
	}
}

func main() {
	// Setup the google sheets api
	// Attempt to read a local client secret
	b, err := ioutil.ReadFile(credentialFile)
	if err != nil {
		log.Fatalf("Unable to read client secret. %v", err)
	}

	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret or confid. %v", err)
		return
	}

	client := getClient(config)

	srv, err := sheets.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client. %v", err)
	}
	// Create a go routine to handle scans
	// Communication will be through a channel
	ch := make(chan scan, 128)
	go scanHandler(srv, ch)
	// Create a new reader interface on stdin
	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			// Stdin should never reach EOF, this condition is likely not a recoverable error
			break
		}

		// Trim the input string, specifically the trailing newline
		trimmed := strings.Trim(input, "\n\r \t")
		// Convert the string to a number
		uid, err := strconv.ParseUint(trimmed, 10, 64)
		if err == nil {
			// Send scan data to the handler routine
			ch <- scan{uid, time.Now()}
		}
	}

	// If the loop is exited close the channel
	close(ch)
}

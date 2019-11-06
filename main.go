package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type scan struct {
	uid       uint64
	timestamp time.Time
}

func scanHandler(c chan scan) {
	for s := range c {
		// s is a new scan
		// The new scan needs to be processed and written to whatever persistance system is being used
		fmt.Printf("New Scan: %v at %v\n", s.uid, s.timestamp.String())
	}
}

func main() {
	// Create a go routine to handle scans
	// Communication will be through a channel
	ch := make(chan scan, 128)
	go scanHandler(ch)
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

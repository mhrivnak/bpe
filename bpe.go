package main

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type RunRequest struct {
	Delay  time.Duration // how long the client is willing to wait
	Notify chan bool     // used to notify the web handler that a puppet run happened
}

type ClientArgs struct {
	Delay time.Duration // how long the client is willing to wait
}

// web handlers will submit new requests to this channel
// buffer allows low-latency responses from the async handler
var incoming = make(chan RunRequest, 100)

// this ensures that only one runner can run at a time
var readyToRun = make(chan bool, 1)

func main() {
	http.HandleFunc("/sync", SyncHandler)
	http.HandleFunc("/async", AsyncHandler)
	go manageQueue()
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe error:", err)
	}
}

func AsyncHandler(w http.ResponseWriter, r *http.Request) {
	args, err := parseClientArgs(r.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		incoming <- RunRequest{args.Delay, nil}
		w.WriteHeader(http.StatusAccepted)
	}
}

func SyncHandler(w http.ResponseWriter, r *http.Request) {
	args, err := parseClientArgs(r.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		// buffer so the runner doesn't have to wait for this handler to
		// receive the notification
		notify := make(chan bool, 1)
		incoming <- RunRequest{args.Delay, notify}
		// wait to be notified of completion
		<-notify
	}
}

func parseClientArgs(url *url.URL) (ClientArgs, error) {
	delay_int, err := strconv.Atoi(url.Query().Get("delay"))
	if err != nil {
		return ClientArgs{}, errors.New("Could not find or parse args")
	}
	return ClientArgs{time.Duration(delay_int) * time.Second}, nil
}

func PuppetRunner(requests chan RunRequest, runNum uint64) {
	// let the next runner start when I am finished
	defer func() { readyToRun <- true }()

	// slice of channels that want to be notified when this particular run completes
	waiters := make([]chan bool, 0)
	for req := range requests {
		if req.Delay < time.Duration(0)*time.Second {
			// this is the sentinal value that tells us to move on
			log.Println("sentinal received; proceeding with run", runNum)
			break
		}
		if req.Notify != nil {
			waiters = append(waiters, req.Notify)
		}
	}

	log.Println("starting puppet run", runNum)
	time.Sleep(8 * time.Second)
	log.Println("finished puppet run", runNum)

	// notify clients who are waiting
	for _, waiter := range waiters {
		waiter <- true
	}
}

func timer(delay time.Duration, notify chan uint64, runNum uint64) {
	log.Println("setting timer", delay)
	<-time.After(delay)
	notify <- runNum
}

func manageQueue() {
	readyToRun <- true
	// Many timers might be attached to this channel, but only the first one
	// to fire will matter. The rest will be ignored.
	start := make(chan uint64)
	queue := make(chan RunRequest)
	var runNum uint64 = 0

	go PuppetRunner(queue, runNum)

	log.Println("ready")
	for {
		select {
		case request := <-incoming:
			log.Println("request received for run", runNum)
			if request.Notify != nil {
				queue <- request
			}
			go timer(request.Delay, start, runNum)

		case firedRunNum := <-start:
			if firedRunNum != runNum {
				// This is a timer from a previous run, which can be ignored. It is
				// important to receive these so the timer goroutines can exit.
				continue
			}
			// a timer has fired, so it's time to run!
			log.Println("timer fired; waiting to start run", runNum)
			for currentRun := runNum; currentRun == runNum; {
				select {
				case request := <-incoming:
					log.Println("request received for run", runNum)
					// keep sending requests to the same runner until it's ready to start
					if request.Notify != nil {
						queue <- request
					}
				case <-readyToRun:
					// the runner is ready, so we send it the sentinal value,
					// create a new runner, and start sending requests to the new one
					queue <- RunRequest{time.Duration(-1) * time.Second, nil}
					queue = make(chan RunRequest)
					runNum += 1
					go PuppetRunner(queue, runNum)
				}
			}
		}
	}
}

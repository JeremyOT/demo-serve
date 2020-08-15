package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	address   = flag.String("address", "", "The address to connect to.")
	interval  = flag.Duration("interval", time.Second, "How often to make requests.")
	duration  = flag.Duration("duration", 0, "How long to make requests for (forever by default).")
	once      = flag.Bool("once", false, "Make a single request and exit.")
	keepAlive = flag.Bool("keep-alive", false, "Use HTTP keep-alives.")
	version   = flag.Bool("version", false, "Print the version and exit.")
	// Build version
	Build = "n/a"
)

func logRequest(client *http.Client) {
	resp, err := client.Get(*address)
	if err != nil {
		log.Printf("Request error: %v", err)
		return
	}
	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Request error: %v", err)
		return
	}
	log.Println(string(buf))
}

func main() {
	flag.Parse()
	if *version {
		fmt.Println("Build:", Build)
		return
	}
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	if !strings.HasPrefix(*address, "http") {
		*address = "http://" + *address
	}
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: !*keepAlive}}
	logRequest(client)
	if *once {
		return
	}
	until := make(<-chan time.Time)
	if *duration > 0 {
		until = time.After(*duration)
	}
	tick := time.Tick(*interval)
	for {
		select {
		case <-sigChan:
			return
		case <-tick:
			logRequest(client)
		case <-until:
			return
		}
	}
}

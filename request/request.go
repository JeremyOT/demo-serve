package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	address     = flag.String("address", "", "The address to connect to.")
	interval    = flag.Duration("interval", time.Second, "How often to make requests.")
	duration    = flag.Duration("duration", 0, "How long to make requests for (forever by default).")
	once        = flag.Bool("once", false, "Make a single request and exit.")
	keepAlive   = flag.Bool("keep-alive", false, "Use HTTP keep-alives.")
	version     = flag.Bool("version", false, "Print the version and exit.")
	quiet       = flag.Bool("quiet", false, "Don't log responses. If used with template, print only the template text.")
	showLatency = flag.Bool("latency", false, "Prefix responses with latency. Ignored if format is supplied.")
	format      = flag.String("format", "", "Log results with the specified go-template. Allowed variables: {{.Response, .Latency, .Seconds, .Body}}.")
	workers     = flag.Int("workers", 1, "Make requests in parallel.")
	protocol    = flag.String("protocol", "http", "{udp, tcp, http}")
	// Build version
	Build            = "n/a"
	responseTemplate *template.Template
)

type response struct {
	Response *http.Response
	Latency  time.Duration
	Seconds  float64
	Body     string
}

func responseString(r *response) string {
	if responseTemplate != nil {
		var buf bytes.Buffer
		if err := responseTemplate.Execute(&buf, r); err != nil {
			return fmt.Sprintf("%v: %v", err, r.Body)
		}
		return string(buf.Bytes())
	}
	return r.Body
}

func logHTTPRequest(client *http.Client) {
	start := time.Now()
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
	l := time.Since(start)
	r := &response{
		Response: resp,
		Latency:  l,
		Seconds:  l.Seconds(),
		Body:     string(buf),
	}
	if *quiet {
		if responseTemplate != nil {
			fmt.Println(responseString(r))
		}
		return
	}
	log.Println(responseString(r))
}

func logRaw() {
	start := time.Now()
	conn, err := net.Dial(*protocol, *address)
	if err != nil {
		log.Printf("%v dial error: %v", *protocol, err)
		return
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(*address)); err != nil {
		log.Printf("%v write error: %v", *protocol, err)
		return
	}
	var buf [4096]byte
	n, err := conn.Read(buf[0:])
	if err != nil {
		log.Printf("%v read error: %v", *protocol, err)
		return
	}
	l := time.Since(start)
	r := &response{
		Latency: l,
		Seconds: l.Seconds(),
		Body:    string(buf[:n]),
	}
	if *quiet {
		if responseTemplate != nil {
			fmt.Println(responseString(r))
		}
		return
	}
	log.Println(responseString(r))
}

func logRequest(client *http.Client) {
	switch *protocol {
	case "http":
		logHTTPRequest(client)
		return
	case "tcp", "udp":
		logRaw()
	default:
		log.Fatalf("Unsupported protocol: %v", *protocol)
	}
}

func monitorSignal(quit chan struct{}, sigChan <-chan os.Signal) {
	<-sigChan
	close(quit)
	<-sigChan
	log.Print("Force quitting...")
	os.Exit(-1)
}

func requestLoop(client *http.Client, quit <-chan struct{}) {
	until := make(<-chan time.Time)
	if *duration > 0 {
		until = time.After(*duration)
	}
	tick := time.Tick(*interval)
	for {
		select {
		case <-quit:
			return
		case <-tick:
			logRequest(client)
		case <-until:
			return
		}
	}
}

func main() {
	flag.Parse()
	if *version {
		fmt.Println("Build:", Build)
		return
	}
	sigChan := make(chan os.Signal, 2)
	quit := make(chan struct{})
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	go monitorSignal(quit, sigChan)
	*protocol = strings.ToLower(*protocol)
	if *protocol == "http" && !strings.HasPrefix(*address, "http") {
		*address = "http://" + *address
	}

	if *format == "" {
		if *showLatency {
			if *quiet {
				*format = "{{.Latency}}"
			} else {
				*format = "[{{.Latency}}] {{.Body}}"
			}
		}
	}
	if *format != "" {
		responseTemplate = template.Must(template.New("response").Parse(*format))
	}

	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: !*keepAlive}}
	if *once {
		logRequest(client)
		return
	}
	if *workers > 1 {
		for i := 0; i < *workers; i++ {
			go requestLoop(client, quit)
		}
		<-quit
		return
	}
	requestLoop(client, quit)
}

package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/JeremyOT/address/lookup"
	"github.com/JeremyOT/httpserver"
)

var (
	address  = flag.String("address", ":8080", "Serve on this address.")
	protocol = flag.String("protocol", "http", "{udp, tcp, http}")
	message  = flag.String("message", "Hello from {{addr}}", "The message to respond with.")
	version  = flag.Bool("version", false, "Print the version and exit.")
	// Build version
	Build = "n/a"
)

func monitorSignal(s *httpserver.Server, sigChan <-chan os.Signal) {
	sig := <-sigChan
	log.Printf("Exiting (%s)...", sig)
	select {
	case <-s.Stop():
		return
	case <-sigChan:
		log.Printf("Force quitting (%s)...", sig)
		os.Exit(-1)
	}
}

type service struct {
	*httpserver.Server
	message *template.Template
}

func (s *service) cpuLoad(request *http.Request) {
	start := time.Now()
	ops := 100000000
	count := request.URL.Query().Get("ops")
	if count != "" {
		var err error
		if ops, err = strconv.Atoi(count); err != nil {
			log.Printf("Error parsing ops: %v", err)
		}
	}
	x := 42.0
	for i := 0; i < ops; i++ {
		x += math.Sqrt(42.0)
	}
	log.Printf("Performed %v operations in %v", ops, time.Now().Sub(start))
}

func (s *service) handleRequest(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/cpu" {
		s.cpuLoad(request)
	}
	s.message.Execute(writer, nil)
}

func localAddr() string {
	addr, err := lookup.GetAddress(true)
	if err != nil {
		panic(err)
	}
	return addr
}

func currentTime() string {
	return time.Now().Format(time.RFC3339)
}

func listen(l net.Listener, s *service) {
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("Failed to accept incoming connection: %v", err)
		}
		log.Printf("Incoming connection from %v", conn.RemoteAddr())
		go func() {
			defer conn.Close()
			var buf [4096]byte
			_, err := conn.Read(buf[:])
			if err != nil {
				log.Printf("Read failed: %v", err)
			}
			var output bytes.Buffer
			s.message.Execute(&output, nil)
			output.WriteTo(conn)
		}()
	}
}

func listenPacket(conn net.PacketConn, s *service) {
	for {
		var buf [4096]byte
		_, addr, err := conn.ReadFrom(buf[:])
		if err != nil {
			log.Printf("Read failed: %v", err)
		}
		log.Printf("Incoming packet from %v", addr)
		var output bytes.Buffer
		s.message.Execute(&output, nil)
		conn.WriteTo(output.Bytes(), addr)
	}
}

func main() {
	flag.Parse()
	if *version {
		fmt.Println("Build:", Build)
		return
	}
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	funcs := template.FuncMap{"env": os.Getenv, "now": currentTime, "addr": localAddr}
	s := &service{
		message: template.Must(template.New("message").Funcs(funcs).Parse(*message)),
	}
	*protocol = strings.ToLower(*protocol)
	switch *protocol {
	case "http":
		s.Server = httpserver.New(s.handleRequest)
		s.Start(*address)
		go monitorSignal(s.Server, sigChan)
		<-s.Wait()
	case "tcp":
		l, err := net.Listen(*protocol, *address)
		if err != nil {
			log.Fatalf("Failed to listen %v %v: %v", *protocol, *address, err)
		}
		go listen(l, s)
		<-sigChan
	case "udp":
		l, err := net.ListenPacket(*protocol, *address)
		if err != nil {
			log.Fatalf("Failed to listen %v %v: %v", *protocol, *address, err)
		}
		go listenPacket(l, s)
		<-sigChan
	default:
		log.Fatalf("Unsupported protocol: %v", *protocol)
	}
}

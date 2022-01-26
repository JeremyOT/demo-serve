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

type multiArg []string

func (a *multiArg) Set(v string) error {
	*a = append(*a, v)
	return nil
}

func (a *multiArg) String() string {
	return strings.Join(*a, ",")
}

var (
	httpAddress multiArg
	tcpAddress  multiArg
	udpAddress  multiArg
	message     = flag.String("message", "Hello from {{addr}}", "The message to respond with.")
	version     = flag.Bool("version", false, "Print the version and exit.")
	// Build version
	Build           = "n/a"
	cachedLocalAddr = ""
)

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

	s.message.Execute(writer, map[string]string{"remote": request.RemoteAddr})
}

func localAddr() string {
	if cachedLocalAddr != "" {
		return cachedLocalAddr
	}
	addr, err := lookup.GetAddress(true)
	if err != nil {
		panic(err)
	}
	cachedLocalAddr = addr
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
		addr := conn.RemoteAddr()
		log.Printf("Incoming connection from %v", addr)
		go func() {
			defer conn.Close()
			var buf [4096]byte
			_, err := conn.Read(buf[:])
			if err != nil {
				log.Printf("Read failed: %v", err)
			}
			var output bytes.Buffer
			s.message.Execute(&output, map[string]string{"remote": addr.String()})
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
		s.message.Execute(&output, map[string]string{"remote": addr.String()})
		conn.WriteTo(output.Bytes(), addr)
	}
}

func monitorSignal(quit chan struct{}, sigChan <-chan os.Signal) {
	<-sigChan
	close(quit)
	<-sigChan
	log.Print("Force quitting...")
	os.Exit(-1)
}

func main() {
	flag.Var(&httpAddress, "http", "Serve on this address.")
	flag.Var(&tcpAddress, "tcp", "Serve on this address.")
	flag.Var(&udpAddress, "udp", "Serve on this address.")
	flag.CommandLine.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nServe multiple ports/protocols by passing each the udp, tcp, http args multiple times.\n\nThe default is equivalent to --http=:8080.")
	}
	flag.Parse()
	if *version {
		fmt.Println("Build:", Build)
		return
	}
	if len(httpAddress) == 0 && len(tcpAddress) == 0 && len(udpAddress) == 0 {
		httpAddress = append(httpAddress, ":8080")
	}
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	quit := make(chan struct{})
	go monitorSignal(quit, sigChan)

	funcs := template.FuncMap{"env": os.Getenv, "now": currentTime, "addr": localAddr}

	s := &service{
		message: template.Must(template.New("message").Funcs(funcs).Parse(*message)),
	}

	for _, a := range httpAddress {
		s.Server = httpserver.New(s.handleRequest)
		if err := s.Start(a); err != nil {
			log.Fatalf("Failed to listen http://%v: %v", a, err)
		}
	}
	for _, a := range tcpAddress {
		l, err := net.Listen("tcp", a)
		if err != nil {
			log.Fatalf("Failed to listen tcp://%v: %v", a, err)
		}
		go listen(l, s)
	}
	for _, a := range udpAddress {
		l, err := net.ListenPacket("udp", a)
		if err != nil {
			log.Fatalf("Failed to listen udp://%v: %v", a, err)
		}
		go listenPacket(l, s)
	}

	<-quit
}

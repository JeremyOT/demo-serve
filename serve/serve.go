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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

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
	httpAddress              multiArg
	tcpAddress               multiArg
	udpAddress               multiArg
	rateMetricSampleInterval = flag.Duration("rate-metric-sample-interval", time.Second, "Rate metrics will be averaged over the specified interval, reported per-second")
	message                  = flag.String("message", "Hello from {{addr}}", "The message to respond with.")
	version                  = flag.Bool("version", false, "Print the version and exit.")
	// Build version
	Build           = "n/a"
	cachedLocalAddr = ""

	requestCount   = atomic.Uint64{}
	requestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "incoming_requests",
			Help: "Count of incoming requests",
		},
		[]string{"host", "path"},
	)
	requestsPerSec = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "incoming_requests_per_second",
		Help: "The rate of incoming requests",
	})
	connectionCount   = atomic.Uint64{}
	connectionCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "incoming_connections",
			Help: "Count of incoming connections",
		},
		[]string{"address"},
	)
	connectionsPerSec = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "incoming_connections_per_second",
		Help: "The rate of incoming connections",
	})

	packetCount   = atomic.Uint64{}
	packetCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "incoming_packets",
			Help: "Count of incoming packets",
		},
		[]string{"address"},
	)
	packetsPerSec = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "incoming_packets_per_second",
		Help: "The rate of incoming packets",
	})
)

func init() {
	prometheus.MustRegister(requestCounter)
	prometheus.MustRegister(requestsPerSec)
	prometheus.MustRegister(connectionCounter)
	prometheus.MustRegister(connectionsPerSec)
	prometheus.MustRegister(packetCounter)
	prometheus.MustRegister(packetsPerSec)
}

func runRateMetrics(interval time.Duration, quit <-chan struct{}) {
	lastRequestCount := requestCount.Load()
	lastConnectionCount := connectionCount.Load()
	lastPacketCount := packetCount.Load()
	lastTick := time.Now()
	t := time.NewTicker(interval)
	for {
		select {
		case <-t.C:
			rc := requestCount.Load()
			cc := connectionCount.Load()
			pc := packetCount.Load()
			realInterval := time.Since(lastTick).Seconds()
			lastTick = time.Now()
			requestsPerSec.Set(float64(rc-lastRequestCount) / realInterval)
			connectionsPerSec.Set(float64(cc-lastConnectionCount) / realInterval)
			packetsPerSec.Set(float64(pc-lastPacketCount) / realInterval)
			lastRequestCount = rc
			lastConnectionCount = cc
			lastPacketCount = pc
		case <-quit:
			return
		}
	}

}

func cpuLoad(request *http.Request) {
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
	log.Printf("Performed %v operations in %v", ops, time.Since(start))
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

func listen(l net.Listener, messageTemplate *template.Template) {
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("Failed to accept incoming connection: %v", err)
		}
		addr := conn.RemoteAddr()
		log.Printf("Incoming connection from %v", addr)
		go func() {
			defer conn.Close()
			connectionCounter.WithLabelValues(conn.LocalAddr().String()).Inc()
			connectionCount.Add(1)
			var buf [4096]byte
			_, err := conn.Read(buf[:])
			if err != nil {
				log.Printf("Read failed: %v", err)
			}
			var output bytes.Buffer
			messageTemplate.Execute(&output, map[string]string{"remote": addr.String()})
			output.WriteTo(conn)
		}()
	}
}

func listenPacket(conn net.PacketConn, messageTemplate *template.Template) {
	for {
		var buf [4096]byte
		_, addr, err := conn.ReadFrom(buf[:])
		if err != nil {
			log.Printf("Read failed: %v", err)
		}
		packetCounter.WithLabelValues(conn.LocalAddr().String()).Inc()
		packetCount.Add(1)
		log.Printf("Incoming packet from %v", addr)
		var output bytes.Buffer
		messageTemplate.Execute(&output, map[string]string{"remote": addr.String()})
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

	messageTemplate := template.Must(template.New("message").Funcs(funcs).Parse(*message))

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestCounter.WithLabelValues(r.Host, r.URL.Path).Inc()
		requestCount.Add(1)
		messageTemplate.Execute(w, map[string]string{"remote": r.RemoteAddr})
	})
	mux.HandleFunc("/cpu", func(w http.ResponseWriter, r *http.Request) {
		requestCounter.WithLabelValues(r.Host, r.URL.Path).Inc()
		requestCount.Add(1)
		cpuLoad(r)
		messageTemplate.Execute(w, map[string]string{"remote": r.RemoteAddr})
	})
	mux.Handle("/metrics", promhttp.Handler())

	for _, a := range httpAddress {
		server := httpserver.New(mux.ServeHTTP)
		if err := server.Start(a); err != nil {
			log.Fatalf("Failed to listen http://%v: %v", a, err)
		}
	}
	for _, a := range tcpAddress {
		l, err := net.Listen("tcp", a)
		if err != nil {
			log.Fatalf("Failed to listen tcp://%v: %v", a, err)
		}
		go listen(l, messageTemplate)
	}
	for _, a := range udpAddress {
		l, err := net.ListenPacket("udp", a)
		if err != nil {
			log.Fatalf("Failed to listen udp://%v: %v", a, err)
		}
		go listenPacket(l, messageTemplate)
	}
	go runRateMetrics(*rateMetricSampleInterval, quit)
	<-quit
}

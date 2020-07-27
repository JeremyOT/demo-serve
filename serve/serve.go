package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JeremyOT/address/lookup"
	"github.com/JeremyOT/httpserver"
)

var (
	address = flag.String("address", ":8080", "Serve on this address.")
	message = flag.String("message", "Hello from {{addr}}", "The message to respond with.")
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

func (s *service) handleRequest(writer http.ResponseWriter, request *http.Request) {
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

func main() {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)
	flag.Parse()
	funcs := template.FuncMap{"env": os.Getenv, "now": currentTime, "addr": localAddr}
	s := &service{
		message: template.Must(template.New("message").Funcs(funcs).Parse(*message)),
	}
	s.Server = httpserver.New(s.handleRequest)
	s.Start(*address)
	go monitorSignal(s.Server, sigChan)
	<-s.Wait()
}

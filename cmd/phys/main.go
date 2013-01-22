package main

import (
	"github.com/ianremmler/phys"
	"code.google.com/p/go.net/websocket"

	"go/build"
	"math/rand"
	"net/http"
	"os"
	"time"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	p := phys.NewPhys()
	p.Run()

	htmlDir := build.Default.GOPATH + "/src/github.com/ianremmler/phys/html"
	http.Handle("/phys/", websocket.Handler(p.WSHandler()))
	http.Handle("/", http.FileServer(http.Dir(htmlDir)))
	port := ":8080"
	if len(os.Args) > 1 {
		port = ":" + os.Args[1]
	}
	if err := http.ListenAndServe(port, nil); err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}

package main

import (
	"github.com/ianremmler/bump"
	"code.google.com/p/go.net/websocket"

	"go/build"
	"math/rand"
	"net/http"
	"os"
	"time"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	p := bump.NewBump()
	p.Run()

	htmlDir := build.Default.GOPATH + "/src/github.com/ianremmler/bump/html"
	http.Handle("/bump/", websocket.Handler(p.WSHandler()))
	http.Handle("/", http.FileServer(http.Dir(htmlDir)))
	port := ":8000"
	if len(os.Args) > 1 {
		port = ":" + os.Args[1]
	}
	if err := http.ListenAndServe(port, nil); err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}

package main

import (
	"remmler.org/go/bump.git"
	"code.google.com/p/go.net/websocket"

	"go/build"
	"math/rand"
	"net/http"
	"os"
	"time"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	bump := bump.NewBump()
	bump.Run()

	clientDir := build.Default.GOPATH + "/src/remmler.org/go/bump.git/client"
	http.Handle("/bump/", websocket.Handler(bump.WSHandler()))
	http.Handle("/", http.FileServer(http.Dir(clientDir)))
	port := ":8000"
	if len(os.Args) > 1 {
		port = ":" + os.Args[1]
	}
	if err := http.ListenAndServe(port, nil); err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}

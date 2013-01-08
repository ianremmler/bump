package main

import (
	"github.com/ianremmler/phys"
	"code.google.com/p/go.net/websocket"

	"go/build"
	"net/http"
)

func main() {
	p := phys.NewPhys()
	defer p.Cleanup()
	p.Run()

	htmlDir := build.Default.GOPATH + "/src/github.com/ianremmler/phys/html"
	http.Handle("/phys/", websocket.Handler(p.WSHandler()))
	http.Handle("/", http.FileServer(http.Dir(htmlDir)))
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}

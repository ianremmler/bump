package phys

import (
	"github.com/ianremmler/gordian"

	"strconv"
	"time"
)

const (
	ticTime    = time.Second / 60
	updateTime = time.Second / 30
)

type Phys struct {
	clients map[gordian.ClientId]struct{}
	phys    <-chan time.Time
	curId   int
	*gordian.Gordian
}

func NewPhys() *Phys {
	p := &Phys{
		clients: make(map[gordian.ClientId]struct{}),
		phys:    time.Tick(ticTime),
		Gordian: gordian.New(),
	}
	return p
}

func (p *Phys) Run() {
	go p.run()
	p.Gordian.Run()
}

func (p *Phys) run() {
	msg := &gordian.Message{}
	data := map[string]interface{}{"type": "message"}
	i := 0
	for {
		select {
		case client := <-p.Control:
			switch client.Ctrl {
			case gordian.CONNECT:
				p.curId++
				client.Id = p.curId
				client.Ctrl = gordian.REGISTER
				p.clients[client.Id] = struct{}{}
				p.Control <- client
			case gordian.CLOSE:
				delete(p.clients, client.Id)
			}
		case <-p.phys:
			data["data"] = strconv.Itoa(i)
			msg.Data = data
			for id, _ := range p.clients {
				msg.To = id
				p.Message <- msg
			}
			i++
		}
	}
}

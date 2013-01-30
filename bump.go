package bump

import (
	"github.com/ftrvxmtrx/gochipmunk/chipmunk"
	"github.com/ianremmler/gordian"

	"fmt"
	"math"
	"time"
)

const (
	simTime        = time.Second / 72
	updateTime     = time.Second / 24
	arenaRadius    = 320
	arenaSegs      = 360
	arenaThickness = 8
	playerRadius   = 16
	playerMass     = 1
)

type player struct {
	id          gordian.ClientId
	body        chipmunk.Body
	shape       chipmunk.Shape
	cursorBody  chipmunk.Body
	cursorJoint chipmunk.Constraint
}

type Player struct {
	Id  gordian.ClientId
	Pos chipmunk.Vect
}

type configMsg struct {
	ArenaRadius  float64
	PlayerRadius float64
}

type Bump struct {
	players     map[gordian.ClientId]player
	simTimer    <-chan time.Time
	updateTimer <-chan time.Time
	curId       int
	space       *chipmunk.Space
	arena       []chipmunk.Shape
	*gordian.Gordian
}

func NewBump() *Bump {
	b := &Bump{
		players:     map[gordian.ClientId]player{},
		arena:       make([]chipmunk.Shape, arenaSegs),
		simTimer:    time.Tick(simTime),
		updateTimer: time.Tick(updateTime),
		Gordian:     gordian.New(24),
	}
	b.setup()
	return b
}

func (b *Bump) setup() {
	b.space = chipmunk.SpaceNew()
	rad := arenaRadius + 0.5*arenaThickness
	for i := range b.arena {
		a0 := float64(i) / arenaSegs * 2.0 * math.Pi
		a1 := float64(i+1) / arenaSegs * 2.0 * math.Pi
		p0 := chipmunk.Vect{rad * math.Cos(a0), rad * math.Sin(a0)}
		p1 := chipmunk.Vect{rad * math.Cos(a1), rad * math.Sin(a1)}
		b.arena[i] = chipmunk.SegmentShapeNew(b.space.StaticBody(), p0, p1,
			0.5*arenaThickness)
		b.arena[i].SetElasticity(1.0)
		b.arena[i].SetFriction(1.0)
		b.space.AddShape(b.arena[i])
	}
}

func (b *Bump) Run() {
	go b.run()
	b.Gordian.Run()
}

func (b *Bump) run() {
	for {
		select {
		case client := <-b.Control:
			b.clientCtrl(client)
		case msg := <-b.InMessage:
			b.handleMessage(&msg)
		case <-b.simTimer:
			b.space.Step(float64(simTime) / float64(time.Second))
		case <-b.updateTimer:
			b.update()
		}
	}
}

func (b *Bump) clientCtrl(client *gordian.Client) {
	switch client.Ctrl {
	case gordian.CONNECT:
		b.connect(client)
	case gordian.CLOSE:
		b.close(client)
	}
}

func (b *Bump) connect(client *gordian.Client) {
	b.curId++

	client.Id = b.curId
	client.Ctrl = gordian.REGISTER
	b.Control <- client
	client = <-b.Control
	if client.Ctrl != gordian.ESTABLISH {
		return
	}

	plr := player{}
	plr.id = client.Id
	moment := chipmunk.MomentForCircle(playerMass, 0, playerRadius, chipmunk.Origin())
	plr.body = chipmunk.BodyNew(playerMass, moment)
	b.space.AddBody(plr.body)
	plr.shape = chipmunk.CircleShapeNew(plr.body, playerRadius, chipmunk.Origin())
	plr.shape.SetElasticity(0.9)
	plr.shape.SetFriction(0.1)
	b.space.AddShape(plr.shape)
	plr.cursorBody = chipmunk.BodyNew(math.Inf(0), math.Inf(0))
	plr.cursorJoint = chipmunk.PivotJointNew2(plr.cursorBody, plr.body,
		chipmunk.Origin(), chipmunk.Origin())
	plr.cursorJoint.SetMaxForce(1000.0)
	b.space.AddConstraint(plr.cursorJoint)
	b.players[plr.id] = plr

	data := configMsg{
		ArenaRadius:  arenaRadius,
		PlayerRadius: playerRadius,
	}
	msg := newMsg("config", data)
	msg.To = plr.id
	b.OutMessage <- msg
}

func (b *Bump) close(client *gordian.Client) {
	plr, ok := b.players[client.Id]
	if !ok {
		return
	}
	b.space.RemoveConstraint(plr.cursorJoint)
	plr.cursorJoint.Free()
	b.space.RemoveBody(plr.body)
	b.space.RemoveShape(plr.shape)
	plr.body.Free()
	plr.shape.Free()
	plr.cursorBody.Free()
	delete(b.players, client.Id)
}

func (b *Bump) handleMessage(msg *gordian.Message) {
	id := msg.From
	plr, ok := b.players[id]
	if !ok {
		return
	}
	rawData, ok := msg.Data.(map[string]interface{})
	if !ok {
		return
	}
	typ, ok := rawData["type"].(string)
	if !ok {
		return
	}
	switch typ {
	case "player":
		data, ok := rawData["data"].(map[string]interface{})
		if !ok {
			return
		}
		rawPos, ok := data["pos"].(map[string]interface{})
		if !ok {
			return
		}
		x, ok := rawPos["x"].(float64)
		if !ok {
			return
		}
		y, ok := rawPos["y"].(float64)
		if !ok {
			return
		}
		pos := chipmunk.Vect{x, y}
		plr.cursorBody.SetPosition(pos)
	}
	b.players[id] = plr
}

func (b *Bump) update() {
	players := map[string]interface{}{}
	for i, plr := range b.players {
		players[fmt.Sprintf("%d", i)] = Player{Id: plr.id, Pos: plr.body.Position()}
	}
	msg := newMsg("state", map[string]interface{}{"players": players})
	for id := range b.players {
		msg.To = id
		b.OutMessage <- msg
	}
}

func newMsg(msgType string, msgData interface{}) gordian.Message {
	msg := gordian.Message{}
	hdr := map[string]interface{}{}
	hdr["type"] = msgType
	hdr["data"] = msgData
	msg.Data = hdr
	return msg
}

package bump

import (
	"github.com/ianremmler/gochipmunk/chipmunk"
	"github.com/ianremmler/gordian"

	"crypto/sha1"
	"fmt"
	"io"
	"math"
	"sync"
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
	color       string
}

type Player struct {
	Pos   chipmunk.Vect
	Color string
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
	mu          sync.Mutex
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
	b.space.SetEnableContactGraph(true)
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
	go b.sim()
	b.Gordian.Run()
}

func (b *Bump) run() {
	for {
		select {
		case client := <-b.Control:
			b.clientCtrl(client)
		case msg := <-b.InBox:
			b.handleMessage(&msg)
		case <-b.updateTimer:
			b.update()
		}
	}
}

func (b *Bump) sim() {
	for {
		<-b.simTimer
		b.mu.Lock()
		b.space.Step(float64(simTime) / float64(time.Second))
		for _, player := range b.players {
			player.body.SetUserData(false)
			player.body.EachArbiter(wallCollisionCheck)
			isCol := player.body.UserData().(bool)
			if isCol {
			}
		}
		b.mu.Unlock()
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

	b.mu.Lock()
	player := player{id: client.Id}
	moment := chipmunk.MomentForCircle(playerMass, 0, playerRadius, chipmunk.Origin())
	player.body = chipmunk.BodyNew(playerMass, moment)
	b.space.AddBody(player.body)
	player.shape = chipmunk.CircleShapeNew(player.body, playerRadius, chipmunk.Origin())
	player.shape.SetElasticity(0.9)
	player.shape.SetFriction(0.1)
	b.space.AddShape(player.shape)
	player.cursorBody = chipmunk.BodyNew(math.Inf(0), math.Inf(0))
	player.cursorJoint = chipmunk.PivotJointNew2(player.cursorBody, player.body,
		chipmunk.Origin(), chipmunk.Origin())
	player.cursorJoint.SetMaxForce(1000.0)
	b.space.AddConstraint(player.cursorJoint)
	player.color = idToColor(player.id)
	b.players[player.id] = player
	b.mu.Unlock()

	data := configMsg{
		ArenaRadius:  arenaRadius,
		PlayerRadius: playerRadius,
	}
	msg := gordian.Message{
		To:   player.id,
		Type: "config",
		Data: data,
	}
	b.OutBox <- msg
}

func (b *Bump) close(client *gordian.Client) {
	b.mu.Lock()
	defer b.mu.Unlock()

	player, ok := b.players[client.Id]
	if !ok {
		return
	}
	b.space.RemoveConstraint(player.cursorJoint)
	player.cursorJoint.Free()
	b.space.RemoveBody(player.body)
	b.space.RemoveShape(player.shape)
	player.body.Free()
	player.shape.Free()
	player.cursorBody.Free()
	delete(b.players, client.Id)
}

func (b *Bump) handleMessage(msg *gordian.Message) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := msg.From
	player, ok := b.players[id]
	if !ok {
		return
	}
	switch msg.Type {
	case "player":
		state := &Player{}
		err := msg.Unmarshal(state)
		if err != nil {
			return
		}
		player.cursorBody.SetPosition(state.Pos)
	}
	b.players[id] = player
}

func (b *Bump) update() {
	players := map[string]Player{}

	b.mu.Lock()
	for i, player := range b.players {
		players[fmt.Sprintf("%d", i)] = Player{
			Pos:   player.body.Position(),
			Color: player.color,
		}
	}
	b.mu.Unlock()

	msg := gordian.Message{
		Type: "state",
		Data: players,
	}
	for id := range b.players {
		msg.To = id
		b.OutBox <- msg
	}
}

func idToColor(id gordian.ClientId) string {
	sha := sha1.New()
	io.WriteString(sha, fmt.Sprintf("%d", id))
	return "#" + fmt.Sprintf("%x", sha.Sum(nil)[:3])
}

func wallCollisionCheck(body chipmunk.Body, arb chipmunk.Arbiter) {
	_, other := arb.Bodies()
	if other.IsStatic() && arb.IsFirstContact() {
		body.SetUserData(true)
	}
}

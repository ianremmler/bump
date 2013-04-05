package bump

import (
	"github.com/ianremmler/gochipmunk/chipmunk"
	"github.com/ianremmler/gordian"

	"fmt"
	"math"
	"sync"
	"time"
)

const (
	simTime        = time.Second / 1000
	updateTime     = time.Second / 24
	arenaRadius    = 500
	arenaSegs      = 360
	arenaThickness = 8
	playerRadius   = 16
	playerMass     = 1
)

const (
	centerType = 1 << iota
	playerType
	wallType
)

const (
	normState = iota
	riskState
	direState
	deadState
)

type player struct {
	id          gordian.ClientId
	team        int
	state       int
	body        chipmunk.Body
	shape       chipmunk.Shape
	cursorBody  chipmunk.Body
	cursorJoint chipmunk.Constraint
}

func (p *player) centerBump() {
	p.state = normState
}

func (p *player) wallBump() {
	switch p.state {
	case normState:
		p.state = riskState
	case direState:
		p.state = normState
		p.body.SetPosition(chipmunk.Vect{})
	}
}

func (p *player) playerBump(other *player) {
	if other.team == p.team {
		return
	}
	switch p.state {
	case riskState:
		p.state = direState
	}
}

type Player struct {
	Pos     chipmunk.Vect
	Team    int
	State   int
}

type configMsg struct {
	ArenaRadius  float64
	PlayerRadius float64
}

type Bump struct {
	players     map[gordian.ClientId]*player
	simTimer    <-chan time.Time
	updateTimer <-chan time.Time
	curId       int
	space       *chipmunk.Space
	arena       []chipmunk.Shape
	center      chipmunk.Shape
	mu          sync.Mutex
	*gordian.Gordian
}

func NewBump() *Bump {
	b := &Bump{
		players:     map[gordian.ClientId]*player{},
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
		b.arena[i].SetCollisionType(wallType)
		b.space.AddShape(b.arena[i])
	}
	b.center = chipmunk.CircleShapeNew(b.space.StaticBody(), playerRadius, chipmunk.Origin())
	b.center.SetElasticity(1.0)
	b.center.SetFriction(1.0)
	b.center.SetCollisionType(centerType)
	b.space.AddShape(b.center)
	b.space.SetUserData(b)
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
			player.body.EachArbiter(checkCollision)
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

func (b *Bump) smallerTeam() int {
	t0Size := 0
	for _, player := range b.players {
		if player.team == 0 {
			t0Size++
		}
	}
	if 2 * t0Size <= len(b.players) {
		return 0
	}
	return 1
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
	player := &player{id: client.Id}
	moment := chipmunk.MomentForCircle(playerMass, 0, playerRadius, chipmunk.Origin())

	player.body = chipmunk.BodyNew(playerMass, moment)
	player.body.SetUserData(client.Id)
	b.space.AddBody(player.body)

	player.shape = chipmunk.CircleShapeNew(player.body, playerRadius, chipmunk.Origin())
	player.shape.SetElasticity(0.9)
	player.shape.SetFriction(0.1)
	player.shape.SetCollisionType(playerType)
	b.space.AddShape(player.shape)

	player.cursorBody = chipmunk.BodyNew(math.Inf(0), math.Inf(0))
	player.cursorJoint = chipmunk.PivotJointNew2(player.cursorBody, player.body,
		chipmunk.Origin(), chipmunk.Origin())
	player.cursorJoint.SetMaxForce(1000.0)
	b.space.AddConstraint(player.cursorJoint)
	player.team = b.smallerTeam()

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
}

func (b *Bump) update() {
	players := map[string]Player{}

	b.mu.Lock()
	for i, player := range b.players {
		players[fmt.Sprintf("%d", i)] = Player{
			Pos:   player.body.Position(),
			Team:  player.team,
			State: player.state,
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

func checkCollision(body chipmunk.Body, arb chipmunk.Arbiter) {
	if !arb.IsFirstContact() {
		return
	}
	bump := body.Space().UserData().(*Bump)
	_, otherShape := arb.Shapes()

	playerId := body.UserData().(gordian.ClientId)
	player := bump.players[playerId]

	switch otherShape.CollisionType() {
	case centerType:
		player.centerBump()
	case playerType:
		otherId := otherShape.Body().UserData().(gordian.ClientId)
		player.playerBump(bump.players[otherId])
	case wallType:
		player.wallBump()
	}
}

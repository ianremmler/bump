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
	arenaRadius    = 320
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
	id    gordian.ClientId
	team  int
	state int
	body  chipmunk.Body
	shape chipmunk.Shape
}

func (p *player) centerBump() {
	p.state = normState
}

func (p *player) wallBump() {
	switch p.state {
	case normState:
		p.state = riskState
	case direState:
		p.state = deadState
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
	Pos   chipmunk.Vect
	Team  int
	State int
}

type configMsg struct {
	Id           string
	ArenaRadius  float64
	PlayerRadius float64
}

type stateMsg struct {
	Players map[string]Player
	Score   []int
}

type Bump struct {
	players     map[gordian.ClientId]*player
	score       []int
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
		score:       []int{0, 0},
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
	b.space.SetDamping(0.5)
	rad := arenaRadius + 0.5*arenaThickness
	for i := range b.arena {
		a0 := float64(i) / arenaSegs * 2.0 * math.Pi
		a1 := float64(i+1) / arenaSegs * 2.0 * math.Pi
		p0 := chipmunk.Vect{rad * math.Cos(a0), rad * math.Sin(a0)}
		p1 := chipmunk.Vect{rad * math.Cos(a1), rad * math.Sin(a1)}
		b.arena[i] = chipmunk.SegmentShapeNew(b.space.StaticBody(), p0, p1, 0.5*arenaThickness)
		b.arena[i].SetElasticity(1.0)
		b.arena[i].SetFriction(1.0)
		b.arena[i].SetCollisionType(wallType)
		b.space.AddShape(b.arena[i])
	}
	b.center = chipmunk.CircleShapeNew(b.space.StaticBody(), 2*playerRadius, chipmunk.Origin())
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
			if player.state == deadState {
				otherTeam := 1 - player.team
				b.score[otherTeam]++
				player.body.SetPosition(chipmunk.Vect{})
				player.state = normState
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

func (b *Bump) smallerTeam() int {
	t0Size := 0
	for _, player := range b.players {
		if player.team == 0 {
			t0Size++
		}
	}
	if 2*t0Size <= len(b.players) {
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
	player.team = b.smallerTeam()
	b.players[player.id] = player

	b.mu.Unlock()

	data := configMsg{
		ArenaRadius:  arenaRadius,
		PlayerRadius: playerRadius,
		Id:           fmt.Sprintf("%d", player.id),
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
	b.space.RemoveBody(player.body)
	b.space.RemoveShape(player.shape)
	player.body.Free()
	player.shape.Free()
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
		dist := state.Pos.Length()
		if dist > 2*playerRadius {
			dir := state.Pos.Mul(20.0 / arenaRadius)
			player.body.ApplyImpulse(dir, chipmunk.Vect{})
		}
	}
}

func (b *Bump) update() {
	b.mu.Lock()

	if b.score[0] > 99 || b.score[1] > 99 {
		b.score[0], b.score[1] = 0, 0
		for _, player := range b.players {
			player.body.SetPosition(chipmunk.Vect{})
			player.state = normState
		}
	}
	state := stateMsg{
		Players: map[string]Player{},
		Score:   b.score,
	}
	for i, player := range b.players {
		state.Players[fmt.Sprintf("%d", i)] = Player{
			Pos:   player.body.Position(),
			Team:  player.team,
			State: player.state,
		}
	}

	b.mu.Unlock()

	msg := gordian.Message{
		Type: "state",
		Data: state,
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

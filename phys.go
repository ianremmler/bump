package phys

import (
	"github.com/ftrvxmtrx/gochipmunk/chipmunk"
	"github.com/ianremmler/gordian"

	"fmt"
	"time"
)

const (
	simTime    = time.Second / 60
	updateTime = time.Second / 20
)

type sim struct {
	space     *chipmunk.Space
	ballBody  chipmunk.Body
	ballShape chipmunk.Shape
	ground    chipmunk.Shape
}

func newSim() *sim {
	s := &sim{}
	gravity := chipmunk.Vect{0, -100}
	s.space = chipmunk.SpaceNew()
	s.space.SetGravity(gravity)
	radius, mass := 5.0, 1.0
	moment := chipmunk.MomentForCircle(mass, 0, radius, chipmunk.Vect{0, 0})
	s.ballBody = chipmunk.BodyNew(mass, moment)
	s.space.AddBody(s.ballBody)
	s.ballShape = chipmunk.CircleShapeNew(s.ballBody, radius, chipmunk.Vect{0, 0})
	s.space.AddShape(s.ballShape)
	s.ballShape.SetFriction(0.7)
	s.ground = chipmunk.SegmentShapeNew(s.space.StaticBody(),
		chipmunk.Vect{-20, 5}, chipmunk.Vect{20, -5}, 0)
	s.space.AddShape(s.ground)
	s.ballBody.SetPosition(chipmunk.Vect{0, 15})
	return s
}

func (s *sim) cleanup() {
	s.space.Free()
	s.ballBody.Free()
	s.ballShape.Free()
	s.ground.Free()
}

type Phys struct {
	clients     map[gordian.ClientId]struct{}
	simTimer    <-chan time.Time
	updateTimer <-chan time.Time
	curId       int
	sim         *sim
	*gordian.Gordian
}

func NewPhys() *Phys {
	p := &Phys{
		clients:     make(map[gordian.ClientId]struct{}),
		simTimer:    time.Tick(simTime),
		updateTimer: time.Tick(updateTime),
		sim:         newSim(),
		Gordian:     gordian.New(),
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
		case <-p.simTimer:
			p.sim.space.Step(float64(simTime) / float64(time.Second))
		case <-p.updateTimer:
			pos := p.sim.ballBody.Position()
			vel := p.sim.ballBody.Velocity()
			s := fmt.Sprintf("pos:%5.2f,%5.2f vel:%5.2f,%5.2f", pos.X, pos.Y, vel.X, vel.Y)
			fmt.Println(s)
			data["data"] = s
			msg.Data = data
			for id := range p.clients {
				msg.To = id
				p.Message <- msg
			}
		}
	}
}

func (p *Phys) Cleanup() {
	p.sim.cleanup()
}

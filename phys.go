package phys

import (
	"github.com/ftrvxmtrx/gochipmunk/chipmunk"
	"github.com/ianremmler/gordian"

	"math/rand"
	"time"
)

const (
	simTime    = time.Second / 72
	updateTime = time.Second / 24
	size       = 500.0
)

type ballInfo struct {
	Pos   chipmunk.Vect
	Angle float64
}

type sim struct {
	space      *chipmunk.Space
	ballBodies []chipmunk.Body
	ballShapes []chipmunk.Shape
	box        []chipmunk.Shape
}

func newSim() *sim {
	rand.Seed(time.Now().UTC().UnixNano())
	s := &sim{}
	s.ballBodies = make([]chipmunk.Body, 42)
	s.ballShapes = make([]chipmunk.Shape, len(s.ballBodies))
	s.box = make([]chipmunk.Shape, 4)
	gravity := chipmunk.Vect{0, -100}
	s.space = chipmunk.SpaceNew()
	s.space.SetGravity(gravity)
	radius, mass := 10.0, 1.0
	moment := chipmunk.MomentForCircle(mass, 0, radius, chipmunk.Vect{0, 0})

	for i := range s.ballBodies {
		s.ballBodies[i] = chipmunk.BodyNew(mass, moment)
		s.space.AddBody(s.ballBodies[i])
		s.ballShapes[i] = chipmunk.CircleShapeNew(s.ballBodies[i], radius,
			chipmunk.Vect{0, 0})
		s.space.AddShape(s.ballShapes[i])
		s.ballShapes[i].SetFriction(0.5)
		s.ballShapes[i].SetElasticity(0.9)
	}

	s.box[0] = chipmunk.SegmentShapeNew(s.space.StaticBody(),
		chipmunk.Vect{0, 0}, chipmunk.Vect{size, 0}, 0)
	s.box[1] = chipmunk.SegmentShapeNew(s.space.StaticBody(),
		chipmunk.Vect{size, 0}, chipmunk.Vect{size, size}, 0)
	s.box[2] = chipmunk.SegmentShapeNew(s.space.StaticBody(),
		chipmunk.Vect{size, size}, chipmunk.Vect{0, size}, 0)
	s.box[3] = chipmunk.SegmentShapeNew(s.space.StaticBody(),
		chipmunk.Vect{0, size}, chipmunk.Vect{0, 0}, 0)
	for i := range s.box {
		s.box[i].SetElasticity(1.0)
		s.box[i].SetFriction(1.0)
		s.space.AddShape(s.box[i])
	}
	s.dropBalls()
	return s
}

func (s *sim) cleanup() {
	// 	s.space.Free()
	// 	s.ballBodies.Free()
	// 	s.ballShape.Free()
	// 	s.box.Free()
}

func (s *sim) dropBalls() {
	for i := range s.ballBodies {
		radius := s.ballShapes[i].(chipmunk.CircleShape).Radius()
		s.ballBodies[i].SetPosition(chipmunk.Vect{radius + rand.Float64()*(size-2*radius),
			0.5*size + rand.Float64()*(0.5*size-radius)})
	}
}

type Phys struct {
	clients     map[gordian.ClientId]chipmunk.Vect
	simTimer    <-chan time.Time
	updateTimer <-chan time.Time
	curId       int
	sim         *sim
	*gordian.Gordian
}

func NewPhys() *Phys {
	p := &Phys{
		clients:     map[gordian.ClientId]chipmunk.Vect{},
		simTimer:    time.Tick(simTime),
		updateTimer: time.Tick(updateTime),
		sim:         newSim(),
		Gordian:     gordian.New(0),
	}
	return p
}

func (p *Phys) Run() {
	go p.run()
	p.Gordian.Run()
}

func (p *Phys) run() {
	msg := gordian.Message{}
	rawData := map[string]interface{}{}
	balls := make([]ballInfo, len(p.sim.ballBodies))
	for {
		select {
		case client := <-p.Control:
			switch client.Ctrl {
			case gordian.CONNECT:
				p.curId++
				client.Id = p.curId
				client.Ctrl = gordian.REGISTER
				p.clients[client.Id] = chipmunk.Vect{}
				p.Control <- client
			case gordian.CLOSE:
				delete(p.clients, client.Id)
			}
		case msg = <-p.InMessage:
			rawData = msg.Data.(map[string]interface{})
			data := rawData["data"].([]interface{})
			idx := int(data[0].(float64)) - 1
			pos := data[1].(map[string]interface{})
			p.clients[msg.From] = chipmunk.Vect{pos["x"].(float64), pos["y"].(float64)}
			if idx >= 0 {
				impulse := chipmunk.Vect{1000*rand.Float64() - 500, 1000*rand.Float64() - 500}
				p.sim.ballBodies[idx].ApplyImpulse(impulse, chipmunk.Vect{0, 0})
			}
		case <-p.simTimer:
			p.sim.space.Step(float64(simTime) / float64(time.Second))
		case <-p.updateTimer:
			for i, bb := range p.sim.ballBodies {
				pos := bb.Position()
				angle := bb.Angle()
				balls[i] = ballInfo{pos, angle}
			}
			data := map[string]interface{}{}
			data["balls"] = balls

			cursors := []chipmunk.Vect{}
			for _, clientPos := range p.clients {
				cursors = append(cursors, clientPos)
			}

			data["cursors"] = cursors
			rawData["data"] = data
			msg.Data = rawData
			for id := range p.clients {
				msg.To = id
				p.OutMessage <- msg
			}
		}
	}
}

func (p *Phys) Cleanup() {
	p.sim.cleanup()
}

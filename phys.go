package phys

import (
	"github.com/ftrvxmtrx/gochipmunk/chipmunk"
	"github.com/ianremmler/gordian"

	"math"
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

type clientData struct {
	body  chipmunk.Body
	joint chipmunk.Constraint
}

type sim struct {
	space      *chipmunk.Space
	ballBodies []chipmunk.Body
	ballShapes []chipmunk.Shape
	box        []chipmunk.Shape
}

func newSim() *sim {
	s := &sim{}
	s.ballBodies = make([]chipmunk.Body, 42)
	s.ballShapes = make([]chipmunk.Shape, len(s.ballBodies))
	s.box = make([]chipmunk.Shape, 4)
	s.space = chipmunk.SpaceNew()
	s.space.SetGravity(chipmunk.Vect{0, -100})
	radius, mass := 10.0, 1.0
	moment := chipmunk.MomentForCircle(mass, 0, radius, chipmunk.Origin())

	for i := range s.ballBodies {
		body := chipmunk.BodyNew(mass, moment)
		s.space.AddBody(body)
		s.ballBodies[i] = body
		shape := chipmunk.CircleShapeNew(body, radius, chipmunk.Origin())
		shape.SetFriction(0.5)
		shape.SetElasticity(0.9)
		shape.SetLayers(1)
		s.space.AddShape(shape)
		s.ballShapes[i] = shape
	}
	pts := []chipmunk.Vect{{0, 0}, {size, 0}, {size, size}, {0, size}}
	for i := range s.box {
		shape := chipmunk.SegmentShapeNew(s.space.StaticBody(), pts[i],
			pts[(i+1)%len(pts)], 0)
		shape.SetElasticity(1.0)
		shape.SetFriction(1.0)
		s.space.AddShape(shape)
		s.box[i] = shape
	}
	s.dropBalls()
	return s
}

func (s *sim) dropBalls() {
	for i := range s.ballBodies {
		radius := s.ballShapes[i].(chipmunk.CircleShape).Radius()
		s.ballBodies[i].SetPosition(chipmunk.Vect{radius + rand.Float64()*(size-2*radius),
			0.5*size + rand.Float64()*(0.5*size-radius)})
	}
}

type Phys struct {
	clients     map[gordian.ClientId]clientData
	simTimer    <-chan time.Time
	updateTimer <-chan time.Time
	curId       int
	sim         *sim
	*gordian.Gordian
}

func NewPhys() *Phys {
	p := &Phys{
		clients:     map[gordian.ClientId]clientData{},
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
				body := chipmunk.BodyNew(math.Inf(0), math.Inf(0))
				p.clients[client.Id] = clientData{body: body}
				p.Control <- client
			case gordian.CLOSE:
				c, ok := p.clients[client.Id]
				if !ok {
					break
				}
				if c.joint != nil {
					p.sim.space.RemoveConstraint(c.joint)
					c.joint.Free()
				}
				c.body.Free()
				delete(p.clients, client.Id)
			}
		case msg = <-p.InMessage:
			id := msg.From
			c, ok := p.clients[id]
			if !ok {
				break
			}
			rawData = msg.Data.(map[string]interface{})
			data := rawData["data"].(map[string]interface{})
			rawPos := data["pos"].(map[string]interface{})
			btn := data["btn"].(bool)
			pos := chipmunk.Vect{rawPos["x"].(float64), rawPos["y"].(float64)}
			c.body.SetPosition(pos)
			isDragging := (c.joint != nil)

			switch {
			case !isDragging && btn:
				shape := p.sim.space.PointQueryFirst(pos, 1, chipmunk.NoGroup)
				if shape != nil {
					c.joint = chipmunk.PivotJointNew2(c.body, shape.Body(),
						chipmunk.Origin(), chipmunk.Origin())
					c.joint.SetMaxForce(1000.0)
					p.sim.space.AddConstraint(c.joint)
				}
			case isDragging && !btn:
				if c.joint != nil {
					p.sim.space.RemoveConstraint(c.joint)
					c.joint.Free()
					c.joint = nil
				}
			}
			p.clients[id] = c
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
			for _, client := range p.clients {
				cursors = append(cursors, client.body.Position())
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

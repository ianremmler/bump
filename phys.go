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

type Phys struct {
	clients     map[gordian.ClientId]clientData
	simTimer    <-chan time.Time
	updateTimer <-chan time.Time
	curId       int
	space       *chipmunk.Space
	ballBodies  []chipmunk.Body
	ballShapes  []chipmunk.Shape
	box         []chipmunk.Shape
	*gordian.Gordian
}

func NewPhys() *Phys {
	p := &Phys{
		clients:     map[gordian.ClientId]clientData{},
		simTimer:    time.Tick(simTime),
		updateTimer: time.Tick(updateTime),
		Gordian:     gordian.New(0),
	}
	p.setup()
	return p
}

func (p *Phys) setup() {
	p.ballBodies = make([]chipmunk.Body, 42)
	p.ballShapes = make([]chipmunk.Shape, len(p.ballBodies))
	p.box = make([]chipmunk.Shape, 4)
	p.space = chipmunk.SpaceNew()
	p.space.SetGravity(chipmunk.Vect{0, -100})

	radius, mass := 10.0, 1.0
	moment := chipmunk.MomentForCircle(mass, 0, radius, chipmunk.Origin())

	for i := range p.ballBodies {
		body := chipmunk.BodyNew(mass, moment)
		p.space.AddBody(body)
		p.ballBodies[i] = body
		shape := chipmunk.CircleShapeNew(body, radius, chipmunk.Origin())
		shape.SetFriction(0.5)
		shape.SetElasticity(0.9)
		shape.SetLayers(1)
		p.space.AddShape(shape)
		p.ballShapes[i] = shape
	}
	pts := []chipmunk.Vect{
		{-radius, -radius},
		{size + radius, -radius},
		{size + radius, size + radius},
		{-radius, size + radius},
	}
	for i := range p.box {
		shape := chipmunk.SegmentShapeNew(p.space.StaticBody(), pts[i],
			pts[(i+1)%len(pts)], radius)
		shape.SetElasticity(1.0)
		shape.SetFriction(1.0)
		p.space.AddShape(shape)
		p.box[i] = shape
	}
}

func (p *Phys) dropBalls() {
	for i := range p.ballBodies {
		radius := p.ballShapes[i].(chipmunk.CircleShape).Radius()
		p.ballBodies[i].SetPosition(chipmunk.Vect{0.5*size + (rand.Float64()*2-1)*radius,
			0.5*size + rand.Float64()*(0.5*size-2*radius)})
	}
}

func (p *Phys) Run() {
	go p.run()
	p.dropBalls()
	p.Gordian.Run()
}

func (p *Phys) run() {
	msg := gordian.Message{}
	rawData := map[string]interface{}{}
	balls := make([]ballInfo, len(p.ballBodies))
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
					p.space.RemoveConstraint(c.joint)
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
				shape := p.space.PointQueryFirst(pos, 1, chipmunk.NoGroup)
				if shape != nil {
					c.joint = chipmunk.PivotJointNew2(c.body, shape.Body(),
						chipmunk.Origin(), chipmunk.Origin())
					c.joint.SetMaxForce(10000.0)
					p.space.AddConstraint(c.joint)
				}
			case isDragging && !btn:
				if c.joint != nil {
					p.space.RemoveConstraint(c.joint)
					c.joint.Free()
					c.joint = nil
				}
			}
			p.clients[id] = c
		case <-p.simTimer:
			p.space.Step(float64(simTime) / float64(time.Second))
		case <-p.updateTimer:
			for i, bb := range p.ballBodies {
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

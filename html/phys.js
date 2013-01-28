var size = 500;
var balls = [];
var cursors = [];
var state = {
	pos: {x: 0, y: 0},
	btn: false,
};

var stage = new Kinetic.Stage({
	container: 'container',
	width: size,
	height: size,
	scale: { x: 1, y: -1 },
	offset: { x: 0, y: size }
});

stage.on('mousedown', function(evt) {
	state.btn = true;
});

stage.on('mouseup mouseleave', function(evt) {
	state.btn = false;
});

var layer = new Kinetic.Layer();

var box = new Kinetic.Rect({
	x: 0,
	y: 0,
	width: size,
	height: size,
	stroke: 'black',
	strokeWidth: 2,
});
layer.add(box);

function newBall() {
	var b = new Kinetic.Group();
	var circle = new Kinetic.Circle({
		radius: 10,
		fill: 'red',
		stroke: 'black',
		strokeWidth: 2,
		listening: false,
	});
	var line = new Kinetic.Line({
		points: [0, 0, 0, 10],
		stroke: 'black',
		strokeWidth: 2,
		listening: false,
	});
	b.add(circle);
	b.add(line);
	b.listening = false;
	balls.push(b);
	layer.add(b);
}

var cursorShape = new Kinetic.Shape({
	drawFunc: function(canvas) {
		if (!cursors.length) {
			return;
		}
		var context = canvas.getContext();
		for (var i = 0; i < cursors.length; i++) {
			context.beginPath();
			context.arc(cursors[i].X, cursors[i].Y, 5, 0, 2 * Math.PI, true);
			context.closePath();
			canvas.fillStroke(this);
		}
	},
	fill: 'black',
	stroke: 'black',
	strokeWidth: 2
});
layer.add(cursorShape);

stage.add(layer);

var ws = $.websocket("ws://" + window.location.host + "/phys/", {
	events: {
		message: function(e) {
			for (var i = 0; i < e.data.balls.length; i++) {
				if (i >= balls.length) {
					newBall();
				}
				balls[i].setPosition(e.data.balls[i].Pos.X, e.data.balls[i].Pos.Y);
				balls[i].setRotation(-e.data.balls[i].Angle);
			}
			cursors = e.data.cursors;
		}
	}
});

function anim() {
	requestAnimationFrame(anim);
	stage.draw();
	var pos = stage.getUserPosition();
	if (pos) {
	  state.pos = {x: pos.x, y: size - pos.y - 1};
	}
	ws.send('message', state);
}

anim();

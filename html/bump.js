var players = {};
var state = {
	pos: {x: 0, y: 0},
};

var stage;
var layer;
var config;

function setup(conf) {
	console.log("setup")
	config = conf
	stage = new Kinetic.Stage({
		container: 'container',
		width: 2 * config.ArenaRadius,
		height: 2 * config.ArenaRadius,
		scale: { x: 1, y: -1 },
		offset: { x: -config.ArenaRadius, y: config.ArenaRadius }
	});
	layer = new Kinetic.Layer();

	layer.add(new Kinetic.Circle({
		x: 0,
		y: 0,
		radius: config.ArenaRadius - 1,
		stroke: 'black',
		strokeWidth: 2,
	}));
	stage.add(layer);

	anim();
}

function newPlayer() {
	console.log("newPlayer")
	return new Kinetic.Circle({
		radius: config.PlayerRadius,
		fill: randColor(),
		stroke: 'black',
		strokeWidth: 2,
		listening: false,
	});
}

var ws = new WebSocket("ws://" + window.location.host + "/bump/");
ws.onmessage = handleMessage;

function handleMessage(evt) {
	msg = JSON.parse(evt.data);
	switch (msg.type) {
	case "config":
	   	setup(msg.data);
		break;
	case "state":
		for (var id in msg.data.players) {
			if (!(id in players)) {
				var p = newPlayer();
				players[id] = p;
				layer.add(p);
			}
			var x = msg.data.players[id].Pos.X;
		   	var y = msg.data.players[id].Pos.Y
			players[id].setPosition(x, y);
		}
		for (var id in players) {
			if (!(id in msg.data.players)) {
				stage.remove(players[id]);
				delete players[id];
			}
		}
		var msg = {
			type: 'player',
			data: state,
		};
		ws.send(JSON.stringify(msg));
		break;
	}
}

function anim() {
	console.log("anim")
	requestAnimationFrame(anim);
	stage.draw();
	var pos = stage.getUserPosition();
	if (pos) {
		state.pos = {x: pos.x - config.ArenaRadius, y: config.ArenaRadius - pos.y};
	}
}

function randColor() {
	return '#' + ('00000' + (Math.random() * 16777216 << 0).toString(16)).substr(-6);
}

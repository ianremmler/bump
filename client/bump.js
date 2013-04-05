var players = {};
var state = { Pos: { X: 0, Y: 0 } };
var stage;
var layer;
var config;
var stateColor = ['green', 'yellow', 'red', 'blue'];

function setup(conf) {
	config = conf;
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
		fill: 'lightgray',
		stroke: 'black',
		strokeWidth: 2
	}));
	layer.add(new Kinetic.Circle({
		x: 0,
		y: 0,
		radius: config.PlayerRadius,
		fill: 'green'
	}));
	stage.add(layer);

	anim();
}

function newPlayer(team) {
	var color = (team === 0) ? 'black' : 'white';
	var player = new Kinetic.Circle({
		radius: config.PlayerRadius,
		fill: color,
		strokeWidth: 4
	});
	return player;
}

var ws = new WebSocket("ws://" + window.location.host + "/bump/");
ws.onmessage = function(evt) {
	msg = JSON.parse(evt.data);
	switch (msg.type) {
	case "config":
		setup(msg.data);
		break;
	case "state":
		updatePlayers(msg.data);
		sendState();
		break;
	default:
		break;
	}
};

function updatePlayers(curPlayers) {
	for (var id in curPlayers) {
		if (!(id in players)) {
			var p = newPlayer(curPlayers[id].Team);
			players[id] = p;
			layer.add(p);
		}
		var x = curPlayers[id].Pos.X;
		var y = curPlayers[id].Pos.Y;
		players[id].setPosition(x, y);
		players[id].setStroke(stateColor[curPlayers[id].State]);
	}
	for (id in players) {
		if (!(id in curPlayers)) {
			players[id].remove();
			delete players[id];
		}
	}
}

function sendState() {
	var msg = {
		type: 'player',
		data: state
	};
	ws.send(JSON.stringify(msg));
}

function anim() {
	requestAnimationFrame(anim);
	stage.draw();
	var pos = stage.getPointerPosition();
	if (pos) {
		state.Pos = { X: pos.x - config.ArenaRadius, Y: config.ArenaRadius - pos.y };
	}
}

var players = {};
var state = { Pos: { X: 0, Y: 0 } };
var stage;
var layer;
var me;
var config;
var scoreboard = [];
var stateColor = ['green', 'yellow', 'red', 'blue'];
var teamColor = ['black', 'white'];

function setup(conf) {
	config = conf;
	stage = new Kinetic.Stage({
		container: 'container',
		width: 2 * config.ArenaRadius + 8,
		height: 2 * config.ArenaRadius + 8,
		scale: { x: 1, y: -1 },
		offset: { x: -config.ArenaRadius - 4, y: config.ArenaRadius + 4 }
	});
	layer = new Kinetic.Layer();

	layer.add(new Kinetic.Circle({
		x: 0,
		y: 0,
		radius: config.ArenaRadius - 1,
		fill: 'lightgray',
		stroke: 'black',
		strokeWidth: 4
	}));

	layer.add(new Kinetic.Circle({
		x: 0,
		y: 0,
		radius: 2 * config.PlayerRadius,
		fill: 'green'
	}));

	me = new Kinetic.Circle({
		x: 0,
		y: 0,
		radius: 6,
		fill: 'gray'
	});
	layer.add(me);

	for (var i = 0; i < 2; i++) {
		var text = new Kinetic.Text({
			fontSize: 72,
			fontFamily: 'monospace',
			x: (config.ArenaRadius - 50) * (2 * i - 1) - 50,
			y: -config.ArenaRadius + 100,
			width: 100,
			height: 200,
			text: '0',
			align: 'center',
			stroke: 'gray',
			fill: teamColor[i],
			scale: { x: 1, y: -1 }
		});
		scoreboard.push(text);
		layer.add(text);
	}

	stage.add(layer);
	anim();
}

function newPlayer(team) {
	var color = teamColor[team];
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
		updatePlayers(msg.data.Players);
		updateScore(msg.data.Score);
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
		if (id == config.Id) {
			me.setPosition(x, y);
			me.moveToTop();
		}
	}
	for (id in players) {
		if (!(id in curPlayers)) {
			players[id].remove();
			delete players[id];
		}
	}
}

function updateScore(score) {
	for (var idx in score) {
		scoreboard[idx].setText(score[idx]);
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

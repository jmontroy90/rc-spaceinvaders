package main

import "time"

const (
	wall  = '\U00002593'
	enemy = '\U000025C8'
	//cursor = '\U000021EA'
	cursor = '\U0001F726'
)

type objectType int

const (
	computer objectType = iota
	player
	env
)

type movement struct {
	deltas         []xy
	freq           time.Duration
	lastMoved      time.Duration // num frames since start
	prevDeltaIndex int           // index of delta used for the previous move
}

type object struct {
	repr       rune // Representation
	pos        xy
	mvmt       movement
	objectType objectType
}

func newEnemy(pos xy) object {
	deltas := []xy{{0, 1}, {1, 1}, {-1, 1}, {-1, 1}, {1, 1}}
	return object{
		repr: enemy,
		pos:  pos,
		mvmt: movement{
			deltas:         deltas,
			freq:           1 * time.Second,
			prevDeltaIndex: len(deltas) - 1, // we start at the end so the next delta is the first
		},
		objectType: computer,
	}
}

func newWall(pos xy) object {
	return object{repr: wall, pos: pos, objectType: env}
}

func newCursor(pos xy) object {
	return object{repr: cursor, pos: pos, objectType: player}
}

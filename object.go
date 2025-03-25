package main

import (
	"time"
)

const (
	wall   = '\U00002593'
	enemy  = '\U000025C8'
	cursor = '\U0001F726'
	//explosion = 'X'
	explosion = '\U000026CC'
	bullet    = '.'
)

type objectType int

const (
	computer objectType = iota
	player
	env
)

type movement struct {
	steps         []xy
	freq          time.Duration
	lastMoved     time.Duration // num frames since start
	prevStepIndex int           // index of delta used for the previous move
}

type expiry struct {
	expiresAfter time.Duration
	createdAt    time.Duration
}

type object struct {
	name       string
	objectType objectType
	repr       rune // Representation
	pos        xy

	isCursor   bool
	cursorMvmt *xy
	hasMvmt    bool
	mvmt       movement
	hasExpiry  bool
	expiry     expiry
}

func (o object) stepObject() object {
	if !o.hasMvmt {
		return o
	}
	prevIndex := (o.mvmt.prevStepIndex + 1) % len(o.mvmt.steps)
	o.mvmt.prevStepIndex = prevIndex
	delta := o.mvmt.steps[prevIndex]
	o.mvmt.lastMoved += o.mvmt.freq
	o.pos = addPos(o.pos, delta)
	return o
}

func (o object) isExpired(clock time.Duration) bool {
	return o.hasExpiry && clock >= o.expiry.createdAt+o.expiry.expiresAfter
}

func (o object) isSteppable(clock time.Duration) bool {
	return clock >= o.mvmt.lastMoved+o.mvmt.freq
}

func newEnemy(pos xy) object {
	deltas := []xy{{0, 1}, {1, 1}, {-1, 1}, {-1, 1}, {1, 1}}
	return object{
		name:    "enemy",
		repr:    enemy,
		pos:     pos,
		hasMvmt: true,
		mvmt: movement{
			steps:         deltas,
			freq:          1 * time.Second,
			prevStepIndex: len(deltas) - 1, // we start at the end so the next delta is the first
		},
		objectType: computer,
	}
}

func newWall(pos xy) object {
	return object{name: "wall", repr: wall, pos: pos, objectType: env}
}

func newCursor(pos xy) object {
	return object{
		name:       "cursor",
		repr:       cursor,
		pos:        pos,
		objectType: player,
		isCursor:   true,
	}
}

func newExplosion(pos xy, createdAt time.Duration) object {
	return object{
		name:       "explosion",
		repr:       explosion,
		pos:        pos,
		objectType: computer,
		hasMvmt:    false,
		hasExpiry:  true,
		expiry:     expiry{expiresAfter: 500 * time.Millisecond, createdAt: createdAt},
	}
}

func newBullet(pos xy, clock time.Duration) object {
	return object{
		name:       "bullet",
		repr:       bullet,
		pos:        pos,
		objectType: computer,
		hasMvmt:    true,
		mvmt: movement{
			steps:     []xy{{0, -1}},
			freq:      50 * time.Millisecond,
			lastMoved: clock,
		},
	}
}

type xy struct {
	x, y int
}

func addPos(this, that xy) xy {
	return xy{x: this.x + that.x, y: this.y + that.y}
}

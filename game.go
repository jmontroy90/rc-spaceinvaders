package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

const (
	defaultFrameRate = 25 * time.Millisecond
	defaultWidth     = 50
	defaultHeight    = 30
)

type game struct {
	objs  map[xy]object
	objMu sync.RWMutex
	// TODO: made it its own thing to avoid concurrency issues with the map; necessary?
	curs  object
	clock time.Duration
	conf  config
}

type config struct {
	width, height int
	frameRate     time.Duration
	cursorPos     xy
}

func newDefaultConfig() config {
	return config{
		width:     defaultWidth,
		height:    defaultHeight,
		frameRate: defaultFrameRate,
		cursorPos: xy{defaultWidth / 2, defaultHeight - 3},
	}
}

func newGame(conf config) *game {
	b := game{
		objs: make(map[xy]object),
		curs: newCursor(conf.cursorPos),
		conf: conf,
	}
	// top + bottom walls
	for x := range conf.width {
		b.addObject(newWall(xy{x, 0}))
		b.addObject(newWall(xy{x, conf.height - 1}))
	}
	// left + right walls
	for y := range conf.height {
		b.addObject(newWall(xy{0, y}))
		b.addObject(newWall(xy{conf.width - 1, y}))
	}
	return &b
}

func (b *game) addObject(obj object) {
	b.objMu.Lock()
	defer b.objMu.Unlock()
	b.objs[obj.pos] = obj
}

func (b *game) render() {
	clearScreen()

	fmt.Printf(instructions())
	for y := range b.conf.height {
		fmt.Printf(padding())
		for x := range b.conf.width {
			if obj, ok := b.find(xy{x, y}); ok {
				fmt.Printf("%c", obj.repr)
			} else {
				fmt.Printf(" ")
			}
		}
		fmt.Printf("\r\n") // raw mode requires manual carraige returns
	}
}

func padding() string {
	return "          "
}

func instructions() string {
	return fmt.Sprintf(
		"\n" + padding() + "- Press 'q' to quit.\r\n" +
			padding() + "- 'wasd' to move up/left/down/right.\r\n\n",
	)
}

func (b *game) find(pos xy) (*object, bool) {
	b.objMu.RLock()
	defer b.objMu.RUnlock()
	if obj, ok := b.objs[pos]; ok {
		return &obj, true
	} else if pos == b.curs.pos {
		return &b.curs, true
	}
	return nil, false
}

func (b *game) mvmtLoop(renderCh chan<- struct{}, exitCh <-chan struct{}) {
	for {
		select {
		case <-exitCh:
			return
		default:
			b.clock += b.conf.frameRate
			// TODO: your frame rate calculation would have to factor in how long this part below takes.
			if shouldRender := b.parseMovement(); shouldRender {
				renderCh <- struct{}{}
			}
			time.Sleep(b.conf.frameRate)
		}
	}
}

func (b *game) parseMovement() bool {
	b.objMu.Lock()
	defer b.objMu.Unlock()
	var shouldRender bool
	for pos, obj := range b.objs {
		if obj.objectType != computer {
			continue
		}
		if b.clock >= obj.mvmt.lastMoved+obj.mvmt.freq {
			// check if we can move this object yet
			prevIndex := (obj.mvmt.prevDeltaIndex + 1) % len(obj.mvmt.deltas)
			delta := obj.mvmt.deltas[prevIndex]
			newPos := addPos(pos, delta)
			if _, ok := b.objs[newPos]; ok {
				continue
			}
			// We can move it!
			delete(b.objs, pos) // remove original
			obj.mvmt.lastMoved += obj.mvmt.freq
			obj.pos = newPos
			obj.mvmt.prevDeltaIndex = prevIndex
			b.objs[obj.pos] = obj
			shouldRender = true
		}
	}
	return shouldRender
}

func (b *game) userInputLoop(renderCh, exitCh chan<- struct{}) {
	reader := bufio.NewReader(os.Stdin)
	for {
		input, _, err := reader.ReadRune()
		if err != nil {
			log.Fatalf("Error reading input: %v", err)
		}
		var delta *xy
		switch input {
		case 'q', 'Q':
			close(exitCh) // broadcasts to all listeners
			return
		case 'w', 'W':
			delta = &xy{0, -1}
		case 'a', 'A':
			delta = &xy{-1, 0}
		case 's', 'S':
			delta = &xy{0, 1}
		case 'd', 'D':
			delta = &xy{1, 0}
		}
		if delta != nil {
			b.curs.pos = addPos(b.curs.pos, *delta)
			renderCh <- struct{}{}
		}
	}
}

type xy struct {
	x, y int
}

func addPos(this, that xy) xy {
	return xy{x: this.x + that.x, y: this.y + that.y}
}

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
	defaultWidth     = 30
	defaultHeight    = 20
)

type game struct {
	objs      map[xy]object
	objMu     sync.RWMutex
	cursorPos xy
	clock     time.Duration
	conf      config
	gameOver  bool
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
		objs:      make(map[xy]object),
		cursorPos: conf.cursorPos,
		conf:      conf,
	}
	b.addObject(newCursor(conf.cursorPos))
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
		fmt.Printf("\r\n") // raw mode requires manual carriage returns
	}
}

func (b *game) objectLoop(renderCh chan<- struct{}, inputCh <-chan xy, exitCh chan struct{}) {
	for {
		select {
		case <-exitCh:
			return
		case step := <-inputCh:
			c, ok := b.objs[b.cursorPos]
			if !ok {
				log.Fatalf("Couldn't find cursor!")
			}
			// TODO: this will be picked up later; is this silly?
			c.cursorMvmt = &step
			b.addObject(c)
		default:
			if b.gameOver {
				renderCh <- struct{}{}
				fmt.Printf("\r\n\n" + padding() + "Game over!")
				time.Sleep(1 * time.Second)
				close(exitCh)
				return
			}
			// TODO: your frame rate calculation would have to factor in how long this part below takes.
			if shouldRender := b.handleObjects(); shouldRender {
				renderCh <- struct{}{}
			}
			time.Sleep(b.conf.frameRate)
			b.clock += b.conf.frameRate
		}
	}
}

func (b *game) inputLoop(inputCh chan<- xy, exitCh chan struct{}) {
	reader := bufio.NewReader(os.Stdin)
	for {
		select {
		case <-exitCh: // TODO: multiple goroutines write this, is that bad?
			return
		default:
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
				inputCh <- *delta
			}
		}
	}
}

func (b *game) handleObjects() bool {
	var shouldRender bool
	for _, orig := range b.objs {
		if updated := b.handleObject(orig); updated {
			shouldRender = true
		}
	}
	return shouldRender
}

func (b *game) handleObject(orig object) (render bool) {
	switch {
	case orig.hasMvmt && orig.isSteppable(b.clock):
		updated := orig.stepObject()
		if found, ok := b.objs[updated.pos]; ok {
			b.handleInteraction(orig, found)
		} else {
			b.removeAt(orig.pos)
			b.addObject(updated)
		}
		render = true
	case orig.hasExpiry && orig.isExpired(b.clock):
		b.removeAt(orig.pos) // at this point in the loop it's time for the explosion to go away
		render = true
	case orig.isCursor && orig.cursorMvmt != nil:
		step := *orig.cursorMvmt
		newPos := addPos(b.cursorPos, step)
		if found, ok := b.objs[newPos]; ok {
			b.handleInteraction(orig, found)
		} else {
			b.removeAt(b.cursorPos)
			orig.pos = newPos
			b.cursorPos = newPos
			orig.cursorMvmt = nil
			b.addObject(orig)
		}
		render = true
	}
	return render
}

func (b *game) handleInteraction(orig, found object) {
	// TODO: this feels smelly, the object abstraction is leaky
	switch {
	case orig.objectType == player && found.objectType == computer:
		fallthrough
	case orig.objectType == computer && found.objectType == player:
		b.removeAt(orig.pos)
		b.removeAt(found.pos)
		b.addObject(newExplosion(found.pos, b.clock))
		b.gameOver = true
	// Note we don't need to invert this case currently, because env objects never move (just walls)!
	case orig.objectType == player && found.objectType == env:
		b.removeAt(orig.pos)
		b.addObject(newExplosion(orig.pos, b.clock))
		b.gameOver = true
	}
}

func (b *game) find(pos xy) (*object, bool) {
	b.objMu.RLock()
	defer b.objMu.RUnlock()
	if obj, ok := b.objs[pos]; ok {
		return &obj, true
	}
	return nil, false
}

func (b *game) removeAt(pos xy) {
	b.objMu.Lock()
	defer b.objMu.Unlock()
	delete(b.objs, pos)
}

func (b *game) addObject(obj object) {
	b.objMu.Lock()
	defer b.objMu.Unlock()
	b.objs[obj.pos] = obj
}

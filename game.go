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
	score     int
}

type config struct {
	width, height   int
	frameRate       time.Duration
	cursorPos       xy
	startNumEnemies int
}

func newDefaultConfig() config {
	return config{
		width:           defaultWidth,
		height:          defaultHeight,
		frameRate:       defaultFrameRate,
		cursorPos:       xy{defaultWidth / 2, defaultHeight - 3},
		startNumEnemies: 15,
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

func (g *game) render() {
	clearScreen()
	fmt.Printf(instructions())
	for y := range g.conf.height {
		fmt.Printf(padding())
		for x := range g.conf.width {
			if obj, ok := g.find(xy{x, y}); ok {
				fmt.Printf("%c", obj.repr)
			} else {
				fmt.Printf(" ")
			}
		}
		fmt.Printf("\r\n") // raw mode requires manual carriage returns
	}
}

func (g *game) objectLoop(renderCh chan<- struct{}, inputCh <-chan rune, exitCh chan struct{}) {
	for {
		select {
		case <-exitCh:
			return
		case input := <-inputCh:
			c, ok := g.objs[g.cursorPos]
			if !ok {
				log.Fatalf("Couldn't find cursor!")
			}
			switch input {
			case 'q', 'Q':
				close(exitCh) // broadcasts to all listeners
				return
			case 'w', 'W':
				c.cursorMvmt = &xy{0, -1}
				g.addObject(c)
			case 'a', 'A':
				c.cursorMvmt = &xy{-1, 0}
				g.addObject(c)
			case 's', 'S':
				c.cursorMvmt = &xy{0, 1}
				g.addObject(c)
			case 'd', 'D':
				c.cursorMvmt = &xy{1, 0}
				g.addObject(c)
			case ' ':
				g.addObject(newBullet(addPos(g.cursorPos, xy{0, -1}), g.clock))
			}
		default:
			if g.gameOver {
				renderCh <- struct{}{}
				fmt.Printf("\r\n\n" + padding() + "Game over!")
				fmt.Printf("\r\n\n"+padding()+"Score: %v", g.score)
				time.Sleep(1 * time.Second)
				close(exitCh)
				return
			}
			// TODO: your frame rate calculation would have to factor in how long this part below takes.
			if shouldRender := g.handleObjects(); shouldRender {
				renderCh <- struct{}{}
			}
			time.Sleep(g.conf.frameRate)
			g.clock += g.conf.frameRate
		}
	}
}

func (g *game) inputLoop(inputCh chan<- rune, exitCh chan struct{}) {
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
			inputCh <- input
		}
	}
}

func (g *game) handleObjects() bool {
	var shouldRender bool
	for _, orig := range g.objs {
		if updated := g.handleObject(orig); updated {
			shouldRender = true
		}
	}
	return shouldRender
}

func (g *game) handleObject(orig object) (render bool) {
	switch {
	case orig.hasMvmt && orig.isSteppable(g.clock):
		updated := orig.stepObject()
		if found, ok := g.objs[updated.pos]; ok {
			g.handleInteraction(orig, found)
		} else {
			g.removeAt(orig.pos)
			g.addObject(updated)
		}
		render = true
	case orig.hasExpiry && orig.isExpired(g.clock):
		g.removeAt(orig.pos) // at this point in the loop it's time for the explosion to go away
		render = true
	case orig.isCursor && orig.cursorMvmt != nil:
		step := *orig.cursorMvmt
		newPos := addPos(g.cursorPos, step)
		if found, ok := g.objs[newPos]; ok {
			g.handleInteraction(orig, found)
		} else {
			g.removeAt(g.cursorPos)
			orig.pos = newPos
			g.cursorPos = newPos
			orig.cursorMvmt = nil
			g.addObject(orig)
		}
		render = true
	}
	return render
}

func (g *game) handleInteraction(orig, found object) {
	// TODO: this feels smelly, the object abstraction is leaky
	switch {
	case orig.objectType == player && found.objectType == computer:
		fallthrough
	case orig.objectType == computer && found.objectType == player:
		g.removeAt(orig.pos)
		g.removeAt(found.pos)
		g.addObject(newExplosion(found.pos, g.clock))
		g.gameOver = true
	// Note we don't need to invert this case currently, because env objects never move (just walls)!
	case orig.objectType == player && found.objectType == env:
		g.removeAt(orig.pos)
		g.addObject(newExplosion(orig.pos, g.clock))
		g.gameOver = true
	case orig.name == "enemy" && found.name == "bullet":
		fallthrough
	case orig.name == "bullet" && found.name == "enemy":
		g.removeAt(orig.pos)
		g.removeAt(found.pos)
		g.addObject(newExplosion(found.pos, g.clock))
		g.score++
	}
}

func (g *game) find(pos xy) (*object, bool) {
	g.objMu.RLock()
	defer g.objMu.RUnlock()
	if obj, ok := g.objs[pos]; ok {
		return &obj, true
	}
	return nil, false
}

func (g *game) removeAt(pos xy) {
	g.objMu.Lock()
	defer g.objMu.Unlock()
	delete(g.objs, pos)
}

func (g *game) addObject(obj object) {
	g.objMu.Lock()
	defer g.objMu.Unlock()
	g.objs[obj.pos] = obj
}

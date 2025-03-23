package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/term"
)

func main() {
	restoreFn, err := configureTerminal()
	if err != nil {
		log.Fatalf("Error configuring terminal: %v", err)
	}
	defer restoreFn()

	conf := newDefaultConfig()
	g := newGame(conf)
	// enemies
	for i := 20; i < 30; i += 2 {
		g.addObject(newEnemy(xy{i, 2}))
		g.addObject(newEnemy(xy{i + 1, 3}))
		g.addObject(newEnemy(xy{i, 4}))
	}
	g.render() // initial render
	renderCh := make(chan struct{})
	exitCh := make(chan struct{})
	go g.mvmtLoop(renderCh, exitCh)
	go g.userInputLoop(renderCh, exitCh)
	startControlLoop(g, renderCh, exitCh)

}

func startControlLoop(g *game, renderCh, exitCh chan struct{}) {
	for {
		select {
		case <-renderCh:
			g.render()
		case <-exitCh:
			fmt.Printf("\r\n\nExiting...")
			time.Sleep(500 * time.Millisecond)
			return
		}
	}
}

func configureTerminal() (restore func(), err error) {
	fd := int(os.Stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return func() {}, err
	}
	fmt.Print("\033[?25l") // ANSI: makes cursor disappear
	fmt.Print("\033[2J")   // ANSI: clear visible screen
	fmt.Print("\033[3J")   // ANSI: clear visible screen
	// closure to enable restoring original terminal state
	return func() {
		_ = term.Restore(fd, old)
		fmt.Print("\033[?25h") // ANSI: make cursor appear
		fmt.Print("\033[2J")   // ANSI: clear visible screen
		fmt.Print("\033[3J")   // ANSI: clear scrollback
		clearScreen()
	}, nil
}

// I don't totally understand how these ANSI codes interact, but this combo plus above works well.
func clearScreen() {
	fmt.Print("\033[H")  // ANSI: move cursor to top left
	fmt.Print("\033[3J") // ANSI: clear scrollback
}

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/tailscale/walk"
)

// startBusy marks the GUI as busy, shows status and counts seconds in the status
// bar until the returned function is called (on the UI thread).
func (g *gui) startBusy(status string) (stop func()) {
	g.busy = true
	g.busyStatus = status
	g.busyStart = time.Now()
	g.setStatus(status)

	ticker := time.NewTicker(time.Second)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				g.app.Synchronize(func() {
					if !g.busy {
						return
					}
					if g.busyProgress != nil {
						if detail := g.busyProgress(); detail != "" {
							g.setStatus(g.busyStatus + " " + detail)
							return
						}
					}
					g.setStatus(fmt.Sprintf("%s %d seconds", g.busyStatus, int(time.Since(g.busyStart).Seconds())))
				})
			case <-done:
				return
			}
		}
	}()
	return func() {
		ticker.Stop()
		close(done)
		g.busy = false
		g.busyProgress = nil
	}
}

func (g *gui) showBusy() {
	walk.MsgBox(g.mw, "Busy", "Please wait for the current task to finish, or press Escape to cancel it.",
		walk.MsgBoxOK|walk.MsgBoxIconWarning)
}

// runTask runs work against ipatool's engine in the background and calls onDone
// on the UI thread with its result. name is written to the Log page. Escape
// cancels the task's context; a cancelled task's result is discarded.
func (g *gui) runTask(name, status string, work func(ctx context.Context, b *backend) (any, error), onDone func(any, error)) {
	if g.busy {
		g.showBusy()
		return
	}
	passphrase := g.account.passphrase.Text()
	if passphrase == "" {
		g.error("Passphrase needed", "Enter a keychain passphrase on the Account page.")
		g.focusWidget(0, g.account.passphrase)
		return
	}

	g.runLocalTask(name, status, func(ctx context.Context) (any, error) {
		b, err := newBackend(passphrase)
		if err != nil {
			return nil, err
		}
		return work(ctx, b)
	}, onDone)
}

// runLocalTask is runTask for work that doesn't use ipatool's engine or the
// Apple account (like installing on an iPhone), so no passphrase is needed.
func (g *gui) runLocalTask(name, status string, work func(ctx context.Context) (any, error), onDone func(any, error)) {
	if g.busy {
		g.showBusy()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	g.cancelTask = cancel
	g.log("> " + name)
	stopBusy := g.startBusy(status)

	go func() {
		result, err := work(ctx)
		g.app.Synchronize(func() {
			stopBusy()
			cancel()
			g.cancelTask = nil
			if cleanup := g.afterTask; cleanup != nil {
				g.afterTask = nil
				cleanup() // runs even when the task was cancelled
			}
			if g.cancelled {
				g.cancelled = false
				g.log("(cancelled)")
				g.setStatus("Cancelled.")
				return
			}
			if err != nil {
				g.log("error: " + err.Error())
				g.setStatus("Failed.")
			} else {
				g.log("done")
				g.setStatus("Done.")
			}
			onDone(result, err)
		})
	}()
}

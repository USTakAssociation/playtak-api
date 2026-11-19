package session

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/transport"
)

func TestPresenceEndsUpCurrentAfterConcurrentLogouts(t *testing.T) {
	h := newHarness(t, Config{})
	watcher := loginGuestWith(h, "")
	var leavers []*client
	for range 10 {
		leavers = append(leavers, loginGuestWith(h, ""))
	}
	watcher.settle()

	// Everyone else leaves at once. Each departure broadcasts presence, so
	// the broadcasts overlap.
	var wg sync.WaitGroup
	for _, c := range leavers {
		wg.Go(func() { c.send("quit") })
	}
	wg.Wait()
	for _, c := range leavers {
		c.expectClosed(transport.CloseNormal)
		<-c.conn.peer.(*Session).Done()
	}

	// Whatever arrived, the last lists must describe who is really online.
	var lastCount, lastPlayers string
	for {
		select {
		case line := <-watcher.conn.lines:
			switch {
			case strings.HasPrefix(line, "OnlinePlayers "):
				lastPlayers = line
			case strings.HasPrefix(line, "Online "):
				lastCount = line
			}
			continue
		case <-time.After(100 * time.Millisecond):
		}
		break
	}
	if lastCount != "Online 1" || lastPlayers != `OnlinePlayers ["Guest1"]` {
		t.Errorf("last presence: %q, %q", lastCount, lastPlayers)
	}
}

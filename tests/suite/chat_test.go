package suite_test

import (
	"fmt"
	"strings"
	"testing"

	"tak-tests/internal/client"
)

// ---- Shout ----

func TestChatShoutBroadcastToAll(t *testing.T) {
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	name1 := client.LoginGuest(t, c1)
	client.LoginGuest(t, c2)

	c1.Send("Shout hello world")
	c1.DrainUntil(fmt.Sprintf("Shout <%s> hello world", name1))
	c2.DrainUntil(fmt.Sprintf("Shout <%s> hello world", name1))
}

func TestChatShoutIncludesAngleBrackets(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	name := client.LoginGuest(t, c)

	c.Send("Shout test message")
	msg := c.DrainUntil("Shout ")

	// The protocol spec requires angle brackets around the player name
	expected := fmt.Sprintf("Shout <%s>", name)
	if !strings.HasPrefix(msg, expected) {
		t.Errorf("Shout message should have angle brackets around name\n  want prefix: %q\n           got: %q", expected, msg)
	}
}

func TestChatShoutSentToSelf(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	name := client.LoginGuest(t, c)

	c.Send("Shout testing self receipt")
	c.DrainUntil(fmt.Sprintf("Shout <%s> testing self receipt", name))
}

// ---- Tell ----

func TestChatTellDeliveredToTarget(t *testing.T) {
	u1, p1 := testUser(t, 1)
	u2, p2 := testUser(t, 2)

	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1, u1, p1)
	client.LoginUser(t, c2, u2, p2)

	c1.Send(fmt.Sprintf("Tell %s hello there", u2))
	c2.DrainUntil(fmt.Sprintf("Tell <%s> hello there", u1))
}

func TestChatToldConfirmationSentToSender(t *testing.T) {
	u1, p1 := testUser(t, 1)
	u2, p2 := testUser(t, 2)

	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1, u1, p1)
	client.LoginUser(t, c2, u2, p2)

	c1.Send(fmt.Sprintf("Tell %s a private message", u2))
	c1.DrainUntil(fmt.Sprintf("Told <%s>", u2))
}

func TestChatTellAngleBracketsInTold(t *testing.T) {
	u1, p1 := testUser(t, 1)
	u2, p2 := testUser(t, 2)

	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1, u1, p1)
	client.LoginUser(t, c2, u2, p2)

	c1.Send(fmt.Sprintf("Tell %s message", u2))

	told := c1.DrainUntil("Told ")
	if !strings.Contains(told, fmt.Sprintf("<%s>", u2)) {
		t.Errorf("Told message should contain <%s>: %q", u2, told)
	}

	tell := c2.DrainUntil("Tell ")
	if !strings.Contains(tell, fmt.Sprintf("<%s>", u1)) {
		t.Errorf("Tell message should contain <%s>: %q", u1, tell)
	}
}

func TestChatTellToNonExistentPlayer(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.DrainUntil("OnlinePlayers")
	c.Send("Tell doesnotexist9999 hello")
	c.Expect("No such player")
}

func TestChatTellToOfflinePlayerStillSendsTold(t *testing.T) {
	// According to the protocol: "You'll receive Told even if player is not logged in"
	u1, p1 := testUser(t, 1)
	u2, _ := testUser(t, 2)

	c1 := client.New(t, telnetAddr(t))
	client.LoginUser(t, c1, u1, p1)
	// u2 is not logged in

	c1.Send(fmt.Sprintf("Tell %s offline message", u2))
	c1.DrainUntil(fmt.Sprintf("Told <%s>", u2))
}

// ---- Rooms ----

func TestChatJoinRoom(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.Send("JoinRoom testroom")
	c.DrainUntil("Joined room testroom")
}

func TestChatLeaveRoom(t *testing.T) {
	c := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c)
	c.Send("JoinRoom testroom")
	c.DrainUntil("Joined room testroom")
	c.Send("LeaveRoom testroom")
	c.DrainUntil("OK")
}

func TestChatShoutRoomDeliveredToMembers(t *testing.T) {
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	c3 := client.New(t, telnetAddr(t))

	name1 := client.LoginGuest(t, c1)
	client.LoginGuest(t, c2)
	client.LoginGuest(t, c3)

	const room = "testchatroom"

	c1.Send("JoinRoom " + room)
	c1.DrainUntil("Joined room " + room)
	c2.Send("JoinRoom " + room)
	c2.DrainUntil("Joined room " + room)
	// c3 does NOT join

	c1.Send(fmt.Sprintf("ShoutRoom %s hello room", room))

	// c2 (in room) should receive it
	c2.DrainUntil(fmt.Sprintf("ShoutRoom %s <%s> hello room", room, name1))

	// c3 (not in room) must not receive a ShoutRoom — but may get OnlinePlayers etc.
	c3.ExpectNoMessageWithPrefix("ShoutRoom", client.NoMessageWindow)
}

func TestChatShoutRoomNotDeliveredToNonMembers(t *testing.T) {
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	client.LoginGuest(t, c1)
	client.LoginGuest(t, c2)

	c1.Send("JoinRoom exclusiveroom")
	c1.DrainUntil("Joined room exclusiveroom")
	// c2 does not join

	c1.Send("ShoutRoom exclusiveroom secret message")
	c2.ExpectNoMessageWithPrefix("ShoutRoom", client.NoMessageWindow)
}

func TestChatShoutRoomAfterLeave(t *testing.T) {
	c1 := client.New(t, telnetAddr(t))
	c2 := client.New(t, telnetAddr(t))
	name1 := client.LoginGuest(t, c1)
	client.LoginGuest(t, c2)

	c2.Send("JoinRoom leavetest")
	c2.DrainUntil("Joined room leavetest")
	c1.Send("JoinRoom leavetest")
	c1.DrainUntil("Joined room leavetest")

	// c2 leaves
	c2.Send("LeaveRoom leavetest")
	c2.DrainUntil("OK")

	// c1 shouts to the room — c2 should NOT receive a ShoutRoom
	c1.Send("ShoutRoom leavetest bye")
	c2.ExpectNoMessageWithPrefix("ShoutRoom", client.NoMessageWindow)

	// But c1 should still receive its own message
	c1.DrainUntil(fmt.Sprintf("ShoutRoom leavetest <%s> bye", name1))
}

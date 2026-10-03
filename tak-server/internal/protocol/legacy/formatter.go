package legacy

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

// Format renders an event as the lines sent to a client using protocol
// version (from the client's "Protocol" command, 0 if it never sent one).
func Format(ev protocol.Event, version int) ([]string, error) {
	switch ev := ev.(type) {
	case protocol.Greeting:
		return []string{"Welcome!", "Login or Register"}, nil
	case protocol.OK:
		return line("OK")
	case protocol.NOK:
		return line("NOK")
	case protocol.LoggedIn:
		return line("Welcome " + ev.Name + "!")
	case protocol.IsMod:
		return line("Is Mod")
	case protocol.AuthFailure:
		return line("Authentication failure")
	case protocol.OnlineCount:
		return line("Online " + strconv.Itoa(ev.Count))
	case protocol.OnlinePlayers:
		return line("OnlinePlayers " + quotedList(ev.Names))
	case protocol.ServerMessage:
		return line("Message " + ev.Text)
	case protocol.Shout:
		return line("Shout <" + ev.From + "> " + ev.Text)
	case protocol.Registered:
		return line("Registered " + ev.Name + ". Check your email for password")
	case protocol.RegistrationFailed:
		msg, ok := registrationErrors[ev.Reason]
		if !ok {
			return nil, fmt.Errorf("legacy: unknown registration error %d", ev.Reason)
		}
		return line("Registration Error: " + msg)
	case protocol.ResetTokenSent:
		return line("Reset token sent")
	case protocol.ResetTokenFailed:
		msg, ok := resetTokenErrors[ev.Reason]
		if !ok {
			return nil, fmt.Errorf("legacy: unknown reset token error %d", ev.Reason)
		}
		return line("Reset Token Error: " + msg)
	case protocol.PasswordReset:
		return line("Password is changed")
	case protocol.PasswordResetFailed:
		switch ev.Reason {
		case protocol.ResetPasswordNoSuchPlayer:
			return line("No such player")
		case protocol.ResetPasswordWrongToken:
			return line("Wrong token")
		}
		return nil, fmt.Errorf("legacy: unknown reset password error %d", ev.Reason)
	case protocol.PasswordChanged:
		return line("Password changed")
	case protocol.WrongPassword:
		return line("Wrong password")
	case protocol.SeekAdded:
		return line("Seek new " + formatSeek(ev.Seek, version))
	case protocol.SeekRemoved:
		return line("Seek remove " + formatSeek(ev.Seek, version))
	case protocol.AcceptRematch:
		return line("Accept Rematch " + strconv.Itoa(ev.SeekID))
	case protocol.RematchSeekCreated:
		return line("Rematch seek created with ID: " + strconv.Itoa(ev.SeekID))
	case protocol.GameStarted:
		return line(formatGameStart(ev, version))
	case protocol.MovePlayed:
		return line(fmt.Sprintf("Game#%d %s", ev.GameID, ev.Move))
	case protocol.GameTime:
		return line(formatTime(ev, version))
	case protocol.GameOver:
		return line(fmt.Sprintf("Game#%d Over %s", ev.GameID, ev.Result))
	case protocol.GameAbandoned:
		return line(fmt.Sprintf("Game#%d Abandoned. %s quit", ev.GameID, ev.Quitter))
	case protocol.MoveRejected:
		return []string{"NOK", "Error:" + ev.Reason}, nil
	case protocol.GameListAdded:
		return line("GameList Add " + formatGameListEntry(ev.Game, version))
	case protocol.GameListRemoved:
		return line("GameList Remove " + formatGameListEntry(ev.Game, version))
	case protocol.PositionShown:
		return line(fmt.Sprintf("Game#%d Show %s", ev.GameID, ev.TPS))
	case protocol.SquareShown:
		return line(fmt.Sprintf("Game#%d Show Sq %s", ev.GameID, ev.Stack))
	case protocol.UndoRequested:
		return line(fmt.Sprintf("Game#%d RequestUndo", ev.GameID))
	case protocol.UndoRemoved:
		return line(fmt.Sprintf("Game#%d RemoveUndo", ev.GameID))
	case protocol.Undone:
		return line(fmt.Sprintf("Game#%d Undo", ev.GameID))
	case protocol.DrawOffered:
		return line(fmt.Sprintf("Game#%d OfferDraw", ev.GameID))
	case protocol.DrawRemoved:
		return line(fmt.Sprintf("Game#%d RemoveDraw", ev.GameID))
	case protocol.GameObserved:
		return line("Observe " + formatGameListEntry(ev.Game, version))
	case protocol.TimeGiven:
		color := "white"
		if ev.To == protocol.Black {
			color = "black"
		}
		return line(fmt.Sprintf("Game#%d GivenTime %s %d", ev.GameID, color, ev.Millis))
	}
	return nil, fmt.Errorf("legacy: no format for %T", ev)
}

func line(s string) ([]string, error) { return []string{s}, nil }

var registrationErrors = map[protocol.RegistrationError]string{
	protocol.RegistrationMalformed:   "Unknown format for username/email. Only [a-z][A-Z][0-9][_] allowed for username, it should be 4-16 characters and should start with letter",
	protocol.RegistrationGuestName:   "Can't register with guest in the name",
	protocol.RegistrationProfanity:   "Username cannot contain profanity",
	protocol.RegistrationNameTaken:   "Username is already taken",
	protocol.RegistrationServerError: "Server error, please try again later",
}

var resetTokenErrors = map[protocol.ResetTokenError]string{
	protocol.ResetTokenNoSuchPlayer:  "No such player",
	protocol.ResetTokenEmailMismatch: "Email address does not match",
}

// quotedList renders names the way Java prints a list of pre-quoted strings:
// ["a", "b"].
func quotedList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = `"` + n + `"`
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

package legacy

import (
	"reflect"
	"strings"
	"testing"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

func TestParseLoggedOut(t *testing.T) {
	for _, tc := range []struct {
		line string
		want protocol.Command
	}{
		{"quit", protocol.Quit{}},
		{"PING", protocol.Ping{}},
		{"PING\r\n", protocol.Ping{}},
		{"PING extra", nil},
		{"Client TakWeb-16.05.26", protocol.ClientName{Name: "TakWeb-16.05.26"}},
		{"Protocol 4", protocol.SetProtocol{Version: 4}},
		{"Protocol 12345678", protocol.SetProtocol{Version: 12345678}},
		{"Login Guest", protocol.LoginGuest{}},
		{"Login Guest abcdefghijklmnopqrst", protocol.LoginGuest{Token: "abcdefghijklmnopqrst"}},
		// Prefix matching: a bad token or even "Guestfoo" still logs in as a new guest.
		{"Login Guest ABC", protocol.LoginGuest{}},
		{"Login Guestfoo password", protocol.LoginGuest{}},
		{"Login alice secret123", protocol.Login{Name: "alice", Password: "secret123"}},
		{"Login ab secret123", nil}, // name too short
		{"Login alice short", nil},  // password too short
		// Passwords over 50 characters are cut to the first 50.
		{"Login alice " + strings.Repeat("p", 60), protocol.Login{Name: "alice", Password: strings.Repeat("p", 50)}},
		{"Register bobby b@example.com", protocol.Register{Name: "bobby", Email: "b@example.com"}},
		{"Register justausername", protocol.RegisterMalformed{}},
		{"SendResetToken alice a@example.com", protocol.SendResetToken{Name: "alice", Email: "a@example.com"}},
		{"ResetPassword alice tok123 newpass1", protocol.ResetPassword{Name: "alice", Token: "tok123", Password: "newpass1"}},
		{"ChangePassword old123 new123", nil}, // only after login
		{"nonsense command xyz", nil},
	} {
		if got := Parse(tc.line, false); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Parse(%q, false) = %#v, want %#v", tc.line, got, tc.want)
		}
	}
}

func TestParseLoggedIn(t *testing.T) {
	for _, tc := range []struct {
		line string
		want protocol.Command
	}{
		{"quit", protocol.Quit{}},
		{"PING", protocol.Ping{}},
		{"ChangePassword old123 new123", protocol.ChangePassword{Old: "old123", New: "new123"}},
		{"Login Guest", nil},
		{"Client foo", nil},
		{"nonsense xyz", nil},
	} {
		if got := Parse(tc.line, true); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Parse(%q, true) = %#v, want %#v", tc.line, got, tc.want)
		}
	}
}

func TestFormat(t *testing.T) {
	for _, tc := range []struct {
		ev   protocol.Event
		want []string
	}{
		{protocol.Greeting{}, []string{"Welcome!", "Login or Register"}},
		{protocol.OK{}, []string{"OK"}},
		{protocol.NOK{}, []string{"NOK"}},
		{protocol.LoggedIn{Name: "Guest12"}, []string{"Welcome Guest12!"}},
		{protocol.IsMod{}, []string{"Is Mod"}},
		{protocol.AuthFailure{}, []string{"Authentication failure"}},
		{protocol.OnlineCount{Count: 3}, []string{"Online 3"}},
		{protocol.OnlinePlayers{Names: []string{"alice", "Guest1"}}, []string{`OnlinePlayers ["alice", "Guest1"]`}},
		{protocol.OnlinePlayers{}, []string{"OnlinePlayers []"}},
		{protocol.ServerMessage{Text: "You've logged in from another window. Disconnecting"},
			[]string{"Message You've logged in from another window. Disconnecting"}},
		{protocol.Shout{From: "Server", Text: "hi 🐢"}, []string{"Shout <Server> hi 🐢"}},
		{protocol.Registered{Name: "bob"}, []string{"Registered bob. Check your email for password"}},
		{protocol.RegistrationFailed{Reason: protocol.RegistrationGuestName}, []string{"Registration Error: Can't register with guest in the name"}},
		{protocol.RegistrationFailed{Reason: protocol.RegistrationNameTaken}, []string{"Registration Error: Username is already taken"}},
		{protocol.RegistrationFailed{Reason: protocol.RegistrationMalformed}, []string{"Registration Error: Unknown format for username/email. Only [a-z][A-Z][0-9][_] allowed for username, it should be 4-16 characters and should start with letter"}},
		{protocol.ResetTokenSent{}, []string{"Reset token sent"}},
		{protocol.ResetTokenFailed{Reason: protocol.ResetTokenNoSuchPlayer}, []string{"Reset Token Error: No such player"}},
		{protocol.ResetTokenFailed{Reason: protocol.ResetTokenEmailMismatch}, []string{"Reset Token Error: Email address does not match"}},
		{protocol.PasswordReset{}, []string{"Password is changed"}},
		{protocol.PasswordResetFailed{Reason: protocol.ResetPasswordNoSuchPlayer}, []string{"No such player"}},
		{protocol.PasswordResetFailed{Reason: protocol.ResetPasswordWrongToken}, []string{"Wrong token"}},
		{protocol.PasswordChanged{}, []string{"Password changed"}},
		{protocol.WrongPassword{}, []string{"Wrong password"}},
	} {
		got, err := Format(tc.ev, 0)
		if err != nil {
			t.Errorf("Format(%#v): %v", tc.ev, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Format(%#v) = %q, want %q", tc.ev, got, tc.want)
		}
	}
}

func TestFormatUnknownReason(t *testing.T) {
	if _, err := Format(protocol.RegistrationFailed{Reason: 99}, 0); err == nil {
		t.Error("expected error for unknown reason")
	}
}

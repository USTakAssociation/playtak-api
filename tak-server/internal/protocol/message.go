// Package protocol defines the server's internal message types. Clients speak
// a wire format (legacy text today, JSON later) that is parsed into Commands
// and rendered from Events by a format-specific package such as legacy.
package protocol

// Command is a request from a client.
type Command interface{ command() }

type (
	Quit struct{}
	Ping struct{}
	// ClientName identifies the client software, e.g. "TakWeb-16.05.26".
	ClientName  struct{ Name string }
	SetProtocol struct{ Version int }
	// LoginGuest logs in as a guest. A non-empty Token rejoins the same guest.
	LoginGuest struct{ Token string }
	Login      struct{ Name, Password string }
	Register   struct{ Name, Email string }
	// RegisterMalformed is a Register command whose arguments are invalid.
	RegisterMalformed struct{}
	SendResetToken    struct{ Name, Email string }
	ResetPassword     struct{ Name, Token, Password string }
	ChangePassword    struct{ Old, New string }

	// PostSeek replaces the session's seek with a new one.
	PostSeek struct {
		Settings GameSettings
		Opponent string
	}
	CancelSeek struct{}
	ListSeeks  struct{}
	AcceptSeek struct{ ID int }
	// Rematch offers a new game to the opponent of a finished game. If they
	// have already offered one, the client is told to accept theirs instead.
	Rematch struct {
		GameID   int
		Settings GameSettings
		Opponent string
	}
)

func (Quit) command()              {}
func (Ping) command()              {}
func (ClientName) command()        {}
func (SetProtocol) command()       {}
func (LoginGuest) command()        {}
func (Login) command()             {}
func (Register) command()          {}
func (RegisterMalformed) command() {}
func (SendResetToken) command()    {}
func (ResetPassword) command()     {}
func (ChangePassword) command()    {}
func (PostSeek) command()          {}
func (CancelSeek) command()        {}
func (ListSeeks) command()         {}
func (AcceptSeek) command()        {}
func (Rematch) command()           {}

// Event is a message to a client.
type Event interface{ event() }

type (
	// Greeting is sent when a connection opens.
	Greeting struct{}
	OK       struct{}
	NOK      struct{}
	LoggedIn struct{ Name string }
	// IsMod tells a just-logged-in moderator or admin about their role.
	IsMod       struct{}
	AuthFailure struct{}
	// OnlineCount is the number of distinct players online.
	OnlineCount   struct{ Count int }
	OnlinePlayers struct{ Names []string }
	// ServerMessage is a notice shown to the user.
	ServerMessage struct{ Text string }
	Shout         struct{ From, Text string }
	// RoomJoined confirms JoinRoom.
	RoomJoined struct{ Room string }
	RoomShout  struct{ Room, From, Text string }
	// TellReceived is a private message; Told is the sender's copy.
	TellReceived struct{ From, Text string }
	Told         struct{ To, Text string }
	// RawLine is sent to the client exactly as given. Only admins' sudo
	// broadcast uses it.
	RawLine struct{ Text string }
	// CommandFailed says why a command was refused, without a NOK.
	CommandFailed       struct{ Reason string }
	Registered          struct{ Name string }
	RegistrationFailed  struct{ Reason RegistrationError }
	ResetTokenSent      struct{}
	ResetTokenFailed    struct{ Reason ResetTokenError }
	PasswordReset       struct{}
	PasswordResetFailed struct{ Reason ResetPasswordError }
	PasswordChanged     struct{}
	WrongPassword       struct{}
	SeekAdded           struct{ Seek Seek }
	SeekRemoved         struct{ Seek Seek }
	// AcceptRematch tells a client its opponent already offered a rematch,
	// as seek SeekID, which the client should accept.
	AcceptRematch      struct{ SeekID int }
	RematchSeekCreated struct{ SeekID int }
)

func (Greeting) event()            {}
func (OK) event()                  {}
func (NOK) event()                 {}
func (LoggedIn) event()            {}
func (IsMod) event()               {}
func (AuthFailure) event()         {}
func (OnlineCount) event()         {}
func (OnlinePlayers) event()       {}
func (ServerMessage) event()       {}
func (Shout) event()               {}
func (RoomJoined) event()          {}
func (RoomShout) event()           {}
func (TellReceived) event()        {}
func (Told) event()                {}
func (CommandFailed) event()       {}
func (RawLine) event()             {}
func (Registered) event()          {}
func (RegistrationFailed) event()  {}
func (ResetTokenSent) event()      {}
func (ResetTokenFailed) event()    {}
func (PasswordReset) event()       {}
func (PasswordResetFailed) event() {}
func (PasswordChanged) event()     {}
func (WrongPassword) event()       {}
func (SeekAdded) event()           {}
func (SeekRemoved) event()         {}
func (AcceptRematch) event()       {}
func (RematchSeekCreated) event()  {}

type RegistrationError int

const (
	RegistrationMalformed RegistrationError = iota
	RegistrationGuestName
	RegistrationNameTaken
	RegistrationServerError
	RegistrationRateLimited
)

type ResetTokenError int

const (
	ResetTokenNoSuchPlayer ResetTokenError = iota
	ResetTokenEmailMismatch
	ResetTokenRateLimited
)

type ResetPasswordError int

const (
	ResetPasswordNoSuchPlayer ResetPasswordError = iota
	ResetPasswordWrongToken
)

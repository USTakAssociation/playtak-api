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
	ServerMessage       struct{ Text string }
	Shout               struct{ From, Text string }
	Registered          struct{ Name string }
	RegistrationFailed  struct{ Reason RegistrationError }
	ResetTokenSent      struct{}
	ResetTokenFailed    struct{ Reason ResetTokenError }
	PasswordReset       struct{}
	PasswordResetFailed struct{ Reason ResetPasswordError }
	PasswordChanged     struct{}
	WrongPassword       struct{}
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
func (Registered) event()          {}
func (RegistrationFailed) event()  {}
func (ResetTokenSent) event()      {}
func (ResetTokenFailed) event()    {}
func (PasswordReset) event()       {}
func (PasswordResetFailed) event() {}
func (PasswordChanged) event()     {}
func (WrongPassword) event()       {}

type RegistrationError int

const (
	RegistrationMalformed RegistrationError = iota
	RegistrationGuestName
	RegistrationProfanity
	RegistrationNameTaken
	RegistrationServerError
)

type ResetTokenError int

const (
	ResetTokenNoSuchPlayer ResetTokenError = iota
	ResetTokenEmailMismatch
)

type ResetPasswordError int

const (
	ResetPasswordNoSuchPlayer ResetPasswordError = iota
	ResetPasswordWrongToken
)

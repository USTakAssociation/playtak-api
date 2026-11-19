package protocol

// SudoAction is a moderator or admin command.
type SudoAction int

const (
	SudoUnknown SudoAction = iota
	SudoGag
	SudoUngag
	SudoKick
	SudoBan
	SudoUnban
	SudoList
	SudoBroadcast
	SudoMod
	SudoUnmod
	SudoAdmin
	SudoUnadmin
	SudoBot
	SudoUnbot
	SudoSetPassword
	SudoRevoke
)

// Sudo is a moderator or admin command.
type Sudo struct {
	// Text is the command as typed, for echoing back and logging, with any
	// password replaced by asterisks.
	Text   string
	Action SudoAction
	Name   string // the player acted on
	Arg    string // ban reason, list group, broadcast text or new password
}

func (Sudo) command() {}

// SudoReply is one line of a sudo command's result.
type SudoReply struct{ Text string }

func (SudoReply) event() {}

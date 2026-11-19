package protocol

// Chat commands.
type (
	PostShout     struct{ Text string }
	PostRoomShout struct{ Room, Text string }
	JoinRoom      struct{ Room string }
	LeaveRoom     struct{ Room string }
	SendTell      struct{ To, Text string }
)

func (PostShout) command()     {}
func (PostRoomShout) command() {}
func (JoinRoom) command()      {}
func (LeaveRoom) command()     {}
func (SendTell) command()      {}

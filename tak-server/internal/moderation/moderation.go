// Package moderation runs moderator and admin ("sudo") commands: gagging,
// kicking and banning players, changing roles and accounts, and server
// announcements.
package moderation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/USTakAssociation/playtak-api/tak-server/internal/db/players"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/player"
	"github.com/USTakAssociation/playtak-api/tak-server/internal/protocol"
)

type Store interface {
	ByNameFold(ctx context.Context, name string) (players.Record, error)
	SetFlag(ctx context.Context, name string, flag players.Flag, on bool) error
	Flagged(ctx context.Context, flag players.Flag) ([]string, error)
}

type Accounts interface {
	SetPassword(ctx context.Context, name, password string) error
	RevokeAll(ctx context.Context, name string) (int64, error)
}

type Games interface {
	Forfeit(ctx context.Context, name string) (bool, error)
}

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Result is what a command produced: lines for the person who ran it, and
// a raw protocol line for everyone, if any.
type Result struct {
	Replies   []string
	Broadcast string
}

type Service struct {
	store       Store
	accounts    Accounts
	games       Games
	registry    *player.Registry
	mail        Mailer
	reloadWords func(context.Context) (int, error)
	log         *slog.Logger
}

func NewService(store Store, accounts Accounts, games Games, registry *player.Registry, mail Mailer,
	reloadWords func(context.Context) (int, error), log *slog.Logger) *Service {
	return &Service{store: store, accounts: accounts, games: games, registry: registry, mail: mail, reloadWords: reloadWords, log: log}
}

// rank orders authority: a moderator acts on players, an admin on players
// and moderators, and nobody on their own rank or above, including
// themselves.
type rank int

const (
	rankPlayer rank = iota
	rankMod
	rankAdmin
)

func rankOf(admin, mod bool) rank {
	switch {
	case admin:
		return rankAdmin
	case mod:
		return rankMod
	}
	return rankPlayer
}

// Allowed reports whether p may use sudo at all.
func Allowed(p *player.Player) bool { return rankOf(p.IsAdmin(), p.IsMod()) >= rankMod }

const (
	noSuchPlayer = "No such player"
	noRights     = "You don't have rights"
	denied       = "Permission denied"
	serverError  = "Server error, please try again later"
)

// Run carries out cmd for actor, who must be Allowed.
func (s *Service) Run(ctx context.Context, actor *player.Player, cmd protocol.Sudo) Result {
	r := s.run(ctx, actor, cmd)
	s.log.Info("sudo", "by", actor.Name, "command", cmd.Text, "result", strings.Join(r.Replies, "; "), "broadcast", r.Broadcast)
	return r
}

func (s *Service) run(ctx context.Context, actor *player.Player, cmd protocol.Sudo) Result {
	me := rankOf(actor.IsAdmin(), actor.IsMod())
	switch cmd.Action {
	case protocol.SudoGag, protocol.SudoUngag:
		return reply(s.gag(ctx, me, cmd.Name, cmd.Action == protocol.SudoGag))
	case protocol.SudoKick:
		return reply(s.kick(me, cmd.Name))
	case protocol.SudoBan:
		return reply(s.ban(ctx, me, cmd.Name, cmd.Arg)...)
	case protocol.SudoUnban:
		return reply(s.unban(ctx, me, cmd.Name))
	case protocol.SudoList:
		return reply(s.list(ctx, cmd.Arg))
	case protocol.SudoUnknown:
		return reply("Command not found")
	}

	// Admin only from here on.
	if me < rankAdmin {
		return reply(denied)
	}
	switch cmd.Action {
	case protocol.SudoBroadcast:
		// Sent as a raw protocol line, as the Java server did: admins use it
		// for Message, Error:, Shout, ShoutRoom and Tell announcements.
		return Result{Broadcast: cmd.Arg}
	case protocol.SudoReloadWords:
		if _, err := s.reloadWords(ctx); err != nil {
			s.log.Error("reload bad word filter", "err", err)
			return reply("Could not reload the bad word filter")
		}
		return reply("Bad word filter reloaded")
	case protocol.SudoMod, protocol.SudoUnmod:
		return reply(s.setRole(ctx, actor, cmd.Name, players.FlagMod, cmd.Action == protocol.SudoMod, "moderator"))
	case protocol.SudoAdmin, protocol.SudoUnadmin:
		return reply(s.setRole(ctx, actor, cmd.Name, players.FlagAdmin, cmd.Action == protocol.SudoAdmin, "admin"))
	case protocol.SudoBot, protocol.SudoUnbot:
		return reply(s.setRole(ctx, actor, cmd.Name, players.FlagBot, cmd.Action == protocol.SudoBot, "bot"))
	case protocol.SudoSetPassword:
		return reply(s.setPassword(ctx, me, cmd.Name, cmd.Arg))
	case protocol.SudoRevoke:
		return reply(s.revoke(ctx, me, cmd.Name))
	}
	return reply("Command not found")
}

func reply(lines ...string) Result { return Result{Replies: lines} }

// registered looks a registered player up by name, ignoring case. It returns
// the reply to send instead if there's no such player or it failed.
func (s *Service) registered(ctx context.Context, name string) (players.Record, string) {
	rec, err := s.store.ByNameFold(ctx, name)
	if errors.Is(err, players.ErrNotFound) {
		return rec, noSuchPlayer
	}
	if err != nil {
		s.log.Error("sudo player lookup", "name", name, "err", err)
		return rec, serverError
	}
	return rec, ""
}

// setFlag saves flag for rec and updates them in memory if they're online.
func (s *Service) setFlag(ctx context.Context, rec players.Record, flag players.Flag, on bool) error {
	if err := s.store.SetFlag(ctx, rec.Name, flag, on); err != nil {
		return err
	}
	switch flag {
	case players.FlagGagged:
		rec.IsGagged = on
	case players.FlagBanned:
		rec.IsBanned = on
	case players.FlagMod:
		rec.IsMod = on
	case players.FlagAdmin:
		rec.IsAdmin = on
	case players.FlagBot:
		rec.IsBot = on
	}
	s.registry.Refresh(rec)
	return nil
}

func (s *Service) gag(ctx context.Context, me rank, name string, on bool) string {
	// An online guest has no record; anyone else is looked up so they can be
	// gagged while offline.
	if p := s.registry.FindOnline(name); p != nil && p.Guest {
		if rankPlayer >= me {
			return noRights
		}
		if p.IsGagged() == on {
			return alreadyGagged(on)
		}
		p.SetGagged(on)
		return p.Name + gagged(on)
	}
	rec, fail := s.registered(ctx, name)
	if fail != "" {
		return fail
	}
	if rankOf(rec.IsAdmin, rec.IsMod) >= me {
		return noRights
	}
	if rec.IsGagged == on {
		return alreadyGagged(on)
	}
	if err := s.setFlag(ctx, rec, players.FlagGagged, on); err != nil {
		s.log.Error("set gag", "name", rec.Name, "err", err)
		return serverError
	}
	return rec.Name + gagged(on)
}

func gagged(on bool) string {
	if on {
		return " gagged"
	}
	return " ungagged"
}

func alreadyGagged(on bool) string {
	if on {
		return "Player is already gagged"
	}
	return "Player is not gagged"
}

func (s *Service) kick(me rank, name string) string {
	p := s.registry.FindOnline(name)
	if p == nil {
		return "Player not logged in"
	}
	if rankOf(p.IsAdmin(), p.IsMod()) >= me {
		return noRights
	}
	disconnect(p)
	return p.Name + " kicked"
}

func disconnect(p *player.Player) {
	for _, c := range p.Clients() {
		c.Disconnect()
	}
}

func (s *Service) ban(ctx context.Context, me rank, name, reason string) []string {
	if p := s.registry.FindOnline(name); p != nil && p.Guest {
		return []string{"Guests can't be banned, use kick"}
	}
	rec, fail := s.registered(ctx, name)
	if fail != "" {
		return []string{fail}
	}
	if rankOf(rec.IsAdmin, rec.IsMod) >= me {
		return []string{noRights}
	}
	if rec.IsBanned {
		return []string{"Player is already banned"}
	}
	if err := s.setFlag(ctx, rec, players.FlagBanned, true); err != nil {
		s.log.Error("set ban", "name", rec.Name, "err", err)
		return []string{serverError}
	}
	// The ban stands from here; what follows tidies up after it.
	if _, err := s.accounts.RevokeAll(ctx, rec.Name); err != nil {
		s.log.Error("revoke tokens of banned player", "name", rec.Name, "err", err)
	}
	if _, err := s.games.Forfeit(ctx, rec.Name); err != nil {
		s.log.Error("end banned player's game", "name", rec.Name, "err", err)
	}
	if rec.Email != "" {
		go func() {
			body := rec.Name + ",\nYou have been banned from playtak.com!\nReason: " + reason
			if err := s.mail.Send(context.WithoutCancel(ctx), rec.Email, "PlayTak.com Ban Notice", body); err != nil {
				s.log.Error("send ban notice", "name", rec.Name, "err", err)
			}
		}()
	}
	var out []string
	if p := s.registry.FindOnline(rec.Name); p != nil {
		disconnect(p)
		out = append(out, rec.Name+" kicked")
	}
	return append(out, rec.Name+" banned")
}

func (s *Service) unban(ctx context.Context, me rank, name string) string {
	rec, fail := s.registered(ctx, name)
	if fail != "" {
		return fail
	}
	if rankOf(rec.IsAdmin, rec.IsMod) >= me {
		return noRights
	}
	if !rec.IsBanned {
		return "Player is not banned"
	}
	if err := s.setFlag(ctx, rec, players.FlagBanned, false); err != nil {
		s.log.Error("clear ban", "name", rec.Name, "err", err)
		return serverError
	}
	return rec.Name + " unbanned"
}

var listFlags = map[string]players.Flag{
	"gag":   players.FlagGagged,
	"mod":   players.FlagMod,
	"ban":   players.FlagBanned,
	"admin": players.FlagAdmin,
}

func (s *Service) list(ctx context.Context, group string) string {
	var names []string
	if group == "online" {
		for _, p := range s.registry.Online() {
			names = append(names, p.Name)
		}
	} else {
		flag, ok := listFlags[group]
		if !ok {
			return "command not found"
		}
		var err error
		if names, err = s.store.Flagged(ctx, flag); err != nil {
			s.log.Error("list players", "group", group, "err", err)
			return serverError
		}
		if group == "gag" { // gagged guests aren't in the database
			for _, p := range s.registry.Online() {
				if p.Guest && p.IsGagged() {
					names = append(names, p.Name)
				}
			}
		}
	}
	slices.SortFunc(names, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	return "[" + strings.Join(names, ", ") + "]"
}

func (s *Service) setRole(ctx context.Context, actor *player.Player, name string, flag players.Flag, on bool, role string) string {
	rec, fail := s.registered(ctx, name)
	if fail != "" {
		return fail
	}
	if strings.EqualFold(rec.Name, actor.Name) {
		return "You can't change your own roles"
	}
	has := map[players.Flag]bool{players.FlagMod: rec.IsMod, players.FlagAdmin: rec.IsAdmin, players.FlagBot: rec.IsBot}[flag]
	article := "a "
	if role == "admin" {
		article = "an "
	}
	switch {
	case on && has:
		return rec.Name + " is already " + article + role
	case !on && !has:
		return rec.Name + " is not " + article + role
	}
	if err := s.setFlag(ctx, rec, flag, on); err != nil {
		s.log.Error("set role", "name", rec.Name, "role", role, "err", err)
		return serverError
	}
	if on {
		return fmt.Sprintf("Added %s as %s", rec.Name, role)
	}
	return fmt.Sprintf("Removed %s as %s", rec.Name, role)
}

func (s *Service) setPassword(ctx context.Context, me rank, name, password string) string {
	rec, fail := s.registered(ctx, name)
	if fail != "" {
		return fail
	}
	if rankOf(rec.IsAdmin, rec.IsMod) >= me {
		return noRights
	}
	if err := s.accounts.SetPassword(ctx, rec.Name, password); err != nil {
		s.log.Error("set password", "name", rec.Name, "err", err)
		return serverError
	}
	if _, err := s.accounts.RevokeAll(ctx, rec.Name); err != nil {
		s.log.Error("revoke tokens after password set", "name", rec.Name, "err", err)
	}
	return "Password set"
}

func (s *Service) revoke(ctx context.Context, me rank, name string) string {
	rec, fail := s.registered(ctx, name)
	if fail != "" {
		return fail
	}
	if rankOf(rec.IsAdmin, rec.IsMod) >= me {
		return noRights
	}
	n, err := s.accounts.RevokeAll(ctx, rec.Name)
	if err != nil {
		s.log.Error("revoke tokens", "name", rec.Name, "err", err)
		return serverError
	}
	return fmt.Sprintf("Revoked %d tokens for %s", n, rec.Name)
}

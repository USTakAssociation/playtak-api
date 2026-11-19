# PlayTak JSON Protocol (draft v1)

**Status: draft for community review.** Nothing here is final. Send comments
before we build it. Open questions are listed at the end.

This document describes a new JSON protocol for the PlayTak server. It sits
next to the current text protocol. The text protocol stays supported, but it
is feature-frozen: it gets fixes, and new features come only to JSON.

## Goals

- Easy to parse in any language. Messages are flat JSON objects.
- Easy to debug. Every log line shows who sent it and what it means.
- Stable. A client written for v1 keeps working while v1 exists.
- Extensible. Additions do not break old clients.
- One protocol for UIs and bots. A bot uses the same commands and events as
  a UI. Only how it logs in can differ. Bot tools, such as a bridge for TEI
  engines, can be built on top of it later.

## Contents

- [Quick start](#quick-start)
1. [Connecting](#1-connecting)
2. [Message format](#2-message-format)
3. [Session](#3-session)
4. [Players](#4-players)
5. [Bots](#5-bots)
6. [Seeks](#6-seeks)
7. [Games](#7-games)
8. [Chat](#8-chat)
9. [Moderation](#9-moderation)
10. [Errors](#10-errors)
11. [Versioning](#11-versioning)
12. [Limits](#12-limits)
13. [REST API](#13-rest-api)
14. [Planned](#14-planned)
15. [Open questions](#15-open-questions)

In this document, **must** means required, and **should** means
recommended.

## Quick start

A UI client that logs in, accepts a seek and plays one move. The client
sends the `->` lines, and the server sends the `<-` lines. `ts` is left out to keep it short.

```
open wss://playtak.com/ws with the subprotocol playtak.json.v1

-> {"cmd":"session.init","id":"1","version":"1.0","client_name":"MyClient","client_version":"1.0"}
<- {"evt":"ack","id":"1","server":"tak-server/1.4.2","version":"1.0"}

-> {"cmd":"session.login","id":"2","token":"<access token from POST /api/v1/auth/login>"}
<- {"evt":"ack","id":"2"}
<- {"evt":"session.welcome","username":"alice","roles":[],"games":[],"private_rooms":[]}
<- {"evt":"seek.list","seeks":[{"seek":498, ...}]}
<- {"evt":"game.list","games":[...]}
<- {"evt":"session.online","count":87,"usernames":[...]}

-> {"cmd":"seek.accept","id":"3","seek":498}
<- {"evt":"ack","id":"3","game":1043}
<- {"evt":"session.games","games":[1043]}

-> {"cmd":"game.attach","id":"4","game":1043}
<- {"evt":"ack","id":"4"}
<- {"evt":"game.start","game":1043,"color":"white", ...}
<- {"evt":"game.state","game":1043,"ply":0,"turn":"white", ...}

-> {"cmd":"game.move","id":"5","game":1043,"move":"a1"}
<- {"evt":"ack","id":"5"}
<- {"evt":"game.state","game":1043,"ply":1,"change":"move","last":"a1", ...}
```

For a bot, see [A minimal bot](#a-minimal-bot).

## 1. Connecting

### Addresses

| Service | Production | Beta |
| --- | --- | --- |
| WebSocket | `wss://playtak.com/ws` | `wss://beta.playtak.com/ws` |
| Telnet (plain TCP) | `playtak.com:10000` | `beta.playtak.com:10000` |
| REST API (this server) | `https://playtak.com` (after the switch to the new server) | `https://beta.playtak.com` |
| Game history and ratings | `https://api.playtak.com` | `https://api.beta.playtak.com` |

REST paths in this document are full paths from the host. For example,
`POST /api/v1/auth/login` on beta is
`https://beta.playtak.com/api/v1/auth/login`.

The game history and ratings API is a separate service. Its documentation is
at `https://api.playtak.com/api`. Finished games are read from it, for
example `GET /v1/games-history/{id}` and `GET /v1/games-history/ptn/{id}`.

The two APIs will be merged into one later, with a single base URL. Until
then, accounts, bots and seeks use this server's REST API, and game history
and ratings use the other one.

### WebSocket

Choose JSON in one of two ways:

| Method | Example |
| --- | --- |
| Subprotocol | `new WebSocket("wss://playtak.com/ws", ["playtak.json.v1"])` |
| Query parameter | `new WebSocket("wss://playtak.com/ws?protocol=json&v=1")` |

With the subprotocol, the server echoes `playtak.json.v1` if it accepts it.
Either way, the first message from the server confirms the protocol. If you
get `Welcome!` as text instead, the server does not support JSON.

Each message is one JSON object in one text frame. Text protocol
connections keep using binary frames, as they do today.

Compression: the server supports the standard WebSocket `permessage-deflate`
extension. Browsers and most libraries turn it on automatically.

The server accepts WebSocket connections from any website (any `Origin`),
so third-party web clients can connect. `client_name` in `session.init` is
required, so the server can tell clients apart in its logs.

### Telnet (raw TCP)

"Telnet" here means a plain TCP connection. No telnet protocol features
are used. You can try it with a tool such as `telnet` or `nc`, but a bot
just opens a TCP socket.

A telnet connection starts in the text protocol, so the server sends its
text greeting first. Switch to JSON with the text command `Protocol json`,
before you log in:

```
<- Welcome!
<- Login or Register
-> Protocol json
<- OK
-> {"cmd":"session.init","id":"h1","version":"1.0","client_name":"MyBot","client_version":"2.1"}
<- {"evt":"ack","id":"h1","server":"tak-server/1.4.2","version":"1.0","ts":1791234560000}
```

`OK` is the last text line. Everything after it is JSON. This is the same
`Protocol` command the text protocol uses to pick a text version
(`Protocol 4`). A WebSocket connection can also switch this way, but the
subprotocol or query parameter is simpler.

Switching is only about the transport. After the switch, `session.init` is
the same on every connection.

Each message is one JSON object on one line, ending with `\n`.

### One protocol per connection

A connection uses JSON or the text protocol for its whole life. It cannot
switch back. Players on JSON and players on the text protocol can still play
each other (see [Versioning](#11-versioning)).

### JSON and text connections for the same player

A player's JSON connections take precedence over text ones:

- If the player has a JSON connection open, the server refuses a text
  login. It tells the text client that the player is logged in on another
  client.
- If the player has a text connection open, a JSON login closes it. A
  second text login does the same today.

### Closing

Before the server closes a connection, it sends `session.closed` with the
reason (see [session.closed](#sessionclosed-event)). WebSocket connections
also get a close code, one per reason, so a client can tell what happened
even if `session.closed` never arrived. Standard codes are used only where
their meaning fits exactly. The rest are in the range reserved for
applications (4000 to 4999).

| Reason | Close code | Defined by |
| --- | --- | --- |
| `shutdown` | `1001` (going away) | [RFC 6455, section 7.4.1](https://www.rfc-editor.org/rfc/rfc6455#section-7.4.1) |
| `message_too_big` | `1009` (message too big) | RFC 6455, section 7.4.1 |
| `server_error` | `1011` (internal error) | RFC 6455, section 7.4.1 |
| `restart` | `1012` (service restart) | [IANA WebSocket Close Code Number Registry](https://www.iana.org/assignments/websocket/websocket.xhtml#close-code-number) |
| `idle` | `4000` | PlayTak |
| `not_reading` | `4001` | PlayTak |
| `send_queue_full` | `4002` | PlayTak |
| `kicked` | `4003` | PlayTak |
| `banned` | `4004` | PlayTak |
| `revoked` | `4005` | PlayTak |
| `api_key_replaced` | `4006` | PlayTak |
| `api_key_revoked` | `4007` | PlayTak |
| `replaced` | `4008` | PlayTak |

Codes `4000` to `4999` are reserved for private use by applications
([RFC 6455, section 7.4.2](https://www.rfc-editor.org/rfc/rfc6455#section-7.4.2)).
Their meaning is defined only by this spec, so other software will not
recognize them.

These codes apply to JSON connections only. Text protocol connections keep
their current close codes. For what each reason means and when to
reconnect, see [session.closed](#sessionclosed-event).

## 2. Message format

### Rules

1. Every message has exactly one of `cmd` (client to server) or `evt`
   (server to client).
2. The value is a name such as `game.move`. Read it as one name. The part
   before the dot is the area, for clients that want to route by area.
3. `cmd`, `evt`, `id` and `ts` are reserved. No data field uses these names.
4. All other fields are data, at the top level. Only lists and records
   (such as a seek in a list, or a [player](#4-players)) are nested.
5. Every `cmd` has an `id`, chosen by the client (string or number). The
   server answers each `cmd` with exactly one `ack` or `error` carrying the
   same `id`.
6. The `ack` or `error` comes before any events the command causes.
7. Every server message has `ts`: the server time in Unix milliseconds.

### Example

```json
{"cmd":"game.move","id":"r9","game":1043,"move":"3c3>12"}
{"evt":"ack","id":"r9","ts":1791234567000}
{"evt":"game.state","game":1043,"ply":24,"change":"move","last":"3c3>12","tps":"x6/x6/x2,1,x3/x6/x6/x6 2 12","turn":"black","white_time":598000,"black_time":587900,"clock_running":true,"ts":1791234567001}
```

### Value conventions

| Kind | Format |
| --- | --- |
| Names (`cmd`, `evt`) | lowercase, dot between area and action, `_` between words |
| Field names | lowercase with `_` between words, full words (`increment`, not `inc`) |
| Times and durations | integer milliseconds |
| Enum values | lowercase strings, for example `"white"`, `"any"`. The exception is `result`, which uses PTN result notation, such as `"R-0"` and `"1/2-1/2"`. |
| Error codes | uppercase, for example `NOT_YOUR_TURN` |
| Missing value | the field is left out. The server never sends `null`. |

### Reading the reference tables

Each command and event below has a table of its fields.

| Column | Meaning |
| --- | --- |
| Type | `string`, `integer`, `number`, `boolean`, `player` (see [Players](#4-players)), or a list of one of these |
| Req. | For commands: whether you must send it. For events: whether the server always sends it. "no" means it can be left out. |

`cmd`, `evt`, `id` and `ts` are not repeated in the tables.

`ts` is useful for showing when things happened (such as chat
messages,), spotting a slow connection, matching client and server logs, and
putting events from different games into server order.

### Ignore what you do not know

Clients must ignore unknown fields, unknown events and unknown enum values.
This is what lets the server add things without breaking you.

### Ordering

- Events for one game always arrive in order.
- The server can send other events between your `cmd` and its `ack`. Match
  replies by `id`.

### Command order

A connection must send `session.init` first, then `session.login`, then
anything else.

| Sent | Error |
| --- | --- |
| Anything except `session.init` and `session.ping` before `session.init` | `INIT_REQUIRED` |
| `session.init` a second time | `ALREADY_INITIALIZED` |
| Anything except `session.login` and `session.ping` before logging in | `NOT_AUTHENTICATED` |
| `session.login` after logging in | `ALREADY_LOGGED_IN` |

## 3. Session

A session is a connection after it has logged in. Commands named
`session.*` act on your own connection.

### session.init (command)

The first JSON command on every connection.

```json
{"cmd":"session.init","id":"h1","version":"1.0","client_name":"MyBot","client_version":"2.1"}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `version` | string | yes | The protocol version this client supports, `major.minor`. See [section 11](#11-versioning). |
| `client_name` | string | yes | Client name, for logs |
| `client_version` | string | yes | Client version, for logs |

Reply: `ack` with these fields.

```json
{"evt":"ack","id":"h1","server":"tak-server/1.4.2","version":"1.0","ts":1791234560000}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `server` | string | yes | Server software name and release, for logs, for example `tak-server/1.4.2`. This is not the protocol version and changes with every release. |
| `version` | string | yes | The protocol version: the highest one the server supports with the same major version as the client's |

Errors: `UNSUPPORTED_VERSION` if the major version is not supported, after
which the connection closes, and `ALREADY_INITIALIZED`.

### session.login (command)

```json
{"cmd":"session.login","id":"r1","token":"<access token>"}
{"cmd":"session.login","id":"r1","guest_token":"<guest token>"}
{"cmd":"session.login","id":"r1","api_key":"<api key>"}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `token` | string | one of the three | An access token from `POST /api/v1/auth/login` or `/api/v1/auth/refresh` |
| `guest_token` | string | one of the three | A guest token from `POST /api/v1/auth/guest` |
| `api_key` | string | one of the three | A bot's API key. See [Bots](#5-bots). |

Send exactly one of the three. The JSON protocol never accepts a password.
Get a token from the [REST API](#13-rest-api) first.

The access token is checked once, at login. The connection stays logged in
after the token expires. A ban or a revoke closes it.

An expired, revoked or unknown token, guest token or API key gets
`AUTH_FAILED`. For a guest, get a new guest token from
`POST /api/v1/auth/guest`, which also gives a new guest name.

Reply: `ack` with no extra fields.

Errors: `AUTH_FAILED`, `RATE_LIMITED`, `INVALID_FIELD`, `INIT_REQUIRED`,
`ALREADY_LOGGED_IN`.

After the `ack`, the server sends, in order:

1. `session.welcome`
2. `seek.list`
3. `game.list`
4. `session.online`

Your games in progress are not attached automatically. `session.welcome`
lists them. Send `game.attach` for each one you want on this connection.

### session.welcome (event)

```json
{"evt":"session.welcome","username":"alice","roles":["mod"],"games":[1043],"private_rooms":[{"room":"private:7","members":["alice","bob"]}],"ts":1791234560100}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `username` | string | yes | Your username. Guest names start with `Guest`, and registered names cannot contain "guest". |
| `roles` | list of string | yes | Any of `mod`, `admin`, `bot`. Empty for most players. |
| `games` | list of integer | yes | Your games in progress. Empty if none. Attach to one with `game.attach`. |
| `private_rooms` | list of object | yes | Your [private rooms](#private-rooms), each with `room` and `members` as in `chat.room`. Empty if none. |

### session.games (event)

Sent to every connection of yours when one of your games starts or ends.
The client decides what to do with a new game: attach to it and show the
board, or add it to a list the player can switch between.

```json
{"evt":"session.games","games":[1043,1051],"ts":1791234565000}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `games` | list of integer | yes | Your games in progress, as in `session.welcome` |

A client that plays should attach to a new game as soon as `session.games`
lists it. If no connection attaches, you are away from the game from the
start (see [Disconnects](#disconnects)).

### session.online (event)

Sent right after login, then at most once a second while the list is
changing. Each one carries the full current list, so a client only ever
needs the latest one. When many players log in or out at once, for example
after a server restart, the server sends the changes from the same second
as one event.

```json
{"evt":"session.online","count":3,"usernames":["alice","bob","Guest123"],"ts":1791234560200}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `count` | integer | yes | Number of entries in `usernames` |
| `usernames` | list of string | yes | Usernames of everyone online, guests included. A player with several connections is listed once. |

### session.notice (event)

An announcement from an admin.

```json
{"evt":"session.notice","text":"Server restarting in 5 minutes","ts":1791234600000}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `text` | string | yes | The announcement |

### session.closed (event)

The last message before the server closes the connection.

```json
{"evt":"session.closed","reason":"restart","retry_ms":30000,"message":"Server restarting","ts":1791234600000}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `reason` | string | yes | See below |
| `message` | string | yes | For people. For `banned`, it includes the ban reason. |
| `retry_ms` | integer | no | With `restart`: how long to wait before reconnecting |

| `reason` | Meaning |
| --- | --- |
| `idle` | The server received nothing from you for 60 seconds |
| `not_reading` | You stopped taking messages off the connection. See [When the server stops sending](#when-the-server-stops-sending). |
| `send_queue_full` | You are reading, but more slowly than messages arrive. See [When the server stops sending](#when-the-server-stops-sending). |
| `message_too_big` | You sent a message over 64 KB |
| `restart` | The server is restarting and will be back shortly. See `retry_ms`. |
| `shutdown` | The server is shutting down, with no return time |
| `kicked` | A moderator kicked you |
| `banned` | A moderator banned you |
| `revoked` | An admin revoked your tokens |
| `api_key_replaced` | The bot's API key was replaced |
| `api_key_revoked` | The bot's API key was revoked |
| `replaced` | You connected again while in a tournament game, and the new connection took over (see [Connections and games](#connections-and-games)) |
| `server_error` | Something failed on the server |

When to reconnect:

| Reason | Reconnect? |
| --- | --- |
| `banned`, `revoked`, `api_key_replaced`, `api_key_revoked` | No. It will fail. |
| `replaced` | No. Another connection of yours took over. Reconnecting would close that one. |
| `restart` | Yes, after `retry_ms` |
| Anything else | Yes, with a growing delay, for example 1, 2, 4 and 8 seconds, up to a minute |

### session.ping (command)

```json
{"cmd":"session.ping","id":"p1"}
{"evt":"ack","id":"p1","ts":1791234560300}
```

No fields. Reply: `ack`.

A client must send something at least every 60 seconds, or the server
closes the connection with `idle`. Any command counts. A client should send
`session.ping` every 30 seconds when it has nothing else to send.

- **Browsers** cannot send WebSocket ping frames from JavaScript. Browser
  clients must use `session.ping`.
- **Other WebSocket clients** can send WebSocket ping frames instead. They
  count as traffic too.
- **Telnet clients** must use `session.ping`.

### Connections and games

- A player can have more than one connection open, for example two tabs or
  a laptop and a phone.
- A player's games belong to the player, not to a connection. A connection
  gets a game's events once it is attached to the game, and any attached
  connection can play in it.
- The server never attaches a connection by itself. When one of your games
  starts or ends, every connection of yours gets `session.games`. A
  connection attaches with `game.attach` and stops with `game.detach`.
  Several connections can be attached to the same game.
- The server handles commands for a game one at a time. If two connections
  send a move at once, the second gets `NOT_YOUR_TURN` or `ILLEGAL_MOVE`.
- A player can play any number of games at once. Each game has its own
  clock.
- Any connection can observe any number of games.

**Tournament games** are the exception:

- A tournament game cannot start while either player has another game in
  progress. `seek.accept` gets `GAMES_IN_PROGRESS`.
- When a tournament game starts, the server removes both players' other open
  seeks, and stops anything either player is observing (sending
  `game.unobserved`).
- While a player is in a tournament game, they cannot start another game or
  observe a game. These get `TOURNAMENT_IN_PROGRESS`.
- While a player is in a tournament game, they have only one connection. A
  new connection replaces the old ones. The server closes them with
  `session.closed` `reason: "replaced"`. This way a connection that died
  without closing, for example on a phone that lost its signal, never locks
  the player out while their clock runs. The new connection then attaches to
  the game.
- If a player has more than one connection open when their tournament game
  starts, they choose which one to keep:
  1. Every connection of theirs gets `session.choose_connection`.
  2. The connection that sends `session.keep_connection` stays. The server
     closes the others with `reason: "replaced"`.
  3. Until then, those connections can only send `session.keep_connection`
     and `session.ping`. Anything else gets `CHOOSE_CONNECTION_FIRST`.
  4. If no connection answers within `timeout_ms` (30 seconds), the server
     keeps the connection that was active most recently and closes the rest.

  The game clock is not paused while the player chooses. If it is their
  turn and the clock is running, the time spent choosing comes off their
  clock.

```json
{"evt":"session.choose_connection","game":1043,"timeout_ms":30000,"ts":1791234563000}
{"cmd":"session.keep_connection","id":"k1"}
{"evt":"ack","id":"k1","ts":1791234564000}
```

| Event or command | Fields |
| --- | --- |
| `session.choose_connection` (event) | `game` (integer): the tournament game. `timeout_ms` (integer): how long the player has to choose. |
| `session.keep_connection` (command) | none. Reply: `ack`. Error: `NOT_FOUND` if there is nothing to choose. |

### After reconnecting

A new connection starts empty. Here is what carries over from your old
connections and what does not.

| Kept | Gone |
| --- | --- |
| Your games in progress. `session.welcome` lists them. Send `game.attach` for each one. | Your seeks, if your last connection closed. Post them again. |
| Your account, roles, and any open draw offers or undo requests in your games | Games you were observing. Send `game.observe` again. |
| Your private rooms, while the other player is still online. `session.welcome` lists them. | Chat rooms you joined with `chat.join`. Send `chat.join` again. |

### Guests

Guests can play, observe, chat and open private rooms. They cannot create or
accept rated or tournament seeks. Those get `PERMISSION_DENIED`.

## 4. Players

Seeks and games describe players with a player record.

```json
{"username":"bob","is_bot":false,"rating":1520}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `username` | string | yes | The player's name |
| `is_bot` | boolean | yes | `true` if the account has the `bot` role |
| `rating` | integer | no | The player's rating. Left out for guests and for accounts a moderator has excluded from ratings. |

Where the rating comes from:

- **In a game,** it is the rating when the game started. It does not change
  during the game.
- **In a seek,** it is the rating when the seek was created. The server
  refreshes it about once an hour, after ratings are recalculated, and sends
  `seek.updated` for each seek whose rating changed.

## 5. Bots

A bot is a registered account with the `bot` role. Each bot has one owner,
a regular player account. A moderator or admin sets the owner. A bot uses
the same protocol as any other client. This section covers what is
different for bots.

### Getting a bot account

Bot management will move to the website later (see [Planned](#14-planned)).
For now:

1. Register the bot's account with `POST /api/v1/auth/register`. It can use
   your own email address, because email addresses do not have to be
   unique.
2. Ask a moderator or admin to give the account the `bot` role and to set
   you as its owner.
3. Log in as yourself and create the bot's API key with
   `POST /api/v1/bots/{name}/api-key`.
4. Connect the bot and log in with the key.

### Logging in

A bot can log in in two ways. Both give the same session.

**Option 1: access token.** This is the same flow as for any registered
player. Use it if your bot already speaks HTTP, or if you do not want a
long-lived secret on disk.

1. Call `POST /api/v1/auth/login` with the bot's username and password.
   Keep both tokens.
2. Send the `accessToken` in `session.login` as `token`.
3. On the next login, use the same `accessToken` while it has not expired.
   It lasts 1 hour (see `accessTokenExpiresAt`). After that, or if
   `session.login` returns `AUTH_FAILED`, call `POST /api/v1/auth/refresh`
   with the `refreshToken`. You get a new `accessToken` and a new
   `refreshToken`. The old refresh token stops working, so save the new one.

A refresh token lasts 30 days. A bot that is offline for longer must log in
with its password again.

**Option 2: API key.** One string in a config file. No HTTP is needed.

```json
{"cmd":"session.login","id":"r1","api_key":"ptk_3f9a..."}
```

- Each bot has at most one key. The bot's owner, a moderator or an admin
  creates it through the [REST API](#bot-api-keys).
- The server shows the full key once, when it is created, and stores only a
  hash. If you lose the key, create a new one. The new key replaces the old
  one.
- Keys do not expire. A key works until it is replaced or revoked, the
  account is banned, or the `bot` role is removed.
- Replacing or revoking a key closes every connection that logged in with
  it. `admin.revoke` also revokes the key.
- A bad or revoked key gets `AUTH_FAILED`.

Keep the key secret. Anyone with the key can play as your bot. Do not commit
it to a public repository.

### A minimal bot

```
connect, then send session.init
send session.login with the api_key
send seek.create

on session.games:  send game.attach for each game you have not attached
on game.start:     note your color (the "color" field)
on game.state:     if turn is your color: think, then send game.move
on game.over:      send seek.create again
on ILLEGAL_MOVE or NOT_YOUR_TURN: send game.sync, then think again
on session.closed or a dropped connection: reconnect (see When to reconnect)
```

### Restarting

If the bot stops or loses its connection, its games carry on and it is away
from them. It has 2 minutes to come back, or less if its clock runs out first. Otherwise it loses those games
with `reason: "abandon"`.

After the bot reconnects:

- Send `game.attach` for each game in `session.welcome`.
- Post your seeks again. The server removed them when the last connection
  closed.

### Draw offers and undo requests

The bot author decides what the bot does with them. The bot can accept
(`game.offer_draw`, `game.request_undo`), decline (`game.decline_draw`,
`game.decline_undo`), or ignore them. An ignored draw offer stays open until
it is withdrawn or the game ends. An ignored undo request ends when a move
is played.

### Recovering from errors

If `game.move` returns `ILLEGAL_MOVE` or `NOT_YOUR_TURN`, your copy of the
board is probably out of date. Send `game.sync` to get the current state,
then think again.

### Komi

`half_komi` is the same value that TEI engines call HalfKomi. Pass it to the
engine as it is.

### Limiting how many games a bot plays

The server has no game limit. To stay under your own limit, post a seek only
while your games in progress plus your open seeks are below that limit. An
accepted seek is removed as its game starts, so the total never goes up.
When a game ends, post a new seek. You never need to cancel a seek that
someone might accept at the same moment.

To stop one player from taking several of your games at once, create your
seeks with `"one_game_per_opponent":true`.

### Keep reading while the engine thinks

Read from the connection on its own thread or task. If the bot stops
reading while the engine searches, the server disconnects it with
`not_reading` (see [When the server stops sending](#when-the-server-stops-sending)).

### First-move rule

When a bot plays black, white must make the first move within 5 minutes, or
within the starting time if that is shorter. Otherwise the bot wins with
`reason: "abandon"`. This stops a player from holding a bot in a game that
never starts.

## 6. Seeks

### Seek record

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `seek` | integer | yes | Seek ID |
| `player` | player | yes | Player who posted it |
| `size` | integer | yes | Board size, 3 to 8 |
| `time` | integer | yes | Starting time per player, ms. Must be more than 0. Untimed games may come later, with async games. |
| `increment` | integer | yes | Added after each move, ms |
| `color` | string | yes | Color the poster plays: `"white"`, `"black"` or `"any"`. With `"any"`, the server picks the colors at random when the game starts. |
| `half_komi` | integer | yes | Komi in half flats, 0 to 8. `5` means 2.5 flats. |
| `pieces` | integer | yes | Flat stones per player |
| `capstones` | integer | yes | Capstones per player |
| `rated` | boolean | yes | Whether the game is rated |
| `tournament` | boolean | yes | Whether it is a tournament game. Anyone can create one for now. This may be limited to tournament tools later. |
| `opening` | string | yes | `"swap"` (normal) or `"double_black_stack"` |
| `increment_scales` | boolean | yes | `true` if the increment grows with each move. See [Time rules](#time-rules). |
| `one_game_per_opponent` | boolean | yes | `true` if a player who already has a game in progress with the poster cannot accept |
| `opponent` | string | no | Only this player can accept. Left out if open. |
| `rematch_of` | integer | no | The game this seek is a rematch of. Left out if it is not a rematch. |
| `extra_time_move` | integer | no | The move number, counted for each player separately, on which that player gets `extra_time`. Left out if none. |
| `extra_time` | integer | no | Extra time added at that move, ms. Sent with `extra_time_move`. |

You only see seeks you can accept. The server filters them:

- by the version you sent in `session.init`
- by `opponent`: the server sends a seek for a named opponent, including a
  rematch, only to its poster and that opponent. Its `seek.added`, `seek.updated` and
  `seek.removed` events go only to those two as well.

A seek for an opponent who is offline stays open. They get it in
`seek.list` when they log in.

### Time rules

- **Increment** (`increment`) is the time added to your clock after each of
  your moves.
- **Scaled increment** (`increment_scales: true`) multiplies the increment
  by your move number, counted for you alone. With `increment` 1 second:
  move 1 adds 1 second, move 2 adds 2, move 3 adds 3. With 2 seconds: 2, 4,
  6. Without it, every move adds the same amount.
- **Extra time** (`extra_time_move` and `extra_time`) is added once, when a
  player makes their own move with that number. For example,
  `extra_time_move` 35 and `extra_time` 300000 give each player 5 more
  minutes on their 35th move.

### Allowed values

| Field | Allowed |
| --- | --- |
| `size` | 3 to 8 |
| `half_komi` | 0 to 8 |
| `time` | more than 0 |
| `increment`, `extra_time` | 0 or more |
| `extra_time_move` | 1 or more |
| `pieces` | 10 to 80 |
| `capstones` | 0 to 5 |

A value outside these ranges gets `INVALID_FIELD`, with `field` naming it,
and no seek is created. The server does not quietly change it to a value it
allows. The text protocol is frozen, so text clients keep today's
behavior: the server changes an out-of-range value to the nearest allowed
one.

Standard pieces for each size, used when `pieces` or `capstones` is left
out:

| Size | 3 | 4 | 5 | 6 | 7 | 8 |
| --- | --- | --- | --- | --- | --- | --- |
| `pieces` | 10 | 15 | 21 | 30 | 40 | 50 |
| `capstones` | 0 | 0 | 1 | 1 | 2 | 2 |

### seek.list (event)

Sent after login and in reply to the `seek.list` command.

```json
{"evt":"seek.list","seeks":[{"seek":498,"player":{"username":"bob","is_bot":false,"rating":1520},"size":6,"time":900000,"increment":15000,"color":"any","half_komi":4,"pieces":30,"capstones":1,"rated":true,"tournament":false,"opening":"swap","increment_scales":false,"one_game_per_opponent":false}],"ts":1791234560100}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `seeks` | list of seek records | yes | Every open seek you can play |

### seek.added, seek.updated (events)

A new seek, or a change to an open seek (for example a refreshed rating).
Both carry a full seek record at the top level. For `seek.updated`, replace
the seek you have with the same `seek` ID.

```json
{"evt":"seek.added","seek":512,"player":{"username":"carol","is_bot":true,"rating":1700},"size":5,"time":600000,"increment":10000,"color":"white","half_komi":0,"pieces":21,"capstones":1,"rated":true,"tournament":false,"opening":"swap","increment_scales":false,"one_game_per_opponent":true,"ts":1791234561000}
```

### seek.removed (event)

```json
{"evt":"seek.removed","seek":512,"ts":1791234562000}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `seek` | integer | yes | The seek that was removed |

### seek.create (command)

Posts a new seek. You can have several open at once, for example one per
time control. Your seeks belong to you, not to a connection. They stay open
until they are accepted or cancelled, or your last connection closes.
Starting a game removes only the seek that was accepted, except for
tournament games (see [Connections and games](#connections-and-games)).

```json
{"cmd":"seek.create","id":"r3","size":6,"time":900000,"increment":15000,"color":"any","half_komi":4,"rated":true}
```

| Field | Type | Req. | Default |
| --- | --- | --- | --- |
| `size` | integer | yes | |
| `time` | integer | yes | |
| `increment` | integer | no | `0` |
| `color` | string | no | `"any"` |
| `half_komi` | integer | no | `0` |
| `pieces` | integer | no | Standard for the size (see [Allowed values](#allowed-values)) |
| `capstones` | integer | no | Standard for the size (see [Allowed values](#allowed-values)) |
| `rated` | boolean | no | `true`, or `false` for guests |
| `tournament` | boolean | no | `false` |
| `opening` | string | no | `"swap"` |
| `increment_scales` | boolean | no | `false` |
| `one_game_per_opponent` | boolean | no | `false` |
| `opponent` | string | no | Open to anyone |
| `extra_time_move` | integer | no | None |
| `extra_time` | integer | with `extra_time_move` | None |

Reply: `ack` with `seek`, the new seek's ID.

```json
{"evt":"ack","id":"r3","seek":499,"ts":1791234561500}
```

Errors: `INVALID_FIELD`, `UNSUPPORTED_VERSION`, `TOURNAMENT_IN_PROGRESS`,
`PERMISSION_DENIED` (a guest asking for a rated or tournament seek),
`NOT_FOUND` (`opponent` is not a registered player, or a guest who is not
online).

### seek.cancel (command)

```json
{"cmd":"seek.cancel","id":"r5","seek":512}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `seek` | integer | yes | One of your seeks |

Reply: `ack`, then `seek.removed`.

Errors: `NOT_FOUND`, `PERMISSION_DENIED` (not your seek).

### seek.accept (command)

```json
{"cmd":"seek.accept","id":"r4","seek":498}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `seek` | integer | yes | The seek to accept |

Reply: `ack` with `game`, the new game's ID. Every connection of both
players then gets `session.games`. Attach with `game.attach` to get
`game.start` and `game.state`.

```json
{"evt":"ack","id":"r4","game":1043,"ts":1791234563000}
```

If two players accept the same seek at once, the first one gets the game.
The second gets `NOT_FOUND`, because the seek is gone.

Errors: `NOT_FOUND` (no such seek, or someone else accepted it first),
`PERMISSION_DENIED` (the seek is for another opponent,
or you are a guest and the seek is rated or a tournament seek),
`ALREADY_PLAYING` (the seek has `one_game_per_opponent` and you already
have a game in progress with its poster), `GAMES_IN_PROGRESS` (a tournament
seek, and you or the poster have another game in progress),
`UNSUPPORTED_VERSION`, `TOURNAMENT_IN_PROGRESS`.

### seek.rematch (command)

Offers a rematch of a finished game. A rematch is a new seek for the same
game settings, with `opponent` set to your opponent and `color` set to the
color they played. Only the two of you see it.

```json
{"cmd":"seek.rematch","id":"r6","game":1043}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `game` | integer | yes | A finished game you played in |

Reply: `ack` with `seek`, the new seek's ID. Your opponent accepts it with
`seek.accept`, like any seek.

If your opponent already offered a rematch of the same game, yours accepts
theirs instead. The `ack` then carries `game`, the new game's ID, as for
`seek.accept`.

Errors: `NOT_FOUND` (no such finished game), `NOT_IN_GAME`.

### seek.list (command)

No fields. Reply: `ack`, then a fresh `seek.list` event.

## 7. Games

### Game record

Used in `game.list` and `game.added`.

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `game` | integer | yes | Game ID |
| `white` | player | yes | White player |
| `black` | player | yes | Black player |
| `size`, `time`, `increment`, `half_komi`, `pieces`, `capstones`, `rated`, `tournament`, `opening`, `increment_scales` | | yes | As in the [seek record](#seek-record) |
| `extra_time_move`, `extra_time` | integer | no | As in the seek record |

A game record has every game rule. `game.start` adds your color and the
moves.

### game.list, game.added, game.removed (events)

Like the seek list, for games in progress. The server sends `game.list`
after login and in reply to the `game.list` command.

```json
{"evt":"game.list","games":[{"game":1043,"white":{"username":"alice","is_bot":false,"rating":1610},"black":{"username":"bob","is_bot":false,"rating":1520},"size":6,"time":900000,"increment":15000,"half_komi":4,"pieces":30,"capstones":1,"rated":true,"tournament":false,"opening":"swap","increment_scales":false}],"ts":1791234560100}
{"evt":"game.added","game":1044,"white":{"username":"carol","is_bot":true,"rating":1700},"black":{"username":"dave","is_bot":false},"size":5,"time":600000,"increment":10000,"half_komi":0,"pieces":21,"capstones":1,"rated":false,"tournament":false,"opening":"swap","increment_scales":false,"ts":1791234561000}
{"evt":"game.removed","game":1044,"ts":1791234590000}
```

| Event | Fields |
| --- | --- |
| `game.list` | `games`: list of game records |
| `game.added` | a game record at the top level |
| `game.removed` | `game`: integer |

### game.start (event)

Sent in reply to `game.attach`, `game.observe` and `game.sync`. It has the
fixed details and the full history. A `game.state` follows right after.

```json
{"evt":"game.start","game":1043,"white":{"username":"alice","is_bot":false,"rating":1610},"black":{"username":"bob","is_bot":false,"rating":1520},"color":"black","size":6,"time":900000,"increment":15000,"half_komi":4,"pieces":30,"capstones":1,"rated":true,"tournament":false,"opening":"swap","increment_scales":false,"moves":["a6","f1","d4","c3"],"ts":1791234563000}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `game` | integer | yes | Game ID |
| `white`, `black` | player | yes | The players |
| `color` | string | no | Your color, `"white"` or `"black"`. Left out if you are observing. |
| `size`, `time`, `increment`, `half_komi`, `pieces`, `capstones`, `rated`, `tournament`, `opening`, `increment_scales` | | yes | As in the [seek record](#seek-record) |
| `extra_time_move`, `extra_time` | integer | no | As in the seek record |
| `moves` | list of string | yes | Every move so far, in [PTN](#moves) |

### game.state (event)

Sent after every change while the game is in progress: a move, an undo, an
offer, or a clock change. It goes to every connection attached to the game,
for both players, and to every observer. It always has the full live
position, so you never need to compare states. The end of the game is a
separate event, [game.over](#gameover-event).

```json
{"evt":"game.state","game":1043,"ply":24,"change":"move","last":"3c3>12","tps":"x6/x6/x2,1,x3/x6/x6/x6 2 12","turn":"black","white_time":598000,"black_time":587900,"clock_running":true,"ts":1791234567001}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `game` | integer | yes | Game ID |
| `ply` | integer | yes | Number of plies played. Goes up by one per move, down on undo. |
| `change` | string | yes | What changed. See the table below. |
| `last` | string | no | The move just played. Sent when `change` is `move`. |
| `tps` | string | yes | The position, in TPS |
| `turn` | string | yes | `"white"` or `"black"` |
| `white_time`, `black_time` | integer | yes | Time left at `ts`, ms |
| `clock_running` | boolean | yes | Whether the clock of the side to move is counting down. See below. |
| `draw_offer`, `undo_offer` | string | no | Username of the player with an open offer. Left out if none. |

What changed:

| `change` | Meaning |
| --- | --- |
| `move` | A move was played. See `last`. |
| `undo` | The last move was undone |
| `draw_offer` | A draw offer was made, withdrawn or declined. The `draw_offer` field shows who has one open, or is left out. |
| `undo_offer` | An undo request was made, withdrawn or declined. The `undo_offer` field shows who has one open, or is left out. |
| `time_given` | A player gave their opponent time |
| `sync` | Nothing changed. This is the state sent right after `game.start`. |

**Showing the clocks.** white_time and black_time are the time left when the server sent the message. Count down the side to move from when you receive it, while clock_running is true. Your display can be off by about the network delay, usually well under a second. Each game.state gives the exact time again. The server's clock is the one that counts.

`clock_running` is `false` before the first move and on a move that ends
the game. Do not work it out from `ply`: once the clock has started, it
keeps running even if every move is undone back to ply 0.

**Staying in sync.** If `ply` moves by more than one from what you expect,
send `game.sync`. You get a fresh `game.start` and `game.state`.

### game.over (event)

Sent once when the game ends, to every connection attached to the game and
to every observer. When a move ends the game (a road or flats), the
`game.state` for that move comes first, so you always see the final move.
Any other ending (resign, time, abandon, agreement, ban) sends only
`game.over`.

```json
{"evt":"game.over","game":1043,"result":"R-0","reason":"road","winner":"alice","ts":1791234599000}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `game` | integer | yes | Game ID |
| `result` | string | yes | In PTN result notation: `"R-0"` or `"0-R"` for a road, `"F-0"` or `"0-F"` for flats, `"1/2-1/2"` for any draw, and `"1-0"` or `"0-1"` for every other win (`resign`, `time`, `abandon`, `ban`). The first character is white's result. |
| `reason` | string | yes | Why. See the table below. |
| `winner` | string | no | The winner's username. Left out for a draw. |

Reasons a game ends:

| `reason` | Meaning |
| --- | --- |
| `road` | A player made a road |
| `flats` | The board filled or a player ran out of pieces, and the flat count (with komi) decided it. With `"1/2-1/2"`, the count was tied. |
| `resign` | A player resigned |
| `time` | A player ran out of time |
| `abandon` | A player left and did not come back in time, or did not make the first move against a bot in time (see [Bots](#5-bots)) |
| `agreement` | Both players agreed to a draw |
| `no_progress` | A draw after 50 plies with no placement and no wall flattened |
| `ban` | A player was banned during the game |

### Moves

Moves are in [PTN](https://ustak.org/portable-tak-notation/), in both
directions.

**From the server**, every move uses the full form, so one pattern parses
them all:

| Move | Form | Examples |
| --- | --- | --- |
| Flat | square | `a1` |
| Wall | `S` + square | `Sa1` |
| Capstone | `C` + square | `Ca1` |
| Stack move | count, square, direction, every drop, then `*` if a wall was flattened | `1a1>1`, `1a2+1`, `3c3>3`, `3c3>12`, `1c3>1*` |

- The count and every drop are always written, even when they are 1.
- `F` is never written. A placement with no letter is a flat.
- `*` is always written when a capstone flattens a wall.
- Directions are `<`, `>`, `+` and `-`.
- Files are lowercase, and piece letters uppercase.
- There are no annotations or comments.

Patterns for the server's form:

| Move | Pattern |
| --- | --- |
| Placement | `^[SC]?[a-h][1-8]$` |
| Stack move | `^[1-8][a-h][1-8][<>+-][1-8]+\*?$` |

**To the server**, `game.move` accepts the full form (recommended) or the
short form:

| Part | Accepted |
| --- | --- |
| Count | Optional when it is 1: `a1>` means `1a1>1` |
| Drops | Optional for a single drop of the whole count: `3c3>` means `3c3>3` |
| `F` | Optional: `Fa1` means `a1` |
| `*` | Optional, but recommended. If you send it and the move does not flatten a wall, you get `ILLEGAL_MOVE`. A move that flattens a wall without `*` is accepted. |
| Directions | `<`, `>`, `+` and `-` only |

Annotations (`'`, `''`, `!`, `?`), comments, and wrong case, such as `A1` or
`sa1`, get `INVALID_FIELD`.

**Compare plies, not move strings.** The server rewrites every move into
its full form, so `last` may not match what you sent. You send `a1>` and
get back `1a1>1`. Track moves by `ply`.

A later minor version can add other move or position formats as an option
in `session.init`. PTN stays the default.

### Game commands

Every command here takes `game` (integer, required): the game ID.

| Command | Other fields | Reply and result | Errors |
| --- | --- | --- | --- |
| `game.move` | `move` (string, required): the move in PTN | `ack`, then `game.state` with `change: "move"` | `NOT_FOUND`, `NOT_IN_GAME`, `NOT_YOUR_TURN`, `ILLEGAL_MOVE` |
| `game.resign` | | `ack`, then `game.over` | `NOT_FOUND`, `NOT_IN_GAME` |
| `game.offer_draw` | | Offers a draw, or accepts the opponent's offer | `NOT_FOUND`, `NOT_IN_GAME` |
| `game.withdraw_draw` | | Withdraws your offer | `NOT_FOUND`, `NOT_IN_GAME`, `NO_OPEN_OFFER` |
| `game.decline_draw` | | Declines the opponent's draw offer | `NOT_FOUND`, `NOT_IN_GAME`, `NO_OPEN_OFFER` |
| `game.request_undo` | | Requests an undo, or accepts the opponent's request | `NOT_FOUND`, `NOT_IN_GAME`, `NOTHING_TO_UNDO` |
| `game.withdraw_undo` | | Withdraws your request | `NOT_FOUND`, `NOT_IN_GAME`, `NO_OPEN_OFFER` |
| `game.decline_undo` | | Declines the opponent's undo request | `NOT_FOUND`, `NOT_IN_GAME`, `NO_OPEN_OFFER` |
| `game.give_time` | | Gives the opponent 15 seconds | `NOT_FOUND`, `NOT_IN_GAME`, `CLOCK_NOT_STARTED`, `COOLDOWN` |
| `game.attach` | | Attaches this connection to one of your games in progress: `ack`, then `game.start` and `game.state` | `NOT_FOUND` (no such game, or it has ended), `NOT_IN_GAME`, `UNSUPPORTED_VERSION` |
| `game.detach` | | Stops this connection getting the game's events. If no connection of yours is still attached, you are away from the game. On a game that has ended: `ack` and nothing else. | `NOT_FOUND`, `GAME_NOT_ATTACHED` |
| `game.observe` | | `ack`, then `game.start` and `game.state` | `NOT_FOUND`, `UNSUPPORTED_VERSION`, `TOURNAMENT_IN_PROGRESS`, `PERMISSION_DENIED` (you play in this game: attach instead) |
| `game.unobserve` | | `ack`. On a game that has ended: `ack` and nothing else. | `NOT_FOUND` |
| `game.sync` | | `ack`, then `game.start` and `game.state` | `NOT_FOUND`, `UNSUPPORTED_VERSION` |

`game.list` takes no fields. Reply: `ack`, then a fresh `game.list` event.

Commands that play in a game (`game.move` to `game.give_time`) need the game
attached to this connection. Otherwise they get `GAME_NOT_ATTACHED`.

A player cannot observe their own game. To follow it on another tab or
device, attach to it there.

A move is undone only after both players request it. A draw offer stays
open until it is withdrawn, declined, or the game ends. An undo request also
ends when a move is played. To decline, send `game.decline_draw` or
`game.decline_undo`, or ignore the offer. A player can give time once every
15 seconds, and not before the first move.

**When a game ends,** every attached connection and every observer gets the
`game.over`, then is detached or stops observing automatically. The server
sends no other event. Rematches (`seek.rematch`) and the
game's chat room keep working after this. A finished game is read from the
[game history](#addresses), not attached to.

### game.unobserved (event)

Sent when the server stops your observing, for example because you started
a tournament game.

```json
{"evt":"game.unobserved","game":1043,"reason":"tournament","ts":1791234563500}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `game` | integer | yes | The game you were observing |
| `reason` | string | yes | `tournament` |

### Disconnects

A player is away from a game when no connection of theirs is attached to
it: their last attached connection closed or detached, or no connection
attached after the game started. Closing or detaching one of several
attached connections does not make them away. The opponent and observers
get:

```json
{"evt":"game.player_away","game":1043,"username":"bob","reconnect_ms":120000,"ts":1791234570000}
{"evt":"game.player_back","game":1043,"username":"bob","ts":1791234580000}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `game` | integer | yes | Game ID |
| `username` | string | yes | The player who left or came back |
| `reconnect_ms` | integer | `player_away` only | Time the player has to come back, ms. 2 minutes, or 15 minutes in a tournament game, or less if their clock runs out first. |

The server sends `game.player_away` for each game the player is away from. If the
player does not come back in time, the game ends with `reason: "abandon"`. A
player can also run out of time while away.

To come back, log in again and send `game.attach` for the game.
`session.welcome` lists your games in progress. The server sends
`game.player_back` when a connection of yours attaches again.

## 8. Chat

### Rooms

| Room | Who is in it |
| --- | --- |
| `global` | Everyone. Joined at login. |
| `game:1043` | Anyone who joins. The server creates it when game 1043 starts. |
| `private:7` | Two players chatting privately. See [Private rooms](#private-rooms). |
| any other name | Players who joined it |

Room names:

- A name is 1 to 64 characters, with no spaces.
- `global` and names that start with `game:` or `private:` are reserved.
  You cannot create them with `chat.join`.
- Joining any other name creates the room if it does not exist. The server
  removes a room when its last member leaves.
- Room names are not filtered. Moderators can kick players who misuse
  them.

You join rooms per connection. Each connection joins its own rooms, and a
new connection must join them again.

Anyone can join a game's room, including players who are not in or
observing the game. The server does not put players or observers in it.
The client decides whether to join, for example automatically when it
attaches to or observes a game. A game's room stays open after the game
ends, for post-game chat, until its last member leaves.

### Private rooms

A private room is a conversation between two players. It works like any
other room for sending and receiving (`chat.send` and `chat.message`), so
you show each message in the room's tab and never compare usernames. It
differs in who is in it and how long it lasts:

- **Opening.** Send `chat.open` with the other player's username. The `ack`
  has the room ID, which the server makes. Opening a room with the same
  player again gives the same room.
- **Members are players, not connections.** Every connection of both
  players gets the room's messages, including the sender's other tabs and
  devices. You do not `chat.join` a private room, and you cannot leave
  one: to hide it, close its tab in your client.
- **Finding out about a room.** When a room is created, every connection
  of both players gets `chat.room`. A new connection finds its rooms in
  `session.welcome`.
- **Offline players.** If the other player is not online, `chat.send` gets
  `NOT_ONLINE` and the message is not kept. The room stays while either
  player is online. Once both are offline, the server removes it, and a
  later `chat.open` makes a new room with a new ID.
- **Privacy.** Only the two players can see a private room. Moderators
  cannot, and message text is not logged. Gags apply as in other rooms.

Text protocol players can be in private rooms too. They see the room as
`Tell` and `Told` lines, and their `Tell` goes to the same room, so text
and JSON players can chat privately.

### Commands

| Command | Fields | Reply | Errors |
| --- | --- | --- | --- |
| `chat.join` | `room` (string, required) | `ack`, then `chat.joined` to the room | `INVALID_FIELD` (bad room name), `PERMISSION_DENIED` (a reserved name, including a private room), `NOT_FOUND` (a game room for a game that does not exist) |
| `chat.leave` | `room` (string, required) | `ack`, then `chat.left` to the room | `NOT_IN_ROOM` (including a private room) |
| `chat.send` | `room` (string, required), `text` (string, required) | `ack`, then `chat.message` to the room | `NOT_IN_ROOM`, `NOT_ONLINE` (a private room whose other player is offline), `INVALID_FIELD`, `PERMISSION_DENIED` (you are gagged) |
| `chat.open` | `username` (string, required) | `ack` with `room`. If the room is new, `chat.room` to both players. | `NOT_ONLINE`, `INVALID_FIELD` (including your own username) |

```json
{"cmd":"chat.send","id":"r19","room":"game:1043","text":"nice road"}
{"evt":"chat.message","room":"game:1043","username":"alice","text":"nice road","ts":1791234590000}
{"cmd":"chat.open","id":"r20","username":"alice"}
{"evt":"ack","id":"r20","room":"private:7","ts":1791234590500}
{"evt":"chat.room","room":"private:7","members":["alice","bob"],"ts":1791234590500}
{"cmd":"chat.send","id":"r21","room":"private:7","text":"gg"}
{"evt":"chat.message","room":"private:7","username":"bob","text":"gg","ts":1791234591000}
{"evt":"chat.joined","room":"game:1043","username":"carol","ts":1791234592000}
{"evt":"chat.left","room":"game:1043","username":"carol","ts":1791234593000}
```

### Events

| Event | Fields | Sent to |
| --- | --- | --- |
| `chat.message` | `room` (string), `username` (string): the sender, `text` (string) | Everyone in the room |
| `chat.room` | `room` (string), `members` (list of string): both players | Every connection of both players, when a private room is created |
| `chat.joined` | `room` (string), `username` (string) | Everyone in the room, including the player who joined. Not sent for `global` or private rooms. |
| `chat.left` | `room` (string), `username` (string) | Everyone in the room, including the player who left. Not sent for `global` or private rooms. |

Messages are limited to 256 characters. Chat is not filtered; moderators
handle abuse with gag, kick and ban.

Messages reach only players who are online. Chat history, messages to
offline players and room member lists may come later (see
[Planned](#14-planned)).

## 9. Moderation

Moderators and admins can use these commands. They match the text protocol
`sudo` commands.

| Command | Fields | Who | `ack` fields |
| --- | --- | --- | --- |
| `admin.gag`, `admin.ungag` | `username` (string) | mod | none |
| `admin.kick` | `username` (string) | mod | none |
| `admin.ban` | `username` (string), `reason` (string) | mod | none |
| `admin.unban` | `username` (string) | mod | none |
| `admin.list` | `group` (string): `gag`, `mod`, `ban`, `admin` or `online` | mod | `usernames`: list of string, sorted |
| `admin.broadcast` | `text` (string) | admin | none |
| `admin.set_role` | `username` (string), `role` (string): `mod`, `admin` or `bot`, `enabled` (boolean) | admin | `changed`: boolean, `false` if the player already had that setting |
| `admin.revoke` | `username` (string) | admin | none |
| `admin.reload_words` | none | admin | `words`: integer, the number of words loaded |

All fields are required.

```json
{"cmd":"admin.ban","id":"a1","username":"spammer","reason":"Spamming chat"}
{"evt":"ack","id":"a1","ts":1791234600000}

{"cmd":"admin.list","id":"a2","group":"mod"}
{"evt":"ack","id":"a2","usernames":["alice","bob"],"ts":1791234600100}

{"cmd":"admin.set_role","id":"a3","username":"MyBot","role":"bot","enabled":true}
{"evt":"ack","id":"a3","changed":true,"ts":1791234600200}

{"cmd":"admin.reload_words","id":"a4"}
{"evt":"ack","id":"a4","words":412,"ts":1791234600300}
```

Moderators act only on players. Admins act on players and moderators.
Nobody acts on themselves.

- `admin.kick` closes the player's connections with `session.closed`
  `reason: "kicked"`. Guests can only be kicked.
- `admin.ban` works on offline players too. It closes the player's
  connections with `reason: "banned"`, revokes their tokens and API key, and
  ends their games in progress with `reason: "ban"`.
- `admin.revoke` revokes the player's access tokens, refresh tokens and API
  key, and closes their connections with `reason: "revoked"`.

Setting a player's password is done through the
[REST API](#admin), since the JSON protocol never carries passwords.

Errors: `PERMISSION_DENIED`, `NOT_FOUND`, `INVALID_FIELD`.

## 10. Errors

```json
{"evt":"error","id":"r9","code":"NOT_YOUR_TURN","message":"Not your turn","ts":1791234567000}
{"evt":"error","code":"INVALID_JSON","message":"Expected a JSON object","ts":1791234600000}
```

| Field | Type | Req. | Meaning |
| --- | --- | --- | --- |
| `code` | string | yes | Stable. Write your code against it. |
| `message` | string | yes | For people. It can change. |
| `field` | string | no | With `INVALID_FIELD`: the field that is wrong |
| `retry_ms` | integer | no | With `RATE_LIMITED` and `COOLDOWN`: how long to wait |
| `game` | integer | no | With `UNSUPPORTED_VERSION`: the game it is about |
| `min_version` | string | no | With `UNSUPPORTED_VERSION`: the version needed |

An error with no `id` is a server notice, not a reply.

| Code | Meaning |
| --- | --- |
| `INVALID_JSON` | The message is not valid JSON. No `id`. |
| `UNKNOWN_COMMAND` | The `cmd` name is not known |
| `INVALID_FIELD` | A field is missing or wrong. See `field`. |
| `UNSUPPORTED_VERSION` | Your protocol version is not supported, or a seek or game needs a newer version. See `min_version`. |
| `INIT_REQUIRED` | Send `session.init` first |
| `ALREADY_INITIALIZED` | `session.init` was already sent on this connection |
| `NOT_AUTHENTICATED` | Log in first |
| `ALREADY_LOGGED_IN` | This connection is already logged in |
| `AUTH_FAILED` | Bad or expired token, or a bad API key |
| `RATE_LIMITED` | Too many requests. See `retry_ms`. |
| `NOT_FOUND` | The seek, game, room or player does not exist |
| `NOT_ONLINE` | The player is not online |
| `NOT_IN_GAME` | You are not playing in that game |
| `ALREADY_PLAYING` | You already have a game in progress with this player, and their seek allows only one |
| `NOT_YOUR_TURN` | It is your opponent's move |
| `ILLEGAL_MOVE` | The move breaks the rules. `message` says why. |
| `NOTHING_TO_UNDO` | No move has been played |
| `NO_OPEN_OFFER` | There is no open draw offer or undo request to withdraw or decline |
| `CLOCK_NOT_STARTED` | Time cannot be given before the first move |
| `COOLDOWN` | Wait before giving time again. See `retry_ms`. |
| `NOT_IN_ROOM` | Join the room first |
| `PERMISSION_DENIED` | You may not do this |
| `TOURNAMENT_IN_PROGRESS` | You are in a tournament game, so you cannot start another game or observe a game |
| `GAMES_IN_PROGRESS` | A tournament game cannot start while you or your opponent have another game in progress. `message` says who. |
| `GAME_NOT_ATTACHED` | Attach this connection to the game with `game.attach` first |
| `CHOOSE_CONNECTION_FIRST` | Your tournament game has started and you have several connections. Send `session.keep_connection` on the one you want to keep. |
| `TIMEOUT` | The server did not finish in time. Try again. |
| `SERVER_ERROR` | Something failed on the server |

## 11. Versioning

### Version numbers

The protocol version is `major.minor`, for example `1.0` or `1.2`.

| Part | Changes when | Old clients |
| --- | --- | --- |
| Major | Something changes in a way that breaks clients: a field is removed or renamed, or its meaning or type changes | Keep their old major version. The server supports the old and new major versions together for an announced period. |
| Minor | Something is added: a command, an event, a field, an enum value, or a new game rule | Keep working. They ignore what they do not know, and never see games that need a newer minor version. |

The WebSocket subprotocol and query parameter name only the major version
(`playtak.json.v1`, `v=1`). The client sends its full version in
`session.init`.

A client that supports a minor version supports everything before it. A
`1.2` client supports all of `1.0`, `1.1` and `1.2`. There is one path
forward, with no optional parts to pick from.

### Version 1.0 is the full baseline

Every v1 client supports everything in this document: all board sizes,
komi, extra time, scaled increment, the Double Black Stack opening, and so
on. None of it is optional.

### Games that need a newer version

A new game rule bumps the minor version. The server knows which version
each seek and game needs, and:

- shows a client only the seeks and games it can play
- refuses `seek.accept`, `game.attach`, `game.observe` and `game.sync` with
  `UNSUPPORTED_VERSION` otherwise
- refuses `seek.create` with `UNSUPPORTED_VERSION` if the seek needs a newer
  version than the client sent

If a player logs in on an older client while one of their games needs a
newer one, `session.welcome` still lists the game, but `game.attach` is
refused. The game carries on. The player is away until they come back on a
client that supports it.

```json
{"evt":"error","id":"r7","code":"UNSUPPORTED_VERSION","game":1043,"min_version":"1.1","message":"This game needs a newer client","ts":1791234560300}
```

**The v1 core.** These never change while v1 exists:
`session.init`, `session.login`, `session.welcome`, `session.games`,
`session.ping`, `session.closed`, `seek.list`, `seek.create`,
`seek.accept`, `game.attach`, `game.start`, `game.state`, `game.over`, `game.move`,
`ack` and `error`. A client or bot that uses only these keeps working.

### Example: a new game rule

Say we add a new opening where the first move can be a wall.

1. Version `1.1` adds the value `"opening":"wall_first"`.
2. The server marks seeks and games using it as needing `1.1`.
3. Clients on `1.0` never see these seeks or games, and cannot accept them
   by mistake. Everything else works for them as before.
4. A client that adds support sends `"version":"1.1"` and sees everything.

No `1.0` client breaks.

### Example: a new field

Say version `1.2` adds a field to `game.state`: `"low_time":true`. The server
sends it to every client. `1.0` and `1.1` clients ignore it. No game needs
version `1.2` for this, because it does not change the rules.

### Example: a breaking change

Say we decide `white_time` should become an object. That breaks clients, so
it becomes version `2.0`. The server keeps serving `1.x` clients during an
announced period, then retires v1.

### Text protocol clients

The text protocol is feature-frozen. It gets fixes, but new features come
only to JSON. Text clients follow the same version rule, so text and JSON
players can play each other:

| Text protocol | Same as JSON version | Can play |
| --- | --- | --- |
| `Protocol 0` to `Protocol 3` | below `1.0` | games without Double Black Stack or scaled increment |
| `Protocol 4` | `1.0` | all `1.0` games |

Text clients never see games that need `1.1` or later. A text client that
logs in while the player has a game it cannot show is not sent that game. It
gets `Message You have a game in progress that needs a newer client`
instead.

### Version history

| Version | Changes |
| --- | --- |
| `1.0` | First version |

## 12. Limits

| Limit | Value |
| --- | --- |
| Message size, client to server | 64 KB. A larger message closes the connection with `message_too_big`. |
| Idle timeout | 60 seconds without traffic |
| Chat message | 256 characters |
| Chat messages per player | 10 per 10 seconds, shouts, room shouts and tells together |
| Commands per connection | 20 per second, with bursts up to 40 (proposed). Over the limit, you get `RATE_LIMITED`. |
| Failed logins per username | 10 per 15 minutes. A successful login resets it. |
| Failed logins per IP | 20 per minute. Successful logins do not count. |
| Registrations per IP | 5 per hour |
| Password reset emails per username | 3 per hour |
| `POST /api/v1/auth/refresh`, `/logout` and `/guest` per IP | 30 per minute, shared |
| `GET /api/v1/seeks` per IP | 60 per minute |

Login limits apply to the REST API and to `session.login` alike.

Server messages have no size limit. A full game snapshot is small: a
300-ply game is about 3 KB of moves, and a full TPS is under 400 bytes.

### When the server stops sending

The server keeps a queue of messages waiting to go out to each connection.
It is normally empty. It fills when a client does not take messages off the
connection fast enough.

| | `not_reading` | `send_queue_full` |
| --- | --- | --- |
| What happened | One send was stuck for 10 seconds | About 1,000 messages are waiting |
| Your client | Has stopped reading | Is reading, but more slowly than messages arrive |
| Common causes | A bot that reads in the same loop as its engine. A client paused in a debugger. An app frozen in the background. | A very slow network, such as a weak mobile signal, during a burst of messages |
| Do you get `session.closed`? | Usually not. The connection just drops. | Yes. The server drops the waiting messages, then sends it. |

Neither is caused by being slow to move, by a slow computer or UI, or by not
sending pings. Not sending anything gives `idle` instead (see
[session.closed](#sessionclosed-event)).

To avoid both, read from the connection on its own thread or task, and
never block that loop. If it happens, your games carry on and you are away.
Reconnect and attach to your games again.

## 13. REST API

Accounts, tokens and bot keys are managed over HTTPS, not over the socket.
See [Addresses](#addresses) for the base URL. The REST API follows its own
conventions, which differ from the socket protocol:

- Request and response bodies are JSON, with camelCase field names.
- Times are RFC 3339 strings, for example `"2026-10-15T18:00:00Z"`.
- Errors are `{"error":"<code>"}` with a lowercase code and an HTTP status.
- `429 Too Many Requests` has a `Retry-After` header, in seconds.
- Endpoints marked "Bearer" need `Authorization: Bearer <access token>`.
- Every request must send an `X-Client-Name` header. See
  [Client name header](#client-name-header).
- Cross-origin requests (CORS) are allowed from any website, so third-party
  web clients can call the API directly from the browser.
- Paths carry the API's major version, for example `/api/v1/auth/login`.
  Additions, such as a new endpoint or a new response field, stay in `v1`,
  and clients ignore fields they do not know. A breaking change gets a new
  version, such as `/api/v2/…`, and the server keeps serving `v1` beside it
  for an announced period. The REST version is separate from the socket
  protocol version.

Endpoints marked *(new)* do not exist yet. The examples use the beta
server.

### Client name header

Every REST request must say which client sent it, in an `X-Client-Name`
header. It is the REST version of `client_name` in `session.init`.

```
X-Client-Name: MyBot/2.1
```

- **Format:** the client's name, then optionally `/` and its version, for
  example `PlayTakUI/3.4` or `MyBot`. Up to 64 characters: letters, digits,
  and `.`, `_`, `-` and `/`.
- **Why:** it shows in the server's logs, so problems and heavy traffic can
  be traced to a client and its author contacted. It is not a password and
  gives no extra access.
- **Missing or invalid:** `400 client_name_required`. This applies to every
  endpoint, including ones that need no login.
- **Browsers:** a web page can set the header with `fetch`. The server
  allows it in cross-origin requests, so third-party web clients work.

```js
fetch("https://beta.playtak.com/api/v1/auth/guest", {
  method: "POST",
  headers: { "X-Client-Name": "MyWebClient/1.0" },
});
```

### Account rules

| Field | Rule |
| --- | --- |
| Username | 4 to 16 characters: a letter, then letters, digits or `_`. Must not contain "guest", or look like an existing name (names that differ only by case, or by look-alike characters, count as the same). |
| Email | Up to 30 characters before the `@` (letters, digits and ``.!#$%&'*+-=?^_``), and 3 to 30 after it (letters, digits, `.` and `-`) |
| Password | 6 to 50 characters, with no spaces |

Email addresses are not unique. Several accounts can share one, as many do
today. Password resets ask for the username as well as the email, so this
does not affect them.

### Accounts

#### POST /api/v1/auth/register

Creates an account. The password is emailed.

```json
{"username":"alice","email":"alice@example.com"}
```

| Status | Body |
| --- | --- |
| `201` | `{"name":"alice"}` |
| `400` | `invalid_username`, `invalid_email` or `guest_name` |
| `409` | `name_taken` |
| `429` | `rate_limited` |

<details>
<summary>Example</summary>

```sh
curl -X POST https://beta.playtak.com/api/v1/auth/register \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","email":"alice@example.com"}'
```

</details>

#### POST /api/v1/auth/login

```json
{"username":"alice","password":"correct horse"}
```

| Status | Body |
| --- | --- |
| `200` | A token pair (below) |
| `401` | `invalid_credentials` |
| `429` | `rate_limited` |

A token pair:

```json
{
  "name": "alice",
  "accessToken": "…",
  "accessTokenExpiresAt": "2026-10-15T19:00:00Z",
  "refreshToken": "…",
  "refreshTokenExpiresAt": "2026-11-14T18:00:00Z"
}
```

Access tokens last 1 hour. Refresh tokens last 30 days.

<details>
<summary>Example</summary>

```sh
curl -X POST https://beta.playtak.com/api/v1/auth/login \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"correct horse"}'
```

</details>

#### POST /api/v1/auth/refresh

```json
{"refreshToken":"…"}
```

| Status | Body |
| --- | --- |
| `200` | A new token pair. The old refresh token stops working. |
| `401` | `invalid_token` |
| `429` | `rate_limited` |

<details>
<summary>Example</summary>

```sh
curl -X POST https://beta.playtak.com/api/v1/auth/refresh \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Content-Type: application/json' \
  -d '{"refreshToken":"<refresh token>"}'
```

</details>

#### POST /api/v1/auth/guest

No body.

| Status | Body |
| --- | --- |
| `200` | `{"name":"Guest123","guestToken":"…","guestTokenExpiresAt":"2026-10-21T02:00:00Z"}` |
| `429` | `rate_limited` |

A guest token is valid for 8 hours after the guest's last activity: a ping,
a command or an API call. `guestTokenExpiresAt` is 8 hours after the token
was issued, and each use pushes it back. A connected guest never expires,
because pings count. After it expires, the guest name is gone, and a new
guest token gives a new name.

Each guest token is one guest. To keep the same guest across tabs, save the
token (for example in `localStorage`) and log in with it in each tab.

<details>
<summary>Example</summary>

```sh
curl -X POST https://beta.playtak.com/api/v1/auth/guest \
  -H 'X-Client-Name: MyClient/1.0'
```

</details>

#### POST /api/v1/auth/logout (Bearer)

```json
{"refreshToken":"…"}
```

| Status | Body |
| --- | --- |
| `204` | none. The tokens are revoked. |

<details>
<summary>Example</summary>

```sh
curl -X POST https://beta.playtak.com/api/v1/auth/logout \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Authorization: Bearer <access token>' \
  -H 'Content-Type: application/json' \
  -d '{"refreshToken":"<refresh token>"}'
```

</details>

#### POST /api/v1/auth/reset/request *(new)*

Emails a password reset token, valid for 1 hour.

```json
{"username":"alice","email":"alice@example.com"}
```

| Status | Body |
| --- | --- |
| `204` | none. Sent whether or not the name and email match, so the endpoint does not reveal accounts. |
| `429` | `rate_limited` (3 per hour per name) |

<details>
<summary>Example</summary>

```sh
curl -X POST https://beta.playtak.com/api/v1/auth/reset/request \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","email":"alice@example.com"}'
```

</details>

#### POST /api/v1/auth/reset/confirm *(new)*

```json
{"username":"alice","token":"…","password":"new password"}
```

| Status | Body |
| --- | --- |
| `204` | none. All of the account's tokens are revoked. |
| `400` | `invalid_token` or `invalid_password` |
| `429` | `rate_limited` |

<details>
<summary>Example</summary>

```sh
curl -X POST https://beta.playtak.com/api/v1/auth/reset/confirm \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","token":"<token from the email>","password":"new password"}'
```

</details>

#### POST /api/v1/auth/password (Bearer) *(new)*

Changes your password.

```json
{"currentPassword":"old password","newPassword":"new password"}
```

| Status | Body |
| --- | --- |
| `200` | A new token pair. All of the account's other tokens are revoked. |
| `400` | `invalid_password` |
| `401` | `invalid_credentials` |
| `429` | `rate_limited` |

<details>
<summary>Example</summary>

```sh
curl -X POST https://beta.playtak.com/api/v1/auth/password \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Authorization: Bearer <access token>' \
  -H 'Content-Type: application/json' \
  -d '{"currentPassword":"old password","newPassword":"new password"}'
```

</details>

### Bot API keys

Each bot has one key at most. These endpoints are for the bot's owner,
moderators and admins. Anyone else gets `403 forbidden`.

#### GET /api/v1/bots (Bearer) *(new)*

Lists the bots you own.

| Status | Body |
| --- | --- |
| `200` | `{"bots":[{"name":"MyBot","hasApiKey":true}]}`. An empty list if you own none. |
| `401` | `invalid_token` (missing, expired or revoked access token) |

<details>
<summary>Example</summary>

```sh
curl https://beta.playtak.com/api/v1/bots \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Authorization: Bearer <access token>'
```

</details>

#### POST /api/v1/bots/{name}/api-key (Bearer) *(new)*

Creates the bot's key, or replaces it. Replacing closes every connection
that used the old key. No body.

| Status | Body |
| --- | --- |
| `201` | `{"apiKey":"ptk_3f9a…","prefix":"ptk_3f9a","createdAt":"2026-10-15T18:00:00Z"}` |
| `403` | `forbidden` |
| `404` | `not_found` (no such bot) |

The full key appears only in this reply.

<details>
<summary>Example</summary>

```sh
curl -X POST https://beta.playtak.com/api/v1/bots/MyBot/api-key \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Authorization: Bearer <access token>'
```

</details>

#### GET /api/v1/bots/{name}/api-key (Bearer) *(new)*

| Status | Body |
| --- | --- |
| `200` | `{"prefix":"ptk_3f9a","createdAt":"2026-10-15T18:00:00Z","lastUsedAt":"2026-10-15T18:05:00Z"}` |
| `403` | `forbidden` |
| `404` | `not_found` (no such bot, or no key) |

`lastUsedAt` is left out if the key has never been used.

<details>
<summary>Example</summary>

```sh
curl https://beta.playtak.com/api/v1/bots/MyBot/api-key \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Authorization: Bearer <access token>'
```

</details>

#### DELETE /api/v1/bots/{name}/api-key (Bearer) *(new)*

Revokes the key and closes every connection that used it.

| Status | Body |
| --- | --- |
| `204` | none |
| `403` | `forbidden` |
| `404` | `not_found` |

<details>
<summary>Example</summary>

```sh
curl -X DELETE https://beta.playtak.com/api/v1/bots/MyBot/api-key \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Authorization: Bearer <access token>'
```

</details>

### Seeks

These endpoints exist for tournament tools. Players and bots use the socket
commands instead. They use the REST conventions above, so their fields
differ from the socket's seek record: times are in seconds and komi is in
flats.

#### GET /api/v1/seeks

Lists open seeks. No authentication.

```json
[{"id":498,"creator":"bob","opponent":null,"timeContingent":900,"timeIncrement":15,"extraTimeTriggerMove":0,"extraTimeAmount":0,"incrementScales":false,"opening":"swap","komi":2,"boardSize":6,"capstones":1,"pieces":30,"unrated":false,"tournament":false,"color":"ANY","pntId":null}]
```

<details>
<summary>Example</summary>

```sh
curl https://beta.playtak.com/api/v1/seeks \
  -H 'X-Client-Name: MyClient/1.0'
```

</details>

#### PUT /api/v1/seeks

Creates a seek for an online player, for example a tournament pairing.
Needs the service key as `Authorization: Bearer <service key>`. The body is
one seek in the format above, without `id`. `pntId` links it to a
tournament game.

| Status | Body |
| --- | --- |
| `200` | The created seek |
| `400` | `invalid_json` or `invalid_color` |
| `401` | `invalid_service_key` |
| `404` | `creator_not_online` |
| `422` | `incompatible_protocol` (the creator's client cannot play the seek) |

<details>
<summary>Example</summary>

```sh
curl -X PUT https://beta.playtak.com/api/v1/seeks \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Authorization: Bearer <service key>' \
  -H 'Content-Type: application/json' \
  -d '{"creator":"alice","opponent":"bob","timeContingent":900,"timeIncrement":15,"opening":"swap","komi":2,"boardSize":6,"capstones":1,"pieces":30,"unrated":false,"tournament":true,"color":"ANY","pntId":77}'
```

</details>

### Admin

#### PUT /api/v1/admin/bots/{name}/owner (Bearer, mod or admin) *(new)*

Sets a bot's owner.

```json
{"owner":"alice"}
```

| Status | Body |
| --- | --- |
| `204` | none |
| `403` | `forbidden` |
| `404` | `not_found` (no such bot or player) |

<details>
<summary>Example</summary>

```sh
curl -X PUT https://beta.playtak.com/api/v1/admin/bots/MyBot/owner \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Authorization: Bearer <access token>' \
  -H 'Content-Type: application/json' \
  -d '{"owner":"alice"}'
```

</details>

#### POST /api/v1/admin/users/{name}/password (Bearer, admin) *(new)*

Sets a player's password. Replaces the text protocol's `sudo set password`.

```json
{"password":"new password"}
```

| Status | Body |
| --- | --- |
| `204` | none. All of the player's tokens are revoked. |
| `400` | `invalid_password` |
| `403` | `forbidden` |
| `404` | `not_found` |

<details>
<summary>Example</summary>

```sh
curl -X POST https://beta.playtak.com/api/v1/admin/users/alice/password \
  -H 'X-Client-Name: MyClient/1.0' \
  -H 'Authorization: Bearer <access token>' \
  -H 'Content-Type: application/json' \
  -d '{"password":"new password"}'
```

</details>

## 14. Planned

Not part of v1, but the design leaves room for them:

- **Chat history.** Reading back messages after a disconnect, sending
  messages to offline players, and room member lists. Private rooms would
  then be kept, not removed when both players go offline.
- **Group private rooms.** `chat.invite` to add players to a private room.
  A two-player room stays two players: inviting a third makes a new room.
  Text protocol players cannot be invited, since `Tell` only reaches one
  player. Leaving a private room comes with this.
- **Blocking.** Stop a player from opening private rooms with you.
- **Bot management on the website.** A page where bot owners manage their
  bot's settings and key, instead of asking a moderator.
- **Give time settings.** A seek setting to turn giving time off, which
  tournaments need, and custom amounts instead of a fixed 15 seconds.
- **A limit on open seeks per player.** Left out of v1 on purpose: events
  where one strong player takes on many others need many seeks open at once.
- **Async (correspondence) games.** They will not count as games in
  progress for the tournament rule, so a player with async games can still
  start a tournament game. While the tournament game is in progress, their
  async games are frozen: they cannot move in them, and move deadlines are
  paused.
- **Several tournament games at once.** Not allowed in v1.
- **Several connections during a tournament game.** In v1 a new connection replaces the old one. This may be relaxed later.

## 15. Open questions

We want feedback on these before we build.

1. **Anything missing?** Tell us what your client or bot needs that is not
   here.

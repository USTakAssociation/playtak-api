# Tak Server

Tak Server to handle online TAK games

## Requirements
- Java 21
- Maven
- sqlite

## Getting Started

There are 2 options to run the project either manually or with docker (*recommended)

Either option you go you will need to create 2 databases which can be done with the following bash script

```bash
sh ../script/development/create_databases.sh
```
This creates the players and games sqlite dbs

Optionally, you can then use the script `scripts/development/add_user.sh` to add users to the local players database with a password of "password".

```bash
./scripts/development/add_user.sh mylocalacct ./players.db
# See scripts/development/add_user.sh comments for more options.
```

### docker
In the root of the repo run the following command
```bash
docker compose up -d --build
```

### manual
Get the dependencies
```bash
mvn dependency:resolve
```
```bash
mvn compile
```
```bash
mvn package
```

copy the properties, message and db files to the target folder
```bash
cp ../playtakdb/players.db ./target
cp ../playtakdb/games.db ./target
cp properties.xml ./target
cp message ./target
```

Configure `porthttp`, `portws`, `db-path` and `event-subscriber-url` in `./target/properties.xml` to your liking.

Finaly run the app
```
cd ./target
java -jar ./takserver-jar-with-dependencies.jar
```

If you want to run the app from `./` via `java -jar ./target/takserver-jar-with-dependencies.jar` then `./properties.xml` will be updated and you may need to reset `<db-path>./target/</db-path>`.

## Rest API for PNT
A http server is running on `porthttp`, allowing the creation of tournament seeks.

Details need to be filled in here. Until then, please see [TakServer.java](./src/main/java/tak/TakServer.java) for details.

### HTTP API
||URL|Body|Return|Comment|
|-|-|-|-|-|
|PUT|/api/v1/seeks|[SeekDto](./src/main/java/tak/SeekDto.java) without `id`|[SeekDto](./src/main/java/tak/SeekDto.java)|Creates a seek|
|GET|/api/v1/seeks|n/a|[List\[SeekDto\]](./src/main/java/tak/SeekDto.java)|Returns list of existing seeks|

## GameUpdateBroadcaster
The [GameUpdateBroadcaster](./src/main/java/tak/GameUpdateBroadcaster.java) reads an URL from `server-settings.event-subscriber-url` (in `properties.xml`)
and sends `POST` requests there containing
```typescript
{
	type: "game.created"|"game.ended",
	game: GameDto
}
```
(see [GameDto](./src/main/java/tak/DTOs/GameDto.java)). If the URL could not be parsed, a `SEVERE` log message will warn about it but the Tak server will still start.

## Server API

|Environment|Telnet `port`|Websocket `portws`|HTTP `porthttp`|
|-|-|-|-|
|Production | 10000 | 9999 | 9998|
|Beta | 10002 | 10001 | 10003 |
|Local dev | 10000 | 9999 | 9998 |

Stand alone clients can connect directly to playtak.com or by running locally on localhost via a websocket on `portws` for encrypted communication or telnet on `port` which will not be encrypted.
<br>
You can find the playtak UI client github here [playtak-ui](https://github.com/USTakAssociation/playtak-ui)

**You can telnet on port `10000` to test the commands.**

Typical communication is like below
* Connect to server via a websocket. Server gives welcome message
* Server sends "Login or Register"
* Client replies with login information or registers (If Client registers, password is sent to the mail and it can login with the password)
* Server accepts name or asks for another name if the one provided is invalid or already taken
* Server sends list of seeks with "Seek new" messages and games in progress with "GameList Add" messages
* Client posts seek or accepts existing seek
* If seek is accepted, server removes existing seeks for both the players (sends "Seek remove" for all) and starts game
* Client sends moves, server validates moves and sends the move to other client. If invalid, server sends a "NOK" message.
* Game progresses and ends either in a win/lose/draw or abandonment.


### Client to Server Communication
The input/output of server is all text.

The client to server commands and their format is as below
(format of all squares is [Capital letter][digit]. e.g., A2, B5, C4, (row numbers start from 1)

Komi is given in half flats as an integer from 0 to 8, denoting komis from +0.0 to +4.0

#### General messages

| Commands to server | Description|
|--------------|------------------|
| PING | Pings to inform server that the client is alive. Recommended ping spacing is 30 seconds. Server may disconnect clients if pings are not received | quit | Sent by client to indicate it is going to quit. Server removes all seeks, abandons (which loses) game if any |
| quit | Closes the connection

#### Pre Authentication Messages

| Commands to server| Description|
|--------------|------------------|
| Client **client name**| Informs the server of the client being connected from|
| Protocol **version** | Changes protocol version, This should only be sent once upon the initial websocket connection, current supported versions range from 0 to 4. Version 4 adds the **scale_increment** flag (increment scaled by move count) to seeks, game starts, observes and the game list, plus the `GiveTime`/`GivenTime` exchange|
| Register **username email**| Register with the given username and email, a password will be sent to the given email|
| Login **username password**| Login with the username and password|
| Login Guest| Login as a guest|
| Login Guest **token**| Login as a guest and set a token for rejoining, use the same token within 4 hours of last activity to continue using the same guest account. The token must be a cryptographically random composition of 20 lower-case letters|
| SendResetToken **username** **email** | Request a token to the email to change a forgotten password with
| ResetPassword **username** **token** **new_password** | Change password using token


#### Post Authentication Messages

| Commands to server| Description|
|--------------|------------------|
| ChangePassword **old_password** **new_password** | Change password while logged in
| Seek **size** **time** **incr** **W/B** | V1 Seeks a game of board size **size** with time per player **time** specified in seconds, increment per move **incr** specified in seconds and an optional choice of color **W** for white, **B** for black |
| Seek **size** **time** **incr** **W/B/A** **komi** **pieces** **capstones** **unrated** **tournament** **opponent**| V2 Seeks a game, **A** is now used for random color, adding options for **komi** and custom piece counts, **unrated** and **tournament** are flags (either *1* or *0*), **opponent** is the name of the opponent allowed to join, blank to allow anyone to join  |
| Seek **size** **time** **incr** **W/B/A** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** **opponent** | V3 Seeks a game, **extra_time_amount** is extra time to add when move trigger is hit, **extra_time_trigger** is the move number on when to add the extra time to clock|
| Seek **size** **time** **incr** **scale_increment** **W/B/A** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** **opponent** | V4 Seeks a game, **scale_increment** is a flag (either *1* or *0*), if set, the increment is scaled by the move count (1-indexed) |
|Rematch **game_id** **size** **time** **increment** **new_color** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** **opponent** |V1 The client can send the rematch command using the last game id and previous game parameters, which creates a new seek with a rematch ID using the passed in game ID, so if both players send a rematch the server will send an "Accept Rematch **seek_id**" to the player that last sent the rematch command to accept the seek using the **seek_id**
|Rematch **game_id** **size** **time** **increment** **scale_increment** **new_color** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** **opponent** |V2 of Rematch command, adds **scale_increment**
| Accept **id**| Accepts the seek with the number **id**|
| Game#**id** P **Sq** C\|W|Sends a 'Place' move to the specified game id. The optional suffix 'C' or 'W' denote if it is a capstone or a wall (standing stone)|
| Game#**id** M **Sq1** **Sq2** **no1** **no2**... | Sends a 'Move' move to the specified game id. **Sq1** is beginning square, **Sq2** is ending square, **no1**, **no2**, **no3**.. are the number of pieces dropped in the in-between squares (including the last square)|
| Game#**id** OfferDraw | Offers the opponent draw or accepts the opponent's draw offer |
| Game#**id** RemoveDraw | Removes your draw offer |
| Game#**id** Resign | Resign the game |
| Game#**id** Show | Prints a somewhat human readable game position of the game number **id**|
| Game#**id** Show **Sq** | Prints the position in the specified square |
| Game#**id** RequestUndo | Requests the other player to undo the last move or accept the other player's undo request |
| Game#**id** RemoveUndo | Removes your undo request|
| Game#**id** GiveTime | Give opponent a flat amount of time |
| List | Request list of seeks |
| GameList | Request list of games in progress |
| Observe **id** | Observe the specified game. Server sends the game moves and clock info |
| Unobserve **id** | Unobserve the specified game 
| Shout **text** | Send message **text** to all logged in players |
| JoinRoom **room** | Join the room **room** |
| ShoutRoom **room** **text** | Send test to players in room **room** |
| LeaveRoom **room** | Leave the room **room** |
| Tell **player** **text** | Send private message **text** to **player** |

#### Moderator/Admin Commands
| Commands to server| Description|
|--------------|------------------|
| sudo gag **user** | Prevent **user** from using chat
| sudo ungag **user** | Allow **user** to use chat again
| sudo ban **user** | Ban **user**
| sudo unban **user** | Unban **user**
| sudo kick **user** | Disconnect **user**
| sudo list **gag\|ban\|mod\|admin\|online** | List group of users

#### Admin only commands
| Commands to server| Description|
|--------------|------------------|
| sudo set password **user** **new_password**| Set password for user
| sudo reload wordconfig | Reloads word filter
| sudo mod **user** | Make user a mod
| sudo unmod **user** | Remove user as mod
| sudo admin **user** | Make user an admin
| sudo unadmin **user** | Remove user as admin
| sudo bot **user** | Mark user as bot
| sudo unbot **user** | Unmark user as bot
| sudo broadcast **msg** | Broadcast message to every client, msg is sent as is to clients, not to be confused by the "Message" message sent by the server


### Protocol compatibility

Newer protocol versions add fields to existing messages. Most of those fields are safe to
withhold from an older client: the server is authoritative for the clock and pushes absolute
times to both players and every spectator after each ply, so a client that never learns about
the increment, increment scaling, extra time or `GivenTime` re-synchronises on the next move,
and fields like `is_bot`, `unrated` and `tournament` are labels the game never depends on.
Game *results* have never depended on the protocol version at all.

The board is the exception. There is no automatic position resync — a client is sent the move
stream and nothing else, and models the position independently for the whole game. So a
setting that changes how the move stream maps onto the board, without changing the moves
themselves, would leave an uninformed client permanently and silently out of step.

The server therefore treats such a setting as requiring a minimum protocol version, and for a
seek or game that uses one it will:

- omit it from `Seek new` / `Seek remove` broadcasts and from the `List` response;
- omit it from `GameList Add` / `GameList Remove` and from the `GameList` response;
- answer `NOK` to `Accept` for that seek, and to `Observe` for that game;
- answer `NOK` to a `Seek` or `Rematch` command that would create one, and reject the
  equivalent `PUT /api/v1/seeks` request, since a client cannot post a seek it could not
  itself play.

Because a client's version is fixed before it registers as a listener, and a seek's and game's
settings are fixed at creation, a client filtered out of an "add" is filtered out of the
matching "remove" too and never holds a stale row.

**Currently one setting is gated this way: `opening`, which requires protocol 4.** An older
client is served every other seek and game as before, increment scaling included. See
`tak.ProtocolFeature` for the registry and the reasoning behind each classification.

### Server to Client Communication

The server to client messages and their format is as below.

#### Authentication

|Messages from server|Description|Protocol Version
|--------------------|-----------|-----------|
|Welcome! |Just a welcome message when connected to server|>= 0
|Login or Register |Login with username/password or login as guest or register after this message|>= 0
|Registered **name**. check your email for password | Sent on successful registration | >= 0
|Reset token sent | Successfully sent reset token | >= 0 
|Password is changed | Password successfully changed (when changed with token) | >= 0
|Password changed | Password successfully changed (when changed after login) | >= 0
|Welcome **name**! |A welcome message indicating that you've logged in as **name**|>= 0
|Is Mod |Sent on login if user is a moderator or admin|>= 0


#### Game Activity
|Messages from server|Description|Protocol Version
|--------------------|-----------|-----------|
|GameList Add **id** **player_white** **player_black** **size** **original_time** **incr** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** |Notifies client that a game has started (which the client can observe if it wants)|0 - 3
|GameList Remove **id** **player_white** **player_black** **size** **original_time** **incr** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** |Notifies client that the game has ended|0 - 3
|GameList Add **id** **player_white** **player_black** **size** **original_time** **incr** **scale_increment** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** |As the protocol 0-3 `GameList Add` but with the **scale_increment** flag immediately after **incr**|>= 4
|GameList Remove **id** **player_white** **player_black** **size** **original_time** **incr** **scale_increment** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** |As the protocol 0-3 `GameList Remove` but with the **scale_increment** flag immediately after **incr**|>= 4
|Seek new **id** **name** **boardsize** **time** **increment** **W/B/A** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** **opponent** |There is a new seek with seek id **id** posted by **name** with board size **boardsize** with **time** seconds for each player. W, B or A denotes the color of the seeker, **opponent** is the name of the player allowed to join, blank to let anyone join | <= 1
|Seek remove **id** **name** **boardsize** **time** **increment** **W/B/A** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** **opponent** |Existing seek id **id** is removed (either the client has joined another game or has changed his seek or has quit)|<= 1
|Seek new **id** **name** **boardsize** **time** **increment** **W/B/A** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** **opponent** **is_bot**  |There is a new seek with seek id **id** posted by **name** with board size **boardsize** with **time** seconds for each player. W, B or A denotes the color of the seeker, **opponent** is the name of the player allowed to join, 0 to let anyone join, **is_bot** is 1 if created by a bot or 0 if not | 2 - 3
|Seek remove **id** **name** **boardsize** **time** **increment** **W/B/A** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** **opponent** **is_bot** |Existing seek id **id** is removed (either the client has joined another game or has changed his seek or has quit)| 2 - 3
|Seek new **id** **name** **boardsize** **time** **increment** **scale_increment** **W/B/A** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** **opponent** **is_bot**  |There is a new seek with seek id **id** posted by **name** with board size **boardsize** with **time** seconds for each player. W, B or A denotes the color of the seeker, **opponent** is the name of the player allowed to join, 0 to let anyone join, **scale_increment** is 1 if increment scales by move count or 0 if not | >= 4
|Seek remove **id** **name** **boardsize** **time** **increment** **scale_increment** **W/B/A** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** **opponent** **is_bot** |Existing seek id **id** is removed (either the client has joined another game or has changed his seek or has quit)| >= 4
|Game Start **id** **size** **player_white** vs **player_black** **your_color** **time** **komi** **pieces** **capstones** **extra_time_trigger** **extra_time_amount** |Notifies client to start a game. The game id. being **id**, players' names being **white_player**, **black_player** and **your_color** being your color which could be either "white" or "black"| <= 1
|Game Start **id** **player_white** vs **player_black** **your color** **size** **time** **increment** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** **is_bot** |Notifies client to start a game. The game id. being **id**, players' names being **white_player**, **black_player** and **your_color** being your color which could be either "white" or "black", **is_bot** is 1 if at least one of the players is a bot, otherwise 0 | 2 - 3
|Game Start **id** **player_white** vs **player_black** **your color** **size** **time** **increment** **scale_increment** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount** **is_bot** |Notifies client to start a game. The game id. being **id**, players' names being **white_player**, **black_player** and **your_color** being your color which could be either "white" or "black", **scale_increment** is 1 if increment scales with movecount, otherwise 0 | >= 4
|Observe **id** **player_white** **player_black** **size** **original_time** **incr** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount**| Start observing the game **id** of board size **size** with original time setting of **original_time** seconds| 0 - 3
|Observe **id** **player_white** **player_black** **size** **original_time** **incr** **scale_increment** **komi** **pieces** **capstones** **unrated** **tournament** **extra_time_trigger** **extra_time_amount**| As the protocol 0-3 `Observe` but with the **scale_increment** flag immediately after **incr**| >= 4
|Accept Rematch **id** | Server tells the client to attempt to accept seek with id **id**| >= 0
|Rematch seek created with ID: **id** | Server has created rematch seek for you | >= 0


#### In Game
|Messages from server|Description|Protocol Version
|--------------------|-----------|-----------|
|Game#**id** P **Sq** C\|W|The 'Place' move played by the other player in game number **id**. The format is same as the command from client to server|>= 0
|Game#**id** M **Sq1** **Sq2** **no1** **no2**...|The 'Move' move played by the other player in game number **id**. The format is same as the command from client to server|>= 0
|Game#**id** Time **whitetime** **blacktime** |Update the clock with the time specified for white and black players, time given in seconds| = 0
|Game#**id** Timems **whitetime** **blacktime** |Update the clock with the time specified for white and black players, time given in milliseconds| >= 1
|Game#**id** Over **result**|Game number **id** is over. **result** is one of *R-0*, *0-R*, *F-0*, *0-F*, *1/2-1/2*| >= 0
|Game#**id** OfferDraw |Indicates the opponent has offered a draw|>= 0
|Game#**id** RemoveDraw |Indicates your opponent has taken back his offer to draw|>= 0
|Game#**id** RequestUndo |Request from opponent to undo the last move|>= 0
|Game#**id** RemoveUndo |Opponent removes his undo request|>= 0
|Game#**id** GivenTime **color** **amount** | The server has given a player **amount** time, in ms. **color** is either *white* or *black* | >= 0
|Game#**id** Undo |Undo the last move. Client is supposed to keep track of previous board states and undo to the last state.|>= 0
|Game#**id** Abandoned. **player** quit|Game number **id** is abandoned by **player** as they quit. Clients can treat this as resign.|>= 0


#### Chat

|Messages from server|Description|Protocol Version
|--------------------|-----------|-----------|
|Shout \<**player**\> **text** |Chat message **text** from **player** (angle brackets are included)|>= 0
|Joined room **room** |Indicates you've joined the room **room**|>= 0
|ShoutRoom **room** \<**player**\> **text** |Message **text** from **player** to chat room **room** (angle brackets are included)|>= 0
|Tell \<**player**\> **text** |Private chat message **text** from **player** (angle brackets are included)|>= 0
|Told \<**player**\> **text** |Confirmation that your message is sent to **player**. You'll receive this even if **player** is not logged in (angle brackets are included) |>= 0


#### Status Messages

|Messages from server|Description|Protocol Version
|--------------------|-----------|-----------|
|Online **count** |**count** players are connected to server|>= 0
|OnlinePlayers \[**"username"**,...\] | A comma seperated list of usernames connected to the server, does not include bots (quotation marks _are_ included)|>= 0
|Message **text** | A message from server. Might be used to indicate announcements like name accepted/server going down, etc|>= 0
|sudoReply **text**| Response to some mod commands | >= 0
|OK | Indicates previous command is ok. Clients can ignore this. *I might remove this message altogether in future as it serves no real purpose*|>= 0
|NOK |Indicates the command client send is invalid or unrecognized|>= 0
|Error: **text** |Generic error message|>= 0
|Registration Error: **reason** |Registration failed |>= 0
|Reset Token Error: **reason** | SendResetToken command failed  |>= 0
|Wrong token | Could not reset password using token, as it was incorrect | >= 0
|Wrong password | Password was incorrect (sent specifically when trying to change password) | >= 0
|No such player| sent if command features player that does not exist | >= 0
|Authentication Failure | Authentication failed |>= 0

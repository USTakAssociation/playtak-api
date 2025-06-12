package tak;

import java.io.BufferedReader;
import java.io.File;
import java.io.FileReader;
import java.io.IOException;
import java.util.HashSet;
import java.util.Set;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.Flow.Publisher;
import java.util.concurrent.Flow.Subscriber;
import java.util.logging.Level;
import java.util.logging.Logger;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.Timer;
import java.util.TimerTask;
import tak.FlowMessages.GameUpdate;
import tak.utils.BadWordFilter;
import tak.utils.ConcurrentHashSet;
import java.util.ArrayList;

/*
 * To change this license header, choose License Headers in Project Properties.
 * To change this template file, choose Tools | Templates
 * and open the template in the editor.
 */
/**
 *
 * @author chaitu
 */
public class Client extends Thread implements Publisher<GameUpdate> {

	Websocket websocket;
	Player player = null;
	int clientNo;
	public int protocolVersion=0;
	private static final long STALE_CONNECTION_TIMEOUT = 60000; // 60 seconds
	private static final long CLEANUP_INTERVAL = 30000; // 30 seconds
	private static Timer cleanupTimer;
	private long lastActivity;

	static AtomicInteger totalClients = new AtomicInteger(0);

	static ConcurrentHashSet<Client> clientConnections = new ConcurrentHashSet<>();
	protected ConcurrentHashSet<Subscriber<? super GameUpdate>> subscribers = new ConcurrentHashSet<>();

	Seek seek = null;
	Set<Game> spectating;
	Set<String> chatRooms;

	String loginString = "^Login ([a-zA-Z][a-zA-Z0-9_]{3,15}) ([^\n\r\\s]{6,50})";
	Pattern loginPattern;

	String loginGuestString = "^Login Guest ?(([a-z]{20})?)";
	Pattern loginGuestPattern;

	String registerString = "^Register ([a-zA-Z][a-zA-Z0-9_]{3,15}) ([A-Za-z.0-9_+!#$%&'*^?=-]{1,30}@[A-Za-z.0-9-]{3,30})";
	Pattern registerPattern;

	String wrongRegisterString = "^Register [^\n\r]{1,256}";
	Pattern wrongRegisterPattern;

	String clientString = "^Client ([A-Za-z-.0-9]{1,60})";
	Pattern clientPattern;

	String protocolString = "^Protocol ([1-9][0-9]{0,8})";
	Pattern protocolPattern;

	String changePasswordString = "^ChangePassword ([^\n\r\\s]{6,50}) ([^\n\r\\s]{6,50})";
	Pattern changePasswordPattern;

	String sendResetTokenString = "^SendResetToken ([a-zA-Z][a-zA-Z0-9_]{3,15}) ([A-Za-z.0-9_+!#$%&'*^?=-]{1,30}@[A-Za-z.0-9-]{3,30})";
	Pattern sendResetTokenPattern;

	String resetPasswordString = "^ResetPassword ([a-zA-Z][a-zA-Z0-9_]{3,15}) ([^\n\r\\s]{6,50}) ([^\n\r\\s]{6,50})";
	Pattern resetPasswordPattern;

	String placeString = "^Game#(\\d+) P ([A-Z])(\\d)( C)?( W)?";
	Pattern placePattern;

	String moveString = "^Game#(\\d+) M ([A-Z])(\\d) ([A-Z])(\\d)(( \\d)+)";
	Pattern movePattern;

	String undoString = "^Game#(\\d+) RequestUndo";
	Pattern undoPattern;

	String removeUndoString = "^Game#(\\d+) RemoveUndo";
	Pattern removeUndoPattern;

	String drawString = "^Game#(\\d+) OfferDraw";
	Pattern drawPattern;

	String removeDrawString = "^Game#(\\d+) RemoveDraw";
	Pattern removeDrawPattern;

	String resignString = "^Game#(\\d+) Resign";
	Pattern resignPattern;

	String seekV3String = "^Seek (\\d) (\\d+) (\\d+) ([WBA]) (\\d+) (\\d+) (\\d+) (0|1) (0|1) (\\d+) (\\d+) ([A-Za-z0-9_]*)";
	Pattern seekV3Pattern;

	String seekV2String = "^Seek (\\d) (\\d+) (\\d+) ([WBA]) (\\d+) (\\d+) (\\d+) (0|1) (0|1) ([A-Za-z0-9_]*)";
	Pattern seekV2Pattern;

	String seekV1String = "^Seek (\\d) (\\d+) (\\d+)( [WB])?";
	Pattern seekV1Pattern;

	String rematchString = "^Rematch (\\d+) (\\d+) (\\d+) (\\d+) ([WBA]) (\\d+) (\\d+) (\\d+) (0|1) (0|1) (\\d+) (\\d+) ([A-Za-z0-9_]*)";
	Pattern rematchPattern;

	String acceptSeekString = "^Accept (\\d+)";
	Pattern acceptSeekPattern;

	String listString = "^List";
	Pattern listPattern;

	String gameListString = "^GameList";
	Pattern gameListPattern;

	String observeString = "^Observe (\\d+)";
	Pattern observePattern;

	String unobserveString = "^Unobserve (\\d+)";
	Pattern unobservePattern;

	String gameString = "^Game#(\\d+) Show$";
	Pattern gamePattern;

	String getSqStateString = "^Game#(\\d+) Show ([A-Z])(\\d)";
	Pattern getSqStatePattern;

	String shoutString = "^Shout ([^\n\r]{1,256})";
	Pattern shoutPattern;

	String shoutRoomString = "^ShoutRoom ([^\n\r\\s]{1,64}) ([^\n\r]{1,256})";
	Pattern shoutRoomPattern;

	String joinRoomString = "^JoinRoom ([^\n\r\\s]{1,64})";
	Pattern joinRoomPattern;

	String leaveRoomString = "^LeaveRoom ([^\n\r\\s]{1,64})";
	Pattern leaveRoomPattern;

	String tellString = "^Tell ([a-zA-Z][a-zA-Z0-9_]{3,15}) ([^\n\r]{1,256})";
	Pattern tellPattern;

	String pingString = "^PING$";
	Pattern pingPattern;

	String sudoString = "sudo ([^\n\r]{1,256})";
	Pattern sudoPattern;

	/* Mod commands start with sudoString */
	String gagString = "sudo gag ([a-zA-Z][a-zA-Z0-9_]{3,15})";
	Pattern gagPattern;

	String unGagString = "sudo ungag ([a-zA-Z][a-zA-Z0-9_]{3,15})";
	Pattern unGagPattern;

	String banString = "sudo ban ([a-zA-Z][a-zA-Z0-9_]{3,15}) ([^\n\n]{1,256})";
	Pattern banPattern;

	String unBanString = "sudo unban ([a-zA-Z][a-zA-Z0-9_]{3,15})";
	Pattern unBanPattern;

	String kickString = "sudo kick ([a-zA-Z][a-zA-Z0-9_]{3,15})";
	Pattern kickPattern;

	String listCmdString = "sudo list ([a-zA-Z]{3,15})";
	Pattern listCmdPattern;

	String reloadWordCmdString = "sudo reload wordconfig";
	Pattern reloadWordCmdPattern;

	//set param user value
	String setString = "sudo set ([a-zA-Z]{3,15}) ([a-zA-Z][a-zA-Z0-9_]{3,15}) ([^\n\r\\s]{6,100})";
	Pattern setPattern;

	// mod strings
	String modString = "sudo mod ([a-zA-Z][a-zA-Z0-9_]{3,15})";
	Pattern modPattern;
	String unModString = "sudo unmod ([a-zA-Z][a-zA-Z0-9_]{3,15})";
	Pattern unModPattern;
	
	// admin strings
	String adminString = "sudo admin ([a-zA-Z][a-zA-Z0-9_]{3,15})";
	Pattern adminPattern;
	String unAdminString = "sudo unadmin ([a-zA-Z][a-zA-Z0-9_]{3,15})";
	Pattern unAdminPattern;
	
	// bot strings
	String botString = "sudo bot ([a-zA-Z][a-zA-Z0-9_]{3,15})";
	Pattern botPattern;
	String unBotString = "sudo unbot ([a-zA-Z][a-zA-Z0-9_]{3,15})";
	Pattern unBotPattern;

	String broadcastString = "sudo broadcast ([^\n\r]{1,256})";
	Pattern broadcastPattern;

	Client(Websocket socket) {
		websocket = socket;
		this.clientNo = totalClients.incrementAndGet();
		this.lastActivity = System.currentTimeMillis(); 

		loginPattern = Pattern.compile(loginString);
		registerPattern = Pattern.compile(registerString);
		clientPattern = Pattern.compile(clientString);
		protocolPattern = Pattern.compile(protocolString);
		changePasswordPattern = Pattern.compile(changePasswordString);
		sendResetTokenPattern = Pattern.compile(sendResetTokenString);
		resetPasswordPattern = Pattern.compile(resetPasswordString);
		placePattern = Pattern.compile(placeString);
		movePattern = Pattern.compile(moveString);
		undoPattern = Pattern.compile(undoString);
		removeUndoPattern = Pattern.compile(removeUndoString);
		drawPattern = Pattern.compile(drawString);
		removeDrawPattern = Pattern.compile(removeDrawString);
		resignPattern = Pattern.compile(resignString);
		wrongRegisterPattern = Pattern.compile(wrongRegisterString);
		seekV3Pattern = Pattern.compile(seekV3String);
		seekV2Pattern = Pattern.compile(seekV2String);
		seekV1Pattern = Pattern.compile(seekV1String);
		rematchPattern = Pattern.compile(rematchString);
		acceptSeekPattern = Pattern.compile(acceptSeekString);
		listPattern = Pattern.compile(listString);
		gameListPattern = Pattern.compile(gameListString);
		gamePattern = Pattern.compile(gameString);
		observePattern = Pattern.compile(observeString);
		unobservePattern = Pattern.compile(unobserveString);
		getSqStatePattern = Pattern.compile(getSqStateString);
		shoutPattern = Pattern.compile(shoutString);
		shoutRoomPattern = Pattern.compile(shoutRoomString);
		joinRoomPattern = Pattern.compile(joinRoomString);
		leaveRoomPattern = Pattern.compile(leaveRoomString);
		tellPattern = Pattern.compile(tellString);
		pingPattern = Pattern.compile(pingString);
		loginGuestPattern = Pattern.compile(loginGuestString);

		sudoPattern = Pattern.compile(sudoString);
		gagPattern = Pattern.compile(gagString);
		unGagPattern = Pattern.compile(unGagString);
		banPattern = Pattern.compile(banString);
		unBanPattern = Pattern.compile(unBanString);
		kickPattern = Pattern.compile(kickString);
		listCmdPattern = Pattern.compile(listCmdString);
		reloadWordCmdPattern = Pattern.compile(reloadWordCmdString);
		setPattern = Pattern.compile(setString);
		modPattern = Pattern.compile(modString);
		unModPattern = Pattern.compile(unModString);
		adminPattern = Pattern.compile(adminString);
		botPattern = Pattern.compile(botString);
		unBotPattern = Pattern.compile(unBotString);
		unAdminPattern = Pattern.compile(unAdminString);
		broadcastPattern = Pattern.compile(broadcastString);

		clientConnections.add(this);
		spectating = new HashSet<>();
		chatRooms = new HashSet<>();
	}

	void sendNOK() {
		send("NOK");
	}

	void send(String st) {
		Log("Send:"+st);
		sendWithoutLogging(st);
	}

	void sendCmdReply(String st) {
		sendWithoutLogging("CmdReply "+st);
	}

	void sendSudoReply(String st) {
		sendWithoutLogging("sudoReply "+st);
	}

	void sendWithoutLogging(String st) {
		websocket.send(st);
	}

	void removeSeeks() {
		Seek.seekStuffLock.lock();
		try{
			if (seek != null) {
				Log("Removing seek " + seek.no + " from player " + this.player.getName());
				Seek.removeSeek(seek.no);
				seek = null;
			}
		}
		finally{
			Seek.seekStuffLock.unlock();
		}
	}

	void unspectateAll() {
		for(Game g: spectating)
			g.unSpectate(player);
		spectating.clear();
	}

	static void sendAll(final String msg) {
		for(Client c: clientConnections){
			c.sendWithoutLogging(msg);
		}
	}

	static void sendAllOnline(final String msg) {
		for(Client c: clientConnections){
			if(c.player!=null){
				c.sendWithoutLogging(msg);
			}
		}
	}

	// method to update lastActivity timestamp
	public void updateLastActivity() {
			this.lastActivity = System.currentTimeMillis();
	}

	void clientQuit() throws IOException {
		clientConnections.remove(this);

		if (player != null) {
			Game game = player.getGame();
			if(game!=null){
				game.playerDisconnected(player);
			}

			Seek.unregisterListener(this);
			Game.unregisterGameListListener(player);
			removeSeeks();
			removeFromAllRooms();
			unspectateAll();

			player.loggedOut();
			sendAllOnline("Online "+clientConnections.size());
			onlinePlayerMessageHandler();
		}

		websocket.kill(201);
		Log("disconnected");
	}

	void Log(Object obj) {
		TakServer.Log(clientNo+":"+((player!=null)?player.getName():"")+":"+obj);
	}

	void disconnect() {
		websocket.kill(202);
	}
	
	// Add static initializer block to start the timer when class loads
	static {
			cleanupTimer = new Timer("ConnectionCleanupTimer", true); // true makes it a daemon thread
			cleanupTimer.scheduleAtFixedRate(new TimerTask() {
					@Override
					public void run() {
							try {
									cleanupStaleConnections();
							} catch (Exception e) {
									TakServer.Log("Error in cleanup timer: " + e.getMessage());
							}
					}
			}, CLEANUP_INTERVAL, CLEANUP_INTERVAL);
	}
	
	public static void cleanupStaleConnections() {
		int before = clientConnections.size();
		long now = System.currentTimeMillis();

		clientConnections.forEach(client -> {
				if (client.websocket.streamended || 
						(now - client.lastActivity > STALE_CONNECTION_TIMEOUT)) {
						try {
								TakServer.Log("Cleaning up stale connection for client: " + client.clientNo);
								client.clientQuit();
						} catch (IOException e) {
								TakServer.Log("Error cleaning up stale connection: " + e.getMessage());
						}
				}
		});

		int after = clientConnections.size();
		if (before != after) {
				TakServer.Log("Cleanup removed " + (before - after) + " stale connections");
		}
	}

	@Override
	public void run() {
		String temp=null;
		try {
			while(!websocket.headerended && !websocket.streamended){
				temp=websocket.recieve(true);
			}
			websocket.send("Welcome!");
			websocket.send("Login or Register");
			Log("Welcome sent");
			mainloop:
			while (!websocket.streamended) {
				while(temp==null){
					temp=websocket.recieve(true);
					if(websocket.streamended){
						break mainloop;
					}
				}
				temp = temp.replaceAll("[\\n\\r]+$","");

				if(temp.equals("quit")){
					break;
				}

				if((pingPattern.matcher(temp)).find()) {
					if(player!=null){
						player.lastActivity=System.nanoTime();
					}
					updateLastActivity();
					sendWithoutLogging("OK");
					temp=null;
					continue;
				}

				Matcher m;

				if (player == null) {
					//Client name set
					if((m = clientPattern.matcher(temp)).find()){
						String clientversion=m.group(1);
						Log("Client !"+clientversion+"!");
						if(clientversion.equals("TreffnonX-08.09.16") || clientversion.equals("TakWeb-16.05.26")){
							sendWithoutLogging("Shout <Server> Your Playtak client is unfortunately no longer compatible. Please go to https://www.playtak.com in order to play.");
						}
						else{
							sendWithoutLogging("OK");
						}
					}
					// What protocol version is this client using
					else if((m = protocolPattern.matcher(temp)).find()){
						this.protocolVersion=Integer.parseInt(m.group(1));
						sendWithoutLogging("OK");
					}
					//Login Guest
					else if ((m=loginGuestPattern.matcher(temp)).find()) {
						Player.loginLock.lock();
						//Log("Guest login");
						try{
							Player.cleanUpGuests();
							String token=m.group(1);
							if(token!=""){
								player=Player.guestsByToken.get(token);
								if(player==null){
									player = new Player(token);
								}
								else if(player.isLoggedIn()){
									Client oldClient = player.getClient();
									player.send("Message You've logged in from another window. Disconnecting");
									player.logout();
									try {
										oldClient.join(1000);
									} catch (InterruptedException ex) {
										Logger.getLogger(Client.class.getName()).log(Level.SEVERE, null, ex);
									}
								}
							}
							else{
								player = new Player();
							}
							player.login(this);
							player.lastActivity=System.nanoTime();

							send("Welcome "+player.getName()+"!");
							Log("Player logged in");

							Seek.registerListener(this);
							Game.registerGameListListener(player);

							sendAllOnline("Online "+clientConnections.size());
							onlinePlayerMessageHandler();
						}
						finally{
							Player.loginLock.unlock();
						}
					}
					//Login
					else if ((m = loginPattern.matcher(temp)).find()) {
						Player.loginLock.lock();
						try{
							String tname = m.group(1).trim();
							synchronized(Player.players) {
								if (Player.players.containsKey(tname)) {
									Player tplayer = Player.players.get(tname);
									String pass = m.group(2).trim();

									if(!tplayer.authenticate(pass)) {
										send("Authentication failure");
									} else {
										if(tplayer.isLoggedIn()) {
											Client oldClient = tplayer.getClient();
											tplayer.send("Message You've logged in from another window. Disconnecting");
											tplayer.logout();
											//Wait for other connection to close before logging in
											try {
												oldClient.join(1000);
											} catch (InterruptedException ex) {
												Logger.getLogger(Client.class.getName()).log(Level.SEVERE, null, ex);
											}
										}

										player = tplayer;

										send("Welcome "+player.getName()+"!");
										if(player.isAdmin() || player.isMod()){
											send("Is Mod");
										}
										player.login(this);

										Seek.registerListener(this);
										Game.registerGameListListener(player);

										sendAllOnline("Online "+clientConnections.size());
										onlinePlayerMessageHandler();
									}
								} else
									send("Authentication failure");
							}
						}
						finally{
							Player.loginLock.unlock();
						}
					}
					//Registration
					else if ((m = registerPattern.matcher(temp)).find()) {
						String tname = m.group(1).trim();
						// prevent swear words in username

						if(tname.toLowerCase().contains("guest")) {
							send("Registration Error: Can't register with guest in the name");
						}
						else if(BadWordFilter.containsBadWord(tname)){
							send("Registration Error: Username cannot contain profanity");
						}
						else {
							synchronized(Player.players) {
								if (Player.isNameTaken(tname)) {
									send("Registration Error: Username is already taken");
								}
								else {
									String email = m.group(2).trim();
									Player tplayer = Player.createPlayer(tname, email);
									send("Registered "+tplayer.getName()+". Check your email for password");
								}
							}
						}
					}
					//Wrong registration chars
					else if ((wrongRegisterPattern.matcher(temp)).find()) {
						send("Registration Error: Unknown format for username/email. Only [a-z][A-Z][0-9][_] allowed for username, it should be 4-16 characters and should start with letter");
					}
					//SendResetToken
					else if ((m = sendResetTokenPattern.matcher(temp)).find()) {
						String tname = m.group(1).trim();
						String email = m.group(2).trim();
						if(Player.players.containsKey(tname)) {
							Player tplayer = Player.players.get(tname);
							if(email.equals(tplayer.getEmail())) {
								tplayer.sendResetToken();
								send("Reset token sent");
							} else {
								send("Reset Token Error: Email address does not match");
							}
						} else {
							send("Reset Token Error: No such player");
						}
					}
					//ResetPassword
					else if ((m = resetPasswordPattern.matcher(temp)).find()) {
						String tname = m.group(1);
						String token = m.group(2);
						String pass = m.group(3);
						if(Player.players.containsKey(tname)) {
							Player tplayer = Player.players.get(tname);
							if(tplayer.resetPassword(token, pass)) {
								send("Password is changed");
							} else {
								send("Wrong token");
							}
						} else {
							send("No such player");
						}
					}
					else
						sendNOK();
				} else {
					Log("Read:"+temp);

					Game game = player.getGame();
					//List all seeks
					if ((m = listPattern.matcher(temp)).find()) {
						Seek.sendListTo(this);
					}
					//Seek a game V3
					else if (game==null && (m = seekV3Pattern.matcher(temp)).find()) {
						Seek.seekStuffLock.lock();
						try{
							if (seek != null) {
								Seek.removeSeek(seek.no);
							}
							int no = Integer.parseInt(m.group(1));
							if(no == 0) {
								Log("Seek remove");
								seek = null;
							} else {
								Seek.COLOR clr = Seek.COLOR.ANY;

								if("W".equals(m.group(4)))
									clr = Seek.COLOR.WHITE;
								else if("B".equals(m.group(4)))
									clr = Seek.COLOR.BLACK;
								seek = Seek.newSeek(
										this,
										Integer.parseInt(m.group(1)),
										Integer.parseInt(m.group(2)),
										Integer.parseInt(m.group(3)),
										clr,
										Integer.parseInt(m.group(5)),
										Integer.parseInt(m.group(6)),
										Integer.parseInt(m.group(7)),
										Integer.parseInt(m.group(8)),
										Integer.parseInt(m.group(9)),
										Integer.parseInt(m.group(10)),
										Integer.parseInt(m.group(11)),
										m.group(12),
										null
								);
								Log("Seek "+seek.boardSize);
							}
						}
						finally{
							Seek.seekStuffLock.unlock();
						}
					}
					//Seek a game V2
					else if (game==null && (m = seekV2Pattern.matcher(temp)).find()) {
						Seek.seekStuffLock.lock();
						try{
							if (seek != null) {
								Seek.removeSeek(seek.no);
							}
							int no = Integer.parseInt(m.group(1));
							if(no == 0) {
								Log("Seek remove");
								seek = null;
							} else {
								Seek.COLOR clr = Seek.COLOR.ANY;

								if("W".equals(m.group(4)))
									clr = Seek.COLOR.WHITE;
								else if("B".equals(m.group(4)))
									clr = Seek.COLOR.BLACK;
								seek = Seek.newSeek(
									this,
									Integer.parseInt(m.group(1)),
									Integer.parseInt(m.group(2)),
									Integer.parseInt(m.group(3)),
									clr,
									Integer.parseInt(m.group(5)),
									Integer.parseInt(m.group(6)),
									Integer.parseInt(m.group(7)),
									Integer.parseInt(m.group(8)),
									Integer.parseInt(m.group(9)),
									0,
									0,
									m.group(10),
									null
								);
								Log("Seek "+seek.boardSize);
							}
						}
						finally{
							Seek.seekStuffLock.unlock();
						}
					}
					//Seek a game V1
					else if (game==null && (m = seekV1Pattern.matcher(temp)).find()) {
						Seek.seekStuffLock.lock();
						try{
							if (seek != null) {
								Seek.removeSeek(seek.no);
							}
							int no = Integer.parseInt(m.group(1));
							if(no == 0) {
								Log("Seek remove");
								seek = null;
							} else {
								Seek.COLOR clr = Seek.COLOR.ANY;

								if(" W".equals(m.group(4)))
									clr = Seek.COLOR.WHITE;
								else if(" B".equals(m.group(4)))
									clr = Seek.COLOR.BLACK;

								int capstonesCount=0;
								int tilesCount=0;
								switch(Integer.parseInt(m.group(1))) {
									case 3: capstonesCount = 0; tilesCount = 10; break;
									case 4: capstonesCount = 0; tilesCount = 15; break;
									case 5: capstonesCount = 1; tilesCount = 21; break;
									case 6: capstonesCount = 1; tilesCount = 30; break;
									case 7: capstonesCount = 2; tilesCount = 40; break;
									case 8: capstonesCount = 2; tilesCount = 50; break;
								}

								seek = Seek.newSeek(
									this,
									Integer.parseInt(m.group(1)),
									Integer.parseInt(m.group(2)),
									Integer.parseInt(m.group(3)),
									clr,
									0,
									tilesCount,
									capstonesCount,
									0,
									0,
									0,
									0,
									"",
									null
								);
								Log("Seek "+seek.boardSize);
							}
						}
						finally{
							Seek.seekStuffLock.unlock();
						}
					}
					//Accept a seek
					else if (game==null && (m = acceptSeekPattern.matcher(temp)).find()) {
						Seek.seekStuffLock.lock();
						try{
							Seek sk = Seek.seeks.get(Integer.parseInt(m.group(1)));
							if (sk != null && game == null && sk.client.player.getGame() == null && sk!=seek && (sk.opponent.toLowerCase().equals(player.getName().toLowerCase()) || sk.opponent.equals(""))) {
								Client otherClient = sk.client;
								int sz = sk.boardSize;
								int time = sk.time;

								removeSeeks();
								otherClient.removeSeeks();
								unspectateAll();
								otherClient.unspectateAll();

								game = new Game(player, otherClient.player, sz, time, sk.incr, sk.color, sk.komi, sk.pieces, sk.capstones, sk.unrated, sk.tournament, sk.triggerMove, sk.timeAmount, sk.pntId);
								notifySubscribers(GameUpdate.gameCreated(game.toDto()));
								for(var subscriber: subscribers) {
									game.subscribe(subscriber);
								}

								game.gameLock.lock();
								try{
									Game.addGame(game);

									player.setGame(game);
									otherClient.player.setGame(game);
									send(createGameStartString(this.protocolVersion, game, sk, player));

									otherClient.send(createGameStartString(otherClient.protocolVersion, game, sk, otherClient.player));
								}
								finally{
									game.gameLock.unlock();
								}
							} else {
								sendNOK();
							}
						}
						finally{
							Seek.seekStuffLock.unlock();
						}
					}
					// handle rematch request
					else if (game == null && (m = rematchPattern.matcher(temp)).find()) {
						// create a new private seek for rematch if both players send a rematch request then have the second player accept it
						Seek.seekStuffLock.lock();
						try{
							if (seek != null) {
								Seek.removeSeek(seek.no);
							}
							Seek sk = null;
							// loop through all the seek hash map and check is a seek data contains the rematch id
							for (Seek s : Seek.seeks.values()) {
								if (s.rematchId == Integer.parseInt(m.group(1))) {
									sk = s;
									break;
								}
							}
							// if the client player is the opponent of the rematch seek, accept it
							if (sk != null && sk.opponent.toLowerCase().equals(player.getName().toLowerCase())) {
								send("Accept Rematch " + sk.no);
							} else  {
								seek = Seek.newRematchSeek(
										this,
										Integer.parseInt(m.group(1)), // ID
										Integer.parseInt(m.group(2)), // size
										Integer.parseInt(m.group(3)), // time
										Integer.parseInt(m.group(4)), // increment
										m.group(5), // color
										Integer.parseInt(m.group(6)), // komi
										Integer.parseInt(m.group(7)), // pieces
										Integer.parseInt(m.group(8)), // capstones
										Integer.parseInt(m.group(9)), // unrated
										Integer.parseInt(m.group(10)), // tournament
										Integer.parseInt(m.group(11)), // triggerMove
										Integer.parseInt(m.group(12)), // timeAmount
										m.group(13)); // opponent
								send("Rematch seek created with ID: " + seek.no);
							}
						}
						finally{
							Seek.seekStuffLock.unlock();
						}
					}
					//Handle place move
					else if (game != null && (m = placePattern.matcher(temp)).find() && game.no == Integer.parseInt(m.group(1))) {
						game.gameLock.lock();
						try{
							Status st = game.placeMove(player, m.group(2).charAt(0), Integer.parseInt(m.group(3)), m.group(4) != null, m.group(5)!=null);
							if(st.isOk()){

								if(game.gameState!=Game.gameS.NONE){
									Player otherPlayer = (game.white==player)?game.black:game.white;
									Game.removeGame(game);
									player.removeGame();
									otherPlayer.removeGame();
								}
								sendWithoutLogging("OK");
							} else {
								sendNOK();
								send("Error:"+st.msg());
							}
						}
						finally{
							game.gameLock.unlock();
						}
					}
					//Handle move move
					else if (game!=null && (m = movePattern.matcher(temp)).find() && game.no == Integer.parseInt(m.group(1))) {
						String args[] = m.group(6).split(" ");
						int argsint[] = new int[args.length-1];
						for(int i=1;i<args.length;i++)
							argsint[i-1] = Integer.parseInt(args[i]);
						game.gameLock.lock();
						try{
							Status st = game.moveMove(player, m.group(2).charAt(0), Integer.parseInt(m.group(3)), m.group(4).charAt(0), Integer.parseInt(m.group(5)), argsint);
							if(st.isOk()){

								if(game.gameState!=Game.gameS.NONE){
									Player otherPlayer = (game.white==player)?game.black:game.white;
									Game.removeGame(game);
									player.removeGame();
									otherPlayer.removeGame();
								}
								sendWithoutLogging("OK");
							} else {
								sendNOK();
								send("Error:"+st.msg());
							}
						}
						finally{
							game.gameLock.unlock();
						}
					}
					//Handle undo offer
					else if (game!=null && (m = undoPattern.matcher(temp)).find() && game.no == Integer.parseInt(m.group(1))) {
						game.undo(player);
						sendWithoutLogging("OK");
					}
					//Handle removing undo offer
					else if (game!=null && (m = removeUndoPattern.matcher(temp)).find() && game.no == Integer.parseInt(m.group(1))) {
						game.removeUndo(player);
						sendWithoutLogging("OK");
					}
					//Handle draw offer
					else if (game!=null && (m = drawPattern.matcher(temp)).find() && game.no == Integer.parseInt(m.group(1))) {
						game.gameLock.lock();
						try{
							game.draw(player);
							Player otherPlayer = (game.white==player)?game.black:game.white;

							if(game.gameState!=Game.gameS.NONE){
								Game.removeGame(game);
								player.removeGame();
								otherPlayer.removeGame();
							}
							sendWithoutLogging("OK");
						}
						finally{
							game.gameLock.unlock();
						}
					}
					//Handle removing draw offer
					else if (game!=null && (m = removeDrawPattern.matcher(temp)).find() && game.no == Integer.parseInt(m.group(1))) {
						game.removeDraw(player);
						sendWithoutLogging("OK");
					}
					//Handle resignation
					else if (game!=null && (m = resignPattern.matcher(temp)).find() && game.no == Integer.parseInt(m.group(1))) {
						game.gameLock.lock();
						try{
							game.resign(player);
							Player otherPlayer = (game.white==player)?game.black:game.white;

							Game.removeGame(game);
							player.removeGame();
							otherPlayer.removeGame();
						}
						finally{
							game.gameLock.unlock();
						}
					}
					//Show game state
					else if (game != null && (m=gamePattern.matcher(temp)).find() && game.no == Integer.parseInt(m.group(1))) {
						send(game.toString());
					}
					//Show sq state for a game
					else if (game != null && (m=getSqStatePattern.matcher(temp)).find() && game.no == Integer.parseInt(m.group(1))) {
						send("Game#"+game.no+" Show Sq "+game.sqState(m.group(2).charAt(0), Integer.parseInt(m.group(3))));
					}
					//GameList
					else if ((m=gameListPattern.matcher(temp)).find()){
						Game.sendGameListTo(player);
					}
					//ObserveGame
					else if ((m=observePattern.matcher(temp)).find()){
						game = Game.games.get(Integer.parseInt(m.group(1)));
						if(game!=null){
							game.gameLock.lock();
							try{
								spectating.add(game);
								game.newSpectator(player);
							}
							finally{
								game.gameLock.unlock();
							}
						} else
							sendNOK();
					}
					//UnobserveGame
					else if ((m=unobservePattern.matcher(temp)).find()){
						game = Game.games.get(Integer.parseInt(m.group(1)));
						if(game!=null){
							game.gameLock.lock();
							try{
								spectating.remove(game);
								game.unSpectate(player);
								sendWithoutLogging("OK");
							}
							finally{
								game.gameLock.unlock();
							}
						} else
							sendNOK();
					}
					//Shout
					else if ((m=shoutPattern.matcher(temp)).find()){
						String msg = "<"+player.getName()+"> "+BadWordFilter.filterText(m.group(1));

						if(!player.isGagged()) {
							sendAllOnline("Shout "+msg);
							IRCBridge.send(msg);
						} else//send to only gagged player
							sendWithoutLogging("Shout <"+player.getName()+"> <Server: You have been muted for inappropriate chat behavior.>");
					}
					//JoinRoom
					else if((m=joinRoomPattern.matcher(temp)).find()) {
						addToRoom(m.group(1));
					}
					//LeaveRoom
					else if((m=leaveRoomPattern.matcher(temp)).find()) {
						removeFromRoom(m.group(1));
						sendWithoutLogging("OK");
					}
					//ShoutRoom
					else if ((m=shoutRoomPattern.matcher(temp)).find()) {
						ChatRoom.shout(m.group(1), this, m.group(2));
					}
					//Tell
					else if ((m=tellPattern.matcher(temp)).find()) {
						if(Player.players.containsKey(m.group(1))) {
							Player tplayer = Player.players.get(m.group(1));
							if(!player.isGagged()){
								tplayer.send("Tell "+"<"+player.getName()+"> "+ BadWordFilter.filterText(m.group(2)));
								send("Told "+"<"+tplayer.getName()+"> " + BadWordFilter.filterText(m.group(2)));
							} else {
								send("Told "+"<"+tplayer.getName()+"> <Server: You have been muted for inappropriate chat behavior.>");
							}
						} else {
							send("No such player");
						}
					}
					//ChangePassword old new
					else if ((m=changePasswordPattern.matcher(temp)).find()) {
						String curPass = m.group(1);
						String newPass = m.group(2);

						if(player.authenticate(curPass)) {
							player.setPassword(newPass);
							send("Password changed");
						} else {
							send("Wrong password");
						}
					}
					//sudo
					else if ((m=sudoPattern.matcher(temp)).find()){
						sudoHandler(temp);
					}
					//Undefined
					else {
						sendNOK();
					}
				}
				temp=websocket.recieve(true);
			}
		} finally {
			try {
				clientQuit();
			} catch (IOException ex) {
				Logger.getLogger(Client.class.getName()).log(Level.SEVERE, null, ex);
			}
		}
	}

	private static String createGameStartString(int protocolVersion, Game game, Seek sk, Player player) {
		String msg = "";
		String msg2 = "";
		if (protocolVersion < 2) {
			msg += "Game Start " + game.no + " " + sk.boardSize + " " + game.white.getName() + " vs " + game.black.getName();
			msg2 += sk.time + " " + sk.komi + " " + sk.pieces + " " + sk.capstones + " " + sk.triggerMove + " " + sk.timeAmount;
		} else  {
			msg += "Game Start " + game.no +" "+game.white.getName()+" vs "+game.black.getName();
			msg2 += sk.boardSize + " " + sk.time + " " + sk.incr + " " + sk.komi + " " + sk.pieces + " " + sk.capstones + " " + sk.unrated + " " + sk.tournament + " " + sk.triggerMove + " " + sk.timeAmount + " ";
			if (sk.botSeek == 1) {
				msg2 += "1";
			} else {
				msg2 += "0";
			}
		}
		return msg + " " + ((game.white == player) ? "white" : "black") + " " + msg2;
	}

	public void onlinePlayerMessageHandler () {
		ArrayList<String> playerNames = new ArrayList<>();
		for(Client c: clientConnections) {
			if(c.player != null && !c.player.isbot)
				playerNames.add('"' + c.player.getName() + '"');
		}
		sendAllOnline("OnlinePlayers "+ playerNames);
	}

	public void addToRoom(String room) {
		chatRooms.add(room);
		ChatRoom.joinRoom(room,this);
		send("Joined room "+room);
	}

	public void removeFromRoom(String room) {
		chatRooms.remove(room);
		ChatRoom.leaveRoom(room,this);
	}

	public void removeFromAllRooms() {
		for(String room : chatRooms){
			ChatRoom.leaveRoom(room,this);
		}
		chatRooms.clear();
	}

	//this has more rights than p
	boolean moreRights(Player p) {
		//if i am mod and other is not mod
		if(player.isMod() && !p.isMod() || player.isAdmin())
			return true;

		return false;
	}

	void sudoHandler(String msg) {
		if(!player.isMod() && !player.isAdmin()) {
			sendNOK();
			return;
		}

		sendSudoReply("> "+msg);

		Matcher m;
		// Un Gag player
		if((m=unGagPattern.matcher(msg)).find()) {
			String name = m.group(1);
			Player p = Player.players.get(name);
			if(p == null) {
				sendSudoReply("No such player");
				return;
			}

			if(!moreRights(p)) {
				sendSudoReply("You don't have rights");
				return;
			}

			if(!p.isGagged()) {
				sendSudoReply("Player is not gagged");
				return;
			}

			p.unGag();
			p.setGagInDB(p.getName(), 0);
			sendSudoReply(p.getName()+" ungagged");
		}
		// Gag player
		else if((m=gagPattern.matcher(msg)).find()) {
			String name = m.group(1);
			Player p = Player.players.get(name);
			if(p == null) {
				sendSudoReply("No such player");
				return;
			}

			if(!moreRights(p)) {
				sendSudoReply("You don't have rights");
				return;
			}

			if(p.isGagged()) {
				sendSudoReply("Player is already gagged");
				return;
			}

			p.gag();
			p.setGagInDB(p.getName(), 1);
			sendSudoReply(p.getName()+" gagged");
		}
		// kick player
		else if((m=kickPattern.matcher(msg)).find()) {
			String name = m.group(1);
			Player p = Player.players.get(name);
			if(p == null) {
				sendSudoReply("No such player");
				return;
			}

			if(!moreRights(p)) {
				sendSudoReply("You dont have rights");
				return;
			}

			Client c = p.getClient();
			if(c == null) {
				sendSudoReply("Player not logged in");
				return;
			}

			c.disconnect();
			sendSudoReply(p.getName()+" kicked");
		}
		// ban player
		else if((m=banPattern.matcher(msg)).find()) {
			String name = m.group(1);
			String reason = m.group(2);
			Player p = Player.players.get(name);
			if(p == null) {
				sendSudoReply("No such player");
				return;
			}

			if(!moreRights(p)) {
				sendSudoReply("You don't have rights");
				return;
			}
			// check if player is already banned
			if(p.isBanned()) {
				sendSudoReply("Player is already banned");
				return;
			}

			p.setBan();
			p.setBanInDB(p.getName(), 1);
			// email player that they have been banned
			EMail.send(p.getEmail(), "PlayTak.com Ban Notice", p.getName() + ",\rYou have been banned from playtak.com!\rReason: " + reason);
			// kick player as well
			Client c = p.getClient();
			if(c == null) {
				sendSudoReply("Player not logged in");
				return;
			}
			c.disconnect();
			sendSudoReply(p.getName()+" kicked");
			sendSudoReply(p.getName()+" banned");
		}
		// un-ban player
		else if((m=unBanPattern.matcher(msg)).find()) {
			String name = m.group(1);
			Player p = Player.players.get(name);
			if(p == null) {
				sendSudoReply("No such player");
				return;
			}

			if(!moreRights(p)) {
				sendSudoReply("You don't have rights");
				return;
			}
			if(!p.isBanned()){
				sendSudoReply("Player is not banned");
				return;
			}

			p.unBan();
			p.setBanInDB(p.getName(), 0);
			sendSudoReply(p.getName()+" unbanned");
		}
		// list commands
		else if((m=listCmdPattern.matcher(msg)).find()) {
			// privileged commands - only for admins and mods
			// return gag list
			if("gag".equals(m.group(1))) {
				String res="[";
				for(Player p: Player.gagList)
					res += p.getName()+", ";

				sendSudoReply(res+"]");
			}
			// return mod list
			else if ("mod".equals(m.group(1))) {
				String res = "[";
				for(Player p: Player.modList)
					res += p.getName()+", ";

				sendSudoReply(res+"]");
			}
			// return mod list
			else if ("ban".equals(m.group(1))) {
				String res = "[";
				for(Player p: Player.banList)
					res += p.getName()+", ";

				sendSudoReply(res+"]");
			}
			// admin list
			else if ("admin".equals(m.group(1))) {
				String res = "[";
				for(Player p: Player.adminList)
					res += p.getName()+", ";
				sendSudoReply(res+"]");
			}
			else if("online".equals(m.group(1))) {
					String res = "[";
					for(Client c: clientConnections) {
						if(c.player != null)
							res += c.player.getName()+", ";
					}
					sendSudoReply(res+"]");
			} else {
				sendSudoReply("command not found");
			}
		}
		// admin commands
		// broadcast message
		else if((m=broadcastPattern.matcher(msg)).find()) {
			if(!player.isAdmin()) {
				sendSudoReply("Permission denied");
				return;
			}
			String bmsg = m.group(1);
			Client.sendAllOnline(bmsg);
			return;
		}
		// reload bad word filter
		else if((m=reloadWordCmdPattern.matcher(msg)).find()) {
			if(!player.isAdmin()) {
				sendSudoReply("Permission denied");
				return;
			}
			BadWordFilter.loadConfigs();
			sendSudoReply("Bad word filter reloaded");
		}
		// add mods
		else if((m=modPattern.matcher(msg)).find()) {
			if(!player.isAdmin()) {
				sendSudoReply("Permission denied");
				return;
			}
			String name = m.group(1);
			Player p = Player.players.get(name);
			if(p == null) {
				sendSudoReply("No such player");
				return;
			}
			p.setMod();
			p.setModInDB(name, 1);
			sendSudoReply("Added "+p.getName()+" as moderator");
		}
		// Remove mod from list
		else if((m=unModPattern.matcher(msg)).find()) {
			if(!player.isAdmin()) {
				sendSudoReply("Permission denied");
				return;
			}
			String name = m.group(1);
			Player p = Player.players.get(name);
			if(p == null) {
				sendSudoReply("No such player");
				return;
			}
			p.unMod();
			p.setModInDB(name, 0);
			sendSudoReply("Removed "+p.getName()+" as moderator");
		}
		// add admin
		else if((m=adminPattern.matcher(msg)).find()) {
			if(!player.isAdmin()) {
				sendSudoReply("Permission denied");
				return;
			}
			String name = m.group(1);
			Player p = Player.players.get(name);
			if(p == null) {
				sendSudoReply("No such player");
				return;
			}
			p.setAdmin();
			p.setAdminInDB(name, 1);
			sendSudoReply("Added "+p.getName()+" as admin");
		}
		// Remove admin from list
		else if((m=unAdminPattern.matcher(msg)).find()) {
			if(!player.isAdmin()) {
				sendSudoReply("Permission denied");
				return;
			}
			String name = m.group(1);
			Player p = Player.players.get(name);
			if(p == null) {
				sendSudoReply("No such player");
				return;
			}
			p.unAdmin();
			p.setAdminInDB(name, 0);
			sendSudoReply("Removed "+p.getName()+" as admin");
		}
		// set bot
		else if((m=botPattern.matcher(msg)).find()) {
			if(!player.isAdmin()) {
				sendSudoReply("Permission denied");
				return;
			}
			String name = m.group(1);
			Player p = Player.players.get(name);
			if(p == null) {
				sendSudoReply("No such player");
				return;
			}
			p.setBot();
			p.setBotInDB(name, 1);
			sendSudoReply("Added "+p.getName()+" as bot");
		}
		// un set bot
		else if((m=unBotPattern.matcher(msg)).find()) {
			if(!player.isAdmin()) {
				sendSudoReply("Permission denied");
				return;
			}
			String name = m.group(1);
			Player p = Player.players.get(name);
			if(p == null) {
				sendSudoReply("No such player");
				return;
			}
			p.unBot();
			p.setBotInDB(name, 0);
			sendSudoReply("Removed "+p.getName()+" as bot");
		}
		// Admin set password for user
		else if((m=setPattern.matcher(msg)).find()) {
			if(!player.isAdmin()) {
				sendSudoReply("Permission denied");
				return;
			}
			String param = m.group(1);
			String name = m.group(2);
			String value = m.group(3);

			Player p = Player.players.get(name);
			if(p == null) {
				sendSudoReply("No such player");
				return;
			}
			if(param.equals("password")) {
				p.setPassword(value);
				sendSudoReply("Password set");
			}
		}
		else {
			sendSudoReply("Command not found");
		}
	}

	static void sigterm() {
		TakServer.Log("Sigterm!");
		try {
			BufferedReader br = new BufferedReader(new FileReader(new File("message")));
			String msg = br.readLine();
			sendAll("Message "+msg);

			int sleep=Integer.parseInt(br.readLine());
			TakServer.Log("sleeping "+sleep+" milliseconds");
			Thread.sleep(sleep);
			sendAll("Message "+br.readLine());

			TakServer.Log("Exiting");
		} catch (IOException | NumberFormatException | InterruptedException ex) {
			TakServer.Log(ex);
		}

	}

	@Override
	public void subscribe(Subscriber<? super GameUpdate> subscriber) {
		subscribers.add(subscriber);
	}
	protected void notifySubscribers(GameUpdate update) {
		for (var subscriber: subscribers) {
			subscriber.onNext(update);
		}
	}
}

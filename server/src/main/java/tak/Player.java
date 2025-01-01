/*
 * To change this license header, choose License Headers in Project Properties.
 * To change this template file, choose Tools | Templates
 * and open the template in the editor.
 */
package tak;

import java.math.BigInteger;
import java.security.SecureRandom;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.logging.Level;
import java.util.logging.Logger;
import org.mindrot.jbcrypt.BCrypt;
import tak.utils.ConcurrentHashSet;
import java.util.concurrent.locks.*;

/**
 *
 * @author chaitu
 */
public class Player {
	public static Lock loginLock=new ReentrantLock();
	public static Map<String, Player> players = new ConcurrentHashMap<>();
	public static Map<String, Player> guestsByToken = new ConcurrentHashMap<>();
	public static ConcurrentHashSet<Player> modList = new ConcurrentHashSet<>();
	public static ConcurrentHashSet<Player> gagList = new ConcurrentHashSet<>();
	public static ConcurrentHashSet<Player> banList = new ConcurrentHashSet<>();
	public static ConcurrentHashSet<String> takenName = new ConcurrentHashSet<>();
	
	static int idCount=0;
	static AtomicInteger guestCount = new AtomicInteger(0);
	
	private final String name;
	private String password;
	private final String email;
	private final int id;//Primary key
	
	private final  boolean guest;
	public boolean isbot;
	public boolean is_admin;
	public boolean is_mod;
	private boolean gag;//don't broadcast his shouts or tells
	public boolean is_banned;

	//variables not in database
	public Client client;
	private Game game;
	public long lastActivity;
	public static long lastCleanup=System.nanoTime();
	
	private String resetToken;
	
	static void cleanUpGuests(){
		long now=System.nanoTime();
		if(now-lastCleanup<3600000000000L){
			return;
		}
		lastCleanup=now;
		loginLock.lock();
		try{
			guestsByToken.forEach( (k, v) -> {
				if(!v.isLoggedIn() && now-v.lastActivity>20000000000000L){
					guestsByToken.remove(k);
					players.remove(v.name);
				}
			});
		}
		finally{
			loginLock.unlock();
		}
	}
	
	Player(String name, String email, String password, int id, boolean guest, boolean bot, boolean admin, boolean is_mod, boolean is_banned, boolean is_gagged) {
		this.name = name;
		this.email = email;
		this.password = password;
		this.id = id;
		this.guest = guest;
		this.resetToken = "";
		this.isbot=bot;
		this.is_admin = admin;
		this.is_mod = is_mod;
		this.is_banned = is_banned;
		this.gag = is_gagged;

		if(is_mod){ setMod(); }
		if(is_banned) { setBan(); }
		if(is_gagged){ gag(); }
		
		client = null;
		game = null;
		this.lastActivity = System.nanoTime();
	}
	
	public static String uniqifyName(String name){
		name=name.toLowerCase();
		name=name.replaceAll("[^0-9a-z]","");
		name=name.replaceAll("[il]","1");
		name=name.replaceAll("[o]","0");
		return name;
	}
	
	public static void takeName(String name){
		takenName.add(uniqifyName(name));
	}
	
	public static boolean isNameTaken(String name){
		return takenName.contains(uniqifyName(name));
	}
	
	public static String hash(String st) {
		return BCrypt.hashpw(st, BCrypt.gensalt());
	}
	
	public boolean authenticate(String candidate) {
		if(guest){
			return false;
		}
		if(isBanned()) {
			return false;
		}
		else{
			try{
				return BCrypt.checkpw(candidate, password);
			}
			catch(IllegalArgumentException e){
				return false;
			}
		}
	}
	
	public void sendResetToken() {
		resetToken = new BigInteger(130, random).toString(32);
		EMail.send(this.email, "playtak.com reset password", "Your reset token is " + resetToken);
	}
	
	public boolean resetPassword(String token, String newPass) {
		if((!"".equals(resetToken)) && token.equals(resetToken)) {
			setPassword(newPass);
			resetToken = "";
			return true;
		}
		return false;
	}
	
	public boolean isLoggedIn() {
		return client!=null;
	}
	
	public Game getGame() {
		return game;
	}
	
	public void setGame(Game g) {
		game = g;
	}
	
	public void removeGame() {
		game = null;
	}
	
	public void send(String msg) {
		if(client != null)
			client.send(msg);
	}
	
	public void sendNOK() {
		if(client != null)
			client.sendNOK();
	}
	
	public void sendWithoutLogging(String msg) {
		if(client != null)
			client.sendWithoutLogging(msg);
	}
	
	public void login(Client c) {
		this.client = c;
		resetToken = "";//If a user is able to login, he has the password
		
		if(game != null)
			game.playerRejoin(this);
	}
	
	public void logout() {
		if(client != null) {
			client.disconnect();
			this.client = null;
		}
	}
	
	public boolean isMod() {
		return is_mod;
	}
	
	public void setMod() {
		is_mod = true;
		modList.add(this);
	}

	public void setModInDB(String name, int mod) {
		String sql = "UPDATE players set is_mod = ? where name = ?;";
		try {
			PreparedStatement stmt = Database.playersConnection.prepareStatement(sql);
			stmt.setInt(1, mod);
			stmt.setString(2, name);
			stmt.executeUpdate();
			stmt.close();
		} catch (SQLException ex) {
			Logger.getLogger(Player.class.getName()).log(Level.SEVERE, null, ex);
		}
	}
	
	public void unMod() {
		is_mod = false;
		modList.remove(this);
	}

	public boolean isAdmin() {
		return is_admin;
	}
	
	public void gag() {
		gag = true;
		Player.gagList.add(this);
	}

	public void setGagInDB(String name, int gagged){
		String sql = "UPDATE players set is_gagged = ? where name = ?;";
		try {
			PreparedStatement stmt = Database.playersConnection.prepareStatement(sql);
			stmt.setInt(1, gagged);
			stmt.setString(2, name);
			stmt.executeUpdate();
			stmt.close();
		} catch (SQLException ex) {
			Logger.getLogger(Player.class.getName()).log(Level.SEVERE, null, ex);
		}
	}
	
	public void unGag() {
		gag = false;
		Player.gagList.remove(this);
	}
	
	public boolean isGagged() {
		return gag;
	}
	
	public void setBan() {
		this.is_banned = true;
		Player.banList.add(this);
	}
	
	public void unBan() {
		this.is_banned = false;
		Player.banList.remove(this);
	}

	public void setBanInDB(String name, int banned) {
		String sql = "UPDATE players set is_banned = ? where name = ?;";
		try {
			PreparedStatement stmt = Database.playersConnection.prepareStatement(sql);
			stmt.setInt(1, banned);
			stmt.setString(2, name);
			stmt.executeUpdate();
			stmt.close();
		} catch (SQLException ex) {
			Logger.getLogger(Player.class.getName()).log(Level.SEVERE, null, ex);
		}
	}
	
	public boolean isBanned() {
		return is_banned;
	}
	
	public void loggedOut() {
		this.client = null;
		if(guest) {
			Player.modList.remove(this);
			Player.gagList.remove(this);
		}
	}
	
	Player(String name, String email, String password, boolean guest) {
		this(name, email, password, guest?0:++idCount, guest, false, false, false, false, false);
	}
	
	Player() {
		this("Guest"+guestCount.incrementAndGet(), "", "", true);
		Player.players.put(this.name, this);
		guestsByToken.put(this.name, this);
	}
	
	Player(String token) {
		this("Guest"+guestCount.incrementAndGet(), "", token, true);
		Player.players.put(this.name, this);
		guestsByToken.put(token, this);
	}
	
	Client getClient() {
		return client;
	}
	
	static SecureRandom random = new SecureRandom();
	public static Player createPlayer(String name, String email) {
		BigInteger pwsource = new BigInteger(95, random);
		BigInteger n26 = new BigInteger("26");
		String tmpPass = "";
		int a;
		// create a temp password
		for(a=0;a<5;a++){
			tmpPass+=pwsource.mod(n26).add(BigInteger.TEN).toString(36);
			pwsource=pwsource.divide(n26);
			tmpPass+=pwsource.mod(n26).add(BigInteger.TEN).toString(36);
			pwsource=pwsource.divide(n26);
			tmpPass+=pwsource.mod(BigInteger.TEN).toString();
			pwsource=pwsource.divide(BigInteger.TEN);
			tmpPass+=pwsource.mod(BigInteger.TEN).toString();
			pwsource=pwsource.divide(BigInteger.TEN);
		}
		
		Player np = new Player(name, email, Player.hash(tmpPass), false);
		String sql = "INSERT INTO players (id,name,password,email) VALUES (?, ?, ?, ?)";
		try {
			PreparedStatement stmt = Database.playersConnection.prepareStatement(sql);
			stmt.setInt(1, np.id);
			stmt.setString(2, np.name);
			stmt.setString(3, np.password);
			stmt.setString(4, np.email);

			stmt.executeUpdate();
			stmt.close();
			
			EMail.send(np.email, "playtak.com password", "Hello "+np.name+", your password is "+tmpPass+" You can change it on playtak.com.");
			players.put(np.name, np);
			takeName(np.name);
		} catch (SQLException ex) {
			Logger.getLogger(Player.class.getName()).log(Level.SEVERE, null, ex);
		}
		return np;
	}

	public static Player getByName(final String name) {
		return players.getOrDefault(name, null);
	}

	@Override
	public String toString() {
		return name+" "+password+" "+email;
	}
	
	public int getRating(long time){
		String sql="SELECT rating, ratingage, ratingbase, unrated FROM players WHERE id=?";
		double decayrate=1000.0*60.0*60.0*24.0*240.0;
		double rating=0.0;
		double ratingage=0.0;
		int ratingbase=0;
		int unrated=1;
		ResultSet rs=null;
		try{
			try( 
				PreparedStatement stmt = Database.playersConnection.prepareStatement(sql)
			){
				stmt.setInt(1, id);
				rs = stmt.executeQuery();
				if (rs.next()) {
					rating=rs.getDouble("rating");
					ratingage=rs.getDouble("ratingage");
					ratingbase=rs.getInt("ratingbase");
					unrated=rs.getInt("unrated");
				}
			} catch (SQLException ex) {
				Logger.getLogger(Player.class.getName()).log(Level.SEVERE, null, ex);
			}
			finally{
				if(rs!=null){
					rs.close();
				}
			}

			if(ratingbase!=0){
				try( 
					PreparedStatement stmt=Database.playersConnection.prepareStatement(sql)
				){
					stmt.setInt(1, ratingbase);
					rs = stmt.executeQuery();
					if (rs.next()) {
						rating=rs.getDouble("rating");
						ratingage=rs.getDouble("ratingage");
						ratingbase=rs.getInt("ratingbase");
						unrated=rs.getInt("unrated");
					}
				} catch (SQLException ex) {
					Logger.getLogger(Player.class.getName()).log(Level.SEVERE, null, ex);
				}
				finally{
					if(rs!=null){
						rs.close();
					}
				}
			}
		}catch (SQLException ex) {
			Logger.getLogger(Player.class.getName()).log(Level.SEVERE, null, ex);
		}
		if(unrated==1){
			return 0;
		}
		if(rating<1500.0){
			return (int)rating;
		}
		double decaytime=((double)time-ratingage)/decayrate;

		if(decaytime<1.0){
			return (int)rating;
		}
		double retention=Math.pow(0.5,decaytime)*2.0;
		if(rating<1700.0){
			return (int)Math.min(rating,1500.0+retention*200.0);
		}
		else{
			return (int)(rating-200.0*(1.0-retention));
		}
	}
	
	public void setPassword(String pass) {
		this.password = Player.hash(pass);
		
		String sql = "UPDATE players set password = ? where id = ?;";
		
		try {
			PreparedStatement stmt = Database.playersConnection.prepareStatement(sql);
			stmt.setString(1, this.password);
			stmt.setInt(2, id);
			stmt.executeUpdate();
			stmt.close();
		} catch (SQLException ex) {
			Logger.getLogger(Player.class.getName()).log(Level.SEVERE, null, ex);
		}
	}
	
	public String getName() {
		return name;
	}
	
	public String getEmail() {
		return email;
	}
	
	public String getPassword() {
		return password;
	}
	
	public static void loadFromDB() {
		idCount=0;
		try (Statement stmt = Database.playersConnection.createStatement();
				ResultSet rs = stmt.executeQuery("SELECT * FROM players;")) {
			while(rs.next()) {
				Player np = new Player(
					rs.getString("name"),
					rs.getString("email"),
					rs.getString("password"),
					rs.getInt("id"),
					false,
					rs.getInt("isbot") == 1,
					rs.getInt("is_admin") == 1,
					rs.getInt("is_mod") == 1,
					rs.getInt("is_banned") == 1,
					rs.getInt("is_gagged") == 1
				);
				players.put(np.name, np);
				takeName(np.name);
				if(idCount<np.id)
					idCount=np.id;
			}
		} catch (SQLException ex) {
			Logger.getLogger(Player.class.getName()).log(Level.SEVERE, null, ex);
		}
	}
}

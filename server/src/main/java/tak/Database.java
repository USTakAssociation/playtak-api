/*
 * To change this license header, choose License Headers in Project Properties.
 * To change this template file, choose Tools | Templates
 * and open the template in the editor.
 */
package tak;

import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.SQLException;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 *
 * @author chaitu
 */
public class Database {
	public static Connection playersConnection;
	public static Connection gamesConnection;
	public static String dbPath;

	public static void initConnection() {
		try {
			Class.forName("org.sqlite.JDBC");
			final String pathPlayersDb = "jdbc:sqlite:" + dbPath + "players.db";
			final String pathGamesDb = "jdbc:sqlite:" + dbPath + "games.db";
			
			System.out.println("Looking for database " + pathPlayersDb);
			playersConnection = DriverManager.getConnection(pathPlayersDb);
			System.out.println("Looking for database " + pathGamesDb);
			gamesConnection = DriverManager.getConnection(pathGamesDb);

			System.out.println("databases connected...");
		} catch (ClassNotFoundException | SQLException ex) {
			Logger.getLogger(Database.class.getName()).log(Level.SEVERE, null, ex);
		}
	}
}

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
import com.zaxxer.hikari.HikariConfig;
import com.zaxxer.hikari.HikariDataSource;

/**
 *
 * @author chaitu
 */
public class Database {
	public static Connection dbConnection;
	public static HikariDataSource ds;
	public static String dbHost;
	public static Integer dbPort;
	public static String dbUsername;
	public static String dbPassword;
	public static String dbName;
	

	public static void initConnection() {		
		try {
			try {
						Class.forName("org.mariadb.jdbc.Driver");
				} catch (ClassNotFoundException e) {
						Logger.getLogger(Database.class.getName()).log(Level.SEVERE, "MariaDB JDBC Driver not found", e);
						throw new RuntimeException("MariaDB JDBC Driver not found", e);
				}
			Class.forName("org.mariadb.jdbc.Driver");
			HikariConfig config = new HikariConfig();
			config.setDriverClassName("org.mariadb.jdbc.Driver");
			config.setJdbcUrl("jdbc:mariadb://" + dbHost + ":" + dbPort + "/" + dbName);
			config.setUsername(dbUsername);
			config.setPassword(dbPassword);
			
			// Additional connection pool settings
			config.setMaximumPoolSize(10);
			config.setMinimumIdle(5);
			
			ds = new HikariDataSource(config);
			dbConnection = ds.getConnection();
			
			// HikariConfig config = new HikariConfig();
			// config.setJdbcUrl("jdbc:mysql://" + dbHost + ":" + dbPort + "/" + dbName );
			// config.setUsername(dbUsername);
			// config.setPassword(dbPassword);

			// HikariDataSource ds = new HikariDataSource(config);
			//ds = (DataSource) ctx.lookup("jdbc:mysql://" + dbHost + ":" + dbPort + "/" + dbName + "?user=" + dbUsername + "&password=" + dbPassword);
			dbConnection = ds.getConnection();

			System.out.println("database connected...");
		} catch (ClassNotFoundException | SQLException ex) {
			Logger.getLogger(Database.class.getName()).log(Level.SEVERE, null, ex);
		}
	}
}

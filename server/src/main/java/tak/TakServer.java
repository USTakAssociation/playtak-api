/*
 * To change this license header, choose License Headers in Project Properties.
 * To change this template file, choose Tools | Templates
 * and open the template in the editor.
 */
package tak;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.ServerSocket;
import java.net.Socket;
import java.net.SocketTimeoutException;
import java.util.Date;
import java.util.logging.Level;
import java.util.logging.Logger;

import com.sun.net.httpserver.HttpServer;

import tak.httpHandlers.AddSeekHandler;

/**
 *
 * @author chaitu
 */
public class TakServer extends Thread {
	public static int port;
	public static int portws;
	public static int portHttp;

	protected static Logger logger = Logger.getLogger(TakServer.class.getName());

	@Override
	public void run() {
		ServerSocket ssocket;
		ServerSocket wsocket;
		HttpServer httpServer;
		// TODO disabling this for now till the native tournament is ready
		// var gameUpdateBroadcaster = new GameUpdateBroadcaster();
		// new Thread(gameUpdateBroadcaster).start();

		try {
			ssocket = new ServerSocket(port);
			Log("Server running at " + port);
			ssocket.setSoTimeout(70);
			wsocket = new ServerSocket(portws);
			Log("WebSocket Server running at " + portws);
			wsocket.setSoTimeout(70);

			// HTTP Server example from https://stackoverflow.com/questions/3732109/simple-http-server-in-java-using-only-java-se-api
			httpServer = HttpServer.create(new InetSocketAddress(portHttp), 0);
			Log("HTTPServer running at " + portHttp);

			// TODO: this matches `/api/v1/seeks*` but should only match exactly `/api/v1/seeks`
			httpServer.createContext("/api/v1/seeks", new AddSeekHandler());
			// Enables parallel processing of requests, probably not necessary
			httpServer.setExecutor(java.util.concurrent.Executors.newCachedThreadPool());
			httpServer.start();

			while (true) {
				try {
					Date date = new Date();
					if ((date.getTime() & 128) == 128) {
						Socket socket = ssocket.accept();
						TakServer.Log("New Telnet client");
						Client cc = new Client(new Telnet(socket));
						// TODO disabling this for now till the native tournament is ready
						//cc.subscribe(gameUpdateBroadcaster);
						cc.start();
					} else {
						Socket socket = wsocket.accept();
						TakServer.Log("New Websocket client");
						Client cc = new Client(new Websocket(socket));
						// TODO disabling this for now till the native tournament is ready
						//cc.subscribe(gameUpdateBroadcaster);
						cc.start();
					}
				} catch (SocketTimeoutException e) {

				}
			}
		} catch (IOException ex) {
			logger.log(Level.SEVERE, null, ex);
		}
		// TODO disabling this for now till the native tournament is ready
		//gameUpdateBroadcaster.stop();
	}

	static void Log(Object obj) {
		logger.info(obj.toString());
	}
}

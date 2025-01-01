/*
 * To change this license header, choose License Headers in Project Properties.
 * To change this template file, choose Tools | Templates
 * and open the template in the editor.
 */
package tak;

import java.io.File;
import java.io.IOException;
import java.net.MalformedURLException;
import java.net.URL;
import java.util.logging.Level;
import java.util.logging.Logger;
import javax.xml.parsers.DocumentBuilder;
import javax.xml.parsers.DocumentBuilderFactory;
import javax.xml.parsers.ParserConfigurationException;
import org.w3c.dom.Document;
import org.w3c.dom.Element;
import org.w3c.dom.Node;
import org.w3c.dom.NodeList;
import org.xml.sax.SAXException;

/**
 *
 * @author chaitu
 */
public class Settings {
	public static Logger logger = Logger.getLogger(Settings.class.getName());

	public static Document doc = null;
	public static void parse() {
		File xmlFile = new File("properties.xml");
		DocumentBuilderFactory dbFactory = DocumentBuilderFactory.newInstance();
		DocumentBuilder dBuilder;
		try {
			dBuilder = dbFactory.newDocumentBuilder();
			doc = dBuilder.parse(xmlFile);
			doc.getDocumentElement().normalize();
		} catch (ParserConfigurationException | SAXException | IOException ex) {
			logger.log(Level.SEVERE, null, ex);
		}
		
		parseEmail();
		parseIRC();
		parseGame();
		parseServer();
	}
	
	private static void parseEmail() {
		NodeList nList = doc.getElementsByTagName("email");
		Node node = nList.item(0);
		Element element = (Element)node;
		EMail.host = element.getElementsByTagName("host").item(0).getTextContent();
		EMail.user = element.getElementsByTagName("user").item(0).getTextContent();
		EMail.password = element.getElementsByTagName("password").item(0).getTextContent();
		EMail.from = element.getElementsByTagName("from").item(0).getTextContent();
		
		logger.info("from "+EMail.from+" host "+EMail.host);
	}
	
	private static void parseIRC() {
		NodeList nList = doc.getElementsByTagName("irc");
		Node node = nList.item(0);
		Element element = (Element)node;
		
		IRCBridge.enabled = "true".equals(element.getElementsByTagName("enabled").item(0).getTextContent());
		IRCBridge.server = element.getElementsByTagName("server").item(0).getTextContent();
		IRCBridge.nick = element.getElementsByTagName("nick").item(0).getTextContent();
		IRCBridge.login = element.getElementsByTagName("login").item(0).getTextContent();
		IRCBridge.channel = element.getElementsByTagName("channel").item(0).getTextContent();
		IRCBridge.password = element.getElementsByTagName("password").item(0).getTextContent();
	}
	
	private static void parseGame() {
		NodeList nList = doc.getElementsByTagName("game");
		Node node = nList.item(0);
		Element element = (Element)node;
		
		Game.reconnectionTime = Integer.parseInt(element.getElementsByTagName("reconnection-time").item(0).getTextContent());
	}

	private static void parseServer() {
		NodeList nList = doc.getElementsByTagName("server-settings");
		Node node = nList.item(0);
		Element element = (Element)node;
		TakServer.port = Integer.parseInt(element.getElementsByTagName("port").item(0).getTextContent());
		TakServer.portws = Integer.parseInt(element.getElementsByTagName("portws").item(0).getTextContent());
		TakServer.portHttp = Integer.parseInt(element.getElementsByTagName("porthttp").item(0).getTextContent());
		Database.dbPath = element.getElementsByTagName("db-path").item(0).getTextContent();
		try {
			String fieldName = "event-subscriber-url";
			GameUpdateBroadcaster.eventSubscriberUrl = new URL(element.getElementsByTagName(fieldName).item(0).getTextContent());
		}
		catch (MalformedURLException ex) {
			logger.log(Level.SEVERE, "Could not parse event-subscriber-url", ex);
		}
	}
}

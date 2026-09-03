package tak;

import java.io.InputStream;
import java.io.OutputStream;
import java.net.Socket;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.locks.Lock;
import java.util.concurrent.locks.ReentrantLock;

/**
 *
 * @author Nohat
 */
public class Telnet extends Websocket {
	InputStream stream;
	OutputStream outstream;
	final int buffersize = 0x10010;
	byte[] readbuffer;
	ByteBuffer readbufferobj;
	int readbufferused;
	int readbufferoffset;

	Lock writelock;

	Telnet(Socket socket) {
		this.socket = socket;
		streamended = false;
		try {
			socket.setSoTimeout(60 * 1000);
			stream = socket.getInputStream();
			outstream = socket.getOutputStream();
		} catch (Throwable t) {
			kill(101);
		}
		readbuffer = new byte[buffersize * 2 + 8];
		readbufferused = 0;
		readbufferobj = ByteBuffer.wrap(readbuffer);
		readbufferoffset = 0;
		headerended = true;

		writelock = new ReentrantLock();
	}

	public String recieve(boolean blocking) {
		try {
			if (streamended) {
				return null;
			}
			int readable = stream.available();
			if (readable > 0) {
				readbufferused += stream.read(readbuffer, readbufferused + readbufferoffset, Math.min(buffersize - readbufferused, readable));
			}

			int a;
			for (a = 0; a < readbufferused; a++) {
				if (readbuffer[a + readbufferoffset] == 10) {
					String msg = new String(readbuffer, readbufferoffset, a, StandardCharsets.ISO_8859_1);
					readbufferoffset += a + 1;
					readbufferused -= a + 1;

					if (readbufferoffset > buffersize) {
						readbufferobj.position(0);
						readbufferobj.put(readbuffer, readbufferoffset, readbufferused);
						readbufferoffset = 0;
					}
					return msg;
				}
			}
			if (readbufferused >= buffersize) {
				kill(102);
				return null;
			}
			if (blocking) {
				int dataread = stream.read(readbuffer, readbufferused + readbufferoffset, buffersize - readbufferused);
				if (dataread < 0) {
					kill(103);
					return null;
				}
				readbufferused += dataread;
				return recieve(false);
			}
			return null;
		} catch (Throwable t) {
			kill(104);
			return null;
		}
	}

	public void send(String msg) {
		if (streamended) {
			return;
		}
		writelock.lock();
		try {
			byte[] bytes = msg.getBytes(StandardCharsets.ISO_8859_1);
			outstream.write(bytes);
			outstream.write(10);
			outstream.flush();
		} catch (Throwable t) {
			kill(105);
		} finally {
			writelock.unlock();
		}
	}

	public void kill(int pos) {
		if (streamended) {
			return;
		}
		try {
			streamended = true;
			try {
				socket.close();
			} catch (Throwable ignore) {
			}

			String remote = "unknown";
			try {
				if (socket != null && socket.getRemoteSocketAddress() != null) remote = socket.getRemoteSocketAddress().toString();
			} catch (Throwable ignore) {}

			String reason;
			switch (pos) {
				case 101:
					reason = "socket setup failure";
					break;
				case 102:
					reason = "buffer overflow";
					break;
				case 103:
					reason = "blocking read returned < 0";
					break;
				case 104:
					reason = "exception in recieve";
					break;
				case 105:
					reason = "exception in send";
					break;
				case 201:
					reason = "client quit";
					break;
				case 202:
					reason = "admin disconnect";
					break;
				default:
					reason = "code " + String.valueOf(pos);
					break;
			}

			TakServer.Log(clientNo + ":" + playerName + ":Stream dead " + String.valueOf(pos) + " (" + reason + ") remote=" + remote + " headerended=" + headerended + " readbufferused=" + readbufferused);
		} catch (Throwable t) {
			try {
				TakServer.Log("Error in kill: " + t.getMessage());
			} catch (Throwable ignore) {}
		}
	}
}

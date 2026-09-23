package tak;

import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.net.ServerSocket;
import java.net.Socket;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.logging.Handler;
import java.util.logging.Level;
import java.util.logging.LogRecord;
import java.util.logging.Logger;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Reproduces the stale-connection reaper race described in the "so many
 * socket closed" investigation: a connection that dies (is reaped, or the
 * socket is otherwise closed) while a client thread is still blocked
 * reading the WebSocket handshake used to be cleaned up twice - once by
 * whoever closed it, and once by Client.run()'s own finally block - which
 * produced duplicate "Stream dead"/"disconnected" log lines and a spurious
 * "Welcome sent" log for a connection that was never actually sent one.
 */
class ClientCleanupRaceTest {

	private final List<String> logMessages = Collections.synchronizedList(new ArrayList<>());
	private Handler captureHandler;
	private Logger takServerLogger;

	private ServerSocket serverSocket;
	private Socket serverSideSocket; // what Websocket wraps, i.e. the "client connection" from the server's POV
	private Socket clientSideSocket; // the peer end; left open and silent to simulate a stuck handshake

	@BeforeEach
	void setUp() throws IOException {
		logMessages.clear();
		takServerLogger = Logger.getLogger(TakServer.class.getName());
		takServerLogger.setLevel(Level.ALL);
		captureHandler = new Handler() {
			@Override
			public void publish(LogRecord record) {
				logMessages.add(record.getMessage());
			}

			@Override
			public void flush() {
			}

			@Override
			public void close() {
			}
		};
		takServerLogger.addHandler(captureHandler);

		serverSocket = new ServerSocket(0);
		clientSideSocket = new Socket("localhost", serverSocket.getLocalPort());
		serverSideSocket = serverSocket.accept();
	}

	@AfterEach
	void tearDown() throws IOException {
		takServerLogger.removeHandler(captureHandler);
		Client.clientConnections.clear();
		closeQuietly(clientSideSocket);
		closeQuietly(serverSideSocket);
		closeQuietly(serverSocket);
	}

	private void closeQuietly(java.io.Closeable c) {
		try {
			if (c != null) c.close();
		} catch (IOException ignore) {
		}
	}

	private long countMessagesContaining(String substring) {
		synchronized (logMessages) {
			return logMessages.stream().filter(m -> m.contains(substring)).count();
		}
	}

	@Test
	void killIsIdempotent_secondCallDoesNotReLogOrReClose() {
		Websocket ws = new Websocket(serverSideSocket);

		ws.kill(201); // e.g. the stale-connection reaper
		ws.kill(13);  // e.g. the client's own thread reacting to the socket being closed out from under it

		assertTrue(ws.streamended);
		assertEquals(1, countMessagesContaining("Stream dead"),
			"kill() should only log once even if called twice for the same connection");
	}

	@Test
	void concurrentKillCallsOnlyRunCleanupOnce_websocket() throws InterruptedException {
		Websocket ws = new Websocket(serverSideSocket);
		runConcurrentKills(ws::kill);

		assertTrue(ws.streamended);
		assertEquals(1, countMessagesContaining("Stream dead"),
			"concurrent kill() calls racing on the same connection should only perform cleanup once");
	}

	@Test
	void concurrentKillCallsOnlyRunCleanupOnce_telnet() throws InterruptedException {
		Telnet telnet = new Telnet(serverSideSocket);
		runConcurrentKills(telnet::kill);

		assertTrue(telnet.streamended);
		assertEquals(1, countMessagesContaining("Stream dead"),
			"concurrent kill() calls racing on the same connection should only perform cleanup once");
	}

	private void runConcurrentKills(java.util.function.IntConsumer kill) throws InterruptedException {
		int threadCount = 20;
		ExecutorService pool = Executors.newFixedThreadPool(threadCount);
		CountDownLatch ready = new CountDownLatch(threadCount);
		CountDownLatch go = new CountDownLatch(1);
		for (int i = 0; i < threadCount; i++) {
			final int pos = i;
			pool.execute(() -> {
				ready.countDown();
				try {
					go.await();
				} catch (InterruptedException ignore) {
				}
				kill.accept(pos);
			});
		}
		ready.await();
		go.countDown();
		pool.shutdown();
		assertTrue(pool.awaitTermination(5, TimeUnit.SECONDS), "kill() calls did not complete in time");
	}

	@Test
	void clientQuitIsIdempotent_secondCallIsANoOp() throws IOException {
		Client client = new Client(new Websocket(serverSideSocket));
		assertTrue(Client.clientConnections.contains(client));

		client.clientQuit(); // e.g. the stale-connection reaper cleaning it up
		assertFalse(Client.clientConnections.contains(client));

		client.clientQuit(); // e.g. Client.run()'s finally block running afterwards

		assertEquals(1, countMessagesContaining("disconnected"),
			"clientQuit() should only perform cleanup (and log) once");
		assertEquals(1, countMessagesContaining("Stream dead"),
			"the second clientQuit() call should not re-trigger kill()'s logging either");
	}

	@Test
	void reaperKillingAConnectionMidHandshake_doesNotProduceAWelcomeSentLog() throws InterruptedException, IOException {
		// This is the actual production scenario: a connection is still stuck
		// in the pre-login handshake read (headerended == false) when
		// something else (the stale-connection reaper, in production) closes
		// it out from under the client's own thread.
		Client client = new Client(new Websocket(serverSideSocket));
		client.start();

		// Give Client.run() a moment to actually enter the blocking
		// handshake read before we pull the rug out from under it.
		Thread.sleep(200);

		client.clientQuit(); // simulates Client.cleanupStaleConnections() reaping it

		client.join(2000);
		assertFalse(client.isAlive(), "client thread should have exited after its socket was closed");

		assertEquals(0, countMessagesContaining("Welcome sent"),
			"a connection that died before completing the handshake was never actually welcomed and should not claim it was");
		assertEquals(1, countMessagesContaining("disconnected"),
			"the connection should be logged as disconnected exactly once, not once per thread that noticed");
	}

	@Test
	void remoteResetDuringHandshake_singleCleanDisconnectNoWelcomeSent() throws InterruptedException, IOException {
		// A second, single-threaded way this same bug showed up in production:
		// no reaper involved at all - the remote peer just resets the TCP
		// connection (RST, not a graceful close) while we're still blocked
		// reading the handshake. That alone used to produce a spurious
		// "Welcome sent" plus a duplicate "Stream dead" line (once from
		// recieve()'s own catch block calling kill(13), once from
		// clientQuit()'s kill(201) in the run() finally block).
		Client client = new Client(new Websocket(serverSideSocket));
		client.start();

		Thread.sleep(200); // let it block in the handshake read

		// setSoLinger(true, 0) forces an RST on close instead of a graceful
		// FIN, matching "SocketException: Connection reset" from the logs.
		clientSideSocket.setSoLinger(true, 0);
		clientSideSocket.close();

		client.join(2000);
		assertFalse(client.isAlive(), "client thread should have exited after the connection was reset");

		assertEquals(0, countMessagesContaining("Welcome sent"),
			"a connection reset before completing the handshake should not claim it was welcomed");
		assertEquals(1, countMessagesContaining("disconnected"));
		assertEquals(1, countMessagesContaining("Stream dead"),
			"recieve()'s own kill(13) and clientQuit()'s kill(201) should not both log");
	}
}

/*
 * To change this license header, choose License Headers in Project Properties.
 * To change this template file, choose Tools | Templates
 * and open the template in the editor.
 */
package tak;

import java.net.URISyntaxException;
import java.net.URL;
import java.net.http.HttpClient;
import java.net.http.HttpClient.Redirect;
import java.net.http.HttpRequest;
import java.net.http.HttpRequest.BodyPublishers;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.concurrent.BlockingDeque;
import java.util.concurrent.LinkedBlockingDeque;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.Flow.Subscriber;
import java.util.concurrent.Flow.Subscription;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.logging.Level;
import java.util.logging.Logger;

import com.fasterxml.jackson.annotation.JsonAutoDetect.Visibility;
import com.fasterxml.jackson.annotation.PropertyAccessor;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;

import tak.FlowMessages.GameUpdate;

/**
 * This class should be subscribed to all games and send out updates to HTTP endpoints
 * of subscribers.
 */
public final class GameUpdateBroadcaster implements Runnable, Subscriber<GameUpdate> {
	public static URL eventSubscriberUrl;

	private Logger logger = Logger.getLogger(TakServer.class.getName());
	private BlockingDeque<GameUpdate> updatesToBroadcast = new LinkedBlockingDeque<>();

	private AtomicBoolean stopped = new AtomicBoolean(false);

	public void stop() {
		logger.info("stopping " + GameUpdateBroadcaster.class.getSimpleName() + " ...");
		stopped.set(true);
	}

	public void run() {
		stopped.set(false);
		if (eventSubscriberUrl == null) {
			logger.severe("failed to start " + GameUpdateBroadcaster.class.getSimpleName() + " because no event-subscriber-url is defined");
			stopped.set(true);
			return;
		}

		logger.info("started " + GameUpdateBroadcaster.class.getSimpleName() + " with event-subscriber-url=" + eventSubscriberUrl);
		try {
			while(!stopped.get()) {
				var update = updatesToBroadcast.poll(1, TimeUnit.SECONDS);
				if (update == null) {
					continue;
				}

				if (logger.isLoggable(Level.FINE)) {
					logger.fine("broadcasting " + update.toString());
				} else {
					logger.info("broadcasting " + update.type + " game#" + update.game.id);
				}

				try {
					var jsonMapper = new ObjectMapper()
						.configure(SerializationFeature.FAIL_ON_EMPTY_BEANS, false)
						.setVisibility(PropertyAccessor.FIELD, Visibility.DEFAULT);
					var jsonString = jsonMapper.writeValueAsString(update);
					logger.fine(jsonString);

					var client = HttpClient.newBuilder()
						.connectTimeout(Duration.ofSeconds(5))
						.followRedirects(Redirect.NORMAL)
						.build();
					var request = HttpRequest.newBuilder(eventSubscriberUrl.toURI())
						.POST(BodyPublishers.ofString(jsonString))
						.timeout(Duration.ofSeconds(15))
						.header("Content-Type", "application/json")
						.build();
					var promise = client.sendAsync(request, HttpResponse.BodyHandlers.ofString());
					promise.join();
				}
				catch(URISyntaxException ex) {
					logger.log(Level.WARNING, "while notifying " + eventSubscriberUrl, ex);
				}
				catch(JsonProcessingException ex) {
					logger.log(Level.WARNING, "while serializing " + update, ex);
				}
			}
		}
		catch (InterruptedException ex) {
			logger.log(Level.WARNING, "loop was interrupted", ex);
		}

		logger.info("stopped " + GameUpdateBroadcaster.class.getSimpleName());
	}

	@Override
	public void onSubscribe(Subscription subscription) {
	}

	@Override
	public void onNext(GameUpdate update) {
		if(stopped.get()) { // prevent queue from filling up when stopped, crashed, or never started properly (due to missing URL);
			logger.severe("Cannot accept updates while stopped (" + update.type + ")");
			return;
		}

		logger.info(update.type);
		if (logger.isLoggable(Level.FINE)) {
			logger.fine(update.toString());
		}

		if (!updatesToBroadcast.offer(update)) {
			logger.severe("Failed to add to deque because there is no space" + update.toString());
		}
	}

	@Override
	public void onError(Throwable throwable) {
		System.err.println("Broadcaster error");
		logger.fine(throwable.toString());
	}

	@Override
	public void onComplete() {
	}
}

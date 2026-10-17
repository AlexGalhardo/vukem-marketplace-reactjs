import { useEffect, useRef } from "react";

// Channels the server streams on GET /events (Go: realtime.Orders and
// realtime.Notifications). The SSE event type is the channel name.
export type EventChannel = "orders" | "notifications";

// useEventStream subscribes to the signed-in user's realtime events and
// calls onEvent with each JSON payload of channel. The browser reconnects
// on its own after a network drop; events sent while disconnected are not
// replayed, so pages should also reload their state (e.g. router.reload)
// when a status matters. Pass enabled=false for guests: /events requires
// a session.
export function useEventStream<T = unknown>(
	channel: EventChannel,
	onEvent: (payload: T) => void,
	enabled = true,
): void {
	// Keep the latest callback without reopening the connection.
	const handler = useRef(onEvent);
	useEffect(() => {
		handler.current = onEvent;
	}, [onEvent]);

	useEffect(() => {
		if (!enabled || typeof EventSource === "undefined") return;

		const source = new EventSource("/events");
		const listener = (event: MessageEvent<string>) => {
			try {
				handler.current(JSON.parse(event.data) as T);
			} catch {
				// Not JSON: ignore rather than break the page.
			}
		};

		source.addEventListener(channel, listener);
		return () => {
			source.removeEventListener(channel, listener);
			source.close();
		};
	}, [channel, enabled]);
}

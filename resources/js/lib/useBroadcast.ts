import { useEffect, useRef, useState } from "react";

// useBroadcast subscribes to vukem broadcast channels (GET /broadcast/ws,
// the drivers/websocket protocol) and reports the connection status.
//
// Transports, in order: WebSocket, reconnecting with exponential backoff
// and jitter; after three failed attempts in a row, server-sent events on
// /broadcast/sse; if that fails too, "fallback": the page polls its
// catch-up endpoint and a WebSocket retry runs every 30 s. onReady runs
// whenever a transport is (re)established, so pages fetch what they missed
// by last message id: events sent while disconnected are never replayed.

export type BroadcastStatus = "connecting" | "live" | "reconnecting" | "fallback";

export interface Member {
	id: string;
	info?: { name?: string; role?: string };
}

export interface BroadcastHandlers {
	onEvent?: (channel: string, event: string, data: unknown) => void;
	onHere?: (channel: string, members: Member[]) => void;
	onJoining?: (channel: string, member: Member) => void;
	onLeaving?: (channel: string, member: Member) => void;
	onReady?: () => void;
}

interface Frame {
	type: string;
	channel?: string;
	event?: string;
	data?: unknown;
	status?: number;
}

const maxBackoff = 30_000;
const wsAttemptsBeforeSSE = 3;

function backoff(attempt: number): number {
	const base = Math.min(maxBackoff, 1000 * 2 ** attempt);
	return base / 2 + Math.random() * (base / 2);
}

export function useBroadcast(
	channels: string[],
	events: string[],
	handlers: BroadcastHandlers,
	enabled = true,
): BroadcastStatus {
	const [status, setStatus] = useState<BroadcastStatus>("connecting");
	const h = useRef(handlers);
	useEffect(() => {
		h.current = handlers;
	}, [handlers]);

	const key = channels.join(",");
	const eventKey = events.join(",");

	useEffect(() => {
		if (!enabled || key === "") return;
		const list = key.split(",");
		const eventList = eventKey.split(",");
		let stopped = false;
		let ws: WebSocket | null = null;
		let sse: EventSource | null = null;
		let timer: ReturnType<typeof setTimeout> | undefined;
		let failures = 0;

		const dispatch = (f: Frame) => {
			const ch = f.channel ?? "";
			switch (f.type) {
				case "event":
					h.current.onEvent?.(ch, f.event ?? "", f.data);
					break;
				case "here":
					h.current.onHere?.(ch, (f.data as Member[]) ?? []);
					break;
				case "joining":
					h.current.onJoining?.(ch, f.data as Member);
					break;
				case "leaving":
					h.current.onLeaving?.(ch, f.data as Member);
					break;
			}
		};

		const later = (fn: () => void, ms: number) => {
			clearTimeout(timer);
			timer = setTimeout(() => {
				if (!stopped) fn();
			}, ms);
		};

		const openSSE = () => {
			setStatus("reconnecting");
			const source = new EventSource(`/broadcast/sse?channels=${encodeURIComponent(list.join(","))}`);
			sse = source;
			let opened = false;
			const relay = (type: string) => (e: MessageEvent<string>) => {
				try {
					const m = JSON.parse(e.data) as { channel: string; event: string; data: unknown };
					dispatch({ type, channel: m.channel, event: m.event, data: m.data });
				} catch {
					// Not JSON: ignore rather than break the page.
				}
			};
			for (const name of eventList) source.addEventListener(name, relay("event"));
			source.addEventListener("presence:here", relay("here"));
			source.addEventListener("presence:joining", relay("joining"));
			source.addEventListener("presence:leaving", relay("leaving"));
			source.onopen = () => {
				opened = true;
				setStatus("live");
				h.current.onReady?.();
			};
			source.onerror = () => {
				// EventSource retries by itself once it has opened; a stream that
				// never opens (blocked, 403, too many connections) means polling.
				if (opened && source.readyState !== EventSource.CLOSED) {
					setStatus("reconnecting");
					return;
				}
				source.close();
				sse = null;
				setStatus("fallback");
				h.current.onReady?.();
				later(() => {
					failures = 0;
					openWS();
				}, maxBackoff);
			};
		};

		const openWS = () => {
			if (typeof WebSocket === "undefined") {
				openSSE();
				return;
			}
			const scheme = window.location.protocol === "https:" ? "wss" : "ws";
			const socket = new WebSocket(`${scheme}://${window.location.host}/broadcast/ws`);
			ws = socket;
			let pending = new Set(list);
			let ready = false;

			socket.onopen = () => {
				for (const channel of list) socket.send(JSON.stringify({ type: "subscribe", channel }));
			};
			socket.onmessage = (e: MessageEvent<string>) => {
				let f: Frame;
				try {
					f = JSON.parse(e.data) as Frame;
				} catch {
					return;
				}
				if (f.type === "subscribed" || (f.type === "error" && f.channel)) {
					pending.delete(f.channel ?? "");
					if (!ready && pending.size === 0) {
						ready = true;
						failures = 0;
						setStatus("live");
						h.current.onReady?.();
					}
					return;
				}
				dispatch(f);
			};
			socket.onclose = () => {
				ws = null;
				pending = new Set();
				if (stopped) return;
				if (!ready) failures++;
				if (failures >= wsAttemptsBeforeSSE && typeof EventSource !== "undefined") {
					openSSE();
					return;
				}
				setStatus("reconnecting");
				later(openWS, backoff(failures));
			};
		};

		openWS();

		const online = () => {
			if (!ws && !sse) {
				failures = 0;
				later(openWS, 0);
			}
		};
		window.addEventListener("online", online);

		return () => {
			stopped = true;
			clearTimeout(timer);
			window.removeEventListener("online", online);
			ws?.close();
			sse?.close();
		};
	}, [key, eventKey, enabled]);

	return status;
}

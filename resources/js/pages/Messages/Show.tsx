import { Field as BaseField } from "@base-ui/react/field";
import { Head, Link } from "@inertiajs/react";
import { type FormEvent, type JSX, type KeyboardEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";

import { button, link } from "../../components/ui";
import {
	EVENT_MESSAGE,
	EVENT_READ,
	EVENT_TYPING,
	MESSAGE_MAX,
	mergeMessages,
	messageSchema,
	postJSON,
} from "../../lib/chat";
import { useT } from "../../lib/i18n";
import { check } from "../../lib/qna";
import { type BroadcastHandlers, type BroadcastStatus, useBroadcast } from "../../lib/useBroadcast";
import type { ChatMessage, ConversationDetail } from "../../types";

interface Props {
	conversation: ConversationDetail;
	messages: ChatMessage[];
	me: number;
	online: boolean;
	channels: { chat: string; presence: string };
}

// How long a "typing" event shows, and how often this side sends one.
const typingShowMs = 4000;
const typingSendMs = 2500;
// Catch-up interval while no realtime transport works.
const pollMs = 5000;

interface Since {
	messages: ChatMessage[];
	their_last_read_id: number;
	online: boolean;
}

// Show is one conversation: live messages, typing and read receipts over
// the broadcast socket, catch-up by message id after every (re)connect,
// polling when no realtime transport works.
export default function Show({
	conversation,
	messages: initial,
	me,
	online: initialOnline,
	channels,
}: Props): JSX.Element {
	const { t, locale } = useT();
	const [messages, setMessages] = useState(initial);
	const [theirRead, setTheirRead] = useState(conversation.their_last_read_id);
	const [online, setOnline] = useState(initialOnline);
	const [typing, setTyping] = useState(false);
	const [body, setBody] = useState("");
	const [error, setError] = useState<string | undefined>();
	const [sending, setSending] = useState(false);
	const lastId = useRef(initial.at(-1)?.id ?? 0);
	const myRead = useRef(conversation.my_last_read_id);
	const typingTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
	const lastTypingSent = useRef(0);
	const bottom = useRef<HTMLDivElement>(null);
	const other = String(conversation.with_id);
	const base = `/messages/${conversation.id}`;

	const schema = useMemo(
		() =>
			messageSchema({
				empty: t("chat.errors.empty"),
				tooLong: t("chat.errors.too_long", { max: MESSAGE_MAX }),
				contact: t("chat.errors.no_contact"),
			}),
		[t],
	);

	const add = useCallback((incoming: ChatMessage[]) => {
		if (incoming.length === 0) return;
		setMessages((current) => mergeMessages(current, incoming));
		lastId.current = Math.max(lastId.current, ...incoming.map((m) => m.id));
	}, []);

	// markRead moves this side's receipt to the newest message while the tab
	// is visible.
	const markRead = useCallback(() => {
		if (document.visibilityState !== "visible" || lastId.current <= myRead.current) return;
		myRead.current = lastId.current;
		void postJSON(`${base}/read`, { last_id: lastId.current });
	}, [base]);

	const catchUp = useCallback(async () => {
		try {
			const res = await fetch(`${base}/since?after=${lastId.current}`, {
				headers: { Accept: "application/json" },
				credentials: "same-origin",
			});
			if (!res.ok) return;
			const data = (await res.json()) as Since;
			add(data.messages);
			setTheirRead((r) => Math.max(r, data.their_last_read_id));
			setOnline(data.online);
			markRead();
		} catch {
			// Offline: the next reconnect or poll tries again.
		}
	}, [add, base, markRead]);

	const handlers = useMemo<BroadcastHandlers>(
		() => ({
			onReady: () => void catchUp(),
			onEvent: (_channel, event, data) => {
				if (event === EVENT_MESSAGE) {
					const m = data as ChatMessage;
					add([m]);
					if (m.sender_id !== me) {
						setTyping(false);
						setOnline(true);
						markRead();
					}
				} else if (event === EVENT_READ) {
					const r = data as { user_id: number; last_read_id: number };
					if (r.user_id !== me) setTheirRead((cur) => Math.max(cur, r.last_read_id));
				} else if (event === EVENT_TYPING) {
					const ty = data as { user_id: number };
					if (ty.user_id === me) return;
					setTyping(true);
					setOnline(true);
					clearTimeout(typingTimer.current);
					typingTimer.current = setTimeout(() => setTyping(false), typingShowMs);
				}
			},
			onHere: (_channel, members) => {
				if (members.some((m) => m.id === other)) setOnline(true);
			},
			onJoining: (_channel, m) => {
				if (m.id === other) setOnline(true);
			},
			onLeaving: (_channel, m) => {
				if (m.id === other) {
					setOnline(false);
					setTyping(false);
				}
			},
		}),
		[add, catchUp, markRead, me, other],
	);

	const status = useBroadcast(
		[channels.chat, channels.presence],
		[EVENT_MESSAGE, EVENT_READ, EVENT_TYPING],
		handlers,
	);

	useEffect(() => {
		if (status !== "fallback") return;
		const id = setInterval(() => void catchUp(), pollMs);
		return () => clearInterval(id);
	}, [status, catchUp]);

	useEffect(() => {
		document.addEventListener("visibilitychange", markRead);
		return () => document.removeEventListener("visibilitychange", markRead);
	}, [markRead]);

	useEffect(() => () => clearTimeout(typingTimer.current), []);

	const count = messages.length;
	useEffect(() => {
		if (count > 0) bottom.current?.scrollIntoView({ block: "end" });
	}, [count]);

	const onInput = (value: string) => {
		setBody(value);
		if (error) setError(undefined);
		const now = Date.now();
		if (value.trim() !== "" && now - lastTypingSent.current > typingSendMs) {
			lastTypingSent.current = now;
			void postJSON(`${base}/typing`);
		}
	};

	const submit = async (e?: FormEvent) => {
		e?.preventDefault();
		if (sending) return;
		const problem = check(schema, body);
		if (problem) {
			setError(problem);
			return;
		}
		setSending(true);
		try {
			const res = await postJSON<{ message?: ChatMessage; errors?: Record<string, string[]> }>(base, { body });
			if (res.status === 201 && res.data?.message) {
				add([res.data.message]);
				myRead.current = Math.max(myRead.current, res.data.message.id);
				setBody("");
				lastTypingSent.current = 0;
			} else if (res.status === 422) {
				setError(res.data?.errors?.body?.[0] ?? t("chat.show.failed"));
			} else if (res.status === 429) {
				setError(t("chat.errors.rate_limited"));
			} else {
				setError(t("chat.show.failed"));
			}
		} catch {
			setError(t("chat.show.failed"));
		} finally {
			setSending(false);
		}
	};

	const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
		if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
			e.preventDefault();
			void submit();
		}
	};

	const time = new Intl.DateTimeFormat(locale, { dateStyle: "short", timeStyle: "short" });
	const lastMine = [...messages].reverse().find((m) => m.sender_id === me);
	const back = conversation.role === "seller" ? "/seller/messages" : "/messages";
	const title = t("chat.show.title", { name: conversation.with });

	return (
		<>
			<Head title={title} />
			<Link href={back} className={`${link} text-sm`}>
				{t("chat.show.back")}
			</Link>

			<section
				aria-labelledby="chat-title"
				className="mt-3 flex h-[min(42rem,calc(100dvh-12rem))] min-h-[24rem] flex-col overflow-hidden rounded-md bg-card shadow-card"
			>
				<header className="flex flex-wrap items-center gap-x-4 gap-y-1 border-b border-line px-4 py-3">
					<div className="min-w-0 flex-1">
						<h1 id="chat-title" className="truncate text-lg font-semibold text-ink">
							{title}
						</h1>
						<p className="truncate text-xs text-muted">
							{conversation.product ? (
								<Link href={`/p/${conversation.product.slug}`} className={link}>
									{t("chat.show.about", { product: conversation.product.title })}
								</Link>
							) : (
								<Link href={`/s/${conversation.shop.slug}`} className={link}>
									{t("chat.show.shop_link")}
								</Link>
							)}
						</p>
					</div>
					<p className="flex items-center gap-1.5 text-xs text-muted" aria-live="polite">
						<span
							aria-hidden="true"
							className={`size-2 rounded-full ${online ? "bg-success-600" : "bg-line"}`}
						/>
						{typing
							? t("chat.show.typing", { name: conversation.with })
							: online
								? t("chat.show.online")
								: t("chat.show.offline")}
					</p>
					<ConnectionStatus status={status} />
				</header>

				<div className="min-h-0 flex-1 overflow-y-auto bg-sunken px-3 py-4 sm:px-4">
					{messages.length === 0 ? (
						<p className="mx-auto max-w-sm py-10 text-center text-sm text-muted">{t("chat.show.empty")}</p>
					) : (
						// The log wraps the list: role="log" on the <ol> itself would drop
						// its list semantics and orphan the items.
						<div role="log" aria-label={t("chat.show.log_label", { name: conversation.with })}>
							<ol className="flex flex-col gap-2">
								{messages.map((m) => {
									const mine = m.sender_id === me;
									return (
										<li
											key={m.id}
											className={`flex flex-col ${mine ? "items-end" : "items-start"}`}
										>
											<div
												className={`max-w-[min(34rem,85%)] rounded-lg px-3 py-2 text-sm break-words whitespace-pre-wrap text-ink ${
													mine
														? "rounded-br-sm bg-action-soft"
														: "rounded-bl-sm border border-line bg-card"
												}`}
											>
												<span className="sr-only">
													{mine ? `${t("chat.show.you")}: ` : `${conversation.with}: `}
												</span>
												{m.body}
											</div>
											<span className="mt-0.5 px-1 text-[0.7rem] text-muted tabular-nums">
												<time dateTime={m.created_at}>
													{time.format(new Date(m.created_at))}
												</time>
												{lastMine?.id === m.id && (
													<>
														{" "}
														·{" "}
														{theirRead >= m.id ? t("chat.show.seen") : t("chat.show.sent")}
													</>
												)}
											</span>
										</li>
									);
								})}
							</ol>
						</div>
					)}
					<div ref={bottom} />
				</div>

				<form onSubmit={submit} noValidate className="border-t border-line p-3">
					<BaseField.Root invalid={Boolean(error)} className="flex flex-col gap-1">
						<BaseField.Label htmlFor="chat-body" className="sr-only">
							{t("chat.show.composer_label")}
						</BaseField.Label>
						<div className="flex items-end gap-2">
							<textarea
								id="chat-body"
								name="body"
								rows={2}
								value={body}
								maxLength={MESSAGE_MAX + 200}
								onChange={(e) => onInput(e.target.value)}
								onKeyDown={onKeyDown}
								placeholder={t("chat.show.composer_label")}
								aria-describedby={error ? "chat-error" : "chat-hint"}
								aria-invalid={Boolean(error)}
								className="min-h-11 flex-1 resize-y rounded-md border border-line bg-card px-3 py-2 text-sm text-ink placeholder:text-muted focus:border-action-500 focus:outline-none"
							/>
							<button type="submit" disabled={sending} className={button}>
								{sending ? t("chat.show.sending") : t("chat.show.send")}
							</button>
						</div>
						{error ? (
							<p id="chat-error" role="alert" className="text-sm text-danger-600">
								{error}
							</p>
						) : (
							<p id="chat-hint" className="flex justify-between gap-3 text-xs text-muted">
								<span>{t("chat.show.composer_hint", { max: MESSAGE_MAX })}</span>
								<span className="shrink-0 tabular-nums">
									{t("chat.show.chars", { count: Array.from(body).length, max: MESSAGE_MAX })}
								</span>
							</p>
						)}
					</BaseField.Root>
				</form>
			</section>
		</>
	);
}

function ConnectionStatus({ status }: { status: BroadcastStatus }): JSX.Element {
	const { t } = useT();
	const label = {
		connecting: t("chat.show.status_connecting"),
		live: t("chat.show.status_live"),
		reconnecting: t("chat.show.status_reconnecting"),
		fallback: t("chat.show.status_fallback"),
	}[status];

	return (
		<p
			data-testid="chat-status"
			data-status={status}
			className={`text-xs ${status === "live" ? "text-success-600" : status === "fallback" ? "text-warning-600" : "text-muted"}`}
		>
			{label}
		</p>
	);
}

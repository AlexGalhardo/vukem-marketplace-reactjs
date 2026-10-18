package chat_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/chat"
)

func TestSocketDeliversMessagesBetweenParticipants(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	id := e.start(t, buyer)
	srv := e.server(t)

	b, _ := e.dial(t, srv, buyer.ID, "")
	s, _ := e.dial(t, srv, e.owner.ID, "")

	b.expectSubscribed(chat.MessagesChannel(id))
	s.expectSubscribed(chat.MessagesChannel(id))
	s.expectSubscribed(chat.InboxChannel(e.owner.ID))

	msg := e.send(t, buyer, id, "Oi! Ainda tem em estoque?")

	for name, sock := range map[string]*socket{"seller": s, "buyer": b} {
		f := sock.next("event", chat.EventMessage)

		var got chat.MessageView
		if err := json.Unmarshal(f.Data, &got); err != nil {
			t.Fatal(err)
		}

		if got.ID != msg || got.Body != "Oi! Ainda tem em estoque?" || got.SenderID != buyer.ID {
			t.Fatalf("%s got %+v", name, got)
		}
	}

	inbox := s.next("event", chat.EventInbox)

	var ev chat.InboxEvent
	if err := json.Unmarshal(inbox.Data, &ev); err != nil {
		t.Fatal(err)
	}

	if ev.ConversationID != id || ev.Unread != 1 {
		t.Fatalf("inbox event %+v, want conversation %d with 1 unread", ev, id)
	}

	// Typing and read receipts reach the other side.
	e.json(t, e.owner).PostJSON(path(id, "/typing"), nil).AssertNoContent()

	if f := b.next("event", chat.EventTyping); !strings.Contains(string(f.Data), itoa(e.owner.ID)) {
		t.Fatalf("typing event %s", f.Data)
	}

	e.json(t, e.owner).PostJSON(path(id, "/read"), map[string]any{"last_id": msg}).AssertOK()

	var read chat.ReadEvent
	if err := json.Unmarshal(b.next("event", chat.EventRead).Data, &read); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(chat.ReadEvent{UserID: e.owner.ID, LastReadID: msg}, read); diff != "" {
		t.Fatalf("read event (-want +got):\n%s", diff)
	}
}

func TestPresenceChannelShowsTheOtherParticipant(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	id := e.start(t, buyer)
	srv := e.server(t)

	b, _ := e.dial(t, srv, buyer.ID, "")
	b.expectSubscribed(chat.PresenceChannel(id))

	var here []struct {
		ID   string          `json:"id"`
		Info chat.MemberInfo `json:"info"`
	}
	if err := json.Unmarshal(b.next("here", "").Data, &here); err != nil {
		t.Fatal(err)
	}

	if len(here) != 1 || here[0].ID != itoa(buyer.ID) || here[0].Info.Role != chat.RoleBuyer {
		t.Fatalf("here = %+v, want only the buyer", here)
	}

	s, _ := e.dial(t, srv, e.owner.ID, "")
	s.expectSubscribed(chat.PresenceChannel(id))

	joining := b.next("joining", "")
	if !strings.Contains(string(joining.Data), `"role":"seller"`) || !strings.Contains(string(joining.Data), "Olga") {
		t.Fatalf("joining = %s, want the seller", joining.Data)
	}

	_ = s.conn.CloseNow()

	if leaving := b.next("leaving", ""); !strings.Contains(string(leaving.Data), itoa(e.owner.ID)) {
		t.Fatalf("leaving = %s, want the seller", leaving.Data)
	}
}

// TestSocketRefusesOtherPeoplesChannels: channel authorization is the
// only thing between a socket and someone else's conversation.
func TestSocketRefusesOtherPeoplesChannels(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	mallory := e.user(t, "Mallory")
	id := e.start(t, buyer)
	srv := e.server(t)

	m, _ := e.dial(t, srv, mallory.ID, "")

	for _, tc := range []struct {
		channel string
		status  int
	}{
		{chat.MessagesChannel(id), http.StatusForbidden},
		{chat.PresenceChannel(id), http.StatusForbidden},
		{chat.InboxChannel(buyer.ID), http.StatusForbidden},
		{chat.InboxChannel(e.owner.ID), http.StatusForbidden},
		{"private-chat.0", http.StatusForbidden},
		{"private-chat.abc", http.StatusForbidden},
		{"private-chat." + itoa(id) + ".x", http.StatusForbidden},
		{"chat", http.StatusForbidden},
		{"private-chat.1,private-chat.2", http.StatusBadRequest},
		{"", http.StatusBadRequest},
	} {
		m.subscribe(tc.channel)

		if f := m.next("error", ""); f.Status != tc.status {
			t.Errorf("subscribe %q: status %d (%s), want %d", tc.channel, f.Status, f.Message, tc.status)
		}
	}

	// Nothing leaks to the refused socket: the buyer's message only
	// reaches subscribers.
	m.expectSubscribed(chat.InboxChannel(mallory.ID))
	e.send(t, buyer, id, "Mensagem privada")
	own := e.start(t, mallory)
	e.send(t, e.owner, own, "Resposta para a Mallory")

	if f := m.next("event", chat.EventInbox); !strings.Contains(string(f.Data), "Resposta para a Mallory") {
		t.Fatalf("mallory's first inbox event is %s, want her own conversation", f.Data)
	}
}

func TestSocketUpgradeChecks(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	srv := e.server(t)

	if _, status := e.dial(t, srv, 0, ""); status == http.StatusSwitchingProtocols {
		t.Fatal("anonymous upgrade accepted")
	}

	// Cross-site WebSocket hijacking: another origin with the victim's
	// cookies is refused before the upgrade.
	if _, status := e.dial(t, srv, buyer.ID, "https://evil.example"); status != http.StatusForbidden {
		t.Fatalf("cross-origin upgrade: status %d, want 403", status)
	}

	for i := range 5 {
		if _, status := e.dial(t, srv, buyer.ID, ""); status != http.StatusSwitchingProtocols {
			t.Fatalf("socket %d: status %d, want 101", i+1, status)
		}
	}

	if _, status := e.dial(t, srv, buyer.ID, ""); status != http.StatusTooManyRequests {
		t.Fatalf("6th socket: status %d, want 429", status)
	}

	// The cap is per user.
	if _, status := e.dial(t, srv, e.owner.ID, ""); status != http.StatusSwitchingProtocols {
		t.Fatalf("another user's socket: status %d, want 101", status)
	}
}

func TestSocketDropsOversizedFrames(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	srv := e.server(t)

	s, _ := e.dial(t, srv, buyer.ID, "")
	big := `{"type":"subscribe","channel":"` + strings.Repeat("a", 8<<10) + `"}`

	if err := s.conn.Write(t.Context(), 1, []byte(big)); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.conn.Read(t.Context()); err == nil {
		t.Fatal("oversized frame did not close the socket")
	}
}

func TestSSEFallbackAuthorizesChannels(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	mallory := e.user(t, "Mallory")
	id := e.start(t, buyer)

	e.json(t, mallory).Get("/broadcast/sse?channels=" + chat.MessagesChannel(id)).AssertForbidden()
	e.json(t, buyer).Get("/broadcast/sse?channels=" + chat.MessagesChannel(id) + "," + chat.InboxChannel(mallory.ID)).
		AssertForbidden()
}

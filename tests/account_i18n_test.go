package tests

import (
	"strings"
	"testing"
)

func TestAccountMessagesFollowTheLocale(t *testing.T) {
	t.Parallel()

	app, fake := queuedApp(t, nil)
	register(t, app.Client(t).Inertia(""), "tom@example.com")

	cases := []struct {
		name, page, path, component, field, want string
		body                                     map[string]any
	}{
		{
			name: "vukem guard message", page: "/login", path: "/login", component: "Auth/Login", field: "email",
			body: map[string]any{"email": "tom@example.com", "password": "wrong"},
			want: "Essas credenciais não correspondem aos nossos registros.",
		},
		{
			name: "built-in rule", page: "/register", path: "/register", component: "Auth/Register", field: "password",
			body: map[string]any{
				"name": "Tom", "email": "tom2@example.com", "password": "short", "password_confirmation": "short",
			},
			want: "O campo senha deve ter pelo menos 12 caracteres.",
		},
		{
			name: "refine message", page: "/register", path: "/register", component: "Auth/Register",
			field: "password_confirmation",
			body: map[string]any{
				"name":                  "Tom",
				"email":                 "tom3@example.com",
				"password":              "long enough password",
				"password_confirmation": "x",
			},
			want: "A confirmação da senha não confere.",
		},
		{
			name:      "account error",
			page:      "/reset-password/abc",
			path:      "/reset-password",
			component: "Auth/ResetPassword",
			field:     "email",
			body: map[string]any{
				"token":                 "abc",
				"email":                 "tom@example.com",
				"password":              newPassword,
				"password_confirmation": newPassword,
			},
			want: "Este link de redefinição de senha é inválido ou expirou.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := app.Client(t).Inertia("").WithHeader("Accept-Language", "pt-BR").WithHeader("Referer", tc.page)
			c.PostJSON(tc.path, tc.body).AssertRedirect(tc.page)
			c.Get(tc.page).AssertInertia(tc.component).PropEquals("errors."+tc.field, tc.want)
		})
	}

	// Flash messages and queued mails are translated too, even when a
	// worker renders the mail later.
	c := app.Client(t).Inertia("").WithHeader("Accept-Language", "pt-BR")
	c.PostJSON("/forgot-password", map[string]any{"email": "tom@example.com"}).AssertRedirect("/forgot-password")

	page := c.Get("/forgot-password").AssertInertia("Auth/ForgotPassword").Page()
	if flash, _ := page["flash"].(map[string]any); !strings.HasPrefix(flash["status"].(string), "Se esse e-mail") {
		t.Errorf("flash = %v", page["flash"])
	}

	runQueued(t, app, fake)

	sent := app.Mails().SentTo("tom@example.com")
	last := sent[len(sent)-1]

	if last.Subject != "Redefina sua senha" || !strings.Contains(last.HTML, "Redefinir senha") {
		t.Errorf("reset mail subject %q, html %q", last.Subject, last.HTML)
	}

	if first := sent[0]; first.Subject != "Verify your email address" {
		t.Errorf("verification mail (English request) subject = %q", first.Subject)
	}
}

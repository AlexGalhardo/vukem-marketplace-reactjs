package moderation_test

import (
	"testing"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/moderation"
)

func TestContainsContact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text string
		want bool
	}{
		{"Great drill, arrived in 3 days.", false},
		{"Paguei R$ 1.000 em 10 vezes, valeu cada centavo.", false},
		{"Comprei em 12/10/2026 e chegou rápido.", false},
		{"Bateria de 5000 mAh dura o dia todo.", false},
		{"A instalação foi simples. Funciona.Comprei outro.", false},
		{"", false},
		{"see https://example.com/deal", true},
		{"visit www.example.org", true},
		{"cheaper at loja.com.br", true},
		{"mail me at ana@example.com", true},
		{"ana (at) example (dot) com", true},
		{"call (11) 98765-4321", true},
		{"+55 11 9 8765 4321", true},
		{"chama no WhatsApp", true},
		{"me acha no zap", true},
		{"t.me/loja", true},
	}

	for _, tt := range tests {
		if got := moderation.ContainsContact(tt.text); got != tt.want {
			t.Errorf("ContainsContact(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

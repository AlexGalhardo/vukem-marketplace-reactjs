package moderation_test

import (
	"testing"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/moderation"
)

func TestContainsContact(t *testing.T) {
	t.Parallel()

	blocked := map[string]string{
		"http link":          "See http://example.org/x for details",
		"https link":         "Better price at https://outra-loja.example",
		"www":                "Look at www.minhaloja and tell me",
		"bare domain":        "compre em minhaloja.com.br mais barato",
		"shortener":          "veja bit.ly/abc123",
		"email":              "write to ana.souza+shop@mail.example.com",
		"email upper":        "ANA@EXAMPLE.ORG please",
		"handle":             "me chama no insta @loja_da_ana",
		"arroba":             "ana arroba gmail",
		"bracket at":         "ana [at] provider [dot] com",
		"ponto com":          "meu site é anaimports ponto com",
		"dot com":            "anaimports dot com",
		"pontocom":           "anaimports pontocom",
		"gmail":              "manda no meu gmail",
		"mobile with ddd":    "liga (11) 98765-4321",
		"mobile spaces":      "whats 11 98765 4321",
		"mobile dots":        "11.98765.4321",
		"mobile plus55":      "+55 11 98765-4321",
		"mobile no ddd":      "987654321",
		"split digits":       "9 8 7 6 5 4 3 2 1",
		"dashed digits":      "9-8-7-6-5-4-3-2-1",
		"landline":           "fixo 3333-4444",
		"landline with ddd":  "(21) 3333-4444",
		"toll free":          "0800 123 4567",
		"international":      "+1 (415) 555-0100",
		"digits in sentence": "my number is 21 99999 8888 call me",
	}
	for name, text := range blocked {
		t.Run("blocks "+name, func(t *testing.T) {
			t.Parallel()

			if !moderation.ContainsContact(text) {
				t.Fatalf("ContainsContact(%q) = false, want true", text)
			}
		})
	}

	allowed := map[string]string{
		"plain":          "Does it come with two batteries and a case?",
		"portuguese com": "Funciona com 220V? Ótimo produto. Com certeza vou comprar.",
		"price":          "O preço de R$ 1.299,90 inclui frete?",
		"cep":            "Entrega no CEP 01310-100 em quantos dias?",
		"cep no dash":    "Entrega no CEP 01310100?",
		"dimensions":     "Tem 120 x 60 x 75 cm ou 30.5 cm?",
		"model":          "Is it compatible with the RTX 4090 and 2 x 16 GB?",
		"decimal":        "Serve para 1.5 kg e 2.5 kg?",
		"email word":     "Do you answer by email?",
		"at sign alone":  "Is it 50% off @ checkout?",
		"review price":   "Paguei R$ 1.000 em 10 vezes, valeu cada centavo.",
		"review date":    "Comprei em 12/10/2026 e chegou rápido.",
		"review battery": "Bateria de 5000 mAh dura o dia todo.",
		"review install": "A instalação foi simples. Funciona.Comprei outro.",
	}
	for name, text := range allowed {
		t.Run("allows "+name, func(t *testing.T) {
			t.Parallel()

			if moderation.ContainsContact(text) {
				t.Fatalf("ContainsContact(%q) = true, want false", text)
			}
		})
	}
}

//nolint:misspell // pt-BR demo vocabulary, not English typos.
package seeders

// Demo activity vocabulary (pt-BR): buyer names and addresses, review and
// question texts. Every text passes moderation.ContainsContact (no links,
// emails, handles or digit runs) and leaves "furadeira" out (see
// demo_catalog_data.go). Generic on purpose: it fits any category.

var (
	buyerFirstNames = []string{
		"Adriana", "Alice", "André", "Antônio", "Beatriz", "Bernardo", "Bianca", "Caio", "Camila", "Carlos",
		"Cecília", "Clara", "Daniel", "Daniela", "Davi", "Eduardo", "Enzo", "Fábio", "Fernanda", "Flávia",
		"Francisco", "Giovana", "Guilherme", "Helena", "Heitor", "Igor", "Isadora", "Júlia", "Juliana", "Larissa",
		"Laura", "Leonardo", "Letícia", "Lorena", "Luana", "Luiz", "Manuela", "Marcelo", "Márcia", "Mariana",
		"Mateus", "Miguel", "Natália", "Otávio", "Patrícia", "Paulo", "Priscila", "Renata", "Ricardo", "Rodrigo",
		"Sabrina", "Sérgio", "Sofia", "Thiago", "Valentina", "Vanessa", "Vinícius", "Yasmin",
	}
	buyerLastNames = []string{
		"Araújo", "Azevedo", "Batista", "Cardoso", "Carvalho", "Castro", "Correia", "Cunha", "Fernandes",
		"Freitas", "Lopes", "Machado", "Mendes", "Monteiro", "Moraes", "Moreira", "Nascimento", "Pinto",
		"Ramos", "Rocha", "Rodrigues", "Sampaio", "Siqueira", "Tavares", "Vieira",
	}
)

// buyerCity is a delivery city: its UF, the first five CEP digits range
// (inside the Correios range of the UF, shipping.UFFromCEP) and districts.
type buyerCity struct {
	uf, name   string
	cepFrom    int
	cepTo      int
	districts  []string
	weightPick int // relative weight: big cities get more buyers
}

var buyerCities = []buyerCity{
	{"SP", "São Paulo", 1000, 5999, []string{"Pinheiros", "Moema", "Tatuapé", "Santana", "Vila Mariana"}, 9},
	{"SP", "Campinas", 13000, 13149, []string{"Cambuí", "Taquaral", "Barão Geraldo"}, 3},
	{"RJ", "Rio de Janeiro", 20000, 23799, []string{"Tijuca", "Botafogo", "Méier", "Barra da Tijuca"}, 6},
	{"MG", "Belo Horizonte", 30000, 31999, []string{"Savassi", "Pampulha", "Funcionários", "Buritis"}, 4},
	{"ES", "Vitória", 29000, 29099, []string{"Jardim da Penha", "Praia do Canto"}, 1},
	{"PR", "Curitiba", 80000, 82999, []string{"Batel", "Água Verde", "Portão"}, 3},
	{"SC", "Florianópolis", 88000, 88099, []string{"Trindade", "Coqueiros", "Centro"}, 2},
	{"RS", "Porto Alegre", 90000, 91999, []string{"Moinhos de Vento", "Menino Deus", "Petrópolis"}, 3},
	{"DF", "Brasília", 70000, 72799, []string{"Asa Norte", "Asa Sul", "Águas Claras"}, 2},
	{"GO", "Goiânia", 74000, 74899, []string{"Setor Bueno", "Setor Oeste", "Jardim América"}, 2},
	{"MT", "Cuiabá", 78000, 78109, []string{"Goiabeiras", "Centro Sul"}, 1},
	{"MS", "Campo Grande", 79000, 79129, []string{"Chácara Cachoeira", "Centro"}, 1},
	{"BA", "Salvador", 40000, 42599, []string{"Barra", "Pituba", "Rio Vermelho", "Itapuã"}, 4},
	{"PE", "Recife", 50000, 52999, []string{"Boa Viagem", "Casa Forte", "Graças"}, 3},
	{"CE", "Fortaleza", 60000, 61599, []string{"Aldeota", "Meireles", "Benfica"}, 3},
	{"PB", "João Pessoa", 58000, 58099, []string{"Manaíra", "Tambaú"}, 1},
	{"RN", "Natal", 59000, 59149, []string{"Ponta Negra", "Tirol"}, 1},
	{"MA", "São Luís", 65000, 65109, []string{"Renascença", "Calhau"}, 1},
	{"PA", "Belém", 66000, 66999, []string{"Nazaré", "Umarizal", "Marco"}, 2},
	{"AM", "Manaus", 69000, 69099, []string{"Adrianópolis", "Ponta Negra", "Flores"}, 2},
	{"TO", "Palmas", 77000, 77299, []string{"Plano Diretor Sul", "Taquaralto"}, 1},
}

var buyerStreets = []string{
	"Rua das Flores", "Avenida Brasil", "Rua São José", "Rua XV de Novembro", "Avenida Getúlio Vargas",
	"Rua Sete de Setembro", "Rua Tiradentes", "Avenida Santos Dumont", "Rua dos Andradas", "Rua Bela Vista",
	"Rua Dom Pedro II", "Avenida Paulista", "Rua Rui Barbosa", "Travessa das Palmeiras", "Rua da Paz",
}

var buyerComplements = []string{"", "", "", "Apto 12", "Apto 304", "Casa 2", "Bloco B", "Fundos", "Sala 5"}

// Carriers of the simulated shipments.
var demoCarriers = []string{"Correios", "Jadlog", "Loggi", "Total Express"}

// Review texts by rating (index 0 is one star).
var (
	reviewTitles = [5][]string{
		{"Não recomendo", "Veio com defeito", "Decepcionante", "Muito abaixo do esperado"},
		{"Deixou a desejar", "Qualidade fraca", "Esperava mais", "Só o básico"},
		{"Razoável", "Cumpre o que promete", "Bom, mas poderia ser melhor", "Custo-benefício ok"},
		{"Muito bom", "Gostei bastante", "Boa compra", "Recomendo", "Ótimo custo-benefício"},
		{"Excelente!", "Perfeito", "Superou as expectativas", "Compraria de novo", "Nota máxima"},
	}
	reviewBodies = [5][]string{
		{
			"Chegou com defeito e o acabamento é bem ruim. Não gostei.",
			"Parou de funcionar na primeira semana de uso.",
			"O produto não corresponde às fotos do anúncio.",
			"Material frágil, quebrou logo nos primeiros dias.",
		},
		{
			"Funciona, mas a qualidade é inferior ao que eu esperava pelo preço.",
			"A entrega atrasou e a embalagem chegou amassada.",
			"Serve para o básico, mas não vale o que paguei.",
			"O tamanho é menor do que parece nas fotos.",
		},
		{
			"Cumpre o prometido, nada além disso.",
			"Produto ok. O acabamento poderia ser melhor.",
			"Bom para o dia a dia, mas tem alguns detalhes que incomodam.",
			"Chegou dentro do prazo. Qualidade mediana.",
		},
		{
			"Produto de boa qualidade e chegou antes do prazo.",
			"Muito bom pelo preço, estou satisfeito com a compra.",
			"Gostei bastante, só achei a embalagem simples.",
			"Atendeu bem ao que eu precisava. O vendedor foi atencioso.",
			"Bem acabado e fácil de usar. Recomendo.",
		},
		{
			"Excelente! Exatamente como no anúncio e chegou super rápido.",
			"Qualidade impecável, superou minhas expectativas.",
			"Perfeito, já é a segunda vez que compro nesta loja.",
			"Muito bem embalado e funciona perfeitamente. Recomendo demais!",
			"Vale cada centavo. Comprei para presente e fez sucesso.",
			"Ótimo produto, entrega rápida e vendedor muito atencioso.",
		},
	}
	sellerRepliesPositive = []string{
		"Muito obrigado pela avaliação! Ficamos felizes que tenha gostado.",
		"Agradecemos a preferência e esperamos vê-lo novamente na loja!",
		"Obrigado pelo retorno! Bom uso do produto.",
	}
	sellerRepliesNegative = []string{
		"Sentimos muito pelo ocorrido. Abra uma solicitação pela plataforma que resolvemos a troca.",
		"Lamentamos a experiência. Já revisamos nosso processo de embalagem e envio.",
		"Obrigado pelo retorno sincero. Vamos analisar o caso e melhorar o produto.",
	}
)

// demoQA is a question and the answers a seller may give to it.
type demoQA struct {
	question string
	answers  []string
}

var demoQuestions = []demoQA{
	{"Olá, o produto é novo e tem nota fiscal?", []string{
		"Olá! Sim, o produto é novo, lacrado e enviamos com nota fiscal.",
		"Sim, todos os nossos produtos são novos e acompanham nota fiscal.",
	}},
	{"Qual é o prazo de garantia?", []string{
		"Oferecemos garantia de doze meses contra defeitos de fabricação.",
		"A garantia é de noventa dias pelo vendedor, mais a garantia do fabricante.",
	}},
	{"Vocês enviam no mesmo dia da compra?", []string{
		"Sim! Compras aprovadas até as catorze horas são enviadas no mesmo dia útil.",
		"Enviamos em até um dia útil após a confirmação do pagamento.",
	}},
	{"Tem disponível em outra cor?", []string{
		"No momento temos apenas as cores do anúncio. Novas cores chegam em breve!",
		"As cores disponíveis são as que aparecem nas opções do anúncio.",
	}},
	{"Serve como presente? Vem em caixa fechada?", []string{
		"Sim, vem na caixa original lacrada, ótimo para presente.",
		"Vem na embalagem original. Podemos incluir um bilhete se quiser!",
	}},
	{"Quais são as medidas exatas do produto?", []string{
		"As medidas estão na descrição do anúncio, logo abaixo das fotos.",
		"Todas as dimensões estão detalhadas na descrição. Qualquer dúvida, estamos aqui!",
	}},
	{"Aceita devolução se não servir?", []string{
		"Sim, você tem sete dias após o recebimento para devolver pela plataforma.",
		"Aceitamos devolução em até sete dias, seguindo a política da plataforma.",
	}},
	{"O frete para o Nordeste demora quanto tempo?", []string{
		"O prazo aparece no carrinho ao informar o seu CEP, normalmente uma semana.",
		"Para o Nordeste costuma levar de cinco a oito dias úteis.",
	}},
	{"Tem pronta entrega ou é sob encomenda?", []string{
		"Temos pronta entrega, o estoque do anúncio é real.",
		"Pronta entrega! Enviamos assim que o pagamento for aprovado.",
	}},
	{"Qual é o material do produto?", []string{
		"O material está descrito na ficha do anúncio. É de ótima qualidade!",
		"É feito com material resistente, conforme a descrição do anúncio.",
	}},
	{"Consigo comprar mais unidades com desconto?", []string{
		"Para várias unidades o carrinho já calcula um frete único. Aproveite!",
		"No momento o preço é o mesmo por unidade, mas o frete sai mais em conta.",
	}},
	{"Funciona em voltagem bivolt?", []string{
		"Depende do modelo; a voltagem está indicada no título e na descrição.",
		"Sim, este modelo é bivolt.",
	}},
	{"Boa tarde, ainda tem estoque?", []string{
		"Boa tarde! Sim, temos estoque disponível.",
		"Temos sim! Pode comprar que enviamos rapidinho.",
	}},
	{"Vocês emitem nota fiscal no CNPJ?", []string{
		"Sim, emitimos nota no CNPJ. Informe os dados após a compra pela plataforma.",
	}},
}

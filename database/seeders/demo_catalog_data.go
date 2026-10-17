//nolint:misspell // pt-BR demo vocabulary, not English typos.
package seeders

// Demo catalog vocabulary (pt-BR). Brands are invented; any resemblance to
// a real trademark is accidental. Prices are whole reais (the generator
// adds the cents), weights grams.
//
// "Furadeira" is left out on purpose: e2e/catalog.spec.ts searches for it
// and expects the DemoShop product on the first page.

// productKind is one kind of product of a category: its title noun, the
// option groups the title picks one value from (in order), and its price
// and weight ranges.
type productKind struct {
	noun               string
	model              bool // a "<brand> <model>" title, e.g. "Smartphone Vexa N12"
	specs              [][]string
	minReais           int
	maxReais           int
	minGrams, maxGrams int
	features           []string
}

type demoCategory struct {
	slug   string
	brands []string
	kinds  []productKind
}

var colors = []string{"Preto", "Branco", "Azul", "Cinza", "Vermelho", "Verde", "Rosa", "Grafite", "Bege", "Prata"}

var demoCategories = []demoCategory{
	{
		slug:   "electronics",
		brands: []string{"Sonora", "Voltix", "Audiara", "Lumio", "Kinetra"},
		kinds: []productKind{
			{
				"Fone de Ouvido Bluetooth",
				true,
				[][]string{{"com Cancelamento de Ruído", "Intra-auricular", "Over-ear", "Esportivo"}, colors},
				59,
				899,
				120,
				400,
				[]string{
					"bateria para até 30 horas",
					"estojo de carregamento USB-C",
					"microfone embutido para chamadas",
				},
			},
			{
				"Caixa de Som Portátil", true,
				[][]string{{"10 W", "20 W", "40 W", "60 W"}, {"à Prova d'Água", "com Luz RGB", "Bluetooth 5.3"}},
				89, 1299, 300, 2500,
				[]string{"proteção contra respingos", "pareamento estéreo entre duas caixas", "entrada auxiliar P2"},
			},
			{
				"Smart TV LED",
				true,
				[][]string{{"32\"", "43\"", "50\"", "55\"", "65\""}, {"Full HD", "4K UHD"}},
				999,
				4999,
				4500,
				22000,
				[]string{
					"Wi-Fi integrado e aplicativos de streaming",
					"três entradas HDMI",
					"controle remoto com comando de voz",
				},
			},
			{
				"Soundbar", true,
				[][]string{{"2.0 Canais", "2.1 Canais", "5.1 Canais"}, {"com Subwoofer", "Bluetooth"}},
				299, 2499, 1500, 6000,
				[]string{"conexão HDMI ARC", "modo noturno", "montagem em parede"},
			},
			{
				"Câmera de Segurança Wi-Fi",
				true,
				[][]string{{"Full HD", "2K", "360°"}, {"Visão Noturna", "Interna", "Externa"}},
				119,
				599,
				250,
				800,
				[]string{
					"detecção de movimento com alerta no celular",
					"áudio bidirecional",
					"gravação em cartão microSD",
				},
			},
			{
				"Projetor Portátil", true,
				[][]string{{"Full HD", "HD"}, {"200 Lumens ANSI", "350 Lumens ANSI"}},
				399, 2199, 900, 3000,
				[]string{"tela de até 120 polegadas", "correção trapezoidal", "alto-falante embutido"},
			},
		},
	},
	{
		slug:   "phones",
		brands: []string{"Vexa", "Novatel", "Orbi", "Kaito", "Zenix"},
		kinds: []productKind{
			{
				"Smartphone",
				true,
				[][]string{
					{"64 GB", "128 GB", "256 GB", "512 GB"},
					{"4 GB RAM", "6 GB RAM", "8 GB RAM", "12 GB RAM"},
					colors,
				},
				699,
				6999,
				160,
				240,
				[]string{
					"tela AMOLED de 6,5 polegadas",
					"câmera tripla de 50 MP",
					"bateria de 5000 mAh com carregamento rápido",
				},
			},
			{
				"Capinha Antiimpacto para Celular", false,
				[][]string{{"Transparente", "Fosca", "com Suporte"}, colors},
				19, 79, 30, 80,
				[]string{"bordas reforçadas", "botões com toque preciso", "compatível com carregamento sem fio"},
			},
			{
				"Película de Vidro 3D", false,
				[][]string{{"Privacidade", "Fosca", "Transparente"}},
				15, 49, 20, 60,
				[]string{"dureza 9H", "kit de aplicação incluso", "cobre a tela inteira"},
			},
			{
				"Carregador Turbo USB-C", true,
				[][]string{{"20 W", "33 W", "65 W"}, {"com Cabo de 1 m", "com Cabo de 2 m", "Duas Portas"}},
				39, 199, 80, 200,
				[]string{"proteção contra superaquecimento", "bivolt automático", "compatível com PD e QC"},
			},
			{
				"Smartwatch",
				true,
				[][]string{
					{"Tela 1,4\"", "Tela 1,8\"", "Tela 2,0\""},
					{"com GPS", "com Monitor Cardíaco", "Resistente à Água"},
					colors,
				},
				149,
				1499,
				40,
				90,
				[]string{
					"notificações do celular no pulso",
					"mais de 100 modos esportivos",
					"bateria para até 10 dias",
				},
			},
			{
				"Power Bank", true,
				[][]string{{"10000 mAh", "20000 mAh", "30000 mAh"}, {"Carregamento Rápido", "com Display Digital"}},
				69, 299, 200, 600,
				[]string{"duas saídas USB e uma USB-C", "indicador de carga", "corpo compacto"},
			},
		},
	},
	{
		slug:   "computers",
		brands: []string{"Datacore", "Pixelon", "Teknos", "Byteria", "Quadrix"},
		kinds: []productKind{
			{
				"Notebook",
				true,
				[][]string{
					{"14\"", "15,6\"", "16\""},
					{"8 GB RAM", "16 GB RAM", "32 GB RAM"},
					{"SSD 256 GB", "SSD 512 GB", "SSD 1 TB"},
				},
				1999,
				8999,
				1300,
				2600,
				[]string{"tela antirreflexo", "teclado retroiluminado", "Wi-Fi 6 e Bluetooth 5.2"},
			},
			{
				"Monitor Gamer", true,
				[][]string{{"24\"", "27\"", "32\""}, {"144 Hz", "165 Hz", "240 Hz"}, {"Full HD", "QHD"}},
				699, 3499, 3000, 7000,
				[]string{"tempo de resposta de 1 ms", "painel IPS", "ajuste de altura e inclinação"},
			},
			{
				"Teclado Mecânico",
				true,
				[][]string{
					{"ABNT2", "Compacto 60%", "TKL"},
					{"Switch Azul", "Switch Vermelho", "Switch Marrom"},
					{"RGB", "Branco"},
				},
				129,
				699,
				500,
				1200,
				[]string{"teclas com tecnologia anti-ghosting", "cabo USB-C removível", "software para macros"},
			},
			{
				"Mouse sem Fio", true,
				[][]string{{"Ergonômico", "Silencioso", "Gamer"}, {"1600 DPI", "4000 DPI", "16000 DPI"}, colors},
				39, 399, 70, 150,
				[]string{"receptor USB nano", "bateria recarregável", "sensor óptico de alta precisão"},
			},
			{
				"SSD NVMe", true,
				[][]string{{"500 GB", "1 TB", "2 TB"}, {"PCIe 3.0", "PCIe 4.0"}},
				199, 1299, 20, 60,
				[]string{"leitura de até 7000 MB/s", "garantia de 5 anos", "formato M.2 2280"},
			},
			{
				"Webcam", true,
				[][]string{{"HD 720p", "Full HD 1080p", "2K"}, {"com Microfone", "com Tampa de Privacidade"}},
				79, 499, 100, 300,
				[]string{"foco automático", "correção de luz baixa", "plug and play"},
			},
		},
	},
	{
		slug:   "home",
		brands: []string{"Lar Bello", "Casamia", "Aconchego", "Madeiro", "Nórdica"},
		kinds: []productKind{
			{
				"Jogo de Cama Casal", false,
				[][]string{{"200 Fios", "300 Fios", "400 Fios"}, {"Percal", "Algodão Egípcio", "Microfibra"}, colors},
				89, 499, 900, 2000,
				[]string{"4 peças com fronhas", "tecido que não desbota", "elástico em toda a volta"},
			},
			{
				"Luminária de Piso", false,
				[][]string{{"Tripé", "Arco", "Articulada"}, {"Madeira", "Metal Preto", "Dourada"}},
				149, 899, 2500, 7000,
				[]string{"bocal E27", "cúpula em tecido", "interruptor no cabo"},
			},
			{
				"Conjunto de Panelas Antiaderentes", false,
				[][]string{{"5 Peças", "7 Peças", "10 Peças"}, {"Cerâmica", "Teflon", "Inox"}},
				199, 1299, 3000, 9000,
				[]string{"cabos que não esquentam", "tampas de vidro temperado", "compatível com indução"},
			},
			{
				"Tapete para Sala", false,
				[][]string{{"1,5 x 2 m", "2 x 2,5 m", "2,5 x 3 m"}, {"Felpudo", "Geométrico", "Sisal"}},
				129, 999, 2000, 8000,
				[]string{"base antiderrapante", "fácil de limpar", "não solta pelos"},
			},
			{
				"Organizador de Armário", false,
				[][]string{{"Kit 4 Peças", "Kit 6 Peças", "Kit 10 Peças"}, {"Tecido", "Acrílico", "Plástico"}},
				39, 199, 300, 1500,
				[]string{"dobrável", "com tampa e alça", "ideal para guarda-roupa e prateleiras"},
			},
			{
				"Cadeira de Escritório", false,
				[][]string{{"Ergonômica", "Presidente", "Gamer"}, {"Tela Mesh", "Couro Sintético"}, colors},
				349, 1999, 9000, 18000,
				[]string{"regulagem de altura a gás", "apoio lombar", "rodízios de nylon"},
			},
		},
	},
	{
		slug:   "appliances",
		brands: []string{"Eletrolar", "Fornax", "Brisa", "Termika", "Doméstica"},
		kinds: []productKind{
			{
				"Air Fryer", true,
				[][]string{{"3,5 L", "4,5 L", "5,5 L", "12 L"}, {"Digital", "Analógica"}, {"127 V", "220 V"}},
				249, 899, 3500, 7000,
				[]string{"cesto antiaderente removível", "timer de até 60 minutos", "cozinha sem óleo"},
			},
			{
				"Liquidificador", true,
				[][]string{{"900 W", "1200 W", "1400 W"}, {"Copo de Vidro", "Copo Acrílico"}, {"127 V", "220 V"}},
				99, 499, 2000, 4000,
				[]string{"lâminas de aço inox", "12 velocidades e pulsar", "copo de 3 litros"},
			},
			{
				"Aspirador de Pó Vertical", true,
				[][]string{{"Sem Fio", "com Fio"}, {"1000 W", "1250 W"}, {"127 V", "220 V", "Bivolt"}},
				199, 1499, 2000, 4500,
				[]string{"filtro HEPA lavável", "bocal para estofados", "reservatório de 1 litro"},
			},
			{
				"Micro-ondas", true,
				[][]string{{"20 L", "27 L", "32 L"}, {"Espelhado", "Branco", "Inox"}, {"127 V", "220 V"}},
				499, 1299, 10000, 16000,
				[]string{"função descongelar", "trava de segurança", "receitas pré-programadas"},
			},
			{
				"Ventilador de Coluna", true,
				[][]string{{"40 cm", "50 cm"}, {"6 Pás", "8 Pás"}, {"127 V", "220 V"}},
				149, 449, 4000, 7000,
				[]string{"três velocidades", "oscilação automática", "altura ajustável"},
			},
			{
				"Cafeteira Elétrica", true,
				[][]string{{"15 Xícaras", "30 Xícaras"}, {"Programável", "com Jarra Térmica"}, {"127 V", "220 V"}},
				89, 599, 1200, 3000,
				[]string{"filtro permanente", "sistema corta-pingos", "placa de aquecimento"},
			},
		},
	},
	{
		slug:   "fashion",
		brands: []string{"Urbana", "Trama", "Piquê", "Rota Sul", "Algodoaria"},
		kinds: []productKind{
			{
				"Camiseta Básica", false,
				[][]string{{"Algodão", "Dry Fit", "Oversized"}, {"P", "M", "G", "GG"}, colors},
				29, 119, 150, 250,
				[]string{"malha macia que não amassa", "costura reforçada", "modelagem unissex"},
			},
			{
				"Calça Jeans", false,
				[][]string{{"Skinny", "Reta", "Wide Leg", "Cargo"}, {"36", "38", "40", "42", "44"}},
				89, 299, 500, 800,
				[]string{"tecido com elastano", "lavagem média", "cinco bolsos"},
			},
			{
				"Tênis Casual", false,
				[][]string{{"Couro Sintético", "Lona", "Knit"}, {"37", "38", "39", "40", "41", "42"}, colors},
				99, 499, 700, 1200,
				[]string{"palmilha macia", "solado de borracha", "cadarço de algodão"},
			},
			{
				"Mochila Executiva", false,
				[][]string{{"20 L", "28 L", "35 L"}, {"Impermeável", "com Entrada USB", "Antifurto"}, colors},
				79, 399, 600, 1300,
				[]string{"compartimento acolchoado para notebook", "alças ergonômicas", "bolso oculto nas costas"},
			},
			{
				"Jaqueta Corta-Vento", false,
				[][]string{{"Masculina", "Feminina"}, {"P", "M", "G", "GG"}, colors},
				119, 399, 300, 700,
				[]string{"capuz embutido na gola", "tecido leve e respirável", "bolsos com zíper"},
			},
			{
				"Bolsa Transversal", false,
				[][]string{{"Pequena", "Média"}, {"Couro Sintético", "Nylon"}, colors},
				59, 299, 300, 800,
				[]string{"alça regulável", "fechamento com zíper", "bolso interno"},
			},
		},
	},
	{
		slug:   "sports",
		brands: []string{"Atlética", "Kanto", "Maré Alta", "Impulso", "Vértice"},
		kinds: []productKind{
			{
				"Tênis de Corrida", false,
				[][]string{{"Amortecido", "Leve", "Trail"}, {"38", "39", "40", "41", "42", "43"}, colors},
				199, 899, 500, 900,
				[]string{"entressola com retorno de energia", "cabedal respirável", "solado com tração"},
			},
			{
				"Bicicleta Aro 29",
				false,
				[][]string{
					{"21 Marchas", "24 Marchas", "27 Marchas"},
					{"Quadro 17", "Quadro 19"},
					{"Freio a Disco", "Freio Hidráulico"},
				},
				1299,
				4999,
				13000,
				17000,
				[]string{"quadro em alumínio", "suspensão dianteira", "câmbio indexado"},
			},
			{
				"Kit Halteres Ajustáveis", false,
				[][]string{{"10 kg", "20 kg", "30 kg"}, {"Emborrachado", "Ferro Fundido"}},
				149, 899, 10000, 30000,
				[]string{"anilhas intercambiáveis", "barras com empunhadura antiderrapante", "presilhas de segurança"},
			},
			{
				"Tapete de Yoga", false,
				[][]string{{"4 mm", "6 mm", "10 mm"}, {"TPE", "PVC", "Cortiça"}, colors},
				49, 249, 800, 2000,
				[]string{"superfície antiderrapante", "alça para transporte", "fácil de higienizar"},
			},
			{
				"Garrafa Térmica Esportiva", false,
				[][]string{{"500 ml", "750 ml", "1 L"}, {"Inox", "com Canudo"}, colors},
				39, 179, 300, 600,
				[]string{"mantém gelado por 24 horas", "livre de BPA", "tampa à prova de vazamento"},
			},
			{
				"Bola de Futebol de Campo", false,
				[][]string{{"Tamanho 5", "Tamanho 4"}, {"Costurada", "Termossoldada"}},
				59, 249, 400, 500,
				[]string{"câmara de butil", "ótima retenção de ar", "uso em grama natural"},
			},
		},
	},
	{
		slug:   "beauty",
		brands: []string{"Botânica", "Pele Viva", "Aurora", "Essenza", "Flor do Cerrado"},
		kinds: []productKind{
			{
				"Sérum Facial", false,
				[][]string{{"Vitamina C", "Ácido Hialurônico", "Niacinamida", "Retinol"}, {"30 ml", "50 ml"}},
				39, 199, 80, 200,
				[]string{"textura leve de rápida absorção", "dermatologicamente testado", "vegano e cruelty-free"},
			},
			{
				"Protetor Solar",
				false,
				[][]string{
					{"FPS 30", "FPS 50", "FPS 70"},
					{"Toque Seco", "com Cor", "Infantil"},
					{"50 g", "120 ml", "200 ml"},
				},
				29,
				149,
				80,
				300,
				[]string{"proteção UVA e UVB", "resistente à água", "não oleoso"},
			},
			{
				"Secador de Cabelo", true,
				[][]string{{"1900 W", "2100 W", "2400 W"}, {"Íons", "Cerâmica"}, {"127 V", "220 V"}},
				129, 699, 500, 900,
				[]string{"duas velocidades e três temperaturas", "jato de ar frio", "bico concentrador"},
			},
			{
				"Kit Shampoo e Condicionador",
				false,
				[][]string{
					{"Cabelos Cacheados", "Cabelos Lisos", "Antiqueda", "Hidratação"},
					{"300 ml", "500 ml", "1 L"},
				},
				39,
				189,
				600,
				2200,
				[]string{"sem sulfatos e parabenos", "fragrância suave", "uso diário"},
			},
			{
				"Perfume",
				false,
				[][]string{
					{"Floral", "Amadeirado", "Cítrico", "Oriental"},
					{"Eau de Parfum", "Eau de Toilette"},
					{"50 ml", "100 ml"},
				},
				89,
				599,
				200,
				450,
				[]string{"fixação prolongada", "frasco de vidro", "notas de saída marcantes"},
			},
			{
				"Paleta de Sombras", false,
				[][]string{{"12 Cores", "18 Cores", "35 Cores"}, {"Nude", "Colorida", "Matte e Cintilante"}},
				29, 179, 100, 300,
				[]string{"alta pigmentação", "espelho embutido", "fácil de esfumar"},
			},
		},
	},
	{
		slug:   "toys",
		brands: []string{"Brincalhão", "Pequenos Gênios", "Toca", "Pipa", "Imagina"},
		kinds: []productKind{
			{
				"Blocos de Montar", false,
				[][]string{{"300 Peças", "500 Peças", "1000 Peças"}, {"Cidade", "Espaço", "Castelo", "Fazenda"}},
				59, 499, 500, 2500,
				[]string{"peças compatíveis entre kits", "manual ilustrado", "estimula a criatividade"},
			},
			{
				"Boneca Articulada", false,
				[][]string{{"30 cm", "45 cm"}, {"com Acessórios", "com Roupinhas Extras"}},
				49, 299, 300, 900,
				[]string{"cabelo para pentear", "membros articulados", "material atóxico"},
			},
			{
				"Carrinho de Controle Remoto", true,
				[][]string{{"1:16", "1:20", "1:24"}, {"4x4", "Drift", "Anfíbio"}, colors},
				99, 699, 600, 2000,
				[]string{"bateria recarregável", "alcance de até 50 metros", "pneus de borracha"},
			},
			{
				"Quebra-Cabeça", false,
				[][]string{{"500 Peças", "1000 Peças", "2000 Peças"}, {"Paisagem", "Mapa do Brasil", "Obras de Arte"}},
				39, 199, 400, 1500,
				[]string{"peças com encaixe perfeito", "papelão de alta gramatura", "pôster de referência"},
			},
			{
				"Jogo de Tabuleiro", false,
				[][]string{{"Estratégia", "Cooperativo", "de Perguntas"}, {"2 a 4 Jogadores", "2 a 6 Jogadores"}},
				59, 349, 500, 2000,
				[]string{"partidas de 30 a 60 minutos", "regras em português", "componentes de madeira"},
			},
			{
				"Pelúcia", false,
				[][]string{{"Urso", "Coelho", "Dinossauro", "Unicórnio"}, {"25 cm", "40 cm", "60 cm"}},
				39, 249, 200, 900,
				[]string{"enchimento antialérgico", "costura reforçada", "lavável à mão"},
			},
		},
	},
	{
		slug:   "tools",
		brands: []string{"Ferrum", "Obra Forte", "Torke", "Mestre", "Pinhão"},
		kinds: []productKind{
			{
				"Parafusadeira a Bateria", true,
				[][]string{{"12 V", "18 V", "20 V"}, {"com 2 Baterias", "com Maleta", "com Kit de Bits"}},
				149, 899, 900, 2200,
				[]string{"torque ajustável em 20 posições", "luz LED de trabalho", "mandril de aperto rápido"},
			},
			{
				"Jogo de Chaves Combinadas", false,
				[][]string{{"8 Peças", "12 Peças", "22 Peças"}, {"Cromo Vanádio", "Aço Carbono"}},
				59, 399, 800, 4000,
				[]string{"acabamento cromado", "estojo organizador", "medidas de 6 a 22 mm"},
			},
			{
				"Serra Tico-Tico", true,
				[][]string{{"500 W", "650 W", "800 W"}, {"127 V", "220 V"}},
				179, 699, 1800, 3000,
				[]string{"corte em ângulo de até 45°", "velocidade variável", "soprador de serragem"},
			},
			{
				"Esmerilhadeira Angular", true,
				[][]string{{"4 1/2\"", "7\""}, {"720 W", "900 W", "2200 W"}, {"127 V", "220 V"}},
				159, 799, 1800, 5500,
				[]string{"empunhadura lateral", "trava do eixo", "protetor de disco ajustável"},
			},
			{
				"Trena a Laser", true,
				[][]string{{"40 m", "60 m", "100 m"}, {"com Nível", "com Bluetooth"}},
				99, 599, 100, 300,
				[]string{"precisão de 2 mm", "cálculo de área e volume", "tela iluminada"},
			},
			{
				"Caixa de Ferramentas", false,
				[][]string{{"com 129 Peças", "com 200 Peças", "Vazia"}, {"Plástica", "Metálica"}},
				59, 599, 1500, 9000,
				[]string{"bandeja removível", "travas reforçadas", "alça ergonômica"},
			},
		},
	},
	{
		slug:   "books",
		brands: []string{"Editora Ipê", "Sabiá Livros", "Pena & Prosa", "Editora Farol", "Folha Seca"},
		kinds: []productKind{
			{
				"Livro: Programação em Go",
				false,
				[][]string{
					{"do Zero ao Avançado", "Guia Prático", "Concorrência na Prática"},
					{"Capa Comum", "Capa Dura"},
				},
				59,
				189,
				400,
				900,
				[]string{"exemplos completos e exercícios", "código-fonte para download", "edição revisada"},
			},
			{
				"Romance",
				false,
				[][]string{
					{"O Último Verão em Olinda", "Ventos do Sul", "A Casa da Ladeira", "Cartas de Ouro Preto"},
					{"Capa Comum", "Edição de Bolso"},
				},
				29,
				89,
				250,
				600,
				[]string{"best-seller nacional", "narrativa envolvente", "autora premiada"},
			},
			{
				"Livro de Receitas",
				false,
				[][]string{
					{"Cozinha Brasileira", "Doces de Vó", "Pratos Veganos", "Churrasco"},
					{"Capa Dura", "Espiral"},
				},
				49,
				159,
				600,
				1400,
				[]string{"mais de 150 receitas", "fotos de cada prato", "dicas de preparo passo a passo"},
			},
			{
				"Box de Histórias em Quadrinhos",
				false,
				[][]string{
					{"Volumes 1 a 3", "Volumes 1 a 5", "Edição Completa"},
					{"Aventura", "Ficção Científica", "Fantasia"},
				},
				89,
				399,
				900,
				3000,
				[]string{"papel couchê", "capas exclusivas", "extras com esboços do autor"},
			},
			{
				"Livro Infantil",
				false,
				[][]string{
					{"A Tartaruga Curiosa", "O Dragão que Tinha Medo", "Viagem ao Fundo do Mar"},
					{"Capa Dura", "Pop-Up"},
				},
				29,
				99,
				200,
				600,
				[]string{"ilustrações coloridas", "indicado de 3 a 7 anos", "páginas grossas"},
			},
			{
				"Guia de Estudos para Concursos", false,
				[][]string{{"Português", "Matemática", "Direito Constitucional", "Raciocínio Lógico"}, {"Edição 2026"}},
				49, 179, 600, 1500,
				[]string{"questões comentadas", "resumos por tópico", "simulados ao final"},
			},
		},
	},
	{
		slug:   "car-parts",
		brands: []string{"Autobras", "Rodovia", "Motrix", "Garagem 21", "Pista"},
		kinds: []productKind{
			{
				"Suporte Veicular para Celular",
				false,
				[][]string{
					{"Magnético", "de Ventosa", "para Saída de Ar"},
					{"com Carregador sem Fio", "Giratório 360°"},
				},
				29,
				199,
				100,
				400,
				[]string{
					"fixação firme mesmo em estrada de terra",
					"instalação sem ferramentas",
					"compatível com celulares de até 7 polegadas",
				},
			},
			{
				"Kit Palhetas Limpador de Para-brisa", false,
				[][]string{{"Par 16\" e 20\"", "Par 18\" e 22\"", "Par 20\" e 24\""}, {"Silicone", "Grafite"}},
				39, 149, 300, 700,
				[]string{"limpeza sem ruídos", "encaixe universal", "borracha resistente ao sol"},
			},
			{
				"Central Multimídia", true,
				[][]string{{"7\"", "9\"", "10\""}, {"Android", "com Espelhamento"}, {"2 DIN", "1 DIN"}},
				399, 1899, 1000, 2500,
				[]string{"GPS e Bluetooth", "entrada para câmera de ré", "rádio FM"},
			},
			{
				"Compressor de Ar Portátil", true,
				[][]string{{"12 V", "Sem Fio"}, {"Digital", "Analógico"}},
				99, 399, 900, 2000,
				[]string{"desligamento automático na pressão definida", "lanterna LED", "bicos para bolas e boias"},
			},
			{
				"Capa para Banco Automotivo", false,
				[][]string{{"Couro Sintético", "Tecido", "Neoprene"}, {"Jogo Completo", "Dianteiros"}, colors},
				99, 599, 1500, 4000,
				[]string{"compatível com airbag lateral", "fácil de instalar", "lavável"},
			},
			{
				"Aspirador Automotivo", true,
				[][]string{{"12 V", "Sem Fio"}, {"100 W", "120 W"}},
				59, 299, 600, 1500,
				[]string{"filtro lavável", "bocais para cantos", "cabo de 4 metros"},
			},
		},
	},
}

// Shop name parts: "<prefix> <surname>" or "<surname> <suffix>", with the
// shop's main category flavour.
var (
	shopPrefixes = []string{
		"Empório",
		"Casa",
		"Mundo",
		"Loja",
		"Cantinho",
		"Ponto",
		"Bazar",
		"Armazém",
		"Espaço",
		"Galeria",
	}
	shopSuffixes = map[string][]string{
		"electronics": {"Eletrônicos", "Áudio e Vídeo", "Tech"},
		"phones":      {"Celulares", "Acessórios Mobile", "Cell"},
		"computers":   {"Informática", "Computadores", "Hardware"},
		"home":        {"Casa e Decoração", "Utilidades", "Decor"},
		"appliances":  {"Eletrodomésticos", "Eletro", "Cozinha"},
		"fashion":     {"Moda", "Modas", "Vestuário"},
		"sports":      {"Esportes", "Fitness", "Aventura"},
		"beauty":      {"Beleza", "Cosméticos", "Perfumaria"},
		"toys":        {"Brinquedos", "Kids", "Diversão"},
		"tools":       {"Ferramentas", "Construção", "Ferragens"},
		"books":       {"Livros", "Livraria", "Leitura"},
		"car-parts":   {"Autopeças", "Acessórios Automotivos", "Car"},
	}
	shopSlogans = []string{
		"Atendimento rápido e envio no mesmo dia útil.",
		"Produtos selecionados com garantia e nota fiscal.",
		"Mais de dez anos vendendo com qualidade.",
		"Preço justo e embalagem caprichada.",
		"Loja familiar com atendimento de verdade.",
		"Enviamos para todo o Brasil.",
	}
	productClosings = []string{
		"Produto novo, em embalagem original lacrada.",
		"Acompanha nota fiscal e garantia de 90 dias do vendedor.",
		"Enviamos em até 24 horas úteis após a confirmação do pagamento.",
		"Tire suas dúvidas pelas perguntas antes de comprar.",
		"Garantia de 12 meses contra defeitos de fabricação.",
	}
	modelSeries = []string{"", "Pro", "Max", "Lite", "Plus", "Air", "Neo", "S", "X", "Ultra"}
)

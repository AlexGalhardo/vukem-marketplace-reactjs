package seeders

// Demo activity vocabulary: buyer names and addresses, review and question
// texts. Every text passes moderation.ContainsContact (no links, emails,
// handles or digit runs) and leaves "drill" and "impact" out (see
// demo_catalog_data.go). Generic on purpose: it fits any category.

var (
	buyerFirstNames = []string{
		"Amanda", "Alice", "Andrew", "Anthony", "Bella", "Benjamin", "Brianna", "Caleb", "Chloe", "Carl",
		"Cecilia", "Clara", "Daniel", "Daniela", "David", "Edward", "Ethan", "Fabian", "Fiona", "Flora",
		"Francis", "Grace", "Gabriel", "Helen", "Henry", "Ivan", "Isabel", "Julia", "Julian", "Lily",
		"Laura", "Leonard", "Leah", "Lauren", "Luna", "Louis", "Mandy", "Marcus", "Martha", "Maria",
		"Matthew", "Michael", "Natalie", "Oliver", "Patricia", "Paul", "Priscilla", "Rebecca", "Richard", "Robert",
		"Sabrina", "Samuel", "Sophia", "Thomas", "Valerie", "Vanessa", "Victor", "Hannah",
	}
	buyerLastNames = []string{
		"Anderson", "Adams", "Bailey", "Baker", "Campbell", "Carter", "Coleman", "Collins", "Cooper",
		"Davis", "Evans", "Foster", "Gray", "Hayes", "Hughes", "Johnson", "Kelly", "Lewis",
		"Morgan", "Murphy", "Parker", "Reed", "Russell", "Taylor", "Wright",
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
	"Maple Street", "Oak Avenue", "Main Street", "Second Street", "Park Avenue",
	"Cedar Road", "Elm Street", "Pine Street", "Lake Drive", "Washington Street",
	"Church Street", "Hill Road", "Willow Lane", "Spring Street", "River Road",
}

var buyerComplements = []string{"", "", "", "Apt 12", "Apt 304", "House 2", "Building B", "Rear", "Suite 5"}

// Carriers of the simulated shipments.
var demoCarriers = []string{"Correios", "Jadlog", "Loggi", "Total Express"}

// Review texts by rating (index 0 is one star).
var (
	reviewTitles = [5][]string{
		{"Not recommended", "Arrived defective", "Disappointing", "Well below expectations"},
		{"Left something to be desired", "Poor quality", "Expected more", "Basic at best"},
		{"Fair", "Does what it promises", "Good, but could be better", "Decent value"},
		{"Very good", "Really liked it", "Good purchase", "Recommended", "Great value"},
		{"Excellent!", "Perfect", "Exceeded expectations", "Would buy again", "Top marks"},
	}
	reviewBodies = [5][]string{
		{
			"Arrived defective and the finish is quite poor. I did not like it.",
			"Stopped working in the first week of use.",
			"The product does not match the photos in the listing.",
			"Fragile material, it broke within the first few days.",
		},
		{
			"It works, but the quality is below what I expected for the price.",
			"Delivery was late and the packaging arrived crushed.",
			"Good for basics, but not worth what I paid.",
			"It is smaller than it looks in the photos.",
		},
		{
			"Does what it promises, nothing more.",
			"Product is okay. The finish could be better.",
			"Good for everyday use, but a few details are annoying.",
			"Arrived on time. Average quality.",
		},
		{
			"Good quality product and it arrived before the deadline.",
			"Very good for the price, I am happy with the purchase.",
			"I really liked it, I just found the packaging plain.",
			"It met what I needed. The seller was attentive.",
			"Well made and easy to use. Recommended.",
		},
		{
			"Excellent! Exactly as in the listing and it arrived super fast.",
			"Flawless quality, exceeded my expectations.",
			"Perfect, this is already my second purchase from this store.",
			"Very well packed and works perfectly. Highly recommended!",
			"Worth every cent. I bought it as a gift and it was a hit.",
			"Great product, fast delivery and a very attentive seller.",
		},
	}
	sellerRepliesPositive = []string{
		"Thank you very much for the review! We are glad you liked it.",
		"Thank you for your preference, we hope to see you again in the store!",
		"Thanks for the feedback! Enjoy the product.",
	}
	sellerRepliesNegative = []string{
		"We are very sorry about what happened. Open a request through the platform and we will sort out the exchange.",
		"We are sorry for the experience. We have already reviewed our packaging and shipping process.",
		"Thank you for your honest feedback. We will look into the case and improve the product.",
	}
)

// demoQA is a question and the answers a seller may give to it.
type demoQA struct {
	question string
	answers  []string
}

var demoQuestions = []demoQA{
	{"Hi, is the product new and does it come with an invoice?", []string{
		"Hi! Yes, the product is new, sealed, and shipped with an invoice.",
		"Yes, all our products are new and come with an invoice.",
	}},
	{"What is the warranty period?", []string{
		"We offer a twelve-month warranty against manufacturing defects.",
		"The warranty is ninety days from the seller, plus the manufacturer's warranty.",
	}},
	{"Do you ship on the same day as the purchase?", []string{
		"Yes! Orders approved by two in the afternoon ship the same business day.",
		"We ship within one business day after payment confirmation.",
	}},
	{"Is it available in another color?", []string{
		"We currently only have the colors shown in the listing. New colors are coming soon!",
		"The available colors are the ones shown in the listing options.",
	}},
	{"Is it a good gift? Does it come in a sealed box?", []string{
		"Yes, it comes in the sealed original box, great for gifts.",
		"It comes in the original packaging. We can include a card if you want!",
	}},
	{"What are the exact dimensions of the product?", []string{
		"The measurements are in the listing description, just below the photos.",
		"All dimensions are detailed in the description. Any questions, we are here!",
	}},
	{"Do you accept returns if it does not fit?", []string{
		"Yes, you have seven days after delivery to return it through the platform.",
		"We accept returns within seven days, following the platform policy.",
	}},
	{"How long does shipping to the Northeast take?", []string{
		"The estimate appears in the cart when you enter your postal code, usually one week.",
		"For the Northeast it usually takes five to eight business days.",
	}},
	{"Is it ready to ship or made to order?", []string{
		"We have it ready to ship, the listed stock is real.",
		"Ready to ship! We send it as soon as payment is approved.",
	}},
	{"What is the product made of?", []string{
		"The material is described in the listing details. It is of great quality!",
		"It is made of durable material, as described in the listing.",
	}},
	{"Can I buy more units at a discount?", []string{
		"For several units the cart already calculates a single shipping fee. Go for it!",
		"At the moment the price is the same per unit, but shipping works out cheaper.",
	}},
	{"Does it work on dual voltage?", []string{
		"It depends on the model; the voltage is shown in the title and description.",
		"Yes, this model is dual voltage.",
	}},
	{"Good afternoon, is there still stock?", []string{
		"Good afternoon! Yes, we have stock available.",
		"We do! You can buy now and we will ship quickly.",
	}},
	{"Do you issue invoices for businesses (CNPJ)?", []string{
		"Yes, we issue invoices under the company tax ID. Send us the details through the platform after the purchase.",
	}},
}

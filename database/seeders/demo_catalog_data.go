package seeders

// workaround(vukem): factory.Faker has no locale or product/company/text data
// (only lorem ipsum), so the demo vocabulary lives here.
//
// Demo catalog vocabulary (English). Brands are invented; any resemblance to
// a real trademark is accidental. Prices are whole reais (the generator
// adds the cents), weights grams.
//
// "Drill" and "Impact" are left out on purpose: e2e/catalog.spec.ts searches
// for "Impact Drill" and expects the DemoShop product on the first page.

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

var colors = []string{"Black", "White", "Blue", "Gray", "Red", "Green", "Pink", "Graphite", "Beige", "Silver"}

var demoCategories = []demoCategory{
	{
		slug:   "electronics",
		brands: []string{"Sonora", "Voltix", "Audiara", "Lumio", "Kinetra"},
		kinds: []productKind{
			{
				"Bluetooth Headphones",
				true,
				[][]string{{"with Noise Cancelling", "In-Ear", "Over-Ear", "Sport"}, colors},
				59,
				899,
				120,
				400,
				[]string{
					"battery life up to 30 hours",
					"USB-C charging case",
					"built-in microphone for calls",
				},
			},
			{
				"Portable Speaker", true,
				[][]string{{"10 W", "20 W", "40 W", "60 W"}, {"Waterproof", "with RGB Light", "Bluetooth 5.3"}},
				89, 1299, 300, 2500,
				[]string{"splash-proof design", "stereo pairing between two speakers", "P2 auxiliary input"},
			},
			{
				"LED Smart TV",
				true,
				[][]string{{"32\"", "43\"", "50\"", "55\"", "65\""}, {"Full HD", "4K UHD"}},
				999,
				4999,
				4500,
				22000,
				[]string{
					"built-in Wi-Fi and streaming apps",
					"three HDMI inputs",
					"voice-command remote control",
				},
			},
			{
				"Soundbar", true,
				[][]string{{"2.0 Channel", "2.1 Channel", "5.1 Channel"}, {"with Subwoofer", "Bluetooth"}},
				299, 2499, 1500, 6000,
				[]string{"HDMI ARC connection", "night mode", "wall-mountable"},
			},
			{
				"Wi-Fi Security Camera",
				true,
				[][]string{{"Full HD", "2K", "360°"}, {"Night Vision", "Indoor", "Outdoor"}},
				119,
				599,
				250,
				800,
				[]string{
					"motion detection with phone alerts",
					"two-way audio",
					"microSD card recording",
				},
			},
			{
				"Portable Projector", true,
				[][]string{{"Full HD", "HD"}, {"200 ANSI Lumens", "350 ANSI Lumens"}},
				399, 2199, 900, 3000,
				[]string{"screen up to 120 inches", "keystone correction", "built-in speaker"},
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
					"6.5-inch AMOLED display",
					"50 MP triple camera",
					"5000 mAh battery with fast charging",
				},
			},
			{
				"Shockproof Phone Case", false,
				[][]string{{"Transparent", "Matte", "with Stand"}, colors},
				19, 79, 30, 80,
				[]string{"reinforced edges", "precise-touch buttons", "wireless charging compatible"},
			},
			{
				"3D Glass Screen Protector", false,
				[][]string{{"Privacy", "Matte", "Transparent"}},
				15, 49, 20, 60,
				[]string{"9H hardness", "application kit included", "covers the entire screen"},
			},
			{
				"Turbo USB-C Charger", true,
				[][]string{{"20 W", "33 W", "65 W"}, {"with 1 m Cable", "with 2 m Cable", "Dual Port"}},
				39, 199, 80, 200,
				[]string{"overheat protection", "automatic voltage switching", "PD and QC compatible"},
			},
			{
				"Smartwatch",
				true,
				[][]string{
					{"1.4\" Display", "1.8\" Display", "2.0\" Display"},
					{"with GPS", "with Heart Rate Monitor", "Water-Resistant"},
					colors,
				},
				149,
				1499,
				40,
				90,
				[]string{
					"phone notifications on your wrist",
					"more than 100 sport modes",
					"battery life up to 10 days",
				},
			},
			{
				"Power Bank", true,
				[][]string{{"10000 mAh", "20000 mAh", "30000 mAh"}, {"Fast Charging", "with Digital Display"}},
				69, 299, 200, 600,
				[]string{"two USB outputs and one USB-C", "charge indicator", "compact body"},
			},
		},
	},
	{
		slug:   "computers",
		brands: []string{"Datacore", "Pixelon", "Teknos", "Byteria", "Quadrix"},
		kinds: []productKind{
			{
				"Laptop",
				true,
				[][]string{
					{"14\"", "15.6\"", "16\""},
					{"8 GB RAM", "16 GB RAM", "32 GB RAM"},
					{"SSD 256 GB", "SSD 512 GB", "SSD 1 TB"},
				},
				1999,
				8999,
				1300,
				2600,
				[]string{"anti-glare screen", "backlit keyboard", "Wi-Fi 6 and Bluetooth 5.2"},
			},
			{
				"Gaming Monitor", true,
				[][]string{{"24\"", "27\"", "32\""}, {"144 Hz", "165 Hz", "240 Hz"}, {"Full HD", "QHD"}},
				699, 3499, 3000, 7000,
				[]string{"1 ms response time", "IPS panel", "height and tilt adjustment"},
			},
			{
				"Mechanical Keyboard",
				true,
				[][]string{
					{"ABNT2", "Compact 60%", "TKL"},
					{"Blue Switch", "Red Switch", "Brown Switch"},
					{"RGB", "White"},
				},
				129,
				699,
				500,
				1200,
				[]string{"anti-ghosting keys", "detachable USB-C cable", "macro software"},
			},
			{
				"Wireless Mouse", true,
				[][]string{{"Ergonomic", "Silent", "Gaming"}, {"1600 DPI", "4000 DPI", "16000 DPI"}, colors},
				39, 399, 70, 150,
				[]string{"nano USB receiver", "rechargeable battery", "high-precision optical sensor"},
			},
			{
				"NVMe SSD", true,
				[][]string{{"500 GB", "1 TB", "2 TB"}, {"PCIe 3.0", "PCIe 4.0"}},
				199, 1299, 20, 60,
				[]string{"read speeds up to 7000 MB/s", "5-year warranty", "M.2 2280 form factor"},
			},
			{
				"Webcam", true,
				[][]string{{"HD 720p", "Full HD 1080p", "2K"}, {"with Microphone", "with Privacy Cover"}},
				79, 499, 100, 300,
				[]string{"autofocus", "low-light correction", "plug and play"},
			},
		},
	},
	{
		slug:   "home",
		brands: []string{"Lar Bello", "Casamia", "Aconchego", "Madeiro", "Nórdica"},
		kinds: []productKind{
			{
				"Double Bed Sheet Set",
				false,
				[][]string{
					{"200 Thread Count", "300 Thread Count", "400 Thread Count"},
					{"Percale", "Egyptian Cotton", "Microfiber"},
					colors,
				},
				89,
				499,
				900,
				2000,
				[]string{"4 pieces with pillowcases", "fade-resistant fabric", "elastic all around"},
			},
			{
				"Floor Lamp", false,
				[][]string{{"Tripod", "Arc", "Articulated"}, {"Wood", "Black Metal", "Gold"}},
				149, 899, 2500, 7000,
				[]string{"E27 socket", "fabric shade", "switch on the cord"},
			},
			{
				"Non-Stick Cookware Set", false,
				[][]string{{"5 Pieces", "7 Pieces", "10 Pieces"}, {"Ceramic", "Teflon", "Stainless Steel"}},
				199, 1299, 3000, 9000,
				[]string{"cool-touch handles", "tempered glass lids", "induction compatible"},
			},
			{
				"Living Room Rug", false,
				[][]string{{"1.5 x 2 m", "2 x 2.5 m", "2.5 x 3 m"}, {"Plush", "Geometric", "Sisal"}},
				129, 999, 2000, 8000,
				[]string{"non-slip base", "easy to clean", "does not shed"},
			},
			{
				"Wardrobe Organizer", false,
				[][]string{{"4-Piece Kit", "6-Piece Kit", "10-Piece Kit"}, {"Fabric", "Acrylic", "Plastic"}},
				39, 199, 300, 1500,
				[]string{"foldable", "with lid and handle", "ideal for wardrobes and shelves"},
			},
			{
				"Office Chair", false,
				[][]string{{"Ergonomic", "Executive", "Gaming"}, {"Mesh", "Synthetic Leather"}, colors},
				349, 1999, 9000, 18000,
				[]string{"gas height adjustment", "lumbar support", "nylon casters"},
			},
		},
	},
	{
		slug:   "appliances",
		brands: []string{"Eletrolar", "Fornax", "Brisa", "Termika", "Doméstica"},
		kinds: []productKind{
			{
				"Air Fryer", true,
				[][]string{{"3.5 L", "4.5 L", "5.5 L", "12 L"}, {"Digital", "Analog"}, {"127 V", "220 V"}},
				249, 899, 3500, 7000,
				[]string{"removable non-stick basket", "timer up to 60 minutes", "oil-free cooking"},
			},
			{
				"Blender", true,
				[][]string{{"900 W", "1200 W", "1400 W"}, {"Glass Jar", "Acrylic Jar"}, {"127 V", "220 V"}},
				99, 499, 2000, 4000,
				[]string{"stainless steel blades", "12 speeds and pulse", "3-liter jar"},
			},
			{
				"Upright Vacuum Cleaner", true,
				[][]string{{"Cordless", "Corded"}, {"1000 W", "1250 W"}, {"127 V", "220 V", "Dual Voltage"}},
				199, 1499, 2000, 4500,
				[]string{"washable HEPA filter", "upholstery nozzle", "1-liter dust container"},
			},
			{
				"Microwave", true,
				[][]string{{"20 L", "27 L", "32 L"}, {"Mirror Finish", "White", "Stainless Steel"}, {"127 V", "220 V"}},
				499, 1299, 10000, 16000,
				[]string{"defrost function", "child safety lock", "preset recipes"},
			},
			{
				"Pedestal Fan", true,
				[][]string{{"40 cm", "50 cm"}, {"6 Blades", "8 Blades"}, {"127 V", "220 V"}},
				149, 449, 4000, 7000,
				[]string{"three speeds", "automatic oscillation", "adjustable height"},
			},
			{
				"Electric Coffee Maker", true,
				[][]string{{"15 Cups", "30 Cups"}, {"Programmable", "with Thermal Carafe"}, {"127 V", "220 V"}},
				89, 599, 1200, 3000,
				[]string{"permanent filter", "drip-stop system", "warming plate"},
			},
		},
	},
	{
		slug:   "fashion",
		brands: []string{"Urbana", "Trama", "Piquê", "Rota Sul", "Algodoaria"},
		kinds: []productKind{
			{
				"Basic T-Shirt", false,
				[][]string{{"Cotton", "Dry Fit", "Oversized"}, {"S", "M", "L", "XL"}, colors},
				29, 119, 150, 250,
				[]string{"soft fabric that does not wrinkle", "reinforced stitching", "unisex fit"},
			},
			{
				"Jeans", false,
				[][]string{{"Skinny", "Straight", "Wide Leg", "Cargo"}, {"36", "38", "40", "42", "44"}},
				89, 299, 500, 800,
				[]string{"stretch denim", "medium wash", "five pockets"},
			},
			{
				"Casual Sneakers", false,
				[][]string{{"Synthetic Leather", "Canvas", "Knit"}, {"37", "38", "39", "40", "41", "42"}, colors},
				99, 499, 700, 1200,
				[]string{"soft insole", "rubber sole", "cotton laces"},
			},
			{
				"Executive Backpack", false,
				[][]string{{"20 L", "28 L", "35 L"}, {"Waterproof", "with USB Port", "Anti-Theft"}, colors},
				79, 399, 600, 1300,
				[]string{"padded laptop compartment", "ergonomic straps", "hidden back pocket"},
			},
			{
				"Windbreaker Jacket", false,
				[][]string{{"Men's", "Women's"}, {"S", "M", "L", "XL"}, colors},
				119, 399, 300, 700,
				[]string{"hood built into the collar", "lightweight breathable fabric", "zippered pockets"},
			},
			{
				"Crossbody Bag", false,
				[][]string{{"Small", "Medium"}, {"Synthetic Leather", "Nylon"}, colors},
				59, 299, 300, 800,
				[]string{"adjustable strap", "zip closure", "inner pocket"},
			},
		},
	},
	{
		slug:   "sports",
		brands: []string{"Atlética", "Kanto", "Maré Alta", "Impulso", "Vértice"},
		kinds: []productKind{
			{
				"Running Shoes", false,
				[][]string{{"Cushioned", "Lightweight", "Trail"}, {"38", "39", "40", "41", "42", "43"}, colors},
				199, 899, 500, 900,
				[]string{"energy-return midsole", "breathable upper", "traction outsole"},
			},
			{
				"29\" Mountain Bike",
				false,
				[][]string{
					{"21-Speed", "24-Speed", "27-Speed"},
					{"Frame 17", "Frame 19"},
					{"Disc Brakes", "Hydraulic Brakes"},
				},
				1299,
				4999,
				13000,
				17000,
				[]string{"aluminum frame", "front suspension", "indexed shifting"},
			},
			{
				"Adjustable Dumbbell Set", false,
				[][]string{{"10 kg", "20 kg", "30 kg"}, {"Rubber-Coated", "Cast Iron"}},
				149, 899, 10000, 30000,
				[]string{"interchangeable weights", "bars with non-slip grip", "safety clips"},
			},
			{
				"Yoga Mat", false,
				[][]string{{"4 mm", "6 mm", "10 mm"}, {"TPE", "PVC", "Cork"}, colors},
				49, 249, 800, 2000,
				[]string{"non-slip surface", "carry strap", "easy to sanitize"},
			},
			{
				"Sports Thermal Bottle", false,
				[][]string{{"500 ml", "750 ml", "1 L"}, {"Stainless Steel", "with Straw"}, colors},
				39, 179, 300, 600,
				[]string{"keeps cold for 24 hours", "BPA-free", "leak-proof lid"},
			},
			{
				"Field Soccer Ball", false,
				[][]string{{"Size 5", "Size 4"}, {"Stitched", "Thermally Bonded"}},
				59, 249, 400, 500,
				[]string{"butyl bladder", "excellent air retention", "for use on natural grass"},
			},
		},
	},
	{
		slug:   "beauty",
		brands: []string{"Botânica", "Pele Viva", "Aurora", "Essenza", "Flor do Cerrado"},
		kinds: []productKind{
			{
				"Facial Serum", false,
				[][]string{{"Vitamin C", "Hyaluronic Acid", "Niacinamide", "Retinol"}, {"30 ml", "50 ml"}},
				39, 199, 80, 200,
				[]string{"lightweight fast-absorbing texture", "dermatologist tested", "vegan and cruelty-free"},
			},
			{
				"Sunscreen",
				false,
				[][]string{
					{"SPF 30", "SPF 50", "SPF 70"},
					{"Dry Touch", "Tinted", "Kids"},
					{"50 g", "120 ml", "200 ml"},
				},
				29,
				149,
				80,
				300,
				[]string{"UVA and UVB protection", "water-resistant", "non-greasy"},
			},
			{
				"Hair Dryer", true,
				[][]string{{"1900 W", "2100 W", "2400 W"}, {"Ionic", "Ceramic"}, {"127 V", "220 V"}},
				129, 699, 500, 900,
				[]string{"two speeds and three heat settings", "cool air shot", "concentrator nozzle"},
			},
			{
				"Shampoo and Conditioner Kit",
				false,
				[][]string{
					{"Curly Hair", "Straight Hair", "Anti-Hair Loss", "Hydrating"},
					{"300 ml", "500 ml", "1 L"},
				},
				39,
				189,
				600,
				2200,
				[]string{"sulfate- and paraben-free", "mild fragrance", "for daily use"},
			},
			{
				"Perfume",
				false,
				[][]string{
					{"Floral", "Woody", "Citrus", "Oriental"},
					{"Eau de Parfum", "Eau de Toilette"},
					{"50 ml", "100 ml"},
				},
				89,
				599,
				200,
				450,
				[]string{"long-lasting hold", "glass bottle", "strong top notes"},
			},
			{
				"Eyeshadow Palette", false,
				[][]string{{"12 Shades", "18 Shades", "35 Shades"}, {"Nude", "Colorful", "Matte and Shimmer"}},
				29, 179, 100, 300,
				[]string{"high pigmentation", "built-in mirror", "easy to blend"},
			},
		},
	},
	{
		slug:   "toys",
		brands: []string{"Brincalhão", "Pequenos Gênios", "Toca", "Pipa", "Imagina"},
		kinds: []productKind{
			{
				"Building Blocks", false,
				[][]string{{"300 Pieces", "500 Pieces", "1000 Pieces"}, {"City", "Space", "Castle", "Farm"}},
				59, 499, 500, 2500,
				[]string{"pieces compatible across sets", "illustrated manual", "encourages creativity"},
			},
			{
				"Articulated Doll", false,
				[][]string{{"30 cm", "45 cm"}, {"with Accessories", "with Extra Outfits"}},
				49, 299, 300, 900,
				[]string{"hair to brush", "articulated limbs", "non-toxic material"},
			},
			{
				"Remote Control Car", true,
				[][]string{{"1:16", "1:20", "1:24"}, {"4x4", "Drift", "Amphibious"}, colors},
				99, 699, 600, 2000,
				[]string{"rechargeable battery", "range up to 50 meters", "rubber tires"},
			},
			{
				"Jigsaw Puzzle", false,
				[][]string{{"500 Pieces", "1000 Pieces", "2000 Pieces"}, {"Landscape", "Map of Brazil", "Art"}},
				39, 199, 400, 1500,
				[]string{"perfect-fit pieces", "heavy cardboard", "reference poster"},
			},
			{
				"Board Game", false,
				[][]string{{"Strategy", "Cooperative", "Trivia"}, {"2 to 4 Players", "2 to 6 Players"}},
				59, 349, 500, 2000,
				[]string{"games lasting 30 to 60 minutes", "clear rulebook", "wooden components"},
			},
			{
				"Plush Toy", false,
				[][]string{{"Bear", "Rabbit", "Dinosaur", "Unicorn"}, {"25 cm", "40 cm", "60 cm"}},
				39, 249, 200, 900,
				[]string{"hypoallergenic filling", "reinforced stitching", "hand washable"},
			},
		},
	},
	{
		slug:   "tools",
		brands: []string{"Ferrum", "Obra Forte", "Torke", "Mestre", "Pinhão"},
		kinds: []productKind{
			{
				"Cordless Screwdriver", true,
				[][]string{{"12 V", "18 V", "20 V"}, {"with 2 Batteries", "with Carry Case", "with Bit Set"}},
				149, 899, 900, 2200,
				[]string{"adjustable torque in 20 positions", "LED work light", "quick-release chuck"},
			},
			{
				"Combination Wrench Set", false,
				[][]string{{"8 Pieces", "12 Pieces", "22 Pieces"}, {"Chrome Vanadium", "Carbon Steel"}},
				59, 399, 800, 4000,
				[]string{"chrome finish", "organizer case", "6 to 22 mm sizes"},
			},
			{
				"Jigsaw", true,
				[][]string{{"500 W", "650 W", "800 W"}, {"127 V", "220 V"}},
				179, 699, 1800, 3000,
				[]string{"angled cuts up to 45°", "variable speed", "dust blower"},
			},
			{
				"Angle Grinder", true,
				[][]string{{"4 1/2\"", "7\""}, {"720 W", "900 W", "2200 W"}, {"127 V", "220 V"}},
				159, 799, 1800, 5500,
				[]string{"side handle", "spindle lock", "adjustable disc guard"},
			},
			{
				"Laser Measure", true,
				[][]string{{"40 m", "60 m", "100 m"}, {"with Level", "with Bluetooth"}},
				99, 599, 100, 300,
				[]string{"2 mm accuracy", "area and volume calculation", "backlit screen"},
			},
			{
				"Toolbox", false,
				[][]string{{"with 129 Pieces", "with 200 Pieces", "Empty"}, {"Plastic", "Metal"}},
				59, 599, 1500, 9000,
				[]string{"removable tray", "reinforced latches", "ergonomic handle"},
			},
		},
	},
	{
		slug:   "books",
		brands: []string{"Editora Ipê", "Sabiá Livros", "Pena & Prosa", "Editora Farol", "Folha Seca"},
		kinds: []productKind{
			{
				"Go Programming Book",
				false,
				[][]string{
					{"from Scratch to Advanced", "Practical Guide", "Concurrency in Practice"},
					{"Paperback", "Hardcover"},
				},
				59,
				189,
				400,
				900,
				[]string{"complete examples and exercises", "source code download", "revised edition"},
			},
			{
				"Novel",
				false,
				[][]string{
					{
						"The Last Summer in Olinda",
						"Winds of the South",
						"The House on the Slope",
						"Letters from Ouro Preto",
					},
					{"Paperback", "Pocket Edition"},
				},
				29,
				89,
				250,
				600,
				[]string{"national bestseller", "immersive storytelling", "award-winning author"},
			},
			{
				"Cookbook",
				false,
				[][]string{
					{"Brazilian Cuisine", "Grandma's Sweets", "Vegan Dishes", "Barbecue"},
					{"Hardcover", "Spiral-Bound"},
				},
				49,
				159,
				600,
				1400,
				[]string{"over 150 recipes", "photos of every dish", "step-by-step preparation tips"},
			},
			{
				"Comic Book Box Set",
				false,
				[][]string{
					{"Volumes 1 to 3", "Volumes 1 to 5", "Complete Edition"},
					{"Adventure", "Science Fiction", "Fantasy"},
				},
				89,
				399,
				900,
				3000,
				[]string{"coated paper", "exclusive covers", "bonus sketches by the author"},
			},
			{
				"Children's Book",
				false,
				[][]string{
					{"The Curious Turtle", "The Dragon Who Was Afraid", "Journey to the Bottom of the Sea"},
					{"Hardcover", "Pop-Up"},
				},
				29,
				99,
				200,
				600,
				[]string{"colorful illustrations", "recommended for ages 3 to 7", "thick pages"},
			},
			{
				"Public Exam Study Guide", false,
				[][]string{{"Portuguese", "Mathematics", "Constitutional Law", "Logical Reasoning"}, {"2026 Edition"}},
				49, 179, 600, 1500,
				[]string{"commented questions", "summaries by topic", "mock exams at the end"},
			},
		},
	},
	{
		slug:   "car-parts",
		brands: []string{"Autobras", "Rodovia", "Motrix", "Garagem 21", "Pista"},
		kinds: []productKind{
			{
				"Car Phone Mount",
				false,
				[][]string{
					{"Magnetic", "Suction Cup", "Air Vent"},
					{"with Wireless Charger", "Swivel 360°"},
				},
				29,
				199,
				100,
				400,
				[]string{
					"secure hold even on dirt roads",
					"tool-free installation",
					"fits phones up to 7 inches",
				},
			},
			{
				"Windshield Wiper Blade Kit",
				false,
				[][]string{
					{"Pair 16\" and 20\"", "Pair 18\" and 22\"", "Pair 20\" and 24\""},
					{"Silicone", "Graphite"},
				},
				39,
				149,
				300,
				700,
				[]string{"quiet wiping", "universal fit", "sun-resistant rubber"},
			},
			{
				"Car Multimedia Head Unit", true,
				[][]string{{"7\"", "9\"", "10\""}, {"Android", "with Screen Mirroring"}, {"2 DIN", "1 DIN"}},
				399, 1899, 1000, 2500,
				[]string{"GPS and Bluetooth", "rear camera input", "FM radio"},
			},
			{
				"Portable Air Compressor", true,
				[][]string{{"12 V", "Cordless"}, {"Digital", "Analog"}},
				99, 399, 900, 2000,
				[]string{"automatic shut-off at set pressure", "LED flashlight", "nozzles for balls and inflatables"},
			},
			{
				"Car Seat Cover", false,
				[][]string{{"Synthetic Leather", "Fabric", "Neoprene"}, {"Full Set", "Front Seats"}, colors},
				99, 599, 1500, 4000,
				[]string{"compatible with side airbags", "easy to install", "washable"},
			},
			{
				"Car Vacuum Cleaner", true,
				[][]string{{"12 V", "Cordless"}, {"100 W", "120 W"}},
				59, 299, 600, 1500,
				[]string{"washable filter", "crevice nozzles", "4-meter cable"},
			},
		},
	},
}

// Shop name parts: "<prefix> <surname>" or "<surname> <suffix>", with the
// shop's main category flavour.
var (
	shopPrefixes = []string{
		"Emporium",
		"House",
		"World",
		"Store",
		"Corner",
		"Point",
		"Bazaar",
		"Warehouse",
		"Space",
		"Gallery",
	}
	shopSuffixes = map[string][]string{
		"electronics": {"Electronics", "Audio and Video", "Tech"},
		"phones":      {"Phones", "Mobile Accessories", "Cell"},
		"computers":   {"Computing", "Computers", "Hardware"},
		"home":        {"Home and Decor", "Utilities", "Decor"},
		"appliances":  {"Appliances", "Electro", "Kitchen"},
		"fashion":     {"Fashion", "Apparel", "Clothing"},
		"sports":      {"Sports", "Fitness", "Adventure"},
		"beauty":      {"Beauty", "Cosmetics", "Perfumery"},
		"toys":        {"Toys", "Kids", "Fun"},
		"tools":       {"Tools", "Construction", "Hardware Store"},
		"books":       {"Books", "Bookshop", "Reading"},
		"car-parts":   {"Auto Parts", "Automotive Accessories", "Car"},
	}
	shopSlogans = []string{
		"Fast service and same-business-day shipping.",
		"Selected products with warranty and invoice.",
		"Over ten years selling with quality.",
		"Fair prices and careful packaging.",
		"Family-run store with real customer service.",
		"We ship across Brazil.",
	}
	productClosings = []string{
		"New product, in sealed original packaging.",
		"Comes with invoice and 90-day seller warranty.",
		"Ships within 24 business hours after payment confirmation.",
		"Check the Q&A before you buy.",
		"12-month warranty against manufacturing defects.",
	}
	modelSeries = []string{"", "Pro", "Max", "Lite", "Plus", "Air", "Neo", "S", "X", "Ultra"}
)

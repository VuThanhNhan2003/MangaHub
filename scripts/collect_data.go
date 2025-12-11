package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Manga represents a manga entry
type Manga struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Author        string   `json:"author"`
	Genres        []string `json:"genres"`
	Status        string   `json:"status"`
	TotalChapters int      `json:"total_chapters"`
	Description   string   `json:"description"`
	CoverURL      string   `json:"cover_url"`
	Year          int      `json:"year"`
	Source        string   `json:"source"` // manual, mangadex, web_scraping
}

// MangaDexResponse represents the MangaDex API response
type MangaDexResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Title struct {
				En string `json:"en"`
				Ja string `json:"ja"`
			} `json:"title"`
			Description struct {
				En string `json:"en"`
			} `json:"description"`
			Status           string `json:"status"`
			Year             int    `json:"year"`
			LastChapter      string `json:"lastChapter"`
			ContentRating    string `json:"contentRating"`
			Tags             []struct {
				Attributes struct {
					Name struct {
						En string `json:"en"`
					} `json:"name"`
				} `json:"attributes"`
			} `json:"tags"`
		} `json:"attributes"`

		// FIXED: Relationships now support both author and cover_art
		Relationships []struct {
			Type       string `json:"type"`
			Attributes struct {
				Name     string `json:"name,omitempty"`     // for author
				FileName string `json:"fileName,omitempty"` // for cover_art
			} `json:"attributes,omitempty"`
		} `json:"relationships"`
	} `json:"data"`
}


func main() {
	log.Println("╔════════════════════════════════════════════════════════╗")
	log.Println("║      MangaHub Data Collection Script                  ║")
	log.Println("╚════════════════════════════════════════════════════════╝")

	var allManga []Manga

	// 1. Manual Entry Data (100 series)
	log.Println("\n📝 Step 1/3: Loading manual entry data...")
	manualData := getManualData()
	allManga = append(allManga, manualData...)
	log.Printf("✅ Loaded %d manga from manual entry\n", len(manualData))

	// 2. MangaDex API Integration (100 series)
	log.Println("\n🌐 Step 2/3: Fetching from MangaDex API...")
	mangadexData := fetchFromMangaDex(100)
	allManga = append(allManga, mangadexData...)
	log.Printf("✅ Fetched %d manga from MangaDex API\n", len(mangadexData))

	// 3. Web Scraping (Educational)
	log.Println("\n🕷️  Step 3/3: Web scraping practice...")
	scrapedData := practiceWebScraping()
	allManga = append(allManga, scrapedData...)
	log.Printf("✅ Scraped %d entries from practice sites\n", len(scrapedData))

	// Save to JSON file
	log.Println("\n💾 Saving data to JSON file...")
	saveToJSON(allManga, "data/manga_collection.json")

	// Generate statistics
	printStatistics(allManga)

	log.Println("\n✅ Data collection completed successfully!")
	log.Printf("📊 Total manga collected: %d\n", len(allManga))
	log.Println("📁 Output file: data/manga_collection.json")
}

// getManualData returns 100 manually entered popular manga
func getManualData() []Manga {
	return []Manga{
		// Shounen (25 series)
		{ID: "one-piece", Title: "One Piece", Author: "Oda Eiichiro", Genres: []string{"Action", "Adventure", "Comedy", "Drama", "Shounen"}, Status: "ongoing", TotalChapters: 1100, Description: "Monkey D. Luffy explores the Grand Line in search of One Piece to become the Pirate King.", Year: 1997, Source: "manual"},
		{ID: "naruto", Title: "Naruto", Author: "Kishimoto Masashi", Genres: []string{"Action", "Adventure", "Shounen", "Martial Arts"}, Status: "completed", TotalChapters: 700, Description: "Naruto Uzumaki dreams of becoming the Hokage of his village.", Year: 1999, Source: "manual"},
		{ID: "bleach", Title: "Bleach", Author: "Kubo Tite", Genres: []string{"Action", "Adventure", "Shounen", "Supernatural"}, Status: "completed", TotalChapters: 686, Description: "Ichigo Kurosaki becomes a Soul Reaper to protect the living world.", Year: 2001, Source: "manual"},
		{ID: "my-hero-academia", Title: "My Hero Academia", Author: "Horikoshi Kouhei", Genres: []string{"Action", "School", "Shounen", "Supernatural"}, Status: "ongoing", TotalChapters: 400, Description: "In a world of superheroes, Izuku Midoriya aims to become the greatest hero.", Year: 2014, Source: "manual"},
		{ID: "demon-slayer", Title: "Demon Slayer: Kimetsu no Yaiba", Author: "Gotouge Koyoharu", Genres: []string{"Action", "Historical", "Shounen", "Supernatural"}, Status: "completed", TotalChapters: 205, Description: "Tanjiro becomes a demon slayer to save his sister and avenge his family.", Year: 2016, Source: "manual"},
		{ID: "attack-on-titan", Title: "Attack on Titan", Author: "Isayama Hajime", Genres: []string{"Action", "Drama", "Fantasy", "Shounen"}, Status: "completed", TotalChapters: 139, Description: "Humanity fights for survival against giant Titans.", Year: 2009, Source: "manual"},
		{ID: "hunter-x-hunter", Title: "Hunter x Hunter", Author: "Togashi Yoshihiro", Genres: []string{"Action", "Adventure", "Shounen", "Fantasy"}, Status: "ongoing", TotalChapters: 390, Description: "Gon searches for his father and becomes a Hunter.", Year: 1998, Source: "manual"},
		{ID: "dragon-ball", Title: "Dragon Ball", Author: "Toriyama Akira", Genres: []string{"Action", "Adventure", "Comedy", "Shounen"}, Status: "completed", TotalChapters: 519, Description: "Goku's journey to find the Dragon Balls and become the strongest fighter.", Year: 1984, Source: "manual"},
		{ID: "jujutsu-kaisen", Title: "Jujutsu Kaisen", Author: "Akutami Gege", Genres: []string{"Action", "Shounen", "Supernatural"}, Status: "ongoing", TotalChapters: 250, Description: "Yuji Itadori joins jujutsu sorcerers to fight curses.", Year: 2018, Source: "manual"},
		{ID: "chainsaw-man", Title: "Chainsaw Man", Author: "Fujimoto Tatsuki", Genres: []string{"Action", "Shounen", "Supernatural", "Horror"}, Status: "ongoing", TotalChapters: 180, Description: "Denji becomes Chainsaw Man to live a normal life.", Year: 2018, Source: "manual"},
		{ID: "black-clover", Title: "Black Clover", Author: "Tabata Yuuki", Genres: []string{"Action", "Fantasy", "Shounen"}, Status: "ongoing", TotalChapters: 370, Description: "Asta aims to become the Wizard King despite having no magic.", Year: 2015, Source: "manual"},
		{ID: "haikyuu", Title: "Haikyuu!!", Author: "Furudate Haruichi", Genres: []string{"Sports", "Shounen", "School"}, Status: "completed", TotalChapters: 402, Description: "Shoyo Hinata's journey to become a great volleyball player.", Year: 2012, Source: "manual"},
		{ID: "the-promised-neverland", Title: "The Promised Neverland", Author: "Shirai Kaiu", Genres: []string{"Mystery", "Psychological", "Shounen"}, Status: "completed", TotalChapters: 181, Description: "Children escape from an orphanage with a dark secret.", Year: 2016, Source: "manual"},
		{ID: "dr-stone", Title: "Dr. Stone", Author: "Inagaki Riichiro", Genres: []string{"Adventure", "Sci-Fi", "Shounen"}, Status: "completed", TotalChapters: 232, Description: "Senku revives civilization with science after humanity turns to stone.", Year: 2017, Source: "manual"},
		{ID: "death-note", Title: "Death Note", Author: "Ohba Tsugumi", Genres: []string{"Mystery", "Psychological", "Supernatural", "Thriller"}, Status: "completed", TotalChapters: 108, Description: "Light Yagami finds a notebook that can kill anyone.", Year: 2003, Source: "manual"},
		{ID: "fullmetal-alchemist", Title: "Fullmetal Alchemist", Author: "Arakawa Hiromu", Genres: []string{"Action", "Adventure", "Drama", "Fantasy", "Shounen"}, Status: "completed", TotalChapters: 108, Description: "Brothers search for the Philosopher's Stone to restore their bodies.", Year: 2001, Source: "manual"},
		{ID: "tokyo-ghoul", Title: "Tokyo Ghoul", Author: "Ishida Sui", Genres: []string{"Action", "Horror", "Psychological", "Seinen"}, Status: "completed", TotalChapters: 143, Description: "Ken Kaneki becomes a half-ghoul and struggles with his identity.", Year: 2011, Source: "manual"},
		{ID: "assassination-classroom", Title: "Assassination Classroom", Author: "Matsui Yuusei", Genres: []string{"Action", "Comedy", "School", "Shounen"}, Status: "completed", TotalChapters: 180, Description: "Students must assassinate their alien teacher before he destroys Earth.", Year: 2012, Source: "manual"},
		{ID: "fire-force", Title: "Fire Force", Author: "Okubo Atsushi", Genres: []string{"Action", "Supernatural", "Shounen"}, Status: "completed", TotalChapters: 304, Description: "Shinra fights spontaneous human combustion with Fire Force.", Year: 2015, Source: "manual"},
		{ID: "blue-exorcist", Title: "Blue Exorcist", Author: "Kato Kazue", Genres: []string{"Action", "Supernatural", "Shounen"}, Status: "ongoing", TotalChapters: 150, Description: "Rin discovers he's Satan's son and becomes an exorcist.", Year: 2009, Source: "manual"},
		{ID: "magi", Title: "Magi: The Labyrinth of Magic", Author: "Ohtaka Shinobu", Genres: []string{"Action", "Adventure", "Fantasy", "Shounen"}, Status: "completed", TotalChapters: 369, Description: "Aladdin's journey through a magical Arabian Nights world.", Year: 2009, Source: "manual"},
		{ID: "toriko", Title: "Toriko", Author: "Shimabukuro Mitsutoshi", Genres: []string{"Action", "Adventure", "Comedy", "Fantasy", "Shounen"}, Status: "completed", TotalChapters: 396, Description: "Gourmet Hunter Toriko searches for rare ingredients.", Year: 2008, Source: "manual"},
		{ID: "world-trigger", Title: "World Trigger", Author: "Ashihara Daisuke", Genres: []string{"Action", "School", "Sci-Fi", "Shounen"}, Status: "ongoing", TotalChapters: 230, Description: "Agents defend Earth from interdimensional invaders.", Year: 2013, Source: "manual"},
		{ID: "kuroko-basketball", Title: "Kuroko's Basketball", Author: "Fujimaki Tadatoshi", Genres: []string{"Sports", "Shounen", "School"}, Status: "completed", TotalChapters: 275, Description: "Kuroko and Kagami aim for basketball championships.", Year: 2008, Source: "manual"},
		{ID: "prince-of-tennis", Title: "The Prince of Tennis", Author: "Konomi Takeshi", Genres: []string{"Sports", "Shounen", "School"}, Status: "completed", TotalChapters: 379, Description: "Tennis prodigy Ryoma aims for national championships.", Year: 1999, Source: "manual"},

		// Seinen (20 series)
		{ID: "berserk", Title: "Berserk", Author: "Miura Kentaro", Genres: []string{"Action", "Adventure", "Drama", "Fantasy", "Horror", "Seinen"}, Status: "ongoing", TotalChapters: 380, Description: "Guts seeks revenge against demons and his former friend Griffith.", Year: 1989, Source: "manual"},
		{ID: "vagabond", Title: "Vagabond", Author: "Inoue Takehiko", Genres: []string{"Action", "Adventure", "Drama", "Historical", "Seinen"}, Status: "ongoing", TotalChapters: 327, Description: "Musashi Miyamoto's journey to become Japan's greatest swordsman.", Year: 1998, Source: "manual"},
		{ID: "vinland-saga", Title: "Vinland Saga", Author: "Yukimura Makoto", Genres: []string{"Action", "Adventure", "Drama", "Historical", "Seinen"}, Status: "ongoing", TotalChapters: 200, Description: "Thorfinn's quest for revenge in the Viking era.", Year: 2005, Source: "manual"},
		{ID: "monster", Title: "Monster", Author: "Urasawa Naoki", Genres: []string{"Drama", "Mystery", "Psychological", "Seinen"}, Status: "completed", TotalChapters: 162, Description: "Dr. Tenma hunts a former patient who became a serial killer.", Year: 1994, Source: "manual"},
		{ID: "one-punch-man", Title: "One Punch Man", Author: "ONE", Genres: []string{"Action", "Comedy", "Parody", "Sci-Fi", "Seinen", "Supernatural"}, Status: "ongoing", TotalChapters: 190, Description: "Saitama can defeat any opponent with one punch.", Year: 2009, Source: "manual"},
		{ID: "parasyte", Title: "Parasyte", Author: "Iwaaki Hitoshi", Genres: []string{"Action", "Drama", "Horror", "Psychological", "Sci-Fi", "Seinen"}, Status: "completed", TotalChapters: 64, Description: "Shinichi coexists with an alien parasite in his hand.", Year: 1988, Source: "manual"},
		{ID: "kingdom", Title: "Kingdom", Author: "Hara Yasuhisa", Genres: []string{"Action", "Drama", "Historical", "Military", "Seinen"}, Status: "ongoing", TotalChapters: 770, Description: "Shin aims to become the greatest general in China's Warring States period.", Year: 2006, Source: "manual"},
		{ID: "tokyo-ghoul-re", Title: "Tokyo Ghoul:re", Author: "Ishida Sui", Genres: []string{"Action", "Drama", "Horror", "Mystery", "Psychological", "Seinen"}, Status: "completed", TotalChapters: 179, Description: "Sequel to Tokyo Ghoul following Haise Sasaki.", Year: 2014, Source: "manual"},
		{ID: "gantz", Title: "Gantz", Author: "Oku Hiroya", Genres: []string{"Action", "Drama", "Horror", "Psychological", "Sci-Fi", "Seinen"}, Status: "completed", TotalChapters: 383, Description: "People fight aliens in a deadly survival game.", Year: 2000, Source: "manual"},
		{ID: "hellsing", Title: "Hellsing", Author: "Hirano Kouta", Genres: []string{"Action", "Horror", "Seinen", "Supernatural"}, Status: "completed", TotalChapters: 92, Description: "Alucard serves the Hellsing Organization hunting vampires.", Year: 1997, Source: "manual"},
		{ID: "homunculus", Title: "Homunculus", Author: "Yamamoto Hideo", Genres: []string{"Drama", "Horror", "Mystery", "Psychological", "Seinen"}, Status: "completed", TotalChapters: 166, Description: "A man gains the ability to see people's psychological traumas.", Year: 2003, Source: "manual"},
		{ID: "pluto", Title: "Pluto", Author: "Urasawa Naoki", Genres: []string{"Drama", "Mystery", "Sci-Fi", "Seinen"}, Status: "completed", TotalChapters: 65, Description: "Detective robot investigates murders of advanced robots.", Year: 2003, Source: "manual"},
		{ID: "goodnight-punpun", Title: "Goodnight Punpun", Author: "Asano Inio", Genres: []string{"Drama", "Psychological", "Seinen", "Slice of Life"}, Status: "completed", TotalChapters: 147, Description: "Punpun's disturbing journey through adolescence to adulthood.", Year: 2007, Source: "manual"},
		{ID: "dorohedoro", Title: "Dorohedoro", Author: "Hayashida Q", Genres: []string{"Action", "Adventure", "Comedy", "Fantasy", "Horror", "Seinen"}, Status: "completed", TotalChapters: 167, Description: "Caiman searches for the sorcerer who cursed him.", Year: 2000, Source: "manual"},
		{ID: "blame", Title: "Blame!", Author: "Nihei Tsutomu", Genres: []string{"Action", "Drama", "Horror", "Psychological", "Sci-Fi", "Seinen"}, Status: "completed", TotalChapters: 66, Description: "Killy searches for humans with Net Terminal Genes.", Year: 1997, Source: "manual"},
		{ID: "ajin", Title: "Ajin: Demi-Human", Author: "Sakurai Gamon", Genres: []string{"Action", "Horror", "Mystery", "Seinen", "Supernatural"}, Status: "completed", TotalChapters: 86, Description: "Immortal demi-humans are hunted by the government.", Year: 2012, Source: "manual"},
		{ID: "inuyashiki", Title: "Inuyashiki", Author: "Oku Hiroya", Genres: []string{"Action", "Drama", "Psychological", "Sci-Fi", "Seinen"}, Status: "completed", TotalChapters: 85, Description: "An old man becomes a powerful robot and fights evil.", Year: 2014, Source: "manual"},
		{ID: "i-am-a-hero", Title: "I Am a Hero", Author: "Hanazawa Kengo", Genres: []string{"Action", "Drama", "Horror", "Psychological", "Seinen"}, Status: "completed", TotalChapters: 264, Description: "A manga artist survives a zombie apocalypse.", Year: 2009, Source: "manual"},
		{ID: "golden-kamuy", Title: "Golden Kamuy", Author: "Noda Satoru", Genres: []string{"Action", "Adventure", "Historical", "Seinen"}, Status: "completed", TotalChapters: 314, Description: "Search for hidden Ainu gold in post-war Hokkaido.", Year: 2014, Source: "manual"},
		{ID: "uzumaki", Title: "Uzumaki", Author: "Ito Junji", Genres: []string{"Drama", "Horror", "Mystery", "Psychological", "Seinen", "Supernatural"}, Status: "completed", TotalChapters: 20, Description: "A town becomes cursed by spirals.", Year: 1998, Source: "manual"},

		// Shoujo (20 series)
		{ID: "fruits-basket", Title: "Fruits Basket", Author: "Takaya Natsuki", Genres: []string{"Comedy", "Drama", "Romance", "Shoujo", "Supernatural"}, Status: "completed", TotalChapters: 136, Description: "Tohru lives with the Sohma family cursed to turn into zodiac animals.", Year: 1998, Source: "manual"},
		{ID: "ouran-high-school", Title: "Ouran High School Host Club", Author: "Hatori Bisco", Genres: []string{"Comedy", "Harem", "Romance", "School", "Shoujo"}, Status: "completed", TotalChapters: 83, Description: "Haruhi accidentally joins an elite host club.", Year: 2002, Source: "manual"},
		{ID: "sailor-moon", Title: "Sailor Moon", Author: "Takeuchi Naoko", Genres: []string{"Fantasy", "Magic", "Romance", "Shoujo"}, Status: "completed", TotalChapters: 60, Description: "Usagi becomes Sailor Moon to fight evil.", Year: 1991, Source: "manual"},
		{ID: "cardcaptor-sakura", Title: "Cardcaptor Sakura", Author: "CLAMP", Genres: []string{"Adventure", "Comedy", "Fantasy", "Magic", "Romance", "Shoujo"}, Status: "completed", TotalChapters: 50, Description: "Sakura captures magical Clow Cards.", Year: 1996, Source: "manual"},
		{ID: "nana", Title: "Nana", Author: "Yazawa Ai", Genres: []string{"Drama", "Music", "Romance", "Shoujo", "Slice of Life"}, Status: "ongoing", TotalChapters: 84, Description: "Two women named Nana navigate love and dreams in Tokyo.", Year: 2000, Source: "manual"},
		{ID: "kimi-ni-todoke", Title: "Kimi ni Todoke", Author: "Shiina Karuho", Genres: []string{"Comedy", "Romance", "School", "Shoujo"}, Status: "completed", TotalChapters: 123, Description: "Shy Sawako tries to make friends and find love.", Year: 2005, Source: "manual"},
		{ID: "ao-haru-ride", Title: "Ao Haru Ride", Author: "Sakisaka Io", Genres: []string{"Comedy", "Drama", "Romance", "School", "Shoujo"}, Status: "completed", TotalChapters: 53, Description: "Futaba reunites with her first love in high school.", Year: 2011, Source: "manual"},
		{ID: "skip-beat", Title: "Skip Beat!", Author: "Nakamura Yoshiki", Genres: []string{"Comedy", "Drama", "Romance", "Shoujo"}, Status: "ongoing", TotalChapters: 300, Description: "Kyoko enters showbiz for revenge but finds her passion.", Year: 2002, Source: "manual"},
		{ID: "lovely-complex", Title: "Lovely★Complex", Author: "Nakahara Aya", Genres: []string{"Comedy", "Romance", "School", "Shoujo"}, Status: "completed", TotalChapters: 68, Description: "Tall girl and short boy fall in love.", Year: 2001, Source: "manual"},
		{ID: "kaichou-wa-maid-sama", Title: "Kaichou wa Maid-sama!", Author: "Fujiwara Hiro", Genres: []string{"Comedy", "Romance", "School", "Shoujo"}, Status: "completed", TotalChapters: 85, Description: "Student council president secretly works at a maid café.", Year: 2005, Source: "manual"},
		{ID: "vampire-knight", Title: "Vampire Knight", Author: "Hino Matsuri", Genres: []string{"Drama", "Mystery", "Romance", "Shoujo", "Supernatural"}, Status: "completed", TotalChapters: 93, Description: "Yuki guards the secret of the Night Class vampires.", Year: 2004, Source: "manual"},
		{ID: "orange", Title: "Orange", Author: "Takano Ichigo", Genres: []string{"Drama", "Romance", "School", "Shoujo", "Sci-Fi"}, Status: "completed", TotalChapters: 22, Description: "Naho receives letters from her future self to save a friend.", Year: 2012, Source: "manual"},
		{ID: "strobe-edge", Title: "Strobe Edge", Author: "Sakisaka Io", Genres: []string{"Drama", "Romance", "School", "Shoujo"}, Status: "completed", TotalChapters: 42, Description: "Ninako's unrequited love for popular Ren.", Year: 2007, Source: "manual"},
		{ID: "hana-yori-dango", Title: "Hana Yori Dango", Author: "Kamio Yoko", Genres: []string{"Comedy", "Drama", "Romance", "School", "Shoujo"}, Status: "completed", TotalChapters: 244, Description: "Poor girl stands up to elite F4 group.", Year: 1992, Source: "manual"},
		{ID: "marmalade-boy", Title: "Marmalade Boy", Author: "Yoshizumi Wataru", Genres: []string{"Comedy", "Drama", "Romance", "School", "Shoujo"}, Status: "completed", TotalChapters: 40, Description: "Parents swap partners and families merge.", Year: 1992, Source: "manual"},
		{ID: "colette", Title: "Colette Decides to Die", Author: "Aoi Makino", Genres: []string{"Drama", "Fantasy", "Romance", "Shoujo"}, Status: "completed", TotalChapters: 52, Description: "Pharmacist tries to live her remaining days fully.", Year: 2018, Source: "manual"},
		{ID: "akatsuki-no-yona", Title: "Akatsuki no Yona", Author: "Kusanagi Mizuho", Genres: []string{"Action", "Adventure", "Comedy", "Fantasy", "Romance", "Shoujo"}, Status: "ongoing", TotalChapters: 230, Description: "Princess Yona gathers legendary dragon warriors.", Year: 2009, Source: "manual"},
		{ID: "the-rose-of-versailles", Title: "The Rose of Versailles", Author: "Ikeda Riyoko", Genres: []string{"Drama", "Historical", "Romance", "Shoujo"}, Status: "completed", TotalChapters: 82, Description: "Oscar serves Marie Antoinette in revolutionary France.", Year: 1972, Source: "manual"},
		{ID: "basara", Title: "Basara", Author: "Tamura Yumi", Genres: []string{"Adventure", "Drama", "Fantasy", "Romance", "Shoujo"}, Status: "completed", TotalChapters: 107, Description: "Sarasa leads rebellion in post-apocalyptic Japan.", Year: 1990, Source: "manual"},
		{ID: "yona-of-the-dawn", Title: "Yona of the Dawn", Author: "Kusanagi Mizuho", Genres: []string{"Action", "Adventure", "Fantasy", "Romance", "Shoujo"}, Status: "ongoing", TotalChapters: 230, Description: "Princess seeks revenge and gathers dragon warriors.", Year: 2009, Source: "manual"},

		// Josei (15 series)
		{ID: "nana-2", Title: "Paradise Kiss", Author: "Yazawa Ai", Genres: []string{"Drama", "Romance", "Josei", "Slice of Life"}, Status: "completed", TotalChapters: 48, Description: "High school girl becomes a model for fashion students.", Year: 2000, Source: "manual"},
		{ID: "nodame-cantabile", Title: "Nodame Cantabile", Author: "Ninomiya Tomoko", Genres: []string{"Comedy", "Drama", "Music", "Romance", "Josei", "Slice of Life"}, Status: "completed", TotalChapters: 146, Description: "Talented pianist and messy pianist find love through music.", Year: 2001, Source: "manual"},
		{ID: "honey-and-clover", Title: "Honey and Clover", Author: "Umino Chica", Genres: []string{"Comedy", "Drama", "Romance", "Josei", "Slice of Life"}, Status: "completed", TotalChapters: 64, Description: "Art students navigate love and career dreams.", Year: 2000, Source: "manual"},
		{ID: "chihayafuru", Title: "Chihayafuru", Author: "Suetsugu Yuki", Genres: []string{"Drama", "Josei", "School", "Sports"}, Status: "completed", TotalChapters: 248, Description: "Chihaya pursues competitive karuta card game.", Year: 2007, Source: "manual"},
		{ID: "usagi-drop", Title: "Usagi Drop", Author: "Unita Yumi", Genres: []string{"Drama", "Josei", "Slice of Life"}, Status: "completed", TotalChapters: 62, Description: "Bachelor adopts his grandfather's illegitimate daughter.", Year: 2005, Source: "manual"},
		{ID: "kids-on-the-slope", Title: "Kids on the Slope", Author: "Kodama Yuki", Genres: []string{"Drama", "Music", "Romance", "Josei", "School", "Slice of Life"}, Status: "completed", TotalChapters: 45, Description: "Friendship and jazz in 1960s Japan.", Year: 2007, Source: "manual"},
		{ID: "princess-jellyfish", Title: "Princess Jellyfish", Author: "Higashimura Akiko", Genres: []string{"Comedy", "Josei", "Slice of Life"}, Status: "completed", TotalChapters: 93, Description: "Otaku girls meet a fashionable cross-dresser.", Year: 2008, Source: "manual"},
		{ID: "chihaya", Title: "Sakamichi no Apollon", Author: "Kodama Yuki", Genres: []string{"Drama", "Historical", "Music", "Romance", "Josei"}, Status: "completed", TotalChapters: 45, Description: "Jazz brings friends together in 1960s Japan.", Year: 2007, Source: "manual"},
        {ID: "emma", Title: "Emma", Author: "Mori Kaoru", Genres: []string{"Drama", "Historical", "Romance", "Josei"}, Status: "completed", TotalChapters: 70, Description: "Victorian maid Emma falls in love with a wealthy gentleman.", Year: 2002, Source: "manual"},
        {ID: "you're-my-pet", Title: "You're My Pet (Tramps Like Us)", Author: "Ogawa Yayoi", Genres: []string{"Comedy", "Romance", "Josei"}, Status: "completed", TotalChapters: 82, Description: "Career woman unexpectedly ends up living with a younger man she calls her pet.", Year: 2000, Source: "manual"},
        {ID: "kuragehime", Title: "Kuragehime (Princess Jellyfish)", Author: "Higashimura Akiko", Genres: []string{"Comedy", "Slice of Life", "Josei"}, Status: "completed", TotalChapters: 93, Description: "Shy otaku girls' lives change after meeting a stylish cross-dresser.", Year: 2008, Source: "manual"},
        {ID: "solanin", Title: "Solanin", Author: "Asano Inio", Genres: []string{"Drama", "Slice of Life", "Josei"}, Status: "completed", TotalChapters: 29, Description: "A young couple struggles with adulthood, dreams, and reality.", Year: 2005, Source: "manual"},
        {ID: "sing-yesterday-for-me", Title: "Sing Yesterday for Me", Author: "Toume Kei", Genres: []string{"Drama", "Romance", "Josei"}, Status: "completed", TotalChapters: 113, Description: "A college graduate in limbo forms complex relationships with two women.", Year: 1997, Source: "manual"},
        {ID: "paradise-kiss", Title: "Paradise Kiss", Author: "Yazawa Ai", Genres: []string{"Drama", "Romance", "Josei", "Slice of Life"}, Status: "completed", TotalChapters: 48, Description: "High school girl becomes a model for a group of fashion designers.", Year: 1999, Source: "manual"},
        {ID: "jellyfish-princess", Title: "Tokyo Tarareba Girls", Author: "Higashimura Akiko", Genres: []string{"Comedy", "Drama", "Josei"}, Status: "completed", TotalChapters: 52, Description: "Women in their 30s struggle with romance, career, and expectations.", Year: 2014, Source: "manual"},
        {ID: "descendant-of-the-sun", Title: "Downfall", Author: "Asano Inio", Genres: []string{"Drama", "Psychological", "Josei"}, Status: "completed", TotalChapters: 17, Description: "A manga artist faces depression, marital issues, and career collapse.", Year: 2017, Source: "manual"},
        {ID: "kitchen-princess", Title: "Shitsuren Chocolatier", Author: "Mizushiro Setona", Genres: []string{"Drama", "Romance", "Josei"}, Status: "completed", TotalChapters: 44, Description: "A chocolatier dedicates his career to winning his crush's heart.", Year: 2008, Source: "manual"},
        {ID: "december-song", Title: "Helter Skelter", Author: "Okazaki Kyoko", Genres: []string{"Drama", "Psychological", "Josei"}, Status: "completed", TotalChapters: 19, Description: "A model spirals into self-destruction after undergoing extreme surgeries.", Year: 1995, Source: "manual"},
        {ID: "suppli", Title: "Suppli", Author: "Okazaki Mari", Genres: []string{"Drama", "Romance", "Josei"}, Status: "completed", TotalChapters: 83, Description: "A career-driven woman struggles with loneliness and relationships.", Year: 2003, Source: "manual"},
        {ID: "after-the-rain", Title: "After the Rain", Author: "Mayuzuki Jun", Genres: []string{"Drama", "Romance", "Josei"}, Status: "completed", TotalChapters: 82, Description: "A former track star falls for her middle-aged restaurant manager.", Year: 2014, Source: "manual"},
        {ID: "poco", Title: "Poco's Udon World", Author: "Shinobu Yoshida", Genres: []string{"Slice of Life", "Josei"}, Status: "completed", TotalChapters: 65, Description: "A man returns to his hometown and befriends a tanuki boy.", Year: 2012, Source: "manual"},
        {ID: "7seeds", Title: "7SEEDS", Author: "Tamura Yumi", Genres: []string{"Adventure", "Drama", "Sci-Fi", "Josei"}, Status: "completed", TotalChapters: 178, Description: "Humans awaken in a future apocalypse to rebuild civilization.", Year: 2001, Source: "manual"},
        {ID: "chouyaku", Title: "Aozora Yell", Author: "Kawahara Kazune", Genres: []string{"Drama", "Romance", "Josei"}, Status: "completed", TotalChapters: 58, Description: "A girl joins brass band club to pursue her dreams.", Year: 2008, Source: "manual"},

        // Isekai (21 series)
        {ID: "rezero", Title: "Re:Zero", Author: "Nagatsuki Tappei", Genres: []string{"Isekai", "Fantasy", "Drama"}, Status: "ongoing", TotalChapters: 200, Description: "Subaru is transported to another world and gains the ability to rewind death.", Year: 2014, Source: "manual"},
        {ID: "overlord", Title: "Overlord", Author: "Maruyama Kugane", Genres: []string{"Isekai", "Fantasy"}, Status: "ongoing", TotalChapters: 75, Description: "A player trapped in a game becomes an undead overlord.", Year: 2010, Source: "manual"},
        {ID: "shield-hero", Title: "The Rising of the Shield Hero", Author: "Aneko Yusagi", Genres: []string{"Isekai", "Fantasy", "Drama"}, Status: "ongoing", TotalChapters: 100, Description: "Naofumi is summoned as the Shield Hero to save another world.", Year: 2012, Source: "manual"},
        {ID: "slime-tensei", Title: "That Time I Got Reincarnated as a Slime", Author: "Fuse", Genres: []string{"Isekai", "Fantasy"}, Status: "ongoing", TotalChapters: 120, Description: "A man reincarnates as a slime with powerful abilities.", Year: 2013, Source: "manual"},
        {ID: "konosuba", Title: "Konosuba", Author: "Akatsuki Natsume", Genres: []string{"Isekai", "Comedy", "Fantasy"}, Status: "ongoing", TotalChapters: 70, Description: "Kazuma is transported to a fantasy world with eccentric companions.", Year: 2013, Source: "manual"},
        {ID: "no-game-no-life", Title: "No Game No Life", Author: "Kamiya Yuu", Genres: []string{"Isekai", "Fantasy", "Psychological"}, Status: "ongoing", TotalChapters: 20, Description: "Genius siblings are transported to a world ruled by games.", Year: 2012, Source: "manual"},
        {ID: "jobless-reincarnation", Title: "Mushoku Tensei", Author: "Rifujin na Magonote", Genres: []string{"Isekai", "Fantasy"}, Status: "ongoing", TotalChapters: 95, Description: "A shut-in reincarnates to live a better life with great magical talent.", Year: 2012, Source: "manual"},
        {ID: "grimgar", Title: "Grimgar of Fantasy and Ash", Author: "Jumonji Ao", Genres: []string{"Isekai", "Fantasy", "Drama"}, Status: "ongoing", TotalChapters: 20, Description: "People awaken in a fantasy world with no memories.", Year: 2013, Source: "manual"},
        {ID: "gate", Title: "GATE", Author: "Yanai Takumi", Genres: []string{"Isekai", "Fantasy", "Military"}, Status: "completed", TotalChapters: 90, Description: "A portal opens in Tokyo connecting to a fantasy world.", Year: 2010, Source: "manual"},
        {ID: "saga-tanya", Title: "The Saga of Tanya the Evil", Author: "Zen Carlo", Genres: []string{"Isekai", "Military", "Fantasy"}, Status: "ongoing", TotalChapters: 65, Description: "A salaryman reincarnates as a girl soldier in a war-torn magical world.", Year: 2012, Source: "manual"},
        {ID: "reincarnated-aristocrat", Title: "The Reincarnated Aristocrat", Author: "Miya Kinojo", Genres: []string{"Isekai", "Fantasy"}, Status: "ongoing", TotalChapters: 70, Description: "A man reincarnates as a noble who excels through strategy.", Year: 2020, Source: "manual"},
        {ID: "sao", Title: "Sword Art Online", Author: "Kawahara Reki", Genres: []string{"Isekai", "Sci-Fi", "Fantasy"}, Status: "ongoing", TotalChapters: 100, Description: "Players trapped in a VRMMO must clear the game to survive.", Year: 2009, Source: "manual"},
        {ID: "tate-yuusha", Title: "Tsukimichi: Moonlit Fantasy", Author: "Azumi Kei", Genres: []string{"Isekai", "Fantasy"}, Status: "ongoing", TotalChapters: 80, Description: "A boy transported to another world becomes overpowered.", Year: 2012, Source: "manual"},
        {ID: "farm-life", Title: "Isekai Nonbiri Nouka", Author: "Kinosuke Naito", Genres: []string{"Isekai", "Slice of Life", "Fantasy"}, Status: "ongoing", TotalChapters: 200, Description: "A man reincarnates to live a peaceful farming life.", Year: 2017, Source: "manual"},
        {ID: "death-march", Title: "Death March kara Hajimaru Isekai Kyousoukyoku", Author: "Ainana Hiro", Genres: []string{"Isekai", "Fantasy"}, Status: "ongoing", TotalChapters: 100, Description: "A programmer is transported to a game world with extreme power.", Year: 2013, Source: "manual"},
        {ID: "ascendance-bookworm", Title: "Ascendance of a Bookworm", Author: "Kazuki Miya", Genres: []string{"Isekai", "Fantasy", "Slice of Life"}, Status: "ongoing", TotalChapters: 60, Description: "A girl reincarnates into a world without books and vows to make them.", Year: 2013, Source: "manual"},
        {ID: "arifureta", Title: "Arifureta", Author: "Shirakome Ryo", Genres: []string{"Isekai", "Fantasy", "Action"}, Status: "ongoing", TotalChapters: 80, Description: "A bullied boy becomes powerful after being betrayed in another world.", Year: 2013, Source: "manual"},
        {ID: "smartphone-isekai", Title: "Isekai wa Smartphone to Tomo ni", Author: "Fuyuhara Patora", Genres: []string{"Isekai", "Fantasy", "Comedy"}, Status: "ongoing", TotalChapters: 70, Description: "A boy is sent to another world with a magical smartphone.", Year: 2013, Source: "manual"},
        {ID: "hundred-lives", Title: "Yuusha Shinda! 100 Lives", Author: "Kawakami Naoki", Genres: []string{"Isekai", "Fantasy", "Comedy"}, Status: "completed", TotalChapters: 215, Description: "A weak protagonist dies repeatedly but grows stronger.", Year: 2014, Source: "manual"},
        {ID: "kobayashi-dragon", Title: "Miss Kobayashi's Dragon Maid", Author: "Coolkyousinnjya", Genres: []string{"Comedy", "Slice of Life", "Fantasy"}, Status: "ongoing", TotalChapters: 130, Description: "Dragons enter the human world and live daily life with their host.", Year: 2013, Source: "manual"},
        {ID: "isekai-ojisan", Title: "Isekai Ojisan", Author: "Hotondoshindeiru", Genres: []string{"Comedy", "Isekai", "Fantasy"}, Status: "ongoing", TotalChapters: 60, Description: "An uncle returns from another world and recounts his adventures.", Year: 2018, Source: "manual"},

        // Horror / Mystery / Fantasy (15 series)
        {ID: "another", Title: "Another", Author: "Ayatsuji Yukito", Genres: []string{"Horror", "Mystery", "Supernatural"}, Status: "completed", TotalChapters: 20, Description: "A cursed classroom suffers mysterious deaths.", Year: 2009, Source: "manual"},
        {ID: "tomie", Title: "Tomie", Author: "Ito Junji", Genres: []string{"Horror", "Supernatural"}, Status: "completed", TotalChapters: 44, Description: "A mysterious girl causes obsession and madness wherever she goes.", Year: 1987, Source: "manual"},
        {ID: "devilman", Title: "Devilman", Author: "Go Nagai", Genres: []string{"Horror", "Action", "Supernatural"}, Status: "completed", TotalChapters: 53, Description: "A teen merges with a demon to protect humanity.", Year: 1972, Source: "manual"},
        {ID: "jjba", Title: "JoJo's Bizarre Adventure", Author: "Araki Hirohiko", Genres: []string{"Action", "Supernatural", "Adventure"}, Status: "ongoing", TotalChapters: 960, Description: "Generational battles between Joestar family and evil forces.", Year: 1987, Source: "manual"},
        {ID: "soul-eater", Title: "Soul Eater", Author: "Ookubo Atsushi", Genres: []string{"Action", "Supernatural", "Fantasy"}, Status: "completed", TotalChapters: 113, Description: "Students train to become weapon meisters.", Year: 2004, Source: "manual"},
        {ID: "noragami", Title: "Noragami", Author: "Adachitoka", Genres: []string{"Action", "Supernatural", "Fantasy"}, Status: "ongoing", TotalChapters: 110, Description: "A minor god takes odd jobs to gain followers.", Year: 2010, Source: "manual"},
        {ID: "made-in-abyss", Title: "Made in Abyss", Author: "Tsukushi Akihito", Genres: []string{"Adventure", "Fantasy", "Mystery"}, Status: "ongoing", TotalChapters: 70, Description: "A girl descends into the mysterious and deadly Abyss.", Year: 2012, Source: "manual"},
        {ID: "claymore", Title: "Claymore", Author: "Yagi Norihiro", Genres: []string{"Action", "Fantasy", "Drama"}, Status: "completed", TotalChapters: 155, Description: "Warriors fight shape-shifting monsters called Yoma.", Year: 2001, Source: "manual"},
        {ID: "pandora-hearts", Title: "Pandora Hearts", Author: "Mochizuki Jun", Genres: []string{"Fantasy", "Mystery", "Drama"}, Status: "completed", TotalChapters: 107, Description: "A boy falls into the Abyss and uncovers secrets of his past.", Year: 2006, Source: "manual"},
        {ID: "the-girl-from-the-other-side", Title: "The Girl From the Other Side", Author: "Nagabe", Genres: []string{"Fantasy", "Mystery"}, Status: "completed", TotalChapters: 47, Description: "A cursed outsider protects a young girl in a dark fairytale world.", Year: 2015, Source: "manual"},
        {ID: "d-grey-man", Title: "D.Gray-man", Author: "Hoshino Katsura", Genres: []string{"Action", "Fantasy", "Supernatural"}, Status: "ongoing", TotalChapters: 245, Description: "Exorcists fight akuma created from human souls.", Year: 2004, Source: "manual"},
        {ID: "fma-brotherhood", Title: "Silver Spoon", Author: "Arakawa Hiromu", Genres: []string{"Slice of Life", "Comedy"}, Status: "completed", TotalChapters: 116, Description: "A city boy attends an agricultural high school.", Year: 2011, Source: "manual"},
        {ID: "magus-bride", Title: "The Ancient Magus' Bride", Author: "Yamazaki Kore", Genres: []string{"Fantasy", "Romance", "Supernatural"}, Status: "ongoing", TotalChapters: 100, Description: "A girl is bought by a mysterious magus to become his apprentice.", Year: 2013, Source: "manual"},
        {ID: "dragon-quest-dai", Title: "Dragon Quest: Dai no Daibouken", Author: "Inada Koji", Genres: []string{"Action", "Adventure", "Fantasy"}, Status: "completed", TotalChapters: 343, Description: "Dai trains to become a hero in a classic fantasy world.", Year: 1989, Source: "manual"},
        {ID: "witch-hat", Title: "Witch Hat Atelier", Author: "Shirahama Kamome", Genres: []string{"Fantasy", "Drama"}, Status: "ongoing", TotalChapters: 70, Description: "A girl enters a magical atelier after discovering spell secrets.", Year: 2016, Source: "manual"},
	}
}

// fetchFromMangaDex fetches manga from MangaDex API
func fetchFromMangaDex(limit int) []Manga {
	var allManga []Manga
	baseURL := "https://api.mangadex.org/manga"
	
	// Fetch in batches to respect rate limits
	batchSize := 20 // Reduced batch size to be more respectful
	offset := 0
	
	for len(allManga) < limit && offset < limit*2 {
		url := fmt.Sprintf("%s?limit=%d&offset=%d&includes[]=author&includes[]=cover_art&contentRating[]=safe&contentRating[]=suggestive&order[relevance]=desc", 
			baseURL, batchSize, offset)
		
		log.Printf("   Fetching batch: offset=%d (collected: %d/%d)...", offset, len(allManga), limit)
		
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("   ⚠️  Error fetching from MangaDex: %v", err)
			break
		}

		if resp.StatusCode != 200 {
			log.Printf("   ⚠️  MangaDex returned status %d", resp.StatusCode)
			resp.Body.Close()
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		
		if err != nil {
			log.Printf("   ⚠️  Error reading response: %v", err)
			break
		}

		var mdResp MangaDexResponse
		if err := json.Unmarshal(body, &mdResp); err != nil {
			log.Printf("   ⚠️  Error parsing response: %v", err)
			break
		}

		// Process manga data
		for _, item := range mdResp.Data {
			if len(allManga) >= limit {
				break
			}

			// Get title (prefer English, fallback to Japanese)
			title := item.Attributes.Title.En
			if title == "" {
				title = item.Attributes.Title.Ja
			}
			if title == "" {
				continue // Skip if no title
			}

			// Get author
			author := "Unknown"
			for _, rel := range item.Relationships {
				if rel.Type == "author" && rel.Attributes.Name != "" {
					author = rel.Attributes.Name
					break
				}
			}

			// Get genres from tags
			var genres []string
			for _, tag := range item.Attributes.Tags {
				if tag.Attributes.Name.En != "" {
					genres = append(genres, tag.Attributes.Name.En)
				}
			}
			if len(genres) == 0 {
				genres = []string{"General"}
			}

			// Parse chapter count
			totalChapters := 0
			if item.Attributes.LastChapter != "" {
				fmt.Sscanf(item.Attributes.LastChapter, "%d", &totalChapters)
			}
			if totalChapters == 0 {
				totalChapters = 50 // Default for ongoing
			}

			// Get description
			description := item.Attributes.Description.En
			if description == "" {
				description = "No description available."
			}
			if len(description) > 500 {
				description = description[:497] + "..."
			}

            // Cover URL (NEW)
            coverURL := ""
            for _, rel := range item.Relationships {
                if rel.Type == "cover_art" && rel.Attributes.FileName != "" {
                    coverURL = fmt.Sprintf(
                        "https://uploads.mangadex.org/covers/%s/%s",
                        item.ID,
                        rel.Attributes.FileName,
                    )
                    break
                }
            }

			// Create ID from title
			id := strings.ToLower(title)
			id = strings.ReplaceAll(id, " ", "-")
			id = strings.ReplaceAll(id, ":", "")
			id = strings.ReplaceAll(id, "'", "")
			id = strings.ReplaceAll(id, "!", "")
			id = strings.ReplaceAll(id, "?", "")
			id = strings.ReplaceAll(id, ",", "")
			id = strings.ReplaceAll(id, ".", "")
			// Limit ID length
			if len(id) > 50 {
				id = id[:50]
			}
			// Add suffix to make unique
			id = fmt.Sprintf("md-%s-%d", id, len(allManga))

			// Map status
			status := item.Attributes.Status
			if status == "" {
				status = "unknown"
			}

			manga := Manga{
				ID:            id,
				Title:         title,
				Author:        author,
				Genres:        genres,
				Status:        status,
				TotalChapters: totalChapters,
				Description:   description,
				CoverURL:      coverURL,
				Year:          item.Attributes.Year,
				Source:        "mangadex",
			}

			allManga = append(allManga, manga)
		}

		offset += batchSize
		
		// Respect rate limits - wait 1 second between requests
		if len(allManga) < limit {
			time.Sleep(1 * time.Second)
		}
		
		if len(mdResp.Data) < batchSize {
			break // No more data
		}
	}

	log.Printf("   ℹ️  Successfully collected %d manga from MangaDex", len(allManga))
	return allManga
}

// practiceWebScraping demonstrates web scraping from practice sites
func practiceWebScraping() []Manga {
	var allManga []Manga

	// 1. Scrape from quotes.toscrape.com (educational purpose)
	log.Println("   📖 Scraping quotes.toscrape.com (educational)...")
	quoteManga := scrapeQuoteSite()
	allManga = append(allManga, quoteManga...)

	// 2. Test httpbin.org API (educational)
	log.Println("   🧪 Testing httpbin.org API (educational)...")
	httpbinManga := testHTTPBin()
	allManga = append(allManga, httpbinManga...)

	return allManga
}

// scrapeQuoteSite scrapes quotes.toscrape.com as educational practice
func scrapeQuoteSite() []Manga {
	var manga []Manga
	
	resp, err := http.Get("https://quotes.toscrape.com/")
	if err != nil {
		log.Printf("   ⚠️  Error accessing quotes site: %v", err)
		return manga
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("   ⚠️  Error parsing HTML: %v", err)
		return manga
	}

	// Create fictional manga based on quotes and authors
	doc.Find(".quote").Each(func(i int, s *goquery.Selection) {
		if i >= 10 { // Limit to 10 entries
			return
		}

		author := s.Find(".author").Text()
		quote := s.Find(".text").Text()
		
		// Clean quote
		quote = strings.ReplaceAll(quote, `"`, "")
		quote = strings.ReplaceAll(quote, `'`, "")
		if len(quote) > 200 {
			quote = quote[:197] + "..."
		}
		
		// Create a fictional manga inspired by the quote
		id := fmt.Sprintf("quote-manga-%d", i+1)
		title := fmt.Sprintf("The Tales of %s", author)
		
		manga = append(manga, Manga{
			ID:            id,
			Title:         title,
			Author:        author,
			Genres:        []string{"Philosophy", "Drama", "Slice of Life"},
			Status:        "completed",
			TotalChapters: 50 + i*10,
			Description:   fmt.Sprintf("A philosophical manga inspired by: %s", quote),
			Year:          2020 + i,
			Source:        "web_scraping_practice",
		})
	})

	log.Printf("   ✅ Created %d fictional manga from quotes", len(manga))
	return manga
}

// testHTTPBin tests httpbin.org API endpoints
func testHTTPBin() []Manga {
	var manga []Manga

	// Test different HTTP methods and responses
	endpoints := []struct {
		url  string
		name string
	}{
		{"https://httpbin.org/uuid", "UUID Generator"},
		{"https://httpbin.org/user-agent", "User Agent"},
		{"https://httpbin.org/headers", "Headers Inspector"},
		{"https://httpbin.org/ip", "IP Address"},
		{"https://httpbin.org/get", "GET Request"},
	}

	for i, endpoint := range endpoints {
		resp, err := http.Get(endpoint.url)
		if err != nil {
			log.Printf("   ⚠️  Error accessing %s: %v", endpoint.url, err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var data map[string]interface{}
		json.Unmarshal(body, &data)

		// Create fictional manga based on API response
		id := fmt.Sprintf("httpbin-test-%d", i+1)
		description := fmt.Sprintf("Educational manga created from testing %s endpoint. This demonstrates HTTP API interactions and JSON parsing.", endpoint.url)
		
		manga = append(manga, Manga{
			ID:            id,
			Title:         fmt.Sprintf("HTTPBin Adventure: %s", endpoint.name),
			Author:        "API Testing Team",
			Genres:        []string{"Sci-Fi", "Technology", "Educational"},
			Status:        "completed",
			TotalChapters: 25,
			Description:   description,
			Year:          2024,
			Source:        "web_scraping_practice",
		})
	}

	log.Printf("   ✅ Created %d test manga from httpbin.org", len(manga))
	return manga
}

// saveToJSON saves manga data to JSON file
func saveToJSON(manga []Manga, filename string) error {
	// Ensure directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manga); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	log.Printf("✅ Saved %d manga to %s", len(manga), filename)
	return nil
}

// printStatistics displays collection statistics
func printStatistics(manga []Manga) {
	log.Println("\n╔════════════════════════════════════════════════════════╗")
	log.Println("║              Collection Statistics                     ║")
	log.Println("╚════════════════════════════════════════════════════════╝")

	// Count by source
	sourceCount := make(map[string]int)
	for _, m := range manga {
		sourceCount[m.Source]++
	}

	log.Println("\n📊 By Source:")
	for source, count := range sourceCount {
		log.Printf("   - %-25s: %d manga\n", source, count)
	}

	// Count by genre
	genreCount := make(map[string]int)
	for _, m := range manga {
		for _, genre := range m.Genres {
			genreCount[genre]++
		}
	}

	log.Println("\n📚 Top Genres:")
	type genreStat struct {
		Genre string
		Count int
	}
	var topGenres []genreStat
	for genre, count := range genreCount {
		topGenres = append(topGenres, genreStat{genre, count})
	}
	
	// Sort by count (bubble sort for simplicity)
	for i := 0; i < len(topGenres)-1; i++ {
		for j := 0; j < len(topGenres)-i-1; j++ {
			if topGenres[j].Count < topGenres[j+1].Count {
				topGenres[j], topGenres[j+1] = topGenres[j+1], topGenres[j]
			}
		}
	}
	
	displayCount := 15
	if len(topGenres) < displayCount {
		displayCount = len(topGenres)
	}
	
	for i := 0; i < displayCount; i++ {
		log.Printf("   %2d. %-25s: %d manga\n", i+1, topGenres[i].Genre, topGenres[i].Count)
	}

	// Count by status
	statusCount := make(map[string]int)
	for _, m := range manga {
		statusCount[m.Status]++
	}

	log.Println("\n📈 By Status:")
	for status, count := range statusCount {
		log.Printf("   - %-25s: %d manga\n", status, count)
	}

	// Year statistics
	var minYear, maxYear int
	yearCount := make(map[int]int)
	for _, m := range manga {
		if m.Year > 0 {
			yearCount[m.Year]++
			if minYear == 0 || m.Year < minYear {
				minYear = m.Year
			}
			if m.Year > maxYear {
				maxYear = m.Year
			}
		}
	}

	if minYear > 0 {
		log.Println("\n📅 Year Range:")
		log.Printf("   Oldest: %d\n", minYear)
		log.Printf("   Newest: %d\n", maxYear)
	}

	log.Println()
}
package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Geralt28/gator/internal/config"
	"github.com/Geralt28/gator/internal/database"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type state struct {
	db     *database.Queries
	config *config.Config
}

type command struct {
	name      string
	arguments []string
}

type commands struct {
	komendy map[string]func(*state, command) error
}

// ******** START:  Struct for RSS feed *********

type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

// ******** END:  Struct for RSS feed *********

func handlerLogin(s *state, cmd command) error {
	if len(cmd.arguments) == 0 {
		return fmt.Errorf("no arguments")
	}
	if len(cmd.arguments) != 1 {
		return fmt.Errorf("error: login expects exactly one argument (username)")
	}
	user, err := s.db.GetUser(context.Background(), cmd.arguments[0])
	if err != nil {
		fmt.Println("error: user does not exist")
		os.Exit(1)
	}
	err = s.config.SetUser(user.Name)
	if err != nil {
		return fmt.Errorf("failed to set user: %v", err)
	}
	fmt.Println("User:", user.Name, "has been logged in!")
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.arguments) == 0 {
		return fmt.Errorf("no arguments")
	}
	if len(cmd.arguments) != 1 {
		return fmt.Errorf("error: register expects exactly one argument (username)")
	} else {
		id := uuid.New()
		user := cmd.arguments[0]
		arg := database.CreateUserParams{
			ID:        id,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Name:      user,
		}
		user_db, err := s.db.CreateUser(context.Background(), arg)
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
		s.config.SetUser(user)
		fmt.Println("User:", user, "has been registered!")
		fmt.Println(user_db)
	}
	return nil
}

func handlerReset(s *state, cmd command) error {
	if len(cmd.arguments) != 0 {
		return fmt.Errorf("no arguments for reset command")
	}
	err := s.db.DeleteUsers(context.Background())
	if err != nil {
		fmt.Println("error: failed to delete users")
		os.Exit(1)
	}
	fmt.Println("User list had beed reset!")
	return nil
}

func handlerUsers(s *state, cmd command) error {
	if len(cmd.arguments) != 0 {
		return fmt.Errorf("no arguments for users command")
	}
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		fmt.Println("error: failed to list users")
		os.Exit(1)
	}
	for _, user := range users {
		if user == s.config.Current_user_name {
			user = user + " (current)"
		}
		fmt.Printf("* %s\n", user)
	}
	return nil
}

func handlerAgg(s *state, cmd command) error {
	url := "https://www.wagslane.dev/index.xml"
	feed, err := fetchFeed(context.Background(), url)
	if err != nil {
		return err
	}
	// Wyswietle podstawowe informacje o feed
	fmt.Printf("Feed Title: %s\n", feed.Channel.Title)
	fmt.Printf("Feed Description: %s\n", feed.Channel.Description)
	fmt.Printf("Feed Link: %s\n\n", feed.Channel.Link)
	// Drukuj poszczegolne elementy feedu
	for _, item := range feed.Channel.Items {
		fmt.Printf("Title: %s\n", item.Title)
		fmt.Printf("Link: %s\n", item.Link)
		fmt.Printf("Published: %s\n", item.PubDate)
		fmt.Printf("Description: %s\n\n", item.Description)
	}
	//fmt.Println(feed)
	return nil
}

func handlerAddFeed(s *state, cmd command) error {
	if len(cmd.arguments) != 2 {
		return fmt.Errorf("error: addfeed expects exactly two arguments (name_of_feed, feed_url)")
	}
	//rss, err := fetchFeed(ctx, cmd.arguments[0])
	//if err != nil {
	//	return err}
	user_name := s.config.Current_user_name
	user, err := s.db.GetUser(context.Background(), user_name)
	if err != nil {
		fmt.Println("User not found in database")
		return err
	}
	s.db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.arguments[0],
		Url:       sql.NullString{String: cmd.arguments[1], Valid: true},
		UserID:    uuid.NullUUID{UUID: user.ID, Valid: true},
	})
	return nil
}

func handlerFeeds(s *state, cmd command) error {
	//if len(cmd.arguments) != 0 {
	//return fmt.Errorf("error: feeds should not have any arguments")
	//}
	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return err
	}
	for _, feed := range feeds {
		fmt.Println("Name:", feed.Name, " | ", "URL:", feed.Url.String, " | ", "User:", feed.User.String)
	}
	return nil
}

func (c *commands) register(name string, f func(*state, command) error) {
	//rejestruje fukcje pod nazwa "name" jako klucz i funcje f, ktora bedzie obslugiwala komende
	c.komendy[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	// zrzuca funkcje "handler" obslugujaca dane polecenie i sprawdza czy jest taka zarejestrowana
	handler, exists := c.komendy[cmd.name]
	if !exists {
		return fmt.Errorf("unknown command: %s", cmd.name)
	}
	// zwraca s config, cmd czyli komendy, wraz z fukncja obslugujaca komende
	return handler(s, cmd)
}

func fetchFeed(ctx context.Context, feedURL string) (*RSS, error) {
	var rss RSS
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Gator")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if err = xml.Unmarshal(data, &rss); err != nil {
		return nil, err
	}
	rss.Channel.Title = html.UnescapeString(rss.Channel.Title)
	rss.Channel.Description = html.UnescapeString(rss.Channel.Description)
	return &rss, nil
}

func main() {
	// odczytaj config
	cfg, err := config.Read()
	if err != nil {
		fmt.Println("Warning: Could not read config file.")
		os.Exit(1)
		//cfg = config.Config{} // Default config if none exists
	}
	//c_cfg.config.SetUser(user)
	//cfg, _ = config.Read()
	//fmt.Println(cfg)

	//zainicjuj zmienna ktora jest powazana z cfg odczytana z dysku
	s := &state{config: &cfg}
	var c_commands = commands{komendy: make(map[string]func(*state, command) error)}

	// zarejestruj polecenia:
	c_commands.register("login", handlerLogin)
	c_commands.register("register", handlerRegister)
	c_commands.register("reset", handlerReset)
	c_commands.register("users", handlerUsers)
	c_commands.register("agg", handlerAgg)
	c_commands.register("addfeed", handlerAddFeed)
	c_commands.register("feeds", handlerFeeds)

	args := os.Args

	if len(args) < 2 {
		fmt.Println("error: not enough arguments")
		os.Exit(1)
	}

	c_command := command{name: args[1], arguments: args[2:]}

	db, err := sql.Open("postgres", s.config.Db_url)
	if err != nil {
		fmt.Println("error: can not open database")
	}
	dbQueries := database.New(db)
	s.db = dbQueries

	// Uruchom polecenie
	if err := c_commands.run(s, c_command); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

}

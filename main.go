package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Geralt28/gator/internal/config"
	"github.com/Geralt28/gator/internal/database"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// List of common RSS datetime formats
var rssDateFormats = []string{
	time.RFC1123,                // "Mon, 02 Jan 2006 15:04:05 MST"
	time.RFC1123Z,               // "Mon, 02 Jan 2006 15:04:05 -0700"
	time.RFC3339,                // "2006-01-02T15:04:05Z"
	"02 Jan 2006 15:04:05 MST",  // "02 Jan 2006 15:04:05 UTC"
	"2006-01-02 15:04:05 -0700", // "2006-01-02 15:04:05 -0700"
	"2006-01-02T15:04:05-0700",  // "2006-01-02T15:04:05-0700"
}

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

func handlerAgg(s *state, cmd command, time_between_reqs string) error {

	fmt.Println("Collecting feeds every", time_between_reqs)
	timeBetweenRequests, err := time.ParseDuration(time_between_reqs)
	if err != nil {
		fmt.Println("error: can not convert string into time duration")
	}
	ticker := time.NewTicker(timeBetweenRequests)
	for ; ; <-ticker.C {
		fmt.Println("updating feeds...")
		fmt.Println()
		scrapeFeeds(s)
	}

	//url := "https://www.wagslane.dev/index.xml"
	//feed, err := fetchFeed(context.Background(), url)
	//if err != nil {
	//	return err
	//}
	//// Wyswietle podstawowe informacje o feed
	//fmt.Printf("Feed Title: %s\n", feed.Channel.Title)
	//fmt.Printf("Feed Description: %s\n", feed.Channel.Description)
	//fmt.Printf("Feed Link: %s\n\n", feed.Channel.Link)
	//// Drukuj poszczegolne elementy feedu
	//for _, item := range feed.Channel.Items {
	//	fmt.Printf("Title: %s\n", item.Title)
	//	fmt.Printf("Link: %s\n", item.Link)
	//	fmt.Printf("Published: %s\n", item.PubDate)
	//	fmt.Printf("Description: %s\n\n", item.Description)
	//}
	////fmt.Println(feed)
	//return nil
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.arguments) != 2 {
		return fmt.Errorf("error: addfeed expects exactly two arguments (name_of_feed, feed_url)")
	}
	//rss, err := fetchFeed(ctx, cmd.arguments[0])
	//if err != nil {
	//	return err}
	s.db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.arguments[0],
		Url:       sql.NullString{String: cmd.arguments[1], Valid: true},
		UserID:    user.ID,
	})

	followCmd := command{
		name:      "follow",
		arguments: []string{cmd.arguments[1]}, // Pass only URL
	}
	err := handlerFollow(s, followCmd, user)
	if err != nil {
		return err
	}
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

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.arguments) != 1 {
		return fmt.Errorf("error: follow should have only one argument: url")
	}
	url := cmd.arguments[0]
	followParams := database.CreateFeedFollowParams{
		Name: user.Name,
		Url:  sql.NullString{String: url, Valid: true},
	}
	createFeedData, err := s.db.CreateFeedFollow(context.Background(), followParams)
	if err != nil {
		return err
	}
	fmt.Println("Feed", url, "followed!")
	fmt.Println("Feed_Name:", createFeedData.FeedName, " | ", "User_Name:", createFeedData.UserName)
	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {
	//if len(cmd.arguments) != 0 {
	//	return fmt.Errorf("error: following should not have any arguments")
	//}
	follows, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return err
	}
	for _, follow := range follows {
		fmt.Println(follow.Feedname)
	}
	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.arguments) != 1 {
		return fmt.Errorf("error: unfollow should have only one argument: url")
	}
	url := cmd.arguments[0]
	Parametry := database.DeleteFeedFollowParams{
		UserID: user.ID,
		Url:    sql.NullString{String: url, Valid: true},
	}
	err := s.db.DeleteFeedFollow(context.Background(), Parametry)
	if err != nil {
		fmt.Println("You are not following feed:", url)
		return err
	}
	return nil
}

func handlerBrowse(s *state, cmd command, user database.User) error {
	dl := len(cmd.arguments)

	if dl > 1 {
		fmt.Println("error: only one optional parameter for browse command")
		return fmt.Errorf("too many parameters for browse command")
	}
	ilosc := 2
	if dl == 1 {
		i, err := strconv.Atoi(cmd.arguments[0])
		ilosc = i
		if err != nil {
			return fmt.Errorf("invalid number format: %v", err)
		}
	}
	PostsForUserParams := database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  int32(ilosc),
	}
	posts, err := s.db.GetPostsForUser(context.Background(), PostsForUserParams)
	if err != nil {
		fmt.Println("error: can not find any posts for user")
		return err
	}
	err = feedDetailPostsPrint(posts)
	if err != nil {
		fmt.Println("error: can show any posts for user")
		return err
	}
	return nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(s *state, cmd command) error {
	return func(s *state, cmd command) error {
		// Get the currently logged-in user
		user, err := s.db.GetUser(context.Background(), s.config.Current_user_name)
		if err != nil {
			fmt.Println("Error: No user is logged in or user not found.")
			return err
		}
		// Call the actual handler, passing the user along
		return handler(s, cmd, user)
	}
}

// Function more to train then necessary. Not sure why it is said Agg funcion need to take parameter and not to set as constant
func middlewareAgg(handler func(s *state, cmd command, time_between_reqs string) error) func(s *state, cmd command) error {
	return func(s *state, cmd command) error {
		// Set 1 minute as a string parameter
		time_between_reqs := "1m0s"
		// Call the actual handler, passing the user along
		return handler(s, cmd, time_between_reqs)
	}
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

func scrapeFeeds(s *state) error {
	feeds, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return err
	}
	for _, feed := range feeds {
		url := feed.Url.String
		rss, err := fetchFeed(context.Background(), url) //w tamtej funkcji dodac url albo jakos przekazac...
		if err != nil {
			fmt.Println("error: could not fetch feed:", url)
			return err
		}
		err = s.db.MarkFeedFetched(context.Background(), feed.ID)
		if err != nil {
			fmt.Println("error: could not mark as fetched:", url)
			return err
		}
		var czas_Valid bool
		for _, item := range rss.Channel.Items {
			DataStr := item.PubDate
			czas, err := parseRSSTime(DataStr)
			if err != nil {
				czas_Valid = false
				fmt.Println("error: could not parse string into date:", DataStr)
			} else {
				czas_Valid = true
			}
			PostParams := database.CreatePostParams{
				Title:       item.Title,
				Url:         item.Link,
				Description: item.Description,
				PublishedAt: sql.NullTime{Time: czas, Valid: czas_Valid},
				FeedID:      feed.ID,
			}
			err = s.db.CreatePost(context.Background(), PostParams)
			if err != nil {
				var pqErr *pq.Error
				if errors.As(err, &pqErr) && pqErr.Code != "23505" { // 23505 = unique_violation
					fmt.Println("error: could not create new post:", err)
				}
			}
		}

		//err = feedBasicPrint(*rss)
		//if err != nil {
		//	fmt.Println("error: could not print feed:", url)
		//	continue
	}
	return nil
}

func parseRSSTime(dateStr string) (time.Time, error) {
	var parsedTime time.Time
	var err error
	// Try each format until one works
	for _, format := range rssDateFormats {
		parsedTime, err = time.Parse(format, dateStr)
		if err == nil {
			return parsedTime, nil // Success!
		}
	}
	return time.Time{}, fmt.Errorf("could not parse date: %s", dateStr)
}

func feedDetailRSSPrint(rss RSS) error {
	// Wyswietle podstawowe informacje o feed
	err := feedBasicPrint(rss)
	if err != nil {
		return err
	}
	fmt.Printf("Feed Title: %s\n", rss.Channel.Title)
	fmt.Printf("Feed Description: %s\n", rss.Channel.Description)
	fmt.Printf("Feed Link: %s\n\n", rss.Channel.Link)
	// Drukuj poszczegolne elementy feedu
	for _, item := range rss.Channel.Items {
		fmt.Printf("Title: %s\n", item.Title)
		fmt.Printf("Link: %s\n", item.Link)
		fmt.Printf("Published: %s\n", item.PubDate)
		fmt.Printf("Description: %s\n\n", item.Description)
	}
	return nil
}

func feedBasicPrint(rss RSS) error {
	// Wyswietle podstawowe informacje o feed
	fmt.Printf("Feed Title: %s\n", rss.Channel.Title)
	fmt.Printf("Feed Description: %s\n", rss.Channel.Description)
	fmt.Printf("Feed Link: %s\n\n", rss.Channel.Link)
	// Drukuj poszczegolne elementy feedu
	//for _, item := range rss.Channel.Items {
	//	fmt.Printf("Title: %s\n", item.Title)
	//	fmt.Printf("Link: %s\n", item.Link)
	//	fmt.Printf("Published: %s\n", item.PubDate)
	//	fmt.Printf("Description: %s\n\n", item.Description)
	//}
	return nil
}

func feedDetailPostsPrint(posts []database.GetPostsForUserRow) error {
	// Drukuj poszczegolne elementy feedu
	for _, item := range posts {
		fmt.Printf("Title: %s\n", item.Title)
		fmt.Printf("Url: %s\n", item.Url)
		fmt.Printf("Published: %s\n", item.PublishedAt.Time)
		fmt.Printf("Description: %s\n\n", item.Description)
	}
	return nil
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
	c_commands.register("agg", middlewareAgg(handlerAgg))
	c_commands.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	c_commands.register("feeds", handlerFeeds)
	c_commands.register("follow", middlewareLoggedIn(handlerFollow))
	c_commands.register("following", middlewareLoggedIn(handlerFollowing))
	c_commands.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	c_commands.register("browse", middlewareLoggedIn(handlerBrowse))

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

package main

import (
	"fmt"
	"os"

	"github.com/Geralt28/gator/internal/config"
)

type state struct {
	config *config.Config
}

type command struct {
	name      string
	arguments []string
}

type commands struct {
	komendy map[string]func(*state, command) error
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.arguments) == 0 {
		return fmt.Errorf("no arguments")
	}
	if cmd.name == "login" && len(cmd.arguments) != 1 {
		return fmt.Errorf("error: login expects exactly one argument (username)")
	} else {
		err := s.config.SetUser(cmd.arguments[0])
		if err != nil {
			return fmt.Errorf("failed to set user: %v", err)
		}
		fmt.Println("User:", cmd.arguments[0], "has been set!")
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

//func (c *commands) login(s *state, cmd command) error {
//	cmd, err := c.komendy[cmd.name]
//	return nil
//}

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

	args := os.Args

	if len(args) < 2 {
		fmt.Println("error: not enough arguments")
		os.Exit(1)
	}

	c_command := command{name: args[1], arguments: args[2:]}

	// Uruchom polecenie
	if err := c_commands.run(s, c_command); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

}

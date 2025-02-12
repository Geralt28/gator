package main

import (
	"fmt"

	"github.com/Geralt28/gator/internal/config"
)

func main() {
	cfg, _ := config.Read()
	user := "Geralt"
	cfg.SetUser(user)
	cfg, _ = config.Read()
	fmt.Println(cfg)

}

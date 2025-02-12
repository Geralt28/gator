package config

import (
	"encoding/json"
	"os"
)

const configFileName = ".gatorconfig.json"

type Config struct {
	Db_url            string `json:"db_url"`
	Current_user_name string `json:"current_user_name"`
}

func (c *Config) SetUser(user string) error {
	c.Current_user_name = user
	json_file, err := json.Marshal(c)
	if err != nil {
		return err
	}
	home_dir, _ := os.UserHomeDir()
	file_name := home_dir + "/" + configFileName
	err = os.WriteFile(file_name, json_file, 0644)
	if err != nil {
		return err
	}
	return nil
}

func Read() (Config, error) {

	var cfg Config
	home_dir, _ := os.UserHomeDir()
	file_name := home_dir + "/" + configFileName
	json_file, err := os.ReadFile(file_name)
	if err != nil {
		return Config{}, err
	}

	err = json.Unmarshal(json_file, &cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil

}

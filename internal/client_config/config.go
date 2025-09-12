package clientconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	ID         string   `json:"id"`
	Rooms      []string `json:"rooms"`
	ServerIP   string   `json:"server_ip"`
	ServerPort uint16   `json:"server_port"`
}

func (c *Config) String() string {
	return fmt.Sprintf("Загруженный канфиг:\n  --> ID: %v\n  --> Комнаты: %v\n  --> IP сервера: %v\n  --> Порт сервера: %v\n", c.ID, c.Rooms, c.ServerIP, c.ServerPort)
}

func MustLoad() *Config {
	path := "internal/client_config/config.json"
	file, err := os.Open(path)
	if err != nil {
		panic(fmt.Sprintf("cannot open config file: %v", err))
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		panic(fmt.Sprintf("cannot decode config file: %v", err))
	}

	return &cfg
}

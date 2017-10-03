package config

import (
	"sync"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port                   string `envconfig:"port" default:"8000"`
	LineChannelSecret      string `envconfig:"line_channel_secret"`
	LineChannelToken       string `envconfig:"line_channel_token"`
	GameMinPlayers         int    `envconfig:"game_min_players" default:"5"`
	GameMaxPlayers         int    `envconfig:"game_max_players" default:"10"`
	GameInitializationTime int    `envconfig:"game_initialization_time" default:"180"`
	GameVotingTime         int    `envconfig:"game_voting_time" default:"30"`
	GameVotingRound        int    `envconfig:"game_voting_round" default:"5"`
	GameMissionTime        int    `envconfig:"game_mission_time" default:"30"`
}

var conf Config
var once sync.Once

func Get() Config {
	once.Do(func() {
		envconfig.MustProcess("", &conf)
	})

	return conf
}

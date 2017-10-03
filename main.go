package main

import (
	"log"
	"net/http"

	"github.com/azaky/resistancebot/config"
	r "github.com/azaky/resistancebot/resistance"
	"github.com/line/line-bot-sdk-go/linebot"
)

var conf = config.Get()

func main() {
	lineBot, err := linebot.New(conf.LineChannelSecret, conf.LineChannelToken)
	if err != nil {
		log.Fatalf("Error when creating line bot: %s", err.Error())
	}
	rLineBot := r.NewLineBot(lineBot)

	http.HandleFunc("/line/callback", rLineBot.EventHandler)

	// Setup root endpoint
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"message":"Hello from resistancebot"}`))
	})

	if err := http.ListenAndServe(":"+conf.Port, nil); err != nil {
		log.Fatalf("Error http.ListenAndServe: %s", err.Error())
	}
}

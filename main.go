package main

import (
	"fmt"
	"log"

	r "github.com/azaky/resistancebot/resistance"
)

type handler struct{}

func (h *handler) OnAbort(game *r.Game) {
	log.Printf("[OnAbort] Game aborted\n")
}
func (h *handler) OnStart(game *r.Game, starter *r.Player, err error) {
	if err != nil {
		log.Printf("[OnStart] error: %v\n", err)
	} else if starter == nil {
		log.Printf("[OnStart] Game started due to timeout\n")
		for _, player := range game.Players {
			log.Printf("\t%s: %v\n", player.Name, player.Role)
		}
	} else {
		log.Printf("[OnStart] Game started by %s\n", starter.Name)
		for _, player := range game.Players {
			log.Printf("\t%s: %v\n", player.Name, player.Role)
		}
	}
}
func (h *handler) OnAddPlayer(game *r.Game, player *r.Player, err error) {
	if err != nil {
		log.Printf("[OnAddPlayer] error: %v\n", err)
	} else {
		log.Printf("[OnAddPlayer] %s added to game\n", player.Name)
	}
}
func (h *handler) OnStartPick(game *r.Game, leader *r.Player) {
	log.Printf("[OnStartPick] %s: Please start picking\n", leader.Name)
}
func (h *handler) OnPick(game *r.Game, leader, picked *r.Player, err error) {
	if err != nil {
		log.Printf("[OnPick] error: %v\n", err)
	} else {
		log.Printf("[OnPick] %s picked %s\n", leader.Name, picked.Name)
	}
}
func (h *handler) OnUnpick(game *r.Game, leader, unpicked *r.Player, err error) {
	if err != nil {
		log.Printf("[OnUnpick] error: %v\n", err)
	} else {
		log.Printf("[OnUnpick] %s unpicked %s\n", leader.Name, unpicked.Name)
	}
}
func (h *handler) OnDonePick(game *r.Game, leader *r.Player, err error) {
	if err != nil {
		log.Printf("[OnDonePick] error: %v\n", err)
	} else {
		log.Printf("[OnDonePick] %s done picking\n", leader.Name)
	}
}
func (h *handler) OnStartVoting(game *r.Game, leader *r.Player, picks []*r.Player) {
	log.Printf("[OnStartVoting] Vote for these:\n")
	for _, player := range picks {
		log.Printf("[OnStartVoting] - %s\n", player.Name)
	}
}
func (h *handler) OnVote(game *r.Game, voter *r.Player, vote bool, err error) {
	if err != nil {
		log.Printf("[OnVote] error: %v\n", err)
	} else {
		log.Printf("[OnVote] %s voted %v\n", voter.Name, vote)
	}
}
func (h *handler) OnVotingDone(game *r.Game, votes map[string]bool, majority bool) {
	log.Printf("[OnVotingDone] Votes:\n")
	for voter, vote := range votes {
		log.Printf("[OnVotingDone] - %s votes %v\n", voter, vote)
	}
	log.Printf("[OnVotingDone] Majority: %v\n", majority)
}
func (h *handler) OnStartMission(game *r.Game, members []*r.Player) {
	log.Printf("[OnStartMission] members:\n")
	for _, member := range members {
		log.Printf("[OnStartMission] - %s\n", member.Name)
	}
}
func (h *handler) OnExecuteMission(game *r.Game, player *r.Player, success bool) {
	log.Printf("[OnExecuteMission] %s choose %v for the mission\n", player.Name, success)
}
func (h *handler) OnMissionDone(game *r.Game, mission *r.Mission) {
	log.Printf("[OnMissionDone] Mission %d-th, success: %v", mission.Round, mission.Success)
}
func (h *handler) OnSpyWin(game *r.Game, message string) {
	log.Printf("[OnSpyWin] %s\n", message)
}
func (h *handler) OnResistanceWin(game *r.Game, message string) {
	log.Printf("[OnResistanceWin] %s\n", message)
}

func main() {
	game := r.NewGame("")
	game.EventHandler = &handler{}

	log.Println("init")
	for {
		var cmd string
		fmt.Scanf("%s", &cmd)
		switch cmd {
		case "add":
			log.Println("cmd:add:name")
			var name string
			fmt.Scanf("%s", &name)
			game.AddPlayer(&r.Player{
				Name: name,
				ID:   name,
			})

		case "start":
			log.Println("cmd:start:name")
			var name string
			fmt.Scanf("%s", &name)
			game.Start(name)

		case "pick":
			log.Println("cmd:pick:picker:picked")
			var picker, picked string
			fmt.Scanf("%s%s", &picker, &picked)
			game.Pick(picker, picked)

		case "donepick":
			log.Println("cmd:donepick")
			var leader string
			fmt.Scanf("%s", &leader)
			game.DonePick(leader)

		case "vote":
			log.Println("cmd:vote")
			var player, vote string
			fmt.Scanf("%s%s", &player, &vote)
			game.Vote(player, vote == "yes")

		case "mission":
			log.Println("cmd:mission")
			var player, success string
			fmt.Scanf("%s%s", &player, &success)
			game.ExecuteMission(player, success == "yes")

		default:
			log.Printf("Unknown: [%s]\n", cmd)
		}
	}
}

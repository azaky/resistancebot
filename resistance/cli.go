package resistance

import (
	"fmt"
	"log"
)

type cliHandler struct{}

func (h *cliHandler) OnCreate(game *Game) {
	log.Printf("[OnCreate] Game created\n")
}
func (h *cliHandler) OnAbort(game *Game, player *Player) {
	log.Printf("[OnAbort] Game aborted by %s\n", player.Name)
}
func (h *cliHandler) OnShowPlayers(game *Game, players []*Player, leaderIndex int, over bool) {
	log.Printf("[OnShowPlayers]\n")
}
func (h *cliHandler) OnStart(game *Game, starter *Player, config *Config, err error) {
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
func (h *cliHandler) OnAddPlayer(game *Game, player *Player, err error) {
	if err != nil {
		log.Printf("[OnAddPlayer] error: %v\n", err)
	} else {
		log.Printf("[OnAddPlayer] %s added to game\n", player.Name)
	}
}
func (h *cliHandler) OnStartPick(game *Game, leader *Player) {
	log.Printf("[OnStartPick] %s: Please start picking\n", leader.Name)
}
func (h *cliHandler) OnPick(game *Game, leader, picked *Player, err error) {
	if err != nil {
		log.Printf("[OnPick] error: %v\n", err)
	} else {
		log.Printf("[OnPick] %s picked %s\n", leader.Name, picked.Name)
	}
}
func (h *cliHandler) OnUnpick(game *Game, leader, unpicked *Player, err error) {
	if err != nil {
		log.Printf("[OnUnpick] error: %v\n", err)
	} else {
		log.Printf("[OnUnpick] %s unpicked %s\n", leader.Name, unpicked.Name)
	}
}
func (h *cliHandler) OnDonePick(game *Game, leader *Player, err error) {
	if err != nil {
		log.Printf("[OnDonePick] error: %v\n", err)
	} else {
		log.Printf("[OnDonePick] %s done picking\n", leader.Name)
	}
}
func (h *cliHandler) OnStartVoting(game *Game, leader *Player, picks []*Player) {
	log.Printf("[OnStartVoting] Vote for these:\n")
	for _, player := range picks {
		log.Printf("[OnStartVoting] - %s\n", player.Name)
	}
}
func (h *cliHandler) OnVote(game *Game, voter *Player, vote bool, err error) {
	if err != nil {
		log.Printf("[OnVote] error: %v\n", err)
	} else {
		log.Printf("[OnVote] %s voted %v\n", voter.Name, vote)
	}
}
func (h *cliHandler) OnVotingDone(game *Game, votes map[string]bool, majority bool) {
	log.Printf("[OnVotingDone] Votes:\n")
	for voter, vote := range votes {
		log.Printf("[OnVotingDone] - %s votes %v\n", voter, vote)
	}
	log.Printf("[OnVotingDone] Majority: %v\n", majority)
}
func (h *cliHandler) OnStartMission(game *Game, members []*Player) {
	log.Printf("[OnStartMission] members:\n")
	for _, member := range members {
		log.Printf("[OnStartMission] - %s\n", member.Name)
	}
}
func (h *cliHandler) OnExecuteMission(game *Game, player *Player, success bool) {
	log.Printf("[OnExecuteMission] %s choose %v for the mission\n", player.Name, success)
}
func (h *cliHandler) OnMissionDone(game *Game, mission *Mission) {
	log.Printf("[OnMissionDone] Mission %d-th, success: %v", mission.Round, mission.Success)
}
func (h *cliHandler) OnSpyWin(game *Game, message string) {
	log.Printf("[OnSpyWin] %s\n", message)
}
func (h *cliHandler) OnResistanceWin(game *Game, message string) {
	log.Printf("[OnResistanceWin] %s\n", message)
}

func main() {
	game := NewGame("", &cliHandler{})

	log.Println("init")
	for {
		var cmd string
		fmt.Scanf("%s", &cmd)
		switch cmd {
		case "add":
			log.Println("cmd:add:name")
			var name string
			fmt.Scanf("%s", &name)
			game.AddPlayer(&Player{
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

package resistance

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/azaky/resistancebot/config"
)

type State int

const (
	// STATE_IDLE: no game: uninitialized or already over.
	STATE_IDLE State = iota
	// STATE_INITIALIZED: it is initialized but not started yet.
	// Players can still be added to this game.
	STATE_INITIALIZED
	// STATE_PICK: leader picks member for mission
	STATE_PICK
	// STATE_VOTING: all players are voting who are going for the mission.
	STATE_VOTING
	// STATE_MISSION: voting reached majority, and the members going for the
	// mission are determining mission outcome
	STATE_MISSION
)

type Role int

const (
	ROLE_SPY Role = iota
	ROLE_RESISTANCE
)

type Player struct {
	ID   string
	Name string
	Role
}

type Mission struct {
	Round   int
	Members []*Player
	Success bool
	Votes   map[string]bool
	MinFail int
}

func (m *Mission) HasMember(playerID string) bool {
	for _, member := range m.Members {
		if member.ID == playerID {
			return true
		}
	}
	return false
}

func (m *Mission) Execute() bool {
	fail := 0
	for _, success := range m.Votes {
		if !success {
			fail++
		}
	}
	m.Success = fail < m.MinFail
	return m.Success
}

func (m *Mission) NSuccess() int {
	nsuccess := 0
	for _, vote := range m.Votes {
		if vote {
			nsuccess++
		}
	}
	return nsuccess
}

func (m *Mission) NFail() int {
	nfail := 0
	for _, vote := range m.Votes {
		if !vote {
			nfail++
		}
	}
	return nfail
}

type pickData struct {
	LeaderID string
	PlayerID string
}

type voteData struct {
	PlayerID string
	Vote     bool
}

type executeMissionData struct {
	PlayerID string
	Success  bool
}

type Game struct {
	ID          string
	Players     []*Player
	NPlayers    int
	State       State
	Round       int
	NRound      int
	Picks       map[string]*Player
	VotingRound int
	Votes       map[string]bool
	LeaderIndex int
	Missions    []*Mission

	r *rand.Rand

	cAddPlayer          chan error
	cAddPlayerData      chan *Player
	cStartData          chan string
	cStart              chan error
	cPick               chan error
	cPickData           chan pickData
	cDonePick           chan error
	cDonePickData       chan string
	cVote               chan error
	cVoteData           chan voteData
	cExecuteMission     chan error
	cExecuteMissionData chan executeMissionData

	EventHandler
}

type EventHandler interface {
	OnCreate(*Game)
	OnAbort(*Game)
	OnStart(*Game, *Player, error)
	OnAddPlayer(*Game, *Player, error)
	OnStartPick(*Game, *Player)
	OnPick(*Game, *Player, *Player, error)
	OnUnpick(*Game, *Player, *Player, error)
	OnDonePick(*Game, *Player, error)
	OnStartVoting(*Game, *Player, []*Player)
	OnVote(*Game, *Player, bool, error)
	OnVotingDone(*Game, map[string]bool, bool)
	OnStartMission(*Game, []*Player)
	OnExecuteMission(*Game, *Player, bool)
	OnMissionDone(*Game, *Mission)
	OnSpyWin(*Game, string)
	OnResistanceWin(*Game, string)
}

var games map[string]*Game = make(map[string]*Game)
var lock *sync.RWMutex = &sync.RWMutex{}
var conf config.Config = config.Get()

func NewGame(id string, eventHandler EventHandler) *Game {
	lock.Lock()
	defer lock.Unlock()

	if game, exists := games[id]; exists {
		return game
	}
	game := &Game{
		ID:                  id,
		Players:             []*Player{},
		NPlayers:            0,
		State:               STATE_INITIALIZED,
		Round:               0,
		NRound:              5,
		VotingRound:         0,
		LeaderIndex:         -1,
		Missions:            []*Mission{},
		cAddPlayer:          make(chan error),
		cAddPlayerData:      make(chan *Player),
		cStartData:          make(chan string),
		cStart:              make(chan error),
		cPick:               make(chan error),
		cPickData:           make(chan pickData),
		cDonePick:           make(chan error),
		cDonePickData:       make(chan string),
		cVote:               make(chan error),
		cVoteData:           make(chan voteData),
		cExecuteMission:     make(chan error),
		cExecuteMissionData: make(chan executeMissionData),
		EventHandler:        eventHandler,
		r:                   rand.New(rand.NewSource(time.Now().Unix())),
	}
	games[id] = game
	go game.daemon()
	return game
}

func GameExistsByID(id string) bool {
	lock.RLock()
	defer lock.RUnlock()

	_, exists := games[id]
	return exists
}

func LoadGame(id string) *Game {
	lock.RLock()
	defer lock.RUnlock()

	if game, exists := games[id]; exists {
		return game
	}
	return nil
}

func DeleteGame(id string) bool {
	lock.RLock()
	defer lock.RUnlock()

	_, exists := games[id]
	delete(games, id)
	return exists
}

func (game *Game) daemon() {
	game.OnCreate(game)

	// init:
	var startError error
	initTimer := time.NewTimer(time.Duration(conf.GameInitializationTime) * time.Second)

	for {
		select {
		case newPlayer := <-game.cAddPlayerData:
			log.Println("c:addPlayer")
			game.cAddPlayer <- game.addPlayer(newPlayer)

		case starter := <-game.cStartData:
			log.Println("c:start")
			startError = game.start(starter)
			game.cStart <- startError
			if startError == nil {
				goto start
			} else {
				startError = nil
			}

		case <-initTimer.C:
			log.Println("c:initTimer")
			startError = game.start("timer")
			goto start
		}
	}

start:
	initTimer.Stop()
	if startError != nil {
		game.cleanup()
		go game.OnAbort(game)
		return
	}
	game.Round = 1

pick:
	game.State = STATE_PICK
	game.VotingRound++
	game.LeaderIndex++
	game.LeaderIndex %= game.NPlayers
	game.Picks = make(map[string]*Player)
	go game.OnStartPick(game, game.leader())

	for {
		select {
		case data := <-game.cPickData:
			log.Println("c:pick")
			game.cPick <- game.pick(data)

		case leader := <-game.cDonePickData:
			log.Println("c:donePick")
			errDonePick := game.donePick(leader)
			game.cDonePick <- errDonePick
			if errDonePick == nil {
				goto voting
			}
		}
	}

voting:
	game.State = STATE_VOTING
	game.Votes = make(map[string]bool)
	go game.OnStartVoting(game, game.leader(), game.GetPicks())
	votingTimer := time.NewTimer(time.Duration(conf.GameVotingTime) * time.Second)

	for {
		select {
		case data := <-game.cVoteData:
			game.cVote <- game.vote(data)

		case <-votingTimer.C:
			goto voting_done
		}
	}

voting_done:
	majority := game.calculateVote()
	go game.OnVotingDone(game, game.Votes, majority)
	if majority {
		goto mission
	} else if game.VotingRound == conf.GameVotingRound {
		game.OnSpyWin(game, fmt.Sprintf("Concensus are not reached after %d times voting. Spy won!", conf.GameVotingRound))
		game.cleanup()
		return
	} else {
		goto pick
	}

mission:
	game.State = STATE_MISSION
	game.startMission()
	missionTimer := time.NewTimer(time.Duration(conf.GameMissionTime) * time.Second)

	for {
		select {
		case data := <-game.cExecuteMissionData:
			game.cExecuteMission <- game.executeMission(data)

		case <-missionTimer.C:
			goto mission_done
		}
	}

mission_done:
	currentMission := game.CurrentMission()
	currentMission.Execute()
	game.OnMissionDone(game, currentMission)

	if game.SpyWin() {
		game.OnSpyWin(game, fmt.Sprintf("Spy won!"))
		game.cleanup()
		return
	}
	if game.ResistanceWin() {
		game.OnResistanceWin(game, fmt.Sprintf("Resistance won!"))
		game.cleanup()
		return
	}

	game.Round++
	game.VotingRound = 0
	goto pick
}

func (game *Game) cleanup() {
	game.State = STATE_IDLE
	delete(games, game.ID)
}

func (game *Game) AddPlayer(newPlayer *Player) error {
	if game.State != STATE_INITIALIZED {
		log.Println("g:AddPlayer error")
		return fmt.Errorf("Cannot add player to a running game")
	}
	log.Println("g:AddPlayer chan set")
	game.cAddPlayerData <- newPlayer
	return <-game.cAddPlayer
}

func (game *Game) addPlayer(newPlayer *Player) error {
	if game.NPlayers == conf.GameMaxPlayers {
		err := fmt.Errorf("Cannot add more players")
		go game.OnAddPlayer(game, newPlayer, err)
		return err
	}
	for _, player := range game.Players {
		if player.ID == newPlayer.ID {
			err := fmt.Errorf("%s is already in the game", player.Name)
			go game.OnAddPlayer(game, newPlayer, err)
			return err
		}
	}
	game.NPlayers++
	game.Players = append(game.Players, newPlayer)
	go game.OnAddPlayer(game, newPlayer, nil)
	return nil
}

func (game *Game) Start(starter string) error {
	if game.State != STATE_INITIALIZED {
		return fmt.Errorf("Game already started")
	}
	game.cStartData <- starter
	return <-game.cStart
}

func (game *Game) start(starter string) error {
	p := game.FindPlayerByID(starter)
	if p == nil && starter != "timer" {
		err := fmt.Errorf("Only players in the game can start the game")
		go game.OnStart(game, nil, err)
		return err
	}
	if game.NPlayers < conf.GameMinPlayers || game.NPlayers > conf.GameMaxPlayers {
		err := fmt.Errorf("Number of players should be between %d and %d", conf.GameMinPlayers, conf.GameMaxPlayers)
		go game.OnStart(game, p, err)
		return err
	}
	game.randomizePlayers()
	game.assignRoles()
	go game.OnStart(game, p, nil)
	return nil
}

func (game *Game) randomizePlayers() {
	for i := game.NPlayers - 1; i >= 0; i-- {
		x := game.r.Intn(i + 1)
		temp := game.Players[i]
		game.Players[i] = game.Players[x]
		game.Players[x] = temp
	}
}

func (game *Game) assignRoles() {
	for _, player := range game.Players {
		player.Role = ROLE_RESISTANCE
	}
	// TODO: num of spies
	numSpy := 1
	for numSpy > 0 {
		x := game.r.Intn(game.NPlayers)
		if game.Players[x].Role == ROLE_SPY {
			continue
		}
		game.Players[x].Role = ROLE_SPY
		numSpy--
	}
}

func (game *Game) FindPlayerByID(id string) *Player {
	for _, player := range game.Players {
		if player.ID == id {
			return player
		}
	}
	return nil
}

func (game *Game) leader() *Player {
	return game.Players[game.LeaderIndex]
}

func (game *Game) Pick(leader, picked string) error {
	if game.State != STATE_PICK {
		return fmt.Errorf("Cannot pick now")
	}
	game.cPickData <- pickData{
		LeaderID: leader,
		PlayerID: picked,
	}
	return <-game.cPick
}

func (game *Game) pick(data pickData) error {
	if data.LeaderID != game.leader().ID {
		// do not call OnPick error, just ignore it
		return fmt.Errorf("You have no right to pick")
	}
	if p, ok := game.Picks[data.PlayerID]; ok {
		delete(game.Picks, data.PlayerID)
		go game.OnUnpick(game, game.leader(), p, nil)
		return nil
	}
	p := game.FindPlayerByID(data.PlayerID)
	if p == nil {
		err := fmt.Errorf("Cannot pick players who are not in the game")
		go game.OnPick(game, game.leader(), nil, err)
		return err
	}
	// TODO: num of picks
	game.Picks[data.PlayerID] = p
	go game.OnPick(game, game.leader(), p, nil)
	return nil
}

func (game *Game) DonePick(leader string) error {
	if game.State != STATE_PICK {
		return fmt.Errorf("Cannot done picking now")
	}
	game.cDonePickData <- leader
	return <-game.cDonePick
}

func (game *Game) donePick(leader string) error {
	if leader != game.leader().ID {
		// do not call OnDonePick errror, just ignore it
		return fmt.Errorf("You have no right to finish picking")
	}
	// TODO: num of picks
	go game.OnDonePick(game, game.leader(), nil)
	return nil
}

func (game *Game) Vote(playerID string, vote bool) error {
	if game.State != STATE_VOTING {
		return fmt.Errorf("Cannot vote now")
	}
	game.cVoteData <- voteData{
		PlayerID: playerID,
		Vote:     vote,
	}
	return <-game.cVote
}

func (game *Game) vote(data voteData) error {
	p := game.FindPlayerByID(data.PlayerID)
	if p == nil {
		err := fmt.Errorf("You are not in the game")
		// go game.OnVote(game, nil, data.Vote, err)
		return err
	}
	game.Votes[data.PlayerID] = data.Vote
	go game.OnVote(game, p, data.Vote, nil)
	return nil
}

func (game *Game) calculateVote() bool {
	yes := 0
	for _, vote := range game.Votes {
		if vote {
			yes++
		} else {
			yes--
		}
	}
	yes -= game.NPlayers - len(game.Votes)
	return yes > 0
}

func (game *Game) GetPicks() []*Player {
	var picks []*Player
	for _, pick := range game.Picks {
		picks = append(picks, pick)
	}
	return picks
}

func (game *Game) CurrentMission() *Mission {
	if game.State != STATE_MISSION {
		return nil
	}
	return game.Missions[len(game.Missions)-1]
}

func (game *Game) startMission() {
	missionMembers := game.GetPicks()
	missionVotes := make(map[string]bool)
	// vote success for mission by default
	for _, member := range missionMembers {
		missionVotes[member.ID] = true
	}
	newMission := &Mission{
		Members: missionMembers,
		Round:   game.Round,
		Votes:   missionVotes,
		// TODO
		MinFail: 1,
	}
	game.Missions = append(game.Missions, newMission)
	go game.OnStartMission(game, missionMembers)
}

func (game *Game) ExecuteMission(playerID string, success bool) error {
	if game.State != STATE_MISSION {
		return fmt.Errorf("Cannot run mission now")
	}
	game.cExecuteMissionData <- executeMissionData{
		PlayerID: playerID,
		Success:  success,
	}
	return <-game.cExecuteMission
}

func (game *Game) executeMission(data executeMissionData) error {
	mission := game.CurrentMission()
	if mission == nil {
		return fmt.Errorf("No running mission")
	}
	if !mission.HasMember(data.PlayerID) {
		return fmt.Errorf("You are not part of mission")
	}

	player := game.FindPlayerByID(data.PlayerID)
	// if player is resistance, vote true no matter what, i.e. ignore
	if player.Role == ROLE_RESISTANCE {
		return nil
	}

	mission.Votes[player.ID] = data.Success
	go game.OnExecuteMission(game, player, data.Success)
	return nil
}

func (game *Game) SpyWin() bool {
	success := 0
	fail := 0
	for _, mission := range game.Missions {
		if mission.Success {
			success++
		} else {
			fail++
		}
	}
	success += game.NRound - len(game.Missions)
	return fail > success
}

func (game *Game) ResistanceWin() bool {
	success := 0
	fail := 0
	for _, mission := range game.Missions {
		if mission.Success {
			success++
		} else {
			fail++
		}
	}
	fail += game.NRound - len(game.Missions)
	return success > fail
}

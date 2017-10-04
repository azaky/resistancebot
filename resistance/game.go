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

type Config struct {
	NPlayers  int
	NSpies    int
	NMembers  []int
	NFail     []int
	NOverview []string
	NRounds   int
}

var gameConfigMap = map[int]*Config{
	// For debug
	1: {
		NPlayers:  1,
		NSpies:    1,
		NMembers:  []int{1, 1, 1, 1, 1},
		NFail:     []int{1, 1, 1, 2, 1},
		NOverview: []string{"1", "1", "1", "1*", "1"},
		NRounds:   5,
	},
	5: {
		NPlayers:  5,
		NSpies:    2,
		NMembers:  []int{2, 3, 2, 3, 3},
		NFail:     []int{1, 1, 1, 1, 1},
		NOverview: []string{"2", "3", "2", "3", "3"},
		NRounds:   5,
	},
	6: {
		NPlayers:  6,
		NSpies:    2,
		NMembers:  []int{2, 3, 4, 3, 4},
		NFail:     []int{1, 1, 1, 1, 1},
		NOverview: []string{"2", "3", "4", "3", "4"},
		NRounds:   5,
	},
	7: {
		NPlayers:  7,
		NSpies:    3,
		NMembers:  []int{2, 3, 3, 4, 4},
		NFail:     []int{1, 1, 1, 2, 1},
		NOverview: []string{"2", "3", "3", "4*", "4"},
		NRounds:   5,
	},
	8: {
		NPlayers:  8,
		NSpies:    3,
		NMembers:  []int{3, 4, 4, 5, 5},
		NFail:     []int{1, 1, 1, 2, 1},
		NOverview: []string{"3", "4", "4", "5*", "5"},
		NRounds:   5,
	},
	9: {
		NPlayers:  9,
		NSpies:    3,
		NMembers:  []int{3, 4, 4, 5, 5},
		NFail:     []int{1, 1, 1, 2, 1},
		NOverview: []string{"3", "4", "4", "5*", "5"},
		NRounds:   5,
	},
	10: {
		NPlayers:  10,
		NSpies:    4,
		NMembers:  []int{3, 4, 4, 5, 5},
		NFail:     []int{1, 1, 1, 2, 1},
		NOverview: []string{"3", "4", "4", "5*", "5"},
		NRounds:   5,
	},
}

type Game struct {
	ID          string
	Players     []*Player
	NPlayers    int
	State       State
	Round       int
	Picks       map[string]*Player
	VotingRound int
	Votes       map[string]bool
	LeaderIndex int
	Missions    []*Mission
	Config      *Config

	spyWonByRejection bool

	r *rand.Rand

	cAddPlayer          chan error
	cAddPlayerData      chan *Player
	cAbortData          chan string
	cAbort              chan error
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
	cShowPlayers        chan interface{}
	cShowPlayersData    chan interface{}

	EventHandler
}

type EventHandler interface {
	OnCreate(*Game)
	OnAbort(*Game, *Player)
	OnStart(*Game, *Player, *Config, error)
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
	OnShowPlayers(*Game, []*Player, int, bool)
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
		VotingRound:         0,
		LeaderIndex:         -1,
		Missions:            []*Mission{},
		cAddPlayer:          make(chan error),
		cAddPlayerData:      make(chan *Player),
		cAbortData:          make(chan string),
		cAbort:              make(chan error),
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
		cShowPlayers:        make(chan interface{}),
		cShowPlayersData:    make(chan interface{}),
		EventHandler:        eventHandler,
		r:                   rand.New(rand.NewSource(time.Now().Unix())),
		spyWonByRejection:   false,
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

		case aborter := <-game.cAbortData:
			log.Println("c:abort")
			game.abort(aborter)
			return

		case <-game.cShowPlayersData:
			log.Println("c:showPlayers")
			game.showPlayers()
			game.cShowPlayers <- nil
		}
	}

start:
	initTimer.Stop()
	if startError != nil {
		game.abort("system")
		return
	}
	game.Round = 1

pick:
	time.Sleep(1 * time.Second)
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

		case aborter := <-game.cAbortData:
			log.Println("c:abort")
			game.abort(aborter)
			return

		case <-game.cShowPlayersData:
			log.Println("c:showPlayers")
			game.showPlayers()
			game.cShowPlayers <- nil
		}
	}

voting:
	time.Sleep(1 * time.Second)
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

		case aborter := <-game.cAbortData:
			log.Println("c:abort")
			game.abort(aborter)
			return

		case <-game.cShowPlayersData:
			log.Println("c:showPlayers")
			game.showPlayers()
			game.cShowPlayers <- nil
		}
	}

voting_done:
	time.Sleep(1 * time.Second)
	majority := game.calculateVote()
	votes := make(map[string]bool)
	for id, vote := range game.Votes {
		votes[game.FindPlayerByID(id).Name] = vote
	}
	go game.OnVotingDone(game, votes, majority)
	time.Sleep(2 * time.Second)
	if majority {
		goto mission
	} else if game.VotingRound == conf.GameVotingRound {
		// force spy win
		game.spyWonByRejection = true
		go game.OnSpyWin(game, fmt.Sprintf("Concensus are not reached after %d times voting. Spy won!", conf.GameVotingRound))
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

		case aborter := <-game.cAbortData:
			log.Println("c:abort")
			game.abort(aborter)
			return

		case <-game.cShowPlayersData:
			log.Println("c:showPlayers")
			game.showPlayers()
			game.cShowPlayers <- nil
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
	lock.Lock()
	defer lock.Unlock()
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

func (game *Game) ShowPlayers() {
	game.cShowPlayersData <- nil
	<-game.cShowPlayers
	return
}

func (game *Game) showPlayers() {
	go game.OnShowPlayers(game, game.Players, game.LeaderIndex, game.Over())
	return
}

func (game *Game) Abort(aborter string) error {
	game.cAbortData <- aborter
	return <-game.cAbort
}

func (game *Game) abort(aborter string) error {
	p := game.FindPlayerByID(aborter)
	if p == nil && aborter != "system" {
		return fmt.Errorf("Only players in the game can abort the game")
	}
	game.cleanup()
	go game.OnAbort(game, p)
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
		go game.OnStart(game, nil, nil, err)
		return err
	}
	c, ok := gameConfigMap[game.NPlayers]
	if !ok {
		// if game.NPlayers < conf.GameMinPlayers || game.NPlayers > conf.GameMaxPlayers {
		err := fmt.Errorf("Number of players should be between %d and %d", conf.GameMinPlayers, conf.GameMaxPlayers)
		go game.OnStart(game, p, nil, err)
		return err
	}
	game.Config = c
	game.randomizePlayers()
	game.assignRoles()
	go game.OnStart(game, p, c, nil)
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
	numSpy := game.Config.NSpies
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
		return fmt.Errorf("You have no right to choose")
	}
	if p, ok := game.Picks[data.PlayerID]; ok {
		delete(game.Picks, data.PlayerID)
		go game.OnUnpick(game, game.leader(), p, nil)
		return nil
	}
	p := game.FindPlayerByID(data.PlayerID)
	if p == nil {
		err := fmt.Errorf("Cannot choose players who are not in the game")
		go game.OnPick(game, game.leader(), nil, err)
		return err
	}
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
	npicks := game.Config.NMembers[game.Round-1]
	if len(game.Picks) != npicks {
		err := fmt.Errorf("You must choose exactly %d people", npicks)
		go game.OnPick(game, game.leader(), nil, err)
		return err
	}
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
		MinFail: game.Config.NFail[game.Round-1],
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
		// don't forget to call event handler
		go game.OnExecuteMission(game, player, data.Success)
		return nil
	}

	mission.Votes[player.ID] = data.Success
	go game.OnExecuteMission(game, player, data.Success)
	return nil
}

func (game *Game) SpyWin() bool {
	if game.Config == nil {
		return false
	}
	if game.spyWonByRejection {
		return true
	}
	success := 0
	fail := 0
	for _, mission := range game.Missions {
		if mission.Success {
			success++
		} else {
			fail++
		}
	}
	success += game.Config.NRounds - len(game.Missions)
	return fail > success
}

func (game *Game) ResistanceWin() bool {
	if game.Config == nil {
		return false
	}
	if game.spyWonByRejection {
		return false
	}
	success := 0
	fail := 0
	for _, mission := range game.Missions {
		if mission.Success {
			success++
		} else {
			fail++
		}
	}
	fail += game.Config.NRounds - len(game.Missions)
	return success > fail
}

func (game *Game) Over() bool {
	return game.SpyWin() || game.ResistanceWin()
}

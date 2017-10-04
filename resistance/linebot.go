package resistance

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/azaky/resistancebot/util"
	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/patrickmn/go-cache"
)

type messageHandler func(*linebot.Event, ...string)
type pair struct {
	Key   string
	Value string
}

type LineBot struct {
	client           *linebot.Client
	textPatterns     map[*regexp.Regexp]messageHandler
	postbackPatterns map[*regexp.Regexp]messageHandler
	usersCache       *cache.Cache
}

func NewLineBot(client *linebot.Client) *LineBot {
	b := &LineBot{
		client:           client,
		textPatterns:     make(map[*regexp.Regexp]messageHandler),
		postbackPatterns: make(map[*regexp.Regexp]messageHandler),
		usersCache:       cache.New(30*time.Minute, 60*time.Minute),
	}
	b.registerTextPattern(`^\s*\.echo\s*(.*)$`, b.echo)
	b.registerTextPattern(`^\s*\.create\s*$`, b.createGame)
	b.registerTextPattern(`^\s*\.abort\s*$`, b.abortGame)
	b.registerTextPattern(`^\s*\.join\s*$`, b.joinGame)
	b.registerTextPattern(`^\s*\.players?\s*$`, b.showPlayers)
	b.registerTextPattern(`^\s*\.start\s*$`, b.startGame)
	b.registerPostbackPattern(`^\.join$`, b.joinGame)
	b.registerPostbackPattern(`^\.pick:(\S+)$`, b.pick)
	b.registerPostbackPattern(`^\.donepick$`, b.donepick)
	b.registerPostbackPattern(`^\.vote:(\S+):(\S+)$`, b.vote)
	b.registerPostbackPattern(`^\.executemission:(\S+):(\S+)$`, b.executeMission)
	return b
}

func (b *LineBot) registerTextPattern(regex string, handler messageHandler) {
	r, err := regexp.Compile(regex)
	if err != nil {
		b.log("Error registering text pattern: %s", err.Error())
		return
	}
	b.textPatterns[r] = handler
}

func (b *LineBot) registerPostbackPattern(regex string, handler messageHandler) {
	r, err := regexp.Compile(regex)
	if err != nil {
		b.log("Error registering postback pattern: %s", err.Error())
		return
	}
	b.postbackPatterns[r] = handler
}

func (b *LineBot) log(format string, args ...interface{}) {
	log.Printf("[LINE] "+format, args...)
}

func (b *LineBot) reply(event *linebot.Event, messages ...string) error {
	var lineMessages []linebot.Message
	for _, message := range messages {
		lineMessages = append(lineMessages, linebot.NewTextMessage(message))
	}
	_, err := b.client.ReplyMessage(event.ReplyToken, lineMessages...).Do()
	if err != nil {
		b.log("Error replying to %+v: %s", event.Source, err.Error())
	}
	return err
}

func (b *LineBot) replyPostback(event *linebot.Event, title, text string, data ...pair) error {
	var actions []linebot.TemplateAction
	for _, p := range data {
		key := p.Key
		if len(key) > 20 {
			key = key[:20]
		}
		actions = append(actions, linebot.NewPostbackTemplateAction(key, p.Value, ""))
	}
	var messages []linebot.Message
	// Send postback every 4 buttons
	for i := 0; i < len(actions); i += 4 {
		if i+4 > len(actions) {
			messages = append(messages, linebot.NewTemplateMessage(title,
				linebot.NewButtonsTemplate("", title, text, actions[i:]...)))
		} else {
			messages = append(messages, linebot.NewTemplateMessage(title,
				linebot.NewButtonsTemplate("", title, text, actions[i:i+4]...)))
		}
	}
	_, err := b.client.ReplyMessage(event.ReplyToken, messages...).Do()
	if err != nil {
		b.log("Error replying postback to %+v: %s", event.Source, err.Error())
	}
	return err
}

func (b *LineBot) replyRaw(event *linebot.Event, lineMessages ...linebot.Message) error {
	_, err := b.client.ReplyMessage(event.ReplyToken, lineMessages...).Do()
	if err != nil {
		b.log("Error replying to %+v: %s", event.Source, err.Error())
	}
	return err
}

func (b *LineBot) push(to string, messages ...string) error {
	var lineMessages []linebot.Message
	for _, message := range messages {
		lineMessages = append(lineMessages, linebot.NewTextMessage(message))
	}
	_, err := b.client.PushMessage(to, lineMessages...).Do()
	if err != nil {
		b.log("Error pushing to %s: %s", to, err.Error())
	}
	return err
}

func (b *LineBot) pushPostback(to string, title, text string, data ...pair) error {
	var actions []linebot.TemplateAction
	for _, p := range data {
		key := p.Key
		if len(key) > 20 {
			key = key[:20]
		}
		actions = append(actions, linebot.NewPostbackTemplateAction(key, p.Value, ""))
	}
	var messages []linebot.Message
	// Send postback every 4 buttons
	for i := 0; i < len(actions); i += 4 {
		if i+4 > len(actions) {
			messages = append(messages, linebot.NewTemplateMessage(title,
				linebot.NewButtonsTemplate("", title, text, actions[i:]...)))
		} else {
			messages = append(messages, linebot.NewTemplateMessage(title,
				linebot.NewButtonsTemplate("", title, text, actions[i:i+4]...)))
		}
	}
	_, err := b.client.PushMessage(to, messages...).Do()
	if err != nil {
		b.log("Error pushing postback to %+v: %s", to, err.Error())
	}
	return err
}

func (b *LineBot) pushTextback(to string, title, text string, data ...pair) error {
	var actions []linebot.TemplateAction
	for _, p := range data {
		key := p.Key
		if len(key) > 20 {
			key = key[:20]
		}
		actions = append(actions, linebot.NewPostbackTemplateAction(key, "?", p.Value))
	}
	var messages []linebot.Message
	// Send postback every 4 buttons
	for i := 0; i < len(actions); i += 4 {
		if i+4 > len(actions) {
			messages = append(messages, linebot.NewTemplateMessage(title,
				linebot.NewButtonsTemplate("", title, text, actions[i:]...)))
		} else {
			messages = append(messages, linebot.NewTemplateMessage(title,
				linebot.NewButtonsTemplate("", title, text, actions[i:i+4]...)))
		}
	}
	_, err := b.client.PushMessage(to, messages...).Do()
	if err != nil {
		b.log("Error pushing postback to %+v: %s", to, err.Error())
	}
	return err
}

func (b *LineBot) warnIncompatibility(event *linebot.Event) error {
	return b.reply(event, "Please add me and upgrade line to v7.5.0")
}

func (b *LineBot) getUserInfo(source *linebot.EventSource) (*linebot.UserProfileResponse, error) {
	if source.UserID == "" {
		return nil, fmt.Errorf("UserID not found")
	}
	// load cache
	cached, exists := b.usersCache.Get(source.UserID)
	if exists {
		user, ok := cached.(*linebot.UserProfileResponse)
		if ok {
			return user, nil
		}
		// Purge bad data from cache
		b.usersCache.Delete(source.UserID)
	}

	// get info from line
	res, err := b.client.GetProfile(source.UserID).Do()
	if err != nil {
		return nil, err
	}

	b.usersCache.Set(source.UserID, res, cache.DefaultExpiration)
	return res, nil
}

func (b *LineBot) EventHandler(w http.ResponseWriter, req *http.Request) {
	events, err := b.client.ParseRequest(req)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			w.WriteHeader(400)
		} else {
			w.WriteHeader(500)
		}
		return
	}

	for _, event := range events {
		b.log("[EVENT][%s] Source: %#v", event.Type, event.Source)
		switch event.Type {

		case linebot.EventTypeJoin:
			go b.handleJoin(event)

		case linebot.EventTypeFollow:
			go b.handleFollow(event)

		case linebot.EventTypeLeave:
			fallthrough
		case linebot.EventTypeUnfollow:
			go b.handleUnfollow(event)

		case linebot.EventTypeMessage:
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				go b.handleTextMessage(event, message)
			}

		case linebot.EventTypePostback:
			go b.handlePostback(event, event.Postback)
		}
	}
}

func (b *LineBot) handleJoin(event *linebot.Event) {
	b.reply(event, `Thanks for adding me! Type ".create" to start a new game.`)
}

func (b *LineBot) handleFollow(event *linebot.Event) {
	b.reply(event, `Thanks for adding me! Invite me to group chats to play.`)
}

func (b *LineBot) handleUnfollow(event *linebot.Event) {
	// do some cleanup perhaps (?)
}

func (b *LineBot) handleTextMessage(event *linebot.Event, message *linebot.TextMessage) {
	b.log("[MESSAGE] %+v: %s", event.Source, message.Text)
	for regex, handler := range b.textPatterns {
		matches := regex.FindStringSubmatch(message.Text)
		if matches != nil {
			handler(event, matches...)
			return
		}
	}
}

func (b *LineBot) handlePostback(event *linebot.Event, postback *linebot.Postback) {
	b.log("[POSTBACK] %+v: %s", event.Source, postback.Data)
	for regex, handler := range b.postbackPatterns {
		matches := regex.FindStringSubmatch(postback.Data)
		if matches != nil {
			handler(event, matches...)
			return
		}
	}
}

func (b *LineBot) echo(event *linebot.Event, args ...string) {
	b.reply(event, args[1])
}

func (b *LineBot) getPlayerFromUser(user *linebot.UserProfileResponse) *Player {
	return &Player{
		ID:   user.UserID,
		Name: user.DisplayName,
	}
}

func (b *LineBot) createGame(event *linebot.Event, args ...string) {
	if event.Source.Type == linebot.EventSourceTypeUser {
		b.reply(event, "Cannot create game here. Create one in group/multichat.")
		return
	}

	user, err := b.getUserInfo(event.Source)
	if err != nil {
		b.warnIncompatibility(event)
		return
	}

	id := util.GetGameID(event.Source)

	if GameExistsByID(id) {
		b.reply(event, "A game is already created.")
		return
	}

	game := NewGame(id, b)
	game.AddPlayer(b.getPlayerFromUser(user))
}

func (b *LineBot) joinGame(event *linebot.Event, args ...string) {
	if event.Source.Type == linebot.EventSourceTypeUser {
		// don't bother reply
		return
	}

	user, err := b.getUserInfo(event.Source)
	if err != nil {
		b.warnIncompatibility(event)
		return
	}

	id := util.GetGameID(event.Source)

	if !GameExistsByID(id) {
		// Auto-create game if not exist
		b.reply(event, `No game to join. Creating a new game ...`)
		game := NewGame(id, b)
		game.AddPlayer(b.getPlayerFromUser(user))
		return
	}

	game := LoadGame(id)
	game.AddPlayer(b.getPlayerFromUser(user))
}

func (b *LineBot) startGame(event *linebot.Event, args ...string) {
	if event.Source.Type == linebot.EventSourceTypeUser {
		// don't bother reply
		return
	}

	user, err := b.getUserInfo(event.Source)
	if err != nil {
		b.warnIncompatibility(event)
		return
	}

	id := util.GetGameID(event.Source)

	if !GameExistsByID(id) {
		b.reply(event, `No game is created. Type ".create" to create a new game`)
		return
	}

	game := LoadGame(id)
	game.Start(user.UserID)
}

func (b *LineBot) abortGame(event *linebot.Event, args ...string) {
	if event.Source.Type == linebot.EventSourceTypeUser {
		// don't bother reply
		return
	}

	user, err := b.getUserInfo(event.Source)
	if err != nil {
		b.warnIncompatibility(event)
		return
	}

	id := util.GetGameID(event.Source)

	if !GameExistsByID(id) {
		return
	}

	game := LoadGame(id)
	game.Abort(user.UserID)
}

func (b *LineBot) showPlayers(event *linebot.Event, args ...string) {
	if event.Source.Type == linebot.EventSourceTypeUser {
		// don't bother reply
		return
	}

	id := util.GetGameID(event.Source)

	if !GameExistsByID(id) {
		return
	}

	game := LoadGame(id)
	game.ShowPlayers()
}

func (b *LineBot) pick(event *linebot.Event, args ...string) {
	id := util.GetGameID(event.Source)

	if !GameExistsByID(id) {
		return
	}

	game := LoadGame(id)
	game.Pick(event.Source.UserID, args[1])
}

func (b *LineBot) donepick(event *linebot.Event, args ...string) {
	id := util.GetGameID(event.Source)

	if !GameExistsByID(id) {
		return
	}

	game := LoadGame(id)
	game.DonePick(event.Source.UserID)
}

func (b *LineBot) vote(event *linebot.Event, args ...string) {
	if len(args) < 3 {
		return
	}
	id := args[1]
	vote := args[2] == "ok"

	if !GameExistsByID(id) {
		return
	}

	game := LoadGame(id)
	game.Vote(event.Source.UserID, vote)
}

func (b *LineBot) executeMission(event *linebot.Event, args ...string) {
	if len(args) < 3 {
		return
	}
	id := args[1]
	vote := args[2] == "success"

	if !GameExistsByID(id) {
		return
	}

	game := LoadGame(id)
	game.ExecuteMission(event.Source.UserID, vote)
}

func (b *LineBot) OnCreate(game *Game) {
	// Create a postback button to join
	b.pushTextback(game.ID,
		"New Game",
		fmt.Sprintf("Game will be started in %d seconds. Commands:", conf.GameInitializationTime),
		pair{"Join", ".join"},
		pair{"Start", ".start"},
		pair{"Abort", ".abort"},
		pair{"Show Players", ".players"},
	)
	// b.push(game.ID, `Commands:
	// .join: Join game
	// .start: Start game
	// .abort: Abort game
	// .players: Show players`)
}

func (b *LineBot) OnAbort(game *Game, aborter *Player) {
	if aborter != nil {
		b.push(game.ID, fmt.Sprintf("Game aborted by %s.", aborter.Name))
	} else {
		b.push(game.ID, "Game aborted.")
	}
}

func (b *LineBot) OnStart(game *Game, starter *Player, c *Config, err error) {
	if err != nil {
		b.push(game.ID, err.Error())
		return
	}

	var messages []string
	if starter == nil {
		messages = append(messages, `Game started. Check your PM to find out your role.`)
	} else {
		messages = append(messages, fmt.Sprintf(`Game started by %s. Check your PM to find out your role.`, starter.Name))
	}

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("There are %d players, and %d among them are spies.", c.NPlayers, c.NSpies))
	buffer.WriteString(fmt.Sprintf("\n\nThere are %d missions, and each mission requires %s members each", c.NRounds, strings.Join(c.NOverview, ", ")))
	buffer.WriteString("\n\n(* means that the mission requires at least 2 fails to fail it)")
	messages = append(messages, buffer.String())

	b.push(game.ID, messages...)

	for _, player := range game.Players {
		if player.Role == ROLE_RESISTANCE {
			b.push(player.ID, fmt.Sprintf("%s, you are a Resistance", player.Name))
		} else {
			var spies []string
			for _, spy := range game.Players {
				if spy.Role == ROLE_SPY && spy.ID != player.ID {
					spies = append(spies, spy.Name)
				}
			}
			b.push(player.ID, fmt.Sprintf("%s, you are a Spy. Other spies are %s", player.Name, strings.Join(spies, ", ")))
		}
	}
}

func (b *LineBot) OnAddPlayer(game *Game, player *Player, err error) {
	if err != nil {
		b.push(game.ID, err.Error())
	} else {
		b.push(game.ID, fmt.Sprintf("%s is added to the game.", player.Name))
	}
}

func (b *LineBot) OnShowPlayers(game *Game, players []*Player, leaderIndex int, over bool) {
	var buffer bytes.Buffer
	if !over {
		buffer.WriteString("Players:")
		for i, player := range players {
			if i == leaderIndex {
				buffer.WriteString(fmt.Sprintf("\n- %s (leader)", player.Name))
			} else {
				buffer.WriteString(fmt.Sprintf("\n- %s", player.Name))
			}
		}
	} else {
		buffer.WriteString("Here are players and their roles:")
		for _, player := range players {
			if player.Role == ROLE_RESISTANCE {
				buffer.WriteString(fmt.Sprintf("\n- %s (Resistance)", player.Name))
			} else {
				buffer.WriteString(fmt.Sprintf("\n- %s (Spy)", player.Name))
			}
		}
	}
	b.push(game.ID, buffer.String())
}

func (b *LineBot) OnStartPick(game *Game, leader *Player) {
	var buttons []pair
	for _, player := range game.Players {
		if leader.ID == player.ID {
			buttons = append(buttons, pair{"(*) " + player.Name, ".pick:" + player.ID})
		} else {
			buttons = append(buttons, pair{player.Name, ".pick:" + player.ID})
		}
	}
	buttons = append(buttons, pair{"Done", ".donepick"})
	b.pushPostback(game.ID,
		fmt.Sprintf("Mission #%d, Leader #%d", game.Round, game.VotingRound),
		fmt.Sprintf("This mission needs %d people", game.Config.NMembers[game.Round-1]),
		buttons...)
	b.push(game.ID,
		fmt.Sprintf("Only for %s: choose people you trust the most to go for the mission. This mission needs %s people. Choose them wisely.",
			leader.Name, game.Config.NOverview[game.Round-1]))
}

func (b *LineBot) OnPick(game *Game, leader *Player, picked *Player, err error) {
	if err != nil {
		b.push(game.ID, err.Error())
		return
	}

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%s chooses %s.\n\nCurrent mission members (need %s people):", leader.Name, picked.Name, game.Config.NOverview[game.Round-1]))
	for _, player := range game.Picks {
		buffer.WriteString(fmt.Sprintf("\n- %s", player.Name))
	}
	b.push(game.ID, buffer.String())
}

func (b *LineBot) OnUnpick(game *Game, leader *Player, unpicked *Player, err error) {
	if err != nil {
		b.push(game.ID, err.Error())
		return
	}

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%s cancels %s.\n\nCurrent mission members (need %s people):", leader.Name, unpicked.Name, game.Config.NOverview[game.Round-1]))
	for _, player := range game.Picks {
		buffer.WriteString(fmt.Sprintf("\n- %s", player.Name))
	}
	if len(game.Picks) == 0 {
		buffer.WriteString(fmt.Sprintf("\n(no members yet)"))
	}
	b.push(game.ID, buffer.String())
}

func (b *LineBot) OnDonePick(game *Game, leader *Player, err error) {
	if err != nil {
		b.push(game.ID, err.Error())
		return
	}
}

func (b *LineBot) OnStartVoting(game *Game, leader *Player, members []*Player) {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("%s has chosen the following people:", leader.Name))
	for _, player := range members {
		if leader.ID == player.ID {
			buffer.WriteString(fmt.Sprintf("\n- %s (leader)", player.Name))
		} else {
			buffer.WriteString(fmt.Sprintf("\n- %s", player.Name))
		}
	}
	buffer.WriteString(fmt.Sprintf("\n\nYou have %d seconds to approve/reject the choice. If you don't vote, it will count as a Reject.", conf.GameVotingTime))
	b.push(game.ID, buffer.String())

	b.pushPostback(game.ID,
		fmt.Sprintf("Mission #%d, Leader #%d", game.Round, game.VotingRound),
		"Vote here",
		pair{"Approve", ".vote:" + game.ID + ":ok"},
		pair{"Reject", ".vote:" + game.ID + ":no"},
	)
}

func (b *LineBot) OnVote(game *Game, player *Player, ok bool, err error) {
	if err != nil {
		b.push(player.ID, err.Error())
		return
	}

	var vote string
	if ok {
		vote = "Approve"
	} else {
		vote = "Reject"
	}
	b.push(player.ID, fmt.Sprintf("You vote %s. You can always change this before the time runs out.", vote))
}

func (b *LineBot) OnVotingDone(game *Game, votes map[string]bool, majority bool) {
	var buffer bytes.Buffer
	buffer.WriteString("Here are the voting result:")
	for voter, vote := range votes {
		if vote {
			buffer.WriteString(fmt.Sprintf("\n- %s voted Approve", voter))
		} else {
			buffer.WriteString(fmt.Sprintf("\n- %s voted Reject", voter))
		}
	}
	if len(votes) == 0 {
		buffer.WriteString("\n(no one votes)")
	} else if len(votes) < game.NPlayers {
		buffer.WriteString(fmt.Sprintf("\n(The rest %d people did not vote)", game.NPlayers-len(votes)))
	}
	if majority {
		buffer.WriteString("\n\nMajority is reached. Mission will be executed.")
	} else {
		if game.VotingRound == conf.GameVotingRound {
			buffer.WriteString("\n\nMajority is not reached.")
		} else {
			buffer.WriteString("\n\nMajority is not reached. Moving on to the next leader.")
		}
	}
	b.push(game.ID, buffer.String())
}

func (b *LineBot) OnStartMission(game *Game, members []*Player) {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("Mission #%d", game.Round))
	buffer.WriteString("\n\nMembers:")
	for _, member := range members {
		buffer.WriteString(fmt.Sprintf("\n- %s", member.Name))
	}
	buffer.WriteString(fmt.Sprintf("\n\nFor all members, check your PM to succeed/fail this mission. If you do not choose, it will be considered as a success. You have %d seconds.", conf.GameMissionTime))
	b.push(game.ID, buffer.String())

	for _, member := range members {
		b.pushPostback(member.ID,
			fmt.Sprintf("Mission #%d", game.Round),
			fmt.Sprintf("Choose the outcome of this mission. You have %d seconds", conf.GameMissionTime),
			pair{"Success", ".executemission:" + game.ID + ":success"},
			pair{"Fail", ".executemission:" + game.ID + ":fail"},
		)
	}
}

func (b *LineBot) OnExecuteMission(game *Game, player *Player, success bool) {
	if player.Role == ROLE_RESISTANCE {
		if success {
			b.push(player.ID, "You choose to succeed this mission.")
		} else {
			b.push(player.ID, "You cannot fail this mission as you are a Resistance")
		}
	} else {
		if success {
			b.push(player.ID, "You choose to succeed this mission.")
		} else {
			b.push(player.ID, "You choose to fail this mission")
		}
	}
}

func (b *LineBot) OnMissionDone(game *Game, mission *Mission) {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("Mission #%d", game.Round))
	buffer.WriteString("\n\nMembers:")
	for _, member := range mission.Members {
		buffer.WriteString(fmt.Sprintf("\n- %s", member.Name))
	}
	if mission.Success {
		buffer.WriteString("\n\nOutcome: Success")
	} else {
		buffer.WriteString("\n\nOutcome: Fail")
	}
	buffer.WriteString(fmt.Sprintf(" (%d success, %d fail)", mission.NSuccess(), mission.NFail()))
	b.push(game.ID, buffer.String())
}

func (b *LineBot) OnSpyWin(game *Game, message string) {
	b.push(game.ID, message)
	b.OnShowPlayers(game, game.Players, -1, true)
}

func (b *LineBot) OnResistanceWin(game *Game, message string) {
	b.push(game.ID, message)
	b.OnShowPlayers(game, game.Players, -1, true)
}

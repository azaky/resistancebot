package resistance

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/azaky/resistancebot/util"
	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/patrickmn/go-cache"
)

type messageHandler func(linebot.Event, ...string)

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
	b.registerTextPattern(`^\s*.echo\s*(.*)$`, b.echo)
	b.registerTextPattern(`^\s*.create\s*$`, b.createGame)
	b.registerTextPattern(`^\s*.join\s*$`, b.joinGame)
	b.registerPostbackPattern(`^\s*.join\s*$`, b.joinGame)
	b.registerTextPattern(`^\s*.start\s*$`, b.startGame)
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

func (b *LineBot) reply(event linebot.Event, messages ...string) error {
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

func (b *LineBot) replyPostback(event linebot.Event, title, text string, data map[string]string) error {
	var actions []linebot.TemplateAction
	for key, value := range data {
		actions = append(actions, linebot.NewPostbackTemplateAction(key, value, ""))
	}
	message := linebot.NewTemplateMessage(title, linebot.NewButtonsTemplate("", title, text, actions...))
	_, err := b.client.ReplyMessage(event.ReplyToken, message).Do()
	if err != nil {
		b.log("Error replying postback to %+v: %s", event.Source, err.Error())
	}
	return err
}

func (b *LineBot) replyRaw(event linebot.Event, lineMessages ...linebot.Message) error {
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

func (b *LineBot) pushPostback(to string, title, text string, data map[string]string) error {
	var actions []linebot.TemplateAction
	for key, value := range data {
		actions = append(actions, linebot.NewPostbackTemplateAction(key, value, ""))
	}
	message := linebot.NewTemplateMessage(title, linebot.NewButtonsTemplate("", title, text, actions...))
	_, err := b.client.PushMessage(to, message).Do()
	if err != nil {
		b.log("Error pushing postback to %+v: %s", to, err.Error())
	}
	return err
}

func (b *LineBot) warnIncompatibility(event linebot.Event) error {
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
			b.handleJoin(event)

		case linebot.EventTypeFollow:
			b.handleFollow(event)

		case linebot.EventTypeLeave:
			fallthrough
		case linebot.EventTypeUnfollow:
			b.handleUnfollow(event)

		case linebot.EventTypeMessage:
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				b.handleTextMessage(event, message)
			}

		case linebot.EventTypePostback:
			b.handlePostback(event, event.Postback)
		}
	}
}

func (b *LineBot) handleJoin(event linebot.Event) {
	b.reply(event, `Thanks for adding me! Type ".create" to start a new game.`)
}

func (b *LineBot) handleFollow(event linebot.Event) {
	b.reply(event, `Thanks for adding me! Invite me to group chats to play.`)
}

func (b *LineBot) handleUnfollow(event linebot.Event) {
	// do some cleanup perhaps (?)
}

func (b *LineBot) handleTextMessage(event linebot.Event, message *linebot.TextMessage) {
	b.log("[MESSAGE] %+v: %s", event.Source, message.Text)
	for regex, handler := range b.textPatterns {
		matches := regex.FindStringSubmatch(message.Text)
		if matches != nil {
			handler(event, matches...)
			return
		}
	}
}

func (b *LineBot) handlePostback(event linebot.Event, postback *linebot.Postback) {
	for regex, handler := range b.textPatterns {
		matches := regex.FindStringSubmatch(postback.Data)
		if matches != nil {
			handler(event, matches...)
			return
		}
	}
}

func (b *LineBot) echo(event linebot.Event, args ...string) {
	b.reply(event, args[1])
}

func (b *LineBot) getPlayerFromUser(user *linebot.UserProfileResponse) *Player {
	return &Player{
		ID:   user.UserID,
		Name: user.DisplayName,
	}
}

func (b *LineBot) createGame(event linebot.Event, args ...string) {
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

	if !GameExistsByID(id) {
		b.reply(event, "A game is already created.")
		return
	}

	game := NewGame(id, b)
	game.AddPlayer(b.getPlayerFromUser(user))
}

func (b *LineBot) joinGame(event linebot.Event, args ...string) {
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

func (b *LineBot) startGame(event linebot.Event, args ...string) {
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

func (b *LineBot) OnCreate(game *Game) {
	// Create a postback button to join
	b.pushPostback(game.ID, "New Game", `Click here or type ".join" to join the game`, map[string]string{
		"Join": ".join",
	})
}

func (b *LineBot) OnAbort(*Game)                             {}
func (b *LineBot) OnStart(*Game, *Player, error)             {}
func (b *LineBot) OnAddPlayer(*Game, *Player, error)         {}
func (b *LineBot) OnStartPick(*Game, *Player)                {}
func (b *LineBot) OnPick(*Game, *Player, *Player, error)     {}
func (b *LineBot) OnUnpick(*Game, *Player, *Player, error)   {}
func (b *LineBot) OnDonePick(*Game, *Player, error)          {}
func (b *LineBot) OnStartVoting(*Game, *Player, []*Player)   {}
func (b *LineBot) OnVote(*Game, *Player, bool, error)        {}
func (b *LineBot) OnVotingDone(*Game, map[string]bool, bool) {}
func (b *LineBot) OnStartMission(*Game, []*Player)           {}
func (b *LineBot) OnExecuteMission(*Game, *Player, bool)     {}
func (b *LineBot) OnMissionDone(*Game, *Mission)             {}
func (b *LineBot) OnSpyWin(*Game, string)                    {}
func (b *LineBot) OnResistanceWin(*Game, string)             {}

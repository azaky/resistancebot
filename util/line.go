package util

import (
	"fmt"

	"github.com/line/line-bot-sdk-go/linebot"
)

func GetGameID(source *linebot.EventSource) string {
	return fmt.Sprintf("%s%s", source.GroupID, source.RoomID)
}

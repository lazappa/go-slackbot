// Package slackbot hopes to ease development of Slack bots by adding helpful
// methods and a mux-router style interface to the github.com/nlopes/slack package.
//
// Incoming Slack RTM events are mapped to a handler in the following form:
// 	bot.Hear("(?i)how are you(.*)").MessageHandler(HowAreYouHandler)
//
// The package adds Reply and ReplyWithAttachments methods:
//	func HowAreYouHandler(ctx context.Context, bot *slackbot.Bot, evt *slack.MessageEvent) {
// 		bot.Reply(evt, "A bit tired. You get it? A bit?", slackbot.WithTyping)
//	}
//
//	func HowAreYouAttachmentsHandler(ctx context.Context, bot *slackbot.Bot, evt *slack.MessageEvent) {
// 		txt := "Beep Beep Boop is a ridiculously simple hosting platform for your Slackbots."
// 		attachment := slack.Attachment{
// 			Pretext:   "We bring bots to life. :sunglasses: :thumbsup:",
// 			Title:     "Host, deploy and share your bot in seconds.",
// 			TitleLink: "https://beepboophq.com/",
// 			Text:      txt,
// 			Fallback:  txt,
// 			ImageURL:  "https://storage.googleapis.com/beepboophq/_assets/bot-1.22f6fb.png",
// 			Color:     "#7CD197",
// 		}
//
//		attachments := []slack.Attachment{attachment}
//		bot.ReplyWithAttachments(evt, attachments, slackbot.WithTyping)
//	}
//
// The slackbot package exposes  github.com/nlopes/slack RTM and Client objects
// enabling a consumer to interact with the lower level package directly:
// 	func HowAreYouHandler(ctx context.Context, bot *slackbot.Bot, evt *slack.MessageEvent) {
// 		bot.RTM.NewOutgoingMessage("Hello", "#random")
// 	}
//
//
// Project home and samples: https://github.com/BeepBoopHQ/go-slackbot
package slackbot

import (
	"fmt"
	"time"

	"context"

	"github.com/slack-go/slack"
)

const (
	WithTyping    bool = true
	WithoutTyping bool = false

	maxTypingSleepMs time.Duration = time.Millisecond * 2000
)

// New constructs a new Bot using the slackToken to authorize against the Slack service.
func New(slackToken string) *Bot {
	b := &Bot{Client: slack.New(slackToken)}
	return b
}

// Bot contains properties of the Slack bot
type Bot struct {
	SimpleRouter
	// Routes to be matched, in order.
	routes []*Route
	// Slack UserID of the bot UserID
	botUserID string
	// Slack UserName of the bot UserName
	botUserName string
	// Slack API
	Client *slack.Client
	RTM    *slack.RTM
}

// Run listens for incoming slack RTM events, matching them to an appropriate handler.
func (b *Bot) Run() {
	b.RTM = b.Client.NewRTM()
	go b.RTM.ManageConnection()
LOOP:
	for {
		select {
		case msg := <-b.RTM.IncomingEvents:
			ctx := context.Background()
			ctx = AddBotToContext(ctx, b)
			switch ev := msg.Data.(type) {
			case *slack.ConnectedEvent:
				fmt.Printf("Connected: %#v, count: %d\n", ev.Info.User, ev.ConnectionCount)
				b.setBotID(ev.Info.User.ID)
				b.setBotName(ev.Info.User.Name)
			case *slack.MessageEvent:
				// ignore messages from the current user, the bot user
				// Slack likes to change if it's ID or Name
				if b.botUserID == ev.User || b.botUserName == ev.User {
					continue LOOP
				}

				ctx = AddMessageToContext(ctx, ev)
				var match RouteMatch
				if matched, ctx := b.Match(ctx, &match); matched {
					match.Handler(ctx)
				}

			case *slack.InvalidAuthEvent:
				fmt.Printf("Invalid credentials\n")
				break LOOP

			case error:
				fmt.Printf("Error %T: %s\n", ev, ev.Error())

			default:
				// Ignore other events..
				// fmt.Printf("Unexpected: %v\n", msg.Data)
			}
		}
	}
}

// Reply replies to a message event with a simple message.
func (b *Bot) Reply(evt *slack.MessageEvent, msg string, typing bool) {
	if typing {
		b.Type(evt, msg)
	}
	b.RTM.SendMessage(b.RTM.NewOutgoingMessage(msg, evt.Channel))
}

// ReplyPost replies to a message event with a simple message using Slack API.
func (b *Bot) ReplyPost(evt *slack.MessageEvent, msg string, typing bool) {
	if typing {
		b.Type(evt, msg)
	}
	postParams := slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{
		AsUser:    true,
		Username:  b.BotUserID(),
		LinkNames: 1,
		UnfurlLinks: true,
		UnfurlMedia: true,
	})
	_, _, _ = b.Client.PostMessage(evt.Channel, slack.MsgOptionText(msg, false), postParams)
}

// ReplyWithAttachments replys to a message event with a Slack Attachments message.
func (b *Bot) ReplyWithAttachments(evt *slack.MessageEvent, attachments []slack.Attachment, typing bool) {
	if typing {
		b.Type(evt, "attachment")
	}
	postParams := slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{
		AsUser:    true,
		Username:  b.botUserID,
		LinkNames: 1,
	})
	_, _, _ = b.Client.PostMessage(evt.Msg.Channel, slack.MsgOptionAttachments(attachments...), postParams)
}

// Type sends a typing message and simulates delay (max 2000ms) based on message size.
func (b *Bot) Type(evt *slack.MessageEvent, msg interface{}) {
	msgLen := msgLen(msg)

	sleepDuration := time.Minute * time.Duration(msgLen) / 3000
	if sleepDuration > maxTypingSleepMs {
		sleepDuration = maxTypingSleepMs
	}

	b.RTM.SendMessage(b.RTM.NewTypingMessage(evt.Channel))
	time.Sleep(sleepDuration)
}

// BotUserID Fetch the botUserID.
func (b *Bot) BotUserID() string {
	return b.botUserID
}

// BotUserName Fetch the botUserName.
func (b *Bot) BotUserName() string {
	return b.botUserName
}

func (b *Bot) setBotID(ID string) {
	b.botUserID = ID
}

func (b *Bot) setBotName(name string) {
	b.botUserName = name
}

// msgLen gets lenght of message and attachment messages. Unsupported types return 0.
func msgLen(msg interface{}) (msgLen int) {
	switch m := msg.(type) {
	case string:
		msgLen = len(m)
	case []slack.Attachment:
		msgLen = len(fmt.Sprintf("%#v", m))
	}
	return
}

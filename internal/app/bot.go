package app

import (
	botConfig "VK-bot/internal/config"
	vkAPI "VK-bot/internal/pkg/api"
	updateObjects "VK-bot/internal/pkg/api/objects"
	coinOperations "VK-bot/internal/pkg/operations/coin"
	commonOperations "VK-bot/internal/pkg/operations/common"
	diceOperations "VK-bot/internal/pkg/operations/dice"
	numberOperations "VK-bot/internal/pkg/operations/number"
	welcomeOperations "VK-bot/internal/pkg/operations/welcome"
	wordOperations "VK-bot/internal/pkg/operations/word"
	"encoding/json"
	"fmt"
	"net/http"
)

type Bot struct {
	cfg            botConfig.Config
	debugMode      bool
	openedChannels map[int]chan string
}

func NewBot(cfg botConfig.Config) *Bot {
	return &Bot{
		cfg:            cfg,
		debugMode:      false,
		openedChannels: make(map[int]chan string),
	}
}

func (b *Bot) Start() {
	longPollServerResponse, err := b.getLongPollServer()
	if err != nil {
		b.log(fmt.Sprintf("%s", err))
		return
	}
	b.log("Bot successfully launched up!\nLongPoll server parameters:\nSERVER: " + longPollServerResponse.Server + "\nKEY: " + longPollServerResponse.Key)
	ts := longPollServerResponse.Ts
	for {
		resp, err := b.getUpdates(longPollServerResponse.Server, longPollServerResponse.Key, ts, b.cfg.Wait)
		if err != nil {
			b.log(fmt.Sprintf("%s", err))
			return
		}
		for _, upd := range resp.Updates {
			go b.updateHandler(&upd)
		}
		ts = resp.Ts
	}
}

func (b *Bot) updateHandler(upd *vkAPI.Update) {
	switch upd.Type {
	case updateObjects.NewMessage:
		var messageObject updateObjects.MessageNewObject
		err := json.Unmarshal(upd.Object, &messageObject)
		if err != nil {
			b.log(fmt.Sprintf("Failed to unmarshal message_new object: %s", err))
			return
		}
		receivedText := messageObject.Message.Text
		senderID := messageObject.Message.FromID
		b.log(fmt.Sprintf("Received text: %s\nFrom id: %d\n", receivedText, senderID))

		if _, ok := b.openedChannels[senderID]; ok {
			b.openedChannels[senderID] <- receivedText
			return
		}

		switch receivedText {
		case "Начать":
			err := welcomeOperations.SendWelcomeMessage(&b.cfg, senderID)
			if err != nil {
				b.log(fmt.Sprintf("Failed to send welcome message: %s", err))
				return
			}
		case coinOperations.TossACoin:
			if _, ok := b.openedChannels[senderID]; ok {
				return
			}
			err := coinOperations.SendCoinMessage(&b.cfg, senderID)
			if err != nil {
				b.log(fmt.Sprintf("Failed to send coin message: %s", err))
				return
			}
			messageChan := make(chan string)
			b.openedChannels[senderID] = messageChan
			go b.coinHandler(senderID)
		case diceOperations.TossADice:
			if _, ok := b.openedChannels[senderID]; ok {
				return
			}
			err := diceOperations.SendDiceMessage(&b.cfg, senderID)
			if err != nil {
				b.log(fmt.Sprintf("Failed to send dice message: %s", err))
				return
			}
			messageChan := make(chan string)
			b.openedChannels[senderID] = messageChan
			go b.diceHandler(senderID)
		case wordOperations.GetAWord:
			if _, ok := b.openedChannels[senderID]; ok {
				return
			}
			err := wordOperations.SendWordMessage(&b.cfg, senderID)
			if err != nil {
				b.log(fmt.Sprintf("Failed to send word message: %s", err))
				return
			}
			messageChan := make(chan string)
			b.openedChannels[senderID] = messageChan
			go b.wordHandler(senderID)
		case numberOperations.GetANumber:
			if _, ok := b.openedChannels[senderID]; ok {
				return
			}
			err := numberOperations.SendNumberMessage(&b.cfg, senderID)
			if err != nil {
				b.log(fmt.Sprintf("Failed to send number message: %s", err))
				return
			}
			messageChan := make(chan string)
			b.openedChannels[senderID] = messageChan
			go b.numberHandler(senderID)
		default:
			err := commonOperations.SendNoOpMessage(&b.cfg, senderID)
			if err != nil {
				b.log(fmt.Sprintf("Failed to send no-op message: %s", err))
				return
			}
		}
	}
}

func (b *Bot) SetDebugMode(mode bool) {
	b.debugMode = mode
}

func (b *Bot) log(message string) {
	if !b.debugMode {
		return
	}
	fmt.Println(message)
}

func (b *Bot) getLongPollServer() (*vkAPI.LongPollServerResponse, error) {
	urlToSend := fmt.Sprintf("%sgroups.getLongPollServer?group_id=%s&access_token=%s&v=%s", b.cfg.ApiURL, b.cfg.GroupID, b.cfg.Token, b.cfg.ApiVer)
	resp, err := http.Get(urlToSend)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var longPollServer vkAPI.LongPollServer

	err = json.NewDecoder(resp.Body).Decode(&longPollServer)
	if err != nil {
		return nil, err
	}
	return &longPollServer.Response, nil
}

func (b *Bot) getUpdates(server, key, ts, wait string) (*vkAPI.LongPollUpdateResponse, error) {
	urlToSend := fmt.Sprintf("%s?act=a_check&key=%s&ts=%s&wait=%s", server, key, ts, wait)
	resp, err := http.Get(urlToSend)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var lpResp vkAPI.LongPollUpdateResponse
	err = json.NewDecoder(resp.Body).Decode(&lpResp)
	if err != nil {
		return nil, err
	}
	return &lpResp, nil
}

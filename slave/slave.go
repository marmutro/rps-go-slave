/*
Rock, Paper, Scissor game slave

Uses a MQTT device as a game console and plays against master (Http Rest-API)

*/

package slave

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

type symbol int

const (
	rock    symbol = iota
	paper          = iota
	scissor        = iota
)

func (sym symbol) String() string {
	names := [...]string{
		"Rock",
		"Paper",
		"Scissor"}

	if sym < rock || sym > scissor {
		return "Out-of-range"
	}
	return names[sym]
}

func up(sym symbol) symbol {
	if sym < scissor {
		return sym + 1
	}
	return rock
}

func down(sym symbol) symbol {
	if sym > rock {
		return sym - 1
	}
	return scissor
}

var boardID string
var masterAddress string
var gameURL string
var mqttClient mqtt.Client
var logger log.Logger
var displayContent [4]string

type gameStatus struct {
	currentSymbol symbol
	encoderValue  int
	ownScore      int
	opponentScore int
}

var status gameStatus

func updateDisplay() {
	message := fmt.Sprintf("%s\n%s\n%s\n%s", displayContent[0], displayContent[1], displayContent[2], displayContent[3])
	token := mqttClient.Publish(messageTopic(), 0, false, message)
	token.Wait()
}

func showSymbol() {
	displayContent[0] = fmt.Sprintf("%s", status.currentSymbol)
	displayContent[1] = fmt.Sprintf("-----")
	displayContent[2] = fmt.Sprintf("Own:      %d", status.ownScore)
	displayContent[3] = fmt.Sprintf("Opponent: %d", status.opponentScore)
	updateDisplay()
}

var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	log.Printf("TOPIC: %s\n", msg.Topic())
	log.Printf("MSG: %s\n", msg.Payload())
}

func topicPrefix() string {
	return fmt.Sprintf("tw/%s", boardID)
}

func playButtonTopic() string {
	return fmt.Sprintf("%s/button/1/status", topicPrefix())
}

func showScoreButtonTopic() string {
	return fmt.Sprintf("%s/button/2/status", topicPrefix())
}

func symbolSelectionTopic() string {
	return fmt.Sprintf("%s/encoder/1/status", topicPrefix())
}

func displaySelectTopic() string {
	return fmt.Sprintf("%s/display/1/show", topicPrefix())
}

func messageTopic() string {
	return fmt.Sprintf("%s/display/1/message", topicPrefix())
}

func symbolSelectionHandler(client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())
	i, err := strconv.Atoi(payload)
	if err == nil {
		if i > status.encoderValue {
			status.currentSymbol = up(status.currentSymbol)
		}
		if i < status.encoderValue {
			status.currentSymbol = down(status.currentSymbol)
		}
		status.encoderValue = i
		log.Printf("Symbol=%s\n", status.currentSymbol)
		showSymbol()

	} else {
		log.Printf("Err=%s Payload=%s\n", err, msg.Payload())
	}
}

func postJSON(url string, json []byte) []byte {
	req, err := http.NewRequest("POST", masterAddress+url, bytes.NewBuffer(json))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		panic("Invalid POST response status " + resp.Status)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	return body
}

func playButtonHandler(client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())
	if payload == "ON" {
		log.Printf("PlayButton Pressed\n")
	} else if payload == "OFF" {
		log.Printf("PlayButton Released\n")
	}
}

// Start the slave
func Start() {

	boardIDPtr := flag.String("id", "b03", "Board-ID")
	masterHostAddressPtr := flag.String("masterip", "192.168.201.99:8080", "Master Address")
	brokerHostAddressPtr := flag.String("brokerip", "192.168.201.99:1883", "Broker Address")
	flag.Parse()

	boardID = *boardIDPtr
	brokerAddress := fmt.Sprintf("tcp://%s", *brokerHostAddressPtr)
	masterAddress = fmt.Sprintf("http://%s", *masterHostAddressPtr)

	status.currentSymbol = rock

	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	opts := mqtt.NewClientOptions().AddBroker(brokerAddress).SetClientID("gotrivial")
	opts.SetKeepAlive(2 * time.Second)
	opts.SetDefaultPublishHandler(f)
	opts.SetPingTimeout(1 * time.Second)

	mqttClient = mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	// select message display 4
	token := mqttClient.Publish(displaySelectTopic(), 0, false, "4")
	token.Wait()

	// subcribe to symbol selection
	if token := mqttClient.Subscribe(symbolSelectionTopic(), 0, symbolSelectionHandler); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}
	// subcribe to play button
	if token := mqttClient.Subscribe(playButtonTopic(), 0, playButtonHandler); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	// register cleanup
	cleanUpChan := make(chan os.Signal)
	signal.Notify(cleanUpChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-cleanUpChan
		log.Println("cleaning up")
		if token := mqttClient.Unsubscribe(symbolSelectionTopic()); token.Wait() && token.Error() != nil {
			fmt.Println(token.Error())
			os.Exit(1)
		}
		if token := mqttClient.Unsubscribe(playButtonTopic()); token.Wait() && token.Error() != nil {
			fmt.Println(token.Error())
			os.Exit(1)
		}

		mqttClient.Disconnect(250)
		log.Println("cleaned up")
		os.Exit(1)
	}()

	log.Printf("register Slave %s on master %s", boardID, masterAddress)
	var game Game
	game.BoardID = boardID

	encodedGame, err := json.Marshal(&game)
	if err != nil {
		panic(err)
	}
	gameURL = string(postJSON("/registry", encodedGame))
	log.Printf("Use gameURL %s", gameURL)

	fmt.Println("Press Ctrl-C to stop")
	for {
		time.Sleep(1000 * time.Second) // or runtime.Gosched() or similar per @misterbee
	}

}

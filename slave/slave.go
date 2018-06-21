/*
Rock, Paper, Scissor game slave

Uses a MQTT device as a game console and plays against master (Http Rest-API)

*/

package slave

import (
	"bufio"
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
	"strings"
	"syscall"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

var boardID string
var masterAddress string
var brokerAddress string
var gameURL string
var mqttClient mqtt.Client
var logger log.Logger
var displayContent [4]string
var simulationMode bool

type gameStatus struct {
	initialized    bool
	encoderValue   int
	currentSymbol  Symbol
	oppenentSymbol Symbol
	ownScore       int
	opponentScore  int
}

var status gameStatus

func postJSON(baseURL string, url string, json []byte) *http.Response {
	req, err := http.NewRequest("POST", baseURL+url, bytes.NewBuffer(json))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		panic("Invalid POST response status " + resp.Status)
	}
	return resp
}

func updateDisplay() {
	message := fmt.Sprintf("%s\n%s\n%s\n%s", displayContent[0], displayContent[1], displayContent[2], displayContent[3])
	fmt.Println("---------------")
	fmt.Println(message)
	if !simulationMode {
		token := mqttClient.Publish(messageTopic(), 0, false, message)
		token.Wait()
	}
}

func showGameState() {
	if status.initialized == false {
		displayContent[0] = "Initializing..."
		displayContent[1] = ""
		displayContent[2] = ""
		displayContent[3] = ""
	} else {
		displayContent[0] = fmt.Sprintf("Own:      %s", status.currentSymbol.String())
		displayContent[1] = fmt.Sprintf("Opponent: %s", status.oppenentSymbol.String())
		displayContent[2] = fmt.Sprintf("Own:      %d", status.ownScore)
		displayContent[3] = fmt.Sprintf("Opponent: %d", status.opponentScore)
	}
	updateDisplay()
}

func selectSymbol(turnRight bool) {
	if turnRight {
		status.currentSymbol = Up(status.currentSymbol)
	} else {
		status.currentSymbol = Down(status.currentSymbol)
	}
	DEBUG.Printf("Symbol=%s\n", status.currentSymbol)
	showGameState()
}

func playGame() {
	var slaveSymbol GameSymbol
	slaveSymbol.Symbol = status.currentSymbol.String()
	encodedSymbol, err := json.Marshal(&slaveSymbol)
	if err != nil {
		panic(err)
	}
	resp := postJSON(gameURL, "", encodedSymbol)
	body, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	DEBUG.Printf("Json=%s", string(body))
	var gameResult PlayResult
	err = json.Unmarshal(body, &gameResult)
	if err != nil {
		panic(err)
	}
	DEBUG.Printf("gameResult=%s", string(body))
	status.ownScore = gameResult.SlaveScore
	status.opponentScore = gameResult.MasterScore
	status.oppenentSymbol = FromString(gameResult.MasterSymbol)
	showGameState()
}

var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	DEBUG.Printf("TOPIC: %s\n", msg.Topic())
	DEBUG.Printf("MSG: %s\n", msg.Payload())
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
		if status.initialized {
			if i > status.encoderValue {
				selectSymbol(true)
			}
			if i < status.encoderValue {
				selectSymbol(false)
			}
		}
	} else {
		log.Printf("Err=%s Payload=%s\n", err, msg.Payload())
	}
}

func playButtonHandler(client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())
	if payload == "CLICKED" {
		if status.initialized == true {
			DEBUG.Printf("PlayButton Released\n")
			playGame()
		}
	}
}

// Start the slave
func Start() {

	boardIDPtr := flag.String("id", "b03", "Board-ID")
	masterHostAddressPtr := flag.String("masterip", "192.168.201.99:8080", "Master Address")
	brokerHostAddressPtr := flag.String("brokerip", "192.168.201.99:1883", "Broker Address")
	verbosePtr := flag.Bool("v", false, "Enable logging output")
	simPtr := flag.Bool("sim", false, "Use simulation mode (Ctrl-by-Keyboard)")
	flag.Parse()

	boardID = *boardIDPtr
	brokerAddress = fmt.Sprintf("tcp://%s", *brokerHostAddressPtr)
	masterAddress = fmt.Sprintf("http://%s", *masterHostAddressPtr)
	simulationMode = *simPtr

	if *verbosePtr == true {
		DEBUG = log.New(os.Stdout, "[slave] ", 0)
	}
	for {
		play()
	}
}

func play() error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered in play", r)
			cleanup()
		}
	}()

	if !simulationMode {
		//mqtt.DEBUG = log.New(os.Stdout, "[mqtt] ", 0)
		mqtt.ERROR = log.New(os.Stdout, "[mqtt] ", 0)
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
	}
	showGameState()

	DEBUG.Printf("register Slave %s on master %s", boardID, masterAddress)
	var board Board
	board.BoardID = boardID

	encodedBoard, err := json.Marshal(&board)
	if err != nil {
		panic(err)
	}
	resp := postJSON(masterAddress, "/registry", encodedBoard)
	gameURL = string(resp.Header.Get("content-location"))
	defer resp.Body.Close()

	if !strings.HasPrefix(gameURL, "http://") {
		gameURL = "http://" + gameURL
	}
	DEBUG.Printf("Use gameURL %s", gameURL)
	status.initialized = true
	showGameState()

	if simulationMode {
		fmt.Println("Press Ctrl-C to stop")
		for {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Enter (<L>eft, <R>ight or <P>lay): ")
			text, _ := reader.ReadString('\n')
			text = strings.ToUpper(text)
			DEBUG.Printf("Text=%s", text)
			if strings.HasPrefix(text, "L") {
				selectSymbol(false)
			}
			if strings.HasPrefix(text, "R") {
				selectSymbol(true)
			}
			if strings.HasPrefix(text, "P") {
				playGame()
			}
		}
	} else {
		// subcribe to symbol selection
		if token := mqttClient.Subscribe(symbolSelectionTopic(), 0, symbolSelectionHandler); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
		// subcribe to play button
		if token := mqttClient.Subscribe(playButtonTopic(), 0, playButtonHandler); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}

		// register cleanup
		cleanUpChan := make(chan os.Signal)
		signal.Notify(cleanUpChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-cleanUpChan
			cleanup()
			os.Exit(1)
		}()

		fmt.Println("Press Ctrl-C to stop")
		for {
			time.Sleep(1000 * time.Second) // or runtime.Gosched() or similar per @misterbee
		}
	}
}

func cleanup() {
	DEBUG.Println("cleaning up")
	if token := mqttClient.Unsubscribe(symbolSelectionTopic()); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	if token := mqttClient.Unsubscribe(playButtonTopic()); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	mqttClient.Disconnect(250)
	DEBUG.Println("cleaned up")
}

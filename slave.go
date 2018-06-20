/*
 * Copyright (c) 2013 IBM Corp.
 *
 * All rights reserved. This program and the accompanying materials
 * are made available under the terms of the Eclipse Public License v1.0
 * which accompanies this distribution, and is available at
 * http://www.eclipse.org/legal/epl-v10.html
 *
 * Contributors:
 *    Seth Hoenig
 *    Allan Stockdill-Mander
 *    Mike Robertson
 */

package main

import (
	"flag"
	"fmt"
	"log"
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

var currentSymbol symbol
var encoderValue = 0

var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("TOPIC: %s\n", msg.Topic())
	fmt.Printf("MSG: %s\n", msg.Payload())
}

func topicPrefix(boardID string) string {
	return fmt.Sprintf("tw/%s", boardID)
}

func playButtonTopic(boardID string) string {
	return fmt.Sprintf("%s/button/1/status", topicPrefix(boardID))
}

func showScoreButtonTopic(boardID string) string {
	return fmt.Sprintf("%s/button/2/status", topicPrefix(boardID))
}

func symbolSelectionTopic(boardID string) string {
	return fmt.Sprintf("%s/encoder/1/status", topicPrefix(boardID))
}

func displaySelectTopic(boardID string) string {
	return fmt.Sprintf("%s/display/1/show", topicPrefix(boardID))
}

func messageTopic(boardID string) string {
	return fmt.Sprintf("%s/display/1/message", topicPrefix(boardID))
}

func symbolSelectionHandler(boardID string, client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())
	i, err := strconv.Atoi(payload)
	if err == nil {
		if i > encoderValue {
			currentSymbol = up(currentSymbol)
		}
		if i < encoderValue {
			currentSymbol = down(currentSymbol)
		}
		encoderValue = i
		fmt.Printf("Symbol=%s\n", currentSymbol)
		token := client.Publish(messageTopic(boardID), 0, false, currentSymbol.String())
		token.Wait()

	} else {
		fmt.Printf("Err=%s Payload=%s\n", err, msg.Payload())
	}
}

func playButtonHandler(boardID string, client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())
	if payload == "ON" {
		fmt.Printf("PlayButton Pressed\n")
	} else {
		fmt.Printf("PlayButton Released\n")
	}
}

func main() {

	boardIDPtr := flag.String("id", "b03", "Board-ID")
	//masterHostAddressPtr := flag.String("masterip", "192.168.201.?", "Master Address")
	brokerHostAddressPtr := flag.String("brokerip", "192.168.201.99:1883", "Broker Address")
	flag.Parse()

	boardID := *boardIDPtr
	brokerAddress := fmt.Sprintf("tcp://%s", *brokerHostAddressPtr)

	currentSymbol = rock

	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	opts := mqtt.NewClientOptions().AddBroker(brokerAddress).SetClientID("gotrivial")
	opts.SetKeepAlive(2 * time.Second)
	opts.SetDefaultPublishHandler(f)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	// select message display 4
	token := c.Publish(displaySelectTopic(boardID), 0, false, "4")
	token.Wait()

	// subcribe to symbol selection
	symbolSelectionHandlerFn := func(client mqtt.Client, msg mqtt.Message) {
		symbolSelectionHandler(boardID, client, msg)
	}
	if token := c.Subscribe(symbolSelectionTopic(boardID), 0, symbolSelectionHandlerFn); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}
	// subcribe to play button
	playButtonHandlerFn := func(client mqtt.Client, msg mqtt.Message) {
		playButtonHandler(boardID, client, msg)
	}
	if token := c.Subscribe(playButtonTopic(boardID), 0, playButtonHandlerFn); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	// register cleanup
	cleanUpChan := make(chan os.Signal)
	signal.Notify(cleanUpChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-cleanUpChan
		fmt.Println("cleaning up")
		if token := c.Unsubscribe(symbolSelectionTopic(boardID)); token.Wait() && token.Error() != nil {
			fmt.Println(token.Error())
			os.Exit(1)
		}
		if token := c.Unsubscribe(playButtonTopic(boardID)); token.Wait() && token.Error() != nil {
			fmt.Println(token.Error())
			os.Exit(1)
		}

		c.Disconnect(250)
		fmt.Println("cleaned up")
		os.Exit(1)
	}()

	fmt.Println("Press Ctrl-C to stop")
	for {
		time.Sleep(1000 * time.Second) // or runtime.Gosched() or similar per @misterbee
	}

}

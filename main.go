package main

import (
	"flag"

	"github.com/marmutro/rps-go-slave/slave"
)

func main() {
	boardIDPtr := flag.String("id", "b03", "Board-ID")
	masterHostAddressPtr := flag.String("masterip", "192.168.201.99:8080", "Master Address")
	brokerHostAddressPtr := flag.String("brokerip", "192.168.201.99:1883", "Broker Address")
	verbosePtr := flag.Bool("v", false, "Enable logging output")
	simPtr := flag.Bool("sim", false, "Use simulation mode (Ctrl-by-Keyboard)")
	flag.Parse()

	slave.Start(*boardIDPtr, *masterHostAddressPtr, *brokerHostAddressPtr, *verbosePtr, *simPtr)
}

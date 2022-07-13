package main

import (
	"fmt"

	zmq "github.com/pebbe/zmq4"
)

func main() {
	x := zmq.SUB
	socket, _ := zmq.NewSocket(x)

	defer socket.Close()

	fmt.Printf("Collecting messages\n")
	socket.SetSubscribe("")
	socket.Connect("tcp://localhost:5556")

	for i := 0; i < 101; i++ {

		data, _ := socket.RecvBytes(0)

		fmt.Println(data)
	}
}

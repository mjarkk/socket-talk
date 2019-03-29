package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mjarkk/socket-talk/talkclient"
)

var end = make(chan struct{})

func main() {
	setupClient1()
	setupClient2()
	<-end
}

func setupClient1() {
	c, err := talkclient.NewClient(talkclient.Options{
		ServerURL: "http://localhost:9090",
	})

	if err != nil {
		fmt.Println("ERROR:", err)
		os.Exit(1)
	}

	go func() {
		for {
			err := c.Connect()
			if err == nil {
				break
			}
			fmt.Println("ERROR:", err)
			time.Sleep(time.Second * 3)
		}
	}()

	<-c.ConnectChan

	c.Subscribe("TestMessage", func(msg *talkclient.WSMessage) {
		var data map[string]string
		err := json.Unmarshal(msg.Bytes, &data)

		if err != nil {
			fmt.Println("ERROR:", err)
			return
		}

		if msg.ExpectsAnswer {
			msg.Aswer(data)
			return
		}
		end <- struct{}{}
	})
}

func setupClient2() {
	c, err := talkclient.NewClient(talkclient.Options{
		ServerURL: "http://localhost:9090",
	})

	if err != nil {
		fmt.Println("ERROR:", err)
		os.Exit(1)
	}

	go func() {
		for {
			err := c.Connect()
			if err == nil {
				break
			}
			fmt.Println("ERROR:", err)
			time.Sleep(time.Second * 3)
		}
	}()

	<-c.ConnectChan

	var out map[string]string
	err = c.SendAndReceived("TestMessage", map[string]string{"a": "b"}, &out)
	if err != nil {
		fmt.Println("send test message recieved error:", err)
		return
	}
	fmt.Println(out)

	err = c.Send("TestMessage", map[string]string{"a": "b"})
	if err != nil {
		fmt.Println("send test message recieved error:", err)
		return
	}
}

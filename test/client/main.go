package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mjarkk/socket-talk/talkclient"
)

func main() {
	end := make(chan struct{})
	go func() {
		s := make(chan os.Signal, 1)
		signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
		<-s
		os.Exit(1)
	}()

	setupClient()
	setupClient()
	<-end
}

func setupClient() {
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
			fmt.Println("ERROR:", err)
			fmt.Println("Reconnecting in 3 seconds")
			time.Sleep(time.Second * 3)
		}
	}()

	<-c.ConnectChan
	c.Subscribe("test", func(msg *talkclient.WSMessage) {
		var data map[string]string
		err := msg.BindJSON(&data)
		if err != nil {
			fmt.Println("Recive Error:", err)
			return
		}

		if !msg.ExpectsAnswer {
			return
		}
		msg.Aswer(data)
	})

	go func() {
		for {
			time.Sleep(time.Second * 4)
			err := c.Send("test", map[string]string{})
			if err != nil {
				fmt.Println("sending test error:", err)
			} else {
				fmt.Println("sended test")
			}
		}
	}()
}

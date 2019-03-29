package talkclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

// CB short for CallBack are the callback options
type CB struct {
	Res interface{} // The response data,
}

// SubscribeT is the global type for all subscriptions
type SubscribeT func(msg *WSMessage)

// Client is the main type from where it's possible to make request
type Client struct {
	ServerURL      string
	ServerWsURL    string
	Connected      bool
	Conn           *websocket.Conn
	DisconnectChan chan error
	ConnectChan    chan struct{}
	Subscriptions  map[string]SubscribeT
}

// Options are options that can be used in the NewClient function
type Options struct {
	ServerURL string // server url, default: http://localhost:8080/
}

// NewClient creates a new client object
func NewClient(options Options) (*Client, error) {
	client := &Client{
		ServerURL:   "http://localhost:8080/",
		ServerWsURL: "ws://localhost:8080/",
	}

	if options.ServerURL != "" {
		if !strings.HasPrefix(options.ServerURL, "http://") && !strings.HasPrefix(options.ServerURL, "https://") {
			return nil, errors.New("ServerURL must start with http:// or https://")
		}

		client.ServerURL = options.ServerURL
		if !strings.HasSuffix(options.ServerURL, "/") {
			client.ServerURL = client.ServerURL + "/"
		}

		client.ServerWsURL = strings.Replace(client.ServerURL, "https://", "ws://", 1)
		client.ServerWsURL = strings.Replace(client.ServerURL, "http://", "ws://", 1)
	}

	client.DisconnectChan = make(chan error)
	client.ConnectChan = make(chan struct{})
	client.Subscriptions = map[string]SubscribeT{}

	go messageHandeler(client)

	return client, nil
}

// Connect connects the client to a websocket
// If the client is already connected it will return nil
func (c *Client) Connect() error {
	if c.Connected {
		return errors.New("Already connected")
	}

	conn, _, err := websocket.DefaultDialer.Dial(c.ServerWsURL+"socketTalk/ws", nil)
	if err != nil {
		return err
	}
	c.Connected = true
	c.Conn = conn
	c.ConnectChan <- struct{}{}

	return <-c.DisconnectChan
}

// WSMessage is a websocket message
type WSMessage struct {
	Bytes         []byte
	ExpectsAnswer bool
	Aswer         func(data interface{})
}

// messageHandeler handles all incomming message
func messageHandeler(c *Client) {
	for {
		if !c.Connected {
			time.Sleep(time.Second * 3)
			continue
		}
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			c.DisconnectChan <- err
			c.Disconnect()
			continue
		}
		go func(message []byte) {
			var data sendMeta
			err := json.Unmarshal(message, &data)
			if err != nil {
				return
			}

			handeler, ok := c.Subscriptions[data.Title]
			if !ok {
				return
			}

			postBytes, err := post(c.ServerURL+"socketTalk/get", struct {
				ID string `json:"ID"`
			}{
				ID: data.MessageID,
			})
			if err != nil {
				return
			}

			handeler(&WSMessage{
				Bytes:         postBytes,
				ExpectsAnswer: data.ExpectsAnswer,
				Aswer: func(content interface{}) {
					send(sendOptions{
						C:             c,
						Title:         data.Title + data.ID,
						ExpectsAnswer: false,
						Data:          content,
					}, sendOverwrites{
						ID: data.ID,
					})
				},
			})
		}(message)
	}
}

// Subscribe can subscibe to a spesific title
// Example handeler:
//
// func(msg *talkclient.WSMessage)  {
//   fmt.Println(string(msg.Bytes))
//   return nil
// }
func (c *Client) Subscribe(title string, handeler SubscribeT) {
	c.Subscriptions[title] = handeler
}

// Disconnect disconnects the currnet connection
func (c *Client) Disconnect() {
	if !c.Connected {
		return
	}
	c.Connected = false
	c.Conn.Close()
	c.DisconnectChan <- nil
}

type sendMeta struct {
	Title         string `json:"title"`
	ID            string `json:"ID"`
	MessageID     string `json:"messageID"`
	ExpectsAnswer bool   `json:"expectsAnswer"`
}

// post makes a post request
func post(url string, msg interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)

	if msg != nil {
		jsonData, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}

		_, err = buf.WriteString(string(jsonData))
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	rawOut, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return rawOut, nil
}

type sendOptions struct {
	C             *Client
	Title         string
	ExpectsAnswer bool
	Data          interface{}
	Res           interface{}
}

type sendOverwrites struct {
	ID string
}

// send is the underlaying function that sends something into the network
func send(options sendOptions, overwrites ...sendOverwrites) error {
	if !options.C.Connected {
		return errors.New("Can't send of a closed connection")
	}

	messageID, err := post(options.C.ServerURL+"socketTalk/set", options.Data)
	if err != nil {
		return err
	}

	uuid, err := uuid.NewV4()
	if err != nil {
		return err
	}

	id := uuid.String()

	sendToWS := sendMeta{
		ID:            id,
		MessageID:     string(messageID),
		ExpectsAnswer: options.ExpectsAnswer,
		Title:         options.Title,
	}

	if len(overwrites) > 0 {
		sendToWS.ID = overwrites[0].ID
	}

	err = options.C.Conn.WriteJSON(sendToWS)
	if err != nil {
		return err
	}

	if !options.ExpectsAnswer {
		return nil
	}

	end := make(chan []byte)
	go func(id string) {
		time.Sleep(time.Second * 30)
		close(end)
	}(id)

	options.C.Subscriptions[options.Title+id] = func(msg *WSMessage) {
		end <- msg.Bytes
	}

	returnData, ok := <-end
	delete(options.C.Subscriptions, options.Title+id)
	if !ok {
		return errors.New("Request timed out")
	}

	err = json.Unmarshal(returnData, &options.Res)
	if err != nil {
		return err
	}

	return nil
}

// Send just sends something into the network
func (c *Client) Send(title string, data interface{}) error {
	return send(sendOptions{
		C:     c,
		Title: title,
		Data:  data,
		Res:   nil,
	})
}

// SureSend sends something into the network and wait for some respone on this message
// If there is no res, it will re-send it
// Use this when you need to be sure the message will actualy be delivered
func (c *Client) SureSend(title string, data interface{}) error {
	var res interface{}
	return c.SendAndReceived(title, data, &res)
}

// SendAndReceived sends something into the network and waits for a response from someone
func (c *Client) SendAndReceived(title string, data interface{}, res interface{}) error {
	return send(sendOptions{
		C:             c,
		Title:         title,
		ExpectsAnswer: true,
		Data:          data,
		Res:           &res,
	})
}

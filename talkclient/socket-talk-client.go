package talkclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mjarkk/socket-talk/src"
	uuid "github.com/satori/go.uuid"
)

// CB short for CallBack are the callback options
type CB struct {
	Res interface{} // The response data,
}

// SubscribeT is the global type for all subscriptions
type SubscribeT struct {
	Handeler     func(msg *WSMessage)
	Subscription string // The non-hased subscription
}

var sockLock sync.Mutex

// Client is the main type from where it's possible to make request
type Client struct {
	ServerURL        string
	ServerWsURL      string
	Connected        bool
	Conn             *websocket.Conn
	DisconnectChan   chan error
	ConnectChan      chan struct{}
	innerConnectChan chan struct{}
	Subscriptions    map[string]SubscribeT
	Auth             func([]byte) []byte
	NoProxy          bool
	Logging          bool
}

// Options are options that can be used in the NewClient function
type Options struct {
	Auth      func([]byte) []byte // A function to authenticate the message if not spesified this will be ignored
	ServerURL string              // server url, default: http://localhost:8080/
	NoProxy   bool                // Turn off proxy settings
	Logging   bool                // If this is true the request will be logged
}

// NewClient creates a new client object
func NewClient(options Options) (*Client, error) {
	client := &Client{
		ServerURL:   "http://localhost:8080/",
		ServerWsURL: "ws://localhost:8080/",
		Auth:        options.Auth,
		NoProxy:     options.NoProxy,
		Logging:     options.Logging,
	}

	if options.ServerURL != "" {
		if !strings.HasPrefix(options.ServerURL, "http://") && !strings.HasPrefix(options.ServerURL, "https://") {
			return nil, errors.New("ServerURL must start with http:// or https://")
		}

		client.ServerURL = options.ServerURL
		if !strings.HasSuffix(options.ServerURL, "/") {
			client.ServerURL = client.ServerURL + "/"
		}

		client.ServerWsURL = client.ServerURL
		client.ServerWsURL = strings.Replace(client.ServerWsURL, "https://", "wss://", 1)
		client.ServerWsURL = strings.Replace(client.ServerWsURL, "http://", "ws://", 1)
	}

	client.DisconnectChan = make(chan error)
	client.ConnectChan = make(chan struct{})
	client.innerConnectChan = make(chan struct{})
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

	dailer := websocket.Dialer{}
	if c.NoProxy {
		dailer.Proxy = func(*http.Request) (*url.URL, error) {
			return nil, nil
		}
	} else {
		dailer.Proxy = http.ProxyFromEnvironment
	}

	conn, _, err := dailer.Dial(c.ServerWsURL+"socketTalk/ws", nil)
	if err != nil {
		return err
	}
	c.Connected = true
	c.Conn = conn
	c.ConnectChan <- struct{}{}
	c.innerConnectChan <- struct{}{}

	var res interface{}
	err = send(sendOptions{
		C:             c,
		Title:         "INIT",
		ExpectsAnswer: true,
		Data:          struct{}{},
		Res:           &res,
	})

	return <-c.DisconnectChan
}

// WSMessage is a websocket message
type WSMessage struct {
	Bytes         []byte                    // The actual message
	ExpectsAnswer bool                      // ExectsAnswer is true when the sender expects an answer back
	Aswer         func(data interface{})    // Aswer sends a message back to the sender
	BindJSON      func(v interface{}) error // Bind the json data to something, this is the same as json.Unmarshal
}

func toTimePart(in int) string {
	toReturn := strconv.Itoa(in)
	if len(toReturn) == 1 {
		toReturn = "0" + toReturn
	}
	return toReturn
}

func (c *Client) log(sendOrRec bool, what string) {
	if c.Logging {
		direction := "←"
		if sendOrRec {
			direction = "→"
		}

		t := time.Now()
		seconds := toTimePart(t.Second())
		minutes := toTimePart(t.Minute())
		hour := toTimePart(t.Hour())
		day := toTimePart(t.Day())
		month := toTimePart(int(t.Month()))

		fmt.Printf(
			"[SOCK-TALK] %v/%v/%v - %v:%v:%v (%v) %v\n",

			t.Year(),
			month,
			day,

			hour,
			minutes,
			seconds,

			direction,
			what,
		)
	}
}

// messageHandeler handles all incomming message
func messageHandeler(c *Client) {
	for {
		if !c.Connected {
			<-c.innerConnectChan
		}
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			c.DisconnectChan <- err
			c.Disconnect()
			continue
		}
		go func(message []byte) {
			var data src.SendMeta
			err := json.Unmarshal(message, &data)
			if err != nil {
				return
			}

			sub, ok := c.Subscriptions[data.Title]
			if !ok {
				c.log(false, data.Title)
				return
			}

			c.log(false, sub.Subscription)

			postBytes := []byte{}
			if data.MessageID != "" {
				postBytes, err = post(c.ServerURL+"socketTalk/get", struct {
					ID string `json:"ID"`
				}{
					ID: data.MessageID,
				}, c.NoProxy)
				if err != nil {
					return
				}
			}

			sub.Handeler(&WSMessage{
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
				BindJSON: func(v interface{}) error {
					return json.Unmarshal(postBytes, &v)
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
func (c *Client) Subscribe(title string, handeler func(msg *WSMessage)) {
	c.Subscriptions[src.Hash(title)] = SubscribeT{
		handeler,
		title,
	}
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

// post makes a post request
func post(url string, msg interface{}, noProxy bool) ([]byte, error) {
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
	if noProxy {
		var transport http.RoundTripper = &http.Transport{
			Proxy: nil,
		}
		client.Transport = transport
	}
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

type endT struct {
	Bytes []byte
	Err   error
}

// send is the underlaying function that sends something into the network
func send(options sendOptions, overwrites ...sendOverwrites) error {
	if !options.C.Connected {
		return errors.New("Can't send to a closed connection")
	}

	messageID, err := post(options.C.ServerURL+"socketTalk/set", options.Data, options.C.NoProxy)
	if err != nil {
		return err
	}

	id := ""
	if len(overwrites) > 0 {
		id = overwrites[0].ID
	} else {
		uuid, err := uuid.NewV4()
		if err != nil {
			return err
		}
		id = uuid.String()
	}

	hashedTitle := src.Hash(options.Title)
	sendToWS := src.SendMeta{
		ID:            id,
		MessageID:     string(messageID),
		ExpectsAnswer: options.ExpectsAnswer,
		Title:         hashedTitle,
	}

	jsonData, err := json.Marshal(sendToWS)
	if err != nil {
		return err
	}

	options.C.log(true, options.Title)

	if options.C.Auth == nil {
		sockLock.Lock()
		err = options.C.Conn.WriteMessage(1, jsonData)
		sockLock.Unlock()
	} else {
		sockLock.Lock()
		err = options.C.Conn.WriteMessage(1, options.C.Auth(jsonData))
		sockLock.Unlock()
	}
	if err != nil {
		return err
	}

	if !options.ExpectsAnswer {
		return nil
	}

	end := make(chan endT)
	go func() {
		time.Sleep(time.Second * 30)
		close(end)
	}()

	subID := src.Hash(hashedTitle + id)

	options.C.Subscriptions[subID] = SubscribeT{
		Handeler: func(msg *WSMessage) {
			end <- endT{
				Bytes: msg.Bytes,
			}
		},
		Subscription: options.Title,
	}

	options.C.Subscriptions[src.Hash("SOCKET_TALK_AUTH_FAILED")] = SubscribeT{
		Handeler: func(msg *WSMessage) {
			end <- endT{
				Err: errors.New("Authentication failed"),
			}
		},
		Subscription: "SOCKET_TALK_AUTH_FAILED",
	}

	returnData, ok := <-end
	delete(options.C.Subscriptions, subID)
	if !ok {
		return errors.New("Request timed out")
	}

	if returnData.Err != nil {
		return returnData.Err
	}

	err = json.Unmarshal(returnData.Bytes, &options.Res)
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
	return c.SendAndReceive(title, data, &res)
}

// SendAndReceive sends something into the network and waits for a response from someone
func (c *Client) SendAndReceive(title string, data interface{}, res interface{}) error {
	return send(sendOptions{
		C:             c,
		Title:         title,
		ExpectsAnswer: true,
		Data:          data,
		Res:           &res,
	})
}

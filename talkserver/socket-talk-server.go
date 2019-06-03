package talkserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mjarkk/socket-talk/src"
	uuid "github.com/satori/go.uuid"
	"gopkg.in/olahol/melody.v1"
)

// Options are some settings to include in the Setup function
type Options struct {
	// Auth validates the request, if this is not defined it will broadcast every message
	// The function it's argument is the message that websocket reciefed
	// The return values are the message that gets send to all other client
	// And a bool that tells if the Auth was correct
	Auth func(msg []byte) ([]byte, bool)

	// if SendKeepAlive is true the server will send a keep alive message every 30 seconds
	// the message is "🤖️"
	SendKeepAlive bool

	// if ExtendURL is spesified the middleware will extends another middleware
	ExtendURL   string
	ExtendWSURL string
}

// Setup sets up the needed routes and sets up the websocket route
func Setup(r *gin.Engine, o ...Options) {
	var options Options
	switch len(o) {
	case 0:
		options = Options{}
	case 1:
		options = o[0]
	default:
		panic("Setup accepts only 1 options argument")
	}

	m := melody.New()
	r.GET("/socketTalk/ws", func(c *gin.Context) {
		m.HandleRequest(c.Writer, c.Request)
	})

	if len(options.ExtendURL) > 0 {
		options.ExtendWSURL = strings.Replace(strings.Replace(options.ExtendURL, "https:", "wss:", -1), "http:", "ws:", -1)
		go func() {
			firstRun := true
			for {
				dailer := websocket.Dialer{}
				conn, _, err := dailer.Dial(options.ExtendWSURL+"/socketTalk/ws", nil)
				if err != nil {
					if firstRun {
						panic("Can't connect to other middleware, error: " + err.Error())
					}
					fmt.Println("can't connect to middleware, trying to reconnect in 4 seconds, error:", err)
					time.Sleep(time.Second * 4)
				}
				firstRun = false
				_, message, err := conn.ReadMessage()
				if err != nil {
					fmt.Println("Read error:", err)
					continue
				}

				m.Broadcast(message)
			}
		}()
	}

	setupCache(r, &options)
	handleMessages(m, &options)
	go keepAlive(m, &options)
}

func handleMessages(m *melody.Melody, o *Options) {
	m.HandleMessage(func(s *melody.Session, msg []byte) {
		defer func() {
			if len(o.ExtendURL) > 0 {
				dailer := websocket.Dialer{}
				conn, _, err := dailer.Dial(o.ExtendWSURL+"/socketTalk/ws", nil)
				if err != nil {
					fmt.Println("Can't send to middleware, error:", err)
					return
				}
				conn.WriteMessage(1, msg)
				conn.Close()
			}
		}()

		if o.Auth == nil {
			m.BroadcastOthers(msg, s)
			return
		}

		msg, ok := o.Auth(msg)
		if !ok {
			send(s, src.SendMeta{
				Title: src.Hash("SOCKET_TALK_AUTH_FAILED"),
			})
			return
		}

		m.BroadcastOthers(msg, s)
	})
}

func keepAlive(m *melody.Melody, o *Options) {
	if o.SendKeepAlive {
		for {
			// A keep alive message that makes sure the clients stay connected
			// This bypasses the timeout on proxyies
			time.Sleep(time.Second * 30)
			m.Broadcast([]byte("🤖️"))
		}
	}
}

var cacheLock sync.RWMutex
var cache = map[string][]byte{}

type getCachePost struct {
	ID string `json:"ID"`
}

func addToCache(toAdd []byte) (string, error) {
	uuid, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	id := src.Hash(uuid.String())

	cacheLock.Lock()
	cache[id] = toAdd
	cacheLock.Unlock()

	go func(id string) {
		time.Sleep(time.Second * 20)

		cacheLock.Lock()
		delete(cache, id)
		cacheLock.Unlock()

	}(id)

	return id, nil
}

func proxy(c *gin.Context, url string) {
	req, err := http.NewRequest("POST", url, c.Request.Body)
	if err != nil {
		c.String(400, "CACHE SET PROXY ERROR: "+err.Error())
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		c.String(400, "CACHE SET PROXY ERROR: "+err.Error())
	}

	rawOut, err := ioutil.ReadAll(res.Body)
	if err != nil {
		c.String(400, "CACHE SET PROXY ERROR: "+err.Error())
	}

	c.Data(res.StatusCode, "text/plain", rawOut)
}

func setupCache(r *gin.Engine, o *Options) {
	r.POST("/socketTalk/set", func(c *gin.Context) {
		if len(o.ExtendURL) > 0 {
			proxy(c, o.ExtendURL+"/socketTalk/set")
		} else {
			buf := new(bytes.Buffer)
			buf.ReadFrom(c.Request.Body)
			id, err := addToCache(buf.Bytes())
			if err != nil {
				c.String(400, err.Error())
				return
			}
			c.String(200, id)
		}
	})

	r.POST("/socketTalk/get", func(c *gin.Context) {
		if len(o.ExtendURL) > 0 {
			proxy(c, o.ExtendURL+"/socketTalk/get")
		} else {
			var data getCachePost
			err := c.ShouldBindJSON(&data)
			if err != nil {
				c.String(400, err.Error())
				return
			}

			cacheLock.Lock()
			cacheItem, ok := cache[data.ID]
			cacheLock.Unlock()

			if !ok {
				c.String(400, "ID is wrong")
				return
			}

			c.Data(200, "text/plain", cacheItem)
		}
	})
}

// send sends something to a spesific object
func send(s *melody.Session, toSend interface{}) error {
	meta, err := json.Marshal(toSend)
	if err != nil {
		return err
	}

	return s.Write(meta)
}

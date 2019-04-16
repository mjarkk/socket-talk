package talkserver

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
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

	setupCache(r)
	handleMessages(m, &options)
	go keepAlive(m, &options)
}

func handleMessages(m *melody.Melody, o *Options) {
	m.HandleMessage(func(s *melody.Session, msg []byte) {
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

func setupCache(r *gin.Engine) {
	r.POST("/socketTalk/set", func(c *gin.Context) {
		buf := new(bytes.Buffer)
		buf.ReadFrom(c.Request.Body)
		id, err := addToCache(buf.Bytes())
		if err != nil {
			c.String(400, err.Error())
			return
		}
		c.String(200, id)
	})

	r.POST("/socketTalk/get", func(c *gin.Context) {
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

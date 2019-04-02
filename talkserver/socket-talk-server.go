package talkserver

import (
	"bytes"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/sha3"
	"gopkg.in/olahol/melody.v1"
)

// Options are some settings to include in the Setup function
type Options struct {
	// Auth validates the request, if this is not defined it will broadcast every message
	// The function it's argument is the message that websocket reciefed
	// The return values are the message that gets send to all other client
	// And a bool that tells if the Auth was correct
	Auth func(msg []byte) ([]byte, bool)
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
	handleMessages(m, options)
	go keepAlive(m)
}

func handleMessages(m *melody.Melody, o Options) {
	m.HandleMessage(func(s *melody.Session, msg []byte) {
		if o.Auth == nil {
			m.BroadcastOthers(msg, s)
			return
		}
		msg, ok := o.Auth(msg)
		if !ok {
			// TODO: Return some message authentication failed
			return
		}
		m.BroadcastOthers(msg, s)
	})
}

func keepAlive(m *melody.Melody) {
	for {
		// A keep alive message that makes sure the clients stay connected
		// This bypasses the timeout on proxyies
		time.Sleep(time.Second * 30)
		m.Broadcast([]byte("🤖️"))
	}
}

var cache = map[string][]byte{}

type getCachePost struct {
	ID string `json:"ID"`
}

func setupCache(r *gin.Engine) {
	r.POST("/socketTalk/set", func(c *gin.Context) {
		uuid, err := uuid.NewV4()
		if err != nil {
			c.String(400, err.Error())
			return
		}
		id := calcSha3(uuid.String())
		c.String(200, id)

		buf := new(bytes.Buffer)
		buf.ReadFrom(c.Request.Body)

		go func(bytes []byte, id string) {
			cache[id] = bytes
			time.Sleep(time.Second * 20)
			delete(cache, id)
		}(buf.Bytes(), id)
	})

	r.POST("/socketTalk/get", func(c *gin.Context) {
		var data getCachePost
		err := c.ShouldBindJSON(&data)
		if err != nil {
			c.String(400, err.Error())
			return
		}

		cacheItem, ok := cache[data.ID]
		if !ok {
			c.String(400, "ID is wrong")
			return
		}

		c.Data(200, "text/plain", cacheItem)
	})
}

func calcSha3(in string) string {
	h := sha3.New256()
	h.Write([]byte(in))
	return fmt.Sprintf("%x", h.Sum(nil))
}

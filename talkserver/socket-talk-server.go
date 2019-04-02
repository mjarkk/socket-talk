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

// Setup sets up the needed routes and sets up the websocket route
func Setup(r *gin.Engine) {
	m := melody.New()
	r.GET("/socketTalk/ws", func(c *gin.Context) {
		m.HandleRequest(c.Writer, c.Request)
	})

	setupCache(r)
	handleMessages(m)
	go keepAlive(m)
}

func handleMessages(m *melody.Melody) {
	m.HandleMessage(func(s *melody.Session, msg []byte) {
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

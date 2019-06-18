package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mjarkk/socket-talk/talkserver"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	talkserver.Setup(r)

	fmt.Println("Running on 0.0.0.0:9090")
	fmt.Println(r.Run(":9090"))
}

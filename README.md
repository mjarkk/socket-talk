# `Socket talk` Talk between servers using a middleware
A helper libary for communicating between servers when using a middleware server.  
This is handy if you have 1 middleware server that is connect to a lot of other servers.  
Example code can be found [here](./example/)   

### Requirements:
- Using [Gin](https://github.com/gin-gonic/gin) on the middleware
- A connection to the middleware that allows http post messages and websockets

### TODOs:
- Make the api more robust. The client side needs quite a bit of code to set up and the behaviour of the code is not compeetly obvious
- Message signing
- Encryption

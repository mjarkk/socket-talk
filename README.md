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

### Logging shows long messages.
Sometimes the logging shows messages like this:
```
...
[SOCK-TALK] 2019/05/14 - 12:32:12 (←) 9dc6637accc5ae749b094050f13cc119afc92470eb7c27653a1d98c17d30b98a
...
[SOCK-TALK] 2019/05/14 - 12:32:12 (→) 368824816eb781e66fbaca77aebd26af0c1d0f05f817c1292656a93984518f7d44ec1200-6199-4985-b44b-802334f6fa9b
...
```
This is because of 2 things with the both of them having the same roots,  
Becuase websockets have a limit to the size of the message this library hashes the subscription/title to always have a fixed sized message.  
If the library logs a incomming message it tryies to match a set user subscription and shows that title, if it can't find a subscript it doesn't know what the real title was so it logs the hashed title.  
The other problem comes when responding to a message, the responses message title is the hash of the origin message + a uuid so that will also result in a wired text.

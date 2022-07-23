# bufconn - higher level interface for sockets in golang
> Please note i have designed this for myself for rapid prototyping socket servicies. I have not desinged it to really be used by  someone else and for this reason I have not put much effort into conforming with go standards
## What does this do?
bufconn provides a type `bufconn.Conn` which wraps a net.Conn. It is particularly useful for prototyping message passing services (for example `LOAD data.txt;STORE data2.txt;` is two messages).
## Examples
### Create `Conn`
```go
c, err := net.Dial("tcp6", ":8000")
if err != nil {
    panic(err)
}
// For msgRecvHandler look at examples below
conn := bufconn.NewConn(c, msgRecvHandler, ';')
```
### Message handler
The message handler will be called whenever a message is completely read into the buffer and an operation is not currently ongoing.
It can block for as long as it needs to, however not other operations or message handlers can be called when it is running
```go
func msgRecvHandler(c *bufconn.C) {
    msg, _ := c.ReadMsg(0)
    if msg != "ping"{
        fmt.Println("Remote did not the message ping")
        return
    }
    c.SendMsg("pong")
    msg2, err := c.ReadMsg(time.Second*5)
    if err != nil{
        fmt.Println("Remote did not send message back in time")
    }
    if msg2 != "pling"{
        fmt.Println("Remote did not send the message pling")
        return
    }
    c.SendMsg("plong")
    fmt.Println("Sequence complete")
}
```
### Operation
We can also create an operation (send the first message)
```go
conn.QueueOperation(func(c *bufconn.C) {
    c.WriteMsg("ping")
    msg, err := c.ReadMsg(time.Second*5)
    if err != nil{
        fmt.Println("Remote did not send message back in time")
    }
    if msg != "pong"{
        fmt.Println("Remote did not send the message pong")
        return
    }
    c.WriteMsg("pling")
    msg2, err := c.ReadMsg(time.Second*5)
    if err != nil{
        fmt.Println("Remote did not send message back in time")
    }
    if msg2 != "plong"{
        fmt.Println("Remote did not send the message plong")
        return
    }
    fmt.Println("Sequence complete")
})
```
## Why bother with all the extra code
It can be annoying to have to deal with multiple goroutines using the same socket. This module allows concurrency whilst not allowing different operations on the socket to interfere with each other
package main

import (
    "fmt"
    "math/rand"
    "time"
    "./graph"
    "os"
    "strconv"
    "net"
    msg "./message"
)


func manageConnection(Conn *net.UDPConn,
                dataCh chan msg.Message,
                timeoutCh chan time.Duration,
                quit chan int) {
    var buffer = make([]byte, 4096)
    var m msg.Message
    var timeout time.Duration
    timeout = time.Duration(0)

    for {
        select {
            case timeout = <- timeoutCh: {}
            case _ = <- quit: {return}
            default: {
                if timeout != 0 {
                    Conn.SetReadDeadline(time.Now().Add(timeout))
                }
                n,addr,err := Conn.ReadFromUDP(buffer)
                _ = addr
                if err != nil {
                    if e, ok := err.(net.Error); !ok || !e.Timeout() {
                        panic(err)
                    } else {
                        m = msg.Message{ID_: 0, Type_: "timeout", Sender_: 0, Origin_: 0, Data_: ""}
                    }
                } else {
                    m = msg.FromJsonMsg(buffer[0:n])
                }
                dataCh <- m
            }
        }
    }
}


type Task struct {
    message msg.Message
    destinaion *net.UDPAddr
}


func manageSendQueue(connection *net.UDPConn,
                     taskCh chan Task,
                     SendInterval time.Duration,
                     quit chan int) {
    var t Task
    var buffer = make([]byte, 4096)


    for {
        select {
            case _ = <- quit: return
            case t = <- taskCh: {
                time.Sleep(SendInterval)
                buffer = t.message.ToJsonMsg()
                _, err := connection.WriteToUDP(buffer, t.destinaion)
                if err  != nil {
                  panic(err)
                }
            }
        }
    }
}

func initGossip(id int, neigLen int, neigAddrs []*net.UDPAddr, taskCh chan Task, ttl int){
    for i := 0; i < ttl; i++ {
        destinaion := rand.Intn(neigLen)
        SendAddr := neigAddrs[destinaion]
        m := Task{msg.Message{ID_: id, Type_: "msg", Sender_: 0, Origin_: 0, Data_: "kek"}, SendAddr}
        taskCh <- m
    }
}


func updateTtl(ttls map[int]int, ttl int, TtlVal int){
    _, ok := ttls[TtlVal]
    if !ok {
        ttls[TtlVal] = ttl
    }
}


func processMsg(id int,
                m msg.Message,
                neigAddrs []*net.UDPAddr,
                neigLen int,
                ttl int,
                ttls map[int]int,
                defaultId int,
                taskCh chan Task){

    updateTtl(ttls, ttl, m.ID_)

    if ttls[m.ID_] > 0 {
        for ; ttls[m.ID_] > 0; {
            destinaion := rand.Intn(neigLen)
            SendAddr := neigAddrs[destinaion]
            m.Sender_ = id

            taskCh <- Task{m, SendAddr}
            ttls[m.ID_] -= 1
        }

        confID := m.ID_+defaultId+id
        updateTtl(ttls, ttl, confID)

        m := msg.Message{ID_: confID, Type_: "conf", Sender_: id, Origin_: m.Origin_, Data_: strconv.Itoa(id)}
        for ; ttls[confID] > 0; {
            destinaion := rand.Intn(neigLen)
            SendAddr := neigAddrs[destinaion]
            taskCh <- Task{m, SendAddr}
            ttls[confID] -= 1
        }
    }
}


func processConf(m msg.Message,
                            id int,
                            neigAddrs []*net.UDPAddr,
                            neigLen int,
                            allRecv []bool,
                            ttls map[int]int,
                            ttl int,
                            taskCh chan Task) bool{

    if m.Origin_ == id {
        confID, err := strconv.Atoi(m.Data_)
        if err  != nil {
          panic(err)
        }
        allRecv[confID] = true
        all := true
        for i := range allRecv {
            if allRecv[i] == false {
                all = false
                break
            }
        }
        if all {
            return true
        }
    } else {
        updateTtl(ttls, ttl, m.ID_)

        if ttls[m.ID_] > 0 {
            for ; ttls[m.ID_] > 0; {
                destinaion := rand.Intn(neigLen)
                SendAddr := neigAddrs[destinaion]
                m.Sender_ = id

                taskCh <- Task{m, SendAddr}
                ttls[m.ID_] -= 1
            }
        }
    }
    return false
}


func gossip(id int,
          g graph.Graph,
          quitCh chan int,
          killCh chan int,
          ttl int,
          init bool) {

    neigboirs, _ := g.Neighbors(id)
    neigLen := len(neigboirs)
    node, _ := g.GetNode(id)
    port := node.Port()

    neigPorts  := make([]int, neigLen)
    neigIds    := make([]int, neigLen)
    neigAddrs  := make([]*net.UDPAddr, neigLen)
    for i := 0; i < neigLen; i++ {
        neigPorts[i] = neigboirs[i].Port()
        var err error
        neigIds[i], err = strconv.Atoi(neigboirs[i].String())
        if err  != nil {
          panic(err)
        }

        neigAddrs[i], err = net.ResolveUDPAddr("udp","127.0.0.1:"+strconv.Itoa(neigPorts[i]))
        if err  != nil {
          panic(err)
        }
    }

    addr,err := net.ResolveUDPAddr("udp","127.0.0.1:"+strconv.Itoa(port))
    if err  != nil {
      panic(err)
    }


    connection, err := net.ListenUDP("udp", addr)
    if err  != nil {
      panic(err)
    }

    dataCh    := make(chan msg.Message)
    taskCh    := make(chan Task, 4096)
    timeoutCh := make(chan time.Duration, 1)

    timeoutCh <- time.Duration(0)

    allRecv := make([]bool, len(g))
    ttls := make(map[int]int)
    defaultId := 5000

    quit1 := make(chan int, 1)
    quit2 := make(chan int, 1)

    go manageConnection(connection, dataCh, timeoutCh, quit1)
    go manageSendQueue(connection, taskCh, time.Millisecond, quit2)


    if init {
        initGossip(id, neigLen, neigAddrs, taskCh, ttl)
        for index, _ := range allRecv {
            allRecv[index] = false
        }
        allRecv[id] = true
    }

    var m msg.Message
    i := 0

    look:
    for ;; i++ {
        select {
            case m = <- dataCh: {
                select {
                    case <- killCh:{}
                    default: {
                        if m.Type_ == "msg" {
                            processMsg(id, m, neigAddrs, neigLen, ttl, ttls, defaultId, taskCh)
                        } else if m.Type_ == "conf" {
                            isFinished := processConf(m, id, neigAddrs, neigLen, allRecv, ttls, ttl, taskCh)

                            if (isFinished) {
                                fmt.Println(i, "iters")
                                break look
                            }
                        } else if m.Type_ == "timeout" {
                        } else {
                            panic("Unknown message type"+m.Type_)
                        }
                    }
                }
            }
        }
    }

    quit1 <- 0
    quit2 <- 0


    quitCh <- 0
}


func main() {

    nodes, _ := strconv.Atoi(os.Args[1])
    port, _ := strconv.Atoi(os.Args[2])
    minDeg, _ := strconv.Atoi(os.Args[3])
    maxDeg, _ := strconv.Atoi(os.Args[4])
    ttl, _ := strconv.Atoi(os.Args[5])

    quitCh := make(chan int)
    killCh := make(chan int, nodes)

    seed := int64(time.Now().Unix())
    rand.Seed(seed)
    g := graph.Generate(nodes, minDeg, maxDeg, port)

    go gossip(0, g, quitCh, killCh, ttl, true)
    for i := 1; i < nodes; i++ {
        go gossip(i, g, quitCh, killCh, ttl, false)
    }

    <-quitCh
    for i := 0; i < nodes; i++ {
    	killCh <- 0
	  }
    return
}

package websocket

import (
	"strconv"

	"changestreamwebsocket/config"
	"changestreamwebsocket/dataservice"
	"changestreamwebsocket/logging"
)

type Pool struct {
	Register   chan *Client
	Unregister chan *Client
	Clients    map[*Client]bool
	Notify     chan Message
	dpool      *dataservice.DataPool
}

func NewPool(dp *dataservice.DataPool) *Pool {
	return &Pool{
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Clients:    make(map[*Client]bool),
		Notify:     make(chan Message),
		dpool:      dp,
	}
}

func (pool *Pool) Start() {
	for {
		select {
		case client := <-pool.Register:
			pool.Clients[client] = true
			if config.ConfigData.Debug {
				logging.DebugLogger.Print("Size of Connection Pool: " + strconv.Itoa(len(pool.Clients)))
			}
			pool.dpool.Resend <- true
		case client := <-pool.Unregister:
			delete(pool.Clients, client)
			if config.ConfigData.Debug {
				logging.DebugLogger.Println("Size of Connection Pool: " + strconv.Itoa(len(pool.Clients)))
			}
		case message := <-pool.Notify:
			if config.ConfigData.Debug {
				logging.DebugLogger.Println("Websocket Pool received message from client: ", message)
			}
		case <-pool.dpool.Broadcast:
			//Push update to listeners
			metricsBytes, err := dataservice.GetPoolBytes(pool.dpool)
			if err != nil {
				logging.ErrorLogger.Println("ERROR: " + err.Error())
			} else {
				message := Message{Type: 1, Body: string(metricsBytes)}
				for client := range pool.Clients {
					if err := client.Conn.WriteJSON(message); err != nil {
						logging.ErrorLogger.Println("ERROR: " + err.Error())
					}
				}
			}
		}
	}
}

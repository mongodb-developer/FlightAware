package main

import (
	"net/http"
	"strings"

	"changestreamwebsocket/config"
	"changestreamwebsocket/dataservice"
	"changestreamwebsocket/logging"
	"changestreamwebsocket/mongodbservice"
	"changestreamwebsocket/websocket"

	"github.com/gorilla/mux"
)

func main() {

	//Read the config file
	config.ReadConfig()

	//Initialise Logging
	logging.InitLogging()
	logging.InfoLogger.Println("Logging Initialized")

	//Set up data service pool
	datapool := dataservice.NewDataPool()
	go datapool.Start()
	go datapool.Purge()
	logging.InfoLogger.Println("Data Service Pool Initialized")

	//Set up websocket pool
	pool := websocket.NewPool(datapool)
	go pool.Start()
	logging.InfoLogger.Println("Websocket Pool Initialized")

	//initialize router
	router := mux.NewRouter()
	logging.InfoLogger.Println("Router Initialized")

	//initialise MongoDB service
	go mongodbservice.InitializeMongoDB(datapool)
	logging.InfoLogger.Println("MonogDB Service Initialized")

	//endpoints
	router.HandleFunc("/changestreamservice/v1/getmetrics", func(w http.ResponseWriter, r *http.Request) {
		serveWs(pool, w, r)
	}).Methods("GET") //Register to listen for Metrics updates

	//serve static client page
	//Set up access to static pages for the management interface
	fs := http.FileServer(http.Dir("./webpage"))
	router.PathPrefix("/changestreamservice/v1/observedflights/").Handler(http.StripPrefix(strings.TrimRight("/changestreamservice/v1/observedflights/", "/"), fs))

	logging.ErrorLogger.Fatal(http.ListenAndServe(":"+config.ConfigData.ServicePort, router))
	logging.InfoLogger.Print("Exiting normally")

}

func serveWs(pool *websocket.Pool, w http.ResponseWriter, r *http.Request) {
	if config.ConfigData.Debug {
		logging.DebugLogger.Println("WebSocket Endpoint Hit")
	}
	conn, err := websocket.Upgrade(w, r)
	if err != nil {
		logging.ErrorLogger.Print(err)
	}

	client := &websocket.Client{
		Conn: conn,
		Pool: pool,
	}

	pool.Register <- client
	client.Read()
}

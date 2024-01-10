package logging

import (
	"log"
	"os"
	"strconv"
	"time"

	"changestreamwebsocket/config"
)

var (
	WarningLogger *log.Logger
	InfoLogger    *log.Logger
	ErrorLogger   *log.Logger
	DebugLogger   *log.Logger
)

func InitLogging() {

	//Get the process ID for log messages
	pid := strconv.Itoa(os.Getpid())

	file, err := os.OpenFile(config.ConfigData.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		msg := "ChangestreamWS encountered an unexpected error initiating the logging system at : " + time.Now().Format("2006-01-02 15:04:05") + "\r\n\r\n" + err.Error()
		log.Fatalln(msg)
	}

	InfoLogger = log.New(file, "INFO ("+pid+"): ", log.Ldate|log.Ltime|log.Lshortfile)
	WarningLogger = log.New(file, "WARNING ("+pid+"): ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(file, "ERROR ("+pid+"): ", log.Ldate|log.Ltime|log.Lshortfile)
	DebugLogger = log.New(file, "DEBUG ("+pid+"): ", log.Ldate|log.Ltime|log.Lshortfile)

}

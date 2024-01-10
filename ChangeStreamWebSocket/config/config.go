package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// SpiderConfig contains config settings
type AppConfig struct {
	Debug             bool
	ServicePort       string
	LogFile           string
	MongoDBURI        string
	MongoDBDatabase   string
	MongoDBCollection string
}

// ConfigData contains application configuration settings read from a JSON formatted file.
var ConfigData AppConfig

func ReadConfig() {

	//read the config file
	if err := godotenv.Load(); err != nil {
		log.Fatal("No .env file found")
	}

	ConfigData.MongoDBURI = os.Getenv("CS_MONGODB_URI")
	if ConfigData.MongoDBURI == "" {
		log.Fatal("You must set your 'CS_MONGODB_URI' environment variable.")
	}
	ConfigData.MongoDBDatabase = os.Getenv("CS_MONGODB_DATABASE")
	if ConfigData.MongoDBDatabase == "" {
		log.Fatal("You must set your 'CS_MONGODB_DATABASE' environment variable.")
	}
	ConfigData.MongoDBCollection = os.Getenv("CS_MONGODB_COLLECTION")
	if ConfigData.MongoDBCollection == "" {
		log.Fatal("You must set your 'CS_MONGODB_COLLECTION' environment variable.")
	}
	boolval, err := strconv.ParseBool(os.Getenv("CS_DEBUG"))
	if err != nil {
		log.Fatal("You must set your 'CS_DEBUG' environment variable.")
	}
	ConfigData.Debug = boolval
	ConfigData.LogFile = os.Getenv("CS_LOGFILE")
	if ConfigData.LogFile == "" {
		log.Fatal("You must set your 'CS_LOGFILE' environment variable.")
	}
	ConfigData.ServicePort = os.Getenv("CS_SERVICEPORT")
	if ConfigData.MongoDBURI == "" {
		log.Fatal("You must set your 'CS_SERVICEPORT' environment variable.")
	}
}

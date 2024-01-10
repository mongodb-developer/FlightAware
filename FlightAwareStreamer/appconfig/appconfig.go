package appconfig

import (
	"encoding/json"
	"log"
	"os"
)

var (
	WarningLogger *log.Logger
	InfoLogger    *log.Logger
	ErrorLogger   *log.Logger
)

// AppConfig contains config settings
type AppConfig struct {
	LogFile                  string `json:"LOGFILE"`
	PIDFile                  string `json:"PIDFILE"`
	KafkaServer              string `json:"KAFKASERVER"`
	KafkaADSBTopic           string `json:"KAFKAADSBTOPIC"`
	KafkaUser                string `json:"KAFKAUSER"`
	KafkaPass                string `json:"KAFKAPASS"`
	KafkaFlightScheduleTopic string `json:"KAFKAFLIGHTSCHEDULETOPIC"`
	PiAwareServer            string `json:"PIAWARESERVER"`
	PiAwareUser              string `json:"PIAWAREUSER"`
	PiAwarePass              string `json:"PIAWAREPASS"`
	PiAwareUseCompression    bool   `json:"PIAWAREUSECOMPRESSION"`
	AeroAPIKey               string `json:"AEROAPIKEY"`
	AeroAPIServer            string `json:"AEROAPISERVER"`
	RetrievalMinute          int    `json:"RETRIEVALMINUTE"`
}

// ConfigData contains application configuration settings read from a JSON formatted file.
var ConfigData AppConfig

// ReadConfig
func ReadConfig() (configjson string, err error) {

	//read the config file
	data, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}
	err = json.Unmarshal(data, &ConfigData)
	if err != nil {
		log.Fatalf("Error unmarshalling config file: %s", err)
	}
	return string(data), nil
}

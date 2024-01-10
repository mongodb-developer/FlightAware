package main

import (
	"bufio"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"flightawarestreamer/appconfig"
	"flightawarestreamer/model"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/sarama"
)

var (
	WarningLogger *log.Logger
	InfoLogger    *log.Logger
	ErrorLogger   *log.Logger
)

var wg sync.WaitGroup

func main() {

	//Read the config file
	_, err := appconfig.ReadConfig()
	if err != nil {
		log.Fatal("FlightAware Streamer encountered an unexpected result reading config data: " + err.Error())
	}

	//InitiateLogging
	initLogging()

	//Check PID File
	if err := CheckPIDFile(appconfig.ConfigData.PIDFile); err != nil {
		msg := "An existing instance of the FlightAware streamer is running - this instance has been cancelled : " + ": " + err.Error() + "\n\n"
		WarningLogger.Fatal(msg)
	}

	wg.Add(2)

	go readAndStreamPIAware()
	go readAndStreamAeroAPI()

	// Wait for both goroutines to complete
	wg.Wait()

}

func readAndStreamAeroAPI() {

	defer wg.Done()

	// Kafka broker addresses
	brokers := []string{appconfig.ConfigData.KafkaServer}

	//15 minutes before the start of each hour, get the scheduled arrivals and departure for that hour
	server := appconfig.ConfigData.AeroAPIServer
	apiKey := appconfig.ConfigData.AeroAPIKey
	rateLimiter := 0
	var resp *http.Response
	defer func() {
		_ = resp.Body.Close()
	}()
	for {
		currDate := time.Now().UTC()
		if currDate.Minute() != appconfig.ConfigData.RetrievalMinute {
			fmt.Printf("Current minute is %d. Target is %d - processing skipped\n", currDate.Minute(), appconfig.ConfigData.RetrievalMinute)
			rateLimiter = 0
			time.Sleep(60 * time.Second)
		} else {
			if rateLimiter >= 9 {
				//we're limited to ten API calls an hour
				//The sleep should push us out of the XX:45 window and
				//force us to wait an hour before making any calls.
				fmt.Printf("Current minute is %d. Target is %d, ratelimiter is %d - processing skipped\n", currDate.Minute(), appconfig.ConfigData.RetrievalMinute, rateLimiter)
				time.Sleep(60 * time.Second)
				rateLimiter = 0
			}
			fmt.Printf("Current minute is %d. Target is %d - processing underway\n", currDate.Minute(), appconfig.ConfigData.RetrievalMinute)
			currDate = currDate.Add(time.Minute * (time.Duration(60 - appconfig.ConfigData.RetrievalMinute)))
			targetDate := currDate.Format("2006-01-02")
			targetHour := currDate.Format("15")
			startDate := "&start=" + targetDate + "T" + targetHour + "%3A00%3A00Z"
			endDate := "&end=" + targetDate + "T" + targetHour + "%3A59%3A59Z"

			param_string := "?type=Airline" + startDate + endDate + "&max_pages=15"

			//Get arrivals
			url := server + "scheduled_arrivals" + param_string

			// Create a new HTTP request
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				WarningLogger.Printf("Error creating AeroAPI scheduled arrivals request: %s", err)
				time.Sleep(5 * time.Second)
				continue
			}
			req.Header.Add("x-apikey", apiKey)
			req.Header.Add("Accept", "application/json; charset=UTF-8")

			client := &http.Client{}
			resp, err := client.Do(req)
			rateLimiter++
			if err != nil {
				WarningLogger.Printf("Error occurred while calling the AeroAPI scheduled arrivals web service: %s", err)
				time.Sleep(5 * time.Second)
				continue
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				WarningLogger.Printf("Error reading AeroAPI scheduled arrivals response body: %s", err)
				time.Sleep(5 * time.Second)
				continue
			}

			var flightResults model.Results
			err = json.Unmarshal(body, &flightResults)
			if err != nil {
				WarningLogger.Printf("Error unmarshaling AeroAPI scheduled arrivals JSON: %s", err)
			}

			fmt.Printf("AeroAPI scheduled arrivals response: %+v\n", flightResults)

			//Get departures
			url = server + "scheduled_departures" + param_string

			// Create a new HTTP request
			req, err = http.NewRequest("GET", url, nil)
			if err != nil {
				WarningLogger.Printf("Error creating AeroAPI scheduled departures request: %s", err)
				time.Sleep(5 * time.Second)
				continue
			}
			req.Header.Add("x-apikey", apiKey)
			req.Header.Add("Accept", "application/json; charset=UTF-8")

			client = &http.Client{}
			resp, err = client.Do(req)
			rateLimiter++
			if err != nil {
				WarningLogger.Printf("Error occurred while calling the AeroAPI scheduled departures web service: %s", err)
				time.Sleep(5 * time.Second)
				continue
			}

			body, err = io.ReadAll(resp.Body)
			if err != nil {
				WarningLogger.Printf("Error reading AeroAPI scheduled departures response body: %s", err)
				time.Sleep(5 * time.Second)
				continue
			}

			err = json.Unmarshal(body, &flightResults)
			if err != nil {
				WarningLogger.Printf("Error unmarshaling AeroAPI scheduled departures JSON: %s", err)
			}

			fmt.Printf("AeroAPI scheduled departures response: %+v\n", flightResults)

			//Create a Kafka producer
			config := sarama.NewConfig()

			// Set the SASL configuration
			config.Net.SASL.Enable = true
			config.Net.SASL.User = appconfig.ConfigData.KafkaUser
			config.Net.SASL.Password = appconfig.ConfigData.KafkaPass
			config.Net.SASL.Mechanism = sarama.SASLTypePlaintext

			// Use SASL_PLAINTEXT
			config.Net.TLS.Enable = false
			config.Net.TLS.Config = nil

			config.Producer.Return.Successes = true
			producer, err := sarama.NewSyncProducer(brokers, config)
			if err != nil {
				WarningLogger.Printf("Failed to start Kafka producer for Flight Schedules: %s", err)
				time.Sleep(5 * time.Second)
				continue
			}

			defer func() {
				if err := producer.Close(); err != nil {
					WarningLogger.Printf("Failed to close Kafka producer for Flight Schedules: %s", err)
				}
			}()

			//Send the arrivals
			for _, currFLight := range flightResults.Arrivals {

				// Serialize flight struct to JSON
				jsonFlight, err := json.Marshal(currFLight)
				if err != nil {
					WarningLogger.Printf("Failed to serialize flight schedule arrival message: %s", err)
					continue
				}
				//Send to Kafka
				msg := &sarama.ProducerMessage{
					Topic: appconfig.ConfigData.KafkaFlightScheduleTopic,
					Value: sarama.ByteEncoder(jsonFlight),
				}
				partition, offset, err := producer.SendMessage(msg)
				if err != nil {
					WarningLogger.Printf("Failed to send flight schedule arrival message to Kafka: %s", err)
					//Wait 30 seconds then try reconnecting
					continue
				}
				log.Printf("Flight Schedule arrival message sent to kafka partition %d at offset %d\n", partition, offset)
			}

			//Now do the departures
			for _, currFLight := range flightResults.Departures {

				// Serialize flight struct to JSON
				jsonFlight, err := json.Marshal(currFLight)
				if err != nil {
					WarningLogger.Printf("Failed to serialize flight schedule departure message: %s", err)
					continue
				}
				//Send to Kafka
				msg := &sarama.ProducerMessage{
					Topic: appconfig.ConfigData.KafkaFlightScheduleTopic,
					Value: sarama.ByteEncoder(jsonFlight),
				}
				partition, offset, err := producer.SendMessage(msg)
				if err != nil {
					WarningLogger.Printf("Failed to send flight schedule departure message to Kafka: %s", err)
					continue
				}
				log.Printf("Flight schedule departure message sent to kafka partition %d at offset %d\n", partition, offset)
			}
			//Success - add a wait to stop us unnecessarilly retreiving the same data again
			time.Sleep(60 * time.Second)
		}
	}
}

func readAndStreamPIAware() {

	defer wg.Done()

	// Define the piaware server address and port
	server := appconfig.ConfigData.PiAwareServer

	// Kafka broker addresses
	brokers := []string{appconfig.ConfigData.KafkaServer}

	//Kafka Topics
	ADSBTopic := appconfig.ConfigData.KafkaADSBTopic

	//Set the initiaition command
	initiationCommand := "live username " + appconfig.ConfigData.PiAwareUser + " password " + appconfig.ConfigData.PiAwarePass
	useCompression := appconfig.ConfigData.PiAwareUseCompression

	// Configure TLS with default settings
	conf := &tls.Config{
		// Ignore piaware's self-signed certificate - not a good idea in the real world...
		InsecureSkipVerify: true,
	}

	//Do the following in a loop to handle disconnects
	for {
		// Establish a TLS connection
		conn, err := tls.Dial("tcp", server, conf)
		if err != nil {
			WarningLogger.Printf("Failed to connect to PIAware: %s", err)
			//Wait 30 seconds then try again
			time.Sleep(30 * time.Second)
			continue
		}
		defer func() {
			if err := conn.Close(); err != nil {
				WarningLogger.Printf("Error closing conection to PIAware: %s", err)
			}
		}()

		// Initiate the stream
		if useCompression {
			initiationCommand += " compression gzip"
		}
		initiationCommand += "\n"
		//send the initiation command
		writer := bufio.NewWriter(conn)
		_, err = writer.WriteString(initiationCommand)
		if err != nil {
			WarningLogger.Printf("Failed to write initiation string to PIAware: %s", err)
			//Wait 30 seconds then try again
			time.Sleep(30 * time.Second)
			continue
		}
		writer.Flush()
		if err != nil {
			WarningLogger.Printf("Failed to flush PIAwareWriter: %s", err)
			//Wait 30 seconds then try again
			time.Sleep(30 * time.Second)
			continue
		}

		reader := bufio.NewReader(conn)

		if useCompression {
			gzipReader, err := gzip.NewReader(conn)
			if err != nil {
				WarningLogger.Printf("Failed to create gzip reader: %s", err)
				return
			}
			defer gzipReader.Close()

			reader = bufio.NewReader(gzipReader)
		}

		//Create a Kafka producer
		config := sarama.NewConfig()

		// Set the SASL configuration
		config.Net.SASL.Enable = true
		config.Net.SASL.User = appconfig.ConfigData.KafkaUser
		config.Net.SASL.Password = appconfig.ConfigData.KafkaPass
		config.Net.SASL.Mechanism = sarama.SASLTypePlaintext

		// Use SASL_PLAINTEXT
		config.Net.TLS.Enable = false
		config.Net.TLS.Config = nil

		config.Producer.Return.Successes = true
		producer, err := sarama.NewSyncProducer(brokers, config)
		if err != nil {
			WarningLogger.Printf("Failed to start Kafka producer for PIAware: %s", err)
			//Wait 30 seconds then try again
			time.Sleep(30 * time.Second)
			continue
		}
		defer func() {
			if err := producer.Close(); err != nil {
				WarningLogger.Printf("Failed to close Kafka producer for PIAware: %s", err)
			}
		}()

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					WarningLogger.Printf("Error reading from PIAware: %s", err)
				}
				break
			}
			fmt.Print("Received: ", line)

			//Send to Kafka
			msg := &sarama.ProducerMessage{
				Topic: ADSBTopic,
				Value: sarama.StringEncoder(line),
			}
			partition, offset, err := producer.SendMessage(msg)
			if err != nil {
				WarningLogger.Printf("Failed to send PIAware message to Kafka: %s", err)
				//Wait 30 seconds then try reconnecting
				time.Sleep(30 * time.Second)
				continue
			}
			log.Printf("PIAware message sent to kafka partition %d at offset %d\n", partition, offset)
		}
	}
}

func initLogging() {

	//Get the process ID for log messages
	pid := strconv.Itoa(os.Getpid())

	file, err := os.OpenFile(appconfig.ConfigData.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		msg := "Garaegepi encountered an unexpected error initiating the logging system : " + err.Error()
		log.Fatalln(msg)
	}

	InfoLogger = log.New(file, "INFO ("+pid+"): ", log.Ldate|log.Ltime|log.Lshortfile)
	WarningLogger = log.New(file, "WARNING ("+pid+"): ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(file, "ERROR ("+pid+"): ", log.Ldate|log.Ltime|log.Lshortfile)

	appconfig.ErrorLogger = ErrorLogger
	appconfig.InfoLogger = InfoLogger
	appconfig.WarningLogger = WarningLogger
}

func CheckPIDFile(pidFile string) error {
	// Read in the pid file as a slice of bytes.
	if piddata, err := os.ReadFile(pidFile); err == nil {
		// Convert the file contents to an integer.
		if pid, err := strconv.Atoi(string(piddata)); err == nil {
			// Look for the pid in the process list.
			if process, err := os.FindProcess(pid); err == nil {
				// Send the process a signal zero kill.
				if err := process.Signal(syscall.Signal(0)); err == nil {
					// We only get an error if the pid isn't running, or it's not ours.
					return fmt.Errorf("pid already running: %d", pid)
				}
			}
		}
	}
	// If we get here, then the pidfile didn't exist,
	// or the pid in it doesn't belong to the user running this app.
	return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0664)
}

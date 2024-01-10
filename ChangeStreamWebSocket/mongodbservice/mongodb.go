package mongodbservice

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"changestreamwebsocket/config"
	"changestreamwebsocket/dataservice"
	"changestreamwebsocket/logging"
)

// ChangeEvent represents the structure of a MongoDB change stream event.
type ChangeEvent struct {
	OperationType     string                 `bson:"operationType"`
	Date              time.Time              `bson:"wallTime"`
	FullDocument      bson.M                 `bson:"fullDocument"`
	UpdateDescription bson.M                 `bson:"updateDescription"`
	DocumentKey       map[string]interface{} `bson:"documentKey"`
	// Add other fields as needed based on your change stream event structure
}

// Representation of ADSB data received from FlightAware
type FlightData struct {
	HexId     string    `bson:"hexid"`
	Tail      string    `bson:"ident"`
	Altitude  string    `bson:"alt"`
	Speed     string    `bson:"gs"`
	Heading   string    `bson:"heading"`
	Latitude  string    `bson:"lat"`
	Longitude string    `bson:"lon"`
	TimeStamp time.Time `bson:"_ts"`
}

func InitializeMongoDB(dpool *dataservice.DataPool) {

	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(config.ConfigData.MongoDBURI))
	if err != nil {
		logging.ErrorLogger.Fatal(err)
	}

	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			logging.WarningLogger.Print(err)
		}
	}()

	collection := client.Database(config.ConfigData.MongoDBDatabase).Collection(config.ConfigData.MongoDBCollection)
	ctx := context.TODO()

	//Set up the Change Stream Cursor
	//Only listen for insert operations
	pipeline := mongo.Pipeline{bson.D{{Key: "$match", Value: bson.D{{Key: "operationType", Value: "insert"}}}}}

	//Request the full document
	options := options.ChangeStream().SetFullDocument(options.UpdateLookup)

	changeStream, err := collection.Watch(ctx, pipeline, options)
	if err != nil {
		logging.ErrorLogger.Fatal(err)
	}
	defer changeStream.Close(ctx)
	logging.InfoLogger.Print("Listening to MonogDB change stream events")
	//Iterate over the cursor
	for changeStream.Next(ctx) {

		var event ChangeEvent
		if err := changeStream.Decode(&event); err != nil {
			logging.ErrorLogger.Fatal(err)
		}

		//Convert to JSON for pretty-printing
		jsonBytes, err := json.MarshalIndent(event, "", "    ")
		if err != nil {
			logging.ErrorLogger.Fatal(err)
		}
		if config.ConfigData.Debug {
			fmt.Printf("Received event: %+s\n", string(jsonBytes))
			logging.DebugLogger.Printf("Received event: %+s\n", string(jsonBytes))
		}
		//Get the FlightAware document from the change event
		var fd FlightData
		fdBytes, err := bson.Marshal(event.FullDocument)
		if err != nil {
			logging.ErrorLogger.Fatal(err)
		}
		if err := bson.Unmarshal(fdBytes, &fd); err != nil {
			logging.ErrorLogger.Fatal(err)
		}

		//Convert to Metrics and send to the data service
		var flightMetrics dataservice.Metrics
		flightMetrics.MetricsList = make(map[string]dataservice.Metric)

		var tail dataservice.Metric
		tail.MetricName = "Tail"
		tail.MetricValue = fd.Tail
		tail.Identifier = fd.HexId
		tail.Timestamp = fd.TimeStamp
		flightMetrics.MetricsList[fd.HexId+"_Tail"] = tail

		var speed dataservice.Metric
		speed.MetricName = "Speed"
		speed.MetricValue = fd.Speed
		speed.Identifier = fd.HexId
		speed.Timestamp = fd.TimeStamp
		flightMetrics.MetricsList[fd.HexId+"_Speed"] = speed

		var alt dataservice.Metric
		alt.MetricName = "Altitude"
		alt.MetricValue = fd.Altitude
		alt.Identifier = fd.HexId
		alt.Timestamp = fd.TimeStamp
		flightMetrics.MetricsList[fd.HexId+"_Altitude"] = alt

		var heading dataservice.Metric
		heading.MetricName = "Heading"
		heading.MetricValue = fd.Heading
		heading.Identifier = fd.HexId
		heading.Timestamp = fd.TimeStamp
		flightMetrics.MetricsList[fd.HexId+"_Heading"] = heading

		var latitude dataservice.Metric
		latitude.MetricName = "Latitude"
		latitude.MetricValue = fd.Latitude
		latitude.Identifier = fd.HexId
		latitude.Timestamp = fd.TimeStamp
		flightMetrics.MetricsList[fd.HexId+"_Latitude"] = latitude

		var longitude dataservice.Metric
		longitude.MetricName = "Longitude"
		longitude.MetricValue = fd.Longitude
		longitude.Identifier = fd.HexId
		longitude.Timestamp = fd.TimeStamp
		flightMetrics.MetricsList[fd.HexId+"_Longitude"] = longitude

		dpool.Update <- flightMetrics

	}
	if err := changeStream.Err(); err != nil {
		logging.ErrorLogger.Fatal(err)
	}

}

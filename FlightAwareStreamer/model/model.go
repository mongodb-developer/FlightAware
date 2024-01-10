package model

import (
	"time"
)

type Airport struct {
	Code     string `json:"code"`
	Timezone string `json:"timezone"`
	Name     string `json:"name"`
	City     string `json:"city"`
}

type Flight struct {
	Ident         string    `json:"ident"`
	Operator      string    `json:"operator"`
	FlightNumber  string    `json:"flight_number"`
	Registration  string    `json:"registration"`
	CodeShares    []string  `json:"codeshares"`
	Origin        Airport   `json:"origin"`
	Destination   Airport   `json:"destination"`
	DepartureTime time.Time `json:"scheduled_out"`
	ArrivalTime   time.Time `json:"scheduled_in"`
	AircraftType  string    `json:"aircraft_type"`
	RouteDistance int       `json:"route_destance"`
}

type Results struct {
	Arrivals   []Flight `json:"scheduled_arrivals"`
	Departures []Flight `json:"scheduled_departures"`
	Pages      int      `json:"num_pages"`
	Link       LinkURL  `json:"links"`
}

type LinkURL struct {
	NextURL string `json:"next"`
}

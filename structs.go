package main

import (
	"html/template"
	"time"
)

// Signature holds the wet signature
type Signature struct {
	Name    string // Who
	Role    string
	Email   string       // Needs to match Creator in order to be consider the report was created by
	DataURI template.URL // What: Graphic signature
}

// Case summarises the cases
type Case struct {
	Title    string
	Images   []template.URL
	Category string
	Status   string
	Details  string
}

// Information pertaining to the Unit
type Information struct { // How to see object in Mongo ?
	Name        string
	Type        string
	Address     string
	Postcode    string
	City        string
	State       string
	Country     string
	Description string
}

// Unit is actually a Product in Bugzilla
type Unit struct {
	Information Information // Stored in Mongo
}

// Item is part of an Inventory
type Item struct {
	Name        string
	Images      []template.URL
	Description string
	// Not needed right now
	// Cases       []Case // TODO: not sure what this looks like in the published report
}

// Report for the Unit and rooms of the unit
type Report struct {
	Name        string // Handover of unit – 20 Maple Avenue, Unit 01-02
	Creator     string // email from bugzilla
	Description string
	Images      []template.URL
	Cases       []Case
	Inventory   []Item
	Rooms       []Room
	Comments    string
}

// Room each can have issues (cases) and an inventory
type Room struct {
	Name        string
	Description string
	Images      []template.URL
	Cases       []Case
	Inventory   []Item
}

// InspectionReport is the top level structure that holds a report
type InspectionReport struct {
	ID         string
	Date       time.Time
	Signatures []Signature
	Unit       Unit
	Report     Report
	Template   string
}

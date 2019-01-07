package main

import (
	"html/template"
	"time"
)

// Signature holds the wet signature
type Signature struct {
	Name    string       `json:"name"` // Who
	Role    string       `json:"role"`
	Email   string       `json:"email"`    // Needs to match Creator in order to be consider the report was created by
	DataURI template.URL `json:"data_uri"` // What: Graphic signature
}

// Case summarises the cases
type Case struct {
	Title    string   `json:"title"`
	Images   []string `json:"images"`
	Category string   `json:"category"`
	Status   string   `json:"status"`
	Details  string   `json:"details"`
}

// Information pertaining to the Unit
type Information struct { // How to see object in Mongo ?
	Name        string `json:"name"`
	Type        string `json:"type"`
	Address     string `json:"address"`
	Postcode    string `json:"postcode"`
	City        string `json:"city"`
	State       string `json:"state"`
	Country     string `json:"country"`
	Description string `json:"description"`
}

// Unit is actually a Product in Bugzilla
type Unit struct {
	Information Information `json:"information"` // Stored in Mongo
}

// Item is part of an Inventory
type Item struct {
	Name        string   `json:"name"`
	Images      []string `json:"images"`
	Description string   `json:"description"`
	// Not needed right now
	// Cases       []Case // TODO: not sure what this looks like in the published report
}

// Report for the Unit and rooms of the unit
type Report struct {
	Name        string   `json:"name"` // Handover of unit – 20 Maple Avenue, Unit 01-02
	Description string   `json:"description"`
	Images      []string `json:"images"`
	Cases       []Case   `json:"cases"`
	Inventory   []Item   `json:"inventory"`
	Rooms       []Room   `json:"rooms"`
	Comments    string   `json:"comments"`
}

// Room each can have issues (cases) and an inventory
type Room struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Images      []string `json:"images"`
	Cases       []Case   `json:"cases"`
	Inventory   []Item   `json:"inventory"`
}

// InspectionReport is the top level structure that holds a report
type InspectionReport struct {
	ID         string      `json:"id"`
	Logo       string      `json:"logo"`
	Date       time.Time   `json:"date"`
	Signatures []Signature `json:"signatures"`
	Unit       Unit        `json:"unit"`
	Report     Report      `json:"report"`
	Template   string      `json:"template"`
	Force      bool        `json:"force"`
}

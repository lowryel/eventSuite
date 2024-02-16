package models

import (
	"time"
	"fmt"


	// "github.com/google/uuid"
	_ "github.com/lib/pq"

	"xorm.io/xorm"
	// "log"
)

//  Models
type  Event struct {
	ID        		int     	`xorm:"'id' pk autoincr"`
	Title     		*string    	`json:"title"`
	Description 	*string    	`json:"description"`
	StartDate 		*string 	`json:"start_date"`
	EndDate   		*string 	`json:"end_date"`
	Location  		*string    	`json:"location"`
	Organizer_id 	int       	`json:"organizer_id"`
	// Attendees	[]RegularUser		`xorm:"'attendees' many2many:event_user;"`
	CreatedAt time.Time 		`json:"created_at"`
	UpdatedAt time.Time 		`json:"updated_at"`
}

type  RegularUser struct {				//User Object
	ID        			int     	`xorm:"'id' pk autoincr"`
	Username     		*string    	`json:"username"`
	Email 				*string    	`xorm:"unique" json:"email"`
	Password 			*string    	`json:"password"`
	Fullname 			*string    	`json:"fullname"`
	Organization  		*string    	`json:"organization"`
	EventsAttending  	[]*Event    `xorm:"'events_attending' many2many:event_user" json:"events_attending"`	//events a user has registered to attend
	CreatedAt 			time.Time 	`json:"created_at"`
	UpdatedAt 			time.Time 	`json:"updated_at"`
}

type  EventUser struct {			// Junction table for querying many-to-many relationships
	EventID	int		`xorm:"'event_id' pk autoincr"`
	UserID	int		`xorm:"'user_id' pk autoincr"`
}

type  Organizer struct {			// Events Manager
	ID        		int    			`xorm:"'id' pk autoincr"`
	Name     		*string    		`json:"name"`
	Description 	*string    		`json:"description"`
	ContactEmail 	*string    		`xorm:"unique" json:"email"`
	ContactPhone 	*string    		`xorm:"unique" json:"phone"`
	EventsManaged  	[]*Event    	`json:"events_managed"`      //xorm:"unique" 
	CreatedAt 		time.Time 		`json:"created_at"`
	UpdatedAt 		time.Time 		`json:"updated_at"`
}

// Define the TicketType enum-like type
type TicketType string

// Define constants for the different ticket types
const (
    GeneralAdmission TicketType = "General Admission"
    EarlyBird        TicketType = "Early Bird"
    Student          TicketType = "Student"
    VIP              TicketType = "VIP"
)

type  Ticket struct {
	ID        			int     		`xorm:"'id' pk autoincr"`
	Event_id 			int       		`json:"event_id"`
	Type     			TicketType    	`json:"type"`
	Price    			float64   		`json:"price"`
	QuantityAvailable 	int    		`json:"quantity_available"`
	StartSaleDate 		*string 		`json:"sale_start"`
	EndSaleDate 		*string 		`json:"sale_end"`
}


// Define the StatusChoice enum-like type
type StatusChoice string

// Define constants for the different status choices
const (
    Pending          StatusChoice = "Pending"
    Confirmed        StatusChoice = "Confirmed"
    Cancelled        StatusChoice = "Cancelled"
    Expired          StatusChoice = "Expired"
)

// Registration for the Events
type Registration struct {
	ID        			int     		`xorm:"'id' pk autoincr"`
	User_id 			int       		`json:"user_id"`
	Ticket_id 			int       		`json:"ticket_id"`
	Quantity 			int       		`json:"quantity"` 	// Number of Tickets registered
	RegistrationDate 	time.Time 		`json:"registration_date"`
	Status 				StatusChoice   `json:"status"`
	CreatedAt 			time.Time 		`json:"created_at"`
	UpdatedAt 			time.Time 		`json:"updated_at"`
}

var engine *xorm.Engine

func DBConnection() (*xorm.Engine, error) {
	// ty := Ticket{}
	// ty.Type = GeneralAdmission
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s  dbname=%s sslmode=disable", "localhost", 5432, "eugene", "cartelo009", "eventsdb")
	engine, err := xorm.NewEngine("postgres", dsn)
	if err != nil{
		return nil, err
	}
	if err := engine.Ping(); err != nil{
		return nil, err
	}
	if err := engine.Sync(
			new(Event), new(RegularUser), new(Organizer), new(Ticket), 
			new(Registration),  new(EventUser),
		); err != nil{
		return nil, err
	}
	if err != nil{
		return nil, err
	}
	return engine, err
}







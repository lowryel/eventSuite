package models

import (
	"fmt"
	"os"
	"time"


	// "github.com/google/uuid"
	_ "github.com/lib/pq"

	"xorm.io/xorm"
	// "log"
)

//  Models
type  Event struct {
	ID        		string     	`xorm:"'id' pk" json:"id"`
	Title     		*string    	`json:"title"`
	Description 	*string    	`json:"description"`
	StartDate 		string 	`json:"start_date"`
	EndDate   		string 	`json:"end_date"`
	Location  		*string    	`json:"location"`
	Organizer_id 	string       	`json:"organizer_id"`
	// Attendees	[]RegularUser		`xorm:"'attendees' many2many:event_user;"`
	CreatedAt time.Time 		`json:"created_at"`
	UpdatedAt time.Time 		`json:"updated_at"`
}

type ArchiveEvent struct{
	Data *Event
}


type LoginData struct{
	Email string
	Username string
	Password string
	Phone	string
	Role	string
	Customer string
}

type Login struct{
	Email string		`json:"email"`
	Password string		`json:"password"`
}

type  RegularUser struct {				//User Object
	UserID        			string     	`json:"id"`
	Username     		*string    	`json:"username"`
	Email 				*string    	`xorm:"unique" json:"email"`
	Password 			*string    	`json:"password"`
	Fullname 			*string    	`json:"fullname"`
	Organization  		*string    	`json:"organization"`
	Role				string		`json:"role" validate:"required eq=user" default:"user"`
	EventsAttending  	[]*Event    `json:"events_attending CASCADE"`	//events a user has registered to attend
	Token				string		`json:"token"`
	Refresh_Token		string		`json:"refresh_token"`
	CreatedAt 			time.Time 	`json:"created_at"`
	UpdatedAt 			time.Time 	`json:"updated_at"`
}

type  EventUser struct {			// Junction table for querying many-to-many relationships
    ID     	int 	`json:"id"`
	EventID	string		`json:"event_id"`
	UserID	string		`json:"user_id"`
}

type  Organizer struct {			// Events Manager
	OrganizerID        		string   		`json:"organizer_id"`
	Name     		*string    		`json:"name"`
	Description 	*string    		`json:"description"`
	Email 			*string    		`xorm:"unique" json:"email"`
	Phone 			*string    		`xorm:"unique" json:"phone"`
	Password 		*string    		`json:"password"`
	Role			string			`json:"role" validate:"required eq=organizer"`
	EventsManaged  	[]*Event    	`json:"events_managed"`      //:"unique" 
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
	ID        			string     		`xorm:"'id' pk"`
	Organizer_id		string			`json:"organizer_id"`
	Event_id 			string       	`json:"event_id"`
	Type     			TicketType    	`json:"type"`
	Price    			float64   		`json:"price"`
	QuantityAvailable 	int    			`json:"quantity_available"` // Total tickets available
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
	ID        			string     		`xorm:"'id' pk"`
	User_id 			string       		`json:"user_id"`
	Event_id 			string       		`json:"event_id"`
	Ticket_id 			string       		`json:"ticket_id"`
	Quantity 			int       		`json:"quantity"` 	// Number of Tickets registered
	RegistrationDate 	time.Time 		`json:"registration_date"`
	Status 				StatusChoice   `json:"status"`
	CreatedAt 			time.Time 		`json:"created_at"`
	UpdatedAt 			time.Time 		`json:"updated_at"`
}


type EventAttendee struct{// EventAttendee is someone who made a booking. This will contain info of attendees sourced from RegularUser table
	Email 				*string    	`xorm:"unique" json:"email"`
	Fullname 			*string    	`json:"fullname"`
	Organization  		*string    	`json:"organization"`
	Username			*string		`json:"username"`

} 


// var engine *xorm.Engine

func DBConnection() (*xorm.Engine, error) {
	// ty := Ticket{}
	// ty.Type = GeneralAdmission
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s  dbname=%s sslmode=disable", "dpg-cnsieimn7f5s73dba1m0-a", 5432, "eventsdb_chc2_user", os.Getenv("DB_PASSWORD"), "eventsdb_chc2")
		
		// dsn := fmt.Sprintf(
		// "host=%s port=%d user=%s password=%s  dbname=%s sslmode=disable", "localhost", 5432, "eugene", os.Getenv("PASSWORD"), "eventsdb")
	engine, err := xorm.NewEngine("postgres", dsn)
	if err != nil{
		return nil, err
	}
	if err := engine.Ping(); err != nil{
		return nil, err
	}
	if err := engine.Sync(
			new(Event), new(Organizer), new(Ticket), new(RegularUser),
			new(Registration),  new(EventUser), new(Login), new(LoginData),
			new(ArchiveEvent),
		); err != nil{
		return nil, err
	}
	return engine, err
}







package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	// "github.com/google/uuid"

	"github.com/gofiber/fiber/v2"
	models "github.com/lowry/eventsuite/Models"
	"xorm.io/xorm"
)

var (
	Engine *xorm.Engine
)

type Repository struct {
	DBConn *xorm.Engine
}


func  (repo *Repository) CreateEvent (ctx *fiber.Ctx) error{
	organizer_id := ctx.Params("organizer_id")
	var event models.Event
	var organizer models.Organizer
	_, err := repo.DBConn.ID(organizer_id).Get(&organizer)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"bad request"})
		log.Println(err)
		return nil
	}
	event.CreatedAt = time.Now().Local()
	event.Organizer_id = ParseUserID(organizer_id)
	// event.Attendees = make([]models.RegularUser, 0)
	err = ctx.BodyParser(&event)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"bad request"})
		log.Println(err)
		return nil
	}
	fmt.Println(event)
	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err != nil{
		log.Fatal("Transaction failed")
	}
	//  save event record first
	_, err = session.Insert(&event)
	if err != nil{
		session.Rollback()
		return err
	}
	// save user record
	organizer.EventsManaged = append(organizer.EventsManaged, &event)
	_, err = session.ID(organizer_id).Update(&organizer)
	if err != nil{
		session.Rollback()
		return err
	}
	log.Println(organizer)
	// save event user record	
	// _, err = session.Insert(&eventUser)
	// if err != nil{
	// 	session.Rollback()
	// 	return err
	// }
	// Commit transaction
	err = session.Commit()
	if err != nil{
		log.Fatal("Transaction failed")
		return err
	}
	// repo.DBConn.Join("INNER",  "event_attendee", "limit 1", "event_attendee.event_id = event.id",)
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{"data":event, "message":"user created"})
	return err
}


func (repo *Repository) CreateUser(ctx *fiber.Ctx) error {
	user := &models.RegularUser{}
	user.CreatedAt = time.Now().Local()
	err := ctx.BodyParser(user)
	if err != nil{
		log.Fatal("invalid input", err)
		return nil
	}
	fmt.Println(user)
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err != nil{
		log.Fatal("session error", err)
		return nil
	}
	_, err = repo.DBConn.Insert(user)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": fmt.Sprintf("error inserting user %s", err),
		})
		session.Rollback()
		return err
	}
	err = session.Commit()
	if err != nil{
			log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusOK).JSON(&fiber.Map{
		"data":user, 
	})
	return err
}


//  Create Organizer (person responsible for creating events)
func (repo *Repository) CreateOrganizer(ctx *fiber.Ctx)error{
	organizer_id := ctx.Params("id")
	organizer := models.Organizer{}
	// looking at getting the organizer by the id from event

	event := []*models.Event{}
	err := ctx.BodyParser(&organizer)
	if err != nil{
		log.Fatal("invalid input", err)
		return nil
	}
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err!= nil{
		log.Fatal("session expired")
		return nil
	}
	err = repo.DBConn.SQL("select * from event where organizer_id = ?", organizer_id).Find(&event)
	if err != nil {
		session.Rollback()
		return err
	}
	fmt.Println(organizer.ID)
	organizer.EventsManaged = event
	organizer.CreatedAt = time.Now().Local()
	_, err = repo.DBConn.Insert(&organizer)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid input"})
		session.Rollback()
		return err
	}
	err = session.Commit()
	if err!=nil{
		log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{"message":"success"})
	return err
}


//  Create Ticket by getting event with event_id and assigning the ticket.Event_id to event_id
func (repo *Repository) CreateTicket(ctx *fiber.Ctx)error {
	event_id := ctx.Params("id")
	fmt.Println(event_id)
	ticket := models.Ticket{}
	event := models.Event{}
	if err := ctx.BodyParser(&ticket); err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "Invalid request"})
		return err
	}
	session := repo.DBConn.NewSession()
	defer session.Close()
	err := session.Begin()
	if err != nil{
		log.Fatal(err)
		return err
	}
	fmt.Println("-----------------------------------------------------------------")
	if event_id == ""{
		log.Fatal("no event referenced", err)
		session.Rollback()
		return nil
	}
	_, err = repo.DBConn.SQL("select * from event where id = ?", event_id).Get(&event)
	if err != nil{
		log.Fatal("operation failed: ", err)
		session.Rollback()
		return err
	}
	ticket.Event_id = event.ID
	if event.ID <= 0{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "such event doesn't exist"})
		session.Rollback()
		return err
	}
	log.Println(ticket.Event_id)
	_, err = repo.DBConn.Insert(&ticket)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "request failed"})
		session.Rollback()
		return err
	}
	err = session.Commit()
	if err != nil{
		log.Fatal("transaction failed",err)
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{"data": ticket})
	return err
}

// Helper function to parse user ID from string to int
func ParseUserID(userID string) int {
	var parsedID int
	fmt.Sscanf(userID, "%d", &parsedID)
	return parsedID
}


func (repo *Repository) BookEvent(ctx *fiber.Ctx) error {
	event_id := ctx.Params("event_id")
	user_id := ctx.Params("user_id")
	user := models.RegularUser{}
	event := models.Event{}
	eventUser := models.EventUser{}

	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err := session.Begin()
	if err != nil{
		log.Fatal("Transaction failed")
	}
	_, err = repo.DBConn.ID(event_id).Get(&event)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid event request"})
		session.Rollback()
		return err
	}
	
	_, err = repo.DBConn.ID(user_id).Get(&user)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid user request"})
		session.Rollback()
		return err
	}
	// Validate absent event_id and user_id
	// if either id is absent, raise error
	//  performing checks
	user.EventsAttending = append(user.EventsAttending, &event)
	if event.ID ==0{
		log.Fatal("invalid event ID")
		session.Rollback()
	}
	log.Println(user.ID, event.ID)
	if user.ID ==0{
		log.Fatal("invalid user ID")
		session.Rollback()
	}

    // Check if the association already exists
    exists, err := repo.DBConn.Where("user_id = ? AND event_id = ?", user.ID, event_id).Exist(&eventUser)
    if err != nil {
        return err
    }
    if exists {
        return errors.New("user is already booked for this event")
    }

	fmt.Println(event)
	//  save event record first
	// _, err = session.Insert(&event)
	// if err != nil{
	// 	session.Rollback()
	// 	return err
	// }
	// save user record
	_, err = session.ID(user_id).Update(&user)
	if err != nil{
		session.Rollback()
		return err
	}
	// save event user record	

	_, err = session.Insert(&eventUser)
	if err != nil{
		session.Rollback()
		return err
	}
	// Commit transaction
	err = session.Commit()
	if err != nil{
		log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{
		"message":"event booked successfully",
		"data":user,
	})

	return err
}


func (repo *Repository) AllEvents(ctx *fiber.Ctx) error {
	events := []models.Event{}

	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err := session.Begin()
	if err != nil{
		log.Fatal("Transaction failed")
	}
	err = repo.DBConn.Find(&events)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid event request"})
		session.Rollback()
		return err
	}

	// Commit transaction
	err = session.Commit()
	if err != nil{
		log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{
		"data":events,
	})
	return err
}


func (repo *Repository) GetUser(ctx *fiber.Ctx) error {
	user_id := ctx.Params("user_id")
	userData := models.RegularUser{}

	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err := session.Begin()
	if err != nil{
		log.Fatal("Transaction failed")
	}
	_, err = repo.DBConn.ID(user_id).Get(&userData)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid user request"})
		session.Rollback()
		return err
	}

	if userData.ID != ParseUserID(user_id){
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid user ID"})
		session.Rollback()
		return err
	}

	// Commit transaction
	err = session.Commit()
	if err != nil{
		log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{
		"data":userData,
	})
	return err
}


func (repo *Repository) GetOrganizer(ctx *fiber.Ctx) error {
	organizer_id := ctx.Params("organizer_id")
	organizerData := models.Organizer{}

	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err := session.Begin()
	if err != nil{
		log.Fatal("Transaction failed")
	}
	_, err = repo.DBConn.ID(organizer_id).Get(&organizerData)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid organizer request"})
		session.Rollback()
		return err
	}

	if organizerData.ID != ParseUserID(organizer_id){
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid organizer ID"})
		session.Rollback()
		return err
	}

	// Commit transaction
	err = session.Commit()
	if err != nil{
		log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{
		"data":organizerData,
	})
	return err
}


func (repo *Repository) GetEvent(ctx *fiber.Ctx) error {
	event_id := ctx.Params("event_id")
	eventData := models.Event{}

	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err := session.Begin()
	if err != nil{
		log.Fatal("Transaction failed")
	}
	_, err = repo.DBConn.ID(event_id).Get(&eventData)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid event request"})
		session.Rollback()
		return err
	}

	if eventData.ID != ParseUserID(event_id){
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid event ID"})
		session.Rollback()
		return err
	}
	// Commit transaction
	err = session.Commit()
	if err != nil{
		log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{
		"data":eventData,
	})
	return err
}


func (repo *Repository) Routes(app *fiber.App) {
	api := app.Group("api")
	api.Get("/", func (*fiber.Ctx)  (error){
		fmt.Println("Hello HTTP SERVER")
		return nil
	})
	api.Post("/event/create/:organizer_id", repo.CreateEvent)
	api.Post("/user/create", repo.CreateUser)
	api.Put("/event/booking/event::event_id/user::user_id", repo.BookEvent)
	api.Post("/organizer/create/:id", repo.CreateOrganizer)
	api.Post("/ticket/create/:id", repo.CreateTicket)
	api.Get("/events", repo.AllEvents)
	api.Get("/user/:user_id", repo.GetUser)
	api.Get("/event/:event_id", repo.GetEvent)
	api.Get("/organizer/:organizer_id", repo.GetOrganizer)
}


func main() {
	fmt.Println("Hello World, An Event Manager Cooking")
	app := fiber.New(
		fiber.Config{
			ServerHeader:	"Fiber",
			AppName: "Evnets Suite",
		},
	)

	engine, err := models.DBConnection()
	if err != nil{
		log.Fatal("DB connection failed", err)

	}
	r := Repository{
		DBConn: engine,
	}
	r.Routes(app)
	app.Listen(":5500")
}



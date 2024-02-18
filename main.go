package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
	// "errors"

	// "github.com/google/uuid"

	"github.com/gofiber/fiber/v2"
	models "github.com/lowry/eventsuite/Models"
	"github.com/lowry/eventsuite/middleware"
	"xorm.io/xorm"
)

var (
	Engine *xorm.Engine
)

type Repository struct {
	DBConn *xorm.Engine
}

func (repo *Repository) CreateEvent(ctx *fiber.Ctx) error {
	organizer_id := ctx.Params("organizer_id")
	var event models.Event
	var organizer models.Organizer
	err := ctx.BodyParser(&event)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "bad request"})
		log.Println(err)
		return nil
	}
	// Validate event inputs
	err = middleware.ValidateEvent(&event)
	if err != nil {
		return err
	}
	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err != nil {
		log.Fatal("Transaction failed")
	}

	_, err = repo.DBConn.ID(organizer_id).Get(&organizer)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "bad request"})
		log.Println(err)
		return err
	}
	if ParseUserID(organizer_id) != organizer.ID {
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": "invalid input ID",
		})
	}
	event.CreatedAt = time.Now().Local()
	event.Organizer_id = ParseUserID(organizer_id)
	// event.Attendees = make([]models.RegularUser, 0)
	//  save event record first
	_, err = session.Insert(&event)
	if err != nil {
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": "invalid input",
		})
	}
	// save user record
	organizer.EventsManaged = append(organizer.EventsManaged, &event)
	_, err = session.ID(organizer_id).Update(&organizer)
	if err != nil {
		session.Rollback()
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{
			"error": "server error",
		})
	}

	// Commit transaction
	err = session.Commit()
	if err != nil {
		log.Fatal("Transaction failed")
		session.Rollback()
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{
			"error": "failed transaction",
		})
	}
	// repo.DBConn.Join("INNER",  "event_attendee", "limit 1", "event_attendee.event_id = event.id",)
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{"data": event, "message": "user created"})
	return err
}

func (repo *Repository) CreateUser(ctx *fiber.Ctx) error {
	user := &models.RegularUser{}
	user.CreatedAt = time.Now().Local()
	err := ctx.BodyParser(user)
	if err != nil {
		log.Fatal("invalid input", err)
		return nil
	}
	// Validate user inputs
	err = middleware.ValidateUser(user)
	if err != nil {
		log.Fatal("invalid input", err)
		return err
	}
	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err != nil {
		log.Fatal("session error", err)
		return nil
	}
	// save user record
	_, err = repo.DBConn.Insert(user)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": fmt.Sprintf("error inserting user %s", err),
		})
		session.Rollback()
		return err
	}
	// Commit transaction
	err = session.Commit()
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{
			"error": "failed transaction",
		})
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{"data": user, "message": "user created"})
	return err
}

// Create Organizer (person responsible for creating events)
func (repo *Repository) CreateOrganizer(ctx *fiber.Ctx) error {
	organizer := models.Organizer{}
	// looking at getting the organizer by the id from event
	err := ctx.BodyParser(&organizer)
	if err != nil {
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid input"})
	}

	// Validate organizer inputs
	// transaction begin
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{
			"error": "failed to start transaction",
		})
	}
	// set time
	organizer.CreatedAt = time.Now().Local()

	// Insert organizer record
	_, err = repo.DBConn.Insert(&organizer)
	if err != nil {
		session.Rollback()
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{
			"error": "failed to insert organizer",
		})
	}
	// Commit the transaction
	if err := session.Commit(); err != nil {
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "transaction failed"})
	}

	return ctx.Status(http.StatusCreated).JSON(&fiber.Map{"message": "organizer created successfully"})
}

// Create Ticket by getting event with event_id and assigning the ticket.Event_id to event_id
func (repo *Repository) CreateTicket(ctx *fiber.Ctx) error {
	event_id := ctx.Params("id")
	fmt.Println(event_id)
	ticket := models.Ticket{}
	event := models.Event{}
	if err := ctx.BodyParser(&ticket); err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "Invalid request"})
		return err
	}
	if event_id == "" {
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "no event referenced"})
	}
	//  Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err := session.Begin()
	if err != nil {
		log.Fatal(err)
		return err
	}

	if event_id == "" {
		log.Fatal("no event referenced", err)
		session.Rollback()
		return nil
	}
	_, err = repo.DBConn.ID(event_id).Get(&event)
	if err != nil {
		log.Fatal("operation failed: ", err)
		session.Rollback()
		return err
	}
	ticket.Event_id = event.ID
	if event.ID <= 0 {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "such event doesn't exist"})
		session.Rollback()
		return err
	}
	log.Println(ticket.Event_id)
	_, err = repo.DBConn.Insert(&ticket)
	if err != nil {
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "request failed"})
		session.Rollback()
		return err
	}
	// Commit the transaction
	if err := session.Commit(); err != nil {
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "transaction failed"})
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

type EventData struct {
	Data *models.Event
}

func (repo *Repository) BookEvent(ctx *fiber.Ctx) error {
	event_id := ctx.Params("event_id")
	user_id := ctx.Params("user_id")
	user := models.RegularUser{}
	event := models.Event{}
	eventUser := models.EventUser{}
	// Validate event and user inputs
	if event_id == "" || user_id == "" {
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "no event or user referenced"})
	}

	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "failed to start transaction"})
	}
	_, err := repo.DBConn.ID(event_id).Get(&event)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid event request"})
		session.Rollback()
		return err
	}

	_, err = repo.DBConn.ID(user_id).Get(&user)
	if err != nil {
		session.Rollback()
		return err
	}

    // Check if the association already exists
    exists, err := repo.DBConn.Where("user_id = ? AND event_id = ?", user.ID, event_id).Exist(&eventUser)
    if err != nil {
        return err
    }
    if exists {
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": "user has already booked for this event",
		})
    }

	//  performing checks
	user.EventsAttending = append(user.EventsAttending, &event)
	if event.ID == 0 {
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "no event referenced"})
	}
	log.Println(user.ID, event.ID)
	if user.ID <= 0 {
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "no event referenced"})
	}


	eventData := EventData{
		Data: &event,
	}

	// save user record
	_, err = session.ID(user_id).Update(&user)
	if err != nil {
		session.Rollback()
		return err
	}

	// save event user record
	eventUser.EventID = event.ID
	eventUser.UserID = user.ID
	_, err = session.Insert(&eventUser)
	if err != nil {
		session.Rollback()
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "failed to insert organizer"})
	}

	// Commit the transaction
	if err := session.Commit(); err != nil {
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "transaction failed"})
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{
		"message": "event booked successfully",
		"data":    eventData,
	})
	return err
}


func (repo *Repository) AllEvents(ctx *fiber.Ctx) error {
	events := []models.Event{}

	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err := session.Begin()
	if err != nil {
		log.Fatal("Transaction failed")
	}
	err = repo.DBConn.Find(&events)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid event request"})
		session.Rollback()
		return err
	}

	// Commit transaction
	err = session.Commit()
	if err != nil {
		log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{
		"data": events,
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
	if err != nil {
		log.Fatal("Transaction failed")
	}
	_, err = repo.DBConn.ID(user_id).Get(&userData)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid user request"})
		session.Rollback()
		return err
	}

	if userData.ID != ParseUserID(user_id) {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid user ID"})
		session.Rollback()
		return err
	}

	// Commit transaction
	err = session.Commit()
	if err != nil {
		log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{
		"data": userData,
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
	if err != nil {
		log.Fatal("Transaction failed")
	}
	_, err = repo.DBConn.ID(organizer_id).Get(&organizerData)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid organizer request"})
		session.Rollback()
		return err
	}

	if organizerData.ID != ParseUserID(organizer_id) {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid organizer ID"})
		session.Rollback()
		return err
	}

	// Commit transaction
	err = session.Commit()
	if err != nil {
		log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{
		"data": organizerData,
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
	if err != nil {
		log.Fatal("Transaction failed")
	}
	_, err = repo.DBConn.ID(event_id).Get(&eventData)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid event request"})
		session.Rollback()
		return err
	}

	if eventData.ID != ParseUserID(event_id) {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid event ID"})
		session.Rollback()
		return err
	}
	// Commit transaction
	err = session.Commit()
	if err != nil {
		log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{
		"data": eventData,
	})
	return err
}

func (repo *Repository) Routes(app *fiber.App) {
	api := app.Group("api")
	api.Get("/", func(c *fiber.Ctx) error {
		fmt.Println("Hello HTTP SERVER")
		return nil
	})
	api.Post("/event/create/:organizer_id", repo.CreateEvent)
	api.Post("/user/create", repo.CreateUser)
	api.Put("/event/booking/event::event_id/user::user_id", repo.BookEvent)
	api.Post("/organizer/create", repo.CreateOrganizer)
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
			ServerHeader: "Fiber",
			AppName:      "Evnets Suite",
		},
	)

	engine, err := models.DBConnection()
	if err != nil {
		log.Fatal("DB connection failed", err)

	}
	r := Repository{
		DBConn: engine,
	}
	r.Routes(app)
	app.Listen(":5500")
}

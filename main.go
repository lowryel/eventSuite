package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	// "os"
	"github.com/joho/godotenv"
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

var (
	secret_key = os.Getenv("SECRET_KEY")
)

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
	// hash password
	hashed_pass, err := middleware.HashPassword(*user.Password)
	if err != nil {
		log.Fatal("couldn't hash password")
		return err
	}
	user.Password = &hashed_pass
	// save user record
	_, err = repo.DBConn.Insert(user)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": fmt.Sprintf("error inserting user %s", err),
		})
		session.Rollback()
		return err
	}
	loginData := models.LoginData{
		Email: *user.Email,
		Username: *user.Username,
		Password: *user.Password,
	}
	// inser data into login data table
	_, err = repo.DBConn.Insert(loginData)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": fmt.Sprintf("error inserting login data: %s", err),
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

func (repo *Repository) LoginHandler(c *fiber.Ctx) error {
	loginObj := &models.Login{}
	err := c.BodyParser(loginObj)
	if err != nil{
		log.Fatal("invalid data")
		return nil
	}

	// retrieve logged in user object with username
	user := &models.LoginData{}
	_, err = repo.DBConn.SQL("select * from login_data where email = ?", loginObj.Email).Get(user)
	if err != nil {
		log.Println(err)
		return err
	}
	// match the saved hash in login table to the login input password
	if err = middleware.HashesMatch(user.Password, loginObj.Password); err != nil{
		c.Status(http.StatusBadRequest).JSON(&fiber.Map{"message":"incorrect username or password"})
		return nil
	}
	// Generate jwt token
	token, err := middleware.GenerateToken(user.Username, user.Email)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(&fiber.Map{"msg":"error in generating token"})
	}
	regular_user := models.RegularUser{
		Token: token,
	}
	_, err = repo.DBConn.Where("email = ?", user.Email).Update(&regular_user)
	if err != nil{
		log.Println("token update failed")
		return nil
	}
	log.Println("Login successful")
	c.Status(http.StatusCreated).JSON(&fiber.Map{"token": token})
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


func (repo *Repository) UpdateEvent(ctx *fiber.Ctx) error {
	event_id := ctx.Params("event_id")
	event := models.Event{}
	session := repo.DBConn.NewSession()
	defer session.Close()
	err := session.Begin()
	if err != nil{
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "server error"})
		return err
	}
	_, err = repo.DBConn.ID(event_id).Get(&event)
	if err!= nil{
		log.Println("failed to retrieve event")
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "server error"})
		session.Rollback()
		return err
	}
	eventData := models.Event{}
	if err = ctx.BodyParser(&eventData); err != nil {
		ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request payload"})
		return nil
	}
	event.Title = eventData.Title
	event.Description = eventData.Description
	event.Location = eventData.Location
	event.StartDate = eventData.StartDate
	event.EndDate = eventData.EndDate
	event.UpdatedAt = time.Now().Local()
	_, err = repo.DBConn.ID(event_id).Update(&event)
	if err != nil {
		ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "failed to update event"})
		session.Rollback()
		return nil
	}
	err = session.Commit()
	if err != nil{
		log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusOK).JSON(&fiber.Map{
		"data": "event update success",
	})
	return err
}


func (repo *Repository) UpdateUserProfile(ctx *fiber.Ctx) error {
	user_id := ctx.Params("user_id")
	user := models.RegularUser{}
	session := repo.DBConn.NewSession()
	defer session.Close()
	err := session.Begin()
	if err != nil{
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "server error"})
		return err
	}
	_, err = repo.DBConn.ID(user_id).Get(&user)
	if err!= nil{
		log.Println("failed to retrieve event")
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "server error"})
		session.Rollback()
		return err
	}
	userData := models.RegularUser{}
	if err = ctx.BodyParser(&userData); err != nil {
		ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request payload"})
		return nil
	}
	user.Username = userData.Username
	user.Email = userData.Email
	user.Fullname = userData.Fullname
	user.Organization = userData.Organization
	user.UpdatedAt = time.Now().Local()
	_, err = repo.DBConn.ID(user_id).Update(&user)
	if err != nil {
		ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "failed to update user data"})
		session.Rollback()
		return nil
	}
	err = session.Commit()
	if err != nil{
		log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusOK).JSON(&fiber.Map{
		"data": "user update success",
	})
	return err
}

func (repo *Repository) UpdateOrganizerProfile(ctx *fiber.Ctx) error {
	organizer_id := ctx.Params("user_id")
	organizer := models.Organizer{}
	session := repo.DBConn.NewSession()
	defer session.Close()
	err := session.Begin()
	if err != nil{
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "server error"})
		return err
	}
	_, err = repo.DBConn.ID(organizer_id).Get(&organizer)
	if err!= nil{
		log.Println("failed to retrieve event")
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "server error"})
		session.Rollback()
		return err
	}
	organizerData := models.Organizer{}
	if err = ctx.BodyParser(&organizerData); err != nil {
		ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request payload"})
		return nil
	}
	organizer.Name = organizerData.Name
	organizer.ContactEmail = organizerData.ContactEmail
	organizer.ContactPhone = organizerData.ContactPhone
	organizer.Description = organizerData.Description
	organizer.UpdatedAt = time.Now().Local()
	_, err = repo.DBConn.ID(organizer_id).Update(&organizer)
	if err != nil {
		ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "failed to update user data"})
		session.Rollback()
		return nil
	}
	err = session.Commit()
	if err != nil{
		log.Fatal("Transaction failed")
		return err
	}
	ctx.Status(http.StatusOK).JSON(&fiber.Map{
		"data": "organizer update success",
	})
	return err
}

// ARCHIVE AND DELETE EVENT
func (repo *Repository) DeleteEvent(ctx *fiber.Ctx) error {
	event_id := ctx.Params("event_id")
	event := models.Event{}
	if event_id == ""{
		log.Println("no event selected for deletion")
		return nil
	}
	session := repo.DBConn.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil{
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "server error"})
		return err
	}

	_, err := repo.DBConn.ID(event_id).Get(&event)
	if err != nil{
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"error fetching data"})
		session.Rollback()
		return nil
	}
	/* ARCHIVE EVENT BEFORE DELETING FROM EVENT TABLE */
	archive := models.ArchiveEvent {
		Data: &event,
	}
	// archive event (not delete entirely)
	_, err = repo.DBConn.Insert(&archive)
	if err != nil{
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"error archiving event"})
		session.Rollback()
		return err
	}

	// Delete event from event table
	_, err = repo.DBConn.ID(event_id).Delete(&event)
	if err != nil{
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"error archiving event"})
		session.Rollback()
		return err
	}
	// commit transaction
	err = session.Commit()
	if err != nil{
		log.Fatal("Transaction failed")
		return err
	}

	ctx.Status(http.StatusOK).JSON(&fiber.Map{"msg":"deletion and archival success", "data":archive})
	return err
}

// ARCHIVE EVENT

// if user.Role == "organizer"; user is inserted into the organizer table and also the user table for login purposes
// if user.Role == "user"


func (repo *Repository) SearchEvent(ctx *fiber.Ctx) error {
	queryParam := ctx.Params("query")
	events := []*models.Event{}

	// err := repo.DBConn.Where("start_date = ?", queryParam).Find(&events)
	err := repo.DBConn.SQL("SELECT id, title, description, location, start_date, end_date, organizer_id FROM event WHERE to_tsvector(coalesce(start_date, '') || title || ' ' || coalesce(description, '')) @@ websearch_to_tsquery(?)", queryParam).Find(&events)
	if err != nil{
		log.Fatal("query error", err)
		return err
	}
	log.Println(events)
	ctx.Status(http.StatusOK).JSON(&fiber.Map{
		"data": events,
	})
	return err
}


func (repo *Repository) Routes(app *fiber.App) {
	api := app.Group("api")
	api.Get("/", func(c *fiber.Ctx) error {
		fmt.Println("Hello HTTP SERVER")
		return nil
	})

	api.Post("/user/create", repo.CreateUser)
	api.Post("/organizer/create", repo.CreateOrganizer)
	api.Post("/ticket/create/:id", repo.CreateTicket)
	api.Get("/events", repo.AllEvents)
	api.Post("/login", repo.LoginHandler)

	app.Use(middleware.JWTMiddleware())
	api.Post("/event/create/:organizer_id", repo.CreateEvent)
	api.Put("/event/booking/event::event_id/user::user_id", repo.BookEvent)
	api.Get("/user/:user_id", repo.GetUser)
	api.Get("/event/:event_id", repo.GetEvent)
	api.Get("/search/event/:query", repo.SearchEvent)
	api.Put("/update/event/:event_id", repo.UpdateEvent)
	api.Put("/update/user/:user_id", repo.UpdateUserProfile)
	api.Delete("/delete/event/:event_id", repo.DeleteEvent)
	api.Put("/update/organizer/:organizer_id", repo.UpdateOrganizerProfile)
	api.Get("/organizer/:organizer_id", repo.GetOrganizer)
}

func main() {
	fmt.Println("Hello World, An Event Manager Cooking")
	err := godotenv.Load()
	if err != nil{
		log.Fatal("Error loading .env file")
	}
	
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

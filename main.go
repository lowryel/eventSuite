package main

import (
	"fmt"
	"log"
	"net/http"
	// "os"
	"time"

	// "os"
	"github.com/joho/godotenv"
	// "errors"

	// "github.com/google/uuid"

	"github.com/gofiber/fiber/v2"
	logger "github.com/lowry/eventsuite/Logger"
	models "github.com/lowry/eventsuite/Models"
	"github.com/lowry/eventsuite/middleware"
	"xorm.io/xorm"
)


type Repository struct {
	DBConn *xorm.Engine
}

// var (
// 	secret_key = os.Getenv("SECRET_KEY")
// )

func (repo *Repository) CreateEvent(ctx *fiber.Ctx) error {
	tokenString := ctx.Get("Authorization")
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "bad request"})
		return err
	}
	// handle role based access
	organizer_id := claims.UserID
	if claims.Role != "organizer"{
		ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{
			"error":"Unauthorized: cannot create events",
		})
		return err
	}
	var event models.Event
	var organizer models.Organizer
	err = ctx.BodyParser(&event)
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
	if organizer_id != organizer.ID {
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": "invalid input ID",
		})
	}
	event.ID = middleware.RandomIDGen(32)
	event.CreatedAt = time.Now().Local()
	event.Organizer_id = organizer_id
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
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{"event": event, "message": "user created"})
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
	user.Role = "user"
	user.ID = middleware.RandomIDGen(32)
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
		Role: user.Role,
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
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{"user data": user, "message": "user created"})
	return err
}


func (repo *Repository) LoginHandler(ctx *fiber.Ctx) error {
	loginObj := &models.Login{}
	err := ctx.BodyParser(loginObj)
	if err != nil{
		log.Fatal("invalid data")
		return nil
	}
	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err != nil {
		log.Fatal("session error", err)
		return nil
	}

	// retrieve registered user object with email
	user := &models.LoginData{}
	_, err = repo.DBConn.SQL("select * from login_data where email = ?", loginObj.Email).Get(user)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"message":"error fetching login data"})
		session.Rollback()
		log.Println(err)
		return err
	}
	// match the saved hash in login table to the login input password
	if err = middleware.HashesMatch(user.Password, loginObj.Password); err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"message":"error matching password"})
		logger.DevLog(err)
		session.Rollback()
		return nil
	}

	if user.Role == "user"{
		regular_user := models.RegularUser{}
		_, err = repo.DBConn.Where("email = ? AND role = ?", user.Email, user.Role).Get(&regular_user)
		if err != nil{
			log.Println("token update failed")
			session.Rollback()
			return nil
		}
		// Generate jwt token
		token, err := middleware.GenerateToken(user.Email, user.Role, regular_user.ID)
		if err != nil {
			session.Rollback()
			return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"msg":"error in generating token"})
		}
		// Commit transaction
		err = session.Commit()
		if err != nil {
			return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{
				"error": "failed transaction",
			})
		}
		log.Println("Login successful")
		return ctx.Status(http.StatusCreated).JSON(&fiber.Map{"token": token})
	}

	organizer := models.Organizer{}
	// ORGANIZER ROLE DECLARATION
	logger.DevLog(user.Email)
	logger.DevLog(user.Role)
	_, err = repo.DBConn.Where("email = ? AND role = ?", user.Email, user.Role).Get(&organizer)
	logger.DevLog(organizer)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"msg":"error generating organizer token"})
		session.Rollback()
		return err
	}
	// Generate jwt token
	token, err := middleware.GenerateToken(user.Email, user.Role, organizer.ID)
	if err != nil {
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"msg":"error in generating token"})
	}
	// Commit transaction
	err = session.Commit()
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{
			"error": "failed transaction",
		})
	}
	log.Println("Login successful")
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{"token": token})
	return err
}



// Create Organizer (person responsible for creating events)
func (repo *Repository) CreateOrganizer(ctx *fiber.Ctx) error {
	organizer := &models.Organizer{}
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

	// hash password
	hashed_pass, err := middleware.HashPassword(*organizer.Password)
	if err != nil {
		log.Fatal("couldn't hash password")
		return err
	}
	organizer.Password = &hashed_pass
	organizer.Role = "organizer"
	organizer.ID = middleware.RandomIDGen(32)
	// save user record
	_, err = repo.DBConn.Insert(organizer)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": fmt.Sprintf("error inserting organizer %s", err),
		})
		session.Rollback()
		return err
	}
	loginData := models.LoginData{
		Email: *organizer.Email,
		Phone: *organizer.Phone,
		Password: *organizer.Password,
		Role:	organizer.Role,
	}
	// inser data into login data table
	_, err = repo.DBConn.Insert(&loginData)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": fmt.Sprintf("error inserting login data: %s", err),
		})
		session.Rollback()
		return err
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
	tokenString := ctx.Get("Authorization")
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil{
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "Invalid request"})
		return err		
	}
	if claims.Role != "organizer"{
		logger.DevLog("Unauthorized access")
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "Unauthorized access"})
		return err
	}
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
	err = session.Begin()
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
	ticket.ID = middleware.RandomIDGen(32)
	ticket.Organizer_id = claims.UserID
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
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{"ticket data": ticket})
	return err
}

// Helper function to parse user ID from string to int
func ParseUserID(userID string) uint32 {
	var parsedID uint32
	fmt.Sscanf(userID, "%d", &parsedID)
	return parsedID
}

type EventData struct {
	Data *models.Event
}

func (repo *Repository) BookEvent(ctx *fiber.Ctx) error {
	event_id := ctx.Params("event_id")
	// Extract the user ID from the JWT token included in the request headers
    tokenString := ctx.Get("Authorization")
	logger.DevLog(tokenString)
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil{
		logger.DevLog(err)
		return nil
	}
	// handle role based access
	if claims.Role != "user"{
		logger.DevLog("organizer can't make booking")
		return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error": "organizer can't make booking"})
	}

    if claims.UserID <=0 {
		logger.DevLog("problem extracting user ID")
        return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error": "unauthorized"})
    }
	user := models.RegularUser{}
	event := models.Event{}
	eventUser := models.EventUser{}
	// Validate event and user inputs
	if event_id == ""{
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "empty event id"})
	}

	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	if err := session.Begin(); err != nil {
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "failed to start transaction"})
	}
	_, err = repo.DBConn.ID(event_id).Get(&event)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid event request"})
		session.Rollback()
		return err
	}
	// fetch user first in order to be able to update it * made this mistake earlier and it worried me a lot *
	_, err = repo.DBConn.ID(claims.UserID).Get(&user)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid user action"})
		session.Rollback()
		return err
	}

    // Check if the association already exists
    exists, err := repo.DBConn.Where("user_id = ? AND event_id = ?", claims.UserID, event_id).Exist(&eventUser)
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
		logger.DevLog("no event referenced")
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "no event referenced"})
	}
	log.Println(user.ID, event.ID)
	if claims.UserID <= 0 {
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "no user referenced"})
	}

	eventData := EventData{
		Data: &event,
	}

	// save user record
	_, err = repo.DBConn.ID(claims.UserID).Update(&user) // update user with new events
	if err != nil {
		session.Rollback()
		return err
	}

	// save event user record
	eventUser.EventID = event.ID
	eventUser.UserID = claims.UserID
	_, err = repo.DBConn.Insert(&eventUser)
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
		"event data":    eventData,
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
		"events data": events,
	})
	return err
}


func (repo *Repository) GetUser(ctx *fiber.Ctx) error {
	tokenString := ctx.Get("Authorization")
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil {
		log.Fatal("token not found")
		return err
	}
	if claims.Role != "user"{
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error":"Unauthorized access",
		})
	}
	user_id := claims.UserID
	userData := models.RegularUser{}

	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{
			"error":"server error",
		})
	}
	_, err = repo.DBConn.ID(user_id).Get(&userData)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid user request"})
		session.Rollback()
		return err
	}

	if userData.ID != user_id {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid user ID"})
		session.Rollback()
		return err
	}

	// Commit transaction
	err = session.Commit()
	if err != nil {
		log.Println("Transaction failed")
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{
		"user data": userData,
	})
	return err
}

func (repo *Repository) GetOrganizer(ctx *fiber.Ctx) error {
	tokenString := ctx.Get("Authorization")
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil {
		logger.DevLog("failed to get organizer ID")
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid request"})
	}
	if claims.Role != "organizer"{
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error":"Unauthorized access",
		})
	}
	organizer_id := claims.UserID
	organizerData := models.Organizer{}

	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err != nil {
		log.Fatal("Transaction failed")
	}
	_, err = repo.DBConn.ID(organizer_id).Get(&organizerData)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid organizer request"})
		session.Rollback()
		return err
	}

	if organizerData.ID != organizer_id {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid organizer ID"})
		session.Rollback()
		return err
	}

	// Commit transaction
	err = session.Commit()
	if err != nil {
		log.Println("Transaction failed")
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{
		"organizer data": organizerData,
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
		log.Println("Transaction failed")
		return err
	}
	ctx.Status(http.StatusCreated).JSON(&fiber.Map{
		"event data": eventData,
	})
	return err
}


func (repo *Repository) UpdateEvent(ctx *fiber.Ctx) error {
	tokenString := ctx.Get("Authorization")
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil{
		logger.DevLog("Invalid token string")
		return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error":"Invalid token string"})
	}
	if claims.Role != "organizer"{
		logger.DevLog("Unauthorized")
		return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error":"Unauthorized access"})
	}
	event_id := ctx.Params("event_id")
	event := models.Event{}
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
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
	tokenString := ctx.Get("Authorization")
	claims, err := middleware.DecodeToken(tokenString[7:])
	if err != nil{
		logger.DevLog("error getting token")
		return err
	}
	if claims.Role != "user"{
		logger.DevLog("Unauthorized")
		return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error":"Unauthorized access"})
	}
	user_id := claims.UserID
	user := models.RegularUser{}
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
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
	tokenString := ctx.Get("Authorization")
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil{
		logger.DevLog("Unauthorized")
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid request"})
	}
	// handle role based access
	if claims.Role != "organizer"{
		logger.DevLog("Unauthorized")
		return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error":"Unauthorized access"})
	}
	organizer_id := claims.UserID
	organizer := models.Organizer{}
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
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
	organizer.Email = organizerData.Email
	organizer.Phone = organizerData.Phone
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
	tokenString := ctx.Get("Authorization")
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil{
		logger.DevLog("invalid")
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid request"})
	}
	if claims.Role != "organizer"{
		logger.DevLog("Unauthorized")
		return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error":"Unauthorized access"})
	}
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

	_, err = repo.DBConn.ID(event_id).Get(&event)
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


// get users subscribed to events
func (repo *Repository) SubscribedEvents(ctx *fiber.Ctx) error {
	tokenString := ctx.Get("Authorization")
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil{
		logger.DevLog("invalid")
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid request"})
	}
	if claims.Role != "organizer"{
		logger.DevLog("Umauthorized")
		return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error":"Unauthorized access"})
	}
	organizer_id := claims.UserID
	event_user := []models.EventUser{}
	events := []models.Event{}
	users := []models.RegularUser{}
	err = repo.DBConn.Find(&event_user)
	if err != nil{
		logger.DevLog("operation failed")
		return err
	}

	for e, event := range event_user {
		err = repo.DBConn.Where("id = ? AND organizer_id = ?",event.EventID, organizer_id).Find(&events)
		if err != nil{
			logger.DevLog("invalid")
			return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid request"})
		}
		err = repo.DBConn.Where("id = ?", event.UserID).Find(&users)
		if err != nil{
			logger.DevLog("invalid")
			return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid request"})
		}
		logger.DevLog(*users[e].Email)
		ctx.JSON(&fiber.Map{"booked events":users[e]})
	}

	return ctx.JSON(&fiber.Map{"booked events":events}) // "users (attendees)":users,
}


func (repo *Repository) SearchEvent(ctx *fiber.Ctx) error {
	queryParam := ctx.Params("query")
	events := []*models.Event{}

	// err := repo.DBConn.Where("start_date = ?", queryParam).Find(&events)
	err := repo.DBConn.SQL("SELECT * FROM event WHERE to_tsvector(coalesce(start_date, '') || title || ' ' || coalesce(description, '')) @@ websearch_to_tsquery(?)", queryParam).Find(&events)
	if err != nil{
		log.Fatal("query error", err)
		return err
	}
	log.Println(events)
	ctx.Status(http.StatusOK).JSON(&fiber.Map{
		"Searched response": events,
	})
	return err
}


func (repo *Repository) AttendeeRegistration(ctx *fiber.Ctx) error {// register and set status to Pending
	ticket_id := ctx.Params("ticket_id")
	tokenString := ctx.Get("Authorization")
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil{
		logger.DevLog(err)
		return nil
	}
	if claims.Role != "user"{
		logger.DevLog("Unauthorized")
		return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error":"Unauthorized access"})
	}
	ticket := models.Ticket{}
	register := models.Registration{}
	err = ctx.BodyParser(&register)
	if err != nil{
		logger.DevLog(err)
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid data"})
	}
	if ticket_id == ""{
		logger.DevLog(err)
		return err
	}
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err!= nil{
		logger.DevLog("Begin session failed")
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"session closed"})
	}
	_, err = repo.DBConn.Where("id = ?", ticket_id).Get(&ticket)
	if err!= nil{
		logger.DevLog("error getting ticket")
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"error fetching ticket"})
	}
	ticketsAvailable, err := middleware.TicketAvailable(string(ticket.Type), register.Quantity, ticket.QuantityAvailable)
	if err != nil{
		logger.DevLog("Ticket exhausted")
		return err
	}

	logger.DevLog(ticket.Type)
	logger.DevLog(ticketsAvailable)
	// update ticket available
	ticket.QuantityAvailable = ticketsAvailable
	_, err = repo.DBConn.ID(ticket.ID).Update(&ticket)
	if err!= nil{
		logger.DevLog("error updating ticket")
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"error updating ticket"})
	}
	register.ID = middleware.RandomIDGen(32)
	register.User_id = claims.UserID
	register.Status = models.Pending
	register.Ticket_id = ticket.ID
	register.CreatedAt = time.Now().Local()
	_, err = repo.DBConn.Insert(&register) //
	if err!= nil{
		logger.DevLog("error during registration")
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"registration failed"})
	}
	err = session.Commit()
	if err != nil{
		logger.DevLog("Transaction failed")
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"failed to commit data"})
	}
	return ctx.Status(http.StatusCreated).JSON(&fiber.Map{"message":"registration complete"})
}


func (repo *Repository) ListOrganizerTickets(ctx *fiber.Ctx) error {
	tokenString := ctx.Get("Authorization")
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil{
		logger.DevLog("invalid token")
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid token"})
	}
	tickets := []models.Ticket{}
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err != nil{
		logger.DevLog("Beginning db session failed")
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"session closed"})
	}
	// get tickets belonging to an organizer
	if claims.Role != "organizer"{
		logger.DevLog("Unauthorized access")
		session.Rollback()
		return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error":"Unauthorized access"})
	}

	err = repo.DBConn.Where("organizer_id = ?", claims.UserID).Find(&tickets)
	if err!= nil{
		logger.DevLog("error getting ticket")
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"error fetching ticket"})
	}
	err = session.Commit()
	if err != nil{
		logger.DevLog("Transaction failed")
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"failed to commit data"})
	}
	return ctx.Status(http.StatusCreated).JSON(&fiber.Map{"tickets":tickets})
}


// need a fix. Too open for every ORGANIZER
func (repo *Repository) ConfirmRegistration(ctx *fiber.Ctx) error { // confirm by PUT after payment is done or something
	registration_id := ctx.Params("registration_id")
	tokenString := ctx.Get("Authorization")
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil{
		logger.DevLog(err)
		return nil
	}
	if claims.Role != "organizer"{
		logger.DevLog("Unauthorized")
		return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error":"Unauthorized access"})
	}
	register := models.Registration{}
	id := middleware.RandomIDGen(32)
	logger.DevLog(id)
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err!= nil{
		logger.DevLog("Begin session failed")
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"session closed"})
	}
	_, err = repo.DBConn.SQL("select * from registration left join ticket on registration.ticket_id = ticket.id left join organizer on ticket.organizer_id = organizer.id where registration.id=?", registration_id).Get(&register)
	if err!= nil{
		logger.DevLog("error getting registration")
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"registration failed"})
	}

	// Have to do Some plenty checks over here
	// ticket total, payment status, elapsed event date, etc total cost of tickets
	register.UpdatedAt = time.Now().Local()
	register.Status = models.Confirmed
	_, err = repo.DBConn.ID(register.ID).Update(&register)
	if err!= nil{
		logger.DevLog("registration status update failed")
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"registration status update failed"})
	}

	err = session.Commit()
	if err != nil{
		logger.DevLog("Transaction failed")
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"failed to commit data"})
	}
	return ctx.Status(http.StatusCreated).JSON(&fiber.Map{"message":register})
}


func (repo *Repository) GetEventRegistrations(ctx *fiber.Ctx) error{
	tokenString := ctx.Get("Authorization")
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil{
		logger.DevLog(err)
		return nil
	}
	if claims.Role != "organizer"{
		logger.DevLog("Unauthorized")
		return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error":"Unauthorized access"})
	}
	tickets := []models.Ticket{}
	registration := []models.Registration{}
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err != nil{
		logger.DevLog("Beginning db session failed")
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"session closed"})
	}
	err = repo.DBConn.Where("organizer_id = ?", claims.UserID).Find(&tickets)
	if err!= nil{
		logger.DevLog(err)
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"error fetching ticket"})
	}

	for _, ticket := range tickets{
		err := repo.DBConn.Where(" ticket_id = ?", ticket.ID).Find(&registration)
		if err != nil{
			logger.DevLog("table does not exist")
			return err
		}
		// Commit transaction
		err = session.Commit()
		if err != nil {
			return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{
				"error": "failed transaction",
			})
		}
	}
	// Commit transaction
	err = session.Commit()
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{
			"error": "failed transaction",
		})
	}
	logger.DevLog("----------------")
	return ctx.Status(http.StatusOK).JSON(&fiber.Map{"registration":registration})
}


func (repo *Repository) Routes(app *fiber.App) {
	api := app.Group("api")
	api.Get("/", func(c *fiber.Ctx) error {
		fmt.Println("Hello HTTP SERVER")
		return nil
	})

	api.Get("/events", repo.AllEvents)
	api.Post("/login", repo.LoginHandler)
	api.Post("/user/create", repo.CreateUser)
	api.Get("/event/:event_id", repo.GetEvent)
	api.Get("/search/event/:query", repo.SearchEvent)
	api.Post("/organizer/create", repo.CreateOrganizer)
	
	app.Use(middleware.JWTMiddleware())
	api.Get("/user/me", repo.GetUser)
	api.Post("/event/create", repo.CreateEvent)
	api.Get("/organizer/registrations", repo.GetEventRegistrations)
	api.Get("/organizer/me", repo.GetOrganizer)
	api.Get("/subevents", repo.SubscribedEvents)
	api.Put("/user/update", repo.UpdateUserProfile)
	api.Post("/ticket/create/:id", repo.CreateTicket)
	api.Get("/tickets", repo.ListOrganizerTickets)
	api.Put("/update/event/:event_id", repo.UpdateEvent)
	api.Post("/registration/user/:ticket_id", repo.AttendeeRegistration)
	api.Put("/registration/confirm/:registration_id", repo.ConfirmRegistration)
	api.Delete("/delete/event/:event_id", repo.DeleteEvent)
	api.Put("/event/booking/:event_id", repo.BookEvent)
	api.Put("/update/organizer/me", repo.UpdateOrganizerProfile)
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
			AppName:      "Events Suite",
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


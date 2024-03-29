package main

import (
	"fmt"
	"log"
	"net/http"

	// "os"
	"time"

	// "os"
	// "github.com/google/uuid"
	"github.com/google/uuid"
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



type  Event struct {
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



type  Organizer struct {			// Events Manager
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
	logger.DevLog(claims.UserID)
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

	_, err = repo.DBConn.Where(" organizer_i_d = ?", organizer_id).Get(&organizer)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "bad request"})
		log.Println(err)
		return err
	}
	if organizer_id != organizer.OrganizerID {
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": "invalid input ID",
		})
	}
	event.ID = uuid.New().String()
	event.CreatedAt = time.Now().Local()
	event.UpdatedAt = time.Now().Local()
	event.Organizer_id = organizer.OrganizerID

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
	_, err = session.Where(" organizer_i_d = ?", organizer_id).Update(&organizer)
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
	user.UserID = uuid.New().String()
	user.CreatedAt = time.Now().Local()
	user.UpdatedAt = time.Now().Local()
	// save user record
	_, err = repo.DBConn.Insert(user)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": fmt.Sprintf("error inserting user %s", err),
		})
		session.Rollback()
		return err
	}
	// Create stripe customer with user details
	customer := middleware.CreateCustomer(*user.Email, *user.Fullname)
	// assign values to login data variables
	loginData := models.LoginData{
		Email: *user.Email,
		Username: *user.Username,
		Password: *user.Password,
		Role: user.Role,
		Customer: customer.ID,
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
		logger.DevLog(user.Email)
		logger.DevLog(user.Role)
		regular_user := models.RegularUser{}
		_, err = repo.DBConn.Where("email = ? AND role = ?", user.Email, user.Role).Get(&regular_user)
		if err != nil{
			log.Println("token update failed")
			session.Rollback()
			return nil
		}
		// Generate jwt token
		token, err := middleware.GenerateToken(user.Email, user.Role, regular_user.UserID)
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
	token, err := middleware.GenerateToken(user.Email, user.Role, organizer.OrganizerID)
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
	err := ctx.BodyParser(organizer)
	if err != nil {
		logger.DevLog(err)
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
	organizer.UpdatedAt = time.Now().Local()
	organizer.OrganizerID = uuid.New().String()

	// hash password
	hashed_pass, err := middleware.HashPassword(*organizer.Password)
	if err != nil {
		log.Fatal("couldn't hash password")
		return err
	}
	organizer.Password = &hashed_pass
	organizer.Role = "organizer"
	// save user record
	_, err = repo.DBConn.Insert(organizer)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": err,
		})
		session.Rollback()
		logger.DevLog(err)
		return err
	}
	customer := middleware.CreateCustomer(*organizer.Email, *organizer.Name)
	logger.DevLog(*organizer)
	loginData := models.LoginData{
		Email: *organizer.Email,
		Phone: *organizer.Phone,
		Password: *organizer.Password,
		Role:	organizer.Role,
		Customer: customer.ID,
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
	event_id := ctx.Params("event_id")
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
	_, err = repo.DBConn.Where(" id = ? " ,event_id).Get(&event)
	if err != nil {
		log.Fatal("operation failed: ", err)
		session.Rollback()
		return err
	}
	ticket.Event_id = event.ID
	ticket.ID = uuid.New().String()
	ticket.Organizer_id = claims.UserID
	if event.ID == "" {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "such event doesn't exist"})
		session.Rollback()
		return err
	}
	exist, err := repo.DBConn.Where(" type=? AND event_id=?", ticket.Type, event.ID).Exist(&ticket)
	if err != nil {
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "invalid request"})
		session.Rollback()
		return err
	}
	logger.DevLog(exist)
	if exist {
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "this ticket type already exists for this event"})
		return err
	}

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
// func ParseUserID(userID uuid.UUID) uuid.UUID {
// 	var parsedID int
// 	fmt.Sscanf(userID, "%d", &parsedID)
// 	return parsedID
// }


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

    if claims.UserID == "" {
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
	_, err = repo.DBConn.Where(" id = ? ", event_id).Get(&event)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid event request"})
		session.Rollback()
		return err
	}
	// fetch user first in order to be able to update it * made this mistake earlier and it worried me a lot *
	_, err = repo.DBConn.Where(" user_i_d = ? ", claims.UserID).Get(&user)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid user action"})
		session.Rollback()
		return err
	}

    // Check if the association already exists
    exists, err := repo.DBConn.Where("user_i_d = ? AND event_i_d = ?", claims.UserID, event_id).Exist(&eventUser)
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
	if event.ID == "" {
		session.Rollback()
		logger.DevLog("no event referenced")
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "no event referenced"})
	}
	log.Println(user.UserID, event.ID)
	if claims.UserID == "" {
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "no user referenced"})
	}

	eventData := EventData{
		Data: &event,
	}

	// save user record
	_, err = repo.DBConn.Where(" user_i_d=? ", claims.UserID).Update(&user) // update user with new events
	if err != nil {
		session.Rollback()
		return err
	}

	// save event and user record
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
	_, err = repo.DBConn.Where(" user_i_d = ? ", user_id).Get(&userData)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid user request"})
		session.Rollback()
		return err
	}

	if userData.UserID != user_id {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid user ID"})
		session.Rollback()
		return err
	}

	// Commit transaction
	err = session.Commit()
	if err != nil {
		logger.DevLog("Transaction failed to commit")
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
	_, err = repo.DBConn.Where(" organizer_i_d = ? ", organizer_id).Get(&organizerData)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid organizer request"})
		session.Rollback()
		return err
	}

	if organizerData.OrganizerID != organizer_id {
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
	_, err = repo.DBConn.Where(" id = ? ", event_id).Get(&eventData)
	if err != nil {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid event request"})
		session.Rollback()
		return err
	}

	if eventData.ID != event_id {
		ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "invalid event ID"})
		session.Rollback()
		return err
	}

	// go middleware.Send_mail()
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
	event_id := ctx.Params("event_id")
	claims, err := middleware.GetIdFromToken(tokenString)
	if err != nil{
		logger.DevLog("Invalid token string")
		return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error":"Invalid token string"})
	}
	// check role based access
	if claims.Role != "organizer"{
		logger.DevLog("Unauthorized")
		return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error":"Unauthorized access"})
	}
	event := models.Event{}
	// Begin transaction
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err != nil{
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "server error"})
		return err
	}
	// Get event data for updating
	_, err = repo.DBConn.Where(" id = ? ", event_id).Get(&event)
	if err!= nil{
		log.Println("failed to retrieve event")
		ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "server error"})
		session.Rollback()
		return err
	}
	eventData := models.Event{}
	if err = ctx.BodyParser(&eventData); err != nil {
		ctx.Status(fiber.StatusBadRequest).JSON(&fiber.Map{"error": "Invalid request payload"})
		return nil
	}
	// update event attributes
	event.Title = eventData.Title
	event.Description = eventData.Description
	event.Location = eventData.Location
	event.StartDate = eventData.StartDate
	event.EndDate = eventData.EndDate
	event.UpdatedAt = time.Now().Local()
	_, err = repo.DBConn.Where(" id=? ", event_id).Update(&event)
	if err != nil {
		ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "failed to update event"})
		session.Rollback()
		return nil
	}
	// commt transaction
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
	claims, err := middleware.GetIdFromToken(tokenString)
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
	_, err = repo.DBConn.Where(" user_i_d = ? ", user_id).Get(&user)
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
	_, err = repo.DBConn.Where(" user_i_d = ? ", user_id).Update(&user)
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
	_, err = repo.DBConn.Where(" organizer_i_d = ? ", organizer_id).Get(&organizer)
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
	_, err = repo.DBConn.Where(" organizer_i_d = ? ", organizer_id).Update(&organizer)
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

	_, err = repo.DBConn.Where(" id = ? ", event_id).Get(&event)
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
	_, err = repo.DBConn.Where(" id = ? ", event_id).Delete(&event)
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


// func (repo *Repository) DeleteUserExpiredEvent(ctx *fiber.Ctx) error {
//     tokenString := ctx.Get("Authorization")
//     claims, err := middleware.GetIdFromToken(tokenString)
//     if err != nil {
//         logger.DevLog("error getting token")
//         return err
//     }

//     eventID := ctx.Params("event_id")

//     if claims.Role != "user" {
//         logger.DevLog("Unauthorized access")
//         return ctx.Status(http.StatusUnauthorized).JSON(&fiber.Map{"error": "unauthorized access"})
//     }

//     event := models.Event{}
//     user := models.RegularUser{}

//     // begin transaction
//     session := repo.DBConn.NewSession()
//     defer session.Close()
//     err = session.Begin()
//     if err != nil {
//         logger.DevLog("server error")
//         return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "server error"})
//     }

//     // get event by ID
//     _, err = repo.DBConn.ID(eventID).Get(&event)
//     if err != nil {
//         logger.DevLog("event unavailable")
//         session.Rollback()
//         return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "error deleting event"})
//     }

//     // get user by ID
//     _, err = repo.DBConn.ID(claims.UserID).Get(&user)
//     if err != nil {
//         logger.DevLog("user unavailable")
//         session.Rollback()
//         return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error": "error deleting event"})
//     }

//     // remove event from user's EventsAttending slice
//     userEvents, err := middleware.RemoveObjectFromSliceByIndex(user.EventsAttending, findEventIndexInSlice(user.EventsAttending, event.ID))
//     if err != nil {
//         logger.DevLog("error removing event from user's EventsAttending slice")
//         session.Rollback()
//         return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "error deleting event"})
//     }
//     user.EventsAttending = userEvents

//     // update user in the database
//     _, err = repo.DBConn.ID(claims.UserID).Update(&user)
//     if err != nil {
//         logger.DevLog("error updating user in database")
//         session.Rollback()
//         return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "error deleting event"})
//     }

//     // delete event from the database
//     _, err = repo.DBConn.ID(eventID).Delete(&event)
//     if err != nil {
//         logger.DevLog("error deleting event from database")
//         session.Rollback()
//         return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "error deleting event"})
//     }

//     err = session.Commit()
//     if err != nil {
//         logger.DevLog("error committing to DB")
//         return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error": "error committing to DB"})
//     }

//     return ctx.Status(http.StatusOK).JSON(&fiber.Map{"msg": "event delete success"})
// }

// // Helper function to find the index of an event in a slice of event IDs
// func findEventIndexInSlice(eventIDs []string, eventID string) int {
//     for i, id := range eventIDs {
//         if id == eventID {
//             return i
//         }
//     }
//     return -1
// }


// get users subscribed to your events
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

	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err!= nil{
		logger.DevLog("Begin session failed")
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"session closed"})
	}

	err = repo.DBConn.Find(&event_user)
	if err != nil{
		logger.DevLog("operation failed")
		session.Rollback()
		return err
	}

	for e, event := range event_user {
		err = repo.DBConn.Where("id = ? AND organizer_id = ?",event.EventID, organizer_id).Find(&events)
		if err != nil{
			logger.DevLog("invalid")
			return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid request"})
		}
		err = repo.DBConn.Where("user_i_d = ?", event.UserID).Find(&users)
		if err != nil{
			logger.DevLog("invalid")
			return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"invalid request"})
		}
		logger.DevLog(*users[e].Email)
		ctx.JSON(&fiber.Map{"booked events":users[e]})
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
	logger.DevLog(&events)
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
	eventUser := models.EventUser{}
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

	evnt := models.Event{}
	_, err = repo.DBConn.Where(" id = ? ", ticket.Event_id).Get(&evnt)
	if err!= nil{
		logger.DevLog("error during registration")
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"registration failed"})
	}

    // Check if the association already exists
    exists, err := repo.DBConn.Where("user_i_d = ? AND event_i_d = ?", claims.UserID, evnt.ID).Exist(&eventUser)
    if err != nil {
        return err
    }
    if !exists {
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{
			"error": "event already registered. re-book",
		})
    }

	ticketsAvailable, err := middleware.TicketAvailable(string(ticket.Type), register.Quantity, ticket.QuantityAvailable)
	if err != nil{
		logger.DevLog("Ticket exhausted")
		return err
	}
	if ticket.Event_id != evnt.ID{
		log.Println(ticket.Event_id, evnt.ID)
	}

	// update tickets_available
	ticket.QuantityAvailable = ticketsAvailable
	_, err = repo.DBConn.Where(" id = ? ", ticket.ID).Update(&ticket)
	if err!= nil{
		logger.DevLog("error updating ticket")
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"error updating ticket"})
	}
	// register.ID = middleware.RandomIDGen(32) // random ID generator
	register.User_id = claims.UserID
	register.Status = models.Pending
	register.Ticket_id = ticket.ID
	register.Event_id = ticket.Event_id
	register.CreatedAt = time.Now().Local()
	register.UpdatedAt = time.Now().Local()
	register.ID = uuid.New().String()

	// Create event registration
	_, err = repo.DBConn.Insert(&register) //
	if err!= nil{
		logger.DevLog("error during registration")
		logger.DevLog(err)
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"registration failed"})
	}
	cost_incurred := float64(register.Quantity) * ticket.Price
	logger.DevLog(cost_incurred)
	middleware.HandleCreatePaymentIntent(int64(cost_incurred), ctx)

	//  delete event association after registration
	_, err = repo.DBConn.Where("user_i_d = ? AND event_i_d = ?", claims.UserID, evnt.ID).Delete(&eventUser)
	if err != nil{
		logger.DevLog("event reset")
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"event reset"})
	}
	logger.DevLog("Event association  deleted...")

	err = session.Commit()
	if err != nil{
		logger.DevLog("Transaction failed")
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"failed to commit data"})
	}
	return ctx.Status(http.StatusCreated).JSON(&fiber.Map{"message":"registration complete"})
}


func (repo *Repository) ListTicketsByEvents(ctx *fiber.Ctx) error {
	event_id := ctx.Params("event_id")

	tickets := []models.Ticket{}
	session := repo.DBConn.NewSession()
	defer session.Close()
	err := session.Begin()
	if err != nil{
		logger.DevLog("Beginning db session failed")
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"session closed"})
	}

	err = repo.DBConn.Where("event_id = ?", event_id).Find(&tickets)
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
	session := repo.DBConn.NewSession()
	defer session.Close()
	err = session.Begin()
	if err!= nil{
		logger.DevLog("Begin session failed")
		return ctx.Status(http.StatusInternalServerError).JSON(&fiber.Map{"error":"session closed"})
	}

	_, err = repo.DBConn.SQL("select * from registration left join ticket on registration.ticket_id = ticket.id left join organizer on ticket.organizer_id = organizer.organizer_i_d where registration.id=?", registration_id).Get(&register)
	if err!= nil{
		logger.DevLog("error getting registration")
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"registration failed"})
	}

	tickets := []models.Ticket{}
	err = repo.DBConn.Where("id = ?", register.Ticket_id).Find(&tickets)
	if err!= nil{
		logger.DevLog(err)
		session.Rollback()
		return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"error fetching ticket"})
	}
	for _, ticket := range tickets{
		if ticket.Organizer_id != claims.UserID{
			logger.DevLog(err)
			return ctx.Status(http.StatusBadRequest).JSON(&fiber.Map{"error":"request not allowed"})
		}		
	}

	// Have to do Some plenty checks over here
	// ticket total, payment status, elapsed event date, etc total cost of tickets
	register.UpdatedAt = time.Now().Local()
	register.Status = models.Confirmed
	_, err = repo.DBConn.Where(" id = ? ", register.ID).Update(&register)
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


func (repo *Repository) GetRegisteredEvents(ctx *fiber.Ctx) error{
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

	api.Post("/login", repo.LoginHandler)
	api.Get("/search/event/:query", repo.SearchEvent)
	api.Get("/tickets/:event_id", repo.ListTicketsByEvents)
	
	// User route
	user:= app.Group("user")
	user.Post("/create", repo.CreateUser)
	user.Use(middleware.JWTMiddleware())
	user.Get("/me", repo.GetUser)
	user.Put("/update", repo.UpdateUserProfile)
	
	// Organizer routes
	organizer := app.Group("organizer")
	organizer.Post("/create", repo.CreateOrganizer)
	organizer.Use(middleware.JWTMiddleware())
	organizer.Get("/me", repo.GetOrganizer)
	organizer.Get("/registrations", repo.GetRegisteredEvents)
	organizer.Put("/update", repo.UpdateOrganizerProfile)
	
	// Event routes
	event:=app.Group("event")
	event.Get("/:event_id", repo.GetEvent)
	event.Get("/events", repo.AllEvents)
	event.Use(middleware.JWTMiddleware())
	event.Post("/create", repo.CreateEvent)
	event.Get("/subevents", repo.SubscribedEvents)
	event.Put("/update/:event_id", repo.UpdateEvent)
	event.Delete("/delete/:event_id", repo.DeleteEvent)
	event.Put("/book/:event_id", repo.BookEvent)

	// Registration routes
	registration:=app.Group("register")
	registration.Use(middleware.JWTMiddleware())
	registration.Put("/confirm/:registration_id", repo.ConfirmRegistration)
	registration.Post("/ticket/create/:event_id", repo.CreateTicket)
	registration.Post("/event/ticket/:ticket_id", repo.AttendeeRegistration)
	// api.Delete("/user/event/delete/:event_id", repo.DeleteUserEvent)
}


func main() {
	logger.DevLog("Hello World, An Event Manager Cooking")
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

	logger.DevLog("FS package")
	engine, err := models.DBConnection()
	if err != nil {
		logger.DevLog(err)
		log.Fatal("DB connection failed", err)
	}
	r := Repository{
		DBConn: engine,
	}
	r.Routes(app)
	app.Listen(":5500")
}


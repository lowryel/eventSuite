package middleware

import (
	"crypto/rand"
	"encoding/binary"
	"errors"

	"log"
	// "math/big"

	logger "github.com/lowry/eventsuite/Logger"
	"github.com/lowry/eventsuite/Models"
)


func ValidateEvent(event *models.Event) error {

  if event.Title == nil {
    return errors.New("title is required")
  }

  return nil
}


func ValidateUser(user *models.RegularUser) (error) {
	if user.Email == nil {
    return errors.New("email is required")
  }

  if user.Password == nil {
    return errors.New("password is required")
  }

  return nil
}


func TicketAvailable(ticketType string, quantity, total_available int) (int, error){
  var err error
  if quantity > total_available{
    logger.DevLog("ticket exhausted")
    log.Panic("ticket out of stock")
    return 0, nil
  }
  switch ticketType{
  case "Student":
    total_available = total_available - quantity
  case "VIP":
    total_available = total_available - quantity
  case "General Admission":
    total_available = total_available - quantity
  case "Early Bird":
    total_available = total_available - quantity
  default:
  }
  return total_available, err
}

func RandomIDGen(id uint32) uint32 {
  binary.Read(rand.Reader, binary.LittleEndian, &id)

  logger.DevLog(id)
  return id
}


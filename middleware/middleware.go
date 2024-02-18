package middleware

import (
	"github.com/lowry/eventsuite/Models"
	"errors"
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




package middleware

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt"
	logger "github.com/lowry/eventsuite/Logger"
	models "github.com/lowry/eventsuite/Models"

	// logger "github.com/lowry/eventsuite/Logger"
	"golang.org/x/crypto/bcrypt"

	// _ "github.com/joho/godotenv"
	"github.com/patrickmn/go-cache"
	// "xorm.io/xorm"
)

// JWTClaims struct represents the claims for JWT
type JWTClaims struct {
  UserID      uint32       `json:"id"`
  Email       string    `json:"email"`
  Username    string 		`json:"username"`
  Role        string    `json:"role"`
  jwt.StandardClaims
}


func HashPassword(pass string)(string, error){
	hash, err :=bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal("")
		return "", err
	}
	return string(hash), nil
}

func HashesMatch(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	// Compare the two hashes
	return err
}

// generate jwt token
func GenerateToken(email, role string, userId uint32) (string, error) {
    claims := JWTClaims{
      UserID: userId,
      Email: email,
      Role: role,
      StandardClaims: jwt.StandardClaims{
        ExpiresAt: time.Now().Add(time.Hour * 24).Unix(),
      },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, err := token.SignedString([]byte(os.Getenv("SECRET_KEY")))
    if err != nil {
        return "", err
    }
    return tokenString, nil
}

// DecodeToken decodes and validates the JWT token and extracts the claims.
func DecodeToken(tokenString string) (*JWTClaims, error) {
    // Parse and validate the JWT token
    token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(os.Getenv("SECRET_KEY")), nil
    })
    if err != nil || !token.Valid {
        return nil, fmt.Errorf("invalid or expired token: %v", err)
    }

    // Extract the claims from the token
    claims, ok := token.Claims.(*JWTClaims)
    if !ok {
        return nil, fmt.Errorf("failed to extract claims from token")
    }

    return claims, err
}

func GetIdFromToken(tokenString string) (*JWTClaims, error){
  claims, err := DecodeToken(tokenString[7:])
  if err != nil{
    log.Println(err)
  }
  return claims, err
}


// JWTMiddleware checks the JWT token in the request headers
// JWTMiddleware caches the parsed and validated token 
// for performance
func JWTMiddleware() fiber.Handler {
  tokenCache := cache.New(5*time.Minute, 10*time.Minute)
  return func(c *fiber.Ctx) error {
    tokenString := c.Get("Authorization")
    if tokenString==""{
      log.Println("token required")
      return c.Status(http.StatusUnauthorized).JSON(&fiber.Map{"message":"Unauthorized"})
    }
    tokenString=tokenString[7:]
    // Check cache
    if token, ok := tokenCache.Get(tokenString); ok {
      c.Locals("user", token)
      return c.Next()
    }
    
    token, err := parseAndValidateToken(tokenString)
    if err != nil {
      log.Printf("Token invalid error: %s\n", err)
      return c.Status(http.StatusBadRequest).JSON(&fiber.Map{
        "message": "Invalid token"})
    }
    // Cache valid token
    tokenCache.Set(string(tokenString), token, 0)
    // Extract and store user identity
    user := token.Claims.(jwt.MapClaims) 
    c.Locals("user", user)
    return c.Next()
  }
}

// Centralized parsing and validation
func parseAndValidateToken(tokenString string) (*jwt.Token, error) {
  token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["Token"])
    }
    return []byte(os.Getenv("SECRET_KEY")), nil
  })
  if err != nil {
    return nil, err 
  }

  // Additional validation checks
  if !token.Valid {
    return nil, ErrorToken
  }
  return token, nil
}
// Custom error types
var (
  ErrorToken= errors.New("token expired")
)


// ProtectedRoute is a protected route that requires a valid JWT token
// func ProtectedRoute(c *fiber.Ctx) error {
//     // user := c.Locals("root").(*jwt.MapClaims)
//     return c.JSON(fiber.Map{"message": "Hello, welcome to a protected route!"})
// }


type Student struct{
  Name string
  Class int
}

type Teacher struct{
  Student []Student
}

func SomeShii(){
  student := Student{
    Name: "Eugene",
    Class: 5,
  }
  teacher := Teacher{
    Student: []Student{
      student,
    },
  }
  fmt.Println(teacher)
  // tea := make(map[string]string)
  // tea := make(map[Student]string)
  // tea[student] = "Eugene"
  // logger.DevLog(tea)
  // delete(tea, student)
  for _, tea := range teacher.Student{
    logger.DevLog(tea.Class)
    slice:= make([]string, 0)
    slice = append(slice, tea.Name)
    logger.DevLog(slice)
  }
  // logger.DevLog(tea)
}

func EventsAttending(Events []*models.Event, event uint32, another_id uint32) ([]*models.Event, error) {
  if event <= 0{
    return nil, errors.New("event missing")
  }
  keys:= Events
  eventMap := make(map[uint32]models.Event)
  eventMap[event] = *Events[0]
  logger.DevLog(eventMap)
  delete(eventMap, event)
  logger.DevLog(eventMap)
  for _, use := range eventMap{
    keys = append(keys, &use)
  }
  logger.DevLog(keys)
  return Events, nil
}


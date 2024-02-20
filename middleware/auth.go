package middleware

import (
	"log"
	"os"
	"time"
	"fmt"
	"errors"
	"net/http"


	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
	"github.com/gofiber/fiber/v2"

	// _ "github.com/joho/godotenv"
	"github.com/patrickmn/go-cache"
	// "xorm.io/xorm"
)

// JWTClaims struct represents the claims for JWT
type JWTClaims struct {
    Email       string    	`json:"email"`
    Username string 		`json:"username"`
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

// generate jwt token with username and email
func GenerateToken(email, username string ) (string, error) {
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, &JWTClaims{
        Username: username,
        StandardClaims: jwt.StandardClaims{
            ExpiresAt: time.Now().Add(time.Hour * 24).Unix(), // Token expires in 24 hours
        },
    })
    tokenString, err := token.SignedString([]byte(os.Getenv("SECRET_KEY")))
    if err != nil {
        return "", err
    }
    return tokenString, nil
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

// func InserToken(db *xorm.Engine, tableName string, token string, username string) (int64, error){
//     // SQL statement
//     // query := ( "UPDATE regular_user SET token = ? WHERE username = ?", token, username)
//     // Execute the SQL statement
//     _ = db.SQL("UPDATE regular_user SET token = ? WHERE username = ?", token, username)
//     // if err != nil{
//     //     return 0, nil
//     // }
//     return 0, nil
// }

// ProtectedRoute is a protected route that requires a valid JWT token
// func ProtectedRoute(c *fiber.Ctx) error {
//     // user := c.Locals("root").(*jwt.MapClaims)
//     return c.JSON(fiber.Map{"message": "Hello, welcome to a protected route!"})
// }




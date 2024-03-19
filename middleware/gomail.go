package middleware


import (
	"github.com/go-mail/mail"
)

func Send_mail() {

	m := mail.NewMessage()

	m.SetHeader("From", "eugeneagbaglo@gmail.com")

	m.SetHeader("To", "ellowry09@gmail.com") // can send to many users

	m.SetAddressHeader("Cc", "eugeneagbaglo@gmail.com", "Oliver")

	m.SetHeader("Subject", "Hello!")

	m.SetBody("text/html", "Hello <b>Kate</b> and <i>Noah</i>!")

	m.Attach("lolcat.jpg")

	d := mail.NewDialer("smtp.gmail.com", 587, "eugeneagbaglo@gmail.com", "")

	// Send the email to Kate, Noah and Oliver.
	if err := d.DialAndSend(m); err != nil {
		panic(err)
	}
}
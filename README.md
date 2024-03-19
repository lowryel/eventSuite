## Event Management System - eventSuite

- eventSuite is an event management API built with Golang, Postgres, XORM, and Fiber framework. It allows users to create, manage, and attend events of various types and categories. It also supports ticketing, registration, and payment features.

### Motivation
- I created this project as a way to learn and practice Golang and web development. I wanted to build a full-featured and scalable API that can handle complex business logic and data models. I also wanted to explore different technologies and frameworks that can help me achieve my goals.

### Features
- CRUD operations for events, users, organizers, tickets, and registrations
- Authentication and authorization using JWT and middleware
- Validation and error handling using custom structs and methods
- Database connection and ORM using XORM and Postgres
- Routing and web framework using Fiber

### Installation
- To install and run this project, you need to have the following tools and dependencies:

- Golang
- Postgres
- XORM
- Fiber
- Other packages listed in go.mod file

- You can clone this repository using the following command:
- `git clone https://github.com/lowryel/eventSuite.git`

- Then, you need to create a database and a user in Postgres and grant the user all privileges on the database. You can use the following commands as an example:

  `CREATE DATABASE eventsuite;
    CREATE USER eventsuite WITH PASSWORD 'eventsuite';
    GRANT ALL PRIVILEGES ON DATABASE eventsuite TO eventsuite;`

- Next, you need to create a .env file in the root directory of the project and add the following environment variables:
  
  `DB_HOST=localhost
    DB_PORT=5432
    DB_USER=eventsuite
    DB_PASSWORD=eventsuite
    DB_NAME=eventsuite
    JWT_SECRET=eventsuite`

- You can change the values of these variables according to your configuration.

- Finally, you can run the project using the following command:

`go run main.go`

- This will start the server on port 3000. You can access the API using http://localhost:5500.

### Usage
* The eventSuite API offers the following endpoints:

`   Post        api/login
	Get     api/search/event/:query
	Get     api/tickets/:event_id`

 `   // User route
    Post       /user/create
    user.Use(middleware.JWTMiddleware())
    Get        /user/me
    Put        /user/update`
    
 `   // Organizer routes
    Post      /organizer/create
    organizer.Use(middleware.JWTMiddleware())
    Get       /organizer/me
    Get       /organizer/registrations
    Put       /organizer/update`
    
 `   // Event routes
    Get                 /event/:event_id
    Get                 /event/events
    event.Use(middleware.JWTMiddleware())
    Post                /event/create
    Get                 /event/subevents
    Put                 /event/update/:event_id
    Delete            /event/delete/:event_id
    Put                 /event/book/:event_id`

 `   // Registration routes
    registration.Use(middleware.JWTMiddleware())
    Put    /register/confirm/:registration_id
    Post   /register/ticket/create/:event_id
    Post   /register/event/ticket/:ticket_id
`

- You can use tools like Postman or curl to make requests and get responses from the API. Here is an example of how to create an event using curl:

```JSON
    curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer <token>" -d '{"title":"Golang Meetup","description":"A meetup for Golang enthusiasts","start_date":"2024-03-10T18:00:00Z","end_date":"2024-03-10T20:00:00Z","location":"Accra, Ghana","organizer_id":1}' http://localhost:5500/api/event/create
```
- This will return a response like this:
  ```JSON
    {
        "id": 1,
        "title": "Golang Meetup",
        "description": "A meetup for Golang enthusiasts",
        "start_date": "2024-03-10T18:00:00Z",
        "end_date": "2024-03-10T20:00:00Z",
        "location": "Accra, Ghana",
        "organizer_id": 1,
        "attendees": []
    }

  ```
#### Certbot - for provisioning ssl certificates
#### AWS WorkMail - for processing customer emails



##### Some Issues Encountered on the way
- Creating a third table known as Junction Table to associate two related tables. 
    I was initially relying on my knowledge from Python-Django deevelopment but then
    discovered you need a third table to store the association. That's where the junction table came in.

- As this is my first major project in Golang, setting up authentication was a bit tricky.
  Even using it to protect the routes was a bit daunting but I figured the best way to learn is to practice.
  I also got help from some online resources and GhatGPT free tier.

- Creating two set of users. While building, I realized I have to create two set of users (Organizer and Regular users).
    So I needed to implement role based access to give certain level of permission. That lead me to (organizer and user) roles

- Another shortfall I faced is clearing an event after is was registered. My initial thought was to delete the 
  event from the user event list after the event is registered but that was impossible to handle. Then I remembered
  I have user and event association in a table (event_user). This table stores user_id with the event booked event_id.
  So I just clear the association from that table and handled the edge cases such as registering same event more than once.

# DEVELOPER SECTION
-----------------------------------------
###### Full text search
- This query indicate that, use whatever the queryParam carries to search through the records and retrieve all the variables b/n SELECT and FROM
```go
    err := DBConn.SQL("SELECT id, title, description, location, start_date, end_date, organizer_id FROM event WHERE to_tsvector(coalesce(start_date, '') || title || ' ' || coalesce(description, '')) @@ websearch_to_tsquery(?)", queryParam).Find(&events)
```

<!-- JOIN query of 3 models -->
* querying 3 models data at a time instead of 3 separate times reduces trips to the db hence improving performance *
```sql
    select * from registration left join ticket on registration.ticket_id = ticket.id left join organizer on ticket.organizer_id = organizer.id where registration.id = ?;
```


##### Event:

                   - id: Unique identifier for the event (integer or UUID)
                   - title: Title of the event
                   - description: Description of the event
                   - start_date: Start date and time of the event (timestamp)
                   - end_date: End date and time of the event (timestamp)
                   - location: Location of the event (e.g., address)
                   - organizer_id: Foreign key referencing the organizer of the event (integer)
                   - attendees: List of users attending the event (many-to-many relationship with User model)

##### User:

                   - id: Unique identifier for the user (integer or UUID)
                   - username: Username of the user
                   - email: Email address of the user
                   - password: Hashed password of the user
                   - full_name: Full name of the user
                   - organization: Organization or company the user belongs to
                   - events_attending: List of events the user is attending (many-to-many relationship with Event model)

##### Organizer:

                   - id: Unique identifier for the organizer (integer or UUID)
                   - name: Name of the organizer
                   - description: Description of the organizer or organization
                   - contact_email: Contact email address of the organizer
                   - contact_phone: Contact phone number of the organizer
                   - events_managed: List of events managed by the organizer (one-to-many relationship with Event model)

##### Ticket:

                   - id: Unique identifier for the ticket (integer or UUID)
                   - event_id: Foreign key referencing the event associated with the ticket (integer)
                   - type: Type of the ticket (e.g., general admission, VIP)
                   - price: Price of the ticket
                   - quantity_available: Number of tickets available
                   - start_sale_date: Start date and time of ticket sales (timestamp)
                   - end_sale_date: End date and time of ticket sales (timestamp)

##### Registration:

                   - id: Unique identifier for the registration (integer or UUID)
                   - user_id: Foreign key referencing the user registering for the event (integer)
                   - event_id: Foreign key referencing the event being registered for (integer)
                   - ticket_id: Foreign key referencing the ticket type being registered for (integer)
                   - quantity: Number of tickets registered
                   - registration_date: Date and time of registration (timestamp)
                   - status: Status of the registration (e.g., pending, confirmed)


* These models should provide a solid foundation for building an Event Management API. Depending on your specific requirements, you may need to add more attributes or additional models to support features such as event categories, venues, sessions, speakers, sponsorships, payments, etc. *


##### Some Guides to follow
------------------------------------------------
* Event Creation and Management:
    Allow users to create, manage, and customize event listings with details such as title, description, date, time, location, and ticket types.

* Ticketing and Registration:
    Enable event organizers to set up various ticket types (e.g., general admission, VIP, early bird) with different prices, quantities, and availability dates.
    Provide attendees with the ability to register for events, purchase tickets, and receive electronic tickets via email or mobile app.

* Promotion and Marketing:
    Offer tools for event promotion, including customizable event pages, social media integration, email campaigns, and discount codes.
    Allow organizers to track and analyze event promotion efforts through built-in analytics and reporting features.

* Attendee Management:
    Provide organizers with attendee management tools, such as guest lists, check-in apps, and attendee communication capabilities.
    Allow attendees to view event details, manage their tickets, and communicate with event organizers.

* Event Discovery:
    Offer a searchable event directory or calendar where users can discover events based on criteria like location, date, category, and interests.
    Provide personalized recommendations and suggestions for events based on user preferences and past attendance history.

* Payment Processing:
    Integrate with payment gateways to securely process online payments for ticket purchases and event fees.
    Support various payment methods, including credit/debit cards, PayPal, and other digital wallets.

* Mobile Experience:
    Develop a mobile-responsive website or native mobile app that allows users to browse, register for, and manage events on their smartphones and tablets.
    Offer features such as event reminders, push notifications, and in-app messaging for enhanced user engagement.

* Analytics and Insights:
    Provide event organizers with detailed analytics and insights into event performance, ticket sales, attendee demographics, and engagement metrics.
    Offer real-time reporting dashboards and downloadable reports to help organizers make informed decisions and optimize their events.

* Customization and Branding:
    Allow event organizers to customize event pages, registration forms, and email communications with their branding elements, logos, and colors.
    Provide flexible design templates and themes to match the look and feel of different types of events and organizations.

* Security and Compliance:
    Implement robust security measures to protect user data, payment information, and event content against unauthorized access and cyber threats.
    Ensure compliance with relevant data protection regulations (e.g., GDPR, CCPA) and industry standards for online ticketing and event management platforms.


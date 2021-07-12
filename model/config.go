package model

type Twilio struct {
	TwilioAccountSid string
	TwilioAuthToken  string
	PhoneNumber string
}

type Config struct {
	Twilio Twilio
	Avalon AvalonDetails
	Mongo  Mongo
}

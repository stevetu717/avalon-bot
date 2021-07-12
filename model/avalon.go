package model


type AvalonDetails struct {
	Amenities map[string]Amenity `mapstructure:"amenities"`
	LeaseId string
	PersonId string
	ReservationName string
	Username string
	Password string
}

type Amenity struct{
	Key string
	Name string
	Id string
}
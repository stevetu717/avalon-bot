package model

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type Reservation struct {
	Id primitive.ObjectID 			`bson:"_id"`
	Datetime time.Time 				`bson:"date_time"`
	Activity string 				`bson:"activity"`
	CreatedBy string				`bson:"created_by"`
}
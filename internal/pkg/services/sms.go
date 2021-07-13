package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/sfreiberg/gotwilio"
	"github.com/sirupsen/logrus"
	"github.com/stevetu717/racquetball-bot/internal/pkg/util"
	"github.com/stevetu717/racquetball-bot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
	"strings"
	"time"
)

type SMSHandler struct {
	logger        *logrus.Logger
	db            *mongo.Collection
	twilio        *gotwilio.Twilio
	avalonService *AvalonService
	config        *model.Config
}

func NewSMSHandler(logger *logrus.Logger, db *mongo.Collection, twilio *gotwilio.Twilio, avalonService *AvalonService, config *model.Config) *SMSHandler {
	return &SMSHandler{logger, db, twilio, avalonService, config}
}

func (sms *SMSHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	userPhoneNumber := r.FormValue("From")
	body := strings.ToLower(r.FormValue("Body"))
	util.LogInfo(sms.logger, "========== Received Text Message ==========")

	util.LogInfo(sms.logger, "From: "+userPhoneNumber)
	util.LogInfo(sms.logger, "Message: "+body)

	if strings.Contains(body, "racquetball") || strings.Contains(body, "tennis") || strings.Contains(body, "basketball") {
		util.LogInfo(sms.logger, "========== BEGIN SCHEDULE WORKFLOW ==========")
		err := sms.handleScheduleSMS(body, userPhoneNumber)
		if err != nil {
			util.LogInfo(sms.logger, "========== END SCHEDULE WORKFLOW ==========")
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte("Internal Server Error"))
			return
		}
	} else if strings.Contains(body, "assist") {
		err := sms.sendSMS(util.SmsHelp, userPhoneNumber)
		if err != nil {
			util.LogSMSError(sms.logger, err, userPhoneNumber, util.SmsHelp)
			rw.WriteHeader(http.StatusInternalServerError)
			util.LogInfo(sms.logger, "========== END SCHEDULE WORKFLOW ==========")
			_, _ = rw.Write([]byte("Internal Server Error"))
			return
		}
	} else {
		message := "Please enter a valid command to the avalon activity reservation system. Text 'assist' for help."
		err := sms.sendSMS(message, userPhoneNumber)
		if err != nil {
			util.LogDebug(sms.logger, "unable to send sms: "+util.SmsHelp+" -  to: "+userPhoneNumber)
			util.LogError(sms.logger, err)
			util.LogInfo(sms.logger, "========== END SCHEDULE WORKFLOW ==========")
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte("Internal Server Error"))
			return
		}
	}
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte("OK"))
	util.LogInfo(sms.logger, "========== END SCHEDULE WORKFLOW ==========")
	return
}

func (sms *SMSHandler) handleScheduleSMS(body string, userPhoneNumber string) error {
	dateTime, activity, err := sms.parseScheduleSMS(body, userPhoneNumber)
	if err != nil {
		return err
	}

	reservation := &model.Reservation{Id: primitive.NewObjectID(), Datetime: dateTime, Activity: activity, CreatedBy: userPhoneNumber}

	if dateTime.Before(time.Now().UTC()) {
		smsErr := sms.sendSMS(util.SmsInvalidDateTime, userPhoneNumber)
		if smsErr != nil {
			util.LogSMSError(sms.logger, err, userPhoneNumber, util.SmsInvalidDateTime)
			return smsErr
		}
	} else if util.DateTimeWithinTwoDays(dateTime) {
		util.LogInfo(sms.logger, "Reservation is within two days. Attempting to make reservation now...")
		err := sms.avalonService.MakeReservation(reservation)

		if err != nil {
			body := fmt.Sprintf(util.SmsFailedReservation, reservation.Activity, reservation.Datetime.Local().Format(util.ReservationDateTimeLayout))
			util.LogError(sms.logger, body)
			smsErr := sms.sendSMS(body, userPhoneNumber)
			if smsErr != nil {
				util.LogSMSError(sms.logger, err, userPhoneNumber, body)
				return smsErr
			}
			return err
		} else {
			body := fmt.Sprintf(util.SmsSuccessfulReservation, reservation.Activity, reservation.Datetime.Local().Format(util.ReservationDateTimeLayout))
			smsErr := sms.sendSMS(body, userPhoneNumber)
			if smsErr != nil {
				util.LogSMSError(sms.logger, err, userPhoneNumber, body)
				return smsErr
			}
			util.LogInfo(sms.logger, body)
		}
	} else {
		util.LogInfo(sms.logger, "Saving reservation to database...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err = sms.db.InsertOne(ctx, reservation)

		if err != nil {
			util.LogDebug(sms.logger, "An error occurred while saving reservation to db")
			util.LogError(sms.logger, err)
			smsErr := sms.sendSMS(util.ReservationError, userPhoneNumber)
			if smsErr != nil {
				util.LogSMSError(sms.logger, err, userPhoneNumber, util.ReservationError)
				return smsErr
			}
			return err
		}

		sms.ScheduleJob(reservation, sms.db, sms.avalonService)

		err = sms.sendSMS(util.ReservationSaved, userPhoneNumber)
		if err != nil {
			util.LogSMSError(sms.logger, err, userPhoneNumber, util.ReservationSaved)
			return err
		}
	}
	return nil
}

func (sms *SMSHandler) parseScheduleSMS(body string, userPhoneNumber string) (time.Time, string, error) {
	dateTime, err := util.GetDateTimeUTC(body)

	if err != nil {
		util.LogError(sms.logger, err)
		smsErr := sms.sendSMS(util.SmsInvalidDateTime, userPhoneNumber)
		if smsErr != nil {
			util.LogSMSError(sms.logger, err, userPhoneNumber, util.SmsInvalidDateTime)
			return time.Time{}, "", smsErr
		}
		return time.Time{}, "", err
	}

	if dateTime.Hour() > 0 && dateTime.Hour() < 12 {
		smsErr := sms.sendSMS(util.SmsInvalidDateTimeRange, userPhoneNumber)
		if smsErr != nil {
			util.LogSMSError(sms.logger, err, userPhoneNumber, util.SmsInvalidDateTimeRange)
			return time.Time{}, "", smsErr
		}
		return time.Time{}, "", errors.New("Invalid time range: " + dateTime.Local().Format("3:04 PM"))
	}

	activity, err := util.GetActivity(body)

	if err != nil {
		util.LogError(sms.logger, err)
		smsErr := sms.sendSMS(util.SmsInvalidActivity, userPhoneNumber)
		if smsErr != nil {
			util.LogSMSError(sms.logger, err, userPhoneNumber, util.SmsInvalidActivity)
			return time.Time{}, "", smsErr
		}
		return time.Time{}, "", err
	}
	return dateTime, activity, nil
}

func (sms SMSHandler) getAction(body string) string {
	if util.ContainsIgnoreCase(body, util.Schedule) {
		return util.Schedule
	}
	return ""
}

func (sms *SMSHandler) sendSMS(message string, userPhoneNumber string) error {
	util.LogInfo(sms.logger, fmt.Sprintf("Sending SMS '%s' to %s", message, userPhoneNumber))
	_, _, err := sms.twilio.SendMMS(sms.config.Twilio.PhoneNumber, userPhoneNumber, message, nil, "", "")
	if err != nil {
		return err
	}
	return nil
}

func (sms *SMSHandler) ScheduleJob(r *model.Reservation, collection *mongo.Collection, avalonService *AvalonService) {
	duration := util.TimeFromNow(r.Datetime)
	timer := time.NewTimer(duration)
	util.LogInfo(sms.logger, "Will attempt to make Reservation "+r.Id.Hex()+" on Avalon.com at "+time.Now().Add(duration).String())

	go func() {
		<-timer.C
		ctx := context.Background()
		util.LogInfo(sms.logger, "Attempting to make Reservation "+r.Id.Hex()+" on Avalon.com ...")
		err := avalonService.MakeReservation(r)

		if err != nil {
			util.LogDebug(sms.logger, "FAIL: Failed to make Reservation on Avalon.com")
			body := fmt.Sprintf(util.SmsFailedReservation, r.Activity, r.Datetime.Local().Format(util.ReservationDateTimeLayout))
			err = sms.sendSMS(body, r.CreatedBy)
			if err != nil {
				util.LogSMSError(sms.logger, err, r.CreatedBy, body)
			}
		} else {
			util.LogInfo(sms.logger, "SUCCESS: Successfully made Reservation on Avalon.com for reservation:"+r.Id.Hex())
			body := fmt.Sprintf(util.SmsSuccessfulReservation, r.Activity, r.Datetime.Local().Format(util.ReservationDateTimeLayout))
			err = sms.sendSMS(body, r.CreatedBy)
			if err != nil {
				util.LogSMSError(sms.logger, err, r.CreatedBy, body)
			}
		}

		util.LogInfo(sms.logger, "Attempting to remove Reservation from database...")
		err = removeJob(ctx, r, collection, sms.logger)

		if err == nil {
			util.LogInfo(sms.logger, "SUCCESS: Removed Reservation from database")
		}
	}()
}

func removeJob(ctx context.Context, reservation *model.Reservation, collection *mongo.Collection, logger *logrus.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := collection.DeleteOne(ctx, bson.M{"_id": reservation.Id})
	if err != nil {
		util.LogDebug(logger, "unable to delete reservation: "+reservation.Id.Hex())
		util.LogError(logger, err)
		return err
	}

	util.LogInfo(logger, "Deleted Reservation: "+reservation.Id.Hex())

	return nil
}

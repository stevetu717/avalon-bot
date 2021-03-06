package util

import (
	"errors"
	"github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"time"
)

var Loc, _ = time.LoadLocation("America/New_York")
var DateTimeRegex = regexp.MustCompile(dateTimeRegexRaw)
var ActivityRegex = regexp.MustCompile(activityRegexRaw)

func ContainsIgnoreCase(string string, substring string) bool {
	return strings.Contains(strings.ToLower(string), substring)
}

func LogInfo(log *logrus.Logger, message string) {
	log.WithFields(logrus.Fields{
		"app": "racquetball-bot",
	}).Info(message)
}

func LogDebug(log *logrus.Logger, message string) {
	log.WithFields(logrus.Fields{
		"app": "racquetball-bot",
	}).Debug(message)
}

func LogError(log *logrus.Logger, error interface{}) {
	log.WithFields(logrus.Fields{
		"app": "racquetball-bot",
	}).Error(error)
}

func LogSMSError(log *logrus.Logger, error interface{}, userPhoneNumber string, message string) {
	log.WithFields(logrus.Fields{
		"app":     "racquetball-bot",
		"to":      userPhoneNumber,
		"message": message,
	}).Error(error)
}

func GetDateTimeUTC(input string) (time.Time, error) {
	dateTimeString := DateTimeRegex.FindString(input)

	if dateTimeString == "" {
		return time.Time{}, errors.New("no valid date/time provided")
	}

	dateTime, err := time.ParseInLocation(ReservationDateTimeLayout, dateTimeString, Loc)

	if err != nil {
		return time.Time{}, errors.New("Unable to parse datetime: " + input)
	}

	dateTime = dateTime.In(time.UTC)
	dateTime = time.Date(dateTime.Year(), dateTime.Month(), dateTime.Day(), dateTime.Hour(), 0, 0, 0, time.UTC)

	return dateTime, err
}

func GetActivity(body string) (string, error) {
	activity := ActivityRegex.FindString(body)

	if activity == "" {
		return "", errors.New("no valid activity provided")
	}

	return activity, nil
}

// Returns the duration until reservations become schedulable
// i.e. the day before the datetime but with 30 seconds extra to prepare payload
func DurationUntilSchedulable(datetime time.Time) time.Duration {
	datetime = datetime.In(Loc)
	datetime = datetime.AddDate(0, 0, -2)
	datetime = time.Date(datetime.Year(), datetime.Month(), datetime.Day(), 23, 59, 30, 0, Loc)
	d := datetime.Sub(time.Now().In(Loc))
	return d
}

func DurationFromNowInLoc(datetime time.Time, Loc *time.Location) time.Duration {
	now := time.Now().In(Loc)
	d := datetime.In(Loc).Sub(now)
	return d
}

func DateTimeWithinTwoDays(dateTime time.Time) bool {
	endDate := time.Now().In(Loc).Add(time.Hour * 48)
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, Loc)
	return dateTime.In(Loc).Before(endDate)
}
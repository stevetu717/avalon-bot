package util

const (
	Schedule         = "schedule"
	ReservationSaved = "Your reservation has been saved. We will attempt to secure it the day before the reservation. Thank you!"
	ReservationError = "Failed to save the reservation. Contact the dev with Rsvp ID: "

	// SMS
	SmsHelp   = "To use this reservation system please text a message in the format of: <activity> mm/dd/yyyy hh:mm <am/pm>. " +
		"Valid activities are racquetball, basketball, and tennis."
	SmsInvalidDateTime = "Please enter a date and time in the correct format. Text 'assist' for help."
	SmsInvalidDateTimeRange = "Amenities are only open between 8AM and 8PM EST. Please try again with a valid time."
	SmsInvalidActivity = "Please enter a valid activity you would like to schedule. Text 'assist' for help."
	SmsSuccessfulReservation = "Your reservation has been successfully made for %s on %s."
	SmsFailedReservation = "We were unable to make your reservation for %s on %s. It may have been taken or the website has changed. Contact the dev!"
)

const (
	dateTimeRegexRaw          = `(?i)(0?[1-9]|1[012])[-\/.](0?[1-9]|[12][0-9]|3[01])[-\/.]2[0-9]\s((0[1-9]:[0-5][0-9]((AM)|(PM)))|([1-9]:[0-5][0-9]((AM)|(PM)))|(1[0-2]:[0-5][0-9]((AM)|(PM))))`
	activityRegexRaw          = `(?i)racquetball|basketball|tennis`
	ReservationDateTimeLayout = `1/2/06 3:04pm`
	AvalonBaseUrl             = "https://www.avalonaccess.com"
	AvalonLoginUrl            = AvalonBaseUrl + "/UserProfile/LogOn"
	AvalonAmenityUrl          = AvalonBaseUrl + "/Information/Information/AmenityReservation?amenityKey="
	AvalonAmenitiesUrl        = AvalonBaseUrl + "/Information/Information/Amenities"
	AvalonSaveReservationUrl  = AvalonBaseUrl + "/Information/Information/SaveAmenityReservation"
	VerificationTokenXpath    = "//form//input[@name=\"__RequestVerificationToken\"]"
	UpcomingReservationsXpath = "//*[@id=\"upcomingReservation\"]/div/div"
)


package services

import (
	"errors"
	"github.com/antchfx/htmlquery"
	"github.com/sirupsen/logrus"
	"github.com/stevetu717/racquetball-bot/internal/pkg/util"
	"github.com/stevetu717/racquetball-bot/model"
	"golang.org/x/net/html"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type AvalonService struct {
	Logger        *logrus.Logger
	AvalonDetails model.AvalonDetails
	HttpClient    *http.Client
}

func (as *AvalonService) MakeReservation(r *model.Reservation) error {
	session := getSession()
	err := as.login(session)
	payload, err := as.prepareReservation(r, session)
	err = as.submitReservation(r, session, payload)
	err = as.validateReservation(r, session)
	return err
}

func getSession() *http.Client {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
	}

	return client
}

func (as *AvalonService) login(session *http.Client) error {
	htmlDoc, err := as.getHtmlDoc(session, util.AvalonLoginUrl)
	userVerificationNode, err := as.getNode(htmlDoc, util.VerificationTokenXpath)
	if err != nil {
		return err
	}

	userVerificationToken, err := getVerificationToken(userVerificationNode)
	if err != nil {
		util.LogError(as.Logger, err)
		return err
	}

	response, err := session.PostForm(util.AvalonLoginUrl, url.Values{
		"UserName":                   {as.AvalonDetails.Username},
		"password":                   {as.AvalonDetails.Password},
		"__RequestVerificationToken": {userVerificationToken},
	})

	if err != nil {
		util.LogDebug(as.Logger, "Unable to make POST request for url: "+util.AvalonLoginUrl)
		util.LogError(as.Logger, err)
		return err
	}

	defer response.Body.Close()

	if response.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(response.Body)
		err = errors.New("HTTP Request failed for url: "+util.AvalonLoginUrl+" - Status Code: "+strconv.Itoa(response.StatusCode)+" - Message: "+string(body))
		util.LogError(as.Logger, err)
		return err
	}

	return nil
}

func (as *AvalonService) prepareReservation(rsvp *model.Reservation, session *http.Client) (url.Values, error) {
	amenity := as.AvalonDetails.Amenities[rsvp.Activity]

	htmlDoc, err := as.getHtmlDoc(session, util.AvalonAmenityUrl+amenity.Key)
	if err != nil {
		return nil, err
	}

	amenityVerificationNode, err := as.getNode(htmlDoc, util.VerificationTokenXpath)
	if err != nil {
		return nil, err
	}

	amenityVerificationToken, err := getVerificationToken(amenityVerificationNode)

	if err != nil {
		util.LogError(as.Logger, err)
		return nil, err
	}

	payload := as.createPayload(rsvp, amenity, amenityVerificationToken)
	return payload, nil
}

func (as *AvalonService) getHtmlDoc(session *http.Client, url string) (string, error) {
	response, err := session.Get(url)

	if err != nil {
		util.LogDebug(as.Logger, "Unable to make GET request for url: "+url)
		util.LogError(as.Logger, err)
		return "", err
	}

	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)

	if response.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(response.Body)
		err = errors.New("HTTP Request failed for url: "+url+" - Status Code: "+strconv.Itoa(response.StatusCode)+" - Message: "+string(body))
		util.LogError(as.Logger, err)
		return "", err
	}

	return string(body), nil
}

func getVerificationToken(node *html.Node) (string, error) {
	for _, attribute := range node.Attr {
		if attribute.Key == "value" {
			return attribute.Val, nil
		}
	}

	return "", errors.New("unable to find attribute containing verification token")
}

func (as *AvalonService) getNode(html string, xPath string) (*html.Node, error) {
	doc, err := htmlquery.Parse(strings.NewReader(html))

	if err != nil {
		util.LogDebug(as.Logger, "Unable to parse HTML document")
		util.LogError(as.Logger, err)
		return nil, err
	}

	nodes, err := htmlquery.QueryAll(doc, xPath)

	if err != nil || len(nodes) == 0 {
		util.LogDebug(as.Logger, "Unable to find element with XPATH: "+xPath)
		util.LogError(as.Logger, err)
		return nil, err
	}

	return nodes[0], nil
}

func (as *AvalonService) getNodes(html string, xPath string) ([]*html.Node, error) {
	doc, err := htmlquery.Parse(strings.NewReader(html))

	if err != nil {
		util.LogError(as.Logger, "Unable to parse HTML document")
		util.LogError(as.Logger, err)
		return nil, err
	}

	nodes, err := htmlquery.QueryAll(doc, xPath)

	if err != nil || len(nodes) == 0 {
		util.LogDebug(as.Logger, "Unable to find elements with XPATH: "+xPath)
		util.LogError(as.Logger, err)

		return nil, err
	}

	return nodes, nil
}

func (as *AvalonService) createPayload(rsvp *model.Reservation, amenity model.Amenity, amenityVerificationToken string) url.Values {
	rsvpDateTime := rsvp.Datetime.In(util.Loc)
	rsvpDate := rsvpDateTime.Format("1/2/2006")
	minDate := rsvpDateTime.Format("1/2/2006") + " 4:00:00 AM"
	maxDate := rsvpDateTime.AddDate(0, 0, 1).Format("1/2/2006") + " 4:00:00 AM"
	selStartTime := rsvpDateTime.Format("Monday-3:04 PM-") + rsvpDateTime.Add(1*time.Hour).Format("3:04 PM")

	payload := url.Values{
		"__RequestVerificationToken": {amenityVerificationToken},
		"AmenityKey":                 {amenity.Key},
		"AmenityId":                  {amenity.Id},
		"Id":                         {""},
		"LeaseId":                    {as.AvalonDetails.LeaseId},
		"PersonId":                   {as.AvalonDetails.PersonId},
		"AmenityName":                {amenity.Name},
		"ReservationMaxDate":         {maxDate},
		"ReservationMinDate":         {minDate},
		"TermsandConditions":         {"True"},
		"Charge":                     {"0"},
		"ReservationDate":            {rsvpDate},
		"Notes":                      {""},
		"SelStartTime":               {selStartTime},
		"NumberOfPeople":             {"1"},
		"ReservationNames":           {as.AvalonDetails.ReservationName},
		"reservation-terms":          {"on"},
	}

	return payload
}

func (as *AvalonService) submitReservation(r *model.Reservation, session *http.Client, payload url.Values) error {
	if !util.DateTimeWithinTwoDays(r.Datetime){
		tom := time.Now().In(util.Loc).Add(24 * time.Hour)
		schedulableTime := time.Date(tom.Year(), tom.Month(), tom.Day(), 0, 0, 0, 0, util.Loc)
		dur := util.DurationFromNowInLoc(schedulableTime, util.Loc)
		util.LogInfo(as.Logger, "Sleeping for "+string(dur.Milliseconds())+" milliseconds...")
		time.Sleep(dur)
	}

	util.LogInfo(as.Logger, "Making reservation request for " + r.CreatedBy + " activity: " + r.Activity)
	response, err := session.Post(util.AvalonSaveReservationUrl, "application/x-www-form-urlencoded", strings.NewReader(payload.Encode()))

	if err != nil {
		util.LogDebug(as.Logger, "Unable to make POST request for url: "+util.AvalonSaveReservationUrl)
		util.LogError(as.Logger, err)
		return err
	}

	defer response.Body.Close()

	if response.StatusCode >= 300 {
		body, err := ioutil.ReadAll(response.Body)
		err = errors.New("HTTP Request failed for url: "+util.AvalonSaveReservationUrl+" - Status Code: "+strconv.Itoa(response.StatusCode)+" - Message: "+string(body))
		util.LogError(as.Logger, err)
		return err
	}

	util.LogInfo(as.Logger, "Request has been posted successfully. Confirming request was accepted...")

	return err
}

func (as *AvalonService) validateReservation(rsvp *model.Reservation, session *http.Client) error {
	confirmed := false
	rsvpDateTime := rsvp.Datetime.In(util.Loc)
	htmlDoc, err := as.getHtmlDoc(session, util.AvalonAmenitiesUrl)

	if err != nil {
		return err
	}

	upcomingReservationsNodes, err := as.getNodes(htmlDoc, util.UpcomingReservationsXpath)
	if err != nil {
		return err
	}

	for _, node := range upcomingReservationsNodes {
		amenity := getUpcomingReservationAmenity(node)
		if strings.Contains(amenity, as.AvalonDetails.Amenities[rsvp.Activity].Name) {
			confirmationDateTime := getUpcomingReservationAmenityDetails(node)
			rsvpDate := rsvpDateTime.Format("January 02, 2006")
			rsvpStartTime := rsvpDateTime.Format("3:04 PM")
			rsvpEndTime := rsvpDateTime.Add(1*time.Hour).Format("3:04 PM")
			if strings.Contains(confirmationDateTime, rsvpDate) && strings.Contains(confirmationDateTime, rsvpStartTime) && strings.Contains(confirmationDateTime, rsvpEndTime) {
				confirmed = true
			}
		}
	}

	if confirmed {
		return nil
	}

	return errors.New("failed to confirm reservation")
}

func getUpcomingReservationAmenityDetails(node *html.Node) string {
	return node.FirstChild.NextSibling.FirstChild.NextSibling.NextSibling.NextSibling.FirstChild.Data

}

func getUpcomingReservationAmenity(node *html.Node) string {
	return node.FirstChild.NextSibling.FirstChild.NextSibling.FirstChild.FirstChild.FirstChild.Data
}

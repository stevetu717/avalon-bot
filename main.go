package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/sfreiberg/gotwilio"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stevetu717/racquetball-bot/internal/pkg/services"
	"github.com/stevetu717/racquetball-bot/internal/pkg/util"
	"github.com/stevetu717/racquetball-bot/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	_ "net/http"
	"time"
)

func main() {
	env := flag.String("env", "local", "defines the env")
	flag.Parse()
	rootContext := context.Background()
	logger := initLogger()
	config := initConfig(*env)

	// Init Twilio
	twilioService := gotwilio.NewTwilioClient(config.Twilio.TwilioAccountSid,
		config.Twilio.TwilioAuthToken)

	// Init AvalonService
	avalonService := &services.AvalonService{Logger: logger, AvalonDetails: config.Avalon, HttpClient: &http.Client{}}

	// Init DB
	dbURI := config.Mongo.URI
	dbClient, err := GetMongoClient(rootContext, dbURI)
	if err != nil {
		logger.Fatal("Unable to establish connection with database - ", err)
	}
	database := dbClient.Database("reservations")
	collection := database.Collection("reservations")

	// Validate DB
	err = validateDB(rootContext, collection, logger)
	if err != nil {
		logger.Fatal("Unable to clean up database - ", err)
	}

	// Init SMSHandler
	smsService := services.NewSMSHandler(logger, collection, twilioService, avalonService, config)

	// Load All Jobs
	loadJobs(rootContext, collection, logger, avalonService, smsService)

	// Init WebServer
	serveMux := http.NewServeMux()
	serveMux.Handle("/", smsService)

	http.ListenAndServe(":8080", serveMux)
}

func loadJobs(ctx context.Context, collection *mongo.Collection, logger *logrus.Logger, avalonService *services.AvalonService, smsService *services.SMSHandler) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.D{})
	if err != nil {
		logger.Fatal("Exception occurred while retrieving jobs - ", err)
	}

	util.LogInfo(logger, "========== LOADING ALL JOBS INTO SCHEDULER ==========")

	count := cursor.RemainingBatchLength()
	for cursor.Next(context.Background()) {
		reservation := model.Reservation{}
		if err = cursor.Decode(&reservation); err != nil {
			logger.Fatal("Unable to serialize document to Reservation - ", err)
		}

		smsService.ScheduleJob(&reservation, collection, avalonService)
		util.LogInfo(logger, "Added Reservation: "+reservation.Id.Hex()+" to scheduler...")
	}

	util.LogInfo(logger, fmt.Sprintf("========== LOADED %d JOBS INTO SCHEDULER ==========", count))

}

func validateDB(ctx context.Context, collection *mongo.Collection, logger *logrus.Logger) error {
	curTime := time.Now().UTC() //TODO: delete all less than tomorrow at 12am
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{
		"date_time": bson.M{
			"$lte": curTime,
		},
	}

	_, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		util.LogDebug(logger, "failed to clean up database")
		util.LogError(logger, err)
		return err
	}

	return nil
}

func initLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	return logger
}

func initConfig(env string) *model.Config {
	config := getLocalConfig()
	if env != "local" {
		config = getAwsConfig(config)
	}

	return config
}

func getAwsConfig(config *model.Config) *model.Config {
	twilioSID, _ := getParam("/avalon-bot/prod/twilio-sid")
	twilioToken, _ := getParam("/avalon-bot/prod/twilio-api-key")
	twilioPhone, _ := getParam("/avalon-bot/prod/twilio-phone")
	username, _ := getParam("/avalon-bot/prod/username")
	password, _ := getParam("/avalon-bot/prod/password")
	dbURI, _ := getParam("/avalon-bot/prod/db-uri")
	leaseId, _ := getParam("/avalon-bot/prod/lease-id")
	personId, _ := getParam("/avalon-bot/prod/person-id")

	if len(twilioSID) == 0 || len(twilioToken) == 0 || len(twilioPhone) == 0 || len(username) == 0 || len(password) == 0 || len(dbURI) == 0 || len(leaseId) == 0 || len(personId) == 0{
		log.Fatal("Unable to retrieve params from AWS parameter store.")
	}

	config.Twilio.TwilioAccountSid = twilioSID
	config.Twilio.TwilioAuthToken = twilioToken
	config.Twilio.PhoneNumber = twilioPhone
	config.Avalon.Username = username
	config.Avalon.Password = password
	config.Mongo.URI = dbURI
	config.Avalon.LeaseId = leaseId
	config.Avalon.PersonId = personId

	return config
}

func getParam(name string) (string, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{Region: aws.String("us-east-2b"), CredentialsChainVerboseErrors: aws.Bool(true)},
		SharedConfigState: session.SharedConfigEnable,
	})

	if err != nil{
		log.Fatal("Unable to create AWS Session", err)
	}

	svc := ssm.New(sess, aws.NewConfig().WithRegion("us-east-2"))

	output, err := svc.GetParameter(
		&ssm.GetParameterInput{
			Name:           aws.String(name),
			WithDecryption: aws.Bool(true),
		},
	)

	if err != nil {
		log.Fatal("Unable to get parameter " + name + " from AWS Session - ", err)
	}

	return aws.StringValue(output.Parameter.Value), nil
}

func getLocalConfig() *model.Config {
	viper.SetConfigFile("config.yml")
	err := viper.ReadInConfig()

	if err != nil {
		log.Fatal("Unable to read in config file", err)
	}

	var config model.Config
	err = viper.Unmarshal(&config)

	if err != nil {
		log.Fatal("Unable to unmarshal config file to struct", err)
	}

	return &config
}

func GetMongoClient(ctx context.Context, URI string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(URI))
	return client, err
}

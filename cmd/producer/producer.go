package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"kafka-notify/pkg/models"
	"log"
	"net/http"
	"strconv"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
)

const (
    ProducerPort       = ":8080"
    KafkaServerAddress = "localhost:9092"
    KafkaTopic         = "notifications"
)

// ============== HELPER FUNCTIONS ==============
var ErrUserNotFoundInProducer = errors.New("user not found")

func findUserByID(id int, users []models.User) (models.User, error) {
    for _, user := range users {
        if user.ID == id {
            return user, nil
        }
    }
    return models.User{}, ErrUserNotFoundInProducer
}


func getIDFromRequest(formValue string, ctx *gin.Context) (int,error) {
	id,err := strconv.Atoi(ctx.PostForm(formValue))

	if err!=nil {
		return 0, fmt.Errorf(
			"failed to parse ID from form value %s: %w", formValue, err)
	}

	return id,nil
}

//=========Kafka Related Function ==========//

func sendKafkaMessage(producer sarama.SyncProducer,users []models.User,ctx *gin.Context,fromID,toId int) error{
	message := ctx.PostForm("message")

	fromUser, err := findUserByID(fromID,users)

	if err != nil {
		return err
	}

	toUser, err := findUserByID(toId,users)

	if err != nil {
		return err
	}

	notification := models.Notification {
		From: fromUser,
		To: toUser,
		Message: message,
	}

	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
    
	msg := &sarama.ProducerMessage{
		Topic: KafkaTopic,
		Key: sarama.StringEncoder(strconv.Itoa(toId)),
		Value: sarama.StringEncoder(notificationJSON),

	}

	_,_,err = producer.SendMessage(msg)

	return err
}

func sendMessageHandler(producer sarama.SyncProducer, users []models.User) gin.HandlerFunc {
	return func(ctx *gin.Context){
		fromId,err := getIDFromRequest("fromID",ctx)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}

		toId, err := getIDFromRequest("toID",ctx)
		if err !=nil {
			ctx.JSON(http.StatusBadRequest,gin.H{"message": err.Error()})
			return
		}

		err = sendKafkaMessage(producer,users,ctx,fromId,toId)

		if err !=nil {
			ctx.JSON(http.StatusInternalServerError,gin.H{"message": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"message": "Notification sent successfully!",
		})
	}

}

func setUpProducer() (sarama.SyncProducer,error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	producer,err := sarama.NewSyncProducer([]string{KafkaServerAddress},config)

	if err != nil {
		return nil, fmt.Errorf("failed to setup producer: %w", err)
	}

	return producer,nil
}


func main(){
	users := []models.User{
		{ID: 1, Name: "Emma"},
		{ID: 2, Name: "Bruno"},
		{ID: 3, Name: "Rick"},
		{ID: 4, Name: "Lena"},
	}

	producer,err := setUpProducer()

	if err != nil {
		log.Fatalf("Failed to initilized producer : %v",err)
	}

	defer producer.Close()

	gin.SetMode(gin.ReleaseMode)
	router:= gin.Default()
	router.POST("/send", sendMessageHandler(producer, users))

	fmt.Printf("Kafka Producer is started at localhost:%s\n",ProducerPort)

	if err := router.Run(ProducerPort);err != nil {
        log.Printf("Failed to run the server!. Error : %v" ,err)
	}
}


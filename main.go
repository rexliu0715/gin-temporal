package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/huandu/xstrings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"go.temporal.io/sdk/client"
)

const (
	invalidRequest    = "Invalid Request"
	unauthorizedError = "Unauthorized"
	serverError       = "Internal Server Error"
	okResponse        = "OK"
)

var (
	TASK_QUEUE            string
	SERVER_PORT           int
	TEMPORAL_TOKEN        string
	TEMPORAL_ADDRESS      string
	TEMPORAL_NAMESPACE    string
	TEMPORAL_HEADER_TOKEN string
	temporalClientOptions client.Options
)

func Init() {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading .env file")
	}

	SERVER_PORT, _ = strconv.Atoi(os.Getenv("SERVER_PORT"))
	TEMPORAL_TOKEN = os.Getenv("TEMPORAL_TOKEN")
	TEMPORAL_HEADER_TOKEN = os.Getenv("TEMPORAL_HEADER_TOKEN")
	TEMPORAL_ADDRESS = os.Getenv("TEMPORAL_ADDRESS")
	TEMPORAL_NAMESPACE = os.Getenv("TEMPORAL_NAMESPACE")
	TASK_QUEUE = os.Getenv("TASK_QUEUE")

	temporalClientOptions = client.Options{
		HostPort:  TEMPORAL_ADDRESS,
		Namespace: TEMPORAL_NAMESPACE,
	}
}

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Header.Get(TEMPORAL_HEADER_TOKEN) != TEMPORAL_TOKEN {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": unauthorizedError,
			})
		}
	}
}

func main() {
	Init()

	gin.SetMode(os.Getenv("GIN_MODE"))

	r := gin.Default()

	r.Use(Auth())

	r.POST("/:workflowType/:workflowName", handleWorkflow)

	r.Run(":" + strconv.Itoa(SERVER_PORT)) // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

func handleWorkflow(c *gin.Context) {
	workflowType := xstrings.ToCamelCase(c.Param("workflowType"))
	workflowName := xstrings.ToCamelCase(c.Param("workflowName"))

	temporalClient, err := client.Dial(temporalClientOptions)
	if err != nil {
		log.Println("Error dialing to Temporal server: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"errors": serverError,
		})
		return
	}

	defer temporalClient.Close()

	id := uuid.New().String()

	workflowOptions := client.StartWorkflowOptions{
		ID:                       id,
		TaskQueue:                TASK_QUEUE,
		WorkflowExecutionTimeout: 60 * time.Second,
	}

	var input *interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Println("Error binding JSON request: ", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": invalidRequest,
		})
		return
	}

	workflowRun, err := temporalClient.ExecuteWorkflow(context.Background(), workflowOptions, workflowType+workflowName, input)
	if err != nil {
		log.Println("Error executing workflow: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"id":     id,
			"errors": serverError,
		})
		return
	}

	var result interface{}
	if err := workflowRun.Get(context.Background(), &result); err != nil {
		log.Println("Error getting workflow response: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"id":     id,
			"errors": serverError,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":   id,
		"data": result,
	})
}

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
	taskQueue             string
	serverPort            int
	temporalToken         string
	temporalAddress       string
	temporalNamespace     string
	temporalHeaderToken   string
	temporalClientOptions client.Options
)

func Init() {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading .env file")
	}

	serverPort, _ = strconv.Atoi(os.Getenv("SERVER_PORT"))
	temporalToken = os.Getenv("TEMPORAL_TOKEN")
	temporalHeaderToken = os.Getenv("TEMPORAL_HEADER_TOKEN")
	temporalAddress = os.Getenv("TEMPORAL_ADDRESS")
	temporalNamespace = os.Getenv("TEMPORAL_NAMESPACE")
	taskQueue = os.Getenv("TASK_QUEUE")

	temporalClientOptions = client.Options{
		HostPort:  temporalAddress,
		Namespace: temporalNamespace,
	}
}

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println(temporalHeaderToken)
		token := c.Request.Header.Get(temporalHeaderToken)
		if token != temporalToken {
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

	r.Run(":" + strconv.Itoa(serverPort)) // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

type workflowInput map[string]interface{}

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
		TaskQueue:                taskQueue,
		WorkflowExecutionTimeout: 60 * time.Second,
	}

	var input workflowInput
	if err := c.BindJSON(&input); err != nil {
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

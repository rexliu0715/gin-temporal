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
	temporalClient        client.Client
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

	c, err := client.Dial(temporalClientOptions)
	if err != nil {
		log.Fatal("Error creating Temporal client: ", err)
	}

	temporalClient = c
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
	ctx := context.Background()

	Init()
	// Graceful shutdown
	go func() {
		defer temporalClient.Close()
		<-ctx.Done()
	}()

	gin.SetMode(os.Getenv("GIN_MODE"))

	r := gin.Default()

	r.Use(Auth())

	r.POST("/:workflowService/:workflowName", handleWorkflow)

	r.Run(":" + strconv.Itoa(SERVER_PORT)) // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

func handleWorkflow(c *gin.Context) {
	taskQueue := c.Request.Header.Get("Temporal-Task-Queue")
	workflowName := xstrings.ToCamelCase(c.Param("workflowService")) + xstrings.ToCamelCase(c.Param("workflowName"))
	workflowExecutionTimeoutStr := c.Request.Header.Get("Temporal-Workflow-Execution-Timeout")

	log.Printf("Starting workflow %s in queue %s", workflowName, taskQueue)

	var input *interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("Error in binding JSON for workflow %s in queue %s: %+v", workflowName, taskQueue, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": invalidRequest,
		})
		return
	}

	workflowExecutionTimeout := 10

	if workflowExecutionTimeoutStr != "" {
		timeout, err := strconv.Atoi(workflowExecutionTimeoutStr)
		if err != nil {
			log.Printf("Error converting workflow timeout string to int: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"errors": serverError,
			})
			return
		}
		workflowExecutionTimeout = timeout
	}

	log.Println("workflowExecutionTimeout", workflowExecutionTimeout)

	workflowOptions := client.StartWorkflowOptions{
		TaskQueue:                taskQueue,
		WorkflowExecutionTimeout: time.Duration(workflowExecutionTimeout) * time.Second,
	}

	workflowRun, err := temporalClient.ExecuteWorkflow(context.Background(), workflowOptions, workflowName, input)

	if err != nil {
		log.Printf("Error executing workflow %s in queue %s: %+v", workflowName, taskQueue, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"errors": serverError,
		})
		return
	}

	var result interface{}
	if err := workflowRun.Get(context.Background(), &result); err != nil {
		log.Printf("Error getting response from workflow %s in queue %s: %+v", workflowName, taskQueue, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"id":     workflowRun.GetID(),
			"errors": serverError,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":   workflowRun.GetID(),
		"data": result,
	})
}
